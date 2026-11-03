# SPEC-004 — Client JSON files read and stored with their source, first screens with sample data, rule book problem list

| | |
|---|---|
| **Status** | In progress (parts 1–9 Done; part 10, disabling a client, added on 2 November 2026) |
| **Phase** | 1 (Build Plan modules 2, 4, 5 and 9; dates 15–20 October 2026) |
| **Depends on** | SPEC-003 (engine run, database), SPEC-002 (developer tools; finished as part 5 of this spec) |
| **Requirements** | IN-01, IN-10..14, IN-20..23, FV-01, FD-02, FD-06, RU-01, RU-02, NF-01, NF-03, NF-04 |
| **Technical spec** | §A6.1, §A6.2, §B1.1, §B2.1–§B2.3, §B4.2, §B4.7, §B11 (screen list), §B14, §B15 |
| **Code / content paths** | `content/profiles/`, `content/vocabulary/`, `cmd/recon-engine` (`facts`), `internal/store`, `web/src/`, `web/src/sample/`, `cmd/recon-dev` (`rulestore-problems`), `docs/Recon_Tool_YAML_Rule_Store/review/` |

## Goal

The tool reads every tax-software JSON file the firm uses, keeps each figure exactly (to the paisa) with the file and place it came from, and can list those figures with their source. The three working screens open in a browser with sample data so the firm can comment on the look and flow. The CA has a list of the problems found in the converted rule book.

## Scope

### 1. Reading client files saved as JSON

- One format profile per tax-software JSON document the firm uses (§A6.2, §B2.2), each with at least two samples and hand-checked expected facts (§B2.3, IN-22). New figure names go into `content/vocabulary/facts.yaml` with a vocabulary version bump and a regenerated `docs/fact-vocabulary.md`.
- Which documents and which software version: from the firm's answer due 17 October (Build Plan, "What we need from the firm"). Until the real anonymised samples arrive, profiles are written against synthetic files in the same shape, marked synthetic in `source_system`. A real sample replaces the synthetic one as soon as it arrives. If it is Excel or PDF instead, that adapter is a separate spec.
- Amounts are kept exactly as written, in integer paise, with no rounding. More than two decimals, a figure written two ways in one file, or a number that is not a number is a validation error in the file's report, never a silent correction.
- A damaged file is refused with `PARSE_ERROR` plus line and column. A file no profile recognises is refused with `PROFILE_NO_MATCH`, and one that two profiles match with `PROFILE_AMBIGUOUS`. The tool never guesses.

### 2. Saving every figure with its source

- Every figure read is stored in `facts` with its document version, file name and exact place (JSON path), as SPEC-003 already does for one file. This spec covers every profile from part 1.
- New command `recon-engine facts --db <file> --input <folder> [--json <file>]` lists the current view's figures for that engagement, one line per figure: document, figure name, period, value (Indian grouping, paise shown) and source (`file path`). It is sorted and has no IDs or times, so it is byte-stable. `--json` writes the same content as canonical JSON, for the screens.
- Round trip: for every profile sample, the figures read back from the database equal the expected facts file (value, period, scope and locator).

### 3. Screens open in the browser, with sample data

- The three working screens in `web/` (React + TypeScript), filled from sample data files in `web/src/sample/`. Their shape is the engine's `--json` output (`recon-engine run --json` and `recon-engine facts --json`), so connecting them to the real engine later does not change the screens.
  - **Season board:** every client for the tax year, with counts of findings and Cannot check items and the date of the last run. Opening a client goes to its Client file.
  - **Client file:** one client's documents (file, recognised format, figures read, not read, errors), the findings grouped by the document to review, each with both figures, the difference and both sources, and the Cannot check list ("Cannot check: figure not found in <document>").
  - **Worklist:** open items for staff to work through (new, vanished and changed ledger heads), with a classification column.
- Keyboard first: every screen can be reached and used without a mouse (arrow keys move between rows, Enter opens, Esc goes back). Screen text follows the wording rules: no banned phrase (`rules.BannedPhrases`) and never a stated correct value.
- Every sample name, PAN and figure is invented and clearly marked "Sample data".
- Runs offline: nothing is loaded from the internet (no web fonts, no CDN). `npm run build` output is embedded with `-tags ui` as today.
- Layouts are ready for the firm's comments by 16 October.

### 4. Checking the converted rule book

- On 15 October, re-run `rulestore-check` against the agreed format (`RULE_TEMPLATE.yaml`, `DOCUMENT_TEMPLATE.yaml`). Fix every formatting problem in the conversion (`rulestore-import`, never by hand-editing generated files), then run the check again until it reports none.
- New developer tool `go run ./cmd/recon-dev rulestore-problems` writes the list of rule problems for the CA, with one row per problem. Columns: rule ID, rule name, problem type, detail and the CA's decision (left blank). Problem types, from today's store:
  - verification status `unverified` (24 rules) or `conflicting` (2 rules);
  - a document field a rule uses that is not in the researched catalogue (`origin: "new (design)"`, 53 fields);
  - a reconciliation rule with no comparisons or no keys;
  - anything else the conversion could not read cleanly, such as text that did not split into steps, categories or parameters.
- Written to `docs/Recon_Tool_YAML_Rule_Store/review/rule-problems.xlsx` (for the CA) and `rule-problems.md` (for the repository), sorted by rule ID then problem type. The same store always gives the same files.
- The list goes to the CA, whose review is due by 20 October (Build Plan).

### 5. Developer checking tools

- Finish SPEC-002: every tool in its list works in a fresh shell, and CI runs the Go checks (which include the rule-store check), the web lint, tests and build on every push. Mark SPEC-002 Done when its criteria are met.
- Add `rule-lint` (every rule has a pass and a fail fixture) and `profile-test` (every profile matches its expected facts) to CI as separate named steps, so a broken rule file or profile is named in the CI log.

