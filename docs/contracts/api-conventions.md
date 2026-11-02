# API conventions

Purpose: how the HTTP API and the command line talk to callers. It summarises the technical spec §A3.1 (server and engine), §B6 (command line), §B11 (API surface) and §B12 (security), checked against `cmd/recon-server/main.go`, `internal/api/api.go` and `cmd/recon-engine/main.go`.

**Built** means it exists in the code today, with the spec that added it. **Planned** means it is only in the technical spec.

## Shape of the system

- `recon-server` serves the browser UI and the API. It **never evaluates a rule itself**: every check runs in a separate `recon-engine` process (§A3.1), started once per request, one at a time, because the engine is the database's single writer.
- Today the server calls the engine's command line and reads its `--json` output. The planned long-lived engine child process with newline-delimited JSON control messages on stdin/stdout and a DB-backed job queue (§A3.1) is **not built**.
- The screens read exactly the shape of the engine's `--json` output (`web/src/shape.ts`). `TestWebSamplesMatchEngine` keeps the screens' sample files in `web/src/sample/` byte-equal to real engine output.

## Binding and transport

| Item | Built today | Planned |
|---|---|---|
| Listen address | `127.0.0.1:8080` unless `--listen` says otherwise (`:8080` for the office network) | Office LAN only |
| Transport | Plain HTTP | HTTPS with a firm-local CA (§B12.2; open question T-Q2) |
| Logins | None (SPEC-004 part 6; Module 16) | Session cookie (HttpOnly, Secure, SameSite=Strict), CSRF token, role check on every handler (§B11, §B12.1) |
| Headers | `Content-Security-Policy: default-src 'self'; style-src 'self' 'unsafe-inline'`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer` | Same |
| UI | Embedded read-only with `-tags ui`; an unknown path returns `index.html` | Same |

The tool is fully offline: no endpoint calls out, and the UI loads no CDN, web font or online resource.

## JSON conventions

- Prefix `/api/v1`. Request and response bodies are JSON, except uploads, which are `multipart/form-data`.
- Successful reads return `200`; a created client returns `201`.
- Money is integer paise in fields named `*_paise` (for example `difference_paise`). A value is `{"kind": "money", "paise": 11245050}`; only the field its kind names is present.
- Every value carries its locator: `{"file_name", "kind", "path", "document_version_id"}`.
- A full PAN is never returned; only its last four characters.
- Lists come in a fixed order, so the same input gives the same bytes.

## Error format

Every API error is one JSON object with a stable code and a plain message:

```json
{"error_code": "BAD_TAX_YEAR", "message": "choose a tax year such as 2025-26"}
```

A refused client adds a list of every problem at once: `422 {"error_code", "message", "problems": [{"field", "code", "message"}]}` (codes such as `BAD_PAN`, `BAD_GSTIN`, `GSTIN_CHECK_DIGIT`; §B1.3).

| HTTP status | When | Example codes |
|---|---|---|
| 400 | The request is malformed | `BAD_REQUEST`, `BAD_TAX_YEAR`, `BAD_CLIENT_ID`, `NO_FILES`, `TOO_MANY_FILES`, `DUPLICATE_FILE_NAME`, `BAD_COMPARISON`, `BAD_RULES` |
| 404 | Unknown endpoint or object | `NOT_FOUND`, `CLIENT_NOT_FOUND`, `FINDING_NOT_FOUND`, `WORKLIST_ITEM_NOT_FOUND` |
| 409 | Client already exists | `CLIENT_EXISTS` |
| 413 | Upload over the limit | `FILE_TOO_LARGE`, `UPLOAD_TOO_LARGE` |
| 415 | File type not accepted | `UNSUPPORTED_FILE_TYPE` (only JSON and CSV for now) |
| 422 | The engine refused the input or a rule (engine exit 2) | the engine's stable code, such as `PARSE_ERROR`, `PROFILE_NO_MATCH`, `CONFLICTING_VALUES`, `COMPARISON_NOT_FOUND`, `RULE_NOT_IN_COMPARISON`, `NO_RULES_SELECTED`, `NOTE_REQUIRED`; `INPUT_ERROR` when the engine gave none |
| 500 | Engine exit 3 or a server fault | `INTERNAL` |

The server takes the code from the engine's standard error (the first `UPPER_CASE:` word). See [error-model.md](error-model.md).

## Endpoints built today

| Method and path | What it does | Engine command | Added in |
|---|---|---|---|
| `GET /api/v1/rules` | Stored rules with status | `rules --json` | SPEC-004 part 6 |
| `GET /api/v1/tax-years` | Tax years the rules cover | `rules --json` | SPEC-004 part 6 |
| `POST /api/v1/runs` | Upload files and run checks. Multipart: files, `tax_year`, `include_testing`, optional `client_id`, `comparison` (repeatable) and `rules:<comparison id>`. At most 20 files, 64 MB each, 256 MB in all. Returns `exit_code`, `report`, `results` (one per type of check), `facts`, and for a client `decisions` | `run --json`, `facts --json` | SPEC-004 parts 6 and 9 |
| `GET /api/v1/clients?tax_year=` | Clients with their latest counts | `clients` | SPEC-004 part 7 |
| `POST /api/v1/clients` | Add a client (`{tax_year, details}`); PAN goes to the engine on standard input | `client-add` | SPEC-004 parts 7 and 8 |
| `GET /api/v1/clients/{id}` | All details, never the full PAN | `client-get` | SPEC-004 part 8 |
| `PUT /api/v1/clients/{id}` | Edit details; an empty PAN keeps it | `client-update` | SPEC-004 part 8 |
| `GET /api/v1/clients/{id}/result?tax_year=` | Latest result of every type, with each finding's decision and history | `result` | SPEC-004 part 7, SPEC-006 part 3 |
| `POST /api/v1/clients/{id}/decisions` | `{tax_year, finding_id, decision, note, by}`; decision is `fix`, `not_an_issue` or `ask_client` | `decide` | SPEC-006 part 3 |
| `GET /api/v1/worklist?tax_year=` | Every client's latest Worklist, with classifications | `worklist` | SPEC-006 part 3 |
| `POST /api/v1/clients/{id}/worklist` | `{tax_year, kind, ledger, classification, by}` | `classify` | SPEC-006 part 3 |
| `GET /api/v1/comparisons` | Types of check | `comparisons` | SPEC-004 part 8 |
| `POST /api/v1/inspect` | One uploaded file: its fields and whether a known format recognises it | `inspect` | SPEC-004 part 8 |
| `POST /api/v1/profiles` | Teach a layout: file plus `mapping`, optional `check_only` | `profile-create` | SPEC-004 part 8 |

Uploads are written to a fresh temporary folder that is removed before the response is sent, so nothing of an upload outlives its request.

## Planned API (§B11)

Engagement-based routes replace the client-based test API: auth (`/login`, `/logout`, `/me`), season board (`/engagements?tax_year=`), uploads and document versions, runs with job progress by polling `/jobs/{id}`, findings (`/engagements/{id}/findings?group=review_document`, `/findings/{sid}/decision`, `/findings/{sid}/history`), draft diffs, rules and Rule Builder, law tables, Format Register, packs, exports and admin. Every handler declares its permission and a test asserts none is registered without one. The exact move from today's routes to these is **Open**; see §B11.

## Command line (`recon-engine`)

The engine is also the Phase 1 command line. Commands and flags are listed in [CLAUDE.md](../../CLAUDE.md) and `recon-engine` with no arguments.

| Exit code | Meaning |
|---|---|
| 0 | No findings (or the command succeeded) |
| 1 | Findings present (`run` only) |
| 2 | Input or rule error: bad flag, unreadable file, unknown format, rule that does not load, refused request. Standard error carries the stable code and file name |
| 3 | Internal error (`internal error:` on standard error) |

- `--json <file>` writes the machine-readable output to a file; standard output keeps the text report. The folder must exist.
- Sensitive or structured input comes on **standard input**, never as an argument: the PAN for `client-add`, client details, decisions, classifications and mappings.
- The text report and `--json` hold no run ID, database ID or time, so the same input gives the same bytes (§B6).
- Without `--include-testing` only `active` rules run; with it, the report's first line says it includes rules the CA has not approved.
