# Handoff — SPEC-013 — Role-based data security

| | |
|---|---|
| **From** | implementer |
| **To (next agent)** | test-engineer |
| **Date** | 2026-10-30 |
| **Spec** | `specs/SPEC-013-role-based-data-security.md` (status as read at the start: Not started; now In progress) |
| **Part / criteria covered** | AC-2, AC-3, AC-4, AC-5, AC-7, AC-7a to AC-7d, AC-8, AC-9 ticked. AC-1 and AC-6 left open (see Open issues) |

## What was done

- Excel export (`internal/export`, excelize): one sheet per review document, a cannot-check sheet and a Worklist sheet; every sheet has a watermark line (firm, user, time, export ID); deleted findings are left out.
- Export route `POST /api/v1/clients/{id}/exports` (Partner and Manager). The file is built in memory and sent under a server-generated name. Writes `export.created` with the export ID, client, tax year, format and row counts.
- PDF format and `POST /api/v1/clients/{id}/query-letter` answer 501 `NOT_AVAILABLE_YET` and write nothing.
- Finding deletion `POST /api/v1/clients/{id}/findings/{finding}/delete` (Partner and Manager, reason required). It writes `finding.deleted`; no table or migration. Roles other than Partner no longer see the finding in result or history; the Partner sees it marked with who, when and why.
- Role refusals: 403 now names the permission; each refused role check writes one `auth.denied` entry (user, route pattern, permission, role).
- Client file views: `client.view` per user, client and tax year a minute (result, history, details, and each client in a Worklist view).
- GSTINs masked to all but the last four characters for Read-only (details response). PAN unchanged (last four only, as stored).
- `recon-server --firm` sets the watermark's firm name.
- Browser: Export Excel and Delete finding controls shown only with `export` and `delete_findings`; the Client file and Rules screens are tested for Read-only, Staff, Manager and Partner.
- Technical spec B12.3 and B13 updated with the as-built design.

## Files changed

| File | Change (added / edited / removed) | Why |
|---|---|---|
| `internal/export/export.go` | added | Excel workbook builder, watermark, export ID |
| `internal/export/export_test.go` | added | Proof of sheets, watermark, deletions, figures |
| `internal/export/doc.go` | edited | Package statement now names SPEC-013 scope |
| `internal/api/access.go` | added | Audit helpers, view debounce, deletion lookup, result filter, GSTIN mask |
| `internal/api/exports.go` | added | Export, query letter (501), delete finding handlers |
| `internal/api/access_test.go` | added | AC-1 to AC-7d over the real server |
| `internal/api/api.go` | edited | `Config.Firm`; view fields on Server; three new routes; `authorize(route, perm, h)`; client file handlers record views and filter deletions |
| `internal/api/auth.go` | edited | `authorize` takes the route; refusal writes `auth.denied` and names the permission |
| `internal/api/auth_test.go` | edited | Walk fills `{finding}`; "no route deletes" check narrowed to user routes (the new finding route is a deletion of a finding) |
| `cmd/recon-server/main.go` | edited | `--firm` flag |
| `web/src/api.ts` | edited | `exportClient`, `deleteFinding` |
| `web/src/types.ts` | edited | `Finding.deleted` (Partner only) |
| `web/src/screens/ClientFile.tsx` | edited | Export button, delete form, deleted marker; gated by permission |
| `web/src/App.tsx` | edited | Passes export and delete handlers only when the role holds the permission; `downloadFile` |
| `web/src/spec013.test.tsx` | added | AC-6 (rendered GSTIN), AC-8 (Client file and Rules by role) |
| `tool1-technical-spec.md` | edited | B12.3 as built; B13 as-built note |
| `specs/SPEC-013-role-based-data-security.md` | edited | Status In progress; criteria ticked; completion record |
| `specs/README.md` | edited | SPEC-013 status In progress |
| `docs/Tool1-File-Guide.docx` | edited | Seven rows added (export.go, export_test.go, doc.go, access.go, access_test.go, exports.go, spec013.test.tsx). Backup of the previous file: scratchpad `Tool1-File-Guide.before-spec013.docx` |
| `docs/agents/handoff-SPEC-013.md` | added | This note |

