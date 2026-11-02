# Test Pack A — "Nimbus Logic Systems Private Limited", AY 2026-27

A synthetic filing pack with **30 planted defects** and **16 negative controls**, for regression-testing the reconciliation engine.

Every entity, PAN, person and figure in this pack is **fictional**. No real client data is present. The defects, however, are modelled on real ones observed in a live filing pack, so they reproduce the *shape* of what the tool must catch.

---

## The fictional assessee

| | |
|---|---|
| Name | Nimbus Logic Systems Private Limited |
| PAN / TAN / GSTIN | AABCN4821K / JPRN09418C / 08AABCN4821K1ZP |
| Status | Domestic company, DPIIT-recognised start-up claiming s.80-IAC |
| AY / PY | 2026-27 / 2025-26 |
| Turnover | ₹6,42,18,350 |
| PBT | ₹3,71,24,600 |
| Auditor | Sheolekar & Company (fictional) |

Chosen deliberately: a MAT-paying company with a large profit-linked deduction, foreign income with a foreign tax credit, and related-party payments. That combination is where cross-form inconsistencies concentrate.

---

## Files

| File | Purpose |
|---|---|
| `01_tax_audit_report_3CD_v1.txt` / `.pdf` | Tax audit report, **draft 1** |
| `02_tax_audit_report_3CD_v2.txt` / `.pdf` | Tax audit report, **draft 2** — for version-diff testing |
| `03_mat_report_29B.txt` / `.pdf` | MAT report |
| `04_ftc_form_67.txt` / `.pdf` | Foreign tax credit statement |
| `05_computation_of_income.txt` / `.pdf` | Computation, MAT credit table, fixed-asset schedule |
| `06_tax_credit_statement_26AS.txt` / `.pdf` | Tax credit statement |
| `07_expense_ledger_FY2024-25.csv` | Prior-year ledger, 264 rows |
| `08_expense_ledger_FY2025-26.csv` | Current-year ledger, 284 rows |
| `09_filing_pack_structured.json` | The whole pack as structured JSON — mimics tax-suite output |
| `10_EXPECTED_FINDINGS.csv` | **Answer key — 30 findings** |
| `11_NEGATIVE_CONTROLS.csv` | **16 things that must NOT be flagged** |

Both `.txt` and `.pdf` are provided for every form. The PDFs use a monospace column layout that mimics utility output — **use them to test layout-preserving extraction.** A plain text dump will misread the clause 18 depreciation table, which is exactly the failure mode to guard against. The `.txt` files are the ground truth for what correct extraction should yield.

---

## How to run the test

### Pass 1 — ingestion only
Feed `09_filing_pack_structured.json`. This isolates **rule-engine** correctness from **parser** correctness. All 30 findings should fire.

### Pass 2 — PDF ingestion
Feed the six `.pdf` files plus both ledger CSVs. Same 30 findings expected. Any divergence from Pass 1 is a parser bug, not a rule bug.

### Pass 3 — version diff
Feed `01_..._v1` then `02_..._v2` as successive drafts of the same document. F11 and F12 should fire.

### Pass 4 — negative controls
Confirm nothing in `11_NEGATIVE_CONTROLS.csv` is flagged. **This matters more than the hit rate.** Per the false-positive budget, fewer than 2 false positives per 100 findings; a rule that fires on a negative control twice gets disabled, not tuned.

---

## Scoring

| Metric | Target |
|---|---|
| Critical findings detected (F01, F02, F03, F10) | **4 / 4** |
| All findings detected | ≥ 26 / 30 |
| False positives against negative controls | **0** |
| Pass 1 vs Pass 2 divergence | 0 findings |

A run that catches 28 findings but raises 3 false positives is a **worse** result than one that catches 24 with none.

---

## Defect map

| Category | Findings |
|---|---|
| Same number, many documents (R1) | F01, F13, F19 |
| Derived-value recomputation (R2) | F03, F04, F07, F08, F22, F26 |
| Dependency propagation (R3) | F02 |
| Internal logical consistency (R4) | F09 |
| Cross-form semantic conflict (R5) | F05, F10, F20 |
| Sequence / completeness (R6) | F14, F15, F21, F23, F24, F25 |
| Field-domain check (R7) | F06, F17 |
| Books vs third-party data (R8) | F18 |
| Mandatory field present elsewhere (R9) | F16 |
| Version regression (R10) | F11, F12 |
| New-head / completeness worklist | F27, F28, F29, F30 |

---

## The four that matter most

**F01 + F02 — the propagation pair.** Depreciation is ₹1,35,862 in the audit report *and* in the computation's own fixed-asset schedule, but ₹1,38,310 in the computation's business-income working. Difference ₹2,448. Correcting it raises business profit by ₹2,448, so the profit-linked deduction must move with it — and does not. **This is the defect class no existing filing suite catches**, because it is an inconsistency between two outputs of the same tool.

**F03 — the MAT understatement.** Form 29B adds back only the current tax provision, omitting ₹9,68,390 of income tax sitting in other expenses above PBT. The tell is that Sl.18 book profit comes out exactly equal to PBT. The computation's own MAT block uses the correct ₹3,80,92,990. MAT understated by **₹1,61,644** — the only planted defect with real money attached.

**F10 — the characterisation conflict.** The audit limb describes the receipts as professional; the deduction claimed on the same signed form is available only on business profits. Two mutually exclusive characterisations, twelve clauses apart.

**F27–F30 — the worklist.** Seven new heads, two vanished, two broken recurrences, one step change. This is the completeness problem that no cross-form rule can reach, and the one practitioners name as their own pain.

---

## Deliberate traps

Three things look wrong and are not. If the engine flags them, the rule is too loose.

| Trap | Why it is correct |
|---|---|
| Clause 18 additions at half rate on 09/10/2025 | 174 days — under 180, so half rate is right. A naive "October is more than six months" rule fires wrongly |
| 15/09/2025 addition at **full** rate | 198 days — over 180. The mirror of the above |
| Clause 44 columns in v2 | They foot exactly. The finding is the *change from v1*, not a footing error |

---

## Known limits of this pack

- Single assessee. Does not exercise the 50-client concurrency target
- No GST return files, PF ECR or ESIC challans — the statutory-reconciliation module needs a second pack
- No accounting-package export; ledgers are supplied as CSV
- Ledger amounts carry ±4% random monthly variation, so ledger totals do **not** tie to the clause 44 figure. That is intentional: the ledger tests new-head detection only. A future pack should make them tie
