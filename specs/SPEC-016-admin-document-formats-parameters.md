# SPEC-016 — Admin: document formats, mapping screen and parameter management

| | |
|---|---|
| **Status** | In progress |
| **Phase** | 6 |
| **Depends on** | SPEC-012 (Admin tab, roles, audit log), SPEC-010 (Rule Builder and rule versions), SPEC-011 (JSON flattened to `Path` / `Value`), SPEC-008 (Excel read by header text), SPEC-009 (comparison history) |
| **Requirements** | IN-12 (layouts taught on a mapping screen), RU-10 to RU-12, RU-41 to RU-44 (versions, audit, disable not delete), NF-04 (roles) |
| **Technical spec** | §B5 (profiles), §B9 (Rule Builder, audit), §B12.1 (role matrix), §B11 (each handler declares a permission), §B3 (`audit_log`) |
| **Code / content paths** | `internal/profile` (mapping, versions, dry run), `content/vocabulary/facts.yaml` and `records.yaml` (parameters, aliases), `content/comparisons/comparisons.yaml`, `internal/store` (format and parameter versions, migration), `internal/auth/roles.go` (new permission), `cmd/recon-engine` (`format-list`, `format-save`, `format-disable`, `format-enable`, `format-versions`, `format-test`, `format-export`, `format-import`, `parameter-add`, `parameter-list`), `internal/api` (format and parameter endpoints), `web/src/screens/Formats.tsx` (new), `web/src/screens/Mapping.tsx` (new), `web/src/screens/Parameters.tsx` (new), `web/src/api.ts`, `web/src/types.ts` |

## Goal

An admin can add a new kind of JSON, Excel (`.xlsx`) or CSV document, map each of its figures to a named parameter the engine knows, add a new parameter when none fits, and see at once which rules can run on the result. Nobody edits YAML or code. Every change is versioned, written to the audit log and can be rolled back. Everything works offline.

Most of the parts exist already: the mapping screen and office profiles (SPEC-004, SPEC-008 part 2), the vocabulary files, the Rules tab and Rule Builder, and the Admin tab. This spec joins them into one place and adds what is missing.

## Scope

### 1. Document formats library (new Admin section)

- A list of the formats the tool can read: name, file type (JSON / `.xlsx` / `.csv`), source system (Tally, tax suite, GSTR-1, 26AS and so on), version, status (testing / active / disabled), mapping coverage ("14 of 16 required parameters mapped") and the date it was last used.
- Built-in formats are shown read only. An office format is created from a sample file, or by **Duplicate** of an existing one.
- **Disable** and **Enable again** need a reason. A format is never deleted (RU-41, RU-44). A disabled format is not offered when a file is recognised.
- **Versions.** A change to a format saves a new version. Earlier versions are kept and listed. A comparison keeps the format version it was read with (SPEC-009). The version in use can be switched back to an earlier one.

### 2. Mapping screen

Teaching a format from a sample file, as a guided screen:

- **Left side: the sample.** Excel shows sheet tabs and a grid. JSON and flattened files show the path tree (`Path` / `Value`, SPEC-011). CSV shows its columns.
- **Right side: the parameters**, from the vocabulary, each with its name, type (amount, text, date, percent, list) and a "required" mark.
- **Ways to map** a figure to a parameter:
  - click a column header or a JSON node, then click the parameter;
  - by header text, matched ignoring capitals (SPEC-008);
  - a fixed cell or path for a single figure;
  - for a list, "the entry that meets these conditions" (for example `section = 194C`), never by position (SPEC-006).
- **Table settings:** the header row, the sheet name or "any sheet", rows to skip, total-row handling, and the column that identifies a record.
- **Value settings per field:** unit (rupees, paise, lakhs, crores), negative style (minus sign, brackets, Dr / Cr), date format, the source of the tax year, GSTIN and PAN checks. An amount a document never holds is "cannot check", not zero (SPEC-008 part 2). Money is converted to integer paise and never passes through floating point.
- **Live preview:** the first 20 rows as the engine would read them, with each value's source (cell or line) beside it, and rows that fail marked.
- **Dry run on save:** which rules can run on this format, and which are "cannot check" because a parameter they need is not mapped. A format with a required parameter unmapped can be saved as **testing**, not as **active**.

