# Tool 1 — Reconciliation & Verification Engine
### Technical Specification v0.1: Backend Architecture and Implementation

| | |
|---|---|
| **Date** | 13 October 2026 |
| **Based on** | `requirements-spec-tool1.md` (Requirements v0.3) and `recon-tool-spec.md` (Product & Engineering Spec v0.2) |
| **Status** | Draft, for review by the development team and the firm's CA |
| **Audience** | Developers (built with Claude Code), technical reviewer |

> All figures, PANs, GSTINs, thresholds and rates in this document are illustrative. No real client data and no real law-table value appears here.

This document turns the requirements into a buildable design. It has two parts:

- **Part A — Backend structure and tool architecture.** The process layout, the code layout, the core domain model and the extension points that let new formats, rules, tax years, law changes and reconciliation areas be added without redesign. Every later decision has to fit inside Part A.
- **Part B — Implementation details.** Database, ingestion, rules, law tables, findings lifecycle, modules, content packs, API, security, testing, build and the phase-by-phase build order.

Requirement IDs (ENV-, IN-, FV-, RU-, RC-, FD-, SC-, NF-) refer to the requirements v0.3. A traceability table is in §B18.

---

# Part A — Backend structure and tool architecture

## A1. What the architecture has to absorb

The requirements name five axes along which Tool 1 will grow (§9 of the requirements). The architecture is designed so that each axis is absorbed by **data**, not by a new program version, wherever possible.

| Growth axis | Example | Absorbed by | Needs a new binary? |
|---|---|---|---|
| New market format, or a new version of one | Tax suite changes its JSON layout; firm adopts a second tax suite | New **format profile** (data) | No |
| New check | CA wants "TDS deducted ≥ 194C threshold" | New **rule** (data), via Rule Builder or rule pack | No |
| New tax year / law change | New rate from a date; Income-tax Act 2025 forms | Tax-year-tagged **rules** + dated **law-table rows** (data) | No |
| New figure the tool must know about | A new clause in the tax audit report | New entry in the **fact vocabulary** (data) + profile + rule | No |
| New file *type* | First `.docx` support; a new binary format | New **adapter** (code) | Yes, rare |
| New check *shape* | A comparison none of the check types can express | New **check type** (code) or a Starlark rule (data) | Starlark: no. Check type: yes, rare |
| New reconciliation area | A future "Fixed-asset register" module | New **module** (code, but plugs into the same fact store and engine) | Yes |
| More clients / more volume | 50 clients, 200k vouchers each, 600+ rules | Worker pool, indexed record store, incremental re-run | No |

The design target is that **95% of change arrives as a signed content pack or a Rule Builder edit**, and only the bottom three rows ever need a release.

## A2. The four contracts

Everything in Tool 1 hangs off four versioned, data-defined contracts. Code sits between them and never hard-codes what they contain.

```
            ┌─────────────────────┐         ┌──────────────────────┐
  files ──► │  FORMAT PROFILES    │ writes  │  FACT VOCABULARY     │
            │  where each figure  │ ──────► │  the controlled list │
            │  sits in a format   │         │  of fact & record    │
            └─────────────────────┘         │  names and types     │
                                            └──────────┬───────────┘
                                                       │ reads
            ┌─────────────────────┐         ┌──────────▼───────────┐
            │  LAW TABLES         │ looked  │  RULES               │ ──► evaluations ──► findings
            │  dated values with  │ ◄────── │  YAML (+ Starlark),  │
            │  law references     │   up by │  by tax year         │
            └─────────────────────┘         └──────────────────────┘
```

| Contract | Changes when | Owned by | Validated against |
|---|---|---|---|
| **Fact vocabulary** (FV-01..04) | A new figure becomes readable | Developer + CA | Its own schema |
| **Format profile** (IN-20..25) | A market format appears or changes | Developer, from sample files | Vocabulary (may only write known facts) + its sample-file tests |
| **Rule** (RU-01..53) | Law, practice or firm preference changes | CA (office rules) / vendor CA (built-in) | Vocabulary (may only read known facts) + rule schema + fixtures |
| **Law table** (RU-31..32) | A rate, threshold, due date or section mapping changes | CA | Law-table schema |

**The decoupling rule (FV-04):** formats change → only profiles change; law changes → only rules and law tables change. A profile never names a rule and a rule never names a format, file or profile. The vocabulary is the only thing both sides share. Code review rejects any change that breaks this.

## A3. Process architecture

Tool 1 runs as up to four executables on the main office desktop (ENV-01). One installer ships them together.

```
┌──────────────────────────── MAIN OFFICE DESKTOP (Windows) ─────────────────────────────┐
│                                                                                         │
│   Staff browsers ──HTTPS (office LAN)──►  ┌───────────────────────────────┐             │
│                                           │ recon-server                  │             │
│                                           │  • HTTP API + embedded React  │             │
│                                           │  • auth, sessions, roles      │             │
│                                           │  • uploads → file vault       │             │
│                                           │  • job queue (DB-backed)      │             │
│                                           │  • supervises recon-engine    │             │
│                                           │  • content-pack install       │             │
│                                           └──────┬──────────────┬─────────┘             │
│                                   stdin/stdout   │              │ SQL                   │
│                                   (JSON lines)   │              ▼                       │
│                                           ┌──────▼────────┐  ┌──────────────────────┐   │
│                                           │ recon-engine  │  │ recon.db             │   │
│                                           │  NO NETWORK   │─►│ SQLite + SQLCipher   │   │
│                                           │  • detect     │  └──────────────────────┘   │
│                                           │  • extract    │  ┌──────────────────────┐   │
│                                           │  • rules      │─►│ vault/ (encrypted    │   │
│                                           │  • lifecycle  │  │ original files)      │   │
│                                           └──────┬────────┘  └──────────────────────┘   │
│                                                  │ os/exec                              │
│                                           ┌──────▼────────┐                             │
│                                           │ pdftotext.exe │ (bundled Poppler)           │
│                                           └───────────────┘                             │
│                                                                                         │
│   recon-collector (optional, later) — Tally pull on the LAN, writes files to an inbox  │
│   recon-updater   — applies a signed binary update from USB, keeps previous for rollback│
└─────────────────────────────────────────────────────────────────────────────────────────┘
        ▲ USB only: signed content packs and signed binary updates (ENV-05). No internet (ENV-03).
```

| Process | Responsibility | Network | Notes |
|---|---|---|---|
| `recon-server` | UI, API, auth, uploads, job queue, engine supervision, content-pack verification and install, exports, backups | Listens on office LAN only | Never evaluates rules itself |
| `recon-engine` | Format detection, extraction, fact/record store writes, rule evaluation, findings lifecycle | **None. Must not import `net` or anything that pulls it in** (NF-04, CLAUDE.md invariant 5) | Also runs stand-alone as the Phase 1 CLI |
| `recon-collector` | Optional live Tally pull; output is a *file* dropped into the vault inbox, so the manual path stays identical (CLAUDE.md invariant 7) | LAN to Tally only | Phase 3+ and only if the firm wants it |
| `recon-updater` | Verifies and swaps binaries from a USB update, keeps N−1 for rollback | None | Separate so the engine and server never replace themselves |

### A3.1 Server ↔ engine protocol

- The server starts **one long-lived** `recon-engine` child process and restarts it if it exits.
- The DB key is passed to the engine as the first line on **stdin** (never in arguments or environment variables, which other processes can read).
- After that, the two exchange newline-delimited JSON messages on stdin/stdout. `os.Stdin`, `os.Stdout` and `encoding/json` do not import `net`.
- Messages are small control messages only. Bulk data (files, facts, findings) goes through the database and vault, not the pipe.

