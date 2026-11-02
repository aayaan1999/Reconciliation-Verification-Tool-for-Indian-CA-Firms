# SPEC-009 — Comparison history: every run of a check type kept as Comparison 1, 2, 3…

| | |
|---|---|
| **Status** | In progress |
| **Phase** | 4 |
| **Depends on** | SPEC-008 |
| **Requirements** | IN-12, RU-53, FD-01, FD-07, NF-03, NF-05 |
| **Technical spec** | §A3, §B3 (`runs`, `run_documents`, `document_versions`), §B11 |
| **Code / content paths** | `internal/store` (`Comparisons`, `RunView`, `RecordsOf`), `cmd/recon-engine/history.go` (`history` command), `internal/api` (one read-only endpoint), `web/src/screens/ClientFile.tsx` (`ComparisonList`), `web/src/api.ts`, `web/src/types.ts` |

## Goal

In the Client file, each type of check (each tab, for example "GST: Sales register vs GSTR-1") keeps every run made for that client and tax year as **Comparison 1, Comparison 2, Comparison 3…**, oldest first. The newest is shown as today. A CA can open any earlier comparison and see exactly what it reported then, which files (and which drafts) it used and which rules ran. Nothing is lost when a new set of files is uploaded and run again. All of it works on one office machine with no internet.

## Why