### 6. Testing the engine from the browser, with filters (added 14 October 2026)

Goal: upload JSON files in the browser, choose a tax year, run the checks, and filter the results and the rules by tax year, category, rule and status, with no command line.

- **Server.** `recon-server --listen 127.0.0.1:8080 --db build/ui.db` binds to this computer only unless another address is given. There are no logins yet (Module 16). For each request it runs `recon-engine` as a separate program (§A3.1); the server never evaluates rules itself, and the engine keeps no network code. Endpoints:
  - `GET /api/v1/rules`: every stored rule (the 304 rule-store rules and the engine rules) with ID, title, type, subject, category, tax years, validity dates, source and status.
  - `GET /api/v1/tax-years`: the tax years the rules cover.
  - `POST /api/v1/runs` (multipart: one or more `.json` files, `tax_year`, `include_testing`): runs `run --json` and `facts --json` on the uploaded files and returns both. An input or rule error comes back with its stable code (for example `PROFILE_NO_MATCH`) and the message.
  - Limits: JSON files only, at most 20 files of 64 MB each and 256 MB in all. Uploads go to a temporary folder that is deleted after the run, and runs are taken one at a time.
- **Engine.** `recon-engine rules --db <file> [--json <file>]` lists the stored rules and their status, read only. The run report's JSON also lists the checks that passed (`passed`) and each result's rule category, so the screens can filter by status and category.
- **Screens.**
  - **Client file:** an "Upload and run" panel (file picker, tax year, an "Include rules not yet approved by the CA" switch, a Run checks button) shows the result in the existing layout, with filters for status (Pass / Fail / Cannot check), rule, category and document, plus a text search.
  - **Rules** (the read-only start of the Rule Builder screen, Build Plan module 13): every rule, filtered by tax year, type, subject, category, status, source and a search on ID or name. Each rule-store rule is marked "Described only, not run yet". A rule with no tax years or dates counts for every year and is labelled "not dated".
  - Keyboard first: `/` moves to the search box and Esc clears the filters. No banned phrase, and nothing loads from the internet.
  - When no server is running (the Vite dev server, or the tests), the screens fall back to the sample data and say so.

### 7. Adding clients, and comparing the client's accounts with our computation (added 15 October 2026)

Goal: add a new client from the Season board, then upload the report the client gives us (their accounts: the profit and loss statement from their accounting software) and the report we prepare (the tax computation from our tax suite), at the same time or separately, and see every figure that differs between them.

- **Clients.** An "Add client" form on the Season board (name, PAN, tax year). The engine stores the client (`recon-engine client-add`, with the PAN read from standard input, never from the command line) in a `clients` table, keeping only a keyed hash of the PAN (HMAC-SHA-256 with a random key held in the database) and its last four characters (§B1.3). The full PAN is never stored or shown. The client's engagement for the tax year keeps its documents across uploads, so the latest draft of each document makes up the current view.
- **Runs for a client.** `recon-engine run --client <id>` uses that client's engagement. A file whose PAN is not the client's gets a validation error (`CONFLICTING_VALUES`) and its figures are not used. The run stores its report, and `recon-engine result --client <id> --tax-year <y>` returns the latest report with the current figures. A client run's report lists every document in the current view, not only the files just uploaded.
- **Sample "client accounts" document.** A new synthetic document type, financial statements (`fs.` figures): a generic accounting export with the profit and loss statement (revenue, expenses, depreciation, profit before tax, tax expense, profit after tax). It has two samples matching the two tax-suite computation v3 samples: one agrees, and one has a planted Rs 27,000 transposition in profit before tax. Computation v3 gains `depreciation_as_per_books`.
- **Comparison rules.** `RECON-PL-001` compares profit before tax in the accounts with net profit as per the P&L in the computation. `RECON-PL-002` compares depreciation in the accounts with depreciation as per books in the computation. Both use `equal` with a Rs 1 tolerance, have pass and fail fixtures, and load as testing.
- **Server and screens.** `GET/POST /api/v1/clients`, `GET /api/v1/clients/{id}/result`, and `client_id` on `POST /api/v1/runs`. With a server running, the Season board lists the clients in the database with their latest counts and has "Add client" (key `n`); opening a client shows its latest result, and an upload adds to that client's documents.

### 8. Client master, comparison types, GST invoice matching and new JSON layouts (added 15 October 2026)

Goal: the CA adds a client with all its details, picks the type of comparison, uploads several JSON files and gets an invoice-by-invoice comparison; files in a layout the tool has never seen are taught to it once on a mapping screen, without code. Decisions taken with the user on 15 October 2026: compare the registers with the portal returns **and** show a sales-vs-purchase summary; the CA picks a **type of check**, and the file type is recognised automatically; new layouts are taught on a **mapping screen**; **JSON only** in this round (Excel and Word later, Build Plan module 10).

Reference data: the synthetic GST pack for August 2026 (Deccan Byte Systems Pvt Ltd, fictional), copied into `testdata/gst_pack_aug2026/` (the JSON files and the answer key). The company's books are correct; the portal returns carry 23 planted errors listed in `ANSWER_KEY.xlsx`.

**8.1 Client master (stage B).**
- "Add client" and "Edit client" forms: legal name, trade name, PAN, constitution (company, LLP, partnership firm, proprietorship, individual, HUF, trust, AOP/BOI, other), date of incorporation, CIN or LLPIN, TAN, one or more GSTINs (with state and registration type: regular, composition, SEZ, ISD, TDS/TCS, non-resident, casual; one marked primary), registered address (two lines, city, state, PIN code), contact person, email, phone, accounting software, notes.
- Checks: PAN, TAN, CIN/LLPIN, PIN code, email and phone formats; each GSTIN's format and check digit, its state code, and that its PAN part (characters 3 to 12) is the client's PAN.
- A change of PAN is checked for duplicates like a new client. Every field can be edited; the engagement's documents and runs stay with the client.
- Privacy: the PAN alone is still kept only as a keyed hash plus its last four characters, but a GSTIN contains the PAN, so a client with a GSTIN has its PAN readable in the database. The database is to be encrypted at rest (Module 14, ADR 0002); until then it is a plain file on the office computer.