```jsonc
// server → engine
{"type":"job.ready","job_id":4182}
{"type":"job.cancel","job_id":4182}
{"type":"shutdown"}
// engine → server
{"type":"job.progress","job_id":4182,"stage":"rules","done":312,"total":604}
{"type":"job.done","job_id":4182,"run_id":901,"findings_open":7,"cannot_check":2}
{"type":"job.failed","job_id":4182,"error_code":"PROFILE_NO_MATCH","detail":"..."}
```

The job's state lives in the `jobs` table, so if either process dies the work resumes from the DB, not from memory (CLAUDE.md invariant 8: in-process queue, state in DB).

### A3.2 Why the engine writes to the DB directly

The alternative (engine is a pure function, server writes everything) was considered and rejected: a client with 200k vouchers would stream tens of MB of facts over the pipe on every run and the incremental re-run target (< 20 s, SC-06) would suffer. Instead:

- **Engine owns writes** to: `document_versions.extraction_*`, `facts`, `records`, `runs`, `run_*`, `evaluations`, `findings` (system transitions), `worklist_items`.
- **Server owns writes** to: users, sessions, clients, engagements, uploads, jobs, finding *decisions*, rules and law tables (via Rule Builder), packs, audit log.
- Both open the database in WAL mode with `busy_timeout`. Each process funnels its writes through **one writer goroutine** with short, batched transactions. The engine writes one client run in a small number of large transactions.
- Load test in Phase 4 confirms this at 50 concurrent clients. If contention is measurable, the fallback is that the engine becomes the single writer for all tables and the server sends it write commands over the pipe. The repository interfaces in §A5 make that switch local to `internal/store`.

## A4. Repository and package layout

One Go module, one React app, content kept beside the code so it is versioned and tested with it.

```
/
├── cmd/
│   ├── recon-engine/          main: engine daemon mode + CLI mode (Phase 1 entry point)
│   ├── recon-server/          main: HTTP server, embeds web/dist via go:embed
│   ├── recon-collector/       main: optional Tally pull (later)
│   ├── recon-updater/         main: binary swap + rollback
│   └── recon-dev/             developer tools: profile tester, rule lint, pack builder/signer,
│                              synthetic data generator, import-graph check. Never shipped to firms.
│
├── internal/
│   ├── money/                 Paise type, parsing from decimal text, rounding modes, Indian formatting
│   ├── period/                canonical period strings (FY, quarter, month), FY arithmetic
│   ├── domain/                pure types: Fact, Record, Locator, Rule, Evaluation, Finding, Run…
│   ├── strictyaml/            strict YAML decoding shared by vocabulary, profiles, rules, law tables (§B4.1)
│   ├── vocab/                 fact vocabulary loader + validator
│   ├── ingest/                detection, Adapter interface, adapter registry, validation report
│   │   └── adapters/          json/  xlsx/  csv/  docx/  tallyxml/  pdftext/  fixedwidth/
│   ├── profile/               format-profile schema, loader, executor (profile + adapter → facts)
│   ├── rules/                 rule YAML schema, strict loader, linter, YAML generator (Rule Builder)
│   ├── lawtable/              law-table schema, loader, as-of-date lookup
│   ├── engine/                planner, dependency index, evaluator, check-type registry
│   │   └── checks/            equal, sum_equals, gte, within_due_date, all_periods_present,
│   │                          field_present, threshold, match_records, co_movement,
│   │                          draft_regression, formula
│   ├── starhost/              Starlark sandbox: step limits, no floats, law() / fact() builtins
│   ├── findings/              stable IDs, value fingerprints, lifecycle state machine, wording templates
│   ├── modules/               tieout/ newheads/ gst/ tds/ payroll/ drafts/   (see A6.4)
│   ├── store/                 SQLite repositories (explicit SQL), migrations, writer goroutine
│   ├── vault/                 encrypted content-addressed file storage
│   ├── jobs/                  DB-backed queue, worker pool, cancellation
│   ├── ipc/                   server ↔ engine message types
│   ├── pack/                  content-pack format, Ed25519 verify, install, rollback, conflict detection
│   ├── audit/                 hash-chained append-only log
│   ├── authz/                 roles and permission matrix (server only)
│   ├── api/                   HTTP handlers (server only)
│   ├── export/                Excel, watermarked PDF, client query letter (server only)
│   └── archtest/              tests that enforce the import rules below
│
├── content/                   everything that ships as signed content packs
│   ├── vocabulary/            facts.yaml, records.yaml  (+ generated docs/fact-vocabulary.md)
│   ├── profiles/<source>/<document>/<version>/   profile.yaml, samples/, expected/
│   ├── rules/<tax-year>/<module>/<RULE-ID>/      rule.yaml, fixtures/pass/, fixtures/fail/, *.star
│   └── lawtables/             one YAML per table
│
├── web/                       React + TypeScript UI (built to web/dist, embedded in recon-server)
├── docs/                      fact-vocabulary.md, fixtures.md, export-guide.md, adr/
├── testdata/                  synthetic end-to-end filing packs (no real client data, ever)
└── third_party/poppler/       vendored pdftotext.exe + licence
```

### A4.1 Import rules (enforced by `internal/archtest`)

| Package group | May import | Must never import |
|---|---|---|
| `domain`, `money`, `period` | standard library only | anything else in the repo |
| `engine`, `checks`, `findings`, `rules`, `lawtable`, `profile`, `ingest`, `starhost`, `modules` | `domain`, `money`, `period`, `vocab`, `store` interfaces | `api`, `authz`, `export`, `net`, `net/*`, `crypto/tls` |
| `cmd/recon-engine` (transitively) | anything above | **`net`, `net/http`, `net/*`, `crypto/tls`, `os/user` via cgo net resolvers** |
| `api`, `authz`, `export` | anything | — (server only) |

The check is a Go test: it runs `go list -deps ./cmd/recon-engine` and fails if any `net` package appears. Its output (the full dependency list) is saved as the build artefact that proves the engine cannot open a socket. Every third-party library added to the engine side (YAML, Starlark, excelize, SQLCipher driver, docx reader) must pass this test before it is merged; see risk T1 in §B17.

## A5. Core domain model

These types live in `internal/domain` and are the only shapes that cross package boundaries. They contain no I/O.

```go
// money
type Paise int64 // all money; ₹1 = 100. Never float64.

// A Locator says exactly where a value came from (FV-01, FD-02).
type Locator struct {
    DocumentVersionID int64
    FileName          string // as uploaded, for display
    Kind              string // "json_path" | "sheet_cell" | "table_cell" | "pdf_pos" | "xml_path" | "line"
    Path              string // "$.business_income.allowed_depreciation" | "TB!D42" | "T3!R7C4" | "p7:l23:c41-52"
}

// A Fact is one scalar figure (Spec v0.2 M2 model).
type Fact struct {
    EngagementID int64
    Name         string   // from the vocabulary, e.g. "computation.allowed_depreciation"
    Period       string   // "FY2025-26" | "FY2025-26:Q1" | "2025-04"
    Scope        string   // "" | GSTIN | branch code | Tally company id   (IN-11)
    Value                 // embedded, so f.Money, f.Text ... read the figure
    Locator      Locator
}

// A Value is one figure; only the field named by Kind is meaningful. Shared by Fact and
// LabeledValue. The zero ValueKind is invalid, so every figure names its kind.
type Value struct {
    Kind   ValueKind // Money | Int | Text | Date | Bool | Rate
    Money  Paise
    Int    int64
    Text   string
    Date   string    // ISO "2025-07-31"
    Bool   bool
    RateBP int64     // basis points, 1% = 100
}

// A Record is one row of a tabular source: voucher, invoice, challan, ECR line.
// Records exist because GST/TDS/payroll matching is row-level, not scalar (RC-03..05).
type Record struct {
    EngagementID int64
    RecordType   string            // from the vocabulary: "voucher" | "gst_invoice" | "tds_challan" | "ecr_row" ...
    Period       string
    Scope        string
    MatchKey     string            // canonical, per record type, e.g. "27ABCDE1234F1Z5|INV-0042|2025-05-14"
    Amounts      map[string]Paise  // named per record-type schema
    Fields       map[string]string // named per record-type schema
    Locator      Locator
}

type EvalStatus uint8 // Pass | Fail | CannotCheck (RU-52); the zero value is invalid

// A Subject is what one evaluation is about within an engagement. Key() is the canonical
// "period|scope|match_key" ("|" and "\" escaped in period and scope; the match key is last
// and written as is). The engagement is passed to StableID separately.
type Subject struct { Period, Scope, MatchKey string }

// A LabeledValue is one side of a comparison: rule input name, label, value, locator.
type LabeledValue struct { Input, Label string; Value; Locator Locator }

type LawRef struct { Act, Section, Reference string }

// An Evaluation is the outcome of one rule for one subject (engagement + period + scope + match key).
type Evaluation struct {
    RunID        int64
    RuleID       string
    RuleVersion  string
    Subject      Subject
    Status       EvalStatus
    Values       []LabeledValue // every side, with locator; in rule-declared order
    Difference   Paise          // computed, never "expected"
    MissingFacts []string       // when CannotCheck
    LawRefs      []LawRef
}

// A Finding is a failed evaluation with a lifecycle (FD-01..05).
type Finding struct {
    StableID         string // hash(rule_id, engagement, period, scope, match_key) — no version, no run
    ValueFingerprint string // hash of input values only (FD-04)
    State            FindingState
    ReviewDocument   string // document type to group under in the UI (FD-06)
    LatestEvaluation Evaluation
}

// A RunSnapshot records every versioned input of a run (RU-53, §A10): engine version,
// vocabulary version, document versions with the profile that read each, rule versions
// and law-table rows used.
type RunSnapshot struct { /* see internal/domain/snapshot.go */ }
```

