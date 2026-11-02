# Handoff — SPEC-010 — Rule Builder: add, edit and disable rules from the Rules tab

| | |
|---|---|
| **From** | implementer |
| **To (next agent)** | test-engineer |
| **Date** | 2026-10-29 |
| **Spec** | `specs/SPEC-010-rule-builder-add-edit-disable.md` (status as read at the start: Not started; set to In progress). SPEC-009 is not Done; the user asked for SPEC-010 to start anyway, and SPEC-009's status was not touched. |
| **Part / criteria covered** | All of scope 1 to 6; AC-2 to AC-12 ticked by test or command; AC-1 and AC-13 open (see below) |

## What was done

- Office rules: a rule is saved as `build/office-rules/<tax-year>/<module>/<ID>/rule.yaml` with its fixtures; built-in files are never written (a test checks their hashes).
- The save path (`recon-engine rule-save`) checks the form, then its fixtures, then loads the file with the strict loader, runs every fixture through the engine, and only then writes the database row, the audit entry and the files, in one transaction. A refusal names each field and writes nothing.
- Edit of a built-in rule writes `<ID> v1-office.1`; an edit of an office rule writes the next version; earlier versions can be listed and switched back to, and "Reset to standard rule" uses the built-in version again.
- Disable and enable need a reason; a disabled rule does not run; its files and versions stay.
- Every create, edit, disable, enable, switch and reset writes one row to `audit_log` (migration 0007, hash chain, UPDATE and DELETE refused by triggers).
- `load-rules` and `run` read `--office-rules`; a run records the office version and source through `run_rules` and `rule_versions`.
- Server endpoints under `/api/v1/rules…` and `/api/v1/rule-options`; the Rules tab has Add rule, Edit, Disable / Enable again and a versions section; the Rule Builder window is a modal.
- Decisions the spec did not cover: see "Open issues" and §B9 of `tool1-technical-spec.md` ("As built (SPEC-010)").

## Files changed

