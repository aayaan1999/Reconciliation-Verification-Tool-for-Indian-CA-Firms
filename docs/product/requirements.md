# Requirements digest (Tool 1, v0.3)

This page is a map of the Tool 1 requirements v0.3 ([requirements-spec-tool1.md](../../requirements-spec-tool1.md), 13 October 2026). It lists each requirement group and ID with a one-line statement and points to the section that holds the full text. It is not a copy: when wording matters, read the source. Where v0.3 differs from the product spec v0.2 ([recon-tool-spec.md](../../recon-tool-spec.md)), v0.3 wins for Tool 1; the differences are listed at the end. Where each requirement is designed is in the technical spec traceability table ([tool1-technical-spec.md §B18](../../tool1-technical-spec.md#b18-traceability)).

## Scope (requirements §1)

Tool 1 is reconciliation only, every check is a rule, and growth (formats, rules, tax years, clients) comes without redesign (§1.1). Out of scope: return preparation, bookkeeping, practice management, audit opinion, tax advice, direct portal connections and AI deciding findings (§1.2).

## Operating environment, ENV-01 to ENV-07 (§2)

| ID | In one line |
|---|---|
| ENV-01 | Installed on one main Windows desktop in the office |
| ENV-02 | Staff use a browser over the office network; nothing installed on their PCs |
| ENV-03 | No internet at any time; screens, fonts and libraries are bundled |
| ENV-04 | All client data stays on the main desktop |
| ENV-05 | Updates and rule packs arrive by USB, signature-checked, with rollback |
| ENV-06 | The main desktop has a fixed network name or IP and stays on in working hours |
| ENV-07 | Daily backup to a second drive, external disk or office NAS |

## Users and roles (§3)

Partner, Manager, Staff and Read-only, with the permissions in §3. The technical spec turns this into a permission matrix (tech spec §B12.1).

## Inputs, IN-01 to IN-26 (§4)

| Group | IDs | In one line |
|---|---|---|
| Input types (§4.1) | IN-01 to IN-06 | JSON prepared by CA staff (primary for income tax); Excel/CSV; Word tables only; Tally XML or Excel exports; text PDFs as a fallback (no scans); portal downloads |
| Multiple files (§4.2) | IN-10 | Any number of files of any type in one run |
| | IN-11 | Several files of one document type are combined by client, tax year, period and GSTIN or branch |
| | IN-12 | A newer version is a new draft, never an overwrite |
| | IN-13 | An identical file uploaded twice is detected by fingerprint and skipped |
| | IN-14 | Every upload shows a validation report |
| Format library (§4.3) | IN-20 | One format profile per market format and version, built from real samples |
| | IN-21 | Profiles are deterministic; no AI or statistical guessing |
| | IN-22 | Each profile is tested against hand-verified figures |
| | IN-23 | Exactly one profile matches a file, or it is rejected with a clear message |
| | IN-24 | A new format or version means a new profile only; rules do not change |
| | IN-25 | A Format Register lists every supported format |
| | IN-26 | Until real samples arrive, a generic synthetic format is used ([docs/fixtures.md](../fixtures.md)) |

"Trained" in §4.3 means configured, not machine-learned.

## Fact vocabulary, FV-01 to FV-04 (§5)

Every figure is a fact with a standard name, an integer-paise value, a period and a locator (FV-01). Names come from a controlled vocabulary ([docs/fact-vocabulary.md](../fact-vocabulary.md), FV-02). Profiles write and rules read only vocabulary names, and both are validated (FV-03). Formats change profiles; law changes rules; neither affects the other (FV-04).

## Rules, RU-01 to RU-53 (§6)

| Group | IDs | In one line |
|---|---|---|
| Storage (§6.1) | RU-01 | Rules are YAML |
| | RU-02 | A strict YAML subset: text quoted, unknown keys rejected, schema-validated |
| | RU-03 | Each rule has plain-English `rule_text_en` and `notes` shown on screen |
| | RU-04 | Complex calculations use sandboxed Starlark referenced from the YAML |
| Content (§6.2) | — | Every rule has ID, version, source, status, title, law reference (mandatory), tax year and dates, inputs, check type, tolerance, severity and who made and approved it |
| Built-in and office rules (§6.3) | RU-10 to RU-13 | A starter library approved by the firm's CA; the CA creates and edits rules; editing a built-in rule saves an office version; packs never overwrite office rules |
| Rule Builder (§6.4) | RU-20 to RU-25 | Form-based rule building; YAML generated; dropdowns list only readable facts; free text not interpreted; Starlark values editable, logic not; test on a past client before activation |
| Law alignment (§6.5) | RU-30 to RU-34 | Every rule cites its law; dated law tables hold changeable values; Income-tax Act 1961 and 2025 side by side; legal correctness is the CA's responsibility |
| Governance (§6.6) | RU-40 to RU-44 | Draft, Test, Partner approves, Active; Disable in one click; every version kept; every change audited; pass and fail fixtures per rule; a wrong rule is disabled, not tuned |
| Execution (§6.7) | RU-50 to RU-53 | Offline, no AI; identical inputs give identical results; a missing figure gives "cannot check", not a mismatch; each run records the rule and law versions used |
| Scale (§6.8) | — | 80 to 120 rules (alpha), 300 to 400 (version 1), 600+ (mature) |
| Wording (§6.9) | — | Findings state only computed differences; never "should", "incorrect" or "must be" |

## Reconciliation functions, RC-01 to RC-06 (§7)

RC-01 cross-document tie-out (R1 to R10 of spec v0.2); RC-02 new heads as a worklist; RC-03 GST; RC-04 TDS; RC-05 payroll; RC-06 draft-vs-draft comparison. All are rule-driven.

## Findings and workflow, FD-01 to FD-08 (§8)

| ID | In one line |
|---|---|
| FD-01 | Stable finding ID (rule, client, period, match key); re-runs update, never duplicate |
| FD-02 | Both values, the difference, both sources and the law reference |
| FD-03 | States: Open, Decided (Fix / Not an issue / Ask client), Awaiting re-check, Resolved, Reopened, Carried forward |
| FD-04 | "Not an issue" stays closed unless the values change |
| FD-05 | A corrected upload re-runs affected rules and resolves or reopens findings |
| FD-06 | Findings are grouped by the document to be fixed |
| FD-07 | Screens: Season board, Client file, Worklist, Rule Builder, Law tables, Format Register, Admin |
| FD-08 | Exports: Excel, watermarked PDF, client query letter |

## Scalability, SC-01 to SC-08 (§9)

New formats as profiles (SC-01); new rules without a new version (SC-02); new tax years as tagged rules and dated law rows (SC-03); 50 clients at once (SC-04); up to 200,000 vouchers per client (SC-05); one client under 3 minutes, 50 under 30, a one-file re-run under 20 seconds (SC-06); 600+ active rules (SC-07); independent modules on one fact store and engine (SC-08).

## Non-functional, NF-01 to NF-09 (§10)

NF-01 integer paise; NF-02 under 2 wrong findings per 100 and under 5 on a clean file; NF-03 no AI, identical output; NF-04 logins, roles, encrypted database, HTTPS on the LAN, no network code in the engine; NF-05 append-only hash-chained audit log; NF-06 Ed25519-signed packs verified offline; NF-07 daily encrypted backup with tested restore; NF-08 arithmetic and sources on every finding, keyboard shortcuts, Hindi/English labels; NF-09 support for the firm's DPDP obligations, with no compliance claim until a legal opinion.

## Stack, architecture, phases, assumptions, questions (§11 to §15)

- Stack (§11) and architecture summary (§12): see [architecture-overview.md](../architecture/architecture-overview.md). NF-04 and §11 name SQLCipher; ADR 0002 replaced it with AES-256-XTS ([ADR-0002](../architecture/decisions/ADR-0002-database-encryption.md)).
- Phase plan (§13): phases 1 to 8, from the CLI skeleton to voice (deferred). The build units in [specs/](../../specs/README.md) follow it.
- Assumptions A1 to A6 (§14): see [constraints-and-assumptions.md](../architecture/constraints-and-assumptions.md).
- Open questions Q1 to Q7 (§15): tax suites and samples, staff documents in scope, Tally version, licence check, Windows test PC, number of users, DPDP opinion. All **Open** unless a spec says otherwise.

## Where v0.3 overrides v0.2 (§16)

| Area | v0.2 | v0.3 (wins for Tool 1) |
|---|---|---|
| Scope | A product with market validation, pricing and a vendor rule service | Tool 1: reconciliation only; commercial sections removed |
| Network | Asymmetric: outbound licence heartbeat, inbound HTTPS updates (v0.2 §4.1, §9) | Fully offline; updates and packs by USB (ENV-03, ENV-05) |
| Inputs | Accounting ODBC first, Excel/CSV, portal files, tax-suite output | Adds JSON prepared by CA staff, Word tables, many files per client; Tally through exports, live pull optional |
| Formats | Adapters per software | A format library of tested, deterministic profiles and a Format Register |
| Rules | YAML written by a vendor CA | YAML kept, strict subset; Rule Builder for the firm's CA; built-in vs office rules; law reference mandatory; law tables; approval workflow |
| Law | Not addressed | Income-tax Act 2025 and its new forms alongside the 1961 Act |
| Screens | Three screens, "resist a fourth" (v0.2 §11.2) | Seven screens (FD-07) |
| Licence | Included | Open question Q4 |
