# SPEC-024 — Clients, engagements, uploads, vault and drafts

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 4 |
| **Depends on** | SPEC-022, SPEC-023 |
| **Requirements** | IN-10, IN-11, IN-12, IN-13, IN-14, ENV-04 |
| **Technical spec** | §B1 (clients, engagements, files, documents), §B2.1 |
| **Code / content paths** | `internal/vault`, `internal/api` (clients, uploads, documents), migration adding `client_scopes` |

## Goal

Staff create clients and tax-year engagements, upload any number of files, and get a validation report for each file. A newer version of a document becomes a new draft, and duplicate files are skipped.

## Scope

- Clients: name, PAN (stored as a hash plus the last 4 characters), and scopes (GSTINs, branches, Tally companies). One engagement per client per tax year.
- Upload API (multipart, many files at once): stream to a temp file, compute SHA-256, check for duplicates within the engagement (IN-13), then store in the vault as `vault/ab/cd/<sha256>.bin`. Files are stored unencrypted in this phase; encryption is SPEC-034, behind the same `vault` interface.
- Slot assignment: the slot (document type, period, scope, part) is proposed from detection, and staff confirm or change it. A file landing in an occupied slot gets `draft_no + 1` (IN-12).
- After an upload, a job is queued (SPEC-023). The validation report is available at `GET /document-versions/{id}/validation` (IN-14).
- Client match: if the profile extracts a PAN from the file, it is compared with the client's PAN hash. A mismatch blocks the file with a clear message.
- Combining files (IN-11): parts and months are stored as separate slots and combined by the engine's resolve step. If two files give different values for the same fact and slot, the report shows `CONFLICTING_VALUES`.

## Out of scope

- Encryption (SPEC-034). UI screens (SPEC-026).

## Acceptance criteria

- [ ] AC-1 — Uploading the same bytes twice to one engagement → the second upload is reported "duplicate, skipped".
- [ ] AC-2 — Uploading a changed computation creates draft 2, and draft 1 remains readable.
- [ ] AC-3 — Twelve monthly GST files upload in one request and land in twelve monthly slots.
- [ ] AC-4 — A file whose PAN does not match the client is blocked.
- [ ] AC-5 — Each upload returns a validation report with the profile, counts, unread items and errors.
- [ ] AC-6 — Staff can upload; Read-only users cannot (403).

## Verification

```
go test ./internal/vault/ ./internal/api/... -run 'Upload|Client|Engagement' -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
