# SPEC-029 — Record matching and date, threshold, period checks

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 5 |
| **Depends on** | SPEC-013, SPEC-020 |
| **Requirements** | RC-03, RC-04, RC-05, RC-01 (R6), requirements §6.2 check types |
| **Technical spec** | §A9, §B4.3 |
| **Code / content paths** | `internal/engine/checks/{match_records,threshold,within_due_date,all_periods_present}` |

## Goal

The engine can compare two sets of rows (invoices, challans, ECR lines) by key, test values against law-table limits and due dates, and detect missing periods, all fast enough for 200,000 rows.

## Scope

| Check type | Behaviour |
|---|---|
| `match_records` | Sorted-merge join of record types A and B on `match_key`, with optional key normalisation declared in the vocabulary (trim, upper-case, strip leading zeros). Reports rows only in A, rows only in B, and rows in both whose amounts differ beyond tolerance. One evaluation per key, so each gets its own stable finding ID |
| `threshold` | Value vs a law-table limit, for the period's as-of date |
| `within_due_date` | Event date vs the law-table due date for the period |
| `all_periods_present` | Expected periods (months/quarters of the FY, or a law-table list) vs the periods that have data (R6) |

- Optional aggregation before matching (`group_by` fields, e.g. supplier GSTIN + month), declared in params.
- Templates show the key, both sides' amounts for each amount field, the differences and the locators. A row only in A shows A's values and "no matching row in B".

## Out of scope

- Fuzzy matching. Any non-exact key match must come from a declared normalisation, never a similarity score.

## Acceptance criteria

- [ ] AC-1 — A synthetic 200,000 × 200,000 invoice match completes in under 30 s and gives the expected counts of only-in-A, only-in-B and amounts-differ.
- [ ] AC-2 — Key normalisation (e.g. `INV-042` vs `inv-042 `) is applied only when declared.
- [ ] AC-3 — Each check has pass/fail fixtures and boundary tests (due date on the day = pass; one day after = fail).
- [ ] AC-4 — `all_periods_present` R6 fixture: 11 of 12 months present → one finding naming the missing month.
- [ ] AC-5 — Results are identical regardless of input row order.

## Verification

```
go test ./internal/engine/checks/... -v
go test ./internal/engine/checks/match_records/ -bench . -run ^$
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
