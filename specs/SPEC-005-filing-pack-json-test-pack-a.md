# SPEC-005 — Tax-suite filing pack JSON: several documents in one file, and Test Pack A, Pass 1

| | |
|---|---|
| **Status** | Done |
| **Phase** | 1 |
| **Depends on** | SPEC-004 |
| **Requirements** | IN-10..14, IN-20..23, FV-01, RU-01, RU-02, NF-01, NF-03 |
| **Technical spec** | §B2.2, §B2.3, §B3, §B4.2, §B4.3, §B14 |
| **Code / content paths** | `internal/profile`, `cmd/recon-engine` (`run`, `inspect`), `content/profiles/taxsuite_pack/`, `content/vocabulary/`, `content/rules/2025-26/tieout/RECON-MAT-*`, `testdata/testpack_nimbus/`, `web/src/screens/ClientFile.tsx` |

## Goal

The tool reads the tax suite's structured filing pack (`09_filing_pack_structured.json` of Test Pack A, the fictional Nimbus Logic Systems Private Limited, AY 2026-27 / tax year 2025-26), in which one JSON file holds several documents. It finds the planted defects that today's check type (`equal`) can reach, and it raises nothing on the pack's negative controls. A scoring test measures the result against the pack's answer key, so later specs can raise the score one check type at a time.

## Background: the first test of Test Pack A (16 October 2026)

Pass 1 (JSON only) was run on the pack on 16 October 2026. The file was refused with `PROFILE_NO_MATCH`, giving **0 of 30 findings** (critical 0 of 4). Four reasons:

1. No format profile knows this layout.
2. The file holds six documents (tax audit report 3CD, Form 29B, Form 67, computation, Form 26AS, financial statements). One file was one document, and a file matching two profiles was refused.
3. The engine has two check types (`equal`, `match_records`); most findings need others (`sum_equals`, `gte`, `formula`, `all_periods_present`, `field_present`, `domain`).
4. Some findings cannot come from the JSON at all: F11 and F12 need two drafts of the audit report, F25 and F26 need fields the JSON does not carry, and F27 to F30 need the two ledger CSVs. The best Pass 1 can ever reach is 22 of 30 (F11, F12, F25, F26 and F27 to F30 need data the JSON does not carry).

This spec fixes 1 and 2 and adds the rules `equal` can already run. Point 3 is planned below as later specs.

## Scope

### 1. Several documents in one file (a bundle)

- A format profile may name a `bundle` (an ID shared by the profiles of one file layout) and a `section` (the JSON path of its part of the file, for example `$.form_3cd`). Each profile of a bundle is an ordinary profile: one document type, its own context, facts and records, its own samples and expected results.
- Detection: when every profile that matches a file names the same bundle, the file is read once with each of them and gives one document per profile. Any other double match (two bundles, or a bundle and a profile without one) is still `PROFILE_AMBIGUOUS`: the tool never guesses.
- Each document of the file is stored as its own draft of its own slot (document type, period, scope, part) and has its own validation report. The file name is the same in each.
- What was not read is listed once, not once per document: a value inside a profile's `section` that nothing read is listed in that document's report; a value outside every section that no profile read is listed in the first document's report (by document type), with the reason "not part of any document this file holds".
- `recon-engine inspect` names every profile of the bundle in `recognised_as`.

### 2. The tax-suite filing pack format (four documents for now)

New profiles under `content/profiles/taxsuite_pack/`, bundle `taxsuite.filing_pack`, detected by the pack's top-level parts:

| Document | Section | Figures read |
|---|---|---|
| Tax audit report (3CD) | `$.form_3cd` | `tar.depreciation_schedule.total` (clause 18 total depreciation allowable) |
| Form 29B (MAT report), new document type `mat_report` | `$.form_29B` | `mat.book_profit` (Sl.18), `mat.tax_payable` (Sl.20) |
| Computation | `$.computation` | net profit as per P&L, depreciation as per books, allowed depreciation, fixed-asset schedule total, business income (PGBP), gross total income, total income, TDS, advance tax, and the new `computation.mat.book_profit` and `computation.mat.tax_payable` (115JB block) |
| Financial statements | `$.financial_statements` | revenue from operations, profit before tax, profit after tax, depreciation |

