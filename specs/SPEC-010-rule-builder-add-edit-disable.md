# SPEC-010 — Rule Builder: add, edit and disable rules from the Rules tab

| | |
|---|---|
| **Status** | In progress |
| **Phase** | 4b |
| **Depends on** | SPEC-009 (build order), SPEC-007 (rule fixtures and `rule-lint`) |
| **Requirements** | RU-10, RU-11, RU-12, RU-21, RU-23, RU-40, RU-41, RU-42, RU-43, RU-44, RU-50, RU-51, RU-53, IN-12 |
| **Technical spec** | §B9 (Rule Builder backend, form → YAML, lifecycle, audit), §B3 (`audit_log`, `rule_versions`, `rule_state`), §A8, §B4.2, §B14 |
| **Code / content paths** | `internal/rules` (`Generate`, a new save path), `cmd/recon-engine` (`rule-save`, `rule-disable`, `rule-enable`, `rule-versions` commands; `--office-rules` flag on `load-rules` and `run`), `internal/store` (`audit_log` migration, rule version and state writes), `internal/api` (rule write endpoints), `web/src/screens/Rules.tsx`, `web/src/screens/RuleBuilder.tsx` (new), `web/src/api.ts`, `web/src/types.ts` |

## Goal

The CA and staff can add a rule, edit an existing rule and disable a rule from the Rules tab, in a window with plain fields. The tool turns what they fill in into a rule YAML file, checks it with the same strict loader the engine uses, and saves it. Nothing needs an internet connection, and nobody types YAML.

## Scope

### 1. Rules tab actions

- An **Add rule** button on the Rules tab opens the Rule Builder window.
- Each rule row gets **Edit** and **Disable** actions. A rule that is already disabled gets **Enable again** instead of Disable.
- The read-only view from SPEC-004 is unchanged for anyone who only reads rules.

### 2. The Rule Builder window (form → YAML)

The window asks for these fields. Labels are plain English; every field has a short help line.

| Field | How it is filled |
|---|---|
| ID | Typed. Must match `RECON-<PREFIX>-<NNN>` and not exist yet for the tax year. |
| Title and rule text | Free text (`title`, `rule_text_en`, `notes`). Stored only as text; never read by the engine (RU-23). |
| Module, category | Chosen from the lists in `content/rules/categories.yaml` and the module names the engine knows. |
| Tax years and valid dates | A tax year and a from-date; an open-ended to-date is allowed. |
| Inputs | Each input picked from the fact vocabulary (`content/vocabulary/facts.yaml`) or record types (`records.yaml`), with a label. No free-text fact names. |
| Check type | Chosen from the check types the engine registers (today: `equal`, `sum_equals`, `gte`, `percent_of`, `present_together`, `table_lookup`, `match_records`). The window shows only the fields that check type uses. |
| Tolerance | A rupee amount, typed as text and stored as integer paise (for example "1.00" → 100). No floating point. |
| Severity | Chosen from the severity list the loader already accepts. |
| Law reference | Act, section and reference. Required (RU-10); a rule without one is refused. |
| Fixtures | At least one pass and one fail fixture, each a small table of figures in the format `internal/rules/fixture.go` reads. |

On save, `rules.Generate()` writes canonical YAML with a stable key order and quoted text values (tech spec §B9). The result goes through the full strict loader and `rule-lint` before anything is written.

### 3. Where the rules are kept

- A new rule is written to `build/office-rules/<tax-year>/<module>/<ID>/rule.yaml`, with its fixtures beside it, and gets `source: "office"`. The folder is given by `--office-rules`, the same way office profiles are given by `--office-profiles`. It is under `build/`, which git ignores.
- Built-in rules in `content/rules/` are never edited. Editing a built-in rule creates `<ID> v1-office.1` in the office folder (RU-12), which the engine then uses in place of the built-in version. "Reset to standard rule" removes the override; the built-in version is still there.
- Each save is a new version. Earlier versions are kept, can be listed, and the rule in use can be switched back to an earlier version (RU-41).

