# SPEC-003 — Rules into the database, and one engine run on one sample file

| | |
|---|---|
| **Status** | Not started |
| **Requirements** | RU-01, RU-02, RU-51, RU-52, RU-53, IN-11, IN-14, NF-01, NF-03 |
| **Technical spec** | §A7, §A8, §A10, §B1, §B2.2, §B4.2, §B4.4, §B4.6, §B6, §B15 |
| **Depends on** | SPEC-001 (YAML rule store) |
| **Paths** | `cmd/recon-engine`, `internal/store` (+ `internal/store/migrations`), `internal/engine`, `internal/findings`, `cmd/recon-dev` (`rulestore-import`, `rulestore-check`), `docs/Recon_Tool_YAML_Rule_Store/recon_yaml/rules/categories.yaml`, `testdata/run/` |

## Goal

One command takes one sample file and gives a result: it recognises the file, reads its figures, runs the rules and prints a report with an exit code. The rules it uses come from the database, loaded from the YAML files.

## Scope

### 1. Category file for the rule store

- `rulestore-import` also writes `rules/categories.yaml`: every `rule.category` found in the rule files, sorted, each with its `type`, `domain` and the sorted list of rule IDs in it. Every text value is quoted (RU-02).
- `rulestore-check` fails if `categories.yaml` is missing, out of date with the rule files, or lists a rule that does not exist.

### 2. Rules into the database

- `internal/store` opens a plain SQLite file (encryption comes in Module 14) and applies numbered migrations from `internal/store/migrations` in one transaction (§B1). Migration `0001` creates only what this spec needs: `rule_versions`, `rule_state`, `documents`, `document_versions`, `facts`, `runs`, `run_documents`, `run_rules`, `evaluations`. Columns follow §B1.1. Money columns are `INTEGER` paise named `*_paise`.
- `recon-engine load-rules --db <file> --store <rule store folder> --rules <engine rules folder>`:
  - Loads all 304 rule-store files (strict YAML) into `rule_versions` (`source = 'rulebook'`, `body_yaml`, `sha256`). Their `rule_state` follows `governance.enabled`. All 304 are `false` today, so all are `disabled`.
  - Loads the runnable engine rules (`recon.rule/1`, such as `RECON-DEP-002`) into `rule_versions` (`source = 'builtin'`) with `rule_state = 'testing'`, because no CA has approved them yet.
  - Loading again with no file changes changes nothing (same row count, same `sha256` values). A changed file adds a new version; it never overwrites one.
- The rule-store files are descriptive (prose comparisons, pseudocode). They are stored and listed, but **not evaluated**. Only `recon.rule/1` rules can be evaluated.

### 3. `recon-engine run`

`recon-engine run --db <file> --input <file or folder> --tax-year 2025-26 [--profiles content/profiles] [--include-testing] [--json <file>]`

For each run, in this order:

1. **Detect.** For each file in `--input` (sorted by name), find the adapter by file type, then run every installed profile's `detect` block. Exactly one profile must match. None is `PROFILE_NO_MATCH`; more than one is `PROFILE_AMBIGUOUS`.
2. **Extract.** The adapter opens the file. The profile executor writes facts with locators to `facts`, plus one validation report per file (`document_versions.validation_report_json`, IN-14).
3. **Resolve.** Build the engagement's current view: the latest draft per document slot (`documents` + `document_versions.draft_no`). With one file there is one slot at draft 1. Uploading the same file again does not make a new draft.
4. **Snapshot.** Create a `runs` row recording the engine version, vocabulary version, the document versions used (`run_documents`, with profile ID and version) and the rule versions used (`run_rules`) (RU-53).
5. **Plan.** Load the rules for the tax year: `active` rules, plus `testing` rules only with `--include-testing`. Build the dependency index (rule → facts it reads) from `CheckType.Inputs`.
6. **Evaluate.** Run the rules in parallel on a worker pool. Sort the results by `(rule_id, subject_key)` before writing them to `evaluations` in one transaction. A missing input gives **Cannot check**, never Fail (RU-52).
7. **Render.** Print to stdout:
   - the validation reports;
   - then the findings, grouped by `review_document`, each with both figures, the difference and both source locators;
   - then a "Cannot check" section naming each missing figure and the document it should come from.

   With `--include-testing`, the first line of the report says the run includes rules not yet approved by the CA. `--json` writes the same content as canonical JSON.

**Exit codes**

