# Tool 1 — Reconciliation & Verification Engine
### Requirements Specification v0.3

| | |
|---|---|
| **Date** | 13 October 2026 |
| **Based on** | Product & Engineering Specification v0.2 (8 Oct 2026) + decisions of 12–13 Oct 2026 |
| **Status** | Draft — for approval |
| **Audience** | Business analyst, CA, development team (built with Claude Code) |

> All figures in this document are illustrative. No real client data appears here.

---

## 1. Purpose and scope

### 1.1 What Tool 1 is

Tool 1 (Priority 1) is a **reconciliation-only** application for an Indian CA firm. It reads the files that the firm's existing systems produce, converts them into standard facts, runs **rules** over those facts, and reports — to the rupee — every place where figures disagree.

Three principles define it:

| Principle | Meaning |
|---|---|
| **Reconciliation only** | Tool 1 compares and reports. It does not prepare returns, file returns, keep books, give tax advice, or manage the practice. |
| **Logic by rule** | Every check is a rule. No reconciliation logic is hard-coded per client. Adding a check means adding a rule, not changing the program. |
| **Scalable** | New input formats, new rules, new tax years and more clients are added without redesign (see §9). |

### 1.2 Out of scope for Tool 1

| Excluded | Reason |
|---|---|
| Return preparation and e-filing | Owned by existing tax suites |
| Bookkeeping | Owned by the accounting package |
| Practice management (tasks, planners, follow-ups) | Separate product category |
| Audit opinion, working papers, risk assessment | Judgment work, not reconciliation |
| Tax advice or "this should be X" statements | Tool reports differences only (§6.9) |
| Direct government-portal connections (GST, TRACES, e-filing) | Requires internet; tool is offline |
| AI deciding or altering findings | Results must be deterministic |

---

## 2. Operating environment

| ID | Requirement |
|---|---|
| ENV-01 | Installed on **one main desktop** in the CA office (Windows). |
| ENV-02 | Staff use it from their own PCs through a **web browser over the office network**. Nothing is installed on staff PCs. |
| ENV-03 | **No internet** at any time. All screens, fonts and libraries are bundled inside the application. |
| ENV-04 | All client data stays on the main desktop. |
| ENV-05 | Software updates and rule packs arrive **by USB**, are signature-checked, and can be rolled back. |
| ENV-06 | The main desktop must have a fixed network name/IP and stay on during working hours. |
| ENV-07 | Daily backup to a second drive / external disk / office NAS. |

---

## 3. Users and roles

| Role | Can do |
|---|---|
| **Partner** | Everything; approve/activate/disable rules; apply rule packs; export; manage users |
| **Manager** | Run reconciliations; decide findings; create/edit draft rules; export |
| **Staff** | Upload files; run reconciliations; decide findings; create draft rules. Cannot delete findings, export, or activate rules |
| **Read-only** | View only |

---

## 4. Input requirements *(updated in v0.3)*

### 4.1 Input types

| ID | Input type | Source | Notes |
|---|---|---|---|
| IN-01 | **JSON** | Prepared or exported **manually by CA staff**, mainly from the firm's tax suite (computation, tax audit report, ITR utility JSON) | Primary input for income-tax documents |
| IN-02 | **Excel / CSV** (.xlsx, .xls, .csv) | Tally exports, GST portal downloads, working schedules prepared by CA staff, client-supplied sheets | |
| IN-03 | **Word** (.docx) | Documents prepared by CA staff or received from clients that contain **tables** of figures | Only tables are read. Free-running text is not interpreted. |
| IN-04 | **Tally exports** (XML preferred, Excel accepted) | Trial balance, ledger master, monthly summary, Day Book, sales/purchase/payroll registers | Exported by staff; optional live pull later |
| IN-05 | **PDF** (text-based) | Statutory forms where no JSON/Excel exists | Fallback only; layout-preserving extraction. **Scanned images are not supported in Tool 1.** |
| IN-06 | Portal downloads | GSTR-1/1A/3B/2B/9, IMS, 26AS/AIS, TRACES files, EPF ECR, ESIC | Downloaded by staff and uploaded |

### 4.2 Multiple files