### 4. Lifecycle

- A saved rule starts as **testing**. Like every rule that loads as testing, it runs only with `--include-testing`, and it is labelled "testing" on the Rules tab.
- **Partner approval (testing → active, RU-40).** A rule in testing has an **Approve** action in the Rule Builder window. Approving needs the Partner's name and a reason, and it writes an audit entry (RU-42). Approval is offered only when the rule's pass and fail fixtures pass in the engine and the rule has been run on at least one client with testing rules included (RU-43). An approved rule is **active**: it runs in every run, without `--include-testing`. The Rules tab labels it "active".
- **Who the Partner is.** Logins and roles are not built yet, so the approver types their name, as staff do today. The window and the audit entry say so. Until roles exist, anyone who can open the window can approve (see the open questions).
- An approved rule that is later disabled goes back to **disabled**, with a reason. Enabling it again returns it to **testing**, so it must be approved again (RU-40).
- **Disable** is the one-click removal from use (RU-40, RU-44). It needs a reason. The rule's files and versions are kept on disk and in the database (RU-41: nothing is deleted). Enable again needs a reason too.

### 5. Audit

Each create, edit, disable and enable writes one `audit_log` entry with the action, the object (rule ID and version), the reason where one is needed, and the time (RU-42). `audit_log` is added by this spec as its minimal table in §B3: append-only triggers, and a hash chain. The audit time never reaches the engine, so runs stay the same (RU-51).

### 6. Runs record the rule used

A run already records the rule version and its status (RU-53). An office rule or an edited version appears in the run's record with its own version and source.

### 7. Help for the window (added after the build)

- The window opens with a **How to add a rule** panel at the top: seven numbered steps, from naming the rule to saving it. A **?** button beside the note "Fields marked * are required" shows or hides the panel. It opens by default when a rule is being added and closed when a rule is being edited.
- Required fields are marked with a red *. The mark is drawn by CSS beside the label text, so the label's text and its accessible name do not change. The required fields are: your name, ID, title, rule text, module, category, tax year, valid from, check type, severity, review document, each figure's name, kind and figure, each legal basis's Act and section, each fixture's kind, name, figure, value, period, source file and locator. Base, rate and figure compared are marked when the check type shows them. Tolerance, valid to, notes and references stay optional.

### 8. The whole rule in the summary, and Partner edits of rule-book rules (added 2 November 2026)

Requested by the user: opening a rule on the Rules tab showed only its type, category, years, version and file, not the rule itself; and a Partner must be able to edit and disable a rule-book rule, not only an engine rule.

- **The whole rule in the summary.** Opening a rule shows everything its file holds, in plain sections, read from the version in use (`rule-get`, `GET /api/v1/rules/{id}`):
  - an engine or office rule: rule text, notes, module, the figures it reads, the check type and its settings, tolerance in rupees, severity, review document, legal basis, and its pass and fail fixtures;
  - a rule-book rule: purpose, governance (verification status, reviewed by and on, last change and its note), validity, legal basis (provisions, applicability, exemptions, thresholds, rounding, due dates), inputs (documents, fields, prerequisites, validations), matching (keys, normalisation, aggregation, comparisons, tolerance), calculation (formula and steps), parameters, outcomes (categories with severity, expected output, recommended action, common discrepancies), tests (worked example, positive, negative), pseudocode, sources and notes;
  - the status, and the reason it is switched off when it is.
  Clicking a rule opens the summary as a popup in the middle of the screen (not a side drawer); it scrolls on its own, so a long rule is never cut off, and its actions stay at its foot. Every rule ID the Rules tab lists can be opened, including rule-book IDs such as `RC-ACC-003` and `GST-001`.
