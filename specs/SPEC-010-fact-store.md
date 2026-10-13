# SPEC-010 — Fact store (SQLite)

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 1 |
| **Depends on** | SPEC-003 |
| **Requirements** | FV-01, RU-53, NF-01, SC-05 |
| **Technical spec** | §A3.2, §B1 |
| **Code / content paths** | `internal/store`, `internal/store/migrations` |

## Goal

Facts, document versions, runs and evaluations are stored in SQLite through explicit SQL, behind interfaces that later allow SQLCipher and a single-writer fallback without touching callers.

## Scope

- Driver choice: must pass the engine archtest. It must be swappable for the SQLCipher driver in SPEC-034 behind the same `store.Open` function. Record the choice and the reason in `docs/adr/`.
- Migration runner: numbered `.sql` files embedded with `go:embed`, applied in order inside a transaction, version stored in a `schema_migrations` table.
- Pragmas on open: `foreign_keys=ON`, `journal_mode=WAL`, `synchronous=NORMAL`, `busy_timeout=5000`.
- Writer goroutine: all writes in this process go through one goroutine with batched transactions.
- Phase 1 tables (from §B1): `clients`, `engagements`, `files`, `documents`, `document_versions`, `facts`, `runs`, `run_documents`, `run_rules`, `evaluations`, `schema_migrations`. Later specs add their own tables by new migrations.
- Repositories implementing the interfaces that `engine` and `profile` declare (`FactReader`, `FactWriter`, `RunWriter`, …).
- Money columns are `INTEGER` and named `*_paise`.

## Out of scope

- Encryption (SPEC-034), records table (SPEC-020), findings tables (SPEC-025), users/sessions (SPEC-022).

## Acceptance criteria

- [ ] AC-1 — A fresh DB migrates to the latest version; running migrations again is a no-op.
- [ ] AC-2 — A failing migration rolls back fully and leaves the version unchanged.
- [ ] AC-3 — Facts written and read back are identical, including locators; values in paise survive round-trip at `MaxInt64` boundaries.
- [ ] AC-4 — No SQL string is built by concatenating user or file values; all use parameters (code review + grep test for `fmt.Sprintf` near `Exec`/`Query`).
- [ ] AC-5 — Writing 200,000 facts for one engagement completes in under 10 s on the reference machine.
- [ ] AC-6 — Engine archtest still passes with the chosen driver.

## Verification

```
go test ./internal/store/... -v
go test ./internal/store/ -bench BenchmarkWriteFacts -run ^$
go test ./internal/archtest/
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
