# Reconciliation & Verification Tool for Indian CA Firms
### Product and Engineering Specification v0.2

**Date:** 8 October 2026
**Audience:** Engineering lead — internal review
**Status:** Draft. Not validated beyond one practitioner interview and one reference filing pack.

> **Note on data in this document.** All figures, entity names, PANs and jurisdictions below are **illustrative**. They preserve the *shape* and *arithmetic relationships* of defects observed in a real filing pack, but no real client data appears anywhere in this document. Do not treat any number here as a real case.

---

## 0. Where the requirements come from

| Source | Contribution |
|---|---|
| **Practitioner interview — Firm A** (mid-size CA firm, multi-partner, Oct 2026) | Their filing suites do no upstream/downstream reconciliation; they ingest Excel and emit JSON for portal upload. All reconciliation is manual, slow, error-prone. Expense and income heads that are new this year get missed and have to be hunted at year-end. Reconciliation needed across GST, TDS, payroll (PF/ESIC) and the accounting system. Practitioner's own estimate: **30–35% workload reduction** if automated to pinpoint accuracy. **Must run offline** — client data cannot go to an internet-connected AI service. |
| **Reference filing pack review** (one private limited company, AY 2026-27, Sep–Oct 2026) | A manual cross-document review producing ~20 deterministic findings (0 wrong) and ~9 judgment findings (6 wrong or withdrawn). Defect catalogue in §6, with illustrative values. |
| **Market research, Oct 2026** | §13. Sourcing quality flagged where weak. |

Items marked **[ASSUMPTION]** are unverified and must be confirmed before build.

---

## 1. The problem

An Indian CA firm prepares a client's annual filing pack across four or five disconnected systems — an accounting package for books, a tax suite for the computation and statutory forms, the GST portal, TRACES, EPFO and ESIC. Each produces its own output. **Nothing checks that those outputs agree with each other.** The same number appears in five places and nobody verifies it is the same number in all five.

Separately, expense and income heads that are new this year have no prior-year counterpart to be checked against, so they are silently omitted and discovered — if at all — during year-end closure.

This tool is the missing consistency layer. It does not prepare filings and does not replace the accounting or tax suites. It reads what those systems produce and reports, to the rupee, every place they disagree.

---

## 2. The design principle that decides everything

> **The tool asserts only what it can compute. Everything else it phrases as a question.**

Evidence from the reference review:

| Finding type | Method | Count | Wrong or withdrawn |
|---|---|---|---|
| Recomputation and cross-document tie-out | Deterministic | ~20 | **0** |
| Applicability, characterisation, "should it be X" | Judgment | ~9 | **6** |

A CA who receives three wrong findings stops reading the fourth. Judgment features are how this product dies.

**Correct output:**

```
[RECON-DEP-002]  Depreciation mismatch
  Tax audit report, depreciation schedule total : 1,12,450
  Computation, "Allowed depreciation"           : 1,15,200
  Difference                                    :     2,750
  Source: TAR_v3.pdf p.7 | comp_20261008.pdf p.1
```

**Prohibited output:**

```
This is incorrect. Depreciation should be 1,12,450.
```

The tool never says "should." It says "these two differ."

---

## 3. Scope

### 3.1 In scope

| Module | Function |
|---|---|
| **M1 — Ingestion** | Accounting-package exports, Excel/CSV, portal downloads, and the PDF/JSON output of the firm's tax suite |
| **M2 — Cross-form tie-out** | Every number appearing in more than one artefact is a node; every pair that must agree is an edge; broken edges are reported |
| **M3 — Completeness / new-head detection** | Year-on-year ledger diff surfacing heads present this year but absent last year, and vice versa |
| **M4 — Statutory reconciliation** | GST, TDS, payroll, against books |
| **M5 — Version diff & propagation** | Diff draft *n* against *n−1*; report what changed, and what depended on it and did **not** change |
| **M6 — Finding lifecycle** | ID, locator, arithmetic, state, audit trail |
| **M7 — Rule-pack updates** | Signed, inbound-only rule updates (§9) |
| **M8 — Voice disposition** | Indian-language voice input for finding disposition, on-premise model |

### 3.2 Explicitly out of scope

| Excluded | Why |
|---|---|
| Form preparation and e-filing | Entrenched desktop suites own this |
| Practice management, deadline tracking, document chasing | Crowded category, many funded players |
| Bookkeeping / ledger entry | The accounting package owns this |
| Audit opinion, risk assessment, working papers | AI audit platforms occupy this |
| Tax advice of any kind | Liability, and §2 |

---

## 4. Three contradictions in the brief, and how they are resolved

### 4.1 "Offline" vs "daily scheduler for government rule updates"

Incompatible only if data flows both ways. Resolution: **asymmetric network**.

| Direction | Allowed | Content |
|---|---|---|
| Outbound from firm | Restricted | Licence heartbeat, rule-pack version number, anonymous crash counter. **Never** client data — no PAN, no amounts, no names, no file contents |
| Inbound to firm | Yes | Signed rule packs and software updates |