### 3. Parameter management

- **Add a parameter:** key, display name, type, the document it belongs to and a description. The key must be unique and follows the existing naming style. No parameter is typed free-hand inside a rule or mapping; it is always picked from this list.
- **Aliases:** other labels that mean the same parameter ("Total Sales", "Gross Turnover"). The mapping screen uses them to suggest matches on the next upload; a suggestion is never applied without the admin's click.
- **Where used:** for each parameter, the rules, comparisons and formats that use it. Disabling a parameter shows what would stop working, and is refused while an active rule needs it.
- **Gap report:** parameters that no active format supplies, and rules whose inputs no active format covers. This adds to the rule problem list from SPEC-004.
- Office parameters are written to an office folder (given by `--office-vocabulary`, like `--office-profiles` and `--office-rules`), under `build/` and git-ignored. Built-in vocabulary files are never edited.

### 4. Comparisons

- An admin can define a comparison type (for example "Sales register vs GSTR-1") by choosing the two documents and which formats can fill each side. This is stored in the office folder; `content/comparisons/comparisons.yaml` is not edited.
- New rules for these parameters are made in the Rule Builder (SPEC-010). This spec links to it from a parameter's "Where used" view ("Add a rule that uses this parameter") but does not change the Rule Builder.

### 5. Governance

- A new permission `manage_formats`, held by the Partner (and by the Manager if the open question below is answered yes). Every handler declares it (§B11). Staff and read-only users can use formats but not change them.
- **Draft, test, publish.** An edit is saved as a draft. Publishing re-reads every saved sample file for that format and shows any figure that changed (`format-test`). A publish that changes a saved sample's result needs the admin's explicit confirmation and a reason.
- **Audit.** Each format and parameter create, edit, disable, enable, publish and rollback writes one `audit_log` entry: action, object and version, reason, user, time. The time never reaches the engine, so runs stay the same.
- **Sample files.** A sample uploaded for teaching is kept only if the admin chooses to keep it, with a warning that it must be synthetic or masked of PANs, GSTINs and names, because the database is not yet encrypted (SPEC-034). No real client data goes into the repo.
- **Unrecognised files.** Files that no format matched are listed for the admin, with a **Teach this format** button that opens the mapping screen on that file.

### 6. Export and import

- A format, with its parameters and aliases, can be exported to one file and imported on another office machine with no network. An import is checked by the same strict loader, shown to the admin as a list of what it adds or changes, and applied only on confirmation. Signing of bundles with Ed25519 is left to the rule-pack work.

## Out of scope

- Editing built-in formats, parameters or rules (an office copy is made instead).
- A new check type, or changes to how a rule is built (SPEC-010).
- PDF layouts and Poppler extraction.
- Starlark rules.
- Encrypting the database and the key handling for it (SPEC-034).
- Signing format bundles.
- Any model-suggested mapping. A suggestion from header text or aliases is allowed; a model never maps, creates or changes a figure (invariant 2).

## Design notes

