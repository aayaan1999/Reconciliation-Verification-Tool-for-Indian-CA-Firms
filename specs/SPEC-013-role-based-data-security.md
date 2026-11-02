# SPEC-013 — Role-based data security

| | |
|---|---|
| **Status** | In progress |
| **Phase** | 6 |
| **Depends on** | SPEC-012 (users, roles, sessions), SPEC-010 (rule activation and disable) |
| **Requirements** | §3 (role table), NF-04 (access control, audit), §15 security checklist |
| **Technical spec** | §B12.1 (role matrix), §B12.2 (audit covers views of a client file, decisions, exports, rule and pack changes; exports watermarked and logged), §B11 (every handler declares a permission) |
| **Code / content paths** | `internal/api` (permission checks on rule, decision and client-file endpoints; view logging), `internal/auth` (the matrix stays the one source), `internal/store` (view audit rows through the existing `audit_log`), `web/src/screens/ClientFile.tsx`, `web/src/screens/Rules.tsx`, `web/src/permissions.ts` |

## Goal

Each role can read, change and export only the client data its role allows, and each view of a client file leaves an audit entry naming the logged-in user. The rules that protect data are the same on the server and in the browser.

## Scope

### 1. Permission check on every data-changing route

- Activating, approving, enabling and switching versions of a rule require `activate_rules` (Partner only). Disabling a rule requires `disable_rules` (Partner only, T-Q3). Saving a draft rule and running its tests require `draft_rules` (Partner, Manager, Staff). The matrix in `internal/auth/roles.go` is the source for these names.
- Deciding a finding and classifying a worklist item require `decide`, which Partner, Manager and Staff hold.
- Uploading files, running a reconciliation and entering client details require `run`, which excludes Read-only.
- A request refused by the role check returns 403 with the permission name, and writes one audit entry `auth.denied` with the user, route and permission. No client figures go into that entry.

### 2. Client file views are audited

- Opening a client file, a client's findings, or a client's worklist writes one `audit_log` entry `client.view` with the user, client ID and tax year. Repeated reloads within the same minute by the same user write one entry, not one each time.
- A view is audited whichever role opens it (Read-only included).

### 3. Data shown to each role

- Read-only and Staff see findings, values and sources in full, as the matrix allows viewing.
- The full PAN and GSTIN are shown only to Partner, Manager and Staff. Read-only sees them masked (last four characters).
- No new data is hidden from the engine; this is only the API and the screen.

### 4. Export and finding deletion

- Viewing clients, findings and the Worklist requires `view`, which every role holds.
- **Excel export** (`POST /engagements/{id}/exports`, format `xlsx`, §B13) requires `export` (Partner and Manager only). The file has one sheet per review document with findings, values, difference, locators, law reference and state; a sheet for cannot-check items; and a sheet for the Worklist. Every sheet carries a watermark line: firm name, logged-in user, timestamp and export ID. Each export writes one `export.created` audit entry with the export ID, client, tax year, format and row counts. No client figures go into the audit entry.
- **Delete a finding** requires `delete_findings` (Partner and Manager only). It needs a reason. The finding is kept in the database and marked deleted, so it can still be listed by a Partner; it is never removed. The delete writes one `finding.deleted` audit entry with the reason.
- A Staff or Read-only request to export or delete is refused with 403 and the permission name, as in §1.
- **Not in this spec:** the PDF export and the client query letter. The PDF library is chosen in Phase 4 and needs a licence review (§B13). Both return a clear "not available yet" message if requested.

### 5. Browser matches the server

