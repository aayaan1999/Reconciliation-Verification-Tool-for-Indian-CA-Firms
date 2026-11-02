# Data and threat model

This page lists the data Tool 1 handles, where it is kept, how it is protected, and the threats against it with the mitigations that exist today and those that are planned. It summarises the technical spec §B1.3, §B10, §B12 and §B17 ([tool1-technical-spec.md](../../tool1-technical-spec.md)), the product spec v0.2 §4.1 and §10 ([recon-tool-spec.md](../../recon-tool-spec.md)), requirements v0.3 ENV-03 to ENV-07 and NF-04 to NF-09 ([requirements-spec-tool1.md](../../requirements-spec-tool1.md)), the ADRs in [../architecture/decisions/](../architecture/decisions/) and the current code. For boundaries and actors see [system-context.md](../architecture/system-context.md).

The security argument in one sentence: **the client data never leaves the office** (§10.1).

## Rule: never put real client data in the repository

All figures, PANs, GSTINs and entity names in the specs, tests, fixtures, test packs and screen sample data are illustrative or invented (CLAUDE.md; requirements A1; tech spec §B14). Test packs (`testdata/testpack_nimbus/`, `testdata/gst_pack_aug2026/`) are fictional companies. A real sample from the firm is anonymised before it comes near the repository; until then development uses synthetic formats (A1, A2).

## Data classes

| Data | Examples | Where it is today | Protection today | Planned |
|---|---|---|---|---|
| Client identity | Name, PAN, TAN, CIN/LLPIN, GSTINs, address, contact person, email, phone | `clients`, `client_gstins` in the SQLite file | PAN stored only as HMAC-SHA-256 (random key in the database) plus last four characters; never returned by the API; passed to the engine on standard input, never on the command line (SPEC-004 parts 7 and 8). A GSTIN contains the PAN in clear (tech spec §B1.3) | Encrypted database |
| Client financial figures | Facts and records: computation, audit report, P&L, invoices, ledger months | `facts`, `records`, `runs.report_json` | None beyond file permissions; the database is a **plain** file today (`internal/store/store.go`) | AES-256-XTS at rest ([ADR-0002](../architecture/decisions/ADR-0002-database-encryption.md), SPEC-034) |
| Full PANs inside documents | A party's PAN in an audit-report clause | Text facts in the database | As above | As above |
| Original uploaded files | JSON and CSV from the tax suite, Tally, portals | A temporary folder, removed before the server answers (tech spec §B11) | Short-lived | Content-addressed vault, AES-256-GCM per file (tech spec §B12.2) |
| Sample files of taught layouts | The file used on the mapping screen | Office profiles folder, beside the profile (SPEC-004 part 8.4) | A plain file, readable by the server's own account only (files `0600`, folders `0700`) | Covered by vault or encryption design: **Open** |
| Decisions and classifications | Fix / Not an issue / Ask client, notes, staff names | `finding_decisions`, `worklist_classifications` | Append-only, enforced by triggers (migration `0006`) | Hash-chained audit log (NF-05) |
| Rules, profiles, vocabulary | YAML content | `content/`, rule store, database | Strict YAML loaders; versions recorded per run | Ed25519-signed packs (NF-06) |
| Credentials and keys | Passwords, sessions, DB master key | Not built | — | argon2id hashes; master key sealed with Windows DPAPI; recovery code printed once (tech spec §B12.2) |
| Browser storage | Colour theme only | `localStorage` | No client data | — |

## Controls