- **The rule book fits the screen.** On a screen wider than 760px the rules table never scrolls left to right: each column has a fixed share of the width, long names, categories and subjects wrap, and a row's action buttons wrap onto a second line. Narrower, the table is stacked as cards, as before.
- **Popups and pages that match the other screens.** Switch off, Enable again and Approve open as a compact popup in the middle of the screen, laid out like the Admin popups and Delete finding: the note, the reason box at full width (required, marked *), then Cancel and the action at the bottom right. The page bar (how many a page, which ones, Previous and Next) sits under the list on every list screen: Rules, Clients (Season board), Worklist and the Admin Users list, which now shows users a page at a time as well. (`web/src/adminView.test.tsx` Users in pages; `web/src/part6.test.tsx` pages.)
- **A Partner edits a rule-book rule.** A rule-book rule gets **Edit this rule** for a Partner only (permission `activate_rules`: the rule book is CA-approved content). The window shows the rule-book fields as plain text boxes and lists (no YAML). Saving needs a reason. The engine (`rulebook-save`) writes the changed rule as canonical rule-store YAML with every text value quoted, reads it back through the strict reader, and stores it as a new **office** version of the rule in `rule_versions` (schema `rulestore/1.0`). The ID, type and domain cannot be changed. `governance.last_modified_by`, `last_modified_on` and `change_note` are set from the Partner's name, the date and the reason. The file under `docs/Recon_Tool_YAML_Rule_Store/` is never changed (as for built-in rules, RU-12). Earlier versions stay, are listed, and the rule-book version can be put back in use with **Reset to rule-book version**. Each save writes one audit entry.
- **A Partner disables and enables a rule-book rule.** **Disable** (permission `disable_rules`) and **Enable again** (permission `activate_rules`) work on rule-book rules with a reason, and write an audit entry. A rule-book rule switched on is **active**, as `governance.enabled: true` makes it on load; it is still described only and is never run. Once a person has set a rule-book rule's status, loading the rule book again does not change it back; before that, the status follows `governance.enabled` as today. A rule-book rule whose office version is in use keeps that version when the rule book is loaded again.
- Staff and managers see the whole rule but get no edit, disable or enable on a rule-book rule; the server refuses them too.

### 9. Disable or enable several rules at once (added 2 November 2026)

Requested by the user: rules could be switched off or on only one at a time from each row's menu; there was no way to select rules.

- **Select rules.** With the server running, a user who may disable or enable rules (`disable_rules` or `activate_rules`) sees a tick box at the start of each row of the Rules table, and one in the heading that selects (or clears) every rule the filters show, on every page, not only the page in view. Selections stay while the page, sort or filters change; the count says how many are selected and how many of them are hidden by the filters. **Clear selection** empties it.
- **The bulk bar.** While rules are selected, a bar over the table says how many, and offers **Disable (n)** for the selected rules that are not switched off (permission `disable_rules`) and **Enable again (n)** for those that are (permission `activate_rules`). A button whose count is 0, or whose permission the user lacks, is not shown.
- **One reason, every rule.** Either button opens the same popup as for one rule: the note, the list of rule IDs it will change, a required reason, Cancel and the action. The reason is written for each rule.
- **All or nothing.** The server sends the IDs to the engine in one call (`rule-disable` or `rule-enable` with `--rules ID,ID,...`), which changes them in one transaction: one audit entry per rule, with the reason; if any rule is refused (unknown, already in that state, a structure that cannot be switched), nothing is changed, and the message names the rule. At most 500 rules in one request. Each rule follows the same lifecycle as when switched alone: a runnable rule enabled again is testing and needs approval again; a rule-book rule enabled again is active.
- Server: `POST /api/v1/rules/bulk/disable` (`disable_rules`) and `POST /api/v1/rules/bulk/enable` (`activate_rules`), body `{"rule_ids": [...], "reason": "..."}`; the name is the signed-in user's. After the change the selection is cleared and the list is read again.

## Out of scope