| ID | Requirement |
|---|---|
| IN-10 | A client engagement can have **any number of files**, of any of the types above, in one run. |
| IN-11 | The same document type can arrive as **several files** (e.g. computation split into parts, 12 monthly GST files, several Tally companies/branches). The tool combines them by client, tax year, period and GSTIN/branch. |
| IN-12 | A newer version of the same document is stored as a **new draft**, not an overwrite, so drafts can be compared (§7, draft comparison). |
| IN-13 | An identical file uploaded twice is detected (file fingerprint) and skipped. |
| IN-14 | Each upload shows a **validation report**: file recognised as which format, figures extracted, anything unreadable. |

### 4.3 Format library — how the system learns market formats

The formats will be **obtained from the market systems** (tax suites, Tally, GST portal, TRACES, EPFO/ESIC and others) as sample files, and the system will be **set up ("trained") on those formats**.

In this specification, **"trained" means configured, not machine-learned**:

| ID | Requirement |
|---|---|
| IN-20 | For each market format (and each version of it), a **format profile** is created from real sample files. The profile states where every figure sits in that format (JSON path, sheet/column, table/row, page/position) and which **standard fact name** it maps to (§5). |
| IN-21 | Format profiles are **deterministic**: the same file always produces the same facts. No AI or statistical guessing. |
| IN-22 | Every format profile is **tested** against its sample files before use; the extracted figures must match a hand-verified list. |
| IN-23 | The tool **auto-detects** which format profile a file matches. If no profile matches, the file is **rejected with a clear message** — the tool never guesses. |
| IN-24 | Adding a new format, or a new version of an existing format, means **adding a format profile only**. Rules do not change. |
| IN-25 | A **Format Register** lists every supported format: source system, document, version, file type, profile version, date added. |
| IN-26 | Until real samples are received, development uses a **generic synthetic format** (documented in `docs/fixtures.md`). |

**Format collection checklist** (to be filled as samples arrive):

| Source system | Documents | File type | Samples received |
|---|---|---|---|
| Tax suite(s) used by the firm — *names to be confirmed* | Computation, tax audit report, ITR JSON | JSON / PDF | ☐ |
| Tally (version to be confirmed) | TB, ledger master, monthly summary, Day Book, registers | XML / Excel | ☐ |
| GST portal | GSTR-1/1A, 3B, 2B, 9, IMS | JSON / Excel | ☐ |
| Income-tax portal / TRACES | 26AS / AIS (Form 168), conso, justification, challans | Text / JSON / PDF | ☐ |
| EPFO / ESIC | ECR, contribution files | Text / Excel | ☐ |
| CA staff working files | Schedules, reconciliations | Excel / Word | ☐ |

---

## 5. Standard fact vocabulary

| ID | Requirement |
|---|---|
| FV-01 | Every figure extracted from any file is stored as a **fact**: standard name, value (integer paise), period, and **locator** (file, page/sheet/path, row). |
| FV-02 | Fact names come from a controlled **fact vocabulary** (`docs/fact-vocabulary.md`), e.g. `tar.depreciation_schedule.total`, `computation.allowed_depreciation`. |
| FV-03 | Format profiles may only write facts in the vocabulary; rules may only read facts in the vocabulary. Both are validated. |
| FV-04 | The vocabulary is the bridge between formats and rules: **formats change → profiles change; law changes → rules change; neither affects the other.** |

---

## 6. Rule requirements

### 6.1 Storage format

| ID | Requirement |
|---|---|
| RU-01 | Rules are stored as **YAML** files (decision confirmed 13 Oct 2026). |
| RU-02 | To avoid YAML misreading, rules follow a **strict subset**: YAML 1.2 parser; tax years, dates, IDs and section numbers always quoted as text; unknown keys rejected; every file validated against a fixed rule schema before it can be saved or loaded. |
| RU-03 | Each rule file carries a plain-English field `rule_text_en` and a `notes` field, shown on screen, so the CA never needs to read the YAML itself. |
| RU-04 | Complex calculations (e.g. MAT book profit, dependency chains) use **Starlark** (sandboxed, deterministic) referenced from the YAML rule. |

### 6.2 Rule content

Every rule must contain:

| Field | Example |
|---|---|
| ID, version | `RECON-DEP-002`, `1` |
| Source | `builtin` or `office` |
| Status | draft / testing / active / disabled |
| Title and `rule_text_en` | Depreciation mismatch — "Depreciation total in the tax audit report must equal 'Allowed depreciation' in the computation." |
| **Law reference** (mandatory) | Act, section/rule, notification/circular (optional) |
| Tax year, valid from / valid to | `"2025-26"`, `"2025-04-01"` / blank |
| Inputs | Fact names A, B (or more) |
| Check type | equal, sum-equals, greater-or-equal, within-due-date, all-periods-present, field-present, threshold, formula (Starlark) |
| Tolerance | e.g. `100` paise (₹1) or a percentage |
| Severity | high / medium / low |
| Created / changed / approved by | user and timestamp |

