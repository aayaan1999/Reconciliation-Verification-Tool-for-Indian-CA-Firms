# SPEC-008 — Rule engine core and `equal` check

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 1 |
| **Depends on** | SPEC-007 |
| **Requirements** | RU-50, RU-51, RU-52, RU-53, SC-07 |
| **Technical spec** | §A6.3, §A7 (steps 4–7), §A8, §B4.3, §B4.4, §B6 |
| **Code / content paths** | `internal/engine`, `internal/engine/checks/equal` |

## Goal

The engine takes a set of facts and a set of rules, and produces sorted, deterministic evaluations: pass, fail or cannot check.

## Scope

- `CheckType` interface and registry (§A6.3).
- `EvalCtx` exposing `Fact`, `Facts`, `Records`, `FactHistory` (stubbed until SPEC-015) and `Law` (stubbed until SPEC-013). No clock, randomness, file or DB access.
- Rule selection: rules whose `applies.tax_years` include the engagement's tax year.
- Dependency index: `rule → inputs` and the inverse `input → rules` (used for incremental runs in SPEC-017).
- Subject expansion: one evaluation per period/scope present; `scope: each | sum`.
- Tolerance: `{ paise: N }` and `{ bp_of_larger: N }`, integer arithmetic only.
- `CannotCheck` with `MissingFacts` when a required input is absent (RU-52).
- Parallel evaluation across rules with a worker count parameter; results sorted by `(rule_id, subject_key)` before return.
- Run snapshot: rule IDs and versions used (RU-53).
- `equal` check type, with pass/fail fixtures.
- Fixture runner: loads each rule's fixtures and asserts expected evaluations (used by `go test`).

## Out of scope

- Other check types (SPEC-012, 014, 015, 029). Persistence (SPEC-010). Wording (SPEC-009).

## Acceptance criteria

- [ ] AC-1 — `equal` passes when |a − b| ≤ tolerance and fails otherwise, for both tolerance kinds (boundary tests at exactly the tolerance).
- [ ] AC-2 — A missing input gives `CannotCheck` naming the missing fact; it never gives `Fail`.
- [ ] AC-3 — The same input with worker counts 1, 4 and 16 gives byte-identical canonical output.
- [ ] AC-4 — Rules not applicable to the tax year are not evaluated and are not in the snapshot.
- [ ] AC-5 — Fixture runner executes every rule's pass and fail fixtures as subtests named after the rule ID.
- [ ] AC-6 — Synthetic benchmark: 600 `equal` rules over 10,000 facts evaluate in under 5 seconds on the reference machine (SC-07 early check).
- [ ] AC-7 — Engine archtest passes.

## Verification

```
go test ./internal/engine/... -v
go test ./internal/engine/ -bench . -run ^$
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
