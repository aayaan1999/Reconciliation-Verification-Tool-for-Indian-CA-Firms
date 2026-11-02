# Estimation and dependencies

Purpose: one place for the build order, the critical path to 30 October and 6 November 2026, outside dependencies and risks. It summarises [specs/README.md](../../specs/README.md), the [implementation plan](../Reconciliation%20%26%20Verification%20Tool%20-%20Implementation%20plan.md) (§4 modules, §6 timeline, §9 what we need from the firm, §12 risks) and [tool1-technical-spec.md](../../tool1-technical-spec.md) (§B16 build order, §B17 risks). Where this file and a source differ, the source wins.

## Fixed dates

| Date | What | Source |
|---|---|---|
| 14 October 2026 | Building started module by module (no sprints) | Plan §14 decisions log |
| 30 October 2026 | Final acceptance, stakeholder demo, go/no-go | Plan §6, §10 |
| 2–6 November 2026 | Second release: logins and roles, drafts, decisions on findings, rule updates by USB | Plan §8 |
| 6 November 2026 | Hard cap. Any slip past it is reported the same day | Plan §6, §12 |

Weekends (24–25 October, 31 October–1 November) are not working days. The plan has no spare day, so any delay is reported the same day (plan §6).

## Spec dependency chain

Specs are built in number order; a spec starts only when its dependencies are Done ([specs/README.md](../../specs/README.md), "How to use").

```
SPEC-001 rule store in YAML ──► SPEC-003 rules into DB, one engine run ──┐
SPEC-002 developer tools ────────────────────────────────────────────────┴─► SPEC-004 JSON files, screens, rule problems
                                                                              └─► SPEC-005 filing pack JSON, Test Pack A Pass 1
                                                                                   └─► SPEC-006 check types, Form 67/26AS, decisions, Worklist
                                                                                        └─► SPEC-007 negative controls, one defect at a time
```

| Spec | Depends on | Status (from the index) |
|---|---|---|
| SPEC-001 | — | Done |
| SPEC-002 | — | Done (WiX licence decision open) |
| SPEC-003 | SPEC-001 | Done |
| SPEC-004 | SPEC-003, SPEC-002 | Done |
| SPEC-005 | SPEC-004 | Done |
| SPEC-006 | SPEC-005 | Done |
| SPEC-007 | SPEC-006 | Not started |

The status in each spec file is the source of truth; this table is a copy. No spec is Verified yet. Verified is set only by a second person (the CA for rule and law-table content).

## Module order (implementation plan §4, §6)

| Module | Dates | Notes |
|---|---|---|
| 1 Core engine and data model | 14 Oct | |
| 2 Rule book in YAML | 14–30 Oct, alongside | SPEC-001 |
| 3 Loading and checking the rules | 14 Oct | SPEC-003 |
| 4 Reading client files (JSON, Excel, PDF) | 14–19 Oct | SPEC-004 covers JSON; Excel and PDF adapters are separate specs (not yet written) |
| 5 Saving the data (SQLite) | 14–19 Oct | SPEC-003, SPEC-004 |
| 6 Running the rules | 14–21 Oct | SPEC-003, SPEC-006 |
| 7 Findings, consistent results, error handling | 22–23 Oct | |
| 8 Putting it together, testing, demo | 26–30 Oct | |
| 9 Screens in the browser | 14–30 Oct, alongside | SPEC-004 |
| 10 Word, Tally and portal downloads | 17–23 Oct | |
| 11 New and missing heads | 20–23 Oct | Worklist from ledger CSVs built in SPEC-006 part 3 |
| 12 GST, TDS and payroll reconciliation | 22–27 Oct | GST invoice matching started in SPEC-004 |
| 13 Rule Builder for the CA | 22–27 Oct | Read-only Rules screen in SPEC-004 part 6 |
| 14 Security, backups, exports, installer | 26–28 Oct | Encryption chosen (ADR 0002) but not yet wired into the store |
| 15 Pilot and go/no-go | 29–30 Oct | |
| 16 Logins and user roles | 2–3 Nov | Second release |
| 17 Upload history and drafts | 2–4 Nov | Second release |
| 18 Staff decisions on findings | 3–4 Nov | Second release; decisions already recorded by SPEC-006 part 3 |
| 19 Rule updates by USB | 4–5 Nov | Second release; signed packs (tech spec §B10) |
| 20 Second release and sign-off | 5–6 Nov | |