New files to add to `docs/Tool1-File-Guide.docx`: done (see above).
New packages classified in `internal/archtest/archtest_test.go`: none (`internal/export` and `internal/api` were already server-side).
New rules added to `testdata/testpack_nimbus/score.yaml`: none.
No migration, no new table, no new dependency (excelize was already required; `go.mod` not edited by this work).

## Commands run

| Command | Result |
|---|---|
| `gofmt -l .` | prints nothing |
| `go vet ./...` | clean |
| `GOOS=windows GOARCH=amd64 go vet ./...` | clean |
| `go test -count=1 ./...` | every package `ok` (cmd/recon-engine 226 s incl. Test Pack A; internal/api 126 s; internal/archtest ok; internal/export ok; web ok) |
| `go test -count=1 ./internal/archtest/ -v` | all guard tests PASS (TestEngineHasNoNetwork, TestEngineBinaryHasNoNetworkCode, TestEveryPackageIsClassified, ...) |
| `go test -count=1 ./internal/api/ -run 'TestAC\|TestExportAndDelete' -v` | 10 of 10 PASS (TestAC1 to TestAC7d, TestExportAndDeleteRoutesAreDeclared) |
| `go test -count=1 ./internal/api/ ./internal/export/ -v` | PASS; route walk: "walked 38 routes for four roles" |
| `go test -count=1 -tags ui ./web/` | ok |
| `go build -tags ui` (recon-server, Windows cross-build of `./cmd/...`) | OK, written to the scratchpad only; `build\recon-server.exe` not touched; the running server was not restarted |
| `go run ./cmd/recon-dev rulestore-check` | 304 rules, 63 documents checked; 0 problems |
| `go run ./cmd/recon-dev rule-lint` | all 50 rules passed |
| `go run ./cmd/recon-dev profile-test` | all 42 samples passed |
| `cd web && npm test` | 11 files, 113 tests passed (spec013.test.tsx: 6 of 6) |
| `cd web && npm run lint` | exit 0; one warning in `screens/Rules.tsx:389` (react set-state-in-effect), in committed code this work does not touch |
| `cd web && npm run build` | built (chunk-size warning only, pre-existing) |

## Acceptance criteria

| Criterion | Status | Evidence |
|---|---|---|
| AC-1 | not met (partly) | `TestAC1RuleActivationIsPartnerOnly`: Staff and Manager get 403 with the permission on disable, enable, approve, switch and reset; the Partner's disable and enable return 200. Approve, switch and reset reach their handlers for the Partner and return business refusals (422, 404, 422). **No law-table route exists in this build**, so that part is untested. |
| AC-2 | met | `TestAC2DraftRulesStaffManagerPartner`: Staff, Manager, Partner save a draft (201); Read-only refused 403 on create and edit (`draft_rules`). The save checks the rule's fixtures (`cmd/recon-engine/rulesave.go`). |
| AC-3 | met | `TestAC3ReadOnlyCannotRunOrEnter`: Read-only 403 (`run`) on runs, client add and edit, inspect, profiles; Staff run 200 and client add 201. |
| AC-4 | met | `TestAC4RefusalWritesOneEntry`: total audit entries +1; entry `auth.denied`, actor `sunil`, object the route pattern, details exactly `permission` and `role`; chain ok. |
| AC-5 | met (same minute only) | `TestAC5ClientViewsAudited`: two reloads in one minute write one entry; a details call for the same year writes none; a second tax year writes one; actor `tara`; chain ok. The minute boundary is not tested (no clock injection). |
| AC-6 | **not met (PAN part)** | `TestAC6GSTINMaskedForReadOnly`: Read-only response has `***********Z1ZF`, not the full GSTIN; Partner, Manager, Staff have the full GSTIN (passes). **The full PAN cannot be shown: only a keyed hash and the last four characters are stored** (`internal/store/clientdetails.go`). Needs a decision on storing it. Rendered text is covered by `spec013.test.tsx` (Read-only masked, Staff full). |
| AC-7 | met | `TestEveryRouteFollowsTheRoleMatrix` (38 routes, four roles) and `TestRoutesAreOnlyRegisteredFromTheDeclaredList` pass; `TestExportAndDeleteRoutesAreDeclared` checks the three new routes and their permissions. **`TestEveryEndpointDeclaresAPermission` does not exist in the repository**; the two tests above cover the same property. |
| AC-7a | met | `TestAC7aAndAC7bExcelExport`: Partner/Manager download an xlsx; Staff/Read-only 403 naming `export`; sheets = review sheets + "Cannot check" + "Worklist"; every sheet's A1 has the watermark with the user, the export ID and "Watermark:". Unit test `TestWorkbookHasTheSheetsAndTheWatermark`. |
| AC-7b | met | Same test: `export.created` with exactly the keys client_id, tax_year, format, sheets, findings, cannot_check, worklist; actor `meera`; chain ok. |
| AC-7c | met | `TestAC7cDeleteFinding`: Staff/Read-only 403 naming `delete_findings`; empty reason 400 REASON_REQUIRED; Manager deletes (200); Manager no longer sees the finding; Partner sees it marked `deleted`; second delete 409; one `finding.deleted` with the reason. |
| AC-7d | met | `TestAC7dPDFAndQueryLetterNotAvailable`: PDF and query letter 501 `NOT_AVAILABLE_YET` for the Partner; no `export.created` written; Staff PDF 403. |
| AC-8 | met | `spec013.test.tsx` (6 tests): Export Excel and Delete finding shown to Partner and Manager, hidden from Staff and Read-only; Rules screen shows no Edit, Disable, Enable or Approve to Staff or Read-only, and the Partner still sees Disable (positive control). |
| AC-9 | met | `go test -count=1 ./internal/archtest/ -v` PASS; no migration added (`internal/store/migrations` still 0001 to 0008); no new dependency. |
| Guide line | met | Seven rows added to `docs/Tool1-File-Guide.docx` (backup kept). |

