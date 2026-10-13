# SPEC-042 — Binary updater

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 7 |
| **Depends on** | SPEC-040, SPEC-041 |
| **Requirements** | ENV-05, NF-06 |
| **Technical spec** | §A3 (`recon-updater`), §B10 |
| **Code / content paths** | `cmd/recon-updater`, `internal/pack` (shared signature verification) |

## Goal

A new version of Tool 1 can be applied from a USB drive. The updater verifies it offline, swaps it in safely, and can roll back to the previous version.

## Scope

- Update file: a zip with the new binaries, a manifest (version, minimum previous version, file hashes, DB migrations included) and an Ed25519 signature, using the same verification code as SPEC-041.
- `recon-updater` is a separate process with no network access. It:
  1. verifies the signature and hashes;
  2. takes a backup (SPEC-037) before changing anything;
  3. stops the `recon-server` service, which also stops the engine;
  4. moves the current binaries to `previous/`;
  5. puts the new binaries in place;
  6. starts the service, which runs the new DB migrations;
  7. checks the health endpoint.
- If the health check fails, the updater restores `previous/` and the pre-update backup automatically, and reports why.
- A manual "Roll back to previous version" action (Partner only), offered from Admin and from the updater command line.
- Started from Admin (Partner only) or by running `recon-updater apply <file>` on the main desktop.
- Every update and rollback is written to the audit log.

## Out of scope

- Downloading updates; Tool 1 has no internet access (ENV-03).

## Acceptance criteria

- [ ] AC-1 — An update with a changed byte or an unknown signing key is rejected, and nothing is stopped or changed.
- [ ] AC-2 — A valid update installs; the service comes back with the new version, and existing data and findings are intact.
- [ ] AC-3 — An update whose new binary fails the health check is rolled back automatically to the previous version and data.
- [ ] AC-4 — A manual rollback after a successful update returns to the previous version, with data readable.
- [ ] AC-5 — `recon-updater` passes the same no-network archtest as the engine.
- [ ] AC-6 — Update and rollback events appear in the audit log.

## Verification

```
go test ./cmd/recon-updater/... ./internal/pack/... -v
go test ./internal/archtest/
```

Plus a manual update/rollback run on the Windows test VM, recorded below.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