- Logins, roles and a separate Partner approval screen (a later spec). Approval in this spec is the Approve action in the window, with the name typed.
- A rule that needs Starlark or a new check type. The window offers only the check types the engine registers. A new check type is a code change (see the technical spec §A7).
- A separate "Test on a client" screen (RU-43). The approval gate in scope 4 uses a run on a client instead.
- Law tables and the Format Register (SPEC-010 coded fields are a separate spec; see Design notes).
- Any use of a language model in writing or checking a rule (invariant 2).

## Design notes

- **Delete is disable.** The request said "delete". RU-41 says nothing is deleted, and invariant 6 says a rule that fires wrongly is disabled, not tuned. So the Delete action is **Disable with a reason**, and the rule can be enabled again. If the CA wants a real delete of a rule that was never run, that is a separate decision (see open questions).
- **Edit creates a version, never overwrites.** The same rule of RU-12 and RU-41.
- **Form, not YAML.** The CA never types YAML (RU-21). The YAML file is a generated artefact; a hand edit of it is picked up by the loader on the next start, as today.
- **Server writes through the engine.** `recon-server` calls `recon-engine rule-save` with the form as JSON on standard input, the same pattern as `decide` and `classify`. The engine does the validation and the write, so the engine stays the only code that loads rules.
- **No network.** The window loads its lists from the running server only. No CDN, no web font (invariant 9).

## Acceptance criteria

- [ ] AC-1 — The Rules tab has an **Add rule** button, and each rule row has **Edit** and **Disable** (or **Enable again** for a disabled rule). Keyboard alone reaches all of them.
- [x] AC-2 — Filling the window for a valid rule saves `build/office-rules/<year>/<module>/<ID>/rule.yaml` with its fixtures, status **testing**, `source: "office"`. The file passes `rule-lint` and the strict loader without change.
- [x] AC-3 — The window refuses, naming the field and in plain words: an unknown fact or record type; no law reference; no pass fixture or no fail fixture; a tolerance or amount that is not a number; an ID that already exists for the year.
- [x] AC-4 — A rule whose pass fixture fails, or whose fail fixture passes, is refused and nothing is written.
- [x] AC-5 — Editing a built-in rule writes `<ID> v1-office.1` to the office folder. The file under `content/rules/` is byte-identical before and after (checked by a test). "Reset to standard rule" makes the built-in version the one in use again.
- [x] AC-6 — Editing an office rule creates a new version. Earlier versions are listed, and switching back to one is recorded.
- [x] AC-7 — **Disable** asks for a reason and stops the rule from running; the files and versions remain. **Enable again** asks for a reason and restores it.
- [x] AC-8 — Each create, edit, disable and enable writes one `audit_log` row with the reason, and the table rejects UPDATE and DELETE (test).
- [x] AC-9 — A run after an office rule is added or edited records that rule's version and source (RU-53), and two runs of the same files give identical results (RU-51).
- [x] AC-10 — Amounts are stored as integer paise; a test enters "1.005" and "0.10" and checks the stored paise.
- [x] AC-11 — No network: the architecture guard passes, and the Rule Builder loads no external script, font or image (test on the built page).
- [x] AC-12 — `go run ./cmd/recon-dev rule-lint`, `go test -count=1 ./...`, `npm test` and `npm run lint` pass; the Test Pack A Pass 1 score is unchanged (22 of 30, critical 4 of 4, 0 false positives).
- [x] AC-13 — Every file this spec created is added to `docs/Tool1-File-Guide.docx` in plain language; `tool1-technical-spec.md` §B9 is updated where the design changed.
- [ ] AC-14 — Adding a rule opens with the steps panel showing; the **?** button hides and shows it, and its state is announced (`aria-expanded`). Keyboard alone reaches the button. (`ruleBuilder.test.tsx`: the help and required-field test; the keyboard part is manual.)
- [x] AC-15 — Required fields carry the required marker and optional ones do not, and the marker does not change the label's text. (`ruleBuilder.test.tsx`: the help and required-field test)
- [ ] AC-16 — The red * sits on the same line as its label in the browser, at phone width and desktop width. (manual: open the Add rule window and look; not checked by a test, because the tests do not lay out the page.)
- [ ] AC-17 — The worked test rule in the section "Worked example" can be entered exactly as written and saves as testing with the message shown there. (manual, on the built app with a fresh database.)
- [ ] AC-18 — A testing rule shows **Approve** only when its fixtures pass and a run on a client has included it. Approving asks for the Partner's name and a reason, sets the rule to active, and writes one audit entry with both. A rule that has not been run on a client is refused with the reason. (`go test` for the approval gate, and the web test for the window; manual check on the app.)
- [x] AC-19 — An active rule runs in a normal run, with no `--include-testing`, and the run records its version and status as active (RU-53). A testing rule still runs only with `--include-testing`.
- [x] AC-20 — Disabling an active rule needs a reason and returns it to disabled; enabling it again returns it to testing, and approval must be given again. Each step writes an audit entry.
- [ ] AC-21 — Opening any rule on the Rules tab, engine or rule book, shows every section listed in scope 8 for it in a centred popup, and the summary scrolls so nothing is cut off; the rules table has no left-right scroll on a screen wider than 760px. A rule-book ID such as `RC-ACC-003` opens (`GET /api/v1/rules/RC-ACC-003` is 200). (web test, `go test`; manual look on the app.)
- [x] AC-22 — A Partner edits a rule-book rule: the save stores a new office version with schema `rulestore/1.0` that reads back through the strict reader, sets `last_modified_by`, `last_modified_on` and `change_note`, writes one audit entry, and leaves the file under `docs/Recon_Tool_YAML_Rule_Store/` byte-identical (test). A save without a reason, or with a changed ID, type or domain, is refused and nothing is written.
- [x] AC-23 — **Reset to rule-book version** puts the rule-book version back in use, with an audit entry; loading the rule book again keeps an office version in use until it is reset (test).
- [x] AC-24 — A Partner disables a rule-book rule with a reason and enables it again with a reason (status active); each writes an audit entry, and loading the rule book again does not change the status a person set (test).
- [x] AC-25 — A manager or staff user sees no Edit, Disable or Enable on a rule-book rule, and the server refuses the edit with 403 (test).
- [x] AC-26 — `go test -count=1 ./...`, `npm test`, `npm run lint` pass; the Test Pack A Pass 1 score is unchanged.