`Evaluation` and `Finding` have a canonical JSON encoding (`CanonicalJSON()`: keys sorted, no whitespace, integers only, no HTML escaping, empty and nil lists both `[]`, a value writes only the field its kind names, `MissingFacts` sorted, values and law refs in rule order). Golden files and determinism tests compare these bytes.

All repository access goes through interfaces declared in the consuming package (for example `engine.FactReader`, `findings.Store`), implemented in `internal/store`. This keeps the engine testable against in-memory fakes and makes the A3.2 fallback a change in one package.

## A6. Extension points

These are the only places the code is designed to grow. Each is a small interface plus a registry populated at start-up. Registries are iterated in **sorted key order** (determinism, NF-03).

### A6.1 Adapter — one per file *type*

```go
// Adapter turns raw bytes into a navigable document. It knows file syntax, not meaning.
type Adapter interface {
    FileType() string                       // "json" | "xlsx" | "csv" | "docx" | "tallyxml" | "pdftext" | "fixedwidth"
    Sniff(head []byte, name string) bool    // cheap check before full parse
    Open(r io.ReaderAt, size int64) (Doc, error)
}

// Doc is what profiles query. Each adapter implements only the selectors that make sense for it.
type Doc interface {
    Select(sel Selector) ([]Cell, error)    // JSON path, sheet+header, table+row label, PDF region, XML path
}
```

Adapters never produce facts. A new file type is one new adapter package plus its registration; nothing else changes.

### A6.2 Format profile — one per market format *and* version (data)

A profile is YAML (§B2). The profile executor takes `(profile, Doc)` and produces `[]Fact` and `[]Record`, using only vocabulary names. Detection runs every installed profile's `detect` block against the file and requires **exactly one** match; zero or several matches reject the file with a message (IN-23).

### A6.3 Check type — one per comparison *shape*

```go
type CheckType interface {
    Name() string                                    // "equal", "sum_equals", "match_records", ...
    ValidateParams(p RuleCheck, v *vocab.Vocab) error // at rule-load time, so bad rules never reach a run
    Inputs(p RuleCheck) []InputRef                   // facts/records read: feeds the dependency index
    Evaluate(ctx EvalCtx, p RuleCheck) ([]Evaluation, error) // pure: reads facts + law tables via ctx only
}
```

`EvalCtx` exposes `Fact()`, `Facts()`, `Records()`, `FactHistory()` (prior drafts) and `Law()` lookups. It has no clock, no randomness, no file access and no DB handle. The `formula` check type delegates to Starlark (§B4.5) and is the escape hatch that keeps new check types rare.

### A6.4 Module — one per reconciliation area (SC-08)

```go
type Module interface {
    ID() string                         // "tieout" | "newheads" | "gst" | "tds" | "payroll" | "drafts"
    RuleNamespaces() []string           // rule-ID prefixes this module owns, e.g. "RECON-", "GST-"
    Prepare(ctx PrepareCtx) error       // optional derived data, e.g. ledger YoY aggregates, invoice match index
    Worklist(ctx EvalCtx) ([]WorklistItem, error) // optional; only newheads uses it in Tool 1
}
```

A module may add **derived records** (for example the GST module builds a per-supplier match index from `gst_invoice` records) but it must store them as ordinary records/facts with vocabulary names so rules can read them. A module never writes findings directly; findings only ever come from rule evaluations. A future reconciliation area is a new module package, new vocabulary entries, new profiles and new rules, on the same store and engine.

### A6.5 Exporter (server side)

`Exporter` renders a run's findings to Excel, watermarked PDF or a client query letter. New output formats are new exporters; they read the same finding view.

## A7. Data flow of one run

```
 upload(s) ──► vault (sha256, dedupe) ──► document_versions (new draft, IN-12/13)
                                                  │
                                       job enqueued (engagement)
                                                  ▼
 1. DETECT      for each new document version: run all profiles' detect blocks → exactly one profile
 2. EXTRACT     adapter.Open + profile executor → facts, records, validation report (IN-14)
 3. RESOLVE     build the engagement's CURRENT view: latest draft per document slot;
                combine parts / months / branches (IN-11); flag conflicting duplicates
 4. SNAPSHOT    create run: freeze rule versions, law-table versions, vocabulary version,
                profile versions and document versions used (RU-53)
 5. PLAN        dependency index: rule → facts/records it reads.
                Full run: all active rules for the tax year.
                Incremental run: only rules whose inputs changed since the previous run (SC-06)
 6. PREPARE     each module's Prepare (derived records, ledger diff aggregates)
 7. EVALUATE    rules in sorted rule-ID order across a worker pool; results sorted before write
 8. LIFECYCLE   reconcile evaluations with existing findings by stable ID (§B7)
 9. WORKLIST    newheads module produces worklist items (separate from findings, RC-02)
10. REPORT      job.done with counts; UI reads results from DB
```

For an incremental run, step 7 evaluates only the affected rules and copies forward the evaluations of unaffected rules from the previous run by reference, so every run is still a complete snapshot. A CI property test asserts **incremental result ≡ full result** on the same inputs.

## A8. Determinism and money (NF-01, NF-03, RU-51)

| Rule | How it is enforced |
|---|---|
| Money is `Paise int64` | Lint forbids `float32/float64` in `money`, `engine`, `checks`, `findings`, `profile`, `lawtable`. Decimal text ("1,12,450.50", "112450.5") is parsed by `money.Parse`, which is string-based and rejects more than 2 decimals unless the profile states a rounding mode |
| Rates are integer basis points; ratios are `num/den` integers | Multiplication uses `money.MulRate(p Paise, bp int64, mode Rounding)`; overflow returns an error, never wraps |
| Rounding is explicit | Every rule or law value that rounds names a mode: `none`, `half_up_rupee`, `nearest_10_rupees`, etc. No implicit rounding anywhere |
| No map-order dependence | Registries, facts, rules and evaluations are always sorted before iteration that affects output; a lint flags `range` over maps in the engine packages unless followed by a sort |
| No clock in rules | `EvalCtx` has no time. "As-of" dates for law-table lookups come from the period being checked, never from `time.Now()`. Run timestamps live only in run metadata and are excluded from finding fingerprints |
| No randomness | No `math/rand` in engine packages (archtest) |
| Same input → same bytes | CI runs each fixture pack twice, with shuffled upload order and different `GOMAXPROCS`, and compares the canonical JSON output byte for byte |
| No AI in the finding path | Nothing in `cmd/recon-engine` may import an ML/LLM client (archtest allow-list) |

