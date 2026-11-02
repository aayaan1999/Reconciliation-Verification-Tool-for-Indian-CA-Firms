---
paths:
  - "internal/store/**"
---

# Database

SQLite through the pure-Go `ncruces/go-sqlite3` driver. Encryption at rest with its AES-256-XTS VFS is decided (`docs/architecture/decisions/ADR-0002-database-encryption.md`) and proven in `encryption_test.go`, but `store.Open` still opens a plain file until key handling lands (SPEC-034). The data model is in `docs/contracts/data-model.md`; the schema design is in `tool1-technical-spec.md` Part B.

## Rules

- **Explicit SQL, no ORM.** Every query is written out and parameterised. Never concatenate input into SQL.
- **Migrations are append-only.** A schema change is a new numbered file in `internal/store/migrations`. Never edit or reorder a migration that has been applied anywhere.
- **Money columns are `INTEGER` paise.** No `REAL` for money.
- **Decisions and audit records are append-only.** A decision on a finding is a new row against the finding's stable ID; nothing is updated in place or deleted.
- **Deterministic reads.** Every query whose result reaches output has an `ORDER BY` that gives a total order.
- **One process owns the file.** The job queue and its state live in the same database; no external services.
- **Never commit a database file.** `*.db` and its side files are git-ignored. Tests create databases in a temporary folder or under `build/`.
- Transactions wrap multi-step writes so a failed run leaves no half-written state.
