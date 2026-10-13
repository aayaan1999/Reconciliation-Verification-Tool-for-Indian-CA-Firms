# SPEC-002 — Money and period primitives

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 1 |
| **Depends on** | SPEC-001 |
| **Requirements** | NF-01, NF-03, FV-01, RU-33 |
| **Technical spec** | §A5, §A8 |
| **Code / content paths** | `internal/money`, `internal/period` |

## Goal

All money arithmetic in Tool 1 goes through one integer type with explicit rounding, and all periods are written in one canonical form.

## Scope

**`internal/money`**

- `type Paise int64`.
- `Parse(text string, unit Unit) (Paise, error)`: string-based, never through `float64`. Accepts plain (`112450.50`), Indian-grouped (`1,12,450.50`) and Western-grouped (`112,450.50`) text, a leading minus, and parentheses for negatives. Rejects more than two decimal places unless a `Rounding` is given. `Unit` is `Rupees` or `Paise`.
- `ParseJSONNumber(json.Number, Unit)`: same rules, for JSON input.
- `Add`, `Sub`, `Neg`, `Abs`, `Sum([]Paise)`: return an error on overflow, never wrap.
- `MulRate(p Paise, bp int64, mode Rounding)` with rates in basis points (1% = 100).
- `MulRatio(p Paise, num, den int64, mode Rounding)`.
- `Rounding` modes: `None` (error if a remainder exists), `HalfUpPaisa`, `HalfUpRupee`, `Nearest10Rupees`, `Down`, `Up`.
- `FormatIndian(p Paise, showPaise bool) string`: `1,12,450` / `1,12,450.50` / `-2,750`.

**`internal/period`**

- Canonical strings: tax year `"2025-26"`; FY period `"FY2025-26"`; quarter `"FY2025-26:Q1"` (Q1 = Apr–Jun); month `"2025-04"`.
- `Parse`, `Validate`, `FYOfDate(date)`, `MonthsOf(fy)`, `QuartersOf(fy)`, `QuarterOfMonth`, `Contains(outer, inner)`, `PriorFY`.
- Dates are ISO `YYYY-MM-DD` strings or a small `Date` type; no `time.Now` anywhere.

## Out of scope

- Which regime applies to a tax year: that is law-table data (SPEC-013), not code.
- Locale formatting in the UI (SPEC-026).

## Design notes

- No `float32` or `float64` may appear in `internal/money`. Add this rule to the archtest from SPEC-001 for `money`, `engine`, `checks`, `findings`, `profile`, `lawtable`.

## Acceptance criteria

- [ ] AC-1 — Table tests cover every accepted and rejected input form for `Parse`, including `"1,12,450"`, `"(2,750.00)"`, `"-0.50"`, `"12.345"` (rejected without rounding), `""` (rejected), `"1e5"` (rejected).
- [ ] AC-2 — Property test: for random `Paise` values, `Parse(FormatIndian(p, true)) == p`.
- [ ] AC-3 — Overflow in `Add`, `Sum`, `MulRate`, `MulRatio` returns an error (tested at `math.MaxInt64` boundaries).
- [ ] AC-4 — Each rounding mode has tests at the half-way point and for negative values.
- [ ] AC-5 — `FormatIndian` matches the requirements §6.9 example (`1,12,450`, `1,15,200`, `2,750`).
- [ ] AC-6 — Period tests: FY of 31 Mar vs 1 Apr; Q1–Q4 of a FY; month list of a FY has 12 entries starting April; invalid strings rejected.
- [ ] AC-7 — The archtest fails if `float64` is used in `internal/money`.

## Verification

```
go test ./internal/money/ ./internal/period/ -v
go test ./internal/archtest/
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