| File | Change (added / edited / removed) | Why |
|---|---|---|
| `internal/audit/chain.go`, `chain_test.go` | added | hash chain of the audit log and its verification |
| `internal/store/migrations/0007_audit_log.sql` | added | the `audit_log` table with append-only triggers |
| `internal/store/audit.go`, `audit_test.go` | added | write, read and test the audit log; rule versions and state |
| `internal/store/rules.go` | edited (rewritten) | per-source revisions; an office version stays in use over a new built-in text; state and version operations |
| `internal/store/store_test.go` | edited | migration count 6 to 7 |
| `internal/rules/builder.go`, `generate.go`, `categories.go`, `builder_test.go` | added | the form, its plain-words checks, YAML writers, the categories list, and tests (including every built-in rule round-tripped through the form) |
| `internal/rules/rule.go` | edited | the `1-office.N` version form, for office rules only |
| `internal/engine/fixtures.go` | added | runs one fixture through the engine for the save check |
| `content/rules/categories.yaml` | added | the categories the window offers (did not exist; to be confirmed by the CA) |
| `cmd/recon-engine/officerules.go`, `rulesave.go`, `rulesave_files.go`, `ruleadmin.go`, `rulesave_test.go` | added | the commands rule-save, rule-get, rule-disable, rule-enable, rule-versions, rule-options; their tests |
| `cmd/recon-engine/main.go` | edited | dispatch and usage for the new commands |
| `cmd/recon-engine/loadrules.go`, `run.go` | edited | `--office-rules` |
| `internal/api/rules.go`, `rules_test.go` | added | the rule endpoints and their tests |
| `internal/api/api.go` | edited | config `OfficeRules`, routes, `--office-rules` for load-rules and run, refusal problems passed through |
| `cmd/recon-server/main.go` | edited | `--office-rules` flag |
| `web/src/ruleForm.ts`, `web/src/staff.ts` | added | pure helpers of the window; the name kept in this browser |
| `web/src/screens/RuleBuilder.tsx` | added | the Rule Builder window and the switch-off dialog |
| `web/src/ruleBuilder.test.tsx` | added | browser tests (AC-1 presence, AC-3 refusals, AC-7 reason, AC-11 no external loads) |
| `web/src/screens/Rules.tsx` | edited (rewritten) | Add rule, row actions, the rule panel's actions and versions; the read-only view unchanged |
| `web/src/api.ts`, `web/src/types.ts`, `web/src/App.tsx`, `web/src/index.css` | edited | rule client calls and types; rules refreshed after a change; styles |
| `tool1-technical-spec.md` | edited | §B9 "As built (SPEC-010)"; §B1.1 audit row marked as built |
| `specs/SPEC-010-rule-builder-add-edit-disable.md`, `specs/README.md` | edited | status In progress; criteria ticked; completion record |
| `docs/Tool1-File-Guide.docx` | edited | entries for the new files (see the guide's new section) |
| `docs/agents/handoff-SPEC-010.md` | added | this note |

New files to add to `docs/Tool1-File-Guide.docx`: all of the "added" rows above (done by the implementer, see AC-13 below).
New packages classified in `internal/archtest/archtest_test.go`: none (no new package; `internal/audit` was already classified as server-side and the engine imports it only through the store, which the guard allows).
New rules added to `testdata/testpack_nimbus/score.yaml`: none (no new built-in rule; the window creates office rules in `build/`).

## Commands run

| Command | Result (exit code, key output) |
|---|---|
| `gofmt -l .` | prints nothing (run after the last Go change) |
| `go vet ./...` and `GOOS=windows GOARCH=amd64 go vet ./...` | exit 0, both |
| `go test -count=1 ./...` | exit 0; every package `ok` including `cmd/recon-engine` (108.6 s), `internal/api`, `internal/archtest`, `internal/rules`, `internal/store`, `internal/audit`, `web` |
| `go test -count=1 ./internal/archtest/ -v` | PASS, all guard tests (including `TestEngineHasNoNetwork`, `TestEngineBinaryHasNoNetworkCode`, `TestEngineSideAvoidsServerPackages`) |
| `go run ./cmd/recon-dev rule-lint` | `all 50 rules passed` |
| `go run ./cmd/recon-dev rule-lint build/ac2-check/office-rules/...` (an office rule saved by `rule-save` into a scratch folder) | `OK RECON-OFF-001 v1 (1 pass, 1 fail fixtures)`, `all 1 rules passed` |
| `go test -count=1 ./cmd/recon-engine -run TestTestPackAPass1 -v` | `22 of 30 findings (22 reachable from the JSON), critical 4 of 4, 0 false positives; 13 of 16 negative controls checked by rules, 1 error(s) in the pack itself`; PASS |
| `go test -count=1 ./internal/rules/... ./internal/store/... ./internal/api/... ./internal/audit/... ./cmd/recon-engine -run 'RuleSave\|RuleDisable\|RuleVersion\|Audit\|OfficeRules\|Rule\|Builder\|Fixture\|Categor\|Form\|Office\|Version' -v` | PASS for every test named below |
| `go test -count=1 -tags ui ./web/` | `ok` |
| `go build -tags ui -o build/ ./cmd/recon-engine ./cmd/recon-server` | exit 0 |
| in `web/`: `npm test` | `Test Files 9 passed (9)`, `Tests 88 passed (88)` |
| in `web/`: `npm run lint` | exit 0; one warning, on the existing `setSelected` effect in `Rules.tsx` (not from this spec's code) |
| in `web/`: `npm run build` | exit 0; a chunk-size warning (the bundle is about 1.08 MB) |
| PowerShell: each created file name searched in the text of `docs/Tool1-File-Guide.docx` after the update | all 23 found; the document XML parses; 29 tables (24 before) |
| `web/dist/index.html` | loads only `/assets/…` (same origin) for script and stylesheet; the URL strings in the bundle are XML namespaces and a React documentation link, which are not fetched |

Named tests that passed (Go): `TestRuleSaveWritesOfficeRule`, `TestRuleSaveRefusesWithTheFieldNamed` (9 cases), `TestRuleSaveRefusesAFixtureTheRuleGetsWrong` (2), `TestRuleSaveStoresAmountsAsPaise`, `TestRuleSaveEditsBuiltInRuleAsOfficeVersion`, `TestRuleVersionsKeepEachEdit`, `TestRuleDisableAndEnableNeedAReason`, `TestAuditLogRecordsEachChange`, `TestRuleSaveRunRecordsTheOfficeVersion`, `TestOfficeRulesLoadWithLoadRules` (cmd/recon-engine); `TestAuditChainVerifiesAndRefusesEdits`, `TestAuditDetailsAreCanonical`, `TestOfficeVersionStandsOverBuiltIn`, `TestSetRuleStatusKeepsVersions` (internal/store); `TestVerifyAcceptsAValidChain`, `TestVerifyFindsAChangedEntry`, `TestVerifyFindsAGap`, `TestHashCoversEveryField` (internal/audit); `TestFormProblems`, `TestFixtureProblems`, `TestEveryBuiltInRuleRoundTripsThroughTheForm`, `TestOfficeVersionOnlyForOfficeRules`, `TestFixtureWrittenBackReadsTheSame`, `TestCategoriesListEveryBuiltInCategory` (internal/rules); `TestRuleSaveThroughServer`, `TestRuleDisableThroughServer`, `TestOfficeRuleListedThroughServer` (internal/api).

## Acceptance criteria

| Criterion | Status (met / not met / not checked) | Evidence |
|---|---|---|
| AC-1 | not checked (partly) | Presence of Add rule, Edit and Disable / Enable again is tested (`ruleBuilder.test.tsx`, "Add rule and each rule row has Edit and Disable"). Keyboard alone reaching each action is manual (Verification step 5) and was not run. |
| AC-2 | met | `TestRuleSaveWritesOfficeRule`; `rule-lint` of an office folder saved by the engine (command above) |
| AC-3 | met | `TestRuleSaveRefusesWithTheFieldNamed` (unknown fact, unknown record type, no law, no pass fixture, no fail fixture, tolerance not a number, tolerance with more than two decimals, ID exists, ID shape); `TestFormProblems`, `TestFixtureProblems`; browser test "a refused save names each field" |
| AC-4 | met | `TestRuleSaveRefusesAFixtureTheRuleGetsWrong` (pass fixture that fails; fail fixture that passes); nothing written and no database row (asserted) |
| AC-5 | met | `TestRuleSaveEditsBuiltInRuleAsOfficeVersion`: `1-office.1` written; `hashTree` of `content/rules` identical before and after; reset makes the built-in version in use again |
| AC-6 | met | `TestRuleVersionsKeepEachEdit`: two versions listed, the earlier text kept under `history/1/`, switch recorded in `audit_log` |
| AC-7 | met | `TestRuleDisableAndEnableNeedAReason` (reason required both ways; a run after disable does not use the rule; files kept); browser test "the Switch off button waits for a reason" |
| AC-8 | met | `TestAuditLogRecordsEachChange` (one row each, actor and reasons in details, UPDATE and DELETE refused, chain verifies); `TestAuditChainVerifiesAndRefusesEdits` |
| AC-9 | met | `TestRuleSaveRunRecordsTheOfficeVersion` (two runs byte-identical; `run_rules` joined to `rule_versions` gives `1-office.1`, `office`) |
| AC-10 | met (with a decision) | `TestRuleSaveStoresAmountsAsPaise`: `0.10` stored as `paise: 10`; `1.005` refused naming `check.tolerance`. The spec's "checks the stored paise" is read as: the half paisa is refused, not stored. Please confirm with the CA. |
| AC-11 | met | `internal/archtest` PASS; browser test "the window loads no external script, style, font or image"; `web/dist/index.html` check (above) |
| AC-12 | met | `go run ./cmd/recon-dev rule-lint` (all 50 passed); `go test -count=1 ./...` (exit 0); `npm test` (88 passed); `npm run lint` (exit 0, one warning, pre-existing); Test Pack A 22 of 30, critical 4 of 4, 0 false positives |
| AC-13 | met | every created file name (23) found in the text of `docs/Tool1-File-Guide.docx` (section 18, 32 entries in 5 tables; the XML parses); §B9 "As built (SPEC-010)" in `tool1-technical-spec.md` |

## Invariant check

- Findings show both values, the difference and both sources; no "should": not affected (no finding text changed; the window's own wording was checked against the banned phrases by the form's `BannedIn` check on titles and rule text).
- Money in integer paise, no floats: yes. Tolerances and fixture amounts are read with `internal/money` in strict mode; no float is used in the new Go code. The web code only types amounts as text.
- No clock, randomness or map-order dependence: yes for the engine's outputs. The audit time is taken in `cmd/recon-engine` (`nowUTC`) and only written to `audit_log`; it never reaches a rule file, a fixture or a run. The fixture and rule writers sort their keys.
- No network import on the engine side (archtest passes): yes.
- No real client data added: yes. All figures are illustrative (`computation.allowed_depreciation` values of 112,450.50 and 115,200.50 are the synthetic samples used elsewhere in the repo).
- Writes: `rule-save` writes files only under the `--office-rules` folder, a temporary folder it removes, and the `--json` path; SQL is explicit and parameterised with `?` placeholders.

## Open issues

1. **AC-1 keyboard, and manual steps 1 to 5 of Verification.** Not run (cannot be run by the implementer). Every control is a standard button, select or input, so Tab reaches them; the test engineer should confirm on the built app.
2. **Guide wording.** The guide entries are the implementer's plain-language text; the test engineer may reword them.
3. **SPEC-009 is not Done.** Its AC-7 and AC-10 manual checks are open. This spec was started on the user's instruction.
4. **`content/rules/categories.yaml` descriptions** are the implementer's wording for the RC-01 list; the CA should confirm them.
5. **Edit window limits.** A built-in tolerance given as a percentage (`bp_of_larger`) cannot be edited in the window; the window refuses the edit rather than dropping the tolerance. No built-in rule uses it today.
6. **Versions and the folder.** Switching to an earlier version changes only the database; the folder keeps the latest saved text, so the folder and the version in use can differ until the next save. The Rules tab shows the version in use.
7. **Lists not yet filtered by RU-22.** The window offers every non-retired figure of the vocabulary, not only those an installed profile can produce.
8. **Authorisation.** There are no logins, so every endpoint accepts any caller on the address the server listens on. Activation (Partner only) is not built and the window says the rule waits for approval.
9. **Chunk-size warning in `npm run build`** (bundle about 1.08 MB, 172 kB gzipped). Not addressed here; not compared with the bundle size before this spec.
10. **Spec reference.** SPEC-010 cites "§B3" for `audit_log`; the table is in §B1.1 and the hash chain is described in §B9 (as built). §B3 is the fact vocabulary and was not changed.