## A9. Concurrency and scale (SC-04..07)

- **Unit of work = one engagement run.** A worker pool of `max(1, cores − 2)` workers (configurable) processes jobs; 50 queued clients on 8 cores run 6 at a time.
- **Within a run**, rule evaluation is parallel across rules, since rules are pure functions over a read-only snapshot. Results are sorted before writing.
- **Volume:** 200k vouchers are stored as `records` with indexed `(engagement_id, record_type, period, match_key)`. Record matching (`match_records`) uses a sorted-merge join in Go over two sorted slices, not N×M loops.
- **600+ rules:** rules are compiled once per run (YAML parsed and validated at load, Starlark programs compiled and cached by content hash). The dependency index makes incremental runs touch only affected rules.
- **Performance budget (8-core / 16 GB):** extraction ≤ 60 s, rules ≤ 60 s, lifecycle + write ≤ 30 s for one large client. A benchmark using the synthetic data generator (`recon-dev synth`) runs in CI from Phase 2 and fails on > 20% regression.
- **Escape hatch:** if one firm exceeds the envelope, DuckDB as a secondary analytical store for records (v0.2 §5.2.2) is the first lever, not a server database.

## A10. Versioning — everything that affects a result has a version

| Thing | Version scheme | Stored in run snapshot |
|---|---|---|
| Engine binary | semver, e.g. `1.4.2` | yes |
| Fact vocabulary | integer, monotonically increasing; names are never reused with a different meaning | yes |
| Format profile | `profile_id` + `profile_version` | yes, per document version |
| Rule | `id` + `version`; office edits create `version` + `-office.N` (RU-12) | yes, per evaluation |
| Law-table row | row id + content hash | yes, per lookup used |
| Content pack | `pack_id` + semver + tax-year scope | yes |

A run can therefore be re-explained years later: "this finding came from rule RECON-DEP-002 v1, law row TDS-THR-0193, profile generic.taxsuite.computation v1, engine 1.4.2."

---

# Part B — Implementation details

## B1. Database schema (SQLite + SQLCipher)

Explicit SQL, migrations as numbered `.sql` files in `internal/store/migrations`, applied at start-up inside a transaction. `PRAGMA foreign_keys = ON; journal_mode = WAL; synchronous = NORMAL; busy_timeout = 5000`. All money columns are `INTEGER` paise and named `*_paise`.

### B1.1 Tables by area

| Area | Table | Key columns (abridged) |
|---|---|---|
| **Firm & users** | `users` | id, username, display_name, role (`partner`/`manager`/`staff`/`readonly`), password_hash (argon2id), active, created_at |
| | `sessions` | id (random), user_id, created_at, last_seen_at, expires_at |
| **Clients** | `clients` | id, name, pan_hash, pan_last4, status |
| | `client_scopes` | id, client_id, kind (`gstin`/`branch`/`tally_company`), code, label |
| | `engagements` | id, client_id, tax_year (`"2025-26"`), regime (`ITA1961`/`ITA2025`), status |
| **Files** | `files` | sha256 (PK), size, vault_path, mime, first_uploaded_at |
| | `documents` | id, engagement_id, document_type (vocabulary), period, scope, part — the **slot** (IN-11) |
| | `document_versions` | id, document_id, draft_no, file_sha256, profile_id, profile_version, uploaded_by, uploaded_at, extraction_status, validation_report_json |
| **Facts** | `facts` | id, document_version_id, engagement_id, name, period, scope, kind, money_paise, int_value, text_value, date_value, rate_bp, locator_kind, locator_path |
| | `records` | id, document_version_id, engagement_id, record_type, period, scope, match_key, amount_paise (primary), amounts_json, fields_json, locator_kind, locator_path |
| **Content** | `vocab_versions`, `vocab_entries` | version, name, kind, record_type, description, unit |
| | `profiles` | profile_id, profile_version, source_system, document_type, file_type, body_yaml, sha256, pack_id, status |
| | `rule_versions` | rule_id, version, source (`builtin`/`office`), tax_years, body_yaml, sha256, pack_id, created_by, created_at |
| | `rule_state` | rule_id, active_version, status (`draft`/`testing`/`active`/`disabled`), approved_by, approved_at, disabled_reason |
| | `law_tables`, `law_rows` | table, row_id, key_json, value_json, valid_from, valid_to, law_ref, sha256, approved_by |
| | `packs` | pack_id, version, kind, signed_by_key, installed_at, installed_by, previous_pack_id, status |
| **Runs** | `jobs` | id, engagement_id, kind (`full`/`incremental`/`rule_test`), state, attempts, requested_by, created_at, started_at, finished_at, error |
| | `runs` | id, job_id, engagement_id, engine_version, vocab_version, base_run_id (for incremental), started_at, finished_at |
| | `run_documents`, `run_rules`, `run_law_rows` | run_id + what was used (RU-53) |
| | `evaluations` | run_id, rule_id, rule_version, subject_key, status, values_json, difference_paise, missing_json |
| **Findings** | `findings` | stable_id (PK), engagement_id, rule_id, subject_key, state, value_fingerprint, review_document, latest_run_id, opened_at, decided_by, decision, decision_note |
| | `finding_events` | id, stable_id, from_state, to_state, actor (user id or `system`), run_id, reason, at |
| | `worklist_items` | id, engagement_id, kind (`new_head`/`vanished`/`broken_recurrence`/`step_change`/`unmapped`), ledger, details_json, classification_json, state |
| **Audit** | `audit_log` | seq (PK), at, actor, action, object_type, object_id, details_json, prev_hash, hash |

### B1.2 Key indexes

- `facts(engagement_id, name, period, scope)`
- `records(engagement_id, record_type, period, match_key)`
- `evaluations(run_id, rule_id)`, `findings(engagement_id, state)`
- `document_versions(document_id, draft_no)` unique

### B1.3 Data protection in the schema

- PAN is stored only as `pan_hash` (for matching uploads to clients) and `pan_last4` (for display). Full PANs inside facts are text facts like any other and stay inside the encrypted DB.
- `audit_log` has `BEFORE UPDATE` and `BEFORE DELETE` triggers that raise an error (append-only, NF-05).
- Client deletion (NF-09) deletes the client's facts, records, files, findings and worklist and writes a tombstone audit entry that keeps IDs but no personal data.

## B2. Ingestion and format profiles

### B2.1 Upload path

1. Server streams the upload to a temp file, computes SHA-256, and identifies the file by its contents (`ingest.Sniff`, magic bytes, not the extension). A photo or other image file is refused with `IMAGE_NOT_SUPPORTED` and never stored; only its name, size, SHA-256 and the reason are kept (SPEC-044). There is no OCR in Tool 1. The server then checks `files`. Identical bytes already present for this engagement → reported as "duplicate, skipped" (IN-13).
2. New bytes are encrypted into the vault as `vault/ab/cd/<sha256>.bin` (AES-256-GCM, key derived from the master key).
3. The user picks (or the tool proposes from detection) the document slot. A file landing in an occupied slot becomes `draft_no + 1` (IN-12); old drafts are kept.
4. A job is enqueued; the engine detects, extracts and writes a **validation report** (IN-14):

```
File: comp_20261008.json
Recognised as: Generic tax suite — Computation (profile generic.taxsuite.computation v1)
Tax year: 2025-26   Client match: PAN ****234F ✓
Figures extracted: 142 facts, 0 records
Not read: 3 fields — $.annexure.notes (text, not in vocabulary), …
Warnings: none
```

