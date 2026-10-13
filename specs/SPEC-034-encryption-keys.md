# SPEC-034 — Encryption at rest and key management

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 6 |
| **Depends on** | SPEC-024 |
| **Requirements** | NF-04 (encrypted database), ENV-04 |
| **Technical spec** | §A3.1, §B12.2, §B17 (T2) |
| **Code / content paths** | `internal/store` (SQLCipher driver), `internal/vault` (AES-256-GCM), `internal/keys` (server only) |

## Goal

The database and every stored file are encrypted with AES-256. The service restarts unattended, and the data can still be recovered on new hardware.

## Scope

- Switch `store.Open` to a SQLCipher driver (cgo) and confirm it passes the engine archtest. Windows builds run on a Windows CI runner or a MinGW cross-toolchain.
- Master key: random 256 bits, created at install, sealed with Windows DPAPI (machine scope) and stored in the data directory.
- Recovery code: the master key, encoded for printing once at install. The restore flow on a new machine accepts it.
- Vault: AES-256-GCM per file, with a key derived from the master key (HKDF, `info = "vault"`). The file name stays the plaintext SHA-256 so deduplication still works.
- The key is handed to the engine only via stdin (SPEC-023).
- Migration tool: encrypts an existing plain database and vault from earlier phases.

## Acceptance criteria

- [ ] AC-1 — `recon.db` cannot be opened by plain SQLite and contains no readable client names (byte-search test).
- [ ] AC-2 — Vault files contain no plaintext from the original (byte-search test), and a tampered vault file fails to decrypt with an error.
- [ ] AC-3 — The service restarts after a reboot with no passphrase prompt.
- [ ] AC-4 — Copying the data directory to another machine without the recovery code fails to open; with the code, it opens.
- [ ] AC-5 — The engine archtest passes with the SQLCipher driver.
- [ ] AC-6 — The plain-to-encrypted migration preserves all data (row counts and file hashes compared).

## Verification

```
go test ./internal/store/... ./internal/vault/... ./internal/keys/... -v
go test ./internal/archtest/
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
