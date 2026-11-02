# SPEC-006 — New check types, Form 67 and 26AS, and every Test Pack A finding the JSON can reach

| | |
|---|---|
| **Status** | Done (16 October 2026): parts 1, 2 and 3 (section 7: finding decisions, and the Worklist from real ledgers) |
| **Phase** | 1 |
| **Depends on** | SPEC-005 |
| **Requirements** | RU-01, RU-02, RU-43, RU-52, FD-02, FV-01, NF-01, NF-03 |
| **Technical spec** | §B4.2, §B4.3, §B4.6, §B3, §B2.2 |
| **Code / content paths** | `internal/engine/checks/*`, `internal/engine`, `internal/rules`, `internal/findings`, `content/vocabulary/`, `content/profiles/taxsuite_pack/`, `content/rules/2025-26/tieout/`, `testdata/testpack_nimbus/score.yaml`, `web/src/screens/ClientFile.tsx`; part 3: `internal/store/migrations/0006_decisions.sql`, `internal/store/decisions.go`, `cmd/recon-engine/decide.go`, `cmd/recon-engine/worklist.go`, `internal/ingest/adapters/csv`, `internal/modules/newheads`, `internal/findings/worklist.go`, `content/profiles/generic/ledger/`, `content/worklist/newheads.yaml`, `web/src/screens/Decisions.tsx`, `web/src/screens/Worklist.tsx` |

## Goal

Test Pack A, Pass 1 (the structured JSON only) detects every planted defect whose data is in the JSON and that a rule can state as a computed fact, with no false positive. On 16 October 2026 it detected 2 of 30 (critical 2 of 4). The target of this spec is 21 of 30, critical 4 of 4, 0 false positives. The user asked on 16 October 2026 for everything planned as SPEC-006, 007 and 008 to be built in one spec.

## Scope

### 1. Check types (all integer arithmetic, no clock, no randomness)

| Check type | What it computes | Fails when | Findings |
|---|---|---|---|
| `sum_equals` | the sum of the `parts`, each added or taken away, against the `total` | the sum and the total differ by more than the tolerance | F02, F08, F18 |
| `gte` | input `a` against input `b` | `a` is lower than `b` by more than the tolerance | F09, F13 |
| `percent_of` | `rate` per cent of `base`, rounded to the rupee (half up), against `value` | they differ by more than the tolerance | F04 |
| `present_together` | each input (a figure, or a column of each row of one record type) against a condition: `blank`, `not_blank`, `zero`, `nonzero`, `equals`, `matches` (a regular expression), `in`, `not_in`, `true`, `false` | every condition holds at once | F05, F10, F17, F19, F20, F21, F22, F24 |
| `table_lookup` | the value of the `value` input against the row of the rule's own `table` whose keys equal the `keys` inputs (a small law table kept in the rule file, offline, with its source) | the value differs from the table's, or the keys are not in the table (Cannot check) | F06 |
| `match_records` (changed) | may compare keys only (`compare_amounts` may be empty) | as before | F14, F15, F16 |

- Each finding shows every value with its source (file and JSON path), the value the check computed (labelled as computed, with no source), and the difference where the figures are money. `present_together` and `table_lookup` show no difference: they report that the values were found together, or the value and the table's value, never a judgment (CLAUDE.md invariant 1).
- A check type refuses, when a rule loads, any parameter that belongs to another check type.

### 2. Two more documents of the filing pack