### B2.2 Profile file format

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

context:                      # where the slot keys come from
  tax_year: { json_path: "$.meta.tax_year" }
  pan:      { json_path: "$.assessee.pan" }
  period:   { const: "FY" }

facts:
  - fact: "computation.allowed_depreciation"
    json_path: "$.business_income.allowed_depreciation"
    unit: "rupees"            # rupees | paise ; parsed as decimal text, never float
    required: true            # missing → validation-report error, not a silent zero
  - fact: "computation.fixed_asset_schedule.total"
    json_path: "$.fixed_assets.schedule_total"
    unit: "rupees"
```

Excel profiles locate columns **by header text, not by column letter**, and find the header row by matching labels, so a moved column does not silently shift values:

```yaml
file_type: "xlsx"
detect:
  - sheet_exists: "Trial Balance"
  - header_row_contains: { sheet: "Trial Balance", labels: ["Particulars", "Debit", "Credit"] }
records:
  - record_type: "tb_line"
    sheet: "Trial Balance"
    header_labels: { ledger: "Particulars", debit: "Debit", credit: "Credit", group: "Group" }
    match_key: [ "ledger" ]
    stop_at_first_blank_row: true
```

PDF profiles (fallback only, IN-05) address **regions** on pages of `pdftotext -layout` output: page, anchor text, line offset and column character range. Each PDF profile must declare its column ranges and is tested against samples where columns sit close together, because column reordering is the known failure mode that produces confident false findings.

Word profiles (IN-03) address tables only: `table_index` or `table_anchor` (text in the first row), then row label and column header. Paragraph text is never read (A4).

### B2.3 Profile testing (IN-22)

Each profile directory contains `samples/` and `expected/` (one hand-verified `*.facts.yaml` per sample). `recon-dev profile-test` and CI require that extraction equals the expected list exactly: same names, values, periods and locators. A profile cannot be installed from a pack unless its tests are listed in the pack manifest as passed at build time.

Until real tax-suite samples arrive (IN-26, A2), the `generic.*` synthetic profiles and `docs/fixtures.md` define the formats.

### B2.4 Format Register (IN-25)

The register is a view over `profiles`: source system, document, format version, file type, profile version, date added, pack, and number of samples tested.

## B3. Fact vocabulary

`content/vocabulary/facts.yaml` and `records.yaml`; `docs/fact-vocabulary.md` is **generated** from them (`go run ./cmd/recon-dev vocab-doc`) and never hand-edited.

Both files start with a `schema` (`recon.vocab.facts/1`, `recon.vocab.records/1`) and the same integer `vocab_version`. `facts.yaml` also lists the **document types**, each with a short `prefix` that every fact name of that type starts with (`tax_audit_report` → `tar.`), so the name tells you its document:

```yaml
schema: "recon.vocab.facts/1"
vocab_version: 1
document_types:
  - name: "tax_audit_report"
    prefix: "tar"
    description: "Tax audit report (Form 3CA/3CB with 3CD)"
    since_vocab: 1
facts:
- name: "tar.depreciation_schedule.total"
  kind: "money"
  periodicity: "FY"
  document_type: "tax_audit_report"
  description: "Total of the depreciation schedule in the tax audit report"
  since_vocab: 1

- name: "computation.allowed_depreciation"
  kind: "money"
  periodicity: "FY"
  document_type: "computation"
  description: "'Allowed depreciation' line in the computation of income"
  since_vocab: 1
```

```yaml
record_types:
  - name: "gst_invoice"
    description: "One invoice line from GSTR-1, GSTR-2B or the books"
    match_key: [ "counterparty_gstin", "invoice_no", "invoice_date" ]
    amounts: [ "taxable_value", "igst", "cgst", "sgst", "cess" ]
    fields:  [ "counterparty_gstin", "invoice_no", "invoice_date", "place_of_supply", "source" ]
    since_vocab: 2
```

`kind` is one of `money`, `int`, `text`, `date`, `bool`, `rate`; `periodicity` is `FY`, `quarter` or `month`. Record types carry `since_vocab` and optional `deprecated_in` like facts. A `match_key` entry must be one of the record's `fields`; a column is in `amounts` or `fields`, never both.

**Version discipline.** Every change increases `vocab_version` in both files. The generated document prints the version and a SHA-256 of the vocabulary's content (not the file bytes, so comments and order do not count). The generator, and a test run in CI, refuse a content change whose version did not increase, and the test fails whenever the committed document differs from the generated one.

Naming: `<document_type>.<section>.<item>`, lower snake case, ASCII. A name is never reused with a different meaning; a meaning change is a new name, and the old one is marked `deprecated_in`. Regime-specific forms (Act 1961 vs Act 2025) get distinct names where their content differs; a `law_table` (`section_map_1961_2025`) maps between section numbers for display.

## B4. Rules

### B4.1 Strict YAML (RU-02)

- Parser: `go.yaml.in/yaml/v3` (the maintained continuation of the archived `gopkg.in/yaml.v3`, same code and API), decoding into typed Go structs with `KnownFields(true)`, so unknown keys are rejected. Implemented once in `internal/strictyaml` (built in SPEC-004) and used by every content loader.
- A yes/no field accepts only plain `true` or `false` (the library on its own would also take `yes`/`on` into a Go `bool`); a number field accepts only plain decimal digits (no `0x`, `1_000`, `+1`, quotes).
- After decoding, the loader walks the YAML node tree and **rejects any plain (unquoted) scalar** in fields declared as text (every Go `string` field): IDs, tax years, dates, section numbers, versions. `tax_years: [2025-26]` fails; `tax_years: ["2025-26"]` passes.
- Anchors, aliases, merge keys, custom tags, duplicate keys and multi-document files are rejected. Every problem in a file is reported at once, each with file and line.
- The same loader runs on save (Rule Builder), on pack install and at engine start.

### B4.2 Rule file format

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
review_document: "computation"     # UI grouping only (FD-06); never appears in finding text
```

**Lifecycle status is not in the file.** Status (draft/testing/active/disabled), approver and timestamps live in `rule_state` and `rule_versions`, because a signed built-in rule file cannot change when a Partner disables it. The UI shows them together with the rule (requirements §6.2).

### B4.3 Check types (initial registry)

| Check type | Inputs | Fails when | Typical category |
|---|---|---|---|
| `equal` | a, b (money/int/text) | `|a − b| > tolerance` | R1 |
| `sum_equals` | parts[], total | `|Σparts − total| > tolerance` | R1, R2 |
| `gte` | a, b | `a < b − tolerance` | R4 |
| `threshold` | a, law value | `a` crosses a law-table limit | TDS, GST |
| `within_due_date` | event date fact/record field, law due-date table | date later than the law-table due date for that period | TDS, payroll |
| `all_periods_present` | fact or record type, period set | any expected period has no value | R6 |
| `field_present` | field, `present_elsewhere` source | blank here while present in the named other place | R9 |
| `domain` | field, allowed set or law table | value outside the allowed set | R7 |
| `match_records` | record type A, record type B, key, amounts | key present on one side only, or amounts differ beyond tolerance | GST, TDS, payroll |
| `co_movement` | a, b, across two drafts | a changed between drafts, b did not (dependency propagation) | R3, RC-06 |
| `draft_regression` | a, across ≥ 3 drafts | value returns to an earlier value (A → B → A) | R10 |
| `formula` | Starlark script | script-computed value ≠ compared fact beyond tolerance | R2 and anything complex |

R5 (cross-form semantic conflict) is expressed as `domain` or `formula` over coded fields (for example, the audit-limb code and the deduction section present together), so the tool reports that both codes are present, not a characterisation.

Tolerance is either `{ paise: N }` or `{ bp_of_larger: N }`, computed in integers.

### B4.4 Missing inputs (RU-52)

