# SPEC-025 — Findings lifecycle and decisions

| | |
|---|---|
| **Status** | Not started (state table needs CA sign-off: T6) |
| **Phase** | 4 |
| **Depends on** | SPEC-023, SPEC-024 |
| **Requirements** | FD-01, FD-02, FD-03, FD-04, FD-05, FD-06 |
| **Technical spec** | §B7 |
| **Code / content paths** | `internal/findings` (lifecycle), `internal/api` (findings), migration adding `findings`, `finding_events` |

## Goal

Findings keep a stable identity across runs and carry staff decisions. They are resolved or reopened automatically when corrected files arrive.

## Scope

- `findings` and `finding_events` tables.
- Engine lifecycle step after each run: apply the system transitions of §B7.3 by stable ID. Every transition writes a `finding_events` row with actor `system` and the run ID.
- User decisions via the API: Fix, Not an issue (note required), Ask client; carry forward (Manager/Partner); delete (per the role matrix).
- The list API groups findings by `review_document` (FD-06).
- History API: all events and evaluations for one finding.
- The `withdrawn` state is implemented behind a flag until T6 is signed off.

## Out of scope

- UI (SPEC-026). Audit log entries (SPEC-036 adds them to the same calls).

## Acceptance criteria

- [ ] AC-1 — Re-running unchanged inputs creates no new findings and no state changes.
- [ ] AC-2 — Each row of the §B7.3 table has a test.
- [ ] AC-3 — "Not an issue" stays closed on a re-run with the same values and reopens when any input value changes. A change to the locator only does not reopen it (FD-04).
- [ ] AC-4 — Uploading a corrected file resolves the finding automatically and records the run (FD-05).
- [ ] AC-5 — A resolved finding that fails again in a later draft becomes `reopened`, not a new finding.
- [ ] AC-6 — `CannotCheck` never resolves a finding.
- [ ] AC-7 — Editing a rule (a new version) keeps the same stable ID.
- [ ] AC-8 — The CA has signed off the state table (recorded below).

## Verification

```
go test ./internal/findings/... -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
