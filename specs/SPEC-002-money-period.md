# SPEC-002 — Money and period primitives

| | |
|---|---|
| **Status** | Blocked (waiting for SPEC-001 to be Done, and for a CA to confirm ₹10 rounding of negative amounts) |
| **Phase** | 1 |
| **Depends on** | SPEC-001 |
| **Requirements** | NF-01, NF-03, FV-01, RU-33 |
| **Technical spec** | §A5, §A8 |
| **Code / content paths** | `internal/money`, `internal/period` |

## Goal

All money arithmetic in Tool 1 goes through one integer type with explicit rounding, and all periods are written in one canonical form.

## Scope

**`internal/money`** (as built)

- `type Paise int64`, with `MaxPaise`, `MinPaise` and `PaisePerRupee`.
- `Parse(text string, unit Unit) (Paise, error)`: string-based, never through `float64`. Accepts plain (`112450.50`), Indian-grouped (`1,12,450.50`) and Western-grouped (`112,450.50`) text, a leading minus, accounting brackets for negatives and a leading `₹`. Refuses more than two decimal places, misplaced commas, exponents, a minus sign together with brackets, and non-digits, with a `*ParseError` giving a plain reason. `Unit` is `UnitRupees` or `UnitPaise`.
- `ParseJSONNumber(json.Number, Unit)`: same rules for JSON input, but no grouping, brackets or `₹`.
- `ParseRounded(text, unit, mode)`: as `Parse`, but extra decimal places are rounded by a named `Rounding`.
- `Add`, `Sub`, `Neg`, `Abs`, `Sum(...Paise)`: return `ErrOverflow` on overflow and never wrap.
- `MulRate(p, rateBP, mode)` with rates in basis points (1% = 100), and `MulRatio(p, num, den, mode)`. Both use exact big-integer arithmetic internally.
- `Rounding` modes: `RoundNone` (`ErrInexact` if a remainder exists), `RoundHalfUpPaisa`, `RoundHalfUpRupee`, `RoundNearestTenRupees`, `RoundDownPaisa`, `RoundUpPaisa`. The zero value is invalid (`ErrRounding`), so every caller must name a mode. "Half up" rounds a result that is exactly halfway away from zero.
- `FormatIndian(p Paise, showPaise bool) (string, error)`: `1,12,450` / `1,12,450.50` / `-2,750`. It returns an error if `showPaise` is false and the amount has paise, so a report never shows a different figure from the one stored.

**`internal/period`** (as built)

- Canonical strings: tax year `"2025-26"`, FY period `"FY2025-26"`, quarter `"FY2025-26:Q1"` (Q1 = Apr–Jun) and month `"2025-04"`. Dates are ISO `"2025-07-31"`.
- Types: `Date`, `TaxYear`, `Period` (with `Kind`: year, quarter or month).
- `ParseDate`, `ParseTaxYear`, `Parse` (period), `Year`, `Quarter`, `Month`, `TaxYearOf(date)`, `TaxYear.Prior()`, `MonthsOf(ty)`, `QuartersOf(ty)`, `QuarterOf(month)`, `MonthOf(date)`, `Contains(outer, inner)`, and `FirstDay`/`LastDay` for tax years and periods. Leap years are handled. Invalid input returns a `*period.Error` with a plain reason. No clock is read.

## Out of scope

- Which regime applies to a tax year: that is law-table data (SPEC-013), not code.
- Locale formatting in the UI (SPEC-026).

## Design notes

- No floating point may appear in the money-handling packages. The archtest rule `TestMoneyPackagesUseNoFloatingPoint` scans `internal/money`, `internal/engine` (including checks), `internal/findings`, `internal/profile` and `internal/lawtable` for float or complex types, float literals, and `strconv`/`math` float conversions, and reports the file and line.

## Acceptance criteria

- [x] AC-1 — Table tests cover every accepted and rejected input form for `Parse`, including `"1,12,450"`, `"(2,750.00)"`, `"-0.50"`, `"12.345"` (rejected without rounding), `""` (rejected) and `"1e5"` (rejected).
- [x] AC-2 — Property test: for random `Paise` values, `Parse(FormatIndian(p, true)) == p`.
- [x] AC-3 — Overflow in `Add`, `Sum`, `MulRate`, `MulRatio` returns an error (tested at `math.MaxInt64` boundaries).
- [x] AC-4 — Each rounding mode has tests at the half-way point and for negative values.
- [x] AC-5 — `FormatIndian` matches the requirements §6.9 example (`1,12,450`, `1,15,200`, `2,750`).
- [x] AC-6 — Period tests: FY of 31 Mar vs 1 Apr; Q1–Q4 of a FY; month list of a FY has 12 entries starting April; invalid strings rejected.
- [x] AC-7 — The archtest fails if `float64` is used in `internal/money`.

## Verification

```
go test -count=1 ./internal/money/ ./internal/period/ -v
go test -count=1 ./internal/archtest/
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| 13 Oct 2026 | Claude Code, approved step by step by the project owner | not yet committed | Built in 7 approved steps. AC-1: about 60 accepted/refused forms in `parse_test.go`. AC-2: round trip of more than 20,000 seeded random amounts plus edge values in `format_test.go`. AC-3: boundary tests in `money_test.go` and `rate_test.go`. AC-4: halfway and negative cases for every mode in `rate_test.go`. AC-5: §6.9 figures in `format_test.go`. AC-6: `period_test.go`. AC-7: proven by break-and-restore (a `float64` GST rate of `0.18` added to `internal/money` was reported at its line, then removed). The API differs from the original draft in naming and in `FormatIndian` returning an error; the Scope above records what was built. |
| 13 Oct 2026 | Claude Code, on the project owner's instruction | not yet committed | Fixes from code review. Status moved from Done to Blocked: SPEC-001 (a dependency) is not Done, and `RoundNearestTenRupees` needs a CA to confirm the rule for negative amounts (refunds). For positive amounts it is now tested against the ss.288A/288B reading "ignore paise, then nearest ₹10" for every value up to ₹3,000. Added `ErrZeroDenominator`, `Rounding.String` and `ParseRounding` (rule names such as `nearest_10_rupees`; unknown names are refused), a 40-decimal-place cap on `ParseRounded`, and a stricter `ParseJSONNumber` (no surrounding spaces or leading zeros). Tests now pin that Dr/Cr and `(-)` forms are refused. Period: `IsZero` and `Valid` on `Date`, `TaxYear` and `Period`; the zero `Period` has no days and contains nothing; years run 0001 to 9999, so the zero values are never valid; `Parse("2025-26")` suggests `FY2025-26`; error reasons no longer say "must be" ("month is outside 01 to 12"). The archtest float guard now allows only integer constants from `math`, and flags `big.Float`, `.Float64()`/`.Float32()` calls and float verbs in `fmt` format strings. |
