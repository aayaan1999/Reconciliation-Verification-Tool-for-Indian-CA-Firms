# SPEC-021 — New-head detection and worklist

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 3 (Phase 3 gate) |
| **Depends on** | SPEC-020 |
| **Requirements** | RC-02; requirements §13 Phase 3 "Worklist generated from two years of ledgers" |
| **Technical spec** | §A6.4, §B8 (`newheads`); v0.2 spec §7 |
| **Code / content paths** | `internal/modules/newheads`, migration adding `worklist_items`, `content/rules/<tax-year>/newheads/` |

## Goal

Comparing two years of Tally ledgers produces a worklist for staff to classify: new heads, vanished heads, broken recurrences, step changes and unmapped heads. The worklist is kept separate from findings.

## Scope

- `Prepare`: aggregate `ledger_month` / `voucher` records per ledger per month for FY *n* and FY *n−1*, where *n−1* is the same client's prior-year engagement.
- Checks (thresholds are rule params, never hard-coded):
  - new head: present in *n*, absent in *n−1*; shows first-seen date, total and transaction count;
  - vanished head: present in *n−1*, absent in *n*;
  - broken recurrence: present in at least 10 of the 12 months of *n−1*, with named months missing in *n*;
  - step change: the value moved by more than the configured percentage;
  - unmapped head: a new head with no confirmed classification.
- Worklist items are stored in `worklist_items` with their details. Classification fields (capital/revenue, audit clause, counterparty GST status, TDS section, statutory-payment category) are stored as user-confirmed facts with `source = user`, for use by SPEC-032.
- CLI: `recon-engine worklist --engagement <id>` prints the worklist in the v0.2 §7.2 layout.
- If the prior year is missing, the worklist shows one notice, "prior year ledgers not loaded", and no items.

## Out of scope

- Worklist screen and classification UI (SPEC-026). Local-model suggestions (not in Tool 1's first release).

## Acceptance criteria

- [ ] AC-1 — A synthetic two-year ledger set produces exactly the expected items of each kind (golden file).
- [ ] AC-2 — No worklist item ever appears in the findings list or the finding counts.
- [ ] AC-3 — Changing the step-change threshold param changes the results without any code change.
- [ ] AC-4 — A ledger renamed between years appears as one vanished head and one new head (no fuzzy matching).
- [ ] AC-5 — Output is identical across runs.

## Verification

```
go test ./internal/modules/newheads/ -v
go run ./cmd/recon-engine worklist --db scratch.db --engagement 1
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