### 6.3 Built-in rules and office rules

| ID | Requirement |
|---|---|
| RU-10 | The tool ships with a **starter rule library** (built-in rules) covering tie-out, new heads, GST, TDS and payroll, each with its law reference. The firm's CA reviews and approves it before go-live. |
| RU-11 | The CA can **create new rules** and **edit existing rules** from the Rule Builder screen (§6.4). |
| RU-12 | Editing a built-in rule saves an **office version** on top; the original is kept. "Reset to standard rule" restores it. |
| RU-13 | Rule packs arriving by USB never overwrite office rules. If a pack changes a rule the CA has edited, both versions are shown and the CA chooses. |

### 6.4 Rule Builder (configurable UI page)

| ID | Requirement |
|---|---|
| RU-20 | The CA builds a rule by **choosing options on a form** (document, figure, check type, tolerance, severity, law reference, tax year, plain-English wording). |
| RU-21 | The system **generates the YAML automatically** from the form. The CA does not type YAML. |
| RU-22 | Figure dropdowns list **only facts the tool can already read** from supported formats, so every saved rule can run. |
| RU-23 | Free-text English is stored as the rule's description only; it is **not interpreted** by the system. |
| RU-24 | Complex (Starlark) rules: the CA can change **values** (rates, limits, dates) only; logic changes come from the developer via rule pack. |
| RU-25 | **Test on a client**: before activation, a draft rule can be run against a past client to preview what it would flag. |

### 6.5 Indian tax law alignment

| ID | Requirement |
|---|---|
| RU-30 | Every rule references its legal basis (Income-tax Act 1961 / Income-tax Act 2025, CGST/SGST/IGST Acts and Rules, EPF Act, ESI Act, Labour Codes, relevant notifications/circulars). |
| RU-31 | **Law tables** hold values that change with law — TDS sections/thresholds/rates, GST rates by date, due dates, e-invoice limits, old↔new Income-tax Act section mapping — each with valid-from / valid-to dates and a law reference. |
| RU-32 | Rules read changeable values from law tables, not hard-coded numbers. A law change = a new law-table row with an effective date; older tax years keep using older values. |
| RU-33 | Both regimes are supported side by side: FY 2025-26 (Income-tax Act 1961, Forms 3CA/3CB/3CD, 26AS, 24Q/26Q/27Q) and Tax Year 2026-27 onward (Income-tax Act 2025, Form 26, Form 168, Forms 138/140/144). |
| RU-34 | Legal correctness of rules and law-table values is the **CA's professional responsibility**; the tool records who approved each rule and value. |

### 6.6 Rule governance

| ID | Requirement |
|---|---|
| RU-40 | Lifecycle: **Draft → Test → Partner approves → Active**; any rule can be **Disabled** with one click. |
| RU-41 | Every version is kept; rollback to any earlier version. |
| RU-42 | Every create/edit/approve/disable is written to the audit log with reason. |
| RU-43 | Every built-in rule ships with one passing and one failing test file; office rules must pass "Test on a client" before activation. |
| RU-44 | A rule that produces wrong findings in use is disabled, not tuned silently. |

### 6.7 Rule execution

| ID | Requirement |
|---|---|
| RU-50 | Rules run inside the offline engine; no AI and no network. |
| RU-51 | Same files + same active rules + same law tables → **identical results every time**. |
| RU-52 | If a fact needed by a rule is missing, the tool reports **"cannot check — figure not found in <document>"**, not a mismatch. |
| RU-53 | Each run records which rule versions and law-table values were used. |

### 6.8 Rule scale targets

| Milestone | Rules |
|---|---|
| Usable alpha | 80–120 |
| Version 1 | 300–400 |
| Mature | 600+ |

### 6.9 Output wording

The tool states only computed differences. Findings never contain "should", "incorrect" or "must be". Example:

```
[RECON-DEP-002]  Depreciation mismatch
  Tax audit report, depreciation schedule total : 1,12,450
  Computation, "Allowed depreciation"           : 1,15,200
  Difference                                    :     2,750
  Source: TAR_v3.pdf p.7 | comp_20261008.json $.allowed_depreciation
  Rule basis: Income-tax Act, 1961 – Sec 32
```