Scope 9:

- [x] AC-27 — Each row of the Rules table has a tick box, and the heading's tick box selects every rule the filters show, across pages; the selection survives a page change and is cleared by **Clear selection**. Keyboard alone reaches all of them (web test).
- [x] AC-28 — With rules selected, **Disable (n)** and **Enable again (n)** count only the selected rules each can change, and are hidden from a role without the permission (web test).
- [x] AC-29 — Disabling or enabling several rules with one reason changes all of them in one transaction, with one audit entry per rule; an empty reason, an unknown rule, or a rule already in that state refuses the whole request and changes nothing (Go test, engine and API).
- [x] AC-30 — A Manager, Staff or Read-only user is refused the bulk routes with 403 (the route matrix test), and more than 500 rules in one request is refused.

## Verification

```
go run ./cmd/recon-dev rule-lint
go test -count=1 ./internal/rules/... ./internal/store/... ./internal/api/... ./cmd/recon-engine -run 'RuleSave|RuleDisable|RuleVersion|Audit|OfficeRules' -v
go test -count=1 ./internal/archtest/ -v
go test -count=1 ./cmd/recon-engine -run TestTestPackAPass1 -v
cd web && npm test && npm run lint && npm run build && cd ..
```

Manual, on the built app (`start-local.cmd`):