The status table in plan §14 shows most modules as Not started; it was not updated as specs were finished. Use [specs/README.md](../../specs/README.md) for what is built.

Effort per module: Open. The plan gives dates, not person-day estimates, and one developer carries the build (plan §12, first risk).

## Critical path to 30 October

1. SPEC-007: prove zero false positives on Test Pack A, Pass 1 (plan §10: "the clean set gives no findings").
2. Module 14: encryption wired into the store, daily backup with a tested restore, exports, Windows installer (needs the WiX licence decision).
3. Module 8: integration, re-test and the demo run of plan §7 (13 steps).
4. Module 15: install on the firm's office desktop, staff run real client files, go/no-go.
5. CA approval of the rules shown in the demo. Without it, rules stay `testing` or `disabled` and the demo runs with `--include-testing` only.

Modules not yet covered by any spec (Excel/PDF/Word/Tally/portal adapters, Rule Builder editing, exports, installer) have no spec file yet. Which of them are in the 30 October demo, and which move after it, is Open; the plan §6 timeline still lists all of them before 30 October.

## Critical path to 6 November

Logins and roles (§B12.1) → drafts (Module 17) → decisions closing on a corrected file (Module 18) → signed rule pack by USB with rollback (§B10, Module 19) → full re-test with logins on → sign-off. If 30 October is no-go, 2–6 November is used for pilot fixes and these modules are re-planned with the business (plan §8).

## External dependencies

| Dependency | Needed by | State | Source |
|---|---|---|---|
| CA approval of rules, with approver name and date | 16 Oct | Open: all 304 rule-store rules load `disabled`; all `recon.rule/1` rules load `testing`; approver name pending | Plan §9; SPEC-001 AC-3; SPEC-003 |
| CA review of the rule problem list | 20 Oct | Open | Plan §9; `recon-dev rulestore-problems` |
| Tax software name, version and anonymised JSON/Excel/PDF samples | 17 Oct | Open: profiles built on synthetic files marked synthetic | Plan §9; SPEC-004; requirements Q1 |
| Word, Tally, GST portal, TRACES, EPF, ESIC samples | 17 Oct | Open | Plan §9; requirements Q2, Q3 |
| Rounding of negative amounts to ₹10 | 20 Oct | Open | Plan §9 |
| Office desktop, backup drive, pilot staff, real files | 28 Oct | Open | Plan §9 |
| Names and roles of everyone who logs in | 30 Oct | Open | Plan §9 |
| USB drive and CA time to approve a signed rule update | 4 Nov | Open | Plan §9 |
| Poppler 26.09.0 `pdftotext` (GPL, separate program) | PDF adapter | Vendored in `third_party/poppler/`; licence obligations before sharing with other firms | [ADR 0003](../architecture/decisions/ADR-0003-bundle-poppler.md); [licences](../licences.md) |
| WiX installer licence (fee EULA from v6) | Module 14 | Open: pay, pin v5 or build from source | SPEC-002; tech spec §B17 T1b |
| HTTPS on the LAN (import firm CA or accept warning) | Module 14/16 | Open (T-Q2) | Tech spec §B17 |
| Licence check in the product (Q4) | Phase 6 | Open | Requirements §15 |

## Risks

| Risk | What we do | Source |
|---|---|---|
| Twenty modules in three and a half weeks for one developer | Modules side by side; sample data first; report slips the same day | Plan §12 |
| Pilot has only two days on real files | Few clients; a wrong rule is switched off, not adjusted | Plan §12; project invariant 6 |
| PDF and portal files misread (column order) | Text-based files only, keep layout, test against hand-checked figures, refuse scans | Plan §12; tech spec T4, T7 |
| Real samples arrive late | Synthetic files in the same shape; swap in real ones | Plan §12 |
| Rule book problems | Daily problem list to the CA; only approved rules switched on | Plan §12 |
| Pilot fixes compete with second-release work | Pilot fixes first | Plan §12 |
| Logins and drafts change demo screens | Build on existing screens; re-run every 30 October test on 6 November | Plan §12 |
| A library pulls in network code | Architecture guard on every push (ADR 0001) | Tech spec T1 |
| Finding states not signed off by the CA | Open (T6) | Tech spec §B7, §B17 |