If any required input fact is absent from the current view, the evaluation is `CannotCheck` with `missing: ["tar.depreciation_schedule.total"]`, shown as **"Cannot check — figure not found in Tax audit report"**. It never becomes a finding and never counts towards false-alarm metrics. Optional inputs are declared `optional: true` with an explicit default only where the law makes absence meaningful.

### B4.5 Starlark rules (RU-04, RU-24)

```yaml
check:
  type: "formula"
  script: "mat_book_profit.star"
  params:                          # the ONLY part the CA may edit (RU-24)
    include_tax_in_other_expenses: true
  compare_to: "computation.mat.book_profit"
  tolerance: { paise: 100 }
```

```python
# mat_book_profit.star — receives ints (paise); returns labeled ints only
def compute(fact, law, params):
    bp = fact("pl.profit_after_tax") + fact("pl.current_tax") - fact("pl.deferred_tax_credit")
    if params["include_tax_in_other_expenses"]:
        bp += fact("schedule.other_expenses.income_tax")
    return {"value": bp, "steps": [("Profit after tax", fact("pl.profit_after_tax")), ...]}
```

Sandbox rules (`internal/starhost`):

- No `load()` except other scripts inside the same pack; no I/O, no time, no randomness (Starlark has none by default; nothing is added).
- Execution step limit per call (`thread.SetMaxExecutionSteps`), so a runaway script fails the rule, not the run.
- **No floats:** a static check rejects float literals and the `/` operator (only `//`), and the host rejects any float in a return value. Money stays integer paise.
- Scripts return **values and step labels only**. Finding wording comes from the engine's templates, so a script cannot write "should" or state a correct value.
- `fact()` on a missing fact aborts the script with a typed error that becomes `CannotCheck`.
- Scripts are compiled once and cached by SHA-256.

### B4.6 Output wording (requirements §6.9, CLAUDE.md invariant 1)

All finding text is produced from fixed templates per check type in `internal/findings/templates`. A CI test renders every template and every rule title with fixture data and fails on any banned word or phrase: `should`, `incorrect`, `must be`, `wrong`, `correct value`, `ought`, `required to`. Amounts use Indian digit grouping (`1,12,450`). Every finding shows both (or all) values, the difference, every locator and the law reference:

```
[RECON-DEP-002]  Depreciation mismatch
  Tax audit report, depreciation schedule total : 1,12,450
  Computation, "Allowed depreciation"           : 1,15,200
  Difference                                    :     2,750
  Source: TAR_v3.pdf p.7 | comp_20261008.json $.business_income.allowed_depreciation
  Rule basis: Income-tax Act, 1961 – Sec 32
```

## B5. Law tables (RU-31..34)

```yaml
schema: "recon.lawtable/1"
table: "tds.section_threshold"
description: "Single-payment and aggregate thresholds by TDS section"
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

- **Lookup is by as-of date**, taken from the period under check (for example the payment date or the period end), never the system clock. Exactly one row must be valid on that date; zero or overlapping rows are a load-time error.
- A law change is a **new row** with a new `valid_from`; the old row gets a `valid_to`. Old tax years keep using old rows automatically (RU-32).
- Every row carries `law_ref` and, in the DB, `approved_by` (RU-34). Edits from the Law tables screen go through the same Draft → Partner approves flow as rules.
- Planned tables: TDS sections/thresholds/rates, GST rates by HSN and date, statutory due dates (TDS, GST, EPF, ESIC, returns), e-invoice applicability limits, Act 1961 ↔ Act 2025 section and form mapping (RU-33).

## B6. Rule engine execution

1. **Load:** active rules for the engagement's tax year (built-in active version, or the office version if one exists and is active), validated against the vocabulary of the run.
2. **Index:** for each rule, the inputs from `CheckType.Inputs`. Stored as `rule → [fact/record names]` and inverted to `name → [rules]`.
3. **Subjects:** per rule, the subjects to evaluate, from `scope: each | sum` and the periods present (for example, one evaluation per GSTIN per month).
4. **Evaluate:** in parallel, each returns `[]Evaluation`; collected, sorted by `(rule_id, subject_key)` and written in one transaction.
5. **Rule test mode (RU-25):** a `rule_test` job runs a draft rule against a past engagement's current view in a scratch run. Its evaluations go to the test run only and never touch `findings`.

## B7. Findings lifecycle (FD-01..05)

### B7.1 Identity

- `stable_id = base32(sha256(rule_id | engagement_id | period | scope | match_key))[:16]`, standard upper-case alphabet. `|` and `\` inside rule ID, period and scope are escaped with `\` so a separator cannot shift between fields; the match key is last and written as is. The rule **version** and the run are deliberately excluded, so re-runs and rule edits update the same finding instead of duplicating it (FD-01).
- `value_fingerprint = hex(sha256(canonical(values without locators)))`. Only the values, in rule order, are hashed; locators, labels and input names are not. Locator changes alone (same figure, new page number in a new draft) do not count as a value change.

### B7.2 States

The requirements list the states (FD-03). The Understanding & Gaps note flagged that the transitions were undefined; this table defines them and needs CA sign-off.

| State | Meaning |
|---|---|
| `open` | Rule fails; no decision yet |
| `decided_fix` | Staff decided the source document will be corrected |
| `decided_not_issue` | Staff decided the difference is explained ("Not an issue") |
| `decided_ask_client` | Waiting on client information |
| `awaiting_recheck` | A new version of a document this finding reads has been uploaded since the decision; re-check pending |
| `resolved` | Latest run passes |
| `reopened` | Was resolved or not-an-issue; now fails again (or values changed) |
| `carried_forward` | Explicitly carried to the next tax year's engagement by a Manager/Partner |
| `withdrawn` *(proposed)* | The rule was disabled or no longer applies; kept for history. **Not in requirements v0.3 — needs approval** |

### B7.3 System transitions applied after every run

| Existing state | Latest evaluation | New state | Note |
|---|---|---|---|
| none | Fail | `open` | |
| none | Pass / CannotCheck | — | no finding created |
| `open`, `decided_fix`, `decided_ask_client`, `awaiting_recheck` | Pass | `resolved` | actor `system`, run id recorded (FD-05) |
| `decided_fix`, `decided_ask_client` | Fail, and one of its input documents has a newer draft | `awaiting_recheck` → `open` with note "still differs after new upload" | |
| `decided_not_issue` | Fail, same fingerprint | unchanged | FD-04 |
| `decided_not_issue` | Fail, fingerprint changed | `reopened` | FD-04 |
| `resolved` | Fail | `reopened` | the "fixed defect reappears in a later draft" gap |
| any | CannotCheck | unchanged; flagged "not re-checked in run N" | never silently resolved |
| any | rule disabled | `withdrawn` (if approved) | |

User transitions (decisions) are API calls checked against the role matrix (§B12) and always write a `finding_events` row and an audit entry with the reason.

## B8. Reconciliation modules (RC-01..06)

| Module | What it adds beyond plain rules | Key record types | Output |
|---|---|---|---|
| `tieout` (RC-01) | Nothing; pure scalar rules R1–R10 | — | findings |
| `drafts` (RC-06) | `FactHistory` across draft numbers for `co_movement` and `draft_regression` | — | findings + a draft-diff view (changed facts, and dependent facts that did not change) |
| `newheads` (RC-02) | Year-on-year ledger aggregation from Tally TB/monthly summary of FY *n* and *n−1*: new, vanished, broken recurrence (≥ 10 of 12 months last year), step change (law-free, threshold in rule params), unmapped head | `tb_line`, `ledger_month` | **worklist items**, not findings |
| `gst` (RC-03) | Builds per-month, per-GSTIN aggregates and invoice match indexes from books, GSTR-1/1A, 3B, 2B, IMS | `gst_invoice`, `gst_summary` | findings |
| `tds` (RC-04) | Section-wise aggregates of books vs challans vs quarterly statements vs TRACES/26AS/AIS; expense-head → expected-section map from the worklist classification | `tds_entry`, `tds_challan`, `tds_credit` | findings |
| `payroll` (RC-05) | Month-wise headcount/gross from salary register vs ECR vs ESIC vs salary TDS | `salary_row`, `ecr_row`, `esic_row` | findings |

Worklist classifications (capital/revenue, audit clause, counterparty GST status, TDS section) are **user-confirmed data**. Once confirmed they are stored as facts with `source = user` and a locator pointing at the worklist decision, so later rules (for example "TDS deducted for heads classified as 194X") can read them. An optional local model may only *pre-fill a suggestion* in this worklist; it never touches findings (CLAUDE.md invariant 2). It is not part of Tool 1's first release.

## B9. Rule governance and Rule Builder backend (RU-10..44)

- **Form → YAML:** the Rule Builder posts a structured JSON form; `rules.Generate()` produces canonical YAML (stable key order, quoted text fields), runs the full strict loader on it, and saves it as a new `rule_versions` row. The CA never types YAML (RU-21). Free text goes into `rule_text_en`/`notes` only (RU-23).
- **Dropdowns (RU-22):** `GET /api/v1/vocabulary?readable=true` returns only facts that at least one installed profile can produce, grouped by document.
- **Built-in edit (RU-12):** editing `RECON-DEP-002 v1` creates `RECON-DEP-002 v1-office.1`, `source: office`. "Reset to standard rule" sets `rule_state.active_version` back to the built-in version. Nothing is deleted (RU-41).
- **Starlark rules (RU-24):** the form exposes only `params`; the script body is read-only.
- **Lifecycle (RU-40):** `draft → testing` (requires at least one rule-test run) `→ active` (Partner only) and any `→ disabled` (one click, reason required). Office rules cannot become active without a passing rule-test run (RU-43).
- **Audit (RU-42):** every transition writes an audit entry with the reason.

## B10. Content packs (ENV-05, RU-13, NF-06)

A content pack is a `.reconpack` file (zip) carried by USB.

```
manifest.json      pack_id, version, kind ("rules" | "profiles" | "lawtables" | "vocabulary" | "bundle"),
                   tax_years, min_engine_version, files[{path, sha256}], tests_passed_at_build