Today every run is already stored (the `runs`, `run_documents`, `run_rules` and `evaluations` tables, and the run's report in `runs.report_json`), but the Client file shows only the latest run of each type (`LatestRuns`, `internal/store/clients.go`). A new upload hides the earlier result on screen, so staff cannot show a partner what the first set of files gave before the client sent corrected ones.

## Scope

### 1. Numbering

- A comparison is one finished run of one type of check (`runs.comparison_id`) for one client and tax year (one engagement).
- Its number is its place among those runs, ordered by run ID: the first is Comparison 1. The number is worked out when read, never stored, so it cannot drift. Runs are never deleted (NF-05), so a number never changes once given.
- A run of every rule (no type chosen, `comparison_id = ''`) is numbered among runs of every rule, separately from the types.
- Each type is numbered separately: Comparison 3 of the GST sales type and Comparison 1 of the expense ledgers type can exist side by side.
- A run that did not finish (`finished_at = ''`) gets no number and is not listed.

### 2. Store (`internal/store`)

- `Comparisons(engagementID, comparisonID)` returns every finished run of that type, oldest first, each with its number, the local finish date and time, whether rules not yet approved by the CA were included, and its stored report.
- `RunView(engagementID, runID)` returns the document versions a run read (its snapshot in `run_documents`) and their figures, sorted like `CurrentView`, which shares the same reading code. `RecordsOf(engagementID, docs)` gives their rows; `CurrentRecords` now uses it too.
- All are parameterised `SELECT`s. No new table and no migration: everything needed is already stored.
- A run made before its report was kept (empty `report_json`) is listed with "Result not kept for this run" and cannot be opened. It is never re-evaluated to fill the gap, because today's rules may differ from the ones it used.

### 3. Engine command (`cmd/recon-engine`)

- `recon-engine history --db … --client N --tax-year … --comparison <type> --json out.json` lists the comparisons: number, finish time, counts of checks passed, failed and cannot check (from the stored report), whether rules in testing were included, and each document's type, file name and draft number. `--comparison ""` lists runs of every rule.
- With `--no K` it also gives Comparison K's report exactly as stored, and the figures and rows of the drafts it read (the same shape as `facts`), so the screen can explore it as it was.
- An unknown client is `CLIENT_NOT_FOUND`; a number with no kept comparison is `COMPARISON_NOT_FOUND` (exit 2), and the message names the number, never client figures. An unknown type simply has no comparisons.

### 4. Server (`internal/api`)

- `GET /api/v1/clients/{id}/comparisons?tax_year=&comparison=` runs `recon-engine history` and returns the list; adding `&no=K` returns that comparison as well.
- It is read-only and passes the same `guard` as every other API call. The client ID, tax year, type (`^[a-z][a-z0-9_]{0,63}$` or empty) and number are checked before the engine runs (`400` otherwise); an unknown client or number is `404`. It never returns a database ID or a PAN.

### 5. Client file screen (`web/src`)

- Each type's tab has a **Comparisons** panel beside its results, newest first: "Comparison 3 (latest) · 2026-10-29 16:10 · 17 pass, 6 fail, 0 cannot check", then 2 and 1 below it. Each entry is a button, reachable by keyboard.
- Each entry shows the files it used ("sales register: SALES_REG_Aug2026.xlsx, draft 2; gstr1: GSTR1_Aug2026.json, draft 1"), and a note when it included rules not yet approved by the CA.
- **Back to latest** returns to the newest comparison. A new run adds Comparison N+1 and opens it. Changing the type of check or the tax year also returns to the latest.

### 6. Exploring an earlier comparison

- Choosing an earlier comparison opens it in the same results view as the latest, with a banner: "Comparison 1 of 3, run on 2026-10-28 16:10. Read only: this is what the tool reported then, from the drafts it read."
- Everything the latest offers for exploring works the same way, from that comparison's own stored report: the stats, the findings grouped by the document to fix with both figures, the difference and both sources, Cannot check, rules not run, passed checks, the filters (status, rule, category, document, search), the findings index and paging, and the keyboard moves.
- The Documents panel and the figures shown for a document (↑ ↓ and Enter) are those of the drafts that comparison read, not of today's drafts.
- It has no decision buttons and no decision filter. Decisions are made on the latest comparison, where they carry forward by the finding's stable ID as today (FD-01).
- Uploading several files at once (up to 20, as today) still makes one comparison per type chosen, not one per file.

### 7. Offline

- Everything comes from the local SQLite database, through the local engine, on the office machine. Nothing is fetched from the internet: no new network code, no cloud storage, no CDN, web font or remote script on the screen. It works on a machine with the network cable pulled out.
- The engine stays network-free (invariant 5). `recon-engine history` adds no import, and the architecture guard must pass unchanged.

## Out of scope

- **Showing what changed between two comparisons** (new failures, findings no longer failing, changed values side by side). It needs the draft-vs-draft view (RC-06, §B7) and comes in a later spec.
- **Deleting or renaming a comparison.** Runs are part of the audit trail (NF-05) and are never deleted from the screen. Client-wise deletion for DPDP (NF-09) comes with Module 14.
- **Exporting an earlier comparison** to Excel or PDF (FD-08): comes with exports.
- **Coded fields and law tables**, reserved for SPEC-009 by SPEC-005 and SPEC-008, move to SPEC-010.

## Design notes

- An earlier comparison is shown from its **stored report**, not by running the rules again. Rules, profiles and decisions may have changed since, and re-running would show something the tool never reported then. The stored report is byte-stable engine output, so showing it again is deterministic (NF-03).
- The comparison number is derived (the run's place, by run ID, among the finished runs of one engagement and type), so no new column or migration is needed, and two screens can never disagree about a number.
- The date and time on the list come from `runs.finished_at` in the store, shown in the office computer's local time like the date already shown today. The engine's report itself still holds no time or run ID.
- A run uses the client's **current view** (the latest draft of each document, §B3), so Comparison 2 may use draft 2 of the sales register and draft 1 of GSTR-1. The file list says exactly which, from `run_documents`.
- When this spec is built, §B11 of the technical spec is updated with the two endpoints as built, in the same commit.

## Acceptance criteria

- [x] AC-1 — Runs of one type for one client and tax year are listed as Comparisons 1, 2, …, oldest first. A run of another type in between does not change their numbers. (`TestComparisonHistory`)
- [x] AC-2 — Each listed comparison shows its date and time, its pass, fail and cannot-check counts, whether rules in testing were included, and each document's file name and draft number, all matching what was run. (`TestComparisonHistory`, `history.test.tsx`)
- [x] AC-3 — Opening Comparison 1 after Comparison 2 used a new draft shows Comparison 1's findings exactly as first reported: its report is byte-equal to the one stored by that run. (`TestComparisonHistory`)
- [x] AC-4 — An earlier comparison is read only: the screen shows no decision buttons, and the API has no way to decide on it (decisions go only to the latest result). (`history.test.tsx`)
- [x] AC-5 — Decisions made on the latest comparison still carry forward to the next run by stable finding ID, as before. (`TestFindingDecisions`, `TestDecisionsThroughServer`, `decisions.test.tsx`, unchanged and passing)
- [x] AC-6 — A run with no stored report is listed as "Result not kept for this run" and cannot be opened, and nothing is re-evaluated. (`TestComparisonHistory`)
- [ ] AC-7 — Uploading 10 files in one run with two types chosen adds exactly one comparison to each of those two types. (manual, step 5)
- [x] AC-8 — A number with no kept comparison gives `COMPARISON_NOT_FOUND` (exit 2 on the command line, 404 from the API), and an unknown client 404, with no client figures in the message; a malformed type or number is refused with 400 before the engine runs. (`TestComparisonHistory`, `TestComparisonHistoryThroughServer`)
- [x] AC-9 — The architecture guard passes unchanged (no new network import in the engine), and the screen loads nothing from the internet (no `http://` or `https://` resource in `web/src`).
- [ ] AC-10 — The list and the read-only view work by keyboard alone. (manual, step 4)
- [x] AC-11 — An earlier comparison is explored like the latest, from its own report: the same stats, findings with their arithmetic and sources, filters, index and paging, and the figures of the drafts it read, not today's. (`TestComparisonHistory` for the drafts' figures, `history.test.tsx` for the view)
- [x] Every file this spec created is added to `docs/Tool1-File-Guide.docx` in plain language.

## Verification

1. `go test -count=1 ./cmd/recon-engine/ -run TestComparisonHistory -v` and `go test -count=1 ./internal/api/ -run TestComparisonHistoryThroughServer -v` (AC-1 to AC-3, AC-6, AC-8, AC-11).
2. `go test -count=1 ./internal/archtest/ -v` (AC-9).
3. In `web/`: `npm run lint`, `npm test`, `npm run build` (`history.test.tsx`: AC-2, AC-4, AC-11).
4. Manual, with the network turned off: start with `start-local.cmd` and add the synthetic GST client (tax year 2026-27). Run "GST: Sales register vs GSTR-1" with `content/profiles/generic_xlsx/sales_register/v1/samples/SALES_REG_Aug2026.xlsx` and `testdata/gst_pack_aug2026/GSTR1_Aug2026.json`, then with `build/xltest/09_hidden_row.xlsx`, then with `build/xltest/05_blank_row_in_middle.xlsx` (each a new draft of the register). Check that Comparisons 1 to 3 are listed with register drafts 1, 2 and 3; that Comparison 1 opens read only with its original 6 findings, and its filters, findings index and document figures work; and that **Back to latest** returns to Comparison 3. Do it all with the keyboard (Tab, Enter) (AC-10).
5. Manual: upload 10 files at once with two types ticked, and check that each of the two types gains exactly one comparison (AC-7).
6. The test-and-verify gates (`gofmt -l .`, vet on host and Windows, all Go tests, Test Pack A unchanged at 22 of 30 with 0 false positives).

## As built (29 October 2026, not yet committed)

- One engine command, `history`, both lists and opens (`--no`), instead of a separate `runs` command and a `result` flag; and one API endpoint with query parameters instead of two path forms, so runs of every rule (type `""`) are covered too.
- `CurrentView` and `RunView` share one reading function; `CurrentRecords` and the history share `RecordsOf`; `facts` and the history share `engagementFacts`. No migration.
- The screen ignores a server without the endpoint (the list is simply not shown), so the latest result still works against an older server.
- 29 October 2026, checked again on the 29th: `TestComparisonHistory`, `TestComparisonHistoryThroughServer`, the architecture guard and `npm test` (78 tests) pass. AC-7 is partly checked: a two-type upload of the four synthetic GST files (sales register, GSTR-1, purchase register, GSTR-2B) on a test database added one comparison to each type, and a refused upload added none. Still open: 10 files in one run (the testdata has only four GST files, and the other sample files are not GST ones), and AC-10 (keyboard only), which has not been run by hand.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
