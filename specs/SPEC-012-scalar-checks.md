# SPEC-012 — Scalar check types

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 2 |
| **Depends on** | SPEC-008 |
| **Requirements** | RC-01, requirements §6.2 (check types) |
| **Technical spec** | §B4.3 |
| **Code / content paths** | `internal/engine/checks/{sum_equals,gte,field_present,domain}`, `internal/findings/templates` |

## Goal

The engine can express the scalar rule categories R1, R2 (simple sums), R4, R5, R7 and R9 without Starlark.

## Scope

| Check type | Params | Fails when |
|---|---|---|
| `sum_equals` | `parts[]` (each with optional `sign: minus`), `total`, tolerance | `|Σparts − total| > tolerance` |
| `gte` | `a`, `b`, tolerance | `a < b − tolerance` |
| `field_present` | `field`, `present_elsewhere[]` | `field` is blank while any `present_elsewhere` fact has a value (R9) |
| `domain` | `field`, `allowed[]` or `law_table` | value outside the allowed set (R7); also covers R5 as "both codes present" |

- Each check type has `ValidateParams`, `Inputs`, `Evaluate`, a wording template, and its own pass/fail fixtures.
- Templates show every part of a sum line by line, then the computed total, the compared value and the difference.

## Out of scope

- `formula` (SPEC-014), draft checks (SPEC-015), record/date/threshold checks (SPEC-029).

## Acceptance criteria

- [ ] AC-1 — Each check type has boundary tests at the tolerance, and a missing-input test giving `CannotCheck`.
- [ ] AC-2 — `sum_equals` shows each part with its locator in the rendered finding.
- [ ] AC-3 — `field_present` reports the place where the value is blank and every place where it is present; it never states which is correct.
- [ ] AC-4 — `domain` with `law_table` reads allowed values through `EvalCtx.Law` (stub until SPEC-013, then real).
- [ ] AC-5 — All templates pass the banned-word test.

## Verification

```
go test ./internal/engine/checks/... ./internal/findings/... -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