manifest.sig       Ed25519 signature over manifest.json
content/…          rule.yaml, *.star, profile.yaml, lawtable.yaml, facts.yaml, fixtures
CHANGELOG.md       human-readable changes, shown before install
```

Install flow (server, Partner only):

1. Verify the signature with a public key embedded in the binary (`crypto/ed25519`, offline). Check every file's SHA-256 against the manifest. Reject on any mismatch.
2. Check `min_engine_version` and vocabulary compatibility.
3. Load every item through the same strict loaders; run the pack's own fixtures in the engine. Any failure rejects the whole pack.
4. **Conflict check (RU-13):** for each rule the pack changes that has an office version, show both and let the CA choose per rule. Office rules are never overwritten.
5. Show the changelog; on confirm, install in one transaction; record `previous_pack_id` for rollback.
6. **Rollback** re-activates the previous versions recorded by the pack install. Nothing is deleted.

Binary updates use the same signature scheme and are applied by `recon-updater`, which keeps the previous binaries for rollback.

The pack signing key lives offline on the vendor side; `recon-dev pack sign` is never shipped to firms.

## B11. API surface (server)

JSON over HTTPS, prefix `/api/v1`, session cookie (HttpOnly, Secure, SameSite=Strict) plus CSRF token. Every handler declares its required permission; a test asserts that no handler is registered without one.

| Area | Endpoints (abridged) |
|---|---|
| Auth | `POST /login`, `POST /logout`, `GET /me` |
| Season board | `GET /engagements?tax_year=` (status, open findings by severity, oldest open, cannot-check count) |
| Clients | `GET/POST /clients`, `GET/POST /clients/{id}/engagements` |
| Files | `POST /engagements/{id}/uploads` (multipart), `GET /engagements/{id}/documents`, `GET /document-versions/{id}/validation` |
| Runs | `POST /engagements/{id}/runs`, `GET /jobs/{id}` (progress), `GET /runs/{id}` |
| Findings | `GET /engagements/{id}/findings?group=review_document`, `GET /findings/{sid}`, `POST /findings/{sid}/decision`, `GET /findings/{sid}/history` |
| Worklist | `GET /engagements/{id}/worklist`, `POST /worklist/{id}/classify` |
| Drafts | `GET /documents/{id}/diff?from=2&to=3` |
| Rules | `GET /rules`, `GET /rules/{id}`, `POST /rules` (form), `POST /rules/{id}/versions`, `POST /rules/{id}/test`, `POST /rules/{id}/transition`, `POST /rules/{id}/reset` |
| Law tables | `GET /lawtables`, `POST /lawtables/{t}/rows`, `POST /lawtables/{t}/rows/{id}/transition` |
| Formats | `GET /formats` (Format Register), `GET /vocabulary` |
| Packs | `POST /packs` (upload + verify, returns conflicts + changelog), `POST /packs/{id}/install`, `POST /packs/{id}/rollback` |
| Export | `POST /engagements/{id}/exports` (`xlsx` / `pdf` / `query_letter`) |
| Admin | users, backup now, backup status, audit-log viewer and chain verification |

Progress updates use polling of `/jobs/{id}` in Tool 1 (simple, works through any proxy). Server-sent events can replace it later without changing the engine.

## B12. Security (NF-04..09)

### B12.1 Role matrix (requirements §3)

| Permission | Partner | Manager | Staff | Read-only |
|---|:-:|:-:|:-:|:-:|
| View clients, findings, worklist | ✓ | ✓ | ✓ | ✓ |
| Upload files, run reconciliation | ✓ | ✓ | ✓ | |
| Decide findings, classify worklist | ✓ | ✓ | ✓ | |
| Delete findings | ✓ | ✓ | | |
| Create/edit draft rules, run rule tests | ✓ | ✓ | ✓ | |
| Activate rules, approve law-table rows | ✓ | | | |
| Disable a rule | ✓ | | | |
| Export | ✓ | ✓ | | |
| Install / roll back packs, manage users | ✓ | | | |

*Disable is Partner-only here because RU-40 puts approval with the Partner; confirm whether Managers may also disable (open question T-Q3).*

### B12.2 Controls

| Control | Implementation |
|---|---|
| Encryption at rest | SQLCipher (AES-256) for `recon.db`; AES-256-GCM for vault files. One random 256-bit master key, sealed with Windows DPAPI (machine scope) so the service restarts unattended, plus a **recovery code** printed once at install for restore on new hardware |
| Key to engine | stdin at start-up only (§A3.1) |
| In transit | HTTPS on the LAN. The installer creates a firm-local CA and server certificate. Staff browsers either import the CA once (configuration, not software) or accept the warning — see T-Q2 |
| Sessions | Idle timeout (default 30 min), absolute timeout (10 h), argon2id password hashes, lockout after repeated failures |
| Audit | Hash chain: `hash = sha256(prev_hash ‖ canonical(row))`. Covers logins, views of a client file, decisions, exports, rule/law/pack changes. Chain verification available in Admin and run daily |
| Exports | Watermark: firm name, user, timestamp, export id; every export logged |
| Backup | Daily encrypted copy of `recon.db` (via SQLite online backup API) and vault to a configured path; keep N days; a monthly restore test into a scratch directory, reported in Admin (NF-07) |
| No network in engine | archtest + build artefact (§A4.1) |

## B13. Exports (FD-08)

- **Excel:** one sheet per review document with findings, values, difference, locators, law reference, state; a sheet for cannot-check items; a sheet for the worklist. Written with `excelize`.
- **PDF:** same content, watermarked. The PDF library is chosen in Phase 4 and must pass the server-side licence review (the server may use network-capable packages, the engine may not).
- **Client query letter:** fixed template listing items in `decided_ask_client`, worded as questions with the computed values; no "should" wording (same banned-word test).

## B14. Testing strategy

| Test | What it proves | Where |
|---|---|---|
| Rule golden fixtures | Each rule has ≥ 1 passing and ≥ 1 failing fixture at fact level, with expected evaluations (RU-43) | `content/rules/**/fixtures`, run by `go test ./internal/engine/...` |
| Profile tests | Extraction equals hand-verified facts for every sample (IN-22) | `content/profiles/**`, `recon-dev profile-test` |
| End-to-end packs | Synthetic filing packs reproduce R1–R10 with **zero** false positives, and the clean pack produces **zero** findings | `testdata/packs/` |
| Determinism | Two runs, shuffled upload order, different `GOMAXPROCS` → byte-identical canonical output | CI |
| Incremental ≡ full | Incremental re-run equals a full run on the same inputs | CI |
| Money properties | Parse/format round-trip, no overflow wrap, rounding modes, Indian grouping | `internal/money` property tests |
| Wording | No banned words in any rendered finding or title | `internal/findings` |
| Architecture | Engine has no `net`; import rules of §A4.1 hold; every API handler has a permission | `internal/archtest` |
| Strict YAML | Unquoted tax year, unknown key, anchor, float literal in Starlark → rejected | `internal/rules`, `internal/starhost` |
| Lifecycle | Every transition in §B7.3 | `internal/findings` |
| Performance | 1 client with 200k vouchers < 3 min; 50 clients < 30 min; one-file re-run < 20 s | benchmark job, from Phase 2 |

No test, fixture or sample may contain real client data (A1).

## B15. Build, packaging and developer commands

| Task | Command |
|---|---|
| Build all binaries | `go build ./cmd/...` |
| Build UI | `cd web && npm ci && npm run build` (output embedded by `recon-server`) |
| All tests | `go test ./...` |
| One package / one test | `go test ./internal/engine/ -run TestRuleFixtures/RECON-DEP-002` |
| Engine import-graph proof | `go test ./internal/archtest/ -run TestEngineHasNoNetwork -v` |
| Profile tests | `go run ./cmd/recon-dev profile-test ./content/profiles/...` |
| Rule lint | `go run ./cmd/recon-dev rule-lint ./content/rules/...` |
| Phase 1 CLI run | `go run ./cmd/recon-engine run --pack testdata/packs/ref-01 --rules content/rules/2025-26 --out report.txt` |
| Synthetic data | `go run ./cmd/recon-dev synth --vouchers 200000 --out testdata/large` |

Packaging: Windows installer (registers `recon-server` as a Windows service, creates data directory, seals master key, creates certificate) and a portable zip. SQLCipher requires cgo; Windows builds are produced on a Windows CI runner or with a MinGW cross-toolchain (risk T2). These commands become the "Commands" section of `CLAUDE.md` when the code is scaffolded.

## B16. Build order — what each phase builds from this structure

The phases are those of requirements §13. Each phase builds only what it needs, but inside the final package layout so nothing is rewritten later.

| Phase | Builds | Explicitly not yet |
|---|---|---|
| **1** | `money`, `period`, `domain`, `vocab` (tiny), `ingest` + `adapters/json`, `profile` (JSON selectors), `rules` strict loader, `engine` with `equal` check, `findings` wording templates, `store` with plain SQLite, `archtest`, `cmd/recon-engine run` CLI. One rule: RECON-DEP-002 end to end; clean file → zero findings | server, UI, encryption, lifecycle persistence, packs |
| **2** | Generic tax-suite JSON profiles (→ real ones when samples arrive, Q1); check types `sum_equals`, `gte`, `field_present`, `domain`, `formula` (+ `starhost`), `co_movement`, `draft_regression`; `drafts` module; 80–120 rules; full fixture suite; determinism + benchmark in CI | Excel/Word |
| **3** | `adapters/xlsx`, `csv`, `docx`, `tallyxml`; records store; `newheads` module and worklist | live Tally pull |
| **4** | `recon-server`, `api`, `authz` (basic), `jobs`, IPC, lifecycle §B7 in DB, React season board / client file / worklist; SQLite contention load test (§A3.2) | Rule Builder |
| **4b** | Rule Builder backend §B9, law tables §B5 + screen, Format Register | |
| **5** | `gst`, `tds`, `payroll` modules; `match_records`, `threshold`, `within_due_date`, `all_periods_present`; portal profiles; `adapters/pdftext`, `fixedwidth` | |
| **6** | SQLCipher, vault encryption, DPAPI key sealing, TLS, full role matrix, audit hash chain, backups, installer, exports; licence check per Q4 | |
| **7** | `pack` (sign/verify/install/rollback/conflicts), `recon-updater` | |
| **8** | Voice — deferred | |

## B17. Technical risks and open decisions

| # | Risk / decision | Mitigation / proposal |
|---|---|---|
| T1 | A third-party library on the engine side (excelize, SQLCipher driver, docx reader, Starlark, YAML) may import `net` transitively | archtest runs on every PR. If a library fails, either pick another, vendor a trimmed copy, or move that extraction to a separate no-DB helper process. Check excelize first, in Phase 3, before relying on it |
| T2 | SQLCipher needs cgo, complicating Windows builds | Windows CI runner from Phase 6; Phase 1–5 use plain SQLite behind the same `store` interface |
| T3 | SQLite write contention between two processes at 50 clients | Single-writer goroutine per process, batched transactions, Phase 4 load test, documented fallback (§A3.2) |
| T4 | PDF column layout misread → confident false findings | PDF is fallback only; region-based profiles with declared column ranges; dense-column samples required in profile tests |
| T5 | Starlark float support cannot be switched off in current `go.starlark.net` | Static rejection of float literals and `/`, runtime rejection of float results (§B4.5) |
| T6 | Finding states and transitions (§B7) are a proposal | CA sign-off needed, including the proposed `withdrawn` state |
| T7 | Scanned PDFs, image pages and photos could be read by OCR, giving guessed figures | No OCR in Tool 1 (A5, IN-05), enforced by archtest `TestNoOCR`. Images refused at upload; fully scanned PDFs rejected; image pages of mixed PDFs listed as not read, and figures on them become Cannot check (SPEC-044). Open S-Q1: scans carrying a scanner-made invisible OCR text layer are proposed to be treated as image pages |
| T-Q1 | "Review document" grouping requires choosing which document a finding is reviewed under | Declared per rule by the CA as a display convention; never shown as "this document is wrong" |
| T-Q2 | HTTPS on the LAN: import a firm CA on staff PCs, or accept a browser warning? | Recommend importing the CA once per PC; needs the firm's agreement since ENV-02 says nothing installed on staff PCs |
| T-Q3 | May Managers disable rules? | Requirements give disable "one click" without naming the role; this spec assumes Partner only |
| T-Q4 | CLAUDE.md (from spec v0.2) gates Phase 1 on "Phase 0 validation"; requirements v0.3 has no Phase 0 | Confirm whether Phase 0 still applies before Phase 1 starts |

## B18. Traceability

| Requirement | Where addressed |
|---|---|
| ENV-01..07 | A3, B10, B12 |
| IN-01..06 | A6.1, B2.2 |
| IN-10..14 | A7, B2.1 |
| IN-20..26 | A2, A6.2, B2.2–B2.4 |
| FV-01..04 | A2, A5, B3 |
| RU-01..04 | B4.1, B4.2, B4.5 |
| RU-10..13 | B9, B10 |
| RU-20..25 | B6 (test mode), B9 |
| RU-30..34 | B5 |
| RU-40..44 | B9, B14 |
| RU-50..53 | A7, A8, B4.4, B6 |
| RC-01..06 | A6.4, B4.3, B8 |
| FD-01..08 | B4.6, B7, B11, B13 |
| SC-01..08 | A1, A6, A9 |
| NF-01..09 | A8, B1.3, B12, B14 |
