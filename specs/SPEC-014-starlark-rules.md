# SPEC-014 — Starlark formula rules

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 2 |
| **Depends on** | SPEC-008, SPEC-013 |
| **Requirements** | RU-04, RU-24, RU-50, RU-51, NF-01, NF-03 |
| **Technical spec** | §A6.3, §B4.5 |
| **Code / content paths** | `internal/starhost`, `internal/engine/checks/formula`, `*.star` beside each rule |

## Goal

Complex calculations (for example MAT book profit, R2) run as sandboxed, deterministic Starlark scripts referenced from a YAML rule. Scripts cannot use floats and cannot write finding text.

## Scope

- `formula` check type: `script`, `params`, `compare_to`, tolerance.
- Host builtins: `fact(name)`, `fact_or(name, default)`, `law(table, key)`; nothing else.
- Script contract: `compute(fact, law, params)` returns a dict with `value` (int) and `steps` (list of `(label, int)`).
- Sandbox:
  - `load()` only for scripts inside the same rule folder or pack;
  - a step limit per call (`SetMaxExecutionSteps`), configurable, default 1,000,000;
  - float literals and the `/` operator are rejected statically, and any float in the result is rejected at run time;
  - a missing fact aborts with a typed error → `CannotCheck`.
- Each script is compiled once per run and cached by SHA-256.
- `params` are the only CA-editable part (RU-24); the rule declares their types and they are validated.
- Template: each step with its value, the computed total, the compared fact and the difference.
- Example rule: `RECON-MAT-001` (book profit recomputation) with pass/fail fixtures using the illustrative R2 figures.

## Out of scope

- Editing params from the UI (SPEC-027).

## Acceptance criteria

- [ ] AC-1 — A script containing `1.5`, `float(x)` or `a / b` is rejected at load with file:line.
- [ ] AC-2 — A script that loops forever fails that rule with "step limit exceeded", and the rest of the run completes.
- [ ] AC-3 — A script cannot read files, time or environment: attempted calls fail at load as undefined names.
- [ ] AC-4 — The `RECON-MAT-001` fail fixture reports the computed 2,95,27,500 against the filed 2,87,75,000, a difference of 7,52,500, with each step shown.
- [ ] AC-5 — Changing a `param` value changes the result and is recorded in the run snapshot.
- [ ] AC-6 — `go.starlark.net` passes the engine archtest.

## Verification

```
go test ./internal/starhost/ ./internal/engine/checks/formula/ -v
go test ./internal/archtest/
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