Manual browser steps in the Verification section were **not run** (no browser session in this run).

## Invariant check

- Findings show both values, the difference and both sources; no "should": not affected (the UI and export show the engine's fields; the new text has no "should").
- Money in integer paise, no floats: yes. The export formats paise with `money.FormatIndian`; rates use integer basis points.
- No clock, randomness or map-order dependence: the export uses `crypto/rand` for the export ID and the clock for the watermark, both in the server-side package (`internal/export`, not in `deterministicDirs`); the engine is unchanged. Audit details are written from sorted maps by the store.
- No network import on the engine side (archtest passes): yes.
- No real client data added: yes (test names and figures are invented; `ZLVCD9565Z` and `27ZLVCD9565Z1ZF` are synthetic and pass the GSTIN check).

## Open issues

| Issue | Severity (blocking / not blocking) | Who decides (implementer / developer / CA) |
|---|---|---|
| AC-6: the full PAN is never stored, so Staff, Manager and Partner cannot see it in full. Options: store the PAN encrypted (needs SPEC-034 key handling and a migration, which AC-9 forbids), or amend AC-6 to "last four for everyone". | blocking for Done | developer |
| AC-1: no law-table route exists; the criterion's law-table clause cannot be tested. | blocking for Done until the developer confirms it is out of scope for this build | developer |
| The Season board's counts still include deleted findings (the engine's `clients` counts are not filtered). | not blocking; visible inconsistency | developer |
| Deleting a finding is recorded by its ID; if a later draft changes its figures the deletion still applies to that finding ID (decisions behave differently: they reopen). Decide which is wanted. | not blocking | developer |
| `client.view` is debounced in server memory and resets on restart; the minute boundary is not tested. | not blocking | test-engineer may add a clock seam |
| Manual browser steps (Verification, sign in as Read-only and Staff, check the Admin tab's audit entries) not run. | blocking for Done | test-engineer / developer |
| SPEC-012 remains In progress (its manual steps were not run); SPEC-010 not changed. | not blocking for this spec | developer |
| The firm name is blank unless `recon-server --firm` is given; `start-local.cmd` does not pass it. | not blocking; the watermark shows "(firm name not set)" | developer |
| The Excel export is not written to disk; it is returned in the response. Confirm that is the intended behaviour. | not blocking | developer |

## Next agent

- Re-run `go test -count=1 ./internal/api/ -run 'TestAC|TestExportAndDelete' -v` and `cd web && npx vitest run src/spec013.test.tsx`; do not trust the ticks without re-running.
- Re-check the archtest (`go test -count=1 ./internal/archtest/ -v`) after any change to `internal/export` imports.
- The browser verification steps are still to be run by a person after `npm run build` and a rebuilt `recon-server` (`-tags ui`); do not restart the running server from an agent.
- Decide AC-6 (PAN) and AC-1 (law tables) before any status change; SPEC-013 stays In progress until then.