The updater is a **separate OS process with its own network namespace**. The reconciliation engine has no network capability compiled in — not firewalled, *absent*. This is demonstrable to a CA in thirty seconds and is the entire security pitch.

Air-gapped mode: rule packs downloadable as a signed `.rulepack` file and side-loaded via USB. Cheap to support, and some firms will insist on it.

### 4.2 "System automatically updates its knowledge base and flags inconsistencies accurately"

**This cannot be done reliably and must not be promised.** Turning a statutory notification or circular into an executable validation rule is a judgment task. An auto-generated wrong rule fires across every installed firm simultaneously.

Honest design, three stages:

| Stage | Who | Output |
|---|---|---|
| **Detect** | Daily scheduler on **vendor** infrastructure, not the firm's | Polls official sources, diffs against last snapshot, classifies the change |
| **Author** | A qualified CA on the vendor payroll | Reads the change, writes or amends the rule plus test cases, approves |
| **Ship** | Build pipeline | Signed, versioned rule pack with changelog and effective-from date |

Firm sees: *"Rule pack 2026.10.3 available — 4 changes. View changelog."* They choose when to apply. **Rule packs are versioned by assessment year**, because a prior-year reassessment must be checked against that year's rules.

**Consequence: a permanent CA headcount.** It is the largest recurring cost and never goes away. §12 prices it.

### 4.3 "₹500/month" vs "profitable"

See §12. At ₹500/month you need roughly **850 paying firms** to break even. The number is probably wrong, and §12 shows the arithmetic so the decision can be made explicitly.

---

## 5. Technology stack

### 5.0 The constraints that choose the stack

Before any technology decision, these are fixed by §1, §4.1 and §11. Every choice in §5.2 is justified against them — not against general popularity.

| # | Constraint | Source | Consequence |
|---|---|---|---|
| **C1** | Runs entirely on firm premises, no cloud | Practitioner requirement | No managed services, no cloud DB, no SaaS dependency anywhere in the data path |
| **C2** | Installs on a Windows machine in an office with no IT staff | Observed reality of the target firm | No runtime prerequisites, no Docker, no "install Python first", no DBA |
| **C3** | The engine must be *provably* unable to reach the network | §4.1 — this is the entire security pitch | Language and libraries must make capability absence demonstrable, not just configured |
| **C4** | Deterministic and reproducible — same input, same output, every time | §2 and §6.4 | No floating-point drift in money maths, no map-iteration-order dependence, no LLM in the decision path |
| **C5** | Rules must be readable and reviewable by a CA, and shippable without recompiling | §4.2, §9 | Rules are data, not code |
| **C6** | 50 concurrent client jobs on commodity office hardware | §11.1 | Real concurrency, modest memory footprint |
| **C7** | Multi-user over office LAN, zero per-workstation install | Staff share machines during season | Server process + browser, not a desktop app |
| **C8** | Small team, hiring in a tier-2 Indian city | Practical | Mainstream languages with a local talent pool |

### 5.1 Deployment model

Single-binary application installed on one machine in the firm's office. Other staff reach it over the office LAN via browser. No cloud, no multi-tenant, no client data leaving the premises.

```
┌──────────────────────────────────────────────────────────┐
│  FIRM PREMISES — no outbound client-data path            │
│                                                           │
│  ┌────────────┐   ┌──────────────┐   ┌────────────────┐  │
│  │ Ingestion  │──▶│ Normalised   │──▶│ Rule engine    │  │
│  │ adapters   │   │ fact store   │   │ (deterministic)│  │
│  └─────▲──────┘   │ encrypted    │   └───────┬────────┘  │
│        │          └──────▲───────┘           │           │
│  Accounting ODBC         │                   ▼           │
│  Excel / CSV      ┌──────┴──────┐   ┌────────────────┐   │
│  Portal JSON/PDF  │ Rule packs  │   │ Findings +     │   │
│  Tax-suite output │ (signed)    │   │ lifecycle      │   │
│                   └──────▲──────┘   └───────┬────────┘   │
│                          │                  ▼            │
│                          │          ┌────────────────┐   │
│                          │          │ Web UI, LAN    │   │
│                          │          └────────────────┘   │
└──────────────────────────┼───────────────────────────────┘
                           │ inbound only, signed
                  ┌────────┴────────┐
                  │ Update service  │  ← vendor infrastructure
                  │ + licence check │
                  └─────────────────┘
```

### 5.2 Stack decisions, with alternatives considered

Each decision below states what was chosen, what else was evaluated, why the alternatives lose **against C1–C8**, and what new information would reverse the decision. A choice with no credible alternative listed is a choice that was not thought about.

---

#### 5.2.1 Core engine language — **Go**

| | |
|---|---|
| **Choice** | Go, compiled to a single statically-linked binary per platform |
| **Confidence** | **High** |

**Why it wins here**