---

## 7. Reconciliation functions (all rule-driven)

| ID | Function | Description |
|---|---|---|
| RC-01 | **Cross-document tie-out** | Same figure across documents; recomputed derived figures; dependency propagation; internal consistency; cross-form conflicts; sequence/completeness; field-domain checks; books vs third-party data; mandatory field present elsewhere; version regression (rule categories R1–R10 of spec v0.2) |
| RC-02 | **New-head / completeness** | New heads, vanished heads, broken monthly recurrence, step changes, unclassified heads → **worklist** for classification |
| RC-03 | **GST reconciliation** | Books vs GSTR-1/1A vs 3B; purchases vs 2B/IMS vs 3B; ITC at risk; supplier-wise summary; control checks (Rule 37/37A, e-invoice 30-day rule, 3-year filing bar) |
| RC-04 | **TDS reconciliation** | Books vs challans vs quarterly statements vs TRACES; missing TDS by expense head; TDS credit vs 26AS/AIS vs turnover |
| RC-05 | **Payroll reconciliation** | Salary vs EPF ECR vs ESIC vs salary TDS; due dates; missing months; wage-definition check (CA rule) |
| RC-06 | **Draft-vs-draft comparison** | Changes between drafts, and dependent figures that did not change |

---

## 8. Findings and workflow

| ID | Requirement |
|---|---|
| FD-01 | Each finding has a **stable ID** (rule + client + period + match key); re-runs update it, never duplicate it. |
| FD-02 | Each finding shows both values, the difference, both source locations and the rule's law reference. |
| FD-03 | Statuses: Open → Decided (Fix / Not an issue / Ask client) → Awaiting re-check → Resolved / Reopened / Carried forward. |
| FD-04 | "Not an issue" stays closed unless the underlying values change. |
| FD-05 | Uploading a corrected file re-runs the affected rules and resolves or reopens findings automatically. |
| FD-06 | Findings are grouped by the **document to be fixed**. |
| FD-07 | Screens: **Season board** (all clients), **Client file** (findings), **Worklist** (new heads), **Rule Builder**, **Law tables**, **Format Register**, Admin. |
| FD-08 | Exports: Excel and watermarked PDF; client query letter. |

---

## 9. Scalability requirements

| ID | Dimension | Requirement |
|---|---|---|
| SC-01 | **New formats** | Added as format profiles (§4.3) without changing rules or engine code |
| SC-02 | **New rules** | Added via Rule Builder or rule pack without a new application version |
| SC-03 | **New tax years / law changes** | Added via tax-year-tagged rules and dated law-table rows |
| SC-04 | **Clients** | 50 clients reconciled concurrently |
| SC-05 | **Volume per client** | Up to 200,000 vouchers, 12 GST returns, 4 TDS statements, 12 payroll files, multiple JSON/Excel/Word/PDF files |
| SC-06 | **Speed** | One client < 3 min; 50 clients < 30 min (8-core / 16 GB); re-run after one changed file < 20 s |
| SC-07 | **Rule count** | Engine performance holds at 600+ active rules |
| SC-08 | **Modularity** | Check modules (§7) are independent; a new reconciliation area can be added as a new module using the same fact store and rule engine |

---

## 10. Non-functional requirements

| ID | Area | Requirement |
|---|---|---|
| NF-01 | Accuracy | All money stored as **integer paise**; no floating point |
| NF-02 | False alarms | < 2 wrong findings per 100; < 5 findings on a clean file |
| NF-03 | Determinism | No AI in the finding path; identical input → identical output |
| NF-04 | Security | Logins and roles; session timeout; encrypted database (SQLCipher, AES-256); HTTPS on the office network; engine contains no network code |
| NF-05 | Audit | Append-only, hash-chained audit log of every view, decision, export, rule and law-table change |
| NF-06 | Integrity | Rule packs and updates signed (Ed25519) and verified offline |
| NF-07 | Backup | Daily encrypted backup; tested restore |
| NF-08 | Usability | Every finding shows its arithmetic and sources; keyboard shortcuts; Hindi/English labels |
| NF-09 | Data protection | Supports the firm's DPDP obligations (retention, client-wise export and deletion, access logs). No compliance claim until legal opinion. |

---

## 11. Technology stack

