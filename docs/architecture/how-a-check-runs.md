# How a check runs

What the backend does, step by step, from the moment a CA uploads a file on the screen to the report they see. The worked example is the "Tax suite: computation checks" type of check with a tax suite JSON. Every other type follows the same path; only the documents and rules differ.

The overall architecture is in [architecture-overview](architecture-overview.md); the design detail is in `tool1-technical-spec.md` Part B (§B2 profiles, §B4 rules, §B6 the run). Where this document and the code disagree, the code wins and this document is corrected.

## Two programs

| Program | Job |
|---|---|
| `recon-server` (`cmd/recon-server`, `internal/api`) | The web front door. It accepts the upload, checks the request and starts the engine. It never checks a figure itself. |
| `recon-engine` (`cmd/recon-engine`, `internal/engine`) | Does the checking. It has no network code (ADR 0001), so the server runs it as a separate program and reads back its report. |

## The type of check

The category picked on the screen is a comparison type in `content/comparisons/comparisons.yaml`. Each type names the documents it needs and the rules it runs:

```yaml
- id: "taxsuite"
  name: "Tax suite: computation checks"
  documents: ["computation", "tax_audit_report"]
  rules: ["RECON-DEP-001", "RECON-DEP-002"]
```

## Step 1: the server receives the upload

`internal/api/api.go`, function `run`.

1. It makes a fresh temporary folder and saves the uploaded files there. It refuses files that are not `.json` or `.csv`, files that are too large, duplicate names and unsafe names.
2. It reads the form fields: `tax_year`, `comparison` (for example `taxsuite`), `rules:<type>` (only when the CA ticked some rules of the type), `include_testing` and `client_id`.
3. `checkRuleChoice` confirms, before anything runs, that the type exists and that every ticked rule belongs to it.
4. It starts the engine once per type of check:

   ```
   recon-engine run --input <temp folder> --tax-year 2025-26 --comparison taxsuite
                    [--rules ...] [--include-testing] [--client N] --json report.json
   ```

5. It reads back the report and the stored figures, sends them to the browser and deletes the temporary folder. Nothing of an upload outlives its request.

## Step 2: recognise the document

`internal/profile/detect.go`.

The engine never guesses what a file is. It tests the file against every format profile in `content/profiles/` (and the office's own taught profiles). The computation profile, for example, says:

```yaml
detect:
  - json_equals: { path: "$.meta.form", value: "COMPUTATION" }
  - json_equals: { path: "$.meta.schema_version", value: "1.0" }
```

| Result | Outcome |
|---|---|
| Exactly one profile matches | The file is that document, for example `computation`. |
| No profile matches | `PROFILE_NO_MATCH`. The layout can be taught on the mapping screen. |
| Several profiles of one bundle match | The file holds several documents (a filing pack); each profile reads its own section. |
| Several other profiles match | `PROFILE_AMBIGUOUS`, naming every match. |

## Step 3: extract the figures

`internal/profile/extract.go`.

The same profile says where each figure is:

```yaml
- fact: "computation.allowed_depreciation"
  json_path: "$.business_income.allowed_depreciation"
  unit: "rupees"
  required: true
```

Each figure becomes a **fact**: a vocabulary name, a value, a period and a locator (file name and JSON path). Money is converted to integer paise, never floating point. A required figure that is absent is a validation error. If the file's own tax year is not the tax year of the run, the file is refused and none of its figures are used.

## Step 4: store the documents

`cmd/recon-engine/run.go`, step 3 in the code.

Each document is stored in SQLite as a draft of its slot (document type and period). For a client's run, each file's PAN must be the client's PAN. The engine then takes the **current view**: the latest version of every document, including documents uploaded for the same client in earlier runs.

## Step 5: plan which rules run

`cmd/recon-engine/run.go`, step 5 in the code.

A stored rule runs only when all of these hold:

| Condition | Why it matters |
|---|---|
| Its status is Active, or Testing with "include testing" ticked | A new rule loads as Testing until the CA approves it. |
| It is one of the chosen type's rules | Picking "Tax suite" runs only RECON-DEP-001 and RECON-DEP-002. |
| It applies to the tax year of the run | Both DEP rules say `tax_years: ["2025-26"]`; for 2026-27 neither runs. |
| It was ticked, when the CA ticked rules | A rule left unticked is reported as "not selected", never counted as checked. |

The run is then recorded with the documents and rules it used (the snapshot).

## Step 6: evaluate

`internal/engine/engine.go` and the check types in `internal/engine/checks/`.

Both tax suite rules use the `equal` check type with a tolerance of `{ paise: 100 }` (one rupee):

| Rule | Input a | Input b |
|---|---|---|
| RECON-DEP-001 | `computation.allowed_depreciation` | `computation.fixed_asset_schedule.total` (same file) |
| RECON-DEP-002 | `tar.depreciation_schedule.total` (tax audit report) | `computation.allowed_depreciation` |

For each rule the engine:

1. Looks up each input figure by name, period and scope.
2. If a figure is not there, gives **Cannot check** and names the missing figure. A missing figure is never treated as zero or as a mismatch.
3. If the same figure is found in two documents with different values, stops with a conflict error. It never picks one.
4. Otherwise the check type decides. For `equal` (`internal/engine/checks/equal/equal.go`) it computes |a − b| in paise: within the tolerance is **Pass**, over it is **Fail**, and a Fail is a finding.

Rules are evaluated in parallel, but the results are sorted by rule and subject, so the same input always gives the same output.

**Uploading only the computation JSON:** RECON-DEP-001 gives Pass or Fail, because both its figures are in that file. RECON-DEP-002 gives Cannot check, naming `tar.depreciation_schedule.total`, until the tax audit report is uploaded too (or a filing-pack bundle that contains it).

## Step 7: the report

`internal/findings`.

The evaluations are stored, the transaction is committed, and the report is written. Each finding shows both values, the difference and where each value came from (file and JSON path). It never says what the figure should be. The screen groups findings by the document that needs fixing (`review_document` in the rule).

The engine's exit code tells the server how the run ended: 0 no findings, 1 findings, 2 a problem with the input or a rule, 3 an internal error.

## In one line

Upload → server saves the file and starts the engine → the profile recognises the document → figures are extracted in paise → documents are stored → the type's rules are chosen by status, tax year and the CA's ticks → each rule compares its figures → Pass, Fail or Cannot check → the report.