- **C2 — zero runtime.** `go build` produces one file with no external dependencies. Copy it to a Windows machine and it runs. There is no "install the runtime first" step, which is the step that kills offline deployments in offices with no IT staff.
- **C3 — demonstrable network absence.** Go's standard library is explicit about imports. A build that never imports `net/http` or `net` **cannot** open a socket, and this is provable by inspecting the import graph — we can generate that proof as a build artefact and hand it to a security-conscious CA. That is a far stronger claim than "we configured a firewall rule."
- **C6 — concurrency is first-class.** Goroutines and channels make the 50-job worker pool (§11.1) straightforward, with memory per job measured in megabytes rather than hundreds.
- **C8 — hiring.** Go is mainstream in Indian backend teams. The language is deliberately small; a competent backend engineer is productive in roughly a week.
- **Cross-compilation is trivial.** `GOOS=windows GOARCH=amd64` from a Linux CI box. No Windows build machine needed.

**Alternatives evaluated**

| Alternative | Why rejected |
|---|---|
| **.NET 8 (C#)** | Genuinely strong on Windows, and self-contained single-file publish now exists, so C2 is satisfiable. Rejected on three narrower grounds: self-contained output is substantially larger; cross-compiling to Linux for the pilot/CI environment is more friction; and proving network-capability absence (C3) is harder because the BCL is one large surface rather than discrete imports. **This is the closest alternative — revisit if the team is already a .NET team.** |
| **Rust** | Better memory-safety guarantees and comparable binary properties. Rejected on C8: hiring and ramp-up cost in a small tier-2 team is materially higher, and the safety advantage does not buy much for a program whose hard part is business rules, not memory management. Revisit if the team already has Rust depth. |
| **Python** | Rejected on C2 and C6. Packaging for offline Windows (PyInstaller/Nuitka) produces large bundles that routinely trigger antivirus false positives — fatal in a CA office. The GIL constrains C6. Acceptable only for the PDF sidecar (§5.2.5). |
| **Node / TypeScript backend** | Rejected on C2 and C3. `pkg`-style bundling is workable but the dependency surface is enormous, and the security story for an audit-adjacent product that must justify its supply chain is weak. |
| **Java** | `jlink` can produce a runtime image, so C2 is satisfiable. Rejected on footprint and on C8 — the Java talent pool skews enterprise rather than desktop-distribution work. |

**What would reverse this:** the team being an existing .NET or Rust shop. The decision is about team fit more than language merit.

---

#### 5.2.2 Data store — **SQLite + SQLCipher**

| | |
|---|---|
| **Choice** | SQLite with the SQLCipher extension for transparent AES-256 page-level encryption. Embedded PostgreSQL only if a firm exceeds the 50-client envelope |
| **Confidence** | **High** |

**Why it wins here**

- **C2 — zero administration.** A CA office has no DBA and no server. SQLite is a file. Backup is "copy the file." Restore is "copy it back." No service to start, no port to secure, no version upgrade to coordinate.
- **Ubiquity as a risk reduction.** SQLite is, by its maintainers' account and by general acceptance, the most widely deployed database engine in existence — it ships inside every major browser and mobile OS. For an offline product that must simply work on unknown hardware for years, boring and universally-exercised beats clever.
- **Encryption at rest without a key-management service.** SQLCipher is an established open-source extension providing page-level AES-256 with the key derived from a firm passphrase plus machine binding. No cloud KMS, which C1 forbids anyway.
- **C4 — determinism.** Single-writer, serialisable by default. No replication lag, no eventual consistency, no surprises in a reconciliation run.
- **Public-domain licence.** No licensing question to answer when a firm's own auditor asks what is inside the product.

**Alternatives evaluated**

| Alternative | Why rejected |
|---|---|
| **PostgreSQL (server install)** | Rejected on C2. Installing and maintaining a Postgres service in a CA office with no IT staff is a support burden out of proportion to the benefit at this data volume. Retained as the escape hatch for unusually large firms. |
| **DuckDB** | Genuinely attractive for the analytical side — the ledger diff in §7 is a columnar workload and DuckDB would be faster. Rejected as the *primary* store because it is weaker for transactional finding-lifecycle state and has a shorter production track record. **Worth revisiting as a secondary analytical engine alongside SQLite if §7 performance disappoints.** |
| **Embedded key-value (BoltDB, Badger)** | Rejected: we need relational queries across facts, findings and periods. Re-implementing joins is not a good use of the team. |
| **Any cloud database** | Rejected by C1 outright. |

**What would reverse this:** a firm materially above 50 clients and 200k vouchers each, where SQLite write contention becomes measurable. That is a Phase 4 load-test question, not a Phase 1 one.

---

#### 5.2.3 User interface — **React + TypeScript, served by the Go binary**

| | |
|---|---|
| **Choice** | Static React/TS bundle embedded in the Go binary, served over the LAN interface. Staff open a browser |
| **Confidence** | **High** on the architecture, **Medium** on React specifically |

**Why it wins here**

- **C7 — zero per-workstation install.** Every machine in a CA office already has a browser. During season, staff use whichever desk is free. A desktop app would mean installing and updating on every workstation; a browser means one update, server-side.
- **Single artefact.** Go's `embed` directive compiles the frontend into the same binary. One file to ship, one file to sign, no asset-path problems.
- **Data never leaves the premises** even though the UI is a web UI, because the server is in the office.
- **C8 — hiring.** React/TS has the deepest talent pool of any UI technology in India by a wide margin.

**Alternatives evaluated**

| Alternative | Why rejected |
|---|---|
| **Electron / Tauri desktop app** | Rejected on C7 and footprint. Electron adds roughly 100MB+ per install and a second runtime to secure; we would still need a server process for the LAN case, so we would be shipping two things instead of one. Tauri is lighter but still per-workstation. |
| **Native Windows (WPF / WinForms)** | Rejected on C7 — per-workstation install and update — and because it forecloses a Linux deployment later. |
| **Server-rendered HTML + HTMX** | **The serious lightweight alternative.** Less JavaScript, smaller bundle, simpler for a small team, and most of our screens are tables and forms. Rejected for the *season board* and *client file* screens, which need live job progress and fast client-side filtering across hundreds of findings. **If the team is backend-heavy, reconsider: HTMX would get to a working UI faster.** |
| **Terminal UI** | Correct for Phase 1 (CLI only, per §14). Not acceptable for staff users. |

---

#### 5.2.4 Rule definition — **declarative YAML, with a Starlark escape hatch**

| | |
|---|---|
| **Choice** | Rules are data: YAML declarations interpreted by the engine. Rules too complex for declarative form use Starlark, a deterministic, sandboxed, Python-like config language |
| **Confidence** | **High** on "rules are data." **Medium** on YAML+Starlark as the specific shape |

**This is the most consequential decision in the document.** Everything in §4.2 and §9 depends on it.

**Why it wins here**

- **C5 — a CA must be able to read a rule.** The rule-authoring CA (§4.2) is not an engineer. A rule they cannot read is a rule they cannot approve, and an unapproved rule is the failure mode that ends the product.
- **Ship rules without shipping a binary.** A rate change or a new clause becomes a signed rule pack, not a release. This is what makes §9 possible at all.
- **Rules become diffable and signable artefacts.** A changelog that shows what changed in a rule is reviewable; a changelog that says "updated engine" is not.
- **C4 — determinism.** Starlark is deterministic by design: no I/O, no clocks, no randomness, hermetic execution. It is used as the configuration language in large build systems precisely for this property. A rule cannot reach the network even accidentally, which reinforces C3.
- **Testability.** Each rule carries its fixtures (§6.4). Data-defined rules make that mechanical.

**Alternatives evaluated**

| Alternative | Why rejected |
|---|---|
| **Rules as Go code** | Rejected on C5 and C2. Every rule change needs a recompile and a full release; the CA cannot review the rules; the §9 pipeline collapses. |
| **Rego / Open Policy Agent** | A real, mature policy DSL and a legitimate contender. Rejected because Rego's evaluation model is oriented to policy decisions over documents, and our core operation is numeric tie-out across a fact graph with explain-the-arithmetic output. We would spend effort fighting the model. Revisit if rule complexity outgrows YAML. |
| **Lua** | Deterministic enough and embeddable, but weaker sandboxing guarantees than Starlark and less readable to a non-programmer. |
| **Embedded Python** | Rejected on C2 (runtime), C3 (full I/O access by default) and C4. |
| **Spreadsheet-defined rules** | Tempting for CA authorship, but versioning, signing and testing a spreadsheet is unpleasant. Could be a *front-end* that emits YAML, later. |

---

#### 5.2.5 PDF extraction — **layout-preserving, and the weakest link in this stack**

| | |
|---|---|
| **Choice** | Vendored `pdftotext -layout` (Poppler) as the primary extractor, invoked as a subprocess; a Python sidecar using `pdfplumber` only if table reconstruction demands it |
| **Confidence** | **Medium — flag this for a Phase 1 spike** |

**Why layout preservation is non-negotiable**

The reference review surfaced defects that a naive text dump would have reported **backwards**, because a plain `pdftotext` run collapses multi-column statutory tables and silently reorders values into the wrong fields. A reconciliation tool that misreads a column produces confident false findings — the §6.4 failure mode. Column position must be preserved, and extraction must be validated against known-good fixtures.

**Why this is the weak link**

Go's PDF ecosystem is markedly thinner than Python's for *table* extraction specifically. That pulls against C2 if we need a Python sidecar, because Python is exactly the runtime we said we would not ship.

Three options, in preference order:

1. **Vendor the Poppler `pdftotext` binary** and shell out with `-layout`. Small, mature, no Python. Handles most structured statutory forms. *Preferred — try this first.*
2. **Parse the tax suite's JSON output instead of its PDF** wherever available (§8.1). The JSON is structured and authoritative, and sidesteps PDF parsing entirely. *This is the real answer wherever it is possible — prioritise it.*
3. **Python sidecar with `pdfplumber`**, bundled, only for documents neither (1) nor (2) handles. Accept the packaging cost knowingly.

**Phase 1 must establish which of the three is needed and for which document types.** Do not plan the rest of the schedule until that is known.

---

#### 5.2.6 Supporting choices

| Layer | Choice | Rationale | Alternatives rejected | Confidence |
|---|---|---|---|---|
| **Excel / CSV** | `excelize` (Go) | Pure Go, no dependency, handles `.xlsx` read/write. Keeps C2 intact | Python `openpyxl` — drags in the runtime we are avoiding | High |
| **Accounting integration** | ODBC / XML-over-HTTP to the package on the LAN | It is the documented integration path for the dominant Indian accounting package, works offline over LAN, and needs no vendor cooperation | Reading the package's data files directly — brittle, undocumented, breaks on upgrade | High |
| **Job queue** | In-process worker pool, state persisted to the local DB | C2: nothing extra to install, no Redis service to secure or restart. Survives restart because state is in the DB | Redis / RabbitMQ / NATS — each is another service in an office with no IT staff | High |
| **Signing** | Ed25519 (RFC 8032) on rule packs and binaries | Small keys and signatures, fast verification, no certificate-authority dependency — so verification works fully offline, which C1 requires. Widely adopted in modern protocols | RSA — larger, slower, no benefit here. X.509/PKI — needs a chain and often network checks | High |
| **Packaging** | MSI for Windows, `.deb`/`.rpm` for Linux, plus portable zip | Windows is the realistic primary target; the portable zip matters for air-gapped firms that cannot run an installer | Installer-only — blocks the air-gap case | High |
| **Testing** | Golden-file fixtures per rule, plus property tests on money arithmetic | §6.4 is unenforceable without mechanical fixtures. Money arithmetic uses integer paise throughout — **never floating point** (C4) | Unit tests on logic alone — would not have caught the column-inversion class of defect | High |
| **Voice** | Sarvam, on-premise model only | Cloud ASR with client names in the audio violates C1/§4.1 outright | Any cloud ASR | **Medium — confirm on-prem availability and office-hardware performance (open question 6)** |
| **Local LLM** | Small quantised model via `llama.cpp`, **off by default**, non-authoritative | Only for §7 classification suggestions and plain-language restatement. May never create, suppress or alter a finding (C4) | Any cloud LLM — violates C1 | **Low — defer past Phase 6** |

---

### 5.3 What we are deliberately not using, and why

| Not used | Reason |
|---|---|
| **Docker / containers on the firm's machine** | C2. "Install Docker Desktop" is where an offline deployment in a CA office dies |
| **Microservices** | One firm, one machine, 50 jobs. Distributed architecture buys nothing and costs deployment simplicity |
| **Any cloud service in the data path** | C1 is the product's entire differentiator |
| **ORM with lazy loading / magic** | C4. Reconciliation queries should be explicit SQL that an engineer can read and reason about |
| **Floating-point money** | C4. Integer paise everywhere. A ₹0.01 drift in a tool whose promise is "to the rupee" destroys trust permanently |
| **An LLM anywhere in the finding decision path** | §2, C4, and §13 risk 2 |

### 5.4 Why no LLM in the core path

The offline constraint forces deterministic matching, which is what §2 wanted anyway. The constraint helps us.

If a local model is used at all, it may do exactly two things, both non-authoritative:

1. Suggest a classification when a new ledger head appears — **user confirms** (§7)
2. Restate an existing computed finding in plain language — cosmetic only

It may **never** create, suppress, or alter a finding.

---

## 6. M2 — Cross-form tie-out engine

### 6.1 Model

- **Fact**: `(artefact, locator, label, value, period)`
- **Edge**: an assertion that two or more facts must be equal, or must satisfy a stated relation
- **Finding**: a broken edge, with both values, the difference, and both locators

### 6.2 Rule categories

Every category below fired on a real filing pack. **Values are illustrative.**

| # | Category | Illustrative instance |
|---|---|---|
| **R1** | **Same number, many documents** | Depreciation: audit report schedule total = ₹1,12,450; computation "allowed depreciation" = ₹1,15,200; the computation's *own* fixed-asset schedule = ₹1,12,450. Three documents, two values |
| **R2** | **Derived-value recomputation** | MAT book profit. As filed: PAT 2,40,00,000 + current tax 48,00,000 − deferred tax 25,000 = **2,87,75,000**. But income tax sitting inside other expenses must also be added back: + 7,50,000 + 2,500 = **2,95,27,500**. Book profit understated ₹7,52,500; MAT understated ≈ ₹1,25,607 |
| **R3** | **Dependency propagation** | Depreciation corrected 1,15,200 → 1,12,450, so business profit rose ₹2,750 — but the profit-linked deduction claimed on that same business profit **did not move**. The dependent node was left stale. No filing suite caught it |
| **R4** | **Internal logical consistency** | Audit report reports gross profit as `0` while net profit is ₹2,85,00,000. GP ≥ NP is unavoidable for an entity with no cost-of-goods line |
| **R5** | **Cross-form semantic conflict** | The audit limb selected describes the entity's receipts as *professional*, while a deduction claimed elsewhere on the same signed form is available only on *business* profits |
| **R6** | **Sequence / completeness** | Employee-contribution schedule has rows for 11 months and none for the twelfth. Quarterly TDS statements present for Q2–Q4, absent for Q1 |
| **R7** | **Field-domain check** | Foreign tax credit form, column headed "Rate of tax as per DTAA", populated with `16.69` — the domestic MAT effective rate. No treaty carries that rate |
| **R8** | **Books vs third-party data** | Tax-credit-statement receipts ₹4,40,00,000 + foreign receipts ₹92,00,000 = ₹5,32,00,000, against reported turnover ₹6,20,00,000. Gap ₹88,00,000 to be explained |
| **R9** | **Mandatory field present elsewhere** | A party's PAN is blank in one clause while the same party's PAN (`ABCDE1234F`) appears in two other clauses of the same form |
| **R10** | **Version regression** | The same field across three drafts: value A → value B → value A. Two wrong turns, no record of why |

### 6.3 Target rule count

| Milestone | Rules | Note |
|---|---|---|
| Usable alpha | 80–120 | Derivable from a single filing pack we understand well |
| Commercial v1 | 300–400 | Covers the tax audit report's full clause set, the MAT report, FTC form, computation, tax credit statement |
| Mature | 600+ | Plus GST, TDS, payroll (§8) |

**Build these by hand.** The rule graph *is* the business. Unglamorous, compounding, and the only part a competitor cannot replicate in a weekend.

### 6.4 False-positive budget

| Metric | Target | Enforcement |
|---|---|---|
| False positives per 100 findings | **< 2** | Every rule ships with golden-file fixtures. A rule that fires wrongly twice in the field is **disabled**, not tuned |
| Findings on a clean file | < 5 | Noise destroys adoption faster than misses do |

---

## 7. M3 — New-head / completeness detection

> Practitioner, verbatim in substance: *"last year xyz were regular expenses like monthly rent, electricity bill etc. but this year that changed, and similarly for income… a lot of expenses that are new in this year get missed and at the time of FY closure they need to find where is this expense bill or expense entry."*

This is a **completeness** problem, not a consistency problem. No cross-form rule catches it. It needs its own module.

### 7.1 Checks

| Check | Logic |
|---|---|
| **New head** | Present in FY *n*, absent in FY *n−1*. Flag with first-appearance date, total value, transaction count |
| **Vanished head** | Present in FY *n−1*, absent in FY *n*. Rent stopping mid-year is either a real change or a missing entry |
| **Broken recurrence** | Appeared in ≥10 of 12 months last year, now missing specific months |
| **Step change** | Same head, value moved more than a configurable threshold versus prior year |
| **Unmapped head** | New head with no assigned treatment: capital vs revenue, audit-report clause, GST registration status of counterparty, TDS section, statutory-payment category |

### 7.2 Output is a worklist, not a finding

These need human classification, not correction:

```
NEW HEADS — FY 2025-26 vs FY 2024-25                 12 items
  Marketing retainer      ₹36,00,000   first seen 12/05/2025   18 txns
    → Audit report clause?  [ related party ▾ ]
    → Counterparty GST?     [ registered ▾ ]
    → TDS section?          [ 194J ▾ ]
    → Statutory payment?    [ no ▾ ]
    [ confirm ]  [ ask client ]  [ voice note ]

BROKEN RECURRENCE                                      3 items
  Employee state insurance — no entry for month 7; present in other 11
```

### 7.3 Why this leads the demo

It is the only pain the practitioner volunteered **unprompted, as his own**. The cross-form checks solve the *client's* pain. This solves the *firm's*. Build both; demo this one first.

---

## 8. M4 — Statutory reconciliation

| Reconciliation | Sources | Core checks |
|---|---|---|
| **GST outward** | Sales register ↔ outward return ↔ summary return ↔ audit-report turnover | Taxable value, tax by head, invoice count, HSN totals, month by month |
| **GST inward / ITC** | Purchase register ↔ auto-drafted ITC statement ↔ summary return ↔ audit-report ITC clause ↔ expenditure break-up clause | Availed vs available, reversals, ineligible credit, supplier-wise gaps |
| **TDS payable** | Books TDS ledgers ↔ quarterly statements ↔ challans ↔ audit-report TDS clause | Section-wise base and rate, short deduction, late deposit, and whether the "interest payable" declaration matches reality |
| **TDS receivable** | Books receipts ↔ tax credit statement ↔ annual information statement ↔ turnover | The R8 gap pattern |
| **Payroll** | Salary register ↔ provident fund ECR ↔ state insurance challan ↔ salary TDS statement ↔ audit-report contribution clauses | Headcount and gross per month, employee vs employer share split, due date vs actual payment date, month-gap detection |

### 8.1 How data actually arrives

| Source | Access | Reality |
|---|---|---|
| **Accounting package** | ODBC / XML over HTTP on the LAN | Well documented, offline. **Highest-value integration, build first** |
| **GST portal** | Programmatic access generally routed via GSP/ASP licensing **[ASSUMPTION — verify cost and eligibility]** | Fallback: user downloads JSON/Excel and drops it in. Works today, zero dependency |
| **Tax credit statement** | No open API found | Manual download, parse the file |
| **Provident fund / state insurance** | No open API found | ECR and challan files are downloadable; parse them |
| **Tax suite output** | Emits JSON for portal upload, plus printable PDFs | **Parse the JSON — structured and authoritative.** PDF parsing as fallback |

**Design rule: every source must have a manual-file-drop path.** API access is an optimisation, never a dependency. This also keeps the product fully functional air-gapped.

---

## 9. M7 — Rule-pack pipeline

| Element | Detail |
|---|---|
| Scheduler location | **Vendor infrastructure.** Daily |
| Sources polled | Official tax and indirect-tax department notification pages, the gazette, the professional institute's announcements, portal schema/utility version endpoints |
| Detection | Content-hash diff plus classification. **An LLM may be used here** — it only decides what a human reads |
| Authoring | CA on payroll. Writes the YAML rule plus one passing and one failing fixture |
| Packaging | Semver, scoped by assessment year. `2026.10.3` effective AY 2026-27 |
| Signing | Ed25519, verified client-side offline |
| Delivery | HTTPS pull by the updater process, or side-loaded signed file |
| Firm control | Changelog shown; firm chooses when to apply; rollback always available |

**The pipeline must never auto-publish a rule no human approved.** One wrong auto-generated rule, firing across every installed firm at once, ends the product.

---

## 10. Security and data protection

### 10.1 Posture

The entire security argument is one sentence a CA can repeat to a client: **"the client data never leaves your office."** Everything below supports that sentence.

| Control | Implementation |
|---|---|
| Data residency | On firm premises. No cloud storage of client data at all |
| Encryption at rest | AES-256 via SQLCipher; key from firm passphrase + machine binding |
| Encryption in transit | TLS on the LAN interface, including locally |
| Network isolation | Engine binary compiled without network capability; updater is a separate process. Documented and demonstrable |
| Access control | Role-based — Partner / Manager / Staff / Read-only. Staff cannot delete findings or export |
| Audit log | Append-only, hash-chained. Every view, disposition, export, rule-pack change |
| Retention | Configurable per assessment year; default aligned to the statutory working-paper retention period the firm already observes |
| Export control | Watermarked PDF carrying firm name, user, timestamp |
| Voice | On-premise model only. Audio deleted after transcription, never transmitted |
| Backup | Encrypted local backup to a firm-chosen path. No vendor-hosted backup |

### 10.2 DPDP Act 2023

The firm is the data fiduciary for its clients' personal data. As a pure on-premise licensor that never receives that data, the vendor sits largely outside the processing chain — a deliberate architectural choice and a selling point.

**[ASSUMPTION — verify with counsel before any marketing claim.]** The status and content of the DPDP Rules, and the precise fiduciary/processor characterisation for an on-prem licensor, need a legal opinion. **Do not publish a "DPDP compliant" claim until that opinion is in hand.** A wrong compliance claim to a CA audience is fatal.

Features that help the *firm* meet its own obligations: per-client engagement record, locate-and-export all data for one assessee, deletion workflow with audit trail, breach-detection logging.

---

## 11. Concurrency, scale, UI

### 11.1 Fifty concurrent clients

| Dimension | Target |
|---|---|
| Concurrent reconciliation jobs | 50 |
| Typical input per client | Accounting export (10k–200k vouchers), 12 GST returns, 4 TDS statements, 12 payroll files, 6–8 PDFs |
| Full run, one client | < 3 minutes on 8-core / 16 GB |
| Full run, 50 clients | < 30 minutes with a worker pool |
| Incremental re-run after one document changes | < 20 seconds |

Worker pool sized to `cores − 2`. Each client is an isolated unit of work. Rules are pure functions over the fact store, so they parallelise trivially. Jobs cancellable and resumable.

### 11.2 UI — three screens, resist a fourth

| Screen | Content |
|---|---|
| **Season board** | All clients, one row each: status, open findings by severity, oldest unanswered query, days to deadline. What a partner looks at in October |
| **Client file** | Findings grouped by severity. Each expands to both values, difference, locators, source thumbnails. Disposition in one click or one voice note |
| **Worklist** | New heads, vanished heads, broken recurrences — classification tasks, not errors |

Rules:
- Every finding shows **the arithmetic**, always. A CA trusts a number, not a verdict
- One-action dismissal, permanent for that file
- Group by **the document to be fixed**, not by rule category — that is how the work is actually done
- Keyboard-first; staff running 50 files will not reach for a mouse
- Hindi/English label toggle

---

## 12. Unit economics

### 12.1 Annual cost base (small team, tier-2 city)

| Item | Annual |
|---|---|
| 2 engineers (loaded) | ₹30,00,000 |
| 1 CA for rule authoring (§4.2, non-negotiable) | ₹12,00,000 |
| 1 support / onboarding | ₹6,00,000 |
| Update infra, code signing, build, licence server | ₹3,00,000 |
| **Total** | **₹51,00,000** |

Excludes founder time and all sales cost.

### 12.2 Break-even

| Price | Per firm per year | Firms to break even |
|---|---|---|
| **₹500/month** | ₹6,000 | **850** |
| ₹1,000/month | ₹12,000 | 425 |
| ₹1,500/month | ₹18,000 | 284 |
| ₹2,500/month | ₹30,000 | 170 |

### 12.3 The problem with ₹500

1. **Gross margin is genuinely high.** On-premise means no per-customer infrastructure cost; the marginal cost of firm 851 is near zero. That part works.
2. **The fixed cost is the rule base and it never stops.** Tax law changes every budget and every circular. The CA cannot be fired. 850 firms from a standing start with no distribution is a hard number.
3. **₹500 may actively hurt.** A comparable AI audit platform publishes ₹3,000 per entity per year — roughly 25× higher per entity. Reportedly, an established audit e-filing tool is free for up to 10 assessees under a professional-institute initiative. CA software spend is trivial against fee income. Pricing 25× below a funded competitor signals "toy," not "bargain."

**Recommendation:** tier by client count, starting around **₹1,500/month for up to 50 clients**. Revisit only if a pilot proves price is the real objection. It almost certainly is not — see §13 risk 1.

---

## 13. What will actually kill this

| # | Risk | Mitigation |
|---|---|---|
| **1** | **The buyer does not feel the pain.** In the reference engagement the practitioner answered twelve review findings "Not applicable" in a single block — and on five of them he was right. Firms that do not believe they have a review problem will not buy a review tool | Lead with §7, which the practitioner volunteered as *his* pain, and with the **30–35% load reduction he estimated himself**. Never lead with "you make mistakes" |
| **2** | **False positives** | §6.4. Deterministic only. No judgment features, ever |
| **3** | **Distribution.** One practitioner relationship, no channel | Free pilot with Firm A. Then professional-institute branch study circles and regional tax bar associations. Relationship market, not performance marketing |
| **4** | **Seasonality.** Tax audit is roughly six weeks | Extend to the post-filing cycle — automated intimations, information-statement mismatches, notices — which arrive in the opposite season. Same engine. **[Not researched. Do that before committing]** |
| **5** | **Rule-base maintenance debt** | Budget the CA permanently from day one, not as a later hire |
| **6** | **An incumbent closes the gap** by adding cross-form checks | Likely eventually. That is also the exit: the verification layer is worth more attached to their distribution than standing alone |
| **7** | **Sample size of one.** Every requirement traces to one practitioner at one firm | Validate with 3–5 more firms before writing production code |

---

## 14. Build sequence

| Phase | Weeks | Deliverable | Gate |
|---|---|---|---|
| **0. Validate** | 2 | Confirm risk 7 with 3–5 firms. Confirm the GSP/ASP question (§8.1). Obtain the DPDP opinion (§10.2) | ≥3 firms describe the §7 pain unprompted |
| **1. Spike** | 4 | Parse a reference filing pack. 80–120 rules. CLI only, no UI | Reproduces every R1–R10 defect with zero false positives |
| **2. Accounting + new-head** | 6 | ODBC ingestion, §7 module, basic UI | Pilot firm runs it on 5 real files and finds it useful |
| **3. Statutory recon** | 8 | §8 — GST, TDS, payroll | Finds gaps on real files the firm had missed |
| **4. Hardening** | 6 | Encryption, RBAC, audit log, licensing, 50-client concurrency | Security review by someone outside the build team |
| **5. Rule pipeline** | 4 | §9 with CA in the loop | One real rule shipped end to end |
| **6. Voice** | 3 | §3.1 M8, on-premise | Staff use it twice without being asked |

**Do not start Phase 1 until Phase 0 clears.** The single-firm sample is the largest risk in this document.

---

## 15. Open questions

| # | Question | Blocks |
|---|---|---|
| 1 | Do 3–5 other firms confirm the new-head and cross-form pains? | Everything |
| 2 | Exact structure of the tax suites' JSON output — parseable reliably across versions? | §8.1, Phase 1 |
| 3 | GSP/ASP licensing — cost, eligibility, necessity | §8.1 |
| 4 | DPDP characterisation of an on-premise licensor | §10.2, all marketing |
| 5 | Does an offline reconciliation product already exist? Research so far found cloud-first tools only | Positioning |
| 6 | Is the on-premise Sarvam deployment available and performant on office hardware? | §5.2, M8 |
| 7 | Does the post-filing notice cycle have the same gap? | §13 risk 4 |
| 8 | Who makes the buying decision — partner, or the manager who feels the pain? | Pricing, GTM |

---

## 16. One-line summary

> It reads what your existing software produces — your accounting package, your tax suite, the portals — and tells you, to the rupee, every place they disagree, plus every expense or income head that is new this year and not yet classified. It runs entirely on your own machine. No client data ever leaves your office.

---

*Nothing in this document is validated beyond one practitioner interview and one reference filing pack. All figures are illustrative.*