1. Rules tab → Add rule → fill a rule with a pass and a fail fixture → save. The rule shows as testing with its source as office.
2. Try each refusal in AC-3 and check the message names the field.
3. Edit a built-in rule, then **Reset to standard rule**.
4. Disable a rule with a reason, check a run no longer uses it, then enable it again.
5. Do steps 1 to 4 with the keyboard only (AC-1).

### Build notes, scope 9 (2 November 2026)

- **Engine:** `rule-disable` and `rule-enable` take `--rules ID,ID,...` (or `--rule` for one, as before; not both). Every rule is checked and changed in one transaction, with one audit entry each and the same time; a refusal of any rule returns before the commit. Refusals added: `NO_RULES_SELECTED`, `TOO_MANY_RULES` (over 500), and an ID named twice (`BAD_REQUEST`). With `--rules` the JSON lists `rules: [{rule_id, version, status}]`; with `--rule` it keeps its old shape.
- **Server:** `POST /api/v1/rules/bulk/disable` and `/bulk/enable`. IDs are checked against the engine and rule-book ID patterns before the engine is called. `ALREADY_DISABLED` and `NOT_DISABLED` now answer 409 for one rule as well (they were 422).
- **Screen:** a tick box in the Rule column of each row and in its heading (select or clear every rule the filters show, part-ticked when only some are). Ticking does not open the rule. The bar over the table shows the count, how many are hidden by the filters, **Disable (n)**, **Enable again (n)** and **Clear selection**. The popup lists the IDs it will change and asks for one reason. Shown only with the server running and to a role holding `disable_rules` or `activate_rules`.
- **Tests:** Go `TestRuleSwitchSeveralAllOrNone` (`cmd/recon-engine/rulebulk_test.go`), `TestBulkRuleSwitchThroughServer` (`internal/api/rulebulk_test.go`) and the route matrix (43 routes); Vitest `web/src/rulesBulk.test.tsx` (6 tests).
- **Not yet done:** the new files are not in `docs/Tool1-File-Guide.docx`, and the screen was not checked by eye in the running app.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| 2026-10-29 | Implementer (Claude) | none (not committed) | Built with SPEC-009 not Done (started on the user's instruction; SPEC-009's manual checks AC-7 and AC-10 still open, its status untouched). Verified by tests and commands: AC-2 to AC-12. AC-13 is ticked: each file this spec created has an entry in docs/Tool1-File-Guide.docx (section 18), and §B9 of tool1-technical-spec.md is updated. Open: AC-1 (keyboard alone reaches every action, manual), and the manual steps 1 to 5 of Verification, which were not run. AC-10: "1.005" is refused rather than stored (a half paisa is not an amount), "0.10" is stored as 10 paise. Status stays In progress. |
| 2026-10-29 | Implementer (Claude) | none (not committed) | Added after the build: the How to add a rule panel and its ? button, the required-field marks (section 7), the worked example, and AC-14 to AC-17. AC-15 is verified by web/src/ruleBuilder.test.tsx (89 web tests pass). AC-14 keyboard, AC-16 layout and AC-17 entry are manual and not yet run. |
| 2026-10-29 | Implementer (Claude) | none (not committed) | Partner approval (scope 4): `rule-approve` (name and reason; gate: the fixtures pass in the engine, and a client's finished run included testing rules and used this version, from `run_rules`), approvable flags on `rules` and `rule-get`, Approve on the Rules tab and in the window, success message per the worked example. AC-19 and AC-20 ticked: `TestActiveRuleRunsWithoutIncludeTesting`, `TestDisabledActiveRuleMustBeApprovedAgain`, `TestSwitchingVersionEndsApproval`. AC-18 left unticked until the manual approval check in the app is run: its gate is covered by `TestRuleApprovalGate`, `TestRuleApprovalNeedsFixturesToPass`, `TestRuleApproveThroughServer` and the approval tests in `web/src/ruleBuilder.test.tsx`. Status stays In progress. |
| 2026-11-02 | Implementer (Claude) | none (not committed) | Scope 8 added on the user's request (whole rule in the summary; Partner edits, disables and enables rule-book rules). New: `rulebook-save`, `rulestore.EncodeYAML`, migration 0009 (`rule_state.status_set_by`), `PUT /api/v1/rules/{id}/rulebook` (activate_rules), `web/src/screens/RuleDetail.tsx`. AC-22 to AC-26 ticked: `TestRulebookEditIsAnOfficeVersion`, `TestRulebookDisableAndEnableStandAcrossReload`, `TestRuleGetReturnsTheWholeRulebookRule`, `TestRulebookRuleThroughServer`, the two scope-8 tests in `web/src/rulesView.test.tsx`; gofmt, vet (host and Windows), `go test -count=1 ./...`, `npm run lint`, `npm test` (230) and `npm run build` pass; Test Pack A unchanged (22 of 30, critical 4 of 4, 0 false positives). AC-21 is left open until the summary is checked by eye in the app. The new files are not yet in docs/Tool1-File-Guide.docx (to add before the spec is marked Done). Status stays In progress. |

## Worked example: a test rule (for AC-17)

Enter these values in the Rule Builder, in the order the form shows them. Fields marked * are required. If `RECON-DEP-901` already exists, use `RECON-DEP-902`.

| Section | Field | Enter |
|---|---|---|
| Who and what | Your name * | any name |
| | ID * | `RECON-DEP-901` |
| | Title * | `Test rule: allowed depreciation equals the schedule total` |
| | Rule text * | `Test rule for the Rule Builder. Compares allowed depreciation in the computation with the fixed-asset schedule total.` |
| Where it applies | Module * / Category * | `tieout` / `R1: Same figure in two places` |
| | Tax year * / Valid from * | `2025-26` / `2025-04-01` (valid to empty) |
| Figures and rows | figure 1: Name / Kind / Which figure | `a` / `A figure` / `computation.allowed_depreciation` |
| | figure 2: Name / Kind / Which figure | `b` / `A figure` / `computation.fixed_asset_schedule.total` |
| Check | Check type * / Tolerance | `equal` / `1.00` |
| Outcome, legal basis and document | Severity * / Review document * | `high` / `computation` |
| | Legal basis: Act * / Section * | `Income-tax Act, 1961` / `32` |
| Fixtures | pass: Kind / Name / figures | `Pass` / `values_agree`; `computation.allowed_depreciation` = `112450.50` and `computation.fixed_asset_schedule.total` = `112450.50`, both `FY2025-26`, file `comp_2025-26_agrees.json`, locators `$.business_income.allowed_depreciation` and `$.fixed_assets.schedule_total` |
| | fail: Kind / Name / figures | `Fail` / `differs_by_10`; the same figures, except the second is `112460.50` and its file is `comp_2025-26_differs.json` |

Click **Save as testing**. The expected message is: "Saved RECON-DEP-901 version 1 as testing, kept in …. It is waiting for approval. Only a Partner's approval makes it active; until then it runs only when a run includes testing rules. The Rules tab labels it "testing"." The rule then appears on the Rules tab with source "Office edit" and status "Testing".
## Open questions for the CA and the project owner

1. **Delete.** Confirm that "delete" means disable with a reason (RU-41), not removal of files. Only a rule that was never run could be removed safely.
2. **Who may disable.** The technical spec leaves open whether managers may disable as well as partners (open question T-Q3). This spec lets anyone with access disable, until roles exist.
3. **Who may approve.** Approval is given by a name typed in the window until logins and roles exist (see scope 4). Confirm the CA accepts this interim step, or whether approval should wait for the roles spec.
4. **Spec number clash.** SPEC-005 and SPEC-008 reserve SPEC-010 for "coded fields and law tables". This spec takes SPEC-010 as requested, so the coded-fields work needs a new number (for example SPEC-012). The reservations in `SPEC-005` and `SPEC-008` are not changed by this spec.
