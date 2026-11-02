# Security and privacy

The tool holds client PANs, GSTINs and full financial statements. The threat model is in `docs/security/data-and-threat-model.md`; these are the working rules.

## Client data

- Never put real client data in the repo: not in code, tests, fixtures, sample JSON, screenshots, commit messages or docs. Use the fictional packs (`testdata/`, Test Pack A "Nimbus Logic Systems") or invent obviously synthetic values.
- Database files (`*.db`, `-wal`, `-shm`, `-journal`), `/data/`, `/vault/` and `/samples-private/` are git-ignored. Do not change that, and do not copy such files into tracked folders.
- Do not log PANs, GSTINs, names or amounts beyond what a finding already shows to an authorised user. Error messages name the file and locator, not the client's figures.
- A PAN reaches the CLI on standard input (`client-add`), never as a command-line argument, so it does not land in shell history or process lists.

## Storage and keys

- Encryption at rest is decided (the `ncruces/go-sqlite3` AES-256-XTS VFS, `docs/architecture/decisions/ADR-0002-database-encryption.md`) but **not yet in use**: `internal/store` still opens a plain SQLite file, and key handling comes with SPEC-034 (Module 14). Until then treat every `.db` file and its backups as unencrypted client data, and do not add new places that store client data outside the database.
- Private signing keys (`*.key`, `*.pem` other than `*.pub.pem`) never enter the repo. Rule packs are signed with Ed25519 offline; only public keys are committed.
- No secrets in source, config or environment defaults.

## Network

- The engine has no network code (invariant 5). Only `recon-server` listens, and only on the address the operator gives (`--listen 127.0.0.1:8080` in development, a LAN address in the office). Never bind to all interfaces by default.
- The UI loads nothing from the internet: no CDNs, web fonts, analytics or remote images.
- The updater is a separate process; it is the only component allowed to fetch anything, and every pack it fetches is signature-checked before use.

## Code

- SQL is explicit and parameterised. Never build SQL from strings that contain input.
- Paths from users or uploaded files are cleaned and kept inside the intended folder (no `..` traversal). Uploaded file names are never trusted as paths.
- External programs (Poppler `pdftotext`) are run with an argument list, never through a shell, and only from `third_party/`.
- Parse untrusted files defensively: size limits, strict decoding, unknown keys rejected where the schema is strict.
