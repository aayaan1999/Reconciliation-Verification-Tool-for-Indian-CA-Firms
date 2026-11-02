# User journeys

This page walks through the main journeys of CA staff and partners, step by step, and says for each step what exists today and what is planned. It is grounded in the build units in [specs/](../../specs/README.md) (SPEC-003 to SPEC-006), the commands in `cmd/recon-engine`, the server in `cmd/recon-server` and the screens in `web/src/screens/`, and in the intended workflow of the product spec v0.2 §11.2 ([recon-tool-spec.md](../../recon-tool-spec.md)), requirements v0.3 §8 ([requirements-spec-tool1.md](../../requirements-spec-tool1.md)) and the technical spec §B7 and §B11 ([tool1-technical-spec.md](../../tool1-technical-spec.md)).

All client names, PANs and figures used on the screens and in the test packs are invented. "Built" means it works in the current code; "Planned" means it is designed but not built.

## How the screens are reached today

- `recon-server --listen 127.0.0.1:8080 --db build/ui.db` serves the screens and a test API (SPEC-004 part 6). It listens on this computer only unless another address is given. There are no logins yet (planned, Module 16, tech spec §B12).
- Every action on a screen becomes one `recon-engine` command, run as a separate program; the server never evaluates a rule itself (tech spec §B11).
- Without a server (Vite dev server, tests), the screens show sample data from `web/src/sample/` and say so.
- Keyboard first: `1` Season board, `2` Worklist, `3` Rules, `/` search, `?` shortcut sheet, `Esc` back (`web/src/screens/Shortcuts.tsx`).

## Journey 1: Partner checks the season (Season board)

| Step | What happens | Status |
|---|---|---|
| 1 | The partner opens the browser on the office address and lands on the Season board | Built (SPEC-004 part 3, `SeasonBoard.tsx`) |
| 2 | One row per client for the tax year: counts of findings, Cannot check items and the date of the last run, summed over the latest result of each comparison type | Built (SPEC-004 parts 7 and 9; `recon-engine clients`) |
| 3 | Open findings by severity, oldest unanswered query, days to deadline (§11.2) | Planned (tech spec §B11 `GET /engagements`) |
| 4 | Enter on a row opens that client's Client file | Built |

## Journey 2: Staff add or edit a client

