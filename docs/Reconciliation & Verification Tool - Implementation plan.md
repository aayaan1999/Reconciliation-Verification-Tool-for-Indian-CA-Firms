# Reconciliation & Verification Tool (Tool 1) — Implementation plan

> Phase 1 product. Working guide for building Tool 1 with Claude Code as the development assistant.
> Companion documents: Business Requirements Document (`docs/Tool1-BRD-Phase1.docx`) and Build Plan (`docs/Reconciliation & Verification Tool — Build Plan (Phase 1).docx`).
> Source of truth: `tool1-technical-spec.md` for design and `specs/SPEC-0NN-*.md` for each build unit. Where this file differs from them, they win; fix this file in the same commit.
> Last updated: 13 October 2026 · Owner: Unnati

**How to use this file with Claude:** start every build session with
"Read CLAUDE.md and docs/Reconciliation & Verification Tool - Implementation plan.md. We are on Week N, Day D, SPEC-0NN." Work one spec at a time, run the tests, commit, and ask Claude to update the [Status](#14-status) section at the end of each day in Week 1 and at the end of each sprint after that.

---

## Table of contents

1. Problem statement
2. Phase 1 scope
3. Non-negotiable rules
4. Technology stack
5. Architecture
6. Repository structure
7. Data model
8. Core flows and logic
9. Rule, profile and report formats
10. Configuration
11. Deployment option A: installer on the office desktop
12. Deployment option B: portable zip and command line
13. Sprint task list
14. Status
15. Testing
16. Later phases (not to be built now)
17. References

---

## 1. Problem statement

A CA firm prepares a client's annual filing pack across four or five systems that do not talk to each other: the accounting package (Tally), the tax suite, the GST portal, TRACES, and EPFO/ESIC. The same figure appears in several documents, and nobody checks that it is the same figure everywhere. Three problems follow:

- **Mismatches go unnoticed.** A reference review of one company's filing pack found about 20 rupee-level mismatches, for example a depreciation figure that differs between the tax audit report and the computation.
- **New heads are missed.** Expense and income heads that appear for the first time have no prior-year counterpart, so they surface late, at year-end closure, if at all.
- **Reconciliation is manual.** GST, TDS, payroll and the books are matched by hand. The pilot firm's partner estimates a 30–35% workload reduction if this is automated to the rupee.

Tool 1 reads the files those systems already produce, turns every figure into a standard fact, runs **rules** over the facts, and reports every place where figures disagree, to the rupee. It states only what it can compute: both values, the difference and where each value came from. It never says what a figure "should" be, never prepares or files a return, and never lets an AI create, hide or change a finding. It runs on one office desktop with **no internet**, and staff use it from a browser over the office network.

The Phase 1 product is piloted at one CA firm. If it meets its success criteria, the same design scales to more rules, more formats and more firms without a redesign.

---

## 2. Phase 1 scope

### In scope

IDs are from the BRD. The Ref column points into `requirements-spec-tool1.md`.

| ID | Requirement | Ref | Built in |
| --- | --- | --- | --- |
| FR-01 | Staff use the tool from a browser on their own PC over the office network | ENV-02 | Sprint 4 |
| FR-02 | Users log in with one of four roles: Partner, Manager, Staff, Read-only | §3 | Sprints 4, 7 |
| FR-03 | Clients and engagements by tax year; a Season board shows every client | FD-07 | Sprint 4 |
| FR-04 | Accept JSON, Excel/CSV, Word tables, Tally XML/Excel, text-based PDF and portal downloads | IN-01..06 | JSON: Week 1; rest: Sprints 3, 6 |
| FR-05 | Any number of files per engagement; combine multi-part files by client, tax year, period, GSTIN/branch | IN-10, IN-11 | Sprint 4 |
| FR-06 | A newer version of a document becomes a new draft; an identical file uploaded twice is skipped | IN-12, IN-13 | Sprint 4 |
| FR-07 | Validation report per upload: format recognised, figures read, anything unreadable | IN-14 | Week 1 (CLI) |
| FR-08 | Detect each file's format; reject a file that matches no format, never guess | IN-23 | Week 1 |
| FR-09 | Each format profile built from samples and tested against hand-verified figures | IN-20..22 | Week 1 |
| FR-10 | Format Register of every supported format and version | IN-25 | Sprint 5 |
| FR-11 | Every figure stored as a standard fact: name, value in paise, period, file and location | FV-01..03 | Week 1 |
| FR-12 | Same figure checked across documents; derived figures recomputed; stale dependants flagged (R1–R10) | RC-01 | R1: Week 1; R1–R10: Sprint 2 |
| FR-13 | Year-on-year ledger comparison into a separate Worklist | RC-02 | Sprint 3 |
| FR-14 | GST reconciliation (books, GSTR-1/1A, 3B, 2B, IMS) | RC-03 | Sprint 6 |
| FR-15 | TDS reconciliation (books, challans, quarterly statements, TRACES) | RC-04 | Sprint 6 |
| FR-16 | Payroll reconciliation (salary, EPF ECR, ESIC, salary TDS) | RC-05 | Sprint 6 |
| FR-17 | Draft-versus-draft comparison, including dependants that did not move | RC-06 | Sprint 2 |
| FR-18 | Findings show both values, the difference, both sources and the law reference; never "should" | FD-02, §6.9 | Week 1 |
| FR-19 | "Cannot check — figure not found in <document>" when an input is missing | RU-52 | Week 1 |
| FR-20 | Stable finding IDs, so a re-run updates a finding instead of duplicating it | FD-01 | Sprint 4 |
| FR-21 | Staff decide each finding (Fix / Not an issue / Ask client) | FD-03, FD-04 | Sprint 4 |
| FR-22 | Re-run on a corrected upload; resolve or reopen findings automatically | FD-05 | Sprint 4 |
| FR-23 | Findings grouped by the document that needs fixing | FD-06 | Week 1 (report); Sprint 4 (UI) |
| FR-24 | Starter rule library with law references and pass/fail test files per rule; CA approves before go-live | RU-10, RU-43 | Weeks 1–2 |
| FR-25 | CA creates and edits rules on a form; the tool writes the YAML | RU-20..23 | Sprint 5 |
| FR-26 | CA tests a draft rule on a past client before activation | RU-25 | Sprint 5 |
| FR-27 | Rule lifecycle Draft → Test → Partner approves → Active; one-click disable; rollback | RU-40, RU-41 | Sprint 5 |
| FR-28 | Edit to a built-in rule saved as an office version; "Reset to standard rule" | RU-12 | Sprint 5 |
| FR-29 | Law tables with valid-from/valid-to dates and law references | RU-31..33 | Sprint 2 (loader); Sprint 5 (screen) |
| FR-30 | Exports: Excel, watermarked PDF, client query letter | FD-08 | Sprint 7 |
| FR-31 | Signed rule packs and updates from USB, with rollback; never overwrite an office rule | ENV-05, RU-13 | Sprint 8 |
| FR-32 | Daily encrypted backup with a tested restore | ENV-07, NF-07 | Sprint 7 |
| FR-33 | Export or delete one client's data on request; retention periods | NF-09 | Sprint 7 |
| FR-34 | Keyboard shortcuts; labels in Hindi and English | NF-08 | Sprint 4 |

### Week 1 scope (strict)

Reconciliation only, from the command line: ingest the filing-pack files (JSON), read them through format profiles into facts, run rules written by hand in YAML, and print findings in a text and a JSON report. **No** web UI, login, voice, AI model, Rule Builder, exports, rule packs, installer, encryption, or GST/TDS/payroll modules in Week 1.

### Out of scope for Phase 1

Return preparation and e-filing, bookkeeping, practice management (tasks, planners, follow-ups), audit opinion and working papers, tax advice or any "this should be X" statement, direct connections to the GST, TRACES or income-tax portals, any AI that decides, hides or changes a finding, OCR of scanned PDFs or photos, and voice (deferred).

---

## 3. Non-negotiable rules

These override convenience. Claude must refuse to write code that breaks them. They come from `CLAUDE.md`.

1. **Assert only what can be computed.** A finding shows both values, the difference and both source locators. Output never says "should", never states a correct value, and never makes an applicability or characterisation judgment. (FR-18)
2. **No LLM in the finding path.** No AI model creates, suppresses or alters a finding. Nothing in `cmd/recon-engine` may import an ML/LLM client. (NFR-06)
3. **Money is integer paise everywhere.** `money.Paise` is `int64`. No `float32`/`float64` in money arithmetic; the archtest float guard enforces this. Rates are integer basis points. (NFR-04)
4. **Determinism.** Same files, same rules, same law tables → byte-identical output. No dependence on map-iteration order; no clock and no randomness in rules. (NFR-05)
5. **The engine has no network code.** `recon-engine` must never import `net`, `net/http`, `crypto/tls` or anything that pulls them in. Every build writes `build/engine-deps-<goos>-<goarch>.txt` as proof. (NFR-03)
6. **Rules are data, not code.** Rules are strict YAML, with Starlark only as a sandboxed escape hatch. A CA must be able to read them. Every rule ships with at least one passing and one failing fixture. A rule that fires wrongly in the field is disabled, not tuned. (FR-24)
7. **Every data source has a manual file-drop path.** API or live pulls (such as Tally) are optional extras, never dependencies. (NFR-14)
8. **Zero install.** One installer on one office desktop; no Docker, no database server, no Redis, no runtime prerequisites. The job queue runs in-process with its state in the database. (NFR-13)
9. **Never use real client data while building.** All figures, PANs and names in this repo are made up. Synthetic packs live in `testdata/`; anonymised samples stay outside the repository.

---

## 4. Technology stack

| Layer | Choice | Notes |
| --- | --- | --- |
| Language | Go (module `go 1.27.0`) | One language for engine, server and tools; builds Windows `.exe` files with nothing else to install |
| Engine | `recon-engine` | No network code; runs as the Week 1 command line and later as the server's child process |
| Server | `recon-server` | HTTP API on the office LAN; embeds the UI; supervises the engine (Sprint 4) |
| Database | SQLite, explicit SQL, no ORM | Plain SQLite until Sprint 7, then SQLCipher (AES-256) behind the same `store.Open`. Driver chosen in SPEC-010 and must pass the archtest |
| Rules and content | Strict YAML via `go.yaml.in/yaml/v3` in `internal/strictyaml` | Unknown keys, unquoted text, anchors, aliases, merge keys, tags and multi-document files are refused |
| Complex rules | Starlark (`go.starlark.net`) in `internal/starhost` | Sandboxed: step limit, no floats, no I/O, no time (Sprint 2) |
| JSON input | Go standard library, `json.Decoder.UseNumber()` | Numbers are never parsed as floats |
| Excel / CSV input | `excelize` | Columns found by header text, not letter (Sprint 3); must pass the archtest |
| Word input | `.docx` table reader | Library chosen in SPEC-019 after the network-code check; tables only, never paragraph text |
| Tally input | XML/Excel exports | Live pull only via the optional `recon-collector` (later) |
| PDF input | Poppler `pdftotext -layout`, bundled in `third_party/poppler/` | Fallback only; region-based profiles with declared column ranges (Sprint 6) |
| UI | React + TypeScript, embedded with `go:embed` | Three screens only: Season board, Client file, Worklist (Sprint 4) |
| Signing | Ed25519 (`crypto/ed25519`) | Rule packs and updates verified offline (Sprint 8) |
| CI | GitHub Actions (`.github/workflows/ci.yml`) | gofmt, vet (host and Windows), tests with `-count=1`, Windows build, on every push |
| Packaging | Windows installer + portable zip | Sprint 7 (SPEC-040) |
| Development | Claude Code + agents `test-engineer` and `code-reviewer` (`.claude/agents/`) | One developer, 8–9 hours a day |

**Not used, on purpose:** cloud hosting, Docker, PostgreSQL, Redis, any hosted or local AI model in the finding path, OCR.

---

## 5. Architecture

```
┌──────────────────────────── MAIN OFFICE DESKTOP (Windows) ─────────────────────────────┐
│                                                                                         │
│   Staff browsers ──HTTPS (office LAN)──►  recon-server  (Sprint 4 onward)               │
│                                            • HTTP API + embedded React UI               │
│                                            • login, roles, uploads → vault             │
│                                            • job queue, state in recon.db              │
│                                            • supervises recon-engine                   │
│                                                 │ stdin/stdout (JSON lines)             │
│                                                 ▼                                       │
│   Week 1: developer ── command line ──►  recon-engine   (NO NETWORK)                    │
│                                            detect → extract → rules → findings          │
│                                                 │                 │                     │
│                                                 ▼                 ▼                     │
│                                          recon.db (SQLite;   pdftotext.exe              │
│                                          SQLCipher from      (bundled, Sprint 6)        │
│                                          Sprint 7)                                      │
│                                                                                         │
│   recon-updater   — applies a signed update from USB, keeps the previous for rollback   │
│   recon-collector — optional, later: Tally pull on the LAN, drops a file in the inbox   │
└─────────────────────────────────────────────────────────────────────────────────────────┘
        ▲ USB only: signed content packs and signed binary updates. No internet, ever.
```

### The four data contracts

Everything hangs off four versioned, data-defined contracts. Code sits between them and never hard-codes what they contain.

| Contract | What it is | Where |
| --- | --- | --- |
| Fact vocabulary | The controlled list of figure names and record types | `content/vocabulary/` |
| Format profile | Where each figure sits in one market format and version | `content/profiles/<source>/<document>/<version>/` |
| Rule | A check over named facts, by tax year | `content/rules/<tax-year>/<module>/<RULE-ID>/` |
| Law table | Dated values (rates, thresholds, due dates) with law references | `content/lawtables/` |

**Decoupling rule:** a profile never names a rule, and a rule never names a file, format or profile. The vocabulary is the only thing both sides share. When a format changes, only the profile changes; when the law changes, only rules and law tables change.

### Processes

| Process | Purpose | Network | From |
| --- | --- | --- | --- |
| `recon-engine` | Detection, extraction, fact store writes, rule evaluation, findings | **None** | Week 1 (CLI) |
| `recon-dev` | Developer tools: profile test, rule lint, vocabulary doc, synthetic data, pack signing. Never shipped to firms | Not shipped | Week 1 |
| `recon-server` | UI, API, auth, uploads, job queue, engine supervision, packs, exports, backups | Office LAN only | Sprint 4 |
| `recon-updater` | Verifies and swaps binaries from USB; keeps N−1 for rollback | None | Sprint 8 |
| `recon-collector` | Optional live Tally pull; output is a file in the inbox | LAN to Tally only | Later |

### Why the engine is a separate program

Keeping all reconciliation in one program with no network packages lets every build prove, from its dependency list, that client data cannot leave the machine through the engine. It also lets Week 1 run the whole reconciliation from the command line before any server or UI exists, and the server later reuses the same program unchanged.

---

## 6. Repository structure

Folders marked *(Week 1)* get real code this week; the rest hold a `doc.go` stating their job until their spec is built.

```
/
├── CLAUDE.md                       # project rules and commands; Claude Code reads it every session
├── tool1-technical-spec.md         # design source of truth
├── requirements-spec-tool1.md      # requirements v0.3
├── recon-tool-spec.md              # product and engineering spec v0.2
├── go.mod / go.sum
├── .github/workflows/ci.yml        # format, vet, tests, Windows build on every push
├── .claude/agents/
│   ├── test-engineer.md            # testing agent
│   └── code-reviewer.md            # peer-review agent (read-only)
├── cmd/
│   ├── recon-engine/               # (Week 1) CLI: recon-engine run ...
│   ├── recon-dev/                  # (Week 1) vocab-doc, profile-test, rule-lint
│   ├── recon-server/               # Sprint 4
│   ├── recon-updater/              # Sprint 8
│   └── recon-collector/            # later, optional
├── internal/
│   ├── money/                      # Paise, Parse, rounding modes, Indian formatting
│   ├── period/                     # FY, quarter, month; FY arithmetic
│   ├── domain/                     # Fact, Locator, Evaluation, Finding, RunSnapshot, canonical JSON
│   ├── strictyaml/                 # strict YAML decoding for all content
│   ├── vocab/                      # vocabulary loader and validator
│   ├── ingest/                     # (Week 1) Adapter, Doc, registry, ValidationReport
│   │   └── adapters/json/          # (Week 1) xlsx/ csv/ docx/ tallyxml/ pdftext/ fixedwidth/ later
│   ├── profile/                    # (Week 1) profile schema, loader, detection, executor
│   ├── rules/                      # (Week 1) rule schema, strict loader, linter
│   ├── engine/                     # (Week 1) planner, dependency index, evaluator
│   │   └── checks/                 # (Week 1) equal; others from Sprint 2
│   ├── findings/                   # (Week 1) wording templates and report; lifecycle in Sprint 4
│   ├── store/                      # (Week 1) SQLite, migrations, writer goroutine
│   ├── lawtable/ starhost/         # Sprint 2
│   ├── modules/                    # tieout/ drafts/ newheads/ gst/ tds/ payroll/
│   ├── vault/ jobs/ ipc/           # Sprint 4
│   ├── authz/ api/ export/ audit/  # server side only, Sprints 4 and 7
│   ├── pack/                       # Sprint 8
│   └── archtest/                   # architecture guard: no net in engine, import rules, no clock/rand/float
├── content/
│   ├── vocabulary/facts.yaml, records.yaml
│   ├── profiles/generic/...        # (Week 1) synthetic tax-suite profiles
│   ├── rules/2025-26/tieout/...    # (Week 1) hand-written rules + fixtures
│   └── lawtables/                  # Sprint 2
├── testdata/
│   ├── packs/ref-01/, clean-01/    # (Week 1) synthetic filing packs
│   └── golden/                     # (Week 1) expected report output
├── third_party/poppler/            # vendored pdftotext.exe + licence
├── web/                            # React + TypeScript UI (Sprint 4)
├── docs/                           # BRD, build plan, this file, File Guide, fact-vocabulary.md, adr/
└── specs/                          # SPEC-001 to SPEC-044 + README.md (index and status)
```

---

## 7. Data model

SQLite with explicit SQL. Migrations are numbered `.sql` files in `internal/store/migrations`, embedded with `go:embed` and applied in order inside a transaction. On open: `PRAGMA foreign_keys = ON; journal_mode = WAL; synchronous = NORMAL; busy_timeout = 5000`. Every money column is `INTEGER` paise and named `*_paise`.

The Week 1 tables below follow tech spec §B1. They are a guide; the migration written in SPEC-010 is the authority.

```sql
CREATE TABLE schema_migrations (
    version         INTEGER PRIMARY KEY,
    applied_at      TEXT NOT NULL
);

CREATE TABLE clients (
    id              INTEGER PRIMARY KEY,
    name            TEXT NOT NULL,
    pan_hash        TEXT,                          -- PAN stored only as a hash ...
    pan_last4       TEXT,                          -- ... and the last 4 characters for display
    status          TEXT NOT NULL
);

CREATE TABLE engagements (
    id              INTEGER PRIMARY KEY,
    client_id       INTEGER NOT NULL REFERENCES clients(id),
    tax_year        TEXT NOT NULL,                 -- "2025-26"
    regime          TEXT NOT NULL,                 -- 'ITA1961' | 'ITA2025'
    status          TEXT NOT NULL
);

CREATE TABLE files (
    sha256          TEXT PRIMARY KEY,              -- identical bytes are stored once
    size            INTEGER NOT NULL,
    vault_path      TEXT,                          -- vault arrives in Sprint 4; NULL for CLI runs
    mime            TEXT,
    first_uploaded_at TEXT NOT NULL
);

CREATE TABLE documents (                          -- the "slot" a file fills
    id              INTEGER PRIMARY KEY,
    engagement_id   INTEGER NOT NULL REFERENCES engagements(id),
    document_type   TEXT NOT NULL,                 -- vocabulary document type, e.g. 'computation'
    period          TEXT NOT NULL,
    scope           TEXT NOT NULL DEFAULT '',      -- GSTIN | branch | Tally company
    part            TEXT NOT NULL DEFAULT ''
);

CREATE TABLE document_versions (                  -- one row per draft
    id              INTEGER PRIMARY KEY,
    document_id     INTEGER NOT NULL REFERENCES documents(id),
    draft_no        INTEGER NOT NULL,
    file_sha256     TEXT NOT NULL REFERENCES files(sha256),
    profile_id      TEXT NOT NULL,
    profile_version TEXT NOT NULL,
    uploaded_by     TEXT,                          -- users arrive in Sprint 4
    uploaded_at     TEXT NOT NULL,
    extraction_status TEXT NOT NULL,
    validation_report_json TEXT
);
CREATE UNIQUE INDEX document_versions_draft ON document_versions(document_id, draft_no);

CREATE TABLE facts (
    id              INTEGER PRIMARY KEY,
    document_version_id INTEGER NOT NULL REFERENCES document_versions(id),
    engagement_id   INTEGER NOT NULL REFERENCES engagements(id),
    name            TEXT NOT NULL,                 -- vocabulary name, e.g. 'tar.depreciation_schedule.total'
    period          TEXT NOT NULL,                 -- 'FY2025-26' | 'FY2025-26:Q1' | '2025-04'
    scope           TEXT NOT NULL DEFAULT '',
    kind            TEXT NOT NULL,                 -- money | int | text | date | bool | rate
    money_paise     INTEGER,
    int_value       INTEGER,
    text_value      TEXT,
    date_value      TEXT,
    rate_bp         INTEGER,
    locator_kind    TEXT NOT NULL,                 -- 'json_path' in Week 1
    locator_path    TEXT NOT NULL                  -- '$.business_income.allowed_depreciation'
);
CREATE INDEX facts_lookup ON facts(engagement_id, name, period, scope);

CREATE TABLE runs (
    id              INTEGER PRIMARY KEY,
    job_id          INTEGER,                       -- jobs table arrives in Sprint 4
    engagement_id   INTEGER NOT NULL REFERENCES engagements(id),
    engine_version  TEXT NOT NULL,
    vocab_version   INTEGER NOT NULL,
    base_run_id     INTEGER,                       -- incremental runs, Sprint 2
    started_at      TEXT NOT NULL,                 -- run metadata only; never in finding output
    finished_at     TEXT
);

CREATE TABLE run_documents (                      -- which drafts a run used (RU-53)
    run_id          INTEGER NOT NULL REFERENCES runs(id),
    document_version_id INTEGER NOT NULL REFERENCES document_versions(id),
    PRIMARY KEY (run_id, document_version_id)
);

CREATE TABLE run_rules (                          -- which rule versions a run used (RU-53)
    run_id          INTEGER NOT NULL REFERENCES runs(id),
    rule_id         TEXT NOT NULL,
    rule_version    TEXT NOT NULL,
    PRIMARY KEY (run_id, rule_id)
);

CREATE TABLE evaluations (
    run_id          INTEGER NOT NULL REFERENCES runs(id),
    rule_id         TEXT NOT NULL,
    rule_version    TEXT NOT NULL,
    subject_key     TEXT NOT NULL,                 -- 'period|scope|match_key'
    status          TEXT NOT NULL,                 -- pass | fail | cannot_check
    values_json     TEXT NOT NULL,                 -- every side, with locator, in rule order
    difference_paise INTEGER,
    missing_json    TEXT                           -- missing fact names when cannot_check
);
CREATE INDEX evaluations_rule ON evaluations(run_id, rule_id);
```

### Tables added later

| Area | Tables | Sprint |
| --- | --- | --- |
| Records (vouchers, invoices, challans, ECR rows) | `records` | 3 |
| Users and sessions | `users`, `sessions` | 4 |
| Jobs and findings lifecycle | `jobs`, `findings`, `finding_events`, `worklist_items` | 3–4 |
| Content governance | `vocab_versions`, `vocab_entries`, `profiles`, `rule_versions`, `rule_state`, `law_tables`, `law_rows`, `run_law_rows` | 2, 5 |
| Packs | `packs` | 8 |
| Audit | `audit_log` (append-only, hash-chained; update/delete triggers raise errors) | 7 |

**What is kept:** all client data stays in `recon.db` (and from Sprint 4 the vault) on the office desktop and its backup drive. A client's data is exported or deleted only on request (SPEC-039), leaving a tombstone audit entry with IDs but no personal data.

---

## 8. Core flows and logic

### 8.1 Week 1 command-line run

```
recon-engine run --input <dir> --rules <dir> --tax-year "2025-26" --db <path> --out <file> [--format text|json]
```

Pipeline (tech spec §A7, steps 1–7 and 10):

1. **Detect.** For each file in `--input`, run every installed profile's `detect` block. Exactly one profile must match.
2. **Extract.** The adapter opens the file; the profile executor writes facts with locators, and a validation report per file.
3. **Resolve.** Build the engagement's current view: the latest draft per document slot.
4. **Snapshot.** Create a run and record the engine version, vocabulary version, document versions and rule versions used.
5. **Plan.** Load rules for the tax year; build the dependency index (rule → facts it reads).
6. **Evaluate.** Run rules in parallel; sort results by `(rule_id, subject_key)` before writing.
7. **Render.** Print validation reports, then findings grouped by review document, then a "Cannot check" section.

| Exit code | Meaning |
| --- | --- |
| `0` | No findings |
| `1` | Findings present |
| `2` | Input or rule error (for example `PROFILE_NO_MATCH`) |
| `3` | Internal error |

### 8.2 Ingest a file

1. Pick the adapter by file content (magic bytes and a cheap `Sniff`), not by extension. An unknown type returns `UNSUPPORTED_FILE_TYPE`.
2. JSON adapter: decode with `UseNumber()`, so `112450.50` stays exact text. Files over the size limit are rejected with a clear error. Malformed JSON returns `PARSE_ERROR` with line and offset.
3. Every `Cell` carries its full concrete path (for example `$.rows[3].amount`), which becomes the fact's locator.
4. The profile executor converts values with `money.Parse` (string-based; Indian grouping `1,12,450` accepted; more than 2 decimals refused unless the profile names a rounding mode).
5. A missing `required` figure is a validation-report error and **no fact**, never a silent zero.
6. Detection: zero matching profiles → `PROFILE_NO_MATCH` naming the file; two or more → `PROFILE_AMBIGUOUS` naming both. Never guess.

Validation report shape (IN-14):

```
File: comp_20261008.json
Recognised as: Generic tax suite — Computation (profile generic.taxsuite.computation v1)
Tax year: 2025-26   Client match: PAN ****234F
Figures extracted: 142 facts, 0 records
Not read: 3 fields — $.annexure.notes (text, not in vocabulary), …
Warnings: none
```

### 8.3 Load rules

1. Read every `rule.yaml` under `--rules` through `internal/strictyaml`. Every problem in a file is reported at once, each with file and line.
2. Validate: at least one `law` entry with `act` and `section`; every input names a vocabulary fact; `check.type` is registered and its params pass `ValidateParams`; `review_document` is a vocabulary document type; `title` passes the banned-word test.
3. Keep only rules whose `applies.tax_years` include the run's tax year.
4. `recon-dev rule-lint` fails any rule without at least one `pass` and one `fail` fixture.

### 8.4 Evaluate a rule

1. For each rule, expand subjects: one evaluation per period/scope present (`scope: each | sum`).
2. Read inputs only through `EvalCtx` (`Fact`, `Facts`; `Records`, `FactHistory` and `Law` arrive in later sprints). `EvalCtx` has no clock, no randomness, no file and no DB access.
3. If a required input is absent → `CannotCheck` with `MissingFacts`. Never `Fail`.
4. `equal`: fail when `|a − b| > tolerance`. Tolerance is `{ paise: N }` or `{ bp_of_larger: N }`, integer arithmetic only.
5. The evaluation records every side (input, label, value, locator) in rule order and the computed difference. It never records an "expected" value.

### 8.5 Render findings

- One fixed template per check type in `internal/findings/templates`; `equal` first.
- Text report: grouped by `review_document`, then rule ID; Indian digit grouping via `money.FormatIndian`.
- "Cannot check" section: `Cannot check — figure not found in <document name>`.
- Canonical JSON report with the same content, byte-identical across runs; used by golden tests.
- Banned words, checked by a test over every template and rule title: `should`, `incorrect`, `must be`, `wrong`, `correct value`, `ought`, `required to`.

### 8.6 Findings lifecycle (Sprint 4)

A finding's identity is `stable_id = base32(sha256(rule_id | engagement_id | period | scope | match_key))[:16]`. The rule version and run are excluded, so a re-run updates the same finding. `value_fingerprint` hashes the values only, so a "Not an issue" decision stays closed until a value changes.

| Existing state | Latest evaluation | New state |
| --- | --- | --- |
| none | Fail | `open` |
| none | Pass / CannotCheck | no finding |
| `open`, `decided_fix`, `decided_ask_client`, `awaiting_recheck` | Pass | `resolved` |
| `decided_not_issue` | Fail, same fingerprint | unchanged |
| `decided_not_issue` | Fail, fingerprint changed | `reopened` |
| `resolved` | Fail | `reopened` |
| any | CannotCheck | unchanged, flagged "not re-checked in run N" |

The states and transitions, including the proposed `withdrawn` state, need CA sign-off before SPEC-025.

### 8.7 New-head detection (Sprint 3)

Compares the Tally trial balance and monthly ledger summary of FY *n* with FY *n−1* and lists new, vanished, broken-recurrence (in ≥ 10 of 12 months last year), step-change and unmapped heads. Output goes to the **Worklist**, never to findings. Staff classifications are stored as facts with `source = user`, so later rules can read them.

### 8.8 Server ↔ engine (Sprint 4)

The server starts one long-lived `recon-engine` and restarts it if it exits. The DB key goes to the engine as the first line on stdin. After that they exchange small newline-delimited JSON control messages; bulk data goes through the database.

```jsonc
{"type":"job.ready","job_id":4182}                                              // server → engine
{"type":"job.progress","job_id":4182,"stage":"rules","done":312,"total":604}    // engine → server
{"type":"job.done","job_id":4182,"run_id":901,"findings_open":7,"cannot_check":2}
{"type":"job.failed","job_id":4182,"error_code":"PROFILE_NO_MATCH","detail":"..."}
```

### 8.9 Error handling

| Failure | Behaviour |
| --- | --- |
| Unknown file type | `UNSUPPORTED_FILE_TYPE`; file listed in the report; exit 2 |
| File matches no profile / two profiles | `PROFILE_NO_MATCH` / `PROFILE_AMBIGUOUS`; never guessed; exit 2 |
| Malformed file | `PARSE_ERROR` with line and offset |
| Required figure missing in a file | Validation-report error (`REQUIRED_FIELD_MISSING`); no fact written |
| Same figure with two values in one slot | `CONFLICTING_VALUES` in the validation report |
| Rule input missing | `CannotCheck`, never a finding, never counted as a false alarm |
| Bad rule file | Rejected with file and line; the run does not start; exit 2 |
| Photo or scanned PDF (Sprint 6) | Refused (`IMAGE_NOT_SUPPORTED`); no OCR |
| Internal error | Exit 3; error logged without document contents |

---

## 9. Rule, profile and report formats

Tool 1 has no AI prompts. Its behaviour is set by these data files, which a CA must be able to read. Every text value is in double quotes; numbers are plain digits.

### 9.1 Rule file (`content/rules/2025-26/tieout/RECON-DEP-002/rule.yaml`)

```yaml
schema: "recon.rule/1"
id: "RECON-DEP-002"
version: "1"
source: "builtin"
module: "tieout"
category: "R1"
title: "Depreciation mismatch"
rule_text_en: "Depreciation total in the tax audit report must equal 'Allowed depreciation' in the computation."
notes: "Compare also RECON-DEP-001 (computation's own fixed-asset schedule)."

law:
  - act: "Income-tax Act, 1961"
    section: "32"
    reference: ""

applies:
  tax_years: [ "2025-26" ]
  valid_from: "2025-04-01"
  valid_to: ""

inputs:
  a: { fact: "tar.depreciation_schedule.total",   label: "Tax audit report, depreciation schedule total" }
  b: { fact: "computation.allowed_depreciation",  label: "Computation, \"Allowed depreciation\"" }

check:
  type: "equal"
  tolerance: { paise: 100 }

severity: "high"
review_document: "computation"
```

`rule_text_en` describes the rule on the rule screen and is not part of finding text, so its "must equal" wording is allowed. Lifecycle status (draft/testing/active/disabled) is **not** in the file; it lives in the database from Sprint 5.

Fixtures sit beside the rule: `fixtures/pass/*.yaml` and `fixtures/fail/*.yaml`, each holding input facts and the expected evaluation(s).

### 9.2 Week 1 hand-written rules

Week 1 has only the `equal` check, so Week 1 rules compare the same figure across two documents (category R1). The vocabulary already holds the three depreciation facts. Every other rule first needs its facts added to `content/vocabulary/facts.yaml` (raise `vocab_version` in both vocabulary files, then run `go run ./cmd/recon-dev vocab-doc`).

| Rule | Compares | Facts in vocabulary? |
| --- | --- | --- |
| RECON-DEP-002 | Tax audit report depreciation total vs computation "Allowed depreciation" | Yes |
| RECON-DEP-001 | Computation's fixed-asset schedule total vs computation "Allowed depreciation" | Yes |
| 3 to 8 further R1 rules | The same figure in the tax audit report and the computation, as chosen with the CA (for example net profit, turnover, disallowances) | Add first |

The CA reads the Week 1 rules at the end of the week. A rule stays out of the starter library until the CA has approved it.

### 9.3 Format profile (`content/profiles/generic/taxsuite.computation/1/profile.yaml`)

```yaml
schema: "recon.profile/1"
profile_id: "generic.taxsuite.computation"
profile_version: "1"
source_system: "Generic tax suite (synthetic, see docs/fixtures.md)"
document_type: "computation"
file_type: "json"
regime: "ITA1961"

detect:                       # all must hold; exactly one profile may match a file
  - json_equals: { path: "$.meta.form", value: "COMPUTATION" }
  - json_equals: { path: "$.meta.schema_version", value: "1.0" }

context:
  tax_year: { json_path: "$.meta.tax_year" }
  pan:      { json_path: "$.assessee.pan" }
  period:   { const: "FY" }

facts:
  - fact: "computation.allowed_depreciation"
    json_path: "$.business_income.allowed_depreciation"
    unit: "rupees"            # parsed as decimal text, never float
    required: true            # missing → validation-report error, not a silent zero
```

Each profile folder has `samples/` and `expected/` (one hand-verified `*.facts.yaml` per sample). `recon-dev profile-test` requires an exact match of names, values, periods and locators. Until real tax-suite samples arrive (Q1), the `generic.*` profiles and `docs/fixtures.md` define the formats; real profiles are added later as new folders, with no code change.

### 9.4 Finding text

```
[RECON-DEP-002]  Depreciation mismatch
  Tax audit report, depreciation schedule total : 1,12,450
  Computation, "Allowed depreciation"           : 1,15,200
  Difference                                    :     2,750
  Source: TAR_v3.pdf p.7 | comp_20261008.json $.business_income.allowed_depreciation
  Rule basis: Income-tax Act, 1961 – Sec 32
```

This block is the golden file `testdata/golden/RECON-DEP-002.txt`; the Week 1 output must match it byte for byte.

### 9.5 Law table (Sprint 2)

```yaml
schema: "recon.lawtable/1"
table: "tds.section_threshold"
key: [ "section" ]
values: { single_paise: "money", aggregate_paise: "money", rate_bp: "rate" }
rows:
  - id: "TDS-THR-0001"
    section: "194X"                 # ILLUSTRATIVE — not a real section or value
    single_paise: 1000000
    aggregate_paise: 5000000
    rate_bp: 1000
    valid_from: "2025-04-01"
    valid_to: ""
    law_ref: "Illustrative reference"
```

Lookup is by the as-of date of the period under check, never the system clock. A law change is a new row with a new `valid_from`.

---

## 10. Configuration

### Week 1 (command line)

| Setting | How it is given | Example |
| --- | --- | --- |
| Input folder | `--input` | `testdata/packs/ref-01` |
| Rules folder | `--rules` | `content/rules/2025-26` |
| Tax year | `--tax-year` (quoted) | `"2025-26"` |
| Database file | `--db` | `C:\recon-dev\week1.db` |
| Report file | `--out` | `report.txt` |
| Report format | `--format` | `text` (default) or `json` |
| Worker count | engine parameter (SPEC-008); tests use 1, 4 and 16 | set in SPEC-008 |

Vocabulary and profiles are read from `content/`. There is no configuration file in Week 1.

### From Sprint 4 (server)

Data directory, listening address on the office LAN, worker count, backup path and retention, session idle timeout (default 30 minutes) and absolute timeout (10 hours). The file format is decided in SPEC-022. Secrets are never stored in it: the master key is sealed with Windows DPAPI (Sprint 7) and given to the engine on stdin only.

### Never committed (`.gitignore`)

`*.db` and SQLite side files, `/data/`, `/vault/`, `/samples-private/`, `/build/`, `*.exe` (except vendored `third_party/**/*.exe`), Office lock files `~$*`, private keys (`*.key`, `*.pem` except `*.pub.pem`), and `.claude/settings.local.json`.

---

## 11. Deployment option A: installer on the office desktop

This is the Phase 1 target (SPEC-040, Sprint 7). One desktop runs everything; staff need only a browser.

### 11.1 Hardware

| Component | Minimum | Recommended |
| --- | --- | --- |
| CPU | 8 cores (performance targets are set on 8 cores) | 8 cores or more |
| RAM | 16 GB | 32 GB, for 50 clients in parallel |
| Disk | 512 GB SSD | 1 TB NVMe SSD |
| Operating system | Windows 10 or 11, 64-bit | Windows 11 Pro |
| Network | Office LAN; fixed network name or IP address | Wired connection to the office switch |
| Backup | One external drive | Two drives in rotation, or the office NAS |
| Power | — | UPS, so a power cut does not interrupt a run or a backup |

No GPU, no cloud server and no recurring hosting fee.

### 11.2 Operating system

- Windows updates installed; a local Windows account to run the service.
- The desktop keeps a fixed IP address or network name, and stays on during working hours (ENV-06).
- No internet connection is needed at any point, including for updates.

### 11.3 Network and security

- Windows Firewall: allow the server's HTTPS port from the office subnet only; block everything else inbound.
- `recon-engine` has no network code; `recon-server` listens on the office LAN only.
- HTTPS: the installer creates a firm-local certificate authority and server certificate. Staff PCs either import that certificate once or accept a browser warning (open decision T-Q2).
- Database encrypted with SQLCipher (AES-256); vault files with AES-256-GCM; master key sealed with Windows DPAPI, plus a recovery code printed once at install.

### 11.4 Install and start (planned)

1. Copy the signed installer to the desktop by USB and run it as an administrator.
2. The installer registers `recon-server` as a Windows service, creates the data directory, seals the master key, creates the certificate and prints the recovery code.
3. Open `https://<desktop-name>:<port>` from a staff PC, log in as the first Partner, and create users.
4. Apply the starter rule pack by USB and check the Format Register lists the expected profiles.
5. Run the clean and reference packs once and confirm the expected results before real client files are uploaded.

### 11.5 Operations

- **Backups:** daily encrypted copy of `recon.db` (SQLite online backup) and the vault to the configured drive; keep N days; a monthly restore test into a scratch folder, reported in Admin.
- **Updates:** signed rule packs and binary updates by USB only; every pack can be rolled back.
- **Monitoring:** job status and backup status in Admin; daily audit-chain verification.
- **Service:** `recon-server` runs as a Windows service set to restart on failure, and restarts the engine if it exits.

---

## 12. Deployment option B: portable zip and command line

Use this for Week 1, for development, and for any office that does not want a service installed. The same programs run from a folder; nothing is installed.

### 12.1 Build

```bash
# Go is installed at C:\Program Files\Go\bin; put it on PATH if `go` is not found
go build ./cmd/...                                              # host build
GOOS=windows GOARCH=amd64 go build -o build/windows/ ./cmd/...  # Windows build from any OS
```

### 12.2 Week 1 run

```bash
go run ./cmd/recon-engine run --input testdata/packs/ref-01 --rules content/rules/2025-26 \
    --tax-year "2025-26" --db build/week1.db --out build/report.txt
go run ./cmd/recon-engine run --input testdata/packs/ref-01 --rules content/rules/2025-26 \
    --tax-year "2025-26" --db build/week1.db --out build/report.json --format json
```

On Windows, the same commands run with `build\windows\recon-engine.exe` in place of `go run ./cmd/recon-engine`.

### 12.3 Checks before every commit

```bash
gofmt -l .                                   # must print nothing
go vet ./... && GOOS=windows GOARCH=amd64 go vet ./...
go test -count=1 ./...                       # includes the architecture guard
go run ./cmd/recon-dev rule-lint ./content/rules/...       # from SPEC-007
go run ./cmd/recon-dev profile-test ./content/profiles/... # from SPEC-006
```

### 12.4 Notes

- **Windows test PC (Q5):** each build's `.exe` is run on the Windows test PC by a named person, and its output must match the developer's run (SPEC-011 AC-7).
- **Portable zip (Sprint 7):** the same binaries plus `third_party/poppler/` and the content packs, unzipped to one folder and started by hand.
- **Moving to the installer later:** copy `recon.db` and the vault; the code does not change.

---

## 13. Sprint task list

One developer, 8–9 hours a day, with the `test-engineer` and `code-reviewer` agents. Each sprint ends with a demo on test files. Tick tasks here as they finish; the spec file's acceptance criteria remain the definition of done.

### Prerequisites (start now: outside waiting times)
- [ ] Claude Code plan with enough usage for full-day sessions plus parallel agents (Day 1)
- [ ] Windows test PC and a named person to run each build, Q5 (Day 5)
- [ ] CA available at the end of Week 1 to read the hand-written rules; booked for rule and law-table review from Week 2
- [ ] CA confirmation of ₹10 rounding for negative amounts (unblocks SPEC-002)
- [ ] Tax-suite name and version, with anonymised JSON samples, Q1 (Week 2)
- [ ] Anonymised Excel and Word staff documents, Q2 (Week 3)
- [ ] Tally version and written export steps, Q3 (Week 3)
- [ ] Number of concurrent staff users, Q6 (Week 4)
- [ ] Decision on finding states (including `withdrawn`) and who may disable a rule (Weeks 4 and 6)
- [ ] Anonymised GST portal, TRACES, EPF ECR and ESIC samples (Week 7)
- [ ] Decisions on the HTTPS certificate on staff PCs and on the licence check, Q4 (Week 9)
- [ ] Office desktop meeting the minimum, two backup drives and a UPS (Week 9)
- [ ] Names of the pilot staff and the 5 real client files for the pilot (Week 13)

### Sprint 1 — Reconciliation core (Week 1) · Gate G1: files in, findings out

**Day 1**
- [ ] SPEC-001: first CI run green on GitHub; mark Done
- [ ] SPEC-004: fact vocabulary (Done; re-check after SPEC-001)
- [ ] SPEC-005: adapter registry (sorted, no duplicates); JSON adapter with `UseNumber()`; JSON-path subset; `PARSE_ERROR` with line/offset; size limit

**Day 2**
- [ ] SPEC-006: profile schema and loader; detection (`PROFILE_NO_MATCH`, `PROFILE_AMBIGUOUS`); executor; `generic.taxsuite.computation` and `generic.taxsuite.tax_audit_report`, two samples each; `recon-dev profile-test`; `docs/fixtures.md`
- [ ] SPEC-010: SQLite driver chosen (passes archtest; ADR in `docs/adr/`); migrations; writer goroutine; Phase 1 tables; 200,000 facts in under 10 s

**Day 3**
- [ ] SPEC-007: rule schema on `internal/strictyaml`; validation; fixture format; `recon-dev rule-lint`
- [ ] Hand-write RECON-DEP-002 and RECON-DEP-001 with pass and fail fixtures
- [ ] Add vocabulary entries for 3–8 further R1 rules; write those rules with fixtures

**Day 4**
- [ ] SPEC-008: `CheckType` registry; `EvalCtx`; rule selection by tax year; dependency index; `equal` check with both tolerance kinds; `CannotCheck`; parallel evaluation sorted before return; fixture runner; 600 rules over 10,000 facts in under 5 s
- [ ] SPEC-009: `equal` template; text report grouped by review document; "Cannot check" section; canonical JSON report; banned-word test; golden file `testdata/golden/RECON-DEP-002.txt`

**Day 5**
- [ ] SPEC-011: `recon-engine run`; synthetic packs `ref-01` and `clean-01`; exit codes 0/1/2/3
- [ ] `test-engineer` checks every acceptance criterion of SPEC-005 to 011; `code-reviewer` reviews each spec's changes; fixes applied
- [ ] Completion records filled in; File Guide updated; statuses set in specs and `specs/README.md`

**Day 6 (buffer)**
- [ ] Carry-over from Days 1–5
- [ ] Stretch: SPEC-018 Excel/CSV adapter, taken ahead of spec order (its only dependency, SPEC-006, is done)

### Sprint 2 — Rule library R1–R10 (Week 2) · Gate G2: R1–R10 caught, zero false alarms
- [ ] SPEC-012: `sum_equals`, `gte`, `field_present`, `domain`
- [ ] SPEC-013: law-table format, loader, as-of-date lookup
- [ ] SPEC-014: Starlark `formula` rules in `internal/starhost` (no floats, step limit)
- [ ] SPEC-015: draft history; `co_movement`, `draft_regression`
- [ ] SPEC-016: starter rules for every category R1–R10, each with pass and fail fixtures
- [ ] SPEC-017: determinism (shuffled order, different `GOMAXPROCS`), incremental ≡ full, performance benchmark in CI

### Sprint 3 — Excel, Word, Tally and new heads (Week 3)
- [ ] SPEC-018: Excel/CSV adapter (if not done on Day 6); excelize passes the archtest
- [ ] SPEC-019: Word table adapter; library chosen after the network-code check
- [ ] SPEC-020: Tally XML adapter and `records` table
- [ ] SPEC-021: new-head detection and Worklist from two years of ledgers

### Sprint 4 — Server and three screens (Weeks 4–5)
- [ ] SPEC-022: `recon-server`, login and roles
- [ ] SPEC-023: DB-backed job queue, engine supervision, stdin/stdout protocol; SQLite contention load test
- [ ] SPEC-024: clients, engagements, uploads, vault, drafts, duplicate detection
- [ ] SPEC-025: findings lifecycle and decisions (after CA sign-off on states)
- [ ] SPEC-026: Season board, Client file, Worklist; keyboard-first; Hindi/English labels

### Sprint 5 — Rule Builder and law tables (Week 6)
- [ ] SPEC-027: rule and law-table governance; Rule Builder form → YAML; rule test on a past client; Partner approval; disable; reset to standard
- [ ] SPEC-028: Format Register and vocabulary browser

### Sprint 6 — GST, TDS and payroll (Weeks 7–8)
- [ ] SPEC-029: `match_records`, `threshold`, `within_due_date`, `all_periods_present`
- [ ] SPEC-030: PDF (`pdftotext -layout`, region profiles) and fixed-width adapters
- [ ] SPEC-044: refuse photos and scanned PDFs; list image pages as not read
- [ ] SPEC-031: GST module
- [ ] SPEC-032: TDS module
- [ ] SPEC-033: payroll module

### Sprint 7 — Security, audit and installer (Weeks 9–10)
- [ ] SPEC-034: SQLCipher, vault encryption, DPAPI key sealing, recovery code
- [ ] SPEC-035: HTTPS, sessions, full role matrix
- [ ] SPEC-036: hash-chained audit log
- [ ] SPEC-037: backup and restore
- [ ] SPEC-038: Excel, watermarked PDF and client query letter exports
- [ ] SPEC-039: client export, deletion and retention
- [ ] SPEC-040: Windows installer and portable zip

### Sprint 8 — Rule packs and updates by USB (Week 11)
- [ ] SPEC-041: content packs: sign, verify, install, conflict check, rollback
- [ ] SPEC-042: `recon-updater`

### Sprint 9 — Testing and tuning (Week 12) · Gate G3: test targets met
- [ ] Full test set (§15) run and reported
- [ ] Speed measured with 200,000 vouchers and 50 clients
- [ ] Any rule that fires wrongly disabled, not tuned
- [ ] CA review and sign-off of the starter rules and law-table values

### Sprint 10 — Pilot (Weeks 13–14) · Gate G4: go/no-go
- [ ] Install on the firm's main desktop
- [ ] Staff run the tool on at least 5 real client files in the office
- [ ] Collect feedback
- [ ] Go/no-go report against the BRD success criteria

---

## 14. Status

| Sprint | State | Notes |
| --- | --- | --- |
| Prerequisites | Not started | |
| 1 Reconciliation core | In progress | SPEC-001 waiting for first CI run; SPEC-002 Blocked (SPEC-001, ₹10 rounding of negatives); SPEC-003 and SPEC-004 Done |
| 2 Rule library R1–R10 | Not started | |
| 3 Excel, Word, Tally, new heads | Not started | |
| 4 Server and three screens | Not started | |
| 5 Rule Builder and law tables | Not started | |
| 6 GST, TDS, payroll | Not started | |
| 7 Security, audit, installer | Not started | |
| 8 Rule packs and updates | Not started | |
| 9 Testing and tuning | Not started | |
| 10 Pilot | Not started | |

The status in each spec file is the source of truth; `specs/README.md` mirrors it, and this table summarises it.

**Decisions log**

| Date | Decision |
| --- | --- |
| 2026-10-13 | Phase 0 confirmed cleared; building started with SPEC-001 |
| 2026-10-13 | Go for everything; engine as a separate program with no network code, proven by a dependency list on every build |
| 2026-10-13 | Strict YAML via `go.yaml.in/yaml/v3` (maintained continuation of `gopkg.in/yaml.v3`) in `internal/strictyaml` |
| 2026-10-13 | No OCR in Tool 1; photos and scanned PDFs are refused (SPEC-044) |
| 2026-10-13 | Plain SQLite until Sprint 7, then SQLCipher behind the same `store.Open` |
| 2026-10-13 | Build model: one developer full time (8–9 hours a day) with Claude Code, plus `test-engineer` and `code-reviewer` agents |
| 2026-10-13 | Week 1 is strictly the reconciliation core from the command line: ingestion, hand-written rules, findings report. No UI, voice or other features |

---

## 15. Testing

### Test set (synthetic, `testdata/packs/`)

| Part | Size | Checks | From |
| --- | --- | --- | --- |
| Reference filing pack with planted defects | 10 defect types (R1–R10) | Each defect caught, with both values, the difference and both sources | Week 1 (depreciation); Week 2 (all) |
| Clean filing pack | 1 pack | Zero findings | Week 1 |
| Packs with one figure removed | 1 per rule category | "Cannot check", never a mismatch | Week 1 |
| Drafts of one document | 3 drafts | Draft comparison and version regression (R10) | Week 2 |
| Ledgers for two years | 2 years | New, vanished, broken-recurrence, step-change heads | Week 3 |
| GST, TDS and payroll files | 12 GST returns, 4 TDS statements, 12 payroll files | Gaps, missing periods, late deposits | Weeks 7–8 |
| Unknown formats, scanned PDFs, photos | 1 of each | Rejected with a clear message, never guessed | Week 1 (unknown); Week 7 (scans) |
| Large synthetic load | 200,000 vouchers; 50 clients | Speed targets; identical results on a second run | Week 2 onward |

Rule fixture format (one file per case, beside the rule):

```yaml
# content/rules/2025-26/tieout/RECON-DEP-002/fixtures/fail/differs-by-2750.yaml
facts:
  - { name: "tar.depreciation_schedule.total",  period: "FY2025-26", money: "112450" }
  - { name: "computation.allowed_depreciation", period: "FY2025-26", money: "115200" }
expect:
  - { status: "fail", difference_paise: 275000 }
```

The exact fixture schema is fixed in SPEC-007; this example shows the intent.

### Targets (from the BRD)

- All ten reference defect types caught with zero false findings
- Fewer than 5 findings on a clean file; fewer than 2 wrong findings per 100 in the pilot
- Same files and rules, run twice → identical results
- One client under 3 minutes; 50 clients under 30 minutes; re-run after one changed file under 20 seconds (8-core, 16 GB)
- Every build's dependency list shows no network code in the engine; security checklist passes
- The pilot firm runs the tool on at least 5 real files and confirms the findings are useful

### Unit and property tests (minimum)

Money parse/format round-trip and rounding modes; no float in money code (archtest); engine has no `net` (archtest); strict YAML rejections; profile detection (none, one, two matches); missing required figure gives no fact; `equal` at exactly the tolerance; `CannotCheck` for a missing input; byte-identical output with 1, 4 and 16 workers; banned-word test over every template and title; golden report for RECON-DEP-002.

### How the agents are used

- **`test-engineer`** (has shell access): writes missing tests, runs `go test -count=1 ./...`, checks each acceptance criterion of the spec and reports every failure with its root cause.
- **`code-reviewer`** (read-only): reviews the changes before the commit that closes a spec and proposes fixes; the developer decides and applies them.
- Agents are peer review, not sign-off. A person (the CA, for rules and law tables) re-runs verification before a spec is marked **Verified**.

---

## 16. Later phases (not to be built now)

Keep the design open for these; do not implement them in Phase 1.

- **Voice:** an on-premise voice interface for staff. Deferred, with no spec yet.
- **Tally live collector (SPEC-043):** optional pull from Tally on the office network that drops a file into the inbox, so the manual path stays the same.
- **Local AI suggestions:** an optional local model may pre-fill a classification suggestion in the Worklist (staff confirm) or restate a finding in plain language. It may never create, suppress or change a finding.
- **Post-filing cycle:** intimations, information-statement mismatches and notices, reusing the same engine. Not yet researched.
- **More firms:** needs a decision on the licence check (Q4), a review of bundled-component licences (Poppler is GPL), and validation that other firms share the same pain.
- **Rule growth:** from 80–120 rules (usable alpha) to 300–400 (version 1) and 600+ (mature).

---

## 17. References

Project documents:

- `CLAUDE.md`: rules, commands, invariants
- `tool1-technical-spec.md`: architecture (Part A) and implementation (Part B)
- `requirements-spec-tool1.md`: requirements v0.3
- `recon-tool-spec.md`: product and engineering spec v0.2
- `specs/README.md`: spec index and status; `specs/SPEC-0NN-*.md`: each build unit
- `docs/Tool1-BRD-Phase1.docx`: business requirements
- `docs/Reconciliation & Verification Tool — Build Plan (Phase 1).docx`: build plan
- `docs/Tool1-File-Guide.docx`: plain-language guide to every file
- `Recon-Tool-Understanding-and-Gaps.docx`: spec summary and open gaps

External:

- Go: https://go.dev/doc/
- go.yaml.in/yaml/v3 (YAML for Go): https://github.com/yaml/go-yaml
- Starlark in Go: https://github.com/google/starlark-go
- excelize: https://github.com/qax-os/excelize
- SQLite write-ahead logging: https://www.sqlite.org/wal.html
- SQLCipher: https://www.zetetic.net/sqlcipher/
- Poppler (pdftotext): https://poppler.freedesktop.org/
- Go Ed25519 package: https://pkg.go.dev/crypto/ed25519
