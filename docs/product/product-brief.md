# Product brief

This page says what the product is, who uses it, what problem it solves, what it deliberately does not do and how success is measured. It summarises the product and engineering spec v0.2 ([recon-tool-spec.md](../../recon-tool-spec.md)), the Tool 1 requirements v0.3 ([requirements-spec-tool1.md](../../requirements-spec-tool1.md), which wins for Tool 1), the build units in [specs/](../../specs/README.md) and the open gaps in `Recon-Tool-Understanding-and-Gaps.docx` (repository root). Where they differ, the requirements v0.3 win; see [requirements.md](requirements.md).

All figures, PANs and names on this page are illustrative.

## The product in one paragraph

An offline consistency layer for an Indian CA firm. It reads what the firm's existing systems produce (the accounting package, usually Tally; the tax suite; the GST portal; TRACES; EPFO and ESIC), turns every figure into a standard fact with its source, runs rules over those facts and reports, to the rupee, every place where two figures disagree. It also lists expense and income heads that are new, vanished or changed compared with the prior year, for staff to classify. It runs on one office computer and no client data leaves the office (§1, §16; requirements §1.1).

## The problem

- A client's filing pack is prepared across four or five disconnected systems. The same number appears in several places and nothing checks that it is the same number everywhere (§1).
- Expense and income heads that are new this year have no prior-year counterpart, so they are missed and hunted for at year-end closure (§1, §7).
- Today this reconciliation is manual, slow and error-prone. The interviewed practitioner estimated a 30 to 35 per cent workload reduction if it were automated to the rupee (§0). This estimate comes from one firm (gaps doc, Part B.1).

## Who uses it

| Role | What they do with it (requirements §3) |
|---|---|
| Partner | Looks at the Season board; approves, activates and disables rules; applies rule packs; exports; manages users |
| Manager | Runs reconciliations, decides findings, creates and edits draft rules, exports |
| Staff | Uploads files, runs reconciliations, decides findings, creates draft rules; cannot delete findings, export or activate rules |
| Read-only | Views only |
| The firm's CA (rule approver) | Reviews and approves the rule book and law-table values (RU-10, RU-34); signs off the finding states (tech spec §B17 T6) |

Roles and logins are planned (Phase 6, tech spec §B12). Today `recon-server` has no logins and a decision records a name typed on screen (SPEC-006 part 3).

## Design principle

The tool asserts only what it can compute (§2). A finding shows both values, the difference and both sources, and the rule's law reference (FD-02). It never contains "should", "incorrect" or "must be", never states a correct value and never makes an applicability or characterisation judgment (requirements §6.9). In the reference review, deterministic findings were wrong 0 times out of about 20; judgment findings were wrong or withdrawn 6 times out of about 9 (§2).

## Scope

In scope for Tool 1 (requirements §7):

| Area | Function |
|---|---|
| RC-01 Cross-document tie-out | The same figure across documents, recomputed figures, dependency propagation, internal consistency, completeness, field domains, books vs third-party data, version regression (rule categories R1 to R10, §6.2) |
| RC-02 New-head / completeness | New, vanished, broken-recurrence, step-change and unclassified heads, as a **worklist**, not findings (§7) |
| RC-03 GST | Books vs GSTR-1/1A vs 3B; purchases vs 2B/IMS vs 3B |
| RC-04 TDS | Books vs challans vs quarterly statements vs TRACES; 26AS/AIS vs turnover |
| RC-05 Payroll | Salary vs EPF ECR vs ESIC vs salary TDS |
| RC-06 Draft comparison | Changes between drafts, and dependent figures that did not change |

## Explicit non-goals

From §3.2 of the v0.2 spec, extended by requirements §1.2:

| Not done | Why |
|---|---|
| Return preparation and e-filing | Owned by the existing tax suites |
| Bookkeeping | Owned by the accounting package |
| Practice management, deadline tracking, document chasing | A separate, crowded category |
| Audit opinion, working papers, risk assessment | Judgment work, not reconciliation |
| Tax advice, or "this should be X" statements | Liability, and the §2 principle |
| Direct connections to government portals (GST, TRACES, e-filing) | Needs internet; Tool 1 is offline (requirements §1.2) |
| AI deciding or altering findings | Results are deterministic (§5.4) |
| Reading scanned PDFs or images | Out of scope for Tool 1 (IN-05, A5) |

## Tool 1 and later tools

- **Tool 1 (Priority 1)** is reconciliation only, rule-driven and fully offline. Updates and rule packs arrive by USB (requirements §1, ENV-03, ENV-05). This is what is being built.
- The v0.2 spec describes a wider product: a vendor-run rule-pack service with a daily scheduler and an inbound update channel (§4.2, §9), licensing and pricing (§12), voice disposition (M8) and an optional local model for classification suggestions (§5.4). Requirements v0.3 removed the commercial sections and made the licence check an open question (Q4). Voice is deferred (requirements §13, phase 8). No later tool is specified yet: **Open**.

## Success measures and gates

| Measure | Target | Source | Status |
|---|---|---|---|
| Phase 1 gate | Reproduce every R1 to R10 defect with zero false positives; a clean file gives zero findings | §14, requirements §13 phases 1 and 2 | Partly met: see Test Pack A below |
| False positives | Fewer than 2 per 100 findings | §6.4, NF-02 | Measured on test packs only |
| Findings on a clean file | Fewer than 5 | §6.4, NF-02 | Clean samples give zero (SPEC-003) |
| Speed | One client under 3 minutes; 50 clients under 30 minutes; re-run after one changed file under 20 seconds | SC-06 | Not yet measured (benchmark planned, tech spec §A9) |
| Rule count | 80 to 120 for a usable alpha; 300 to 400 for version 1 | requirements §6.8 | The rule book has 304 rules, described only; the runnable engine rules are in `content/rules/` |

**Test Pack A** (`testdata/testpack_nimbus/`, a fictional company, Nimbus Logic Systems) is scored on every test run (`go test -count=1 ./cmd/recon-engine -run TestTestPackAPass1 -v`). Pass 1 (JSON and the two ledger CSVs) scores 22 of 30 findings, critical 4 of 4, with 0 false positives. 22 is the most the JSON can reach: F11 and F12 need two drafts of the audit report, and F25 and F26 need fields only in the PDF (SPEC-005, SPEC-006). The ledger CSVs give exactly F27 to F30 on the Worklist. SPEC-007 (not started) turns the 16 negative controls into checks, so that zero false positives is proven rather than inferred.

## Open items that affect the product

- Every requirement traces to one practitioner and one filing pack (§13 risk 7; gaps doc B.1). CLAUDE.md records Phase 0 as cleared on 13 October 2026; tech spec §B17 T-Q4 still lists it as a question.
- Tax suite names and real sample files: **Open** (requirements Q1 to Q3). Development uses synthetic formats (A1, A2).
- Clients whose Tally books are kept off-site (gaps doc B.2): covered by the manual export-and-upload path; no further design yet.
- Licence check (Q4) and the DPDP legal opinion (Q7): **Open**.