| Control | Built | Planned |
|---|---|---|
| No network code in the engine | Import-graph and linked-symbol proofs on every test run ([ADR-0001](../architecture/decisions/ADR-0001-engine-network-proof.md)) | — |
| No internet anywhere | No CDN, web fonts or online lookups in the UI (SPEC-004 part 3) | — |
| Encryption at rest | Decided and proven in a test (`internal/store/encryption_test.go`) | Used for `recon.db`; key handling, DPAPI, recovery code (SPEC-034) |
| LAN exposure | Server binds `127.0.0.1:8080` unless `--listen` is given, and warns at start when the address can be reached from other computers; upload limits 20 files, 64 MB each, 256 MB total; Excel workbooks at most 1,000,000 filled cells (about 1.6 GB of memory at most). Until logins, requests a web page could make through a staff browser are refused: a `Host` that is not localhost, an IP address or a name given with `--host` (DNS rebinding), a POST or PUT from another site (`Origin`, `Sec-Fetch-Site`), and an API body that is neither JSON nor a file upload (`internal/api` `guard`, `TestGuard`) | HTTPS with a firm-local certificate (T-Q2 **Open**); session cookie HttpOnly, Secure, SameSite=Strict, CSRF token (§B11) |
| Authentication and roles | None: no logins, a typed name on decisions | Partner / Manager / Staff / Read-only matrix; idle timeout 30 min, absolute 10 h; lockout (§B12.1, §B12.2) |
| Audit | Append-only decisions and classifications | Hash-chained audit log of logins, views, decisions, exports, rule/law/pack changes; daily chain check (NF-05) |
| Rule packs and updates | — | `.reconpack` with `manifest.json` signed by Ed25519; public key in the binary; SHA-256 per file; fixtures run before install; office rules never overwritten; rollback (§B10). Signing key offline on the vendor side; `recon-dev pack sign` never shipped |
| Exports | — | Watermark with firm, user, time and export ID; every export logged; Staff cannot export (§B13) |
| Backup | — | Daily encrypted, signed backup; monthly restore test (NF-07) |
| Data protection | — | Client-wise export and deletion with a tombstone audit entry (NF-09, §B1.3). No DPDP compliance claim until a legal opinion (§10.2, Q7) |

## Threats (STRIDE)

| # | Category | Threat | Mitigation today | Planned |
|---|---|---|---|---|
| T-S1 | Spoofing | Anyone on the LAN uses the server as any staff member once it listens beyond `127.0.0.1` | Default bind to this computer only | Logins, roles, sessions, HTTPS |
| T-S2 | Spoofing | A forged rule pack or binary update on USB | — | Ed25519 signature and per-file hashes verified offline before install |
| T-T1 | Tampering | Someone with the database file changes or replays pages | — (plain file) | XTS encryption hides content but gives no page-level tamper detection; start-up integrity check, signed backups, hash-chained audit log (ADR-0002) |
| T-T2 | Tampering | A decision is edited or deleted to hide a finding | Triggers refuse updates and deletes on `finding_decisions`; findings themselves are recomputed from the files on every run | Audit hash chain |
| T-T3 | Tampering | A wrong rule produces confident wrong findings | Rules are strict YAML with pass and fail fixtures; new engine rules load as testing; reports flag testing rules | Partner approval workflow; a wrong rule is disabled, not tuned (RU-44) |
| T-T4 | Tampering | A PDF column misread produces a false finding | JSON preferred; PDF adapter not built | `pdftotext -layout`, region-based profiles, dense-column samples (§B17 T4, ADR-0003) |
| T-R1 | Repudiation | Staff deny making a decision | Name and time stored with each decision; history append-only | Logged-in user instead of a typed name; audit log |
| T-I1 | Information disclosure | The database file is copied off the desktop or from a backup | — (plain file) | AES-256-XTS; encrypted backups. XTS is deterministic: old copies show which sectors changed, not their content (ADR-0002) |
| T-I2 | Information disclosure | Client data leaves through the network | The engine has no connection code; no external calls anywhere | — |
| T-I3 | Information disclosure | Poppler's bundled `libcurl` opens a URL | The PDF adapter is not built | Pass only absolute vault paths, never a URL; a Poppler build without curl before release (ADR-0003) |
| T-I4 | Information disclosure | PAN visible in process lists or logs | PAN on standard input; the API never returns it | Same for the DB key (§A3.1) |
| T-I5 | Information disclosure | Traffic on the LAN read by another machine | — (plain HTTP) | HTTPS (T-Q2 **Open**) |
| T-I6 | Information disclosure | Real client data committed to the repository | The rule above; only invented data in tests and samples | — |
| T-D1 | Denial of service | Very large or deeply nested uploads | 64 MB per file, 20 files, 256 MB total; JSON nesting limit 512; one run at a time | Worker pool with cancellation (§A9) |
| T-D2 | Denial of service | Disk or database loss | — | Daily backup, tested restore |
| T-E1 | Elevation of privilege | Staff activate a rule or export | No roles yet | Role matrix checked on every handler; a test asserts every handler declares a permission (§B11) |
| T-E2 | Elevation of privilege | Malicious content in a rule or profile runs code | Rules and profiles are data read by strict loaders; no code execution | Starlark sandbox: no I/O, no clock, no floats, step limits (§B4.5) |

## Open

- HTTPS certificate distribution to staff PCs (tech spec §B17 T-Q2).
- Whether Managers may disable rules (T-Q3).
- Protection of sample files kept beside office profiles.
- DPDP legal opinion (Q7; gaps doc Part B.5).
- Treatment of scans with a scanner-made OCR text layer (§B17 T7, S-Q1).