- **Decided 3 November 2026: one model, not two.** A *format* is a profile (`internal/profile`, `content/profiles`, `recon.profile`) and a *parameter* is a fact or record type of the vocabulary (`content/vocabulary`). There is no separate "field mapping" system: the mapping screen is the existing teach flow (`inspect`, `profile-create`, `LayoutMapping`), and it writes versions to the new tables. Aliases and office parameters extend the vocabulary files' model; profiles may still only write names the vocabulary holds (FV-03). The words "required figures", "coverage" and the dry run are worked out from the profile's `required` figures and from the rules' inputs.
- **Build order:** (1) database and standard content stored by `load-rules` (done: migration 0011, `internal/store/formats.go`, `cmd/recon-engine/formatseed.go`); (2) engine commands to list, save, switch, disable and add; (3) API endpoints and the `manage_formats` permission; (4) the screens on the teach flow.
- **Built so far (3 November 2026):** steps 1 to 4 in a first form. Database: migration 0011, `internal/store/formats.go`. Engine: `load-rules --profiles` stores the standard formats and parameters; `formats`, `format-disable`, `format-enable`, `format-switch`, `parameter-add`, `parameter-disable`, `parameter-enable`, `parameter-alias`; `profile-create --db --by` also stores a taught layout as an office format. API: eight endpoints behind the new `manage_formats` permission (Partner). Screen: `web/src/screens/Formats.tsx` (Formats, Parameters and Gaps tabs, popups, "Add a format" through the existing mapping screen).
- **Edit, publish and dry run (built 3 November 2026):** migration 0012 adds `mapping_json`, `profile_version`, `published_at` and `published_by` to `format_versions` (a version is still never changed; publishing is recorded once). `profile-create --format <id> --db --by` saves an edit as a **draft** version (the layout's next profile version, superseding the one before it) and writes no file; `format-get` reads the mapping to edit; `format-publish` writes the profile files, checks the engine loads them, and makes the draft the version in use as testing or active. A run reads an office format only when it is active (or testing, with `--include-testing`) and only the version in use, so a draft, a switched-off format and an older version are never read (`usableProfiles`). A run records the stored version it read with (`profile_version_id` on `document_versions` and `run_documents`). The preview of a layout and the publish answer carry a dry run: the rules that read the layout's document, which can run and which cannot check, with the figure each lacks. Screen: Edit the mapping (choose a sample, then the existing mapping screen opens on the stored mapping), Draft and Publish in the versions table, Publish as testing or active.
- **Also built (3 November 2026):** files no format recognised (migration 0013; a run that finds no profile notes the file's name, size and hash; the Unrecognised files tab teaches a format for it or dismisses it with a reason); kept samples (`profile-create --keep-sample`, Partner only) that `format-publish` reads again with the version in use and with the draft, refusing a publish that reads any differently until it is confirmed with a reason; export and import of an office format with its parameters as one bundle (`format-export`, `format-import --check-only`); amounts in lakhs and crores and Dr or Cr after an amount (`internal/money`, `sign: "drcr"`); CSV layouts; a preview of 20 rows; the list's coverage ("figures the rules need") and last-used date; `docs/Tool1-File-Guide.docx` and `tool1-technical-spec.md` §B12.1.
- **Still open, so this spec is not Done:** AC-5 (a row that fails is not marked in the preview table: a layout whose rows fail is refused, naming the place); AC-7 (the mapping screen's suggestions come from the usual names of a column, not from the aliases of an office parameter, which a profile cannot write yet because the vocabulary loader does not read office parameters); AC-8 (the refusal to disable a parameter that an active rule reads is built, but no rule can read an office parameter yet, so no test reaches it); AC-18 (Test Pack A Pass 1 still scores 22 of 30, but `TestExcelGivesTheSameFindings` fails, as it did before this spec). The spec's verification also needs `go test ./...` to pass, and three groups of tests fail for reasons outside this spec (the vocabulary document and the screens' sample data are out of date after the vocabulary moved to version 10, and the Excel and JSON totals differ since SPEC-015), and two tests in `spec013.test.tsx` fail in the web suite.
- Migration 0011 stores a parameter's type as the vocabulary kind (`money`, `int`, `text`, `date`, `bool`, `rate`, `record`), not the simpler list used in the first sketch of this spec.

- The mapping screen produces the same office profile files `recon-engine profile-create` writes today. The command and the screen use one code path, so a format taught on the screen can also be created from the command line.
- Aliases and office parameters are data in YAML files, read by the same strict loader as the built-in vocabulary, so a bad file is refused with the file and the line named.
- A parameter's type decides how a value is read (`internal/profile`): money to paise, dates to ISO dates, percents to basis points. No floating point.
- The sort order of formats, parameters, aliases and dry-run results is fixed (by key), so the same input gives the same output (invariant 4).
- If a format version changes a figure's meaning, older comparisons still open read only exactly as they were reported (SPEC-009).
- This spec changes `tool1-technical-spec.md` §B5 and §B12.1 (new permission); update them in the same commit.

## Open questions

1. Does the Manager also get `manage_formats`, or only the Partner? (Recommended: the Partner only at first, as for rule approval.)
2. Should a new office parameter need Partner approval before a rule can use it, as a rule does (testing → active)?
3. How long are kept sample files held, and may the admin delete one? (Recommended: yes, an admin may remove a kept sample; the audit entry stays.)

## Acceptance criteria

- [x] AC-1 — The Admin area lists formats with name, file type, source, version, status, coverage and last-used date; built-in formats are read only.
- [x] AC-2 — An admin uploads a sample `.xlsx`, a JSON file and a CSV file and teaches each as a new office format on the mapping screen, with no YAML typed. The result is the same profile file `profile-create` writes for the same mapping.
- [x] AC-3 — Mapping works by clicking a column or node, by header text (capitals ignored), by a fixed cell or path, and by "entry that meets conditions" for a list.
- [x] AC-4 — Value settings (unit, negative style, date format, tax-year source, GSTIN / PAN check) are applied; every money amount is integer paise; an amount a document never holds is "cannot check", not zero.
- [ ] AC-5 — The live preview shows the first 20 rows as the engine reads them, with each value's source cell or line; rows that fail are marked.
- [x] AC-6 — The dry run lists which rules can run and which are "cannot check"; a format with a required parameter unmapped cannot be saved as active.
- [ ] AC-7 — An admin adds a parameter (key, name, type, document, description) and aliases. A duplicate key is refused. The mapping screen suggests a match from an alias but applies it only on click.
- [ ] AC-8 — Each parameter shows where it is used; disabling a parameter that an active rule needs is refused, naming the rule.
- [x] AC-9 — The gap report lists parameters no active format supplies and rules whose inputs are not covered.
- [x] AC-10 — Every change to a format or parameter saves a new version; earlier versions are listed and one can be switched back to. Nothing is deleted. Disable and Enable again need a reason.
- [x] AC-11 — Publishing a format re-reads its kept samples and shows any changed figure; a changed result needs confirmation and a reason.
- [x] AC-12 — A comparison keeps the format version it was read with, and an earlier comparison still opens read only exactly as reported (SPEC-009).
- [x] AC-13 — The permission `manage_formats` guards every format and parameter endpoint; a Staff and a Read-only user get "not allowed" on each, and the UI hides the actions from them (checked in `internal/api` and `web` tests).
- [x] AC-14 — Each create, edit, disable, enable, publish and rollback writes one audit entry with action, object, version, reason, user and time.
- [x] AC-15 — A file no format recognises appears in the unrecognised-files list with a **Teach this format** button that opens the mapping screen on it.
- [x] AC-16 — A format bundle exports to a file and imports on a second database; the import shows what it adds or changes and applies only on confirmation; a malformed bundle is refused naming the file and line.
- [x] AC-17 — The same input gives the same output twice (formats, parameters, dry-run and preview in a fixed order); built-in content files are never changed by any action.
- [ ] AC-18 — The Test Pack A Pass 1 score is unchanged (22 of 30, critical 4 of 4, 0 false positives), and the Excel and CSV samples still give the same findings and Worklist.
- [x] AC-19 — The architecture guard still passes (no network code; each new package is classified), and the screens load nothing from the internet.
- [x] Every file this spec created is added to `docs/Tool1-File-Guide.docx` in plain language.

## Verification

- `gofmt -l .` prints nothing; `go vet ./...` and `GOOS=windows GOARCH=amd64 go vet ./...` pass.
- `go test -count=1 ./...` passes, including `internal/archtest`.
- `go test -count=1 ./cmd/recon-engine -run TestTestPackAPass1 -v` still scores 22 of 30.
- `go test ./cmd/recon-engine -run TestExcelSamples` passes with no `-update`.
- Tests for: mapping by each of the four ways (AC-3); value settings and paise (AC-4); dry-run and coverage (AC-6); duplicate and in-use parameters (AC-7, AC-8); versions and rollback (AC-10); sample re-read on publish (AC-11); role refusals (AC-13); audit entries (AC-14); export then import on a second database (AC-16).
- `cd web && npm test && npm run lint && npm run build` pass.
- Manual: open the Admin area, teach a new `.xlsx` format from a synthetic sample, add a parameter with an alias, run the dry run, publish it, upload a file in that format on a client and see the comparison, then disable the format and check that the file is no longer recognised.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| 2026-11-03 | Claude Code, with the Partner | not committed | Not complete: AC-5, AC-7, AC-8 and AC-18 are open and the verification does not yet pass (see Design notes). Status stays In progress. |