| Step | What happens | Status |
|---|---|---|
| 1 | "+ Add client" (key `n`) opens the form: legal name, trade name, PAN, constitution, CIN/LLPIN, TAN, GSTINs, address, contact, accounting software, notes | Built (SPEC-004 part 8.1, `ClientForm.tsx`) |
| 2 | Each identity number is checked (formats; each GSTIN's check character, state code and PAN part); every problem is shown at once | Built (`internal/client`) |
| 3 | The engine stores the client; the PAN goes on standard input and is kept as a keyed hash plus its last four characters | Built (`recon-engine client-add`, `client-update`, `client-get`) |
| 4 | "Edit client" (key `e`) on the Client file changes any field; documents and runs stay with the client | Built |

## Journey 3: Upload files and run the checks (Client file)

| Step | What happens | Status |
|---|---|---|
| 1 | In the Client file's "Upload and run" panel, staff tick one or more comparison types (for example "Sales register vs GSTR-1"); each lists its rules, all ticked to start | Built (SPEC-004 parts 8.2 and 9; `content/comparisons/comparisons.yaml`) |
| 2 | Staff choose the tax year and, if wanted, "Include rules not yet approved by the CA" (testing rules) | Built |
| 3 | Staff pick JSON or CSV files (at most 20 files, 64 MB each) | Built for JSON and CSV (SPEC-004, SPEC-006 part 3). Excel, Word, Tally XML and PDF planned (tech spec §B16 phases 3 and 5) |
| 4 | The engine detects the format of each file (exactly one profile, or the file is refused), reads every figure with its source, stores a new draft, takes the client's current view and evaluates the chosen rules | Built (`recon-engine run --client <id> --comparison <id> --rules <IDs>`) |
| 5 | A file whose PAN is not the client's is refused (`CONFLICTING_VALUES`) | Built (SPEC-004 part 7) |
| 6 | The Client file shows one tab per comparison type: documents read and missing, findings grouped by the document to review, Cannot check, Not run (not selected) and passed checks | Built |
| 7 | Uploads go to a temporary folder deleted after the run | Built. The encrypted file vault and duplicate detection by fingerprint are planned (IN-13, tech spec §B2.1) |
| 8 | Run in the background with progress, from a DB-backed job queue | Planned (tech spec §A3, `internal/jobs` is empty) |

Each finding shows both values, the difference and both sources (file and JSON path). It never says what the right figure is.

## Journey 4: Decide findings

| Step | What happens | Status |
|---|---|---|
| 1 | Staff select a finding (`j`/`k`) and choose **Fix** (`1`), **Not an issue** (`2`, a note is required) or **Ask client** (`3`), typing their name | Built (SPEC-006 part 3, `Decisions.tsx`; `recon-engine decide`) |
| 2 | The decision is kept against the finding's stable ID with note, name, time and value fingerprint, in an append-only history | Built (migration `0006`, table `finding_decisions`) |
| 3 | On a later run, the same values keep the decision; changed values make the finding Open again, showing the earlier decision as made on other values | Built (FD-04 behaviour) |
| 4 | A Decision filter (Open, Fix, Not an issue, Ask client) and open counts per tab | Built |
| 5 | The full lifecycle (Awaiting re-check, Resolved, Reopened, Carried forward, proposed Withdrawn) applied by the system after each run | Planned (tech spec §B7.2 and §B7.3; needs CA sign-off, risk T6) |
| 6 | The correction itself is made in the source software (Tally, tax suite, GST return) and the corrected file uploaded again | Outside the tool, by design (§3.2) |
| 7 | Client query letter from the Ask client items; Excel and watermarked PDF exports | Planned (FD-08, tech spec §B13) |

## Journey 5: Classify new and vanished heads (Worklist)

| Step | What happens | Status |
|---|---|---|
| 1 | Staff upload last year's and this year's monthly expense ledger CSVs; last year's is stored as last year's document | Built (SPEC-006 part 3) |
| 2 | The engine compares the two years and lists new heads, vanished heads, broken monthly recurrence and step changes, with both years' totals and month counts | Built (`internal/modules/newheads`, thresholds in `content/worklist/newheads.yaml`) |
| 3 | The Worklist screen shows every client's items for the tax year; staff write a classification (clause, GST status, TDS section, 43B category) and their name | Built (`Worklist.tsx`; `recon-engine worklist`, `classify`) |
| 4 | The tool never fills in a classification; worklist items are never findings | Built (project invariants 1 and 2) |
| 5 | Unmapped heads; reading ledgers from the Tally trial balance; storing classifications as facts for later rules; an optional local-model suggestion that staff confirm | Planned (tech spec §B8) |

On Test Pack A the two ledger CSVs give exactly F27 to F30.

## Journey 6: Review the rules (Rules screen)

| Step | What happens | Status |
|---|---|---|
| 1 | The Rules screen lists every stored rule: the 304 rule-book rules (marked "Described only, not run yet") and the runnable engine rules | Built (SPEC-004 part 6, `Rules.tsx`; `recon-engine rules`) |
| 2 | Filters by tax year, type, subject, category, status, source, and search on ID or name | Built |
| 3 | The CA builds or edits a rule on a form, tests it on a past client, and a Partner activates it | Planned (Rule Builder, RU-20 to RU-25, tech spec §B9, phase 4b) |
| 4 | The CA reviews the rule-book problem list (`recon-dev rulestore-problems`, written to `docs/Recon_Tool_YAML_Rule_Store/review/`) | Built (SPEC-004 part 4); the CA's decisions are **Open** |

## Journey 7: Teach a new JSON layout (mapping screen)

| Step | What happens | Status |
|---|---|---|
| 1 | A file no profile recognises is refused with `PROFILE_NO_MATCH`; the Client file offers "Teach this layout" | Built (SPEC-004 part 8.4, `ClientFile.tsx`, `TeachLayout.tsx`) |
| 2 | The tool lists every field of the file with an example value (`recon-engine inspect`) | Built |
| 3 | Staff pick the document type, the rows (for registers), the field for each column, the period and GSTIN fields, and one field and value that identify the layout | Built |
| 4 | A preview shows the rows it would read (`profile-create --check-only`); on save, the engine writes an office profile into the office profiles folder, with the file beside it as its sample | Built (`recon-engine profile-create`) |
| 5 | From then on, files in that layout are recognised like any other; office profiles are marked as such in the validation report | Built |
| 6 | Teaching Excel or Word layouts; a Format Register screen | Planned (Build Plan module 10; IN-25) |

## Not covered yet

Law tables screen, Format Register, Admin (users, backups, audit-log viewer), rule-pack install by USB, and draft-vs-draft comparison are planned (FD-07, RC-06, tech spec §B10, §B16). Voice disposition is deferred (requirements §13 phase 8).
