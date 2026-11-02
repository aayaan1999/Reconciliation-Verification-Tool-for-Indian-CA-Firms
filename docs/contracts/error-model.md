# Error model

Purpose: what can go wrong, how each kind of problem is named, and how it reaches the user. It summarises the technical spec §A6.1 (adapter errors), §B2.1 (validation report), §B4.4 (missing inputs), §B4.6 (output wording) and §B6 (engine execution), checked against `internal/ingest/errors.go`, `internal/ingest/report.go`, `cmd/recon-engine/main.go`, `cmd/recon-engine/run.go` and `internal/api/api.go`.

## The principle: the engine never guesses

The tool asserts only what it can compute (spec §2, project invariant 1). When it cannot read or compute something it says so, with a stable code and the place, and moves the item out of the findings. It never:

- treats a missing figure as zero or as a mismatch: a rule with a missing input is **Cannot check** (§B4.4), not a finding;
- picks one of two different values read for the same figure: the run stops with `CONFLICTING_VALUES` (§B6 step 4);
- picks a list entry by position: a figure in a list is read from the entry that meets the profile's conditions, and two entries meeting them is `ENTRY_AMBIGUOUS` (SPEC-006 part 2);
- picks one of two matching formats: that is `PROFILE_AMBIGUOUS`;
- reads a figure by OCR or a language model (no OCR in Tool 1; no LLM in the finding path, invariant 2).

## Categories

| Category | Where it arises | How it surfaces | Exit code |
|---|---|---|---|
| **Input refused** | A file that cannot be read at all | Run stops before anything is written; standard error carries the code and file name | 2 |
| **Validation error in a file** | A file that reads but has a problem (wrong tax year, a required figure absent) | The file's validation report lists it; the file is stored but its figures are not used | 0 or 1 |
| **Cannot check** | A rule whose input is not in the current view | Listed under "Cannot check" with the missing figure and the document it comes from | 0 or 1 |
| **Rule or content error** | A rule, rule-store file, vocabulary or comparisons file that does not load | `load-rules` or `run` stops; the message names the rule and file | 2 |
| **Refused request** | Bad flag, unknown client, finding or Worklist item, missing note | Stable code on standard error; API status 400/404/409/422 | 2 |
| **Internal error** | A bug or environment fault | `internal error:` on standard error; API `500 INTERNAL` | 3 |
| **Finding** | A rule that fails | In the report, grouped by review document | 1 |

A finding is not an error: it is the product working. Exit code 1 means "findings present".

## Ingestion codes (`internal/ingest/errors.go`)

An ingestion failure is an `*ingest.Error` with a stable `Code`, the file name and, when known, line, column and byte offset. Codes are shown in validation reports and matched by tests and the UI, so their text never changes.

| Code | Meaning | Added in |
|---|---|---|
| `UNSUPPORTED_FILE_TYPE` | No adapter recognises the file, or more than one does | SPEC-003 |
| `FILE_TOO_LARGE` | Over the adapter's size limit (JSON, CSV and Excel default 64 MB), or an Excel workbook that unzips to more than four times that | SPEC-003, SPEC-008 |
| `PARSE_ERROR` | Not valid for its type: malformed JSON, invalid UTF-8, a repeated key, text after the value, nesting over 512 levels; for CSV an unnamed or repeated column, a short row or an open quote (always with line and column); for Excel a damaged workbook, or a header label the profile names found twice in its header row | SPEC-003, SPEC-006 part 3, SPEC-008 |
| `PROFILE_NO_MATCH` | No format profile's `detect` block matches | SPEC-006 |
| `PROFILE_AMBIGUOUS` | Several profiles match (outside a single bundle) | SPEC-006 |
| `REQUIRED_FIELD_MISSING` | A required figure is not in the file | SPEC-006 |
| `CONFLICTING_VALUES` | One figure read with two different values; also a file for another tax year, a PAN that is not the client's, or two values for one figure across files in a run | SPEC-006 |
| `ENTRY_AMBIGUOUS` | Several list entries meet the conditions of a figure read from one | SPEC-006 part 2 |
| `HIDDEN_CONTENT` | A warning, not an error: a hidden sheet, row or column of an Excel workbook was read like a visible one | SPEC-008 |
| `EMPTY_ROWS` | A warning: a table of an Excel sheet has empty rows inside it; the rows below were read as part of the table | SPEC-008 part 2 |
| `GSTIN_FROM_CLIENT` | A warning: a GST document that does not say whose GSTIN it is was given the client's only GSTIN (named by its last four characters) | SPEC-008 part 2 |