**8.2 Comparison types (stage C).** The CA picks what to compare; each type names the documents it needs and the rules it runs (`content/comparisons/comparisons.yaml`, data, not code):
- Tax suite: computation checks (computation, tax audit report; RECON-DEP-001, RECON-DEP-002)
- Client's accounts vs our computation (financial statements, computation; RECON-PL-001, RECON-PL-002)
- GST: Sales register vs GSTR-1, B2B invoices (RECON-GST-001)
- GST: Purchase register vs GSTR-2B, B2B invoices (RECON-GST-013)
- GST: Sales vs purchase summary (no rule: a side-by-side summary of output and input tax for the month; the two are different transactions and are never expected to be equal, so this produces no finding)

`recon-engine run --comparison <id>` runs only that type's rules. The Client file shows the documents the type needs, which are present, and which are still missing, and accepts several files in one upload.

**8.3 GST invoice matching (stage A).**
- New document types: sales register, purchase register (the company's books), GSTR-1 and GSTR-2B (portal). New record types, one per document: B2B invoices of each, with invoice number, counterparty GSTIN, date, place of supply, rate, taxable value, IGST, CGST, SGST, cess and invoice value.
- Format profiles for the pack's four JSON layouts. The profile language gains what nested GST files need: a column from the row's parent object (`^.ctin`), amounts summed over item lines (`.itms[*].itm_det.txval`), dates written `dd-mm-yyyy`, periods written `mmyyyy`, the tax year worked out from the period, the PAN read from the GSTIN, and row filters (only B2B invoices, not cancelled).
- Records are stored with their source (migration 0003) and a new check type, `match_records`, compares two record types key by key: matched; only in one document; duplicated in one; amounts that differ beyond the tolerance; fields that differ (GSTIN, place of supply, rate, date); and an unmatched pair from the same counterparty with the same date and the same amounts but a different invoice number is reported once as "invoice number differs", never guessed beyond that. Lines of one invoice are added together first.
- Rules `RECON-GST-001` (sales register vs GSTR-1 B2B, keyed on invoice number) and `RECON-GST-013` (purchase register vs GSTR-2B B2B, keyed on supplier GSTIN and invoice number), each with pass and fail fixtures, loaded as testing.
- Expected on the pack: E01 (only in books), E02 (taxable value and IGST differ), E03 (GSTIN differs), E04 (place of supply and tax heads differ), E05 (duplicated in GSTR-1), E06 (rate and IGST differ); F01 (not in GSTR-2B), F02 (taxable value and IGST differ), F03 (invoice number differs), F04 (not in the books), F06 (date differs); every other invoice matched. Not in this round: E07–E12 (credit notes, B2C, exports, HSN and document summaries), F05 (ITC availability) and G01–G05 (GSTR-3B); they need more rules and are listed for later.
- A finding about a record shows, for each figure that differs, both values, the difference and both sources (file and JSON path of the row).

**8.4 New JSON layouts: the mapping screen (stage D).** For a file no profile recognises, the Client file offers "Teach this layout": the tool lists every field in the file with an example value; the user picks the document type, the rows (for registers), which field is each column (invoice number, GSTIN, taxable value ...), the period and GSTIN fields, and one field and value that identify the layout. The server asks the engine to write a new format profile (`recon-engine profile-create`) into the office profiles folder (`data/profiles/office/...`); the profile must read the uploaded file with no errors before it is saved, and the file is kept beside it as its sample. From then on files in that layout are recognised like any other. Office profiles are marked as such in the validation report.

### 9. Choosing rules, and several comparison types per client (added 15 October 2026)

Goal: after picking a type of check, the CA chooses which of its rules to run, and one client can be compared on several types (for example sales vs GSTR-1 and purchases vs GSTR-2B), each keeping its own result. Decisions taken with the user on 15 October 2026: the "All checks" choice is removed from the screen; every rule starts ticked on each run (nothing is remembered between runs); several types are picked with checkboxes, each with its rules listed under it.

**9.1 Rule selection.**
- In the Client file's "Upload and run" panel, the types of check are checkboxes. Ticking a type lists that type's rules (and only those) under it, with ID, name and status (Active / Testing). Every rule starts ticked; the CA can untick any.
- A Testing rule can be ticked only when "Include rules not yet approved by the CA" is on. A ticked type must keep at least one rule ticked. A type with no rules (the sales vs purchase summary) shows no rule list.
- The engine runs only the ticked rules: `recon-engine run --comparison <id> --rules <ID,ID>`. `--rules` without `--comparison` is refused (`RULES_NEED_COMPARISON`), a rule that is not part of the type is refused (`RULE_NOT_IN_COMPARISON`), and an empty list is refused (`NO_RULES_SELECTED`).
- The report lists every rule of the type that was left out as "Not run: not selected" (`not_selected` in the JSON), so a partial run never reads as a complete one. Left-out rules are not counted as checks.

**9.2 Several comparison types per client.**
- One upload can run several types: `POST /api/v1/runs` takes `comparison` once per type and, for each, `rules:<comparison id>` with the ticked rule IDs separated by commas. The server checks every type and rule before running anything, then runs the engine once per type, one after the other. The screen always sends at least one type; with none, the API still runs every rule (for the command line and older callers).
- Each run records its type (migration `0005`: `runs.comparison_id`). A client keeps its latest result **per type**, so running one type no longer replaces another type's result. `recon-engine result` and `GET /api/v1/clients/{id}/result` return `results`: the latest run of each type, with its date and report.
- The Client file shows one tab per type, each with its documents (read / not uploaded yet), counts, findings, Cannot check, Not run and passed lists, and the date of that type's last run. Left and right arrows move between tabs.
- The Season board shows, for each client, the sum of the counts of the latest result of each type, and the date of the most recent run.
- Sample data: the sample company Deccan Byte Systems shows two tabs, sales vs GSTR-1 and purchases vs GSTR-2B.

### 10. Disabling a client (added 2 November 2026)

Goal: a client the firm no longer works for, or one added by mistake, can be taken off the Season board and the Worklist without losing anything. Requested by the user on 2 November 2026. It follows the same pattern as users (SPEC-012) and rules (SPEC-010): **disable, never delete**, a reason every time, and an audit entry.

**10.1 What disabling does.**
- A Partner disables a client from its Client file (**Disable client**) and gives a reason. A reason is also needed to **Enable again**. Permission `disable_clients`, held by the Partner only (§B12.1); the server refuses anyone else with 403.
- Nothing is deleted: the client's details, GSTINs, documents, drafts, runs, comparisons, decisions and Worklist classifications all stay in the database. Enabling the client again restores it exactly as it was.
- A disabled client is left off the Season board and the Worklist. The Season board has a **Show disabled clients** switch that lists them as well, each marked **Disabled**, so a Partner can find one and enable it again.
- A disabled client's Client file still opens, read only: its results, history and figures are shown as they were, with a banner "Disabled on <date> by <name>: <reason>". Its results can still be exported.
- While a client is disabled, every change to it is refused with `CLIENT_DISABLED` (HTTP 409): uploading and running checks, editing its details, deciding findings, deleting findings, classifying Worklist items. The screen hides these actions and shows the banner instead.
- The PAN stays taken: adding a new client with a disabled client's PAN is refused with `CLIENT_EXISTS`, and the message says that client is disabled and can be enabled again.

**10.2 Storage and commands.**
- The `clients.status` column from migration `0002` already allows `active` and `archived`; a disabled client is stored as `archived` (no table rebuild). Migration `0010` adds who disabled it, when and why (`status_reason`, `status_by`, `status_at`), shown in the banner.
- New engine command `client-disable` and `client-enable` (`--db --client`; standard input `{"by", "reason"}`), refusals `REASON_REQUIRED`, `CLIENT_NOT_FOUND`, `ALREADY_DISABLED`, `NOT_DISABLED`. Each writes one `audit_log` entry (`client.disable`, `client.enable`) with the reason, in the same transaction.
- `clients --all` also lists disabled clients; every listed client and `result` carry `status` (`active` or `disabled`) and, when disabled, `disabled_on`, `disabled_by` and `disabled_reason`.
- Server: `POST /api/v1/clients/{id}/disable` and `POST /api/v1/clients/{id}/enable` (`{"reason"}`; the name comes from the signed-in user), and `GET /api/v1/clients?all=1`.

## Out of scope

- Excel, PDF, Word, Tally and portal adapters (Build Plan modules 4 and 10, separate specs).
- Connecting the screens to the real engine through `recon-server` and the API (§B11), uploads from the browser, and the "Run checks" button.
- The Rule Builder screen, logins and roles.
- Changing a rule's content or switching rules on. The CA decides; this spec only lists problems.
- Turning rule-store rules into runnable `recon.rule/1` rules.

## Design notes

- `recon-engine facts` reads only and opens no write transaction. It uses the same engagement key as `run` (the absolute `--input` path), and with no stored engagement it exits `2`.
- The sample data in `web/src/sample/` is generated from real engine output on the synthetic packs (`recon-engine run --json` on `testdata/run/*`) plus hand-made clients for the Season board. A test fails if a sample file no longer matches the shape the engine writes.
- The `.xlsx` problem list is written with `excelize`, already a dependency. `recon-dev` is never shipped, so this adds nothing to the engine.
- If part 1 changes the vocabulary or the profile format, update `tool1-technical-spec.md` §B2/§B3 in the same change.

## Acceptance criteria

- [x] AC-1 — Every JSON document type the firm named has a profile with at least two samples (real and anonymised, or synthetic and marked so), and `go run ./cmd/recon-dev profile-test` passes for all of them.
- [x] AC-2 — Amounts are read to the paisa with no rounding. A sample with three decimals, one with a figure written twice with different values, and one with a non-number each give a validation error naming the place; none gives a figure.
- [x] AC-3 — A damaged JSON file exits `2` with `PARSE_ERROR`, line and column. An unknown one exits `2` with `PROFILE_NO_MATCH`. A file matching two profiles exits `2` with `PROFILE_AMBIGUOUS`. Nothing is written.
- [x] AC-4 — For every profile sample, the facts read back from the database equal the expected facts, including the locator.
- [x] AC-5 — `recon-engine facts` lists every stored figure with its file and place. Two runs give byte-identical output, and the output has no IDs or times.
- [x] AC-6 — `npm run build` succeeds, and the Season board, Client file and Worklist open in the browser from the embedded build with sample data, with no request leaving the machine.
- [x] AC-7 — The Client file shows each sample finding with both figures, the difference and both sources, grouped by the document to review, plus the Cannot check list. A test checks that a rendered screen contains no banned phrase.
- [x] AC-8 — Every screen can be used with the keyboard alone. Vitest tests cover moving between rows, opening a row and going back.
- [x] AC-9 — A Vitest test fails if a sample data file does not match the engine's `--json` shape.
- [x] AC-10 — `rulestore-check` reports 0 problems after the conversion fixes. No generated rule file was edited by hand: `rulestore-import` followed by `git diff` shows no change.
- [x] AC-11 — `rulestore-problems` writes `rule-problems.xlsx` and `rule-problems.md`, covering at least the unverified, conflicting and design-field problems. Running it twice gives identical files.
- [x] AC-12 — SPEC-002 is Done. CI runs `rule-lint`, `profile-test`, the Go tests, the architecture guard and the web lint, tests and build on every push.
- [x] AC-13 — `go test -count=1 ./...`, `gofmt -l .` (prints nothing), `go vet` (host and Windows), `npm run lint` and `npm test` pass. The architecture guard passes and every new package is classified.
- [x] AC-14 — Every file this spec created is added to `docs/Tool1-File-Guide.docx` in plain language, and `tool1-technical-spec.md` is updated where the design changed.


Part 6:

- [x] AC-15 — Uploading `testdata/run/taxsuite` in the browser shows the Rs 90 RECON-DEP-001 finding with both figures and both sources.
- [x] AC-16 — Uploading the unknown and damaged samples shows `PROFILE_NO_MATCH` and `PARSE_ERROR` with the file name, and nothing is stored.
- [x] AC-17 — Every filter (tax year, type, subject, category, status, source, rule, document, search) narrows the list correctly, and clearing the filters restores it. Vitest tests cover this.
- [x] AC-18 — Go tests of the endpoints, against an in-process test server, cover a successful run, an input error, a refused non-JSON file, the upload limits, and the removal of temporary files.
- [x] AC-19 — The server binds to 127.0.0.1 by default, and the architecture guard still proves the engine links no network code.
- [x] AC-20 — The file guide and the technical spec (§B11, §B15) are updated, and CLAUDE.md lists four screens (Rules being the start of the Rule Builder).


Part 7:

- [x] AC-21 — A client added from the screen is listed on the Season board, and is still listed after the server restarts. The database holds only the PAN's hash and last four characters, and no screen or API response shows the full PAN.
- [x] AC-22 — Uploading the client's accounts and our computation, in one upload or in two, gives the RECON-PL-001 finding (Rs 27,000) with both figures and both sources, and the clean pair gives no finding.
- [x] AC-23 — A file whose PAN is not the client's is flagged (`CONFLICTING_VALUES`), and its figures are not used.
- [x] AC-24 — The new profile, rules and fixtures pass `profile-test` and `rule-lint`; Go, API and Vitest tests cover adding a client, client runs and the PAN check; the architecture guard passes; and the file guide and the technical spec are updated.


Part 8:

- [x] AC-25 — A client can be added with all its details and edited later; invalid PAN, TAN, CIN/LLPIN, PIN, email, phone and GSTIN values are refused with a clear code, including a GSTIN whose check digit or PAN part is wrong.
- [x] AC-26 — The comparison types are listed from `comparisons.yaml`; a run for a type evaluates only its rules, and the Client file shows which of its documents are present and which are missing.
- [x] AC-27 — On the GST pack, RECON-GST-001 reports E01–E06 and RECON-GST-013 reports F01–F04 and F06, each with both values, the difference and both sources, and every other invoice matches. The sales-vs-purchase summary shows both sides' totals and produces no finding.
- [x] AC-28 — The sales and purchase register JSON files and GSTR-1 and GSTR-2B JSON files of the pack are recognised and read (profile tests with expected records).
- [x] AC-29 — A JSON file in a new layout can be taught on the mapping screen; the saved profile reads the file with no errors and the next upload of that layout is recognised. A mapping that cannot read its own file is refused.
- [x] AC-30 — Go, API and Vitest tests cover each of the above; the architecture guard passes (new packages classified, no network in the engine); the file guide, the technical spec and CLAUDE.md are updated.


Part 9:

- [x] AC-31 — Ticking a type lists only that type's rules; a run evaluates only the ticked rules, and the report lists the others as "Not run: not selected" without counting them as checks.
- [x] AC-32 — A rule outside the type (`RULE_NOT_IN_COMPARISON`), `--rules` without a type (`RULES_NEED_COMPARISON`) and an empty selection (`NO_RULES_SELECTED`) are refused, and through the API nothing is stored when any type of the upload is refused.
- [x] AC-33 — Two types run for one client, in one upload or in two, each keep their own latest result, and the Client file shows both as tabs; a later run of one type does not change the other's.
- [x] AC-34 — The Season board's counts for a client are the sum over its types' latest results.
- [x] AC-35 — Go, API and Vitest tests cover each of the above; the architecture guard passes; the file guide, the technical spec and CLAUDE.md are updated.


Part 10:

- [x] AC-36 — A Partner disables a client with a reason and enables it again with a reason; each writes one audit entry with the reason, and an empty reason is refused (`REASON_REQUIRED`).
- [x] AC-37 — A disabled client is not on the Season board or the Worklist; with **Show disabled clients** on it is listed and marked Disabled.
- [x] AC-38 — A disabled client's Client file opens read only with the banner; upload and run, edit, decide, delete finding and classify are refused with `CLIENT_DISABLED` (409), and nothing is written.
- [x] AC-39 — Enabling the client again restores it with all its documents, runs and decisions unchanged; adding a new client with its PAN while it is disabled is refused with `CLIENT_EXISTS`.
- [x] AC-40 — A Manager, Staff or Read-only user gets no Disable or Enable action, and the server refuses them with 403.
- [ ] AC-41 — Go, API and Vitest tests cover each of the above; the architecture guard passes; the file guide and the technical spec are updated.

## Verification

```
go run ./cmd/recon-dev profile-test
go run ./cmd/recon-dev rule-lint
go run ./cmd/recon-dev rulestore-import && git diff --exit-code docs/Recon_Tool_YAML_Rule_Store
go run ./cmd/recon-dev rulestore-check
go run ./cmd/recon-dev rulestore-problems
go run ./cmd/recon-engine load-rules --db build/recon.db --store docs/Recon_Tool_YAML_Rule_Store/recon_yaml --rules content/rules
go run ./cmd/recon-engine run --db build/recon.db --input testdata/run/clean --tax-year 2025-26 --include-testing
go run ./cmd/recon-engine facts --db build/recon.db --input testdata/run/clean
cd web && npm ci && npm run lint && npm test && npm run build && cd ..
go build -tags ui -o build/ ./cmd/recon-engine ./cmd/recon-server
build/recon-server --listen 127.0.0.1:8080 --db build/ui.db   # open http://127.0.0.1:8080: upload, run, filter
go test -count=1 ./internal/api/
go test -count=1 ./...
go test -count=1 ./internal/archtest/ -v
gofmt -l .
go vet ./... && GOOS=windows GOARCH=amd64 go vet ./...
```

## Build notes (14 October 2026)

- **Part 1:** until the firm names its tax-suite documents, a dummy full tax-suite export stands in for them: generic computation v3 (`content/profiles/generic/computation/v3`). It has every head of income, the additions and deductions from book profit, gross total income, Chapter VI-A deductions, total income, tax, taxes paid and the fixed-asset schedule. Two samples, each with its arithmetic checked: one where everything agrees, and one with a planted Rs 90 transposition (98,760 vs 98,670), which RECON-DEP-001 finds (`testdata/run/taxsuite`). Seven new figure names were added (vocabulary version 2). `profile-test` passes for all 8 samples (computation v1, v2, v3 and the tax audit report). Exactness is proven on the shipped samples: three decimals and a non-number are validation errors at their JSON path with no figure stored, a key written twice is refused (`PARSE_ERROR` with line and column), and a file two profiles recognise is refused (`PROFILE_AMBIGUOUS`) with nothing written.
- **Part 2:** `recon-engine facts --db --input [--json]` lists every tax year of that input's engagement. A round-trip test runs every profile sample through `run` and `facts` and compares the result with its expected facts, locators included.
- **Part 3:** the screens are in `web/src/screens`. Their sample data is in `web/src/sample`: the engine's own `run --json` and `facts --json` output on `testdata/run/{clean,differs,missing}`, kept byte-equal by `TestWebSamplesMatchEngine`, plus hand-made clients and Worklist items. `recon-server --listen` serves the embedded build read-only as a preview; it was checked over HTTP (page and script served, no external reference). `.gitattributes` keeps JSON line endings LF, so a Windows checkout does not change the bytes the tests compare.
- **Part 4:** `rulestore-problems` found 365 problems in 258 rules: 24 unverified, 2 conflicting, 60 uses of fields not in the catalogue, 21 rules without sources, 26 without a passing example, 18 without a failing example, and 214 statutory rules without `valid_from`. `rulestore-check` reports 0 problems. `rulestore-import` now keeps the store's date, so re-running it changes nothing; CI checks this.
- **Part 5:** CI runs `rulestore-check`, the workbook-match check, `rule-lint` and `profile-test` as named steps. SPEC-002 is Done, with the WiX licence decision recorded there.

### Part 6 (15 October 2026)

- `recon-server` (`internal/api`) serves the UI and the test API described in part 6. Every check runs in a separate `recon-engine` process; runs are taken one at a time; uploads go to a fresh temporary folder that is removed before the response is written. An engine refusal comes back as HTTP 422 with its stable code (`PROFILE_NO_MATCH`, `PARSE_ERROR`, ...). A file name is reduced to its last element and safe characters, so `..\..\evil.json` is stored as `evil.json` inside the folder.
- `recon-engine rules --db [--json]` lists the 306 stored rules (2 runnable, 304 described only) with type, subject, category, tax years, validity dates, status and source. Stored rule file paths are now relative to their folder (`recon_yaml/rules/...`, `rules/2025-26/...`), so they read the same wherever the program runs.
- The run JSON gains `passed` and a `category` on every result. The screens' sample data was regenerated from the engine, and a new `rules.json` sample comes from `rules --json`.
- Screens: an Upload and run panel and result filters (status, rule, category, document, search) in the Client file; a Rules screen (key 3) with filters for tax year, type, subject, category, status, source and a search ("/" to search, Esc to clear). A rule without tax years or dates counts for every year and is labelled "not dated". Without a server the screens show sample data and say so.
- Two rule-book rule names use banned phrases ("wrong" in RC-GST-022, "must be" in TB-002). They are shown as the CA wrote them in the Rules list, and `rulestore-problems` now reports them as `BANNED_WORDING` (367 problems in 259 rules) so the CA rewords them before either rule is made runnable.
- Checked end to end against the real server: the rules and tax-year endpoints, uploading `testdata/run/taxsuite` (the Rs 90 finding), uploading the unknown sample (`PROFILE_NO_MATCH`), and no temporary upload left afterwards. Go tests: `internal/api` (success, input errors, limits, non-JSON, path in a file name, temporary-file removal, read-only UI). Vitest: 30 tests, including every filter and the upload panel against a mocked server.

### Part 7 (15 October 2026)

- Migration `0002` adds `clients` (name, `pan_hash`, `pan_last4`, status), `settings` (the PAN hashing key), `engagements.client_id` and `runs.report_json`. The PAN hash is HMAC-SHA-256 with a random 32-byte key made on first use, so a copied hash cannot be looked up against all PANs without the key. A test checks that the full PAN is in neither the database file nor its WAL file, nor in any response.
- New engine commands: `client-add` (PAN on standard input; `BAD_PAN`, `BAD_CLIENT_NAME`, `CLIENT_EXISTS`), `clients --tax-year` (each client with the local date and counts of its latest run, and its number of documents), and `result --client --tax-year` (latest stored report and current figures). `run --client` uses the client's engagement. A file whose PAN is not the client's refuses the whole upload with `CONFLICTING_VALUES` and the file name, so another client's document can never become a draft of this one. A client run's report lists every document in the current view.
- New synthetic document: the client's financial statements (`fs.`, vocabulary version 3), with two samples whose sums add up, matching the two computation v3 samples. Computation v3 also reads `depreciation_as_per_books`. New rules: `RECON-PL-001` (profit before tax vs net profit as per P&L) and `RECON-PL-002` (book depreciation), each with two pass and two fail fixtures, loaded as testing. Test inputs: `testdata/run/compare-clean` and `compare-differs`.
- Server: `GET /api/v1/clients`, `POST /api/v1/clients` (201; 409 `CLIENT_EXISTS`; 422 `BAD_PAN`), `GET /api/v1/clients/{id}/result` (404 for an unknown client), and `client_id` on `POST /api/v1/runs`. The PAN goes to the engine on standard input and is never logged or returned.
- Screens: "+ Add client" (key `n`) on the Season board; with a server running, the board lists the database's clients with their latest counts, opening a client loads its latest result, and an upload adds to its documents. Without a server, a fifth sample client, Sample Engineering Works, shows the accounts-vs-computation comparison.
- Checked end to end over HTTP: add a client, upload the accounts (4 Cannot check), upload our computation separately (2 findings, including Rs 27,000 on RECON-PL-001), a file with another PAN refused, and the board counts updated. Tests: Go (`internal/store`, `internal/api`, `cmd/recon-engine`, `internal/engine` fixtures), 35 Vitest tests.

### Part 8 (15 October 2026)

- **Client master (8.1):** migration `0004` adds the client's details and a `client_gstins` table. `internal/client` checks every field and lists every problem with its code (`BAD_PAN`, `BAD_TAN`, `BAD_CIN`, `BAD_PIN`, `BAD_EMAIL`, `BAD_PHONE`, `BAD_GSTIN`, `GSTIN_CHECK_DIGIT`, `GSTIN_STATE`, `GSTIN_PAN_MISMATCH`, ...). New engine commands `client-update` and `client-get`; `client-add` also takes all details as JSON. Server: `GET/PUT /api/v1/clients/{id}`. Screens: the Add and Edit client form (key `n` on the Season board, `e` in the Client file). A new Go test keeps the form's lists of kinds of client, registration types and states equal to the engine's.
- **Comparison types (8.2):** `content/comparisons/comparisons.yaml` (five types), `internal/comparison`, `recon-engine comparisons` and `run --comparison`. The report lists the type's documents as read or not uploaded yet. Found while checking end to end: the sales vs purchase summary also totalled a GSTR-1 left in the client's engagement by an earlier upload. Totals now cover only the type's own documents (and add with overflow checks), and the test now includes a GSTR-1 to prove it.
- **GST invoice matching (8.3):** migration `0003` stores rows (`records`) with their source. Four new profiles (sales and purchase registers, GSTR-1, GSTR-2B), each with a full and a short sample and expected rows. The profile language gains `^` parent paths, `[*]` sums over item lines, `date_format`, period `format`, `from_period`, `from_gstin` and `where` filters. The new check type `match_records` gives one result per invoice, and `KEY_DIFF` is reported only when each row is the other's single candidate. `RECON-GST-001` and `RECON-GST-013` each have 2 pass and 2 fail fixtures and load as testing. On the pack, the answer-key test finds exactly E01–E06 and F01–F04 and F06, and every other invoice matches. `matchrecords` now has its own unit tests for every result code, the no-guessing rule and order independence.
- **New layouts (8.4):** `recon-engine inspect` and `profile-create` (`internal/profile/inspect.go`, `mapping.go`), `POST /api/v1/inspect` and `POST /api/v1/profiles`, and the "Teach this layout" screen. Office profiles have IDs starting `office.` and `source_system` "Taught in this office: ...".
- **Checked end to end over HTTP:** a mistyped GSTIN is refused with `GSTIN_CHECK_DIGIT`; a client is added and edited without retyping the PAN, and its 3 documents are kept; sales vs GSTR-1 gives 6 findings in 23 checks; the summary gives 0 checks and the two registers' totals only; an unknown layout gives `PROFILE_NO_MATCH`, is taught, and the next upload is read as `office.office_sales_export.sales_register`.
- Not in this round, as the spec says: E07–E12, F05 and G01–G05.

### Part 9 (15 October 2026)

- **Engine:** `run --rules` (refusals `RULES_NEED_COMPARISON`, `RULE_NOT_IN_COMPARISON`, `NO_RULES_SELECTED`, all before anything is written). Rules of the type that were not chosen are listed in the report (`not_selected`; text section "Not run: not selected") and not counted as checks. Migration `0005` adds `runs.comparison_id`. `result` returns `results`, the latest run of each type, and the client list sums their counts. The screens' sample data was regenerated; the only change is the new, empty `not_selected` list.
- **Server:** `comparison` may repeat, with `rules:<comparison id>` for each. The server checks every type and rule against the comparison list before running anything, so a wrong choice for any type stores nothing, then runs the engine once per type and returns `results`.
- **Screens:** the "All checks" choice is gone. Types are checkboxes, and each ticked type lists only its own rules, all ticked (nothing remembered between runs). Rules not yet approved can be ticked only with that switch on, and Run stays off while a ticked type has no rule ticked. The Client file has one tab per type (left and right arrows), and each tab lists its rules not run. The sample company Deccan Byte Systems is now one client with two tabs. A reply without `results` from an older server still shows its single report.
- **Tests:** Go `cmd/recon-engine/part9_test.go` and `internal/api/part9_test.go`, and Vitest `web/src/part9.test.tsx` (52 web tests in all). The earlier upload tests now tick a type before running, as the screen requires.
- **Checked end to end over HTTP on a fresh database:** a rule of another type is refused (`RULE_NOT_IN_COMPARISON`) and nothing is stored. Sales vs GSTR-1 and purchases vs GSTR-2B in one upload give 23 checks (6 fail) and 13 checks (5 fail), and the board shows 36 and 11. A tax-suite run with only RECON-DEP-001 lists RECON-DEP-002 as not selected. The existing `build/ui.db` was upgraded by migration `0005` and keeps its clients.
- **Fix after testing (15 October 2026): the tax year of a client file.** The Client file always opened on the Season board's tax year (2025-26), and the upload panel went back to it, so 2026-27 GST results were stored but not shown, and a new upload of 2026-27 files went in as 2025-26 and was refused (`CONFLICTING_VALUES`). Now `result` returns `run_tax_years` and the Client file opens on the latest tax year with a run, with a "Results for tax year" choice. The upload panel starts on that year, and `inspect` returns a recognised file's `tax_year`, so choosing files sets the run's tax year to theirs (with a note, or a warning when the files disagree). Each tab names its tax year. Tests: `TestTaxYearHints` (Go) and three Vitest tests (54 web tests in all).
- **Clearer results (15 October 2026), from testing.** The Documents table's column is now "Figures / rows read" and shows the invoice rows read from a GST file ("26 rows"), not "0" figures. A type that runs no check (the sales vs purchase summary) says so and points to its totals, instead of showing empty Findings, Cannot check and Passed lists. After a run, the tab opened is the type that found the most differences. Vitest: 57 tests.
- **Rows in the document panel (15 October 2026), from testing.** Opening a GST file in the Documents table said "No figures were stored", because the figure list (`facts --json`, and the `facts` of a client result) carried single figures only. Each document now also lists its rows (`records`: record type, key, fields, amounts in paise and the JSON path of the row; also in the text listing), and the panel "Read from <file>" shows figures, rows or both, whatever the document type. A file with nothing stored says why. The screens' sample data was regenerated; the only change is the new `records` lists.
- **Why a part of a file was not read, and what the result words mean (15 October 2026), asked by the user.** Each value not read now carries a `kind` and a plain reason in the validation report (`unread[].kind`, `row`, `reason`): `row_left_out` (a row the format leaves out, with the condition it failed, for example `.supply_type is "CDNR"; this format reads only rows where .supply_type is "B2B"`), `column_not_used` (a column of a row that was read; no check uses it yet) or `not_read` (another part of the file). The panel "Read from <file>" ends with "Not read, and why", grouped as rows left out (by reason, with the rows), columns not used (with counts) and other parts of the file (by top-level part). The Results show "What do result, finding and cannot check mean?". Reports stored earlier keep their old reason and count as `not_read` until the files are run again. Tests: `TestUnreadReasons` (Go) and three Vitest tests (61 web tests).

### Part 10 (2 November 2026)

- **Store:** migration `0010` adds `clients.status_reason`, `status_by`, `status_at` and the trigger `clients_no_delete`. A disabled client is `status = 'archived'` (migration 0002's value). `ClientByID` and `ClientDetails` now return a disabled client too (`Client.Disabled`), so its file still opens; `Clients()` (the board and the Worklist) still lists only clients in use, and `AllClients()` lists every one. `SetClientDisabled` keeps who, when and why.
- **Engine:** `client-disable` and `client-enable` (standard input `{"by", "reason"}`; `REASON_REQUIRED`, `BAD_NAME`, `BAD_REASON`, `CLIENT_NOT_FOUND`, `ALREADY_DISABLED`, `NOT_DISABLED`), each with its audit entry in the same transaction. `clients --all`; `status` and the `disabled_*` fields on every listed client and in `result`. `run --client`, `client-update`, `decide` and `classify` refuse a disabled client with `CLIENT_DISABLED` before writing anything.
- **Server:** permission `disable_clients` (Partner only; §B12.1 updated), `POST /api/v1/clients/{id}/disable` and `/enable`, `GET /api/v1/clients?all=1`. `CLIENT_DISABLED`, `ALREADY_DISABLED` and `NOT_DISABLED` answer 409. Deleting a finding of a disabled client is refused by the server with `CLIENT_DISABLED`.
- **Screens:** the Client file has **Disable client** (Partner) and, on a disabled client, a read-only banner with **Enable again**; both ask for a reason. Upload and run, Edit, decisions and Delete finding are hidden on a disabled client. The Season board reads every client (`all=1`), leaves the disabled ones off the list and the tiles, and offers **Show disabled clients (n)**, which lists them greyed and marked Disabled. "What each role can do" has the new row.
- **Tests:** Go `TestClientDisableAndEnable`, `TestDisabledClientLeavesTheWorklist` (`cmd/recon-engine/clientdisable_test.go`), `TestDisableClientThroughServer` (`internal/api/clientdisable_test.go`), the role-matrix tests updated; Vitest `web/src/clientDisable.test.tsx` and the updated `adminView.test.tsx`.
- **Not yet done:** the new files are not in `docs/Tool1-File-Guide.docx` (AC-41 stays open), and the screens were not checked by eye in the running app.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| 14 October 2026 | Unnati (with Claude Code) | not committed yet | All 14 criteria met; the synthetic tax-suite export stands in until the firm's formats arrive. |
| 15 October 2026 | Unnati (with Claude Code) | not committed yet | Part 6 added and done: AC-15 to AC-20 met. |
| 15 October 2026 | Unnati (with Claude Code) | not committed yet | Part 7 added and done: AC-21 to AC-24 met. |
| 15 October 2026 | Unnati (with Claude Code) | not committed yet | Part 8 added and done: AC-25 to AC-30 met. SPEC-004 Done. |
| 15 October 2026 | Unnati (with Claude Code) | not committed yet | Part 9 added and done: AC-31 to AC-35 met. SPEC-004 Done again. |