| Layer | Choice |
|---|---|
| Engine and server | Go — Windows .exe; engine built without network packages |
| Database | SQLite + SQLCipher; money in integer paise |
| Rules | **YAML** (strict subset, schema-validated) + Starlark for complex rules |
| UI | React + TypeScript, built into the .exe, opened in a browser over the office network |
| JSON input | Go standard library |
| Excel / CSV input | excelize |
| Word input | .docx table extraction (Office Open XML) — library to be selected in the Word-adapter phase |
| PDF input | Poppler `pdftotext -layout` (bundled) |
| Tally input | XML/Excel exports; optional live pull via a separate collector process |
| Signing | Ed25519 |
| Packaging | Windows installer + portable zip |

---

## 12. Architecture summary

```
Files (JSON / Excel / Word / Tally / portal / PDF)
        │
        ▼
Format detection ──► Format profile (per market format & version) ──► Standard facts (fact store)
                                                                          │
Active rules (built-in + office, YAML/Starlark, by tax year) ──┐          │
Law tables (values by valid-from date) ────────────────────────┴──► Rule engine
                                                                          │
                                                                          ▼
                                     Findings ──► Staff decisions ──► Re-check on new upload
```

Processes: `recon-server` (UI/API, office network) · `recon-engine` (no network) · `recon-collector` (optional Tally pull).

---

## 13. Phase plan (current)

| # | Build | Done when |
|---|---|---|
| 1 | Skeleton, fact store, YAML rule engine, one rule end to end (CLI) | Depreciation mismatch caught; clean file gives zero findings |
| 2 | First format profile (generic JSON → real tax-suite JSON when samples arrive) + 80–120 rules | R1–R10 caught, zero false alarms |
| 3 | Excel and Word adapters + Tally exports; New-head check | Worklist generated from two years of ledgers |
| 4 | UI — season board, client file, worklist | Staff decide findings in the browser |
| 4b | Rule Builder, law tables, Format Register | CA creates/edits a rule, tests it on a client, Partner activates it |
| 5 | GST, TDS, payroll recon (portal file profiles) | Gaps found on test files |
| 6 | Encryption, roles, audit log, installer (licence check — see §15) | Security checklist passes |
| 7 | Rule-pack loader (signature, USB, rollback, office-rule conflict handling) | Signed pack installed by USB and rolled back |
| 8 | Voice | Deferred |

---

## 14. Assumptions

| # | Assumption |
|---|---|
| A1 | Development uses synthetic data only until anonymised real samples are provided. |
| A2 | The first format profile is built against a generic tax-suite JSON format until real samples arrive. |
| A3 | CA staff can prepare/export JSON, Excel and Word files consistently using a written export guide. |
| A4 | Word files contain figures in tables; figures in free text are not read. |
| A5 | PDFs are text-based; scanned PDFs are out of scope for Tool 1. |
| A6 | The firm's CA approves the starter rule library and law-table values before go-live. |

---

## 15. Open questions

| # | Question | Blocks |
|---|---|---|
| Q1 | Which tax suite(s) and versions does the firm use? Samples of their JSON output? | Phase 2 |
| Q2 | Which Excel/Word documents prepared by CA staff are in scope? Samples? | Phase 3 |
| Q3 | Tally version and export steps? | Phase 3 |
| Q4 | Licence check — keep (if Tool 1 will be installed at more firms) or remove (single office)? | Phase 6 |
| Q5 | Windows test PC and person to run each build? | Phase 1 |
| Q6 | Number of concurrent staff users? | Phase 4 sizing |
| Q7 | DPDP legal opinion obtained? | Any compliance claim |

---

## 16. Changes from spec v0.2

| Area | v0.2 | v0.3 |
|---|---|---|
| Scope | Product with market validation, pricing, vendor rule service | **Tool 1 / P1: reconciliation only**, rule-driven, scalable; commercial sections removed |
| Inputs | Accounting ODBC, Excel/CSV, portal JSON/PDF, tax-suite output | Adds **manually prepared JSON by CA staff, Excel and Word, multiple files per client**; Tally via exports |
| Formats | Adapters per software | **Format library**: profiles built ("trained") from market-system samples; deterministic; register of supported formats |
| Rules | YAML authored by vendor CA | **YAML retained**, strict subset; **Rule Builder** UI for CA to create/edit; built-in vs office rules; law reference mandatory; law tables; approval workflow |
| Law | Not addressed | Income-tax Act 2025 and new form numbers supported alongside 1961 Act |
| Deployment | Firm premises, optional inbound internet | **Fully offline**; updates and rule packs by USB |
| Licence | Included | Open question (Q4) |