`UNSUPPORTED_FILE_TYPE`, `PARSE_ERROR`, `PROFILE_NO_MATCH` and `PROFILE_AMBIGUOUS` stop the run (exit 2). The run detects and extracts every input file before writing anything, so a refused file leaves the database unchanged (§B6 step 6). Examples are in `testdata/run/unknown` and `testdata/run/damaged`.

Planned: `IMAGE_NOT_SUPPORTED` for photos and fully scanned PDFs (§B2.1, SPEC-044), and the engine's `job.failed` message with an `error_code` (§A3.1).

## The validation report (`internal/ingest/report.go`, IN-14)

Every file read gets a report, shown under the run's files and stored in `document_versions.validation_report_json`. It holds:

- what the file was recognised as (profile ID and version, source system, document type), its tax year and whether its PAN matched the client (`matched`, `mismatched`, `not_checked`; only the last four characters are shown);
- how many facts and records were read;
- **Not read**: every part of the file not turned into a fact or record, with its locator and why (`row_left_out`, `column_not_used`, `not_read`);
- warnings and errors, each with code, message and locator. **Any error means the file's figures are not used.**

Lists are sorted (by locator, then code and message), so the same file always gives a byte-identical report (IN-21). `testdata/run/missing` shows `REQUIRED_FIELD_MISSING` leading to Cannot check rather than a finding.

## Engine and API refusal codes

Besides the ingestion codes, `recon-engine` uses these on standard error (exit 2): `BAD_REQUEST`, `BAD_NAME`, `BAD_DECISION`, `BAD_NOTE`, `NOTE_REQUIRED`, `BAD_KIND`, `BAD_CLASSIFICATION`, `CLIENT_EXISTS`, `CLIENT_NOT_FOUND`, `INVALID_CLIENT`, `FINDING_NOT_FOUND`, `WORKLIST_ITEM_NOT_FOUND`, `COMPARISON_NOT_FOUND`, `RULES_NEED_COMPARISON`, `RULE_NOT_IN_COMPARISON`, `NO_RULES_SELECTED`, `INVALID_MAPPING`, `PROFILE_EXISTS`. Flag and file problems without a code are reported as plain messages and become `INPUT_ERROR` in the API. Client detail checks (`BAD_PAN`, `BAD_GSTIN`, `GSTIN_CHECK_DIGIT`, ...) are listed together in one refusal (§B1.3). The HTTP mapping is in [api-conventions.md](api-conventions.md).

## Rule load errors

- `load-rules` refuses a vocabulary, rule store or engine rule that does not load, naming the folder and the file (exit 2). Rules are read with strict YAML: an unknown key, an anchor or an unquoted tax year is a load error, not a run-time surprise (§B4.1).
- `run` re-checks each stored rule against the run's vocabulary; a rule that no longer loads stops the run (`rule <ID> does not load`).
- A profile selector that the adapter cannot parse (for example an unsupported JSON path) is a load error of the profile (§A6.1).
- `rulestore-check`, `rule-lint` and `profile-test` find these problems before a run; see [../quality/ci-quality-gates.md](../quality/ci-quality-gates.md).

## Finding wording (§B4.6, project invariant 1)

Every finding shows:

- both (or all) values, each with its label;
- the difference, computed and in Indian digit grouping (`1,12,450`);
- every source locator (file plus path or page);
- the law reference, when the rule names one.

Finding text never says "should", never states a correct or expected value, and never makes an applicability or characterisation judgment. The banned phrases are `should`, `incorrect`, `must be`, `wrong`, `correct value`, `ought`, `required to` (`rules.BannedPhrases` in `internal/rules/wording.go`), matched as whole words ignoring case. They are refused in rule titles and labels, and the screens' list must equal the Go list (`web/src/App.test.tsx`). A Cannot check line reads, for example, "Cannot check: figure not found in Tax audit report: ...". The rule book's remedy and judgment columns are kept as reviewer notes and never reach finding text (§B4.7).

## Open

- Finding states and system transitions after a run (§B7.2, §B7.3) need CA sign-off (§B17 T6).
- Scans with an invisible OCR text layer (§B17 T7, open question S-Q1).