- Buttons and tabs for an action are shown only when the signed-in role holds its permission (`web/src/permissions.ts` reads the server's list). A refused action shows the server's message, not a blank.

## Out of scope

- PDF export and the client query letter (Phase 4, after the PDF library licence review).
- Database encryption at rest and key handling (SPEC-034, ADR 0002).
- Masking inside exports and the engine's own output.
- Locking a client to specific users (no client-level access list exists in the requirements).

## Design notes

- The matrix lives in one table in `internal/auth/roles.go`. This spec adds the permission names it needs and nothing else; the Partner, Manager, Staff and Read-only rows do not change.
- View logging uses `audit_log` as it stands (migration 0007). The debounce is stored in the server's memory and reset on restart; it does not change the audit rows.
- The masking rule is applied in the API response, so the browser never holds the full PAN or GSTIN for a Read-only user.

## Acceptance criteria

- [ ] AC-1 — a Staff or Manager request to activate, approve, enable, switch versions, disable a rule, or change a law-table row is refused with 403 and the permission name; the same request by a Partner succeeds (test per route).
- [x] AC-2 — Staff, Manager and Partner can save a draft rule and run its tests; Read-only cannot do either (test per role).
- [x] AC-3 — Read-only cannot upload, run a reconciliation or enter client details; Staff can (test per role).
- [x] AC-4 — a refused request writes exactly one `auth.denied` entry naming the user, route and permission and holding no client figures; the audit chain verifies.
- [x] AC-5 — opening a client file writes one `client.view` entry per user, client and tax year per minute, whatever the role; the chain verifies.
- [ ] AC-6 — Read-only sees a masked PAN and GSTIN in the API response and on the Client file screen; Staff, Manager and Partner see them in full (test on the response body and the rendered text).
- [x] AC-7 — every route in the walk declares a permission (`TestEveryEndpointDeclaresAPermission` still passes), and the route walk covers the export and delete routes for all four roles.
- [x] AC-7a — Partner and Manager can export an Excel file for a client and tax year; Staff and Read-only get 403. The file has a sheet per review document, a cannot-check sheet and a Worklist sheet, and each sheet carries the watermark (firm name, user, timestamp, export ID).
- [x] AC-7b — each export writes one `export.created` entry with export ID, client, tax year, format and row counts, and no client figures; the chain verifies.
- [x] AC-7c — Partner and Manager can delete a finding with a reason; Staff and Read-only get 403. The finding is marked deleted, not removed; it is still listed for a Partner; one `finding.deleted` entry holds the reason.
- [x] AC-7d — a PDF or query-letter request returns the "not available yet" message and writes no export.
- [x] AC-8 — the browser hides each action whose permission the signed-in role lacks; a test checks Read-only and Staff on the Rules and Client file screens.
- [x] AC-9 — no new table, migration or dependency is added; the engine binary still links no network code (`go test -count=1 ./internal/archtest/ -v` passes).
- [x] Every file this spec created or changed in its role is added to `docs/Tool1-File-Guide.docx` in plain language.

## Verification

- `go test -count=1 ./internal/auth/... ./internal/api/...` covers AC-1 to AC-7.
- `cd web && npm test && npm run lint && npm run build` covers AC-6 and AC-8.
- `go test -count=1 ./internal/archtest/ -v` covers AC-9.
- Manual, in the browser after a rebuild: sign in as Read-only and as Staff and check the Rules and Client file screens; confirm the audit log in the Admin tab shows `client.view` entries under each login name.

## Open questions

- Decided: Read-only sees the PAN and GSTIN masked.
- Decided: only the Partner can disable a rule (T-Q3 resolved as Partner only).
- Decided: export and finding deletion are included; PDF and the query letter are not.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| 2026-10-30 | implementer | not committed | Built and verified except two criteria; status stays In progress. **AC-1**: the Partner and role checks pass for every rule route that exists, but there is no law-table route in this build, so the law-table part is untested. **AC-6**: the GSTIN is masked for Read-only and in full for the other roles (tested on the response and the rendered text), but the PAN is never stored in full (only a keyed hash and the last four characters), so the full PAN cannot be shown to Partner, Manager or Staff until the storage decision is made. **Manual browser steps** (Verification) were not run. **AC-7** names `TestEveryEndpointDeclaresAPermission`, which is not in the repository; `TestRoutesAreOnlyRegisteredFromTheDeclaredList` and `TestEveryRouteFollowsTheRoleMatrix` cover it. **Design as built** (technical spec B12.3, B13): a finding deletion is the `finding.deleted` audit entry, hidden from non-Partner roles, so no table or migration was added; the export route is `POST /api/v1/clients/{id}/exports`; the query letter is `POST /api/v1/clients/{id}/query-letter`; the firm name is set with `recon-server --firm`. A client's Season board counts still include deleted findings (not adjusted). |