- Context of every document: tax year from `$.assessee.previous_year` (the pack's AY 2026-27 is tax year 2025-26), PAN from `$.assessee.pan`, period the whole year.
- Form 67 and Form 26AS are not read yet (no rule needs them); they are listed as not read, with the reason.
- New figure names go into `content/vocabulary/facts.yaml` with a vocabulary version bump and a regenerated `docs/fact-vocabulary.md`.
- Samples: the pack's JSON as delivered, and a corrected copy (`nimbus_pack_corrected.json`: allowed depreciation 1,35,862; Form 29B Sl.12 73,26,872, Sl.18 3,80,92,990, Sl.19 57,13,949 and Sl.20 63,58,482), each with hand-checked expected figures.

### 3. Rules for F03

- `RECON-MAT-001`: book profit in Form 29B (Sl.18) against book profit in the computation's 115JB block.
- `RECON-MAT-002`: tax payable under section 115JB in Form 29B (Sl.20) against the computation's 115JB block.
- Both `equal` with a Rs 1 tolerance, two pass and two fail fixtures each, loaded as testing.
- F01 is already covered by `RECON-DEP-001` (computation, allowed depreciation against its fixed-asset schedule) and `RECON-DEP-002` (tax audit report against the computation).

### 4. Test Pack A, Pass 1, as a standing test

- The pack's JSON, answer key, negative controls and README are copied into `testdata/testpack_nimbus/`.
- `testdata/testpack_nimbus/score.yaml` maps each answer-key finding to the rules that detect it (empty when no rule does yet), and each negative control to the rules that touch its figures.
- A Go test runs Pass 1 on the pack and on the corrected copy and reports the score: findings detected out of 30, critical out of 4, and false positives. It fails if a mapped finding is missed, if any rule fires that is not mapped to a finding, or if the corrected copy gives any finding.

### 5. Screens

- A bundle file shows one row per document in the Documents table ("Recognised as" names each document), and each row opens its own figures, rows and "Not read, and why".

## Out of scope (planned next, one spec each)

- **SPEC-006, arithmetic checks:** `sum_equals` (F02, F18), `gte` (F07, F09, F13), `formula` with rates (F04, F08), and the Form 67 and 26AS documents they need.
- **SPEC-007, completeness checks:** `all_periods_present` (F14, F15), `field_present` (F16, F21, F24).
- **SPEC-010, coded fields** (first planned as SPEC-008, then SPEC-009; those numbers went to Excel files and comparison history): `domain` and law tables (F06 DTAA rates, F17, F22), and coded-field combinations for F05, F10, F19, F20. These are reported only as "these two coded values are present together", never as a judgment (CLAUDE.md invariant 1), with wording approved by the CA. F23 (43B(b) equal to the March employee share) needs a rule design decision first.
- Pass 2 (the PDFs), Pass 3 (version diff of the two 3CD drafts: F11, F12) and the ledger CSVs with new-head detection (F27 to F30).

## Design notes

- A bundle keeps "one profile, one document type". Sharing a file is the only new idea, so profile tests, expected files and the vocabulary checks need no other change. A profile with `section` reads only inside it: every fact path and row path must start with the section.
- The tax year: the pack states AY 2026-27 and previous year 2025-26. The tool's tax year is the financial year, as in the Income-tax Act, 2025, so the profiles read `previous_year`.
- The score is measured, not asserted to rise: a missed mapped finding or an unmapped finding fails the test; a finding with no rule yet is reported as not yet covered.

## Acceptance criteria

- [x] AC-1 — A file matching the profiles of one bundle is read once per profile, each as its own document with its own validation report; a file matching two bundles, or a bundle and another profile, is `PROFILE_AMBIGUOUS`. A value not read is listed once, in the right document's report. Go tests cover these cases.
- [x] AC-2 — The four filing-pack profiles each have two samples with hand-checked expected figures, and `go run ./cmd/recon-dev profile-test` passes for all of them.
- [x] AC-3 — `RECON-MAT-001` and `RECON-MAT-002` pass `rule-lint` with two pass and two fail fixtures each.
- [x] AC-4 — Pass 1 on the pack finds F01 (`RECON-DEP-001` and `RECON-DEP-002`, Rs 2,448 each) and F03 (`RECON-MAT-001` Rs 9,68,390 and `RECON-MAT-002` Rs 1,61,644), each with both values, the difference and both sources; `RECON-PL-001` and `RECON-PL-002` pass; no other rule fails. The test reports the score: 2 of 30, critical 2 of 4, 0 false positives.
- [x] AC-5 — The corrected copy gives no finding.
- [x] AC-6 — No negative control (N01 to N16) is flagged.
- [x] AC-7 — The Client file shows each document of the pack as its own row, and each opens its own figures and "Not read, and why". Vitest covers this.
- [x] AC-8 — `go test -count=1 ./...`, `gofmt -l .`, `go vet` (host and Windows), `npm run lint`, `npm test` and the architecture guard pass; the file guide, the technical spec (§B2.2) and CLAUDE.md are updated.

## Verification

```
go run ./cmd/recon-dev profile-test
go run ./cmd/recon-dev rule-lint
go test -count=1 ./cmd/recon-engine -run TestTestPackAPass1 -v
go run ./cmd/recon-engine load-rules --db build/nimbus.db --store docs/Recon_Tool_YAML_Rule_Store/recon_yaml --rules content/rules
go run ./cmd/recon-engine run --db build/nimbus.db --input testdata/testpack_nimbus/09_filing_pack_structured.json --tax-year 2025-26 --include-testing
go test -count=1 ./...
cd web && npm run lint && npm test && cd ..
```

## Build notes (16 October 2026)

- **Part 1:** `bundle` and `section` in the profile format; `profile.DetectAll` and `profile.ExtractAll` (`internal/profile/bundle.go`); every validation report names its `document_type`. `run` reads a bundle file as one document per profile; `inspect` names every profile of the bundle; the profile tests accept a bundle sample for each of its profiles. The text report says "1 file (4 documents) read". Unit tests: `internal/profile/bundle_test.go` (one bundle read once per document, two bundles or a bundle and another profile refused, section enforced, unread values listed once).
- **Part 2:** vocabulary 5: document type `mat_report` (prefix `mat`), `mat.book_profit`, `mat.tax_payable`, `computation.mat.book_profit`, `computation.mat.tax_payable`. Four profiles under `content/profiles/taxsuite_pack/`, each with the pack as delivered and a corrected copy (F01 to F04 fixed, with their knock-on figures), expected figures generated from the JSON and cross-checked against the README and answer key. `profile-test`: 26 samples pass.
- **Part 3:** `RECON-MAT-001` and `RECON-MAT-002` (`content/rules/2025-26/tieout/`), two pass and two fail fixtures each. A new comparison type, "Tax suite filing pack" (`taxsuite_filing_pack`), runs the six rules that read the pack's documents, so a pack run has no unrelated Cannot check.
- **Part 4:** `testdata/testpack_nimbus/` (pack JSON, answer key, negative controls, README) and `score.yaml`. `TestTestPackAPass1` result: **2 of 30 findings (22 reachable from the JSON), critical 2 of 4, 0 false positives**: F01 by RECON-DEP-001 and RECON-DEP-002 (Rs 2,448 each), F03 by RECON-MAT-001 (Rs 9,68,390) and RECON-MAT-002 (Rs 1,61,644); RECON-PL-001 and RECON-PL-002 pass; the corrected copy gives no finding.
- **Part 5:** the Documents table shows one row per document of a file ("09_filing_pack_structured.json, tax audit report"), each opening its own figures and "Not read, and why". A sample client, "Nimbus Logic Systems Private Limited (Test Pack A)", shows the pack (`testdata/run/filing-pack`). Vitest: `web/src/spec005.test.tsx` (65 web tests).
- Everything runs offline: the pack, profiles, rules and scoring are local files; nothing is fetched.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| 16 October 2026 | Unnati (with Claude Code) | not committed yet | All 8 criteria met. Test Pack A Pass 1: 2 of 30 (critical 2 of 4), 0 false positives; next, SPEC-006 (sum_equals, gte, formula). |