- **Form 67** (new document type `form_67`, prefix `ftc`): country, source of income, DTAA article, DTAA rate, income outside India, foreign tax paid, tax payable under the normal provisions, credit claimed.
- **Form 26AS** (new document type `form_26as`, prefix `as26`): total amount credited.
- More figures of the tax audit report, Form 29B and the computation, and the rows of clause 20(b) (PF and ESI), 26 (section 43B), 31(a) and 31(c) (loans) and 34(b) (TDS statements, 24Q and 26Q) as record types. Vocabulary version 6.
- A profile of a bundle may read a figure outside its section (the assessee's status); a value not read is listed only if no profile of the bundle read it.

### 3. Rules (each `equal`/new check type rule has two pass and two fail fixtures, and loads as testing)

| Finding | Rule | Check |
|---|---|---|
| F02 | RECON-PGBP-001 | profit + book depreciation + income tax and interest added back − interest income − clause 18 depreciation, against the clause 33 deduction |
| F04 | RECON-MAT-003 | 15 per cent of Form 29B Sl.18, against Sl.19 |
| F05 | RECON-FTC-001 | Form 67: business income and DTAA article 12, found together |
| F06 | RECON-FTC-002 | Form 67 DTAA rate against the rule's treaty table (India–Portugal, Article 12: 10 per cent) |
| F07 | RECON-FTC-003 | Form 67: foreign tax paid against the credit claimed |
| F08 | RECON-FTC-004 | Form 67: credit claimed − tax payable under the normal provisions, against the excess credit in the computation's MAT credit table |
| F09 | RECON-TAR-040 | clause 40: gross profit against net profit (`gte`) |
| F10 | RECON-TAR-008 | clause 8 audit limb of profession and clause 33 deduction under 80-IAC, found together |
| F13 | RECON-TAR-044 | clause 44 expenditure with other registered entities against 194J payments in clause 34(a) (`gte`) |
| F14 | RECON-TAR-020 | clause 20(b): PF due dates against ESI due dates |
| F15 | RECON-TAR-034 | clause 34(b): 24Q due dates against 26Q due dates |
| F16 | RECON-TAR-031 | clause 31(a) against 31(c), by name: PAN |
| F17 | RECON-TAR-032, RECON-TAR-033 | clause 31(a) and 31(c) rows: nature code not in Loan, Deposit, Specified sum, Specified advance |
| F18 | RECON-REC-001 | 26AS amount credited − interest income + Form 67 income outside India, against clause 40 turnover |
| F19 | RECON-TAR-021 | clause 21(a) "Personal expenditure" and assessee status Company, found together |
| F20 | RECON-TAR-035 | interest on TDS added back in the computation and clause 34(c) "not liable under 201(1A)", found together |
| F21 | RECON-TAR-022 | clause 22 total payable to micro and small enterprises nil and clause 44 payments to unregistered entities, found together |
| F22 | RECON-TAR-026 | clause 26 rows: section 43B(a) and nature TDS, found together |
| F24 | RECON-TAR-050, RECON-MAT-004 | UDIN blank in Form 3CD and in Form 29B |

The wording of the `present_together` and `table_lookup` rules, and the treaty table, are for the CA to approve before any of them is switched on.

### 4. Scoring

- `testdata/testpack_nimbus/score.yaml` maps each finding above to its rules; `TestTestPackAPass1` requires every one to fire with its difference, no unmapped rule to fire, and the corrected copy of the pack (every reachable defect corrected) to give no finding.
- Negative controls: the rules that read a control's figures are listed against it and must not fail on it.

### 5. Screens

- A finding of `present_together` or `table_lookup` shows its values and sources, and no difference line. A computed value is labelled and has no source.

### 6. Part 2: the last finding the JSON can reach (added 16 October 2026)

Goal: reach the JSON maximum of Test Pack A, Pass 1 (22 of 30), and make sure every figure the findings use is read from the right entry of a list. The tests of the negative controls and the one-defect-at-a-time test are in SPEC-007.

- **6.1 F23 (43B(b) carries only the March employee share).** Clause 26 shows 43B(b) PF 34,180 and ESI 318, the same as the clause 20(b) rows due 15/04/2026. Wording to be decided with the CA before building: (a) a factual match, "clause 26 43B(b) PF equals the clause 20(b) PF for the month due 15/04/2026", reported as found together with no conclusion drawn; or (b) not built, left to manual review and recorded as such.
- **6.2 List entries read by content, not position.** The 194J row of clause 34(a), the AY 2026-27 row of the MAT credit table and the first entries of clauses 21(a) and 33 are read today as `[1]`, `[2]` and `[0]`. They are to be selected by what they hold (section "194J", ay "2026-27"), so a tax suite that orders the lists differently still gives the right figures; a missing entry gives Cannot check, never another entry's figure.

Acceptance criteria of part 2:

- [x] AC-8 — F23 is built as decided in 6.1, or recorded as not built; Pass 1 scores 22 of 30 (21 if F23 is not built), critical 4 of 4, 0 false positives.
- [x] AC-9 — No figure of the filing pack is read by its position in a list; a missing entry gives Cannot check. Profile tests cover a list in another order.
- [x] AC-10 — All tests and the architecture guard pass; the file guide, the technical spec and CLAUDE.md are updated.

### 7. Part 3: decisions on findings, and the Worklist from real ledgers (added 16 October 2026)

Goal: staff decide each finding on screen (Fix, Not an issue, Ask client), the decision is kept with who, when and why, and the Worklist shows the new, vanished and changed ledger heads of real ledgers instead of sample data.

**7.1 Decisions on findings**

- For a client in the database, each finding of the Client file has three decisions: **Fix** (the document is to be corrected), **Not an issue** (reviewed; nothing to change; a note saying why is required) and **Ask client** (a query for the client). A finding with no decision is **Open**.
- A decision is kept against the finding's stable ID (§B7.1) for the client's engagement, with the decision, the note, who decided (a name typed on screen until logins exist, Module 16), when, and the finding's value fingerprint. Every decision is added to an append-only history; nothing is overwritten.
- Later runs: the same finding with the same values keeps its decision. If its values change, the decision is shown as made on other values and the finding is Open again, so a fixed defect that comes back, or a figure that moves, is looked at again (FD-04). A finding no longer found keeps its history.
- A decision never changes a finding: the tool still shows both values, the difference and both sources (CLAUDE.md invariant 1).
- Engine: `recon-engine decide` (the engine stays the database's only writer) and `result` returning each finding's ID, current decision and history. Server: `POST /api/v1/clients/{id}/decisions`. Screens: the three decisions and the note on each finding, the decision and its history, a Decision filter (Open, Fix, Not an issue, Ask client), and the number of open findings on each tab.

**7.2 The Worklist from real ledgers**

- A CSV reader (file type `csv`, first row the column names) gives the same tree as the JSON reader, so CSV formats are written like JSON ones (`$[*]`, `.amount_inr`). Every source keeps a manual file path (CLAUDE.md invariant 7).
- A ledger document type and a monthly ledger row (head, group, month, amount), and a format for the test pack's expense ledger CSV. The tax year comes from the months in the file.
- Last year's ledger uploaded with this year's is stored as last year's document (the engagement of the prior tax year), not refused.
- The year-on-year comparison (`internal/modules/newheads`, deterministic) gives Worklist items, never findings: **new head** (this year, not last year), **vanished** (last year, not this year), **broken recurrence** (every month last year, some months missing this year, listed), **step change** (this year's total at least twice, or at most half, last year's, and the larger at least Rs 1,00,000). The thresholds are data (`content/worklist/newheads.yaml`).
- Each item has a classification column: staff write the classification (clause mapping, GST status, TDS section, 43B category) and their name; the tool never classifies (CLAUDE.md invariant 2). Classifications are kept across runs.
- Engine: `recon-engine worklist` and `classify`. Server: `GET /api/v1/worklist`, `POST /api/v1/clients/{id}/worklist` (the classification belongs to a client's engagement, so the client is in the path). The Worklist screen shows the database's items for the tax year, with the client, the kind, both years' figures and the classification; without a server it keeps the sample data and says so.
- Test Pack A: the two ledger CSVs give exactly F27 (7 new heads), F28 (2 vanished), F29 (Electricity charges, no 2025-11; Internet and broadband, no 2026-02) and F30 (Travelling expenses - overseas, about 3.9 times), and nothing else.

Acceptance criteria of part 3:

- [x] AC-11 — A finding can be decided Fix, Not an issue (with a note) or Ask client on screen; the decision, note, name and time are kept, with an append-only history, and survive a server restart.
- [x] AC-12 — A later run keeps the decision of a finding whose values are unchanged; a finding whose values changed is Open again and shows the earlier decision as made on other values.
- [x] AC-13 — The CSV reader and the ledger format read the test pack's ledgers (profile tests with expected rows); last year's ledger in the same upload is stored as last year's.
- [x] AC-14 — The comparison gives exactly F27 to F30 on Test Pack A, and the test fails on any other item.
- [x] AC-15 — The Worklist screen shows the real items with a classification column; a classification is kept across runs and restarts; the tool never fills one in.
- [x] AC-16 — Go, API and Vitest tests cover each of the above; the architecture guard passes; the file guide, the technical spec and CLAUDE.md are updated.

## Out of scope

- F23 (43B(b) equal to the March employee share): a rule design decision first.
- F11, F12 (version diff), F25, F26 (PDF-only fields): not in the JSON. (F27 to F30 come from the ledger CSVs, in part 3.)
- Signed law-table packs (the treaty table is kept in the rule file for now).

## Acceptance criteria

- [x] AC-1 — The five new check types and the `match_records` change are built, registered, unit-tested, and refuse parameters of other check types.
- [x] AC-2 — Form 67 and Form 26AS are read from the filing pack; the new figures and rows are in vocabulary version 6; `profile-test` passes with every sample's expected figures and rows hand-checked.
- [x] AC-3 — Every rule in the table passes `rule-lint` with two pass and two fail fixtures.
- [x] AC-4 — Test Pack A, Pass 1: 21 of 30 findings, critical 4 of 4, 0 false positives; each finding shows its values, the computed value where there is one, the difference where it means something, and every source. The corrected copy gives no finding.
- [x] AC-5 — No negative control is flagged.
- [x] AC-6 — Finding text has no banned phrase and states no correct value or characterisation; `present_together` findings say only that the values were found together.
- [x] AC-7 — `go test -count=1 ./...`, `gofmt`, `go vet` (host and Windows), `npm run lint`, `npm test` and the architecture guard pass; the file guide, the technical spec and CLAUDE.md are updated.

## Verification

```
go run ./cmd/recon-dev profile-test
go run ./cmd/recon-dev rule-lint
go test -count=1 ./cmd/recon-engine -run TestTestPackAPass1 -v
go test -count=1 ./cmd/recon-engine -run "TestTestPackAWorklist|TestWorklistNeedsBothYears|TestFindingDecisions" -v
go test -count=1 ./internal/modules/newheads ./internal/ingest/adapters/csv ./internal/api
go test -count=1 ./...
cd web && npm run lint && npm test && cd ..
```

## Build notes (16 October 2026)

- **Numbering:** the user asked for "Spec 7"; the next free number was 006, so everything planned as SPEC-006, 007 and 008 is in this one spec.
- **Check types:** `sum_equals`, `gte`, `percent_of`, `present_together` (on figures, or on each row of one record type, through a new `rules.RowEvaluator` and `engine.evaluateRowRule`) and `table_lookup` (`Outcome.Missing` for keys with no row), registered in `internal/engine/checks/registry.go`; `match_records` matches keys alone when `compare_amounts` is empty. Each refuses the parameters of the others (`rules.OtherParams`). Unit tests: `internal/engine/checks/checks_test.go` (rounding, labels, missing table row, eleven refused rules).
- **Report and screens:** findings carry `show_difference` (false for `present_together` and `table_lookup`); a computed value (sum, percentage, law table's value) has no source; a blank text shows as "(blank)"; a legal reference that is not a section number (an article, a rule) is shown without "Sec".
- **Documents:** vocabulary 6 (document types `form_67`, `form_26as`; 29 figures; 7 record types: PF and ESI payments, 24Q and 26Q statements, loans taken and repaid, section 43B items). Six filing-pack profiles; a profile of a bundle may read a figure outside its section (the assessee's status), and a value is listed as not read only if no profile read it. The corrected copy now corrects every defect Pass 1 can reach. `profile-test`: 30 samples pass.
- **Rules:** 21 new rules (table in Scope 3), each with two pass and two fail fixtures; all 29 engine rules pass `rule-lint` and their fixtures. The "Tax suite filing pack" comparison type runs all 27 rules that read the pack.
- **Result, Test Pack A Pass 1: 21 of 30 findings, critical 4 of 4 (F01, F02, F03, F10), 0 false positives**; every difference matches the answer key (F02 Rs 2,448; F04 Rs 6,28,148; F07 Rs 1,23,287; F08 Rs 8,18,272; F09 Rs 3,71,24,600; F13 Rs 12,61,100; F18 Rs 98,40,350). The corrected copy gives no finding; no negative control is flagged. Not covered: F23 (rule design decision) and the 8 findings whose data is not in the JSON.
- **Tests made sturdier:** the older tests that listed every shipped rule now count the rule files, so a new rule needs no test edit.
- **For the CA before any of these rules is switched on:** the wording of the `present_together` rules (F05, F10, F19, F20, F21, F22), the treaty table of RECON-FTC-002 (India–Portugal, Article 12: 10 per cent), the allowed nature codes of RECON-TAR-032/033, and whether RECON-FTC-003 (foreign tax paid against credit claimed) is wanted, since the Rule 128 cap makes them differ in ordinary cases.
- Everything runs offline: rules, the treaty table and the scoring are local files.

### Part 2 build notes (16 October 2026)

- **F23:** wording (a) was built, as the user did not choose and (a) was the recommendation: `RECON-TAR-023` (PF) and `RECON-TAR-024` (ESI) report the clause 26 section 43B(b) sum and the clause 20(b) employees' contribution with the latest due date as the same figure, found together, with no conclusion. Both load as testing; the CA approves the wording, or they are disabled (option (b)). A new `present_together` condition, `same_as`, compares two figures.
- **Figures from list entries by content:** a profile fact may give `rows`, `where` (each `{path, equals}` or `{path, equals_path}`, a value elsewhere in the file), `json_path` inside the entry and `combine`: `one` (several entries meeting the conditions: warning `ENTRY_AMBIGUOUS`, no figure), `sum` (money) or `latest` (`by` a date column; a tie is `ENTRY_AMBIGUOUS`). No entry: no figure (rules that need it say Cannot check; RECON-TAR-008 and -021 count an absent deduction or amount as nil through an optional input). Vocabulary 7 retires `tar.chapter_via.section`, `tar.chapter_via.amount` and `tar.inadmissible.first_nature` and adds `tar.deduction_80iac.amount`, `tar.inadmissible.personal_expenditure` and the four F23 figures. The filing-pack profiles read no figure by position; a third sample of each, every list reversed, gives the same figures (36 samples pass). Unit tests: `internal/profile/factrows_test.go`.
- **Result, Test Pack A Pass 1: 22 of 30, the JSON maximum; critical 4 of 4; 0 false positives;** the corrected copy (now with the employer's share in 43B(b)) gives no finding.

### Part 3 build notes (16 October 2026)

- **Decisions on findings (7.1):** migration 0006 adds `finding_decisions` and `worklist_classifications` (rows are only added: triggers refuse updates and deletes) and `runs.worklist_json`. The report gives each finding its stable ID (`finding_id`, §B7.1, from the engagement, rule, subject and inputs) and a `value_fingerprint`. `recon-engine decide` (decision as JSON on standard input; codes `BAD_DECISION`, `BAD_NAME`, `NOTE_REQUIRED`, `FINDING_NOT_FOUND`) keeps a decision against the client's latest results, and `result` returns each finding's state, current decision and history. A decision taken on other values shows as such and the finding is Open again. Server: `POST /api/v1/clients/{id}/decisions`. Screens: Fix, Not an issue (note required) and Ask client on every finding card, the history, a Decision filter, "N open" on each tab, and a "Your name" field kept in the browser until logins exist.
- **CSV reader:** `internal/ingest/adapters/csv` turns a CSV file (UTF-8, byte-order mark allowed, first row the column names, each named once) into the JSON reader's tree, so CSV formats use JSON paths and every value keeps its row (`$[3].amount_inr`). Refused: an unnamed or repeated column, a short row, an open quote, text that is not UTF-8 (`PARSE_ERROR` with line and column).
- **Ledger format:** vocabulary 8 adds the document type `ledger` and the record type `ledger_month` (key ledger head and month; group; amount). `content/profiles/generic/ledger/v1` reads the test pack's expense ledger CSV; the tax year comes from the first row's month (`from_month`, a new context option). `profile-test`: 38 samples pass, the two ledgers with all 264 and 284 rows worked out from the CSV.
- **Last year's ledger:** a ledger whose tax year is the one before the run's is stored in the client's engagement for that year, not refused; a later run of the client finds it there. Any other file of another tax year is still refused (`CONFLICTING_VALUES`), so only a ledger can be last year's.
- **The comparison:** `internal/modules/newheads` compares the two years' rows, in integer paise and big-integer products (no floating point): new head, vanished, broken recurrence (a head in every month last year's ledger covers, missing in some month this year's ledger covers, so a part-year ledger does not mark every head) and step change (at least 2.00 times or at most 0.50 times, the larger total at least Rs 1,00,000; `content/worklist/newheads.yaml`, read as data). Items are sorted by kind and ledger head; the rows read are listed with each. Without last year's ledger nothing is new, and without this year's nothing has vanished.
- **Report and run:** when both years' ledgers are there, the report has a Worklist section (text and JSON `worklist`), headed as not findings; it is never counted as a check. The run keeps the items with itself (`runs.worklist_json`). A new type of check, "Expense ledgers: this year against last year" (`ledger_year_on_year`, `worklist: true`: one document type, no rules, no totals), lets the upload screen run it.
- **Commands and server:** `recon-engine worklist` (every client's latest Worklist for a tax year, with classifications) and `classify` (JSON on standard input: kind, ledger, classification, by; codes `BAD_KIND`, `BAD_NAME`, `BAD_CLASSIFICATION`, `WORKLIST_ITEM_NOT_FOUND`). Server: `GET /api/v1/worklist?tax_year=`, `POST /api/v1/clients/{id}/worklist`; uploads and the tax-year check now take `.csv` as well as `.json` (teaching a layout stays JSON only).
- **Worklist screen:** the database's items for the tax year with the client, the kind, last year's and this year's totals, what was counted (months, missing months, times) and a classification that staff write (free text: clause, GST status, TDS section, 43B category) and save under their name; the date and name are shown. The fixed list of sample classifications is gone: the tool never fills one in. Without the server, the screen shows the engine's real Worklist on Test Pack A (`web/src/sample/worklist.json`, kept equal to the engine by `TestWebSamplesMatchEngine`) and says that classifications are not kept. The upload screen sets the tax year to this year's when last year's ledger is chosen with it.
- **Result, Test Pack A: exactly F27 to F30, 12 items and nothing else** (`TestTestPackAWorklist`; `score.yaml` lists them): 7 new heads, 2 vanished, Electricity charges with no entry for 2025-11 and Internet and broadband with none for 2026-02, and Travelling expenses - overseas from Rs 9,83,627 to Rs 37,72,925 (3.84 times; the answer key's "approx 3.9" is worked from rounded figures). Vanished totals are the exact sums (Courier and postage Rs 49,460; Office rent - Malviya Nagar Rs 19,86,242; the answer key rounds them).
- **Tests:** `internal/modules/newheads` (each kind, threshold edges, one year only, order of rows, the thresholds file), `internal/ingest/adapters/csv`, `cmd/recon-engine/decisions_test.go` and `worklist_test.go` (prior-year storage, classification kept across runs, append-only), `internal/api/decisions_test.go` and `worklist_test.go` (kept across a restart), `web/src/decisions.test.tsx` and `worklist.test.tsx`. 72 web tests pass; the architecture guard passes.
- **For the CA:** whether the classification should be free text (as built) or chosen from fixed lists (clause 44 category, TDS section, 43B item), and whether the thresholds (2.00, 0.50, Rs 1,00,000) suit the firm.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| 16 October 2026 | Unnati (with Claude Code) | not committed yet | All 7 criteria met. Test Pack A Pass 1: 21 of 30, critical 4 of 4, 0 false positives. |
| 16 October 2026 | Unnati (with Claude Code) | not committed yet | Part 2 done: AC-8 to AC-10 met. Pass 1: 22 of 30 (the JSON maximum), critical 4 of 4, 0 false positives. |
| 16 October 2026 | Unnati (with Claude Code) | not committed yet | Part 3 done: AC-11 to AC-16 met. Decisions on findings; the Worklist from the ledger CSVs gives exactly F27 to F30. |
