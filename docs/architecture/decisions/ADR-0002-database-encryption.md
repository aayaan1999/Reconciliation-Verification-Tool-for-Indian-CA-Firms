# ADR 0002 — Database encryption: the SQLite driver's AES-256-XTS instead of SQLCipher

- **Status:** Accepted (project owner, 14 October 2026)
- **Affects:** technical spec §B1, §B11 and risk T2, SPEC-034 (encryption and keys), CLAUDE.md stack line

## Context

The spec asked for SQLite + SQLCipher (AES-256) so that `recon.db` cannot be read without the key. SQLCipher is C code: every build machine and CI would need a C compiler (MinGW), the build could no longer run with cgo disabled, and it would replace the pure-Go SQLite driver chosen in ADR 0001.

That driver (`github.com/ncruces/go-sqlite3`) ships an encryption layer, the `xts` VFS. It encrypts every database, journal and WAL file with AES-XTS using standard library cryptography, with no C code.

## Decision

`recon.db` is encrypted with the `xts` VFS using a **64-byte key, which selects AES-256-XTS**. The store opens it as `file:recon.db?vfs=xts&hexkey=<128 hex digits>`, with `PRAGMA temp_store = memory`. A text password (`textkey`) is never used, because it derives only an AES-128 key. Where the key comes from and how it is protected is SPEC-034.

`internal/store/encryption_test.go` proves:
- with the key, data reads back;
- plain SQLite, or the wrong key, gets "file is not a database";
- the file on disk contains neither the client name nor the SQLite header nor any table definition.

## Consequences

- One self-contained program and no C compiler, as before; the network proof (ADR 0001) still covers the driver.
- **No tamper detection.** SQLCipher adds a checksum (HMAC) to every page, so a changed page is detected. XTS does not: someone with the file but not the key cannot read it, but could corrupt or replay old pages without the tool noticing at the database layer. Mitigations, to be built in SPEC-034 and the backup spec:
  - the audit log is already hash-chained;
  - backups are signed and verified before a restore;
  - the database runs an integrity check at start-up.
- **Deterministic encryption.** Someone holding several copies of the file (for example old backups) can see which 512-byte sectors changed between them, though not what they contain.
- An encrypted file is not SQLCipher-compatible: standard SQLCipher tools cannot open it, only this driver with the key.
- Revisit if a firm or auditor specifically requires SQLCipher or page-level authentication. The `adiantum` VFS from the same driver is the faster alternative; it has the same tamper caveat.