| Exit code | Meaning |
|---|---|
| `0` | No findings |
| `1` | Findings present |
| `2` | Input or rule error (for example `PROFILE_NO_MATCH`, `PROFILE_AMBIGUOUS`, `UNSUPPORTED_FILE_TYPE`, `PARSE_ERROR`, a rule that fails to load, a missing flag) |
| `3` | Internal error (for example the database cannot be written) |

An exit-`2` error prints its stable code and the file name (with line and column for `PARSE_ERROR`), and still writes nothing partial: the run's transaction is rolled back.

### 4. The sample file

- **One format only.** The real sample (any company finance document) comes from Parth. Until it arrives, and to keep today's target safe, the run is proved on the existing generic tax-suite computation JSON samples (`content/profiles/generic/computation/v1/samples/`).
- When Parth's file arrives: if it is JSON, add a profile for it with two samples and expected facts (§B2.3). If it is Excel or PDF, that adapter is a separate spec (Module 4). This spec does not build it.
- To get a Pass/Fail result from one file, add **one** `recon.rule/1` rule whose two inputs are both figures in that single document, using the existing `equal` check. Give it passing and failing fixtures (CLAUDE.md invariant 6). It stays `testing` until the CA approves it.

### Out of scope

- Evaluating the 304 rule-store rules (they need converting into runnable checks rule by rule).
- Findings lifecycle and stable finding states (§B7), the server, the UI, encryption, incremental runs, and Excel/PDF/Word adapters.

## Acceptance criteria

- [ ] AC-1: `rulestore-import` writes `rules/categories.yaml`, and `rulestore-check` fails when it is missing or out of date.
- [ ] AC-2: `recon-engine load-rules` loads 304 rule-store rules (all `disabled`) and the engine rules (`testing`). A second load with no changes leaves the database unchanged.
- [ ] AC-3: `recon-engine run` on a clean sample with `--include-testing` prints the validation report and no findings, and exits `0`.
- [ ] AC-4: `recon-engine run` on a sample with a planted difference prints the finding with both figures, the difference and both locators, and exits `1`.
- [ ] AC-5: A sample with a needed figure removed gives a "Cannot check" line naming the figure, not a finding.
- [ ] AC-6: A file no profile recognises exits `2` with `PROFILE_NO_MATCH` and the file name. A damaged JSON file exits `2` with `PARSE_ERROR` and its line and column. Nothing partial is written.
- [ ] AC-7: A forced database failure (read-only DB file) exits `3`.
- [ ] AC-8: Without `--include-testing`, no `testing` or `disabled` rule is evaluated.
- [ ] AC-9: Each run stores its snapshot (engine version, vocabulary version, document versions with profile, rule versions). Evaluations are stored sorted by `(rule_id, subject_key)`.
- [ ] AC-10: Two runs on the same input, one with `GOMAXPROCS=1` and one with `GOMAXPROCS=8`, give byte-identical stdout and `--json` output. The report contains no timestamps.
- [ ] AC-11: Report text never contains a banned phrase (`rules.BannedPhrases`: no "should", no stated correct value).
- [ ] AC-12: The architecture check passes. Every new package is classified in `internal/archtest/archtest_test.go` and added to `deterministicDirs` where it applies, and `recon-engine` still links no network code.
- [ ] AC-13: The new rule has passing and failing fixtures, and `go test -count=1 ./...`, `gofmt -l .` (prints nothing) and `go vet` (host and Windows) pass.
- [ ] AC-14: Every new file is added to `docs/Tool1-File-Guide.docx`, and `tool1-technical-spec.md` is updated where the build changed the design.

## Verification

```
go run ./cmd/recon-dev rulestore-import
go run ./cmd/recon-dev rulestore-check
go run ./cmd/recon-engine load-rules --db build/recon.db --store docs/Recon_Tool_YAML_Rule_Store/recon_yaml --rules content/rules
go run ./cmd/recon-engine run --db build/recon.db --input testdata/run/clean --tax-year 2025-26 --include-testing      # exit 0
go run ./cmd/recon-engine run --db build/recon.db --input testdata/run/differs --tax-year 2025-26 --include-testing    # exit 1
go run ./cmd/recon-engine run --db build/recon.db --input testdata/run/unknown --tax-year 2025-26                      # exit 2, PROFILE_NO_MATCH
go test -count=1 ./...
go test -count=1 ./internal/archtest/ -v
gofmt -l .
go vet ./... && GOOS=windows GOARCH=amd64 go vet ./...
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
