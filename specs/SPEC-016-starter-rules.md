# SPEC-016 — Starter rule library: tie-out R1–R10

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 2 (Phase 2 gate) |
| **Depends on** | SPEC-012, SPEC-013, SPEC-014, SPEC-015 |
| **Requirements** | RU-10, RU-30, RU-43, RC-01, NF-02; requirements §6.8 and §13 Phase 2 |
| **Technical spec** | §B4, §B14; v0.2 spec §6.2 |
| **Code / content paths** | `content/rules/2025-26/tieout/**`, `content/rules/CATALOGUE.md` (generated), `testdata/packs/` |

## Goal

80–120 built-in tie-out rules exist, each with a law reference and fixtures. Together they reproduce every reference defect category R1–R10 with zero false alarms.

## Scope

- Rules for the computation, tax audit report, MAT computation, FTC form and tax credit statement. They use the generic synthetic formats until real samples arrive (Q1).
- Coverage: at least one rule per category R1–R10. Each category has its own synthetic end-to-end pack in `testdata/packs/r01`..`r10`, plus a combined `ref-full` pack and a `clean-full` pack.
- `recon-dev rule-catalogue` generates `content/rules/CATALOGUE.md`: ID, title, category, law reference, check type, fixture counts.
- CA review sheet: an export of the catalogue for the firm's CA to sign off (RU-10). The sign-off is recorded in the completion record.

## Out of scope

- GST/TDS/payroll rules (SPEC-031..033). Real tax-suite profiles (Q1).

## Acceptance criteria

- [ ] AC-1 — Between 80 and 120 rules load and pass `rule-lint`.
- [ ] AC-2 — Every rule has a law reference and at least one pass and one fail fixture, and its fixtures pass.
- [ ] AC-3 — Each pack `r01`..`r10` produces the expected finding(s) for its category and nothing else.
- [ ] AC-4 — `clean-full` produces **zero** findings (stricter than the NF-02 field target of fewer than 5).
- [ ] AC-5 — `ref-full` produces exactly the expected list of findings (golden file), i.e. zero false positives.
- [ ] AC-6 — `CATALOGUE.md` is generated and up to date (CI diff check).
- [ ] AC-7 — The firm's CA has reviewed the catalogue. Record the sign-off; the status becomes **Verified** only after it.

## Verification

```
go run ./cmd/recon-dev rule-lint ./content/rules/2025-26/...
go test ./internal/engine/ -run TestRuleFixtures -v
go test ./testdata/... -run TestPacks -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
