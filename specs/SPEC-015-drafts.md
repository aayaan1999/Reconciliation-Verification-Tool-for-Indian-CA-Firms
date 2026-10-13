# SPEC-015 — Draft history and draft comparison

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 2 |
| **Depends on** | SPEC-008, SPEC-010 |
| **Requirements** | IN-12, RC-06, RC-01 (R3, R10) |
| **Technical spec** | §A6.4 (`drafts` module), §B4.3 (`co_movement`, `draft_regression`), §B8 |
| **Code / content paths** | `internal/modules/drafts`, `internal/engine/checks/{co_movement,draft_regression}` |

## Goal

The engine can see every earlier draft of a document. It reports figures that moved while the figures depending on them did not (R3), and values that went back to an earlier value (R10).

## Scope

- `EvalCtx.FactHistory(name, period, scope)` returns the value in each `draft_no`, with locators, oldest first.
- `co_movement` check (`driver` fact, `dependant` fact): fails when the driver changed between the last two drafts and the dependant did not. The finding shows both facts in both drafts and the driver's change.
- `draft_regression` check: fails when a value returns to an earlier value across three or more drafts (A → B → A). The finding lists every draft's value.
- Draft diff: for a document, list the facts that changed between draft *m* and draft *n*. Using the dependencies declared in `co_movement` rules, also list dependent figures that did not change. CLI: `recon-engine diff --document <id> --from 2 --to 3`.

## Out of scope

- Draft diff screen (SPEC-026).

## Acceptance criteria

- [ ] AC-1 — R3 fixture: depreciation moves 1,15,200 → 1,12,450 between drafts while the profit-linked deduction is unchanged → one `co_movement` finding showing the 2,750 change.
- [ ] AC-2 — R10 fixture: A → B → A across three drafts → one `draft_regression` finding listing all three values.
- [ ] AC-3 — With only one draft, both checks return `Pass` (nothing to compare), not `CannotCheck`.
- [ ] AC-4 — Draft diff output is sorted and identical across runs.

## Verification

```
go test ./internal/modules/drafts/ ./internal/engine/checks/... -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
