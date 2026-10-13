# SPEC-033 — Payroll reconciliation module

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 5 (Phase 5 gate, together with SPEC-031 and SPEC-032) |
| **Depends on** | SPEC-029, SPEC-030 |
| **Requirements** | RC-05, IN-06 |
| **Technical spec** | §A6.4, §B8 (`payroll`); v0.2 spec §8 |
| **Code / content paths** | `internal/modules/payroll`, `content/profiles/epfo/**`, `content/profiles/esic/**`, `content/rules/<tax-year>/payroll/**` |

## Goal

The salary register, EPF ECR, ESIC contributions and salary TDS agree month by month, contributions are paid by their due dates, and no month is missing.

## Scope

- Profiles for EPF ECR and ESIC contribution files and for the salary register (Excel/Tally), using synthetic samples until real ones arrive.
- Vocabulary: `salary_row`, `ecr_row` and `esic_row` record types, plus monthly headcount and gross facts.
- Rules:
  - headcount and gross pay per month across sources;
  - employee vs employer share totals;
  - payment date vs due date (from a law table);
  - missing months (`all_periods_present`, R6);
  - a wage-definition check, written by the CA as a Starlark rule whose params the CA sets (requirements RC-05 "CA rule").

## Acceptance criteria

- [ ] AC-1 — A synthetic payroll pack with a missing month, a late payment and a gross mismatch produces exactly those three findings, and the clean pack produces none.
- [ ] AC-2 — Findings show the month, both sources' values and the difference.
- [ ] AC-3 — The wage-definition rule's params are editable in the Rule Builder, and changing them changes the result.
- [ ] AC-4 — Profile tests pass for every payroll profile.
- [ ] AC-5 — Phase 5 gate: gaps are found on the GST, TDS and payroll test files (recorded).

## Verification

```
go test ./internal/modules/payroll/ -v
go test ./testdata/... -run TestPacks/payroll -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
