# SPEC-023 — Job queue, engine supervision and IPC

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 4 |
| **Depends on** | SPEC-011, SPEC-022 |
| **Requirements** | SC-04, SC-06; CLAUDE.md invariant 8 (in-process queue, state in DB) |
| **Technical spec** | §A3.1, §A3.2, §A9 |
| **Code / content paths** | `internal/jobs`, `internal/ipc`, `cmd/recon-engine` (daemon mode), migration adding `jobs` |

## Goal

The server queues reconciliation jobs in the database and drives one long-lived engine process over stdin/stdout. Jobs survive a restart of either process, and 50 clients can be queued at once.

## Scope

- `jobs` table with states `queued`, `running`, `done`, `failed`, `cancelled`.
- Server side: enqueue, cancel and list jobs. The server starts `recon-engine daemon`, writes the DB key as the first stdin line, and restarts the engine with back-off if it exits.
- Engine daemon: a worker pool of `max(1, cores − 2)` (configurable) that claims jobs from the DB. On start, it re-queues any jobs left `running`.
- The JSON-lines messages of §A3.1 (`job.ready`, `job.cancel`, `shutdown`, `job.progress`, `job.done`, `job.failed`), versioned with a `proto` field.
- Progress is reported through the API (`GET /jobs/{id}`) for polling.
- Load test: 50 synthetic clients queued together while the UI is in use, measuring SQLite write contention (§A3.2).

## Out of scope

- Uploading files (SPEC-024). Lifecycle (SPEC-025).

## Acceptance criteria

- [ ] AC-1 — Killing the engine mid-job: the server restarts it, the job completes, and the result equals an uninterrupted run.
- [ ] AC-2 — Restarting the server with jobs queued: the jobs resume from the DB.
- [ ] AC-3 — Cancelling a running job stops it within 5 s and leaves no partial run marked complete.
- [ ] AC-4 — The DB key never appears in process arguments or environment variables (checked on Windows with a process listing).
- [ ] AC-5 — 50 synthetic clients complete in under 30 min on the reference machine (SC-06), with API responses staying under 1 s during the run.
- [ ] AC-6 — Contention results are recorded in `docs/adr/`. If writes time out, the §A3.2 fallback is scheduled as a new spec.

## Verification

```
go test ./internal/jobs/... ./internal/ipc/... -v
go run ./cmd/recon-dev loadtest --clients 50
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
