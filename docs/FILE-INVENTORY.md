# Tool 1 — File inventory

This is the single list of every file in the application: backend, frontend, database, content (rules, profiles, law tables), configuration, build, testing and documentation. For each file it records the location, the purpose and what it contributes to the application.

## How to read and maintain this document

| Status | Meaning |
|---|---|
| **Exists** | The file is in the repository now |
| **Planned** | A spec says this file will be built; the spec ID is shown. The final file name may differ slightly; correct it here when the file is created |
| **Generated** | Produced by a tool or build step; never edited by hand |
| **Local only** | Lives on a machine but must never be committed |

**Rule:** any change that adds, renames, moves or deletes a file updates this inventory in the same commit. When a spec is marked Done, every row with its ID changes from Planned to Exists. Repeating files (one per rule, one per profile, one per migration) are described once, by pattern, in their section rather than listed one by one.

Paths are relative to the repository root.

---

## 1. Documentation and specifications

| File | Location | Purpose | Contribution to the application | Status |
|---|---|---|---|---|
| `CLAUDE.md` | `/` | Guidance for Claude Code: current state, non-negotiable rules, architecture summary, commands | Keeps every AI-assisted change inside the project's rules (paise money, determinism, no network in the engine) | Exists |
| `recon-tool-spec.md` | `/` | Product and engineering spec v0.2 | Original source of design decisions: stack, rule categories R1–R10, security posture | Exists |
| `requirements-spec-tool1.md` | `/` | Requirements v0.3 for Tool 1 | The requirement IDs (ENV, IN, FV, RU, RC, FD, SC, NF) every spec traces back to | Exists |
| `tool1-technical-spec.md` | `/` | Technical spec: Part A architecture, Part B implementation | The design every piece of code must follow | Exists |
| `Recon-Tool-Understanding-and-Gaps.docx` | `/` | Plain-language summary of the spec plus open gaps | Input for decisions on finding states, reappearing defects, off-site Tally books | Exists |
| `~$con-Tool-Understanding-and-Gaps.docx` | `/` | Microsoft Word lock file, created while the .docx is open | None; it should be deleted and excluded by `.gitignore` (SPEC-001) | Local only |
| `README.md` | `specs/` | Index of all build specs with dependencies and status | The progress tracker for the whole build | Exists |
| `TEMPLATE.md` | `specs/` | Template for a new spec | Keeps every spec in the same checkable format | Exists |
| `SPEC-001-*.md` … `SPEC-043-*.md` | `specs/` | One build unit each: scope, acceptance criteria, verification, completion record | Defines what "done" means for each part of the application | Exists (43 files) |
| `FILE-INVENTORY.md` | `docs/` | This document | Central list of every file and its role | Exists |
| `fact-vocabulary.md` | `docs/` | Human-readable list of every fact and record name | Lets the CA and developers see which figures the tool knows | Generated (SPEC-004) |
| `fixtures.md` | `docs/` | Description of the synthetic generic formats used until real samples arrive | Allows development and testing with no real client data | Planned (SPEC-006) |
| `export-guide.md` | `docs/` | Step-by-step guide for staff exporting from Tally, the tax suite and portals | Makes uploaded files consistent so profiles recognise them | Planned (SPEC-020) |
| `security-checklist.md` | `docs/` | Security checks to pass before release | The Phase 6 gate | Planned (SPEC-035) |
| `NNNN-*.md` | `docs/adr/` | Architecture decision records (driver choice, `.xls` decision, contention results…) | Records why each technical choice was made | Planned (SPEC-010 onwards) |
| `CATALOGUE.md` | `content/rules/` | List of all built-in rules with law reference, check type and fixture counts | The document the firm's CA reviews and signs off | Generated (SPEC-016) |

## 2. Claude Code configuration

| File | Location | Purpose | Contribution to the application | Status |
|---|---|---|---|---|
| `code-reviewer.md` | `.claude/agents/` | Read-only senior code-reviewer subagent (Sonnet) | Reviews changes for quality, security and the project's non-negotiable rules | Exists |
| `test-engineer.md` | `.claude/agents/` | Senior QA/test-engineer subagent (Sonnet) | Writes and runs tests, checks spec acceptance criteria, reports root causes | Exists |

## 3. Backend — executables (`cmd/`)

| File | Location | Purpose | Contribution to the application | Status |
|---|---|---|---|---|
| `main.go` | `cmd/recon-engine/` | Engine entry point: CLI mode (`run`, `diff`, `worklist`) and daemon mode | The reconciliation engine itself; contains no network code | Planned (SPEC-001, SPEC-011, SPEC-023) |
| `main.go` | `cmd/recon-server/` | Server entry point: loads config, opens DB, starts HTTP server and engine | What staff connect to from their browsers | Planned (SPEC-001, SPEC-022) |
| `embed.go` | `cmd/recon-server/` | `go:embed` of the built UI (`web/dist`) | Puts the whole UI inside the one server binary | Planned (SPEC-022) |
| `main.go` | `cmd/recon-updater/` | Applies a signed program update from USB and rolls back | Safe offline upgrades | Planned (SPEC-001, SPEC-042) |
| `main.go` | `cmd/recon-collector/` | Optional live pull of Tally reports on the LAN | Saves staff from manual Tally exports (optional) | Planned (SPEC-043) |
| `main.go` | `cmd/recon-dev/` | Developer tools: `profile-test`, `rule-lint`, `rule-catalogue`, `vocab-doc`, `synth`, `loadtest`, `pack build`, `pack sign` | Testing, content building and signing; never shipped to firms | Planned (SPEC-001 onwards) |

## 4. Backend — core libraries (`internal/`)

| File | Location | Purpose | Contribution to the application | Status |
|---|---|---|---|---|
| `doc.go` | every `internal/*/` package | States the package's single responsibility | Keeps package boundaries clear | Planned (SPEC-001) |
| `money.go` | `internal/money/` | `Paise` type and overflow-checked arithmetic | All money is exact integer paise; no rounding drift | Planned (SPEC-002) |
| `parse.go` | `internal/money/` | Parses amounts from text and JSON numbers without floats | Reads `1,12,450.50` exactly as 11245050 paise | Planned (SPEC-002) |
| `rate.go` | `internal/money/` | Basis-point rates and explicit rounding modes | Tax and rate calculations with declared rounding | Planned (SPEC-002) |
| `format.go` | `internal/money/` | Indian digit grouping | Shows `1,12,450` in findings and exports | Planned (SPEC-002) |
| `period.go` | `internal/period/` | Tax years, FY, quarters, months; FY arithmetic | Lines up monthly, quarterly and annual figures | Planned (SPEC-002) |
| `fact.go`, `record.go`, `locator.go` | `internal/domain/` | Fact, Record and Locator types | The shared shapes every package uses | Planned (SPEC-003) |
| `evaluation.go`, `finding.go`, `run.go` | `internal/domain/` | Evaluation, Finding, run snapshot types | What the engine produces and stores | Planned (SPEC-003) |
| `identity.go` | `internal/domain/` | Stable finding ID and value fingerprint | Re-runs update findings instead of duplicating them | Planned (SPEC-003) |
| `canonical.go` | `internal/domain/` | Canonical JSON encoding | Byte-identical output for determinism tests | Planned (SPEC-003) |
| `vocab.go` | `internal/vocab/` | Loads and validates the fact vocabulary | Profiles and rules can only use known figure names | Planned (SPEC-004) |
| `strictyaml.go` | `internal/rules/` (shared helper) | Strict YAML loading: unknown keys, unquoted text, anchors rejected | Stops YAML from silently misreading rules or profiles | Planned (SPEC-007) |
| `adapter.go`, `registry.go` | `internal/ingest/` | Adapter interface and registry | Lets new file types plug in without changing other code | Planned (SPEC-005) |
| `report.go` | `internal/ingest/` | Validation report per uploaded file | Tells staff what was recognised, read and not read | Planned (SPEC-005) |
| `json.go` | `internal/ingest/adapters/json/` | JSON reader with JSON-path selectors | Reads tax-suite JSON (the primary income-tax input) | Planned (SPEC-005) |
| `xlsx.go` | `internal/ingest/adapters/xlsx/` | Excel reader, columns by header text | Reads Tally Excel, GST downloads, staff schedules | Planned (SPEC-018) |
| `csv.go` | `internal/ingest/adapters/csv/` | CSV reader with declared delimiter and encoding | Reads CSV downloads and exports | Planned (SPEC-018) |
| `docx.go` | `internal/ingest/adapters/docx/` | Word table reader (tables only) | Reads figures from staff and client Word documents | Planned (SPEC-019) |
| `tallyxml.go` | `internal/ingest/adapters/tallyxml/` | Streaming Tally XML reader | Reads TB, ledgers, Day Book, registers | Planned (SPEC-020) |
| `pdftext.go` | `internal/ingest/adapters/pdftext/` | Runs bundled `pdftotext -layout`; region selectors | Reads text PDFs (fallback only) with columns preserved | Planned (SPEC-030) |
| `fixedwidth.go` | `internal/ingest/adapters/fixedwidth/` | Delimited and fixed-width text reader | Reads TRACES, EPFO and ESIC text files | Planned (SPEC-030) |
| `profile.go`, `detect.go`, `executor.go` | `internal/profile/` | Format-profile schema, detection, extraction | Turns any supported file into standard facts | Planned (SPEC-006) |
| `rule.go`, `loader.go`, `lint.go` | `internal/rules/` | Rule schema, strict loader, linter | Loads the CA-readable YAML rules safely | Planned (SPEC-007) |
| `generate.go` | `internal/rules/` | Builds canonical YAML from the Rule Builder form | CA creates rules without typing YAML | Planned (SPEC-027) |
| `lawtable.go`, `lookup.go` | `internal/lawtable/` | Law-table loader and as-of-date lookup | Law changes become dated rows, not code changes | Planned (SPEC-013) |
| `engine.go`, `planner.go`, `depindex.go` | `internal/engine/` | Rule selection, dependency index, parallel evaluation | Runs all rules deterministically; enables fast re-runs | Planned (SPEC-008, SPEC-017) |
| `checktype.go`, `evalctx.go` | `internal/engine/` | Check-type interface and evaluation context (no clock, no I/O) | Keeps every check pure and repeatable | Planned (SPEC-008) |
| `<check>.go` | `internal/engine/checks/<check>/` | One package per check type: `equal`, `sum_equals`, `gte`, `field_present`, `domain`, `formula`, `co_movement`, `draft_regression`, `match_records`, `threshold`, `within_due_date`, `all_periods_present` | The comparison shapes rules can use | Planned (SPEC-008, 012, 014, 015, 029) |
| `host.go`, `sandbox.go` | `internal/starhost/` | Starlark sandbox: no floats, step limits, `fact()`/`law()` | Complex calculations (e.g. MAT book profit) safely | Planned (SPEC-014) |
| `lifecycle.go` | `internal/findings/` | Finding states and automatic transitions after each run | Resolves or reopens findings when corrected files arrive | Planned (SPEC-025) |
| `templates/*.tmpl`, `render.go` | `internal/findings/` | Fixed wording templates and report rendering | Every finding shows both values, the difference and sources, never "should" | Planned (SPEC-009) |
| `module.go` | `internal/modules/` | Module interface | Lets new reconciliation areas plug in | Planned (SPEC-008) |
| `drafts.go` | `internal/modules/drafts/` | Draft history and draft-to-draft diff | Catches stale dependent figures and regressions (R3, R10) | Planned (SPEC-015) |
| `newheads.go` | `internal/modules/newheads/` | Year-on-year ledger comparison | Produces the new / vanished / broken-recurrence worklist | Planned (SPEC-021) |
| `gst.go` | `internal/modules/gst/` | GST aggregates and match indexes | Books vs GSTR-1/1A/3B/2B/9/IMS reconciliation | Planned (SPEC-031) |
| `tds.go` | `internal/modules/tds/` | TDS aggregates by section and quarter | Payable and receivable TDS reconciliation | Planned (SPEC-032) |
| `payroll.go` | `internal/modules/payroll/` | Monthly payroll aggregates | Salary vs EPF vs ESIC vs salary TDS reconciliation | Planned (SPEC-033) |
| `db.go`, `writer.go` | `internal/store/` | Opens the DB with pragmas; single writer goroutine | Safe, fast writes from both processes | Planned (SPEC-010) |
| `facts.go`, `records.go`, `runs.go`, `findings.go` … | `internal/store/` | Repositories with explicit SQL | All persistence; swap to SQLCipher without touching callers | Planned (SPEC-010 onwards) |
| `vault.go` | `internal/vault/` | Content-addressed storage of uploaded originals (encrypted from SPEC-034) | Keeps every uploaded file, deduplicated | Planned (SPEC-024, SPEC-034) |
| `queue.go`, `pool.go` | `internal/jobs/` | DB-backed job queue and worker pool | 50 clients reconciled concurrently; survives restarts | Planned (SPEC-023) |
| `messages.go` | `internal/ipc/` | Server ↔ engine JSON-lines messages | Lets the server drive the network-free engine | Planned (SPEC-023) |
| `keys.go` | `internal/keys/` | Master key, DPAPI sealing, recovery code | Encryption at rest with unattended restart | Planned (SPEC-034) |
| `certs.go` | `internal/certs/` | Firm-local CA and server certificate | HTTPS on the office network | Planned (SPEC-035) |
| `authz.go`, `matrix.go` | `internal/authz/` | Roles and permission matrix | Partner / Manager / Staff / Read-only enforcement | Planned (SPEC-022, SPEC-035) |
| `router.go`, `*_handlers.go` | `internal/api/` | HTTP API handlers | Everything the UI does goes through here | Planned (SPEC-022 onwards) |
| `audit.go`, `verify.go` | `internal/audit/` | Hash-chained audit log and chain verification | Tamper-evident record of every view, decision and change | Planned (SPEC-036) |
| `backup.go`, `restore.go` | `internal/backup/` | Daily encrypted backup, restore, restore test | Protects the firm's data | Planned (SPEC-037) |
| `excel.go`, `pdf.go`, `queryletter.go` | `internal/export/` | Exporters | Excel, watermarked PDF and client query letter | Planned (SPEC-038) |
| `dataprotection.go` | `internal/dataprotection/` | Client export, deletion, retention, access report | Supports the firm's data-protection obligations | Planned (SPEC-039) |
| `pack.go`, `verify.go`, `install.go` | `internal/pack/` | Content-pack format, Ed25519 verification, install, rollback | Rule and format updates by USB, safely | Planned (SPEC-041) |
| `collector.go` | `internal/collector/` | Tally request and file-drop logic | Optional live Tally pull | Planned (SPEC-043) |

## 5. Database

| File | Location | Purpose | Contribution to the application | Status |
|---|---|---|---|---|
| `NNNN_<name>.sql` | `internal/store/migrations/` | Numbered schema migrations, embedded and applied at start-up. First: `0001_core.sql` (clients, engagements, files, documents, document_versions, facts, runs, evaluations). Later specs add records, law tables, worklist, users/sessions, jobs, findings, rule versions, profiles, packs, audit_log | Defines and evolves the database schema | Planned (SPEC-010 onwards) |
| `recon.db` (+ `-wal`, `-shm`) | data directory on the main desktop | The SQLite/SQLCipher database | Holds all facts, findings, users and history | Local only |
| `vault/ab/cd/<sha256>.bin` | data directory | Stored original uploaded files | Every uploaded file, deduplicated and (from SPEC-034) encrypted | Local only |
| `master.key.sealed` | data directory | DPAPI-sealed master key | Unlocks the DB and vault on this machine | Local only (SPEC-034) |

## 6. Content (data shipped in signed packs)

| File | Location | Purpose | Contribution to the application | Status |
|---|---|---|---|---|
| `facts.yaml` | `content/vocabulary/` | Controlled list of fact names | The bridge between formats and rules | Planned (SPEC-004) |
| `records.yaml` | `content/vocabulary/` | Record types (invoice, voucher, challan, ECR row…) and their keys | Enables row-level matching for GST, TDS and payroll | Planned (SPEC-004, SPEC-020) |
| `profile.yaml` | `content/profiles/<source>/<document>/<version>/` | One format profile per market format and version | Teaches the tool where each figure sits in that format | Planned (SPEC-006 onwards) |
| `samples/*`, `expected/*.facts.yaml` | same folder as each profile | Sample files and hand-verified expected facts | Proves each profile extracts exactly the right figures | Planned (SPEC-006 onwards) |
| `rule.yaml` | `content/rules/<tax-year>/<module>/<RULE-ID>/` | One rule (e.g. `RECON-DEP-002`) | A single check the CA can read and approve | Planned (SPEC-011 onwards) |
| `*.star` | same folder as the rule | Starlark script for a formula rule | Complex calculations such as MAT book profit | Planned (SPEC-014) |
| `fixtures/pass/*.yaml`, `fixtures/fail/*.yaml` | same folder as the rule | Input facts and expected results | Proves each rule fires when it should and only then | Planned (SPEC-007 onwards) |
| `<table>.yaml` | `content/lawtables/` | Dated law values: TDS thresholds and rates, GST rates, due dates, 1961↔2025 section map, regime by tax year | Law changes without code changes | Planned (SPEC-013) |

## 7. Frontend (`web/`)

| File | Location | Purpose | Contribution to the application | Status |
|---|---|---|---|---|
| `package.json`, `package-lock.json` | `web/` | UI dependencies and scripts | Reproducible UI builds | Planned (SPEC-026) |
| `vite.config.ts` (or chosen bundler config), `tsconfig.json` | `web/` | Build and TypeScript settings | Produces the bundle embedded in the server | Planned (SPEC-026) |
| `index.html`, `main.tsx`, `App.tsx` | `web/`, `web/src/` | UI shell and routing | Entry point of the browser application | Planned (SPEC-026) |
| `api.ts` | `web/src/lib/` | Typed client for `/api/v1` | All UI calls to the server | Planned (SPEC-026) |
| `SeasonBoard.tsx` | `web/src/screens/` | Season board: one row per client with status and finding counts | The partner's overview of the season | Planned (SPEC-026) |
| `ClientFile.tsx` | `web/src/screens/` | Uploads, findings grouped by document to fix, decisions, drafts, history | Where staff do the reconciliation work | Planned (SPEC-026) |
| `Worklist.tsx` | `web/src/screens/` | New / vanished / broken-recurrence heads and classification | Completeness work, separate from findings | Planned (SPEC-026) |
| `RuleBuilder.tsx` | `web/src/screens/` | Form-based rule creation, testing, approval | CA manages rules without YAML | Planned (SPEC-027) |
| `LawTables.tsx` | `web/src/screens/` | View, add and approve dated law values | CA maintains rates, thresholds, due dates | Planned (SPEC-027) |
| `FormatRegister.tsx` | `web/src/screens/` | Supported formats and readable figures | Shows what the tool can read | Planned (SPEC-028) |
| `Admin.tsx` (+ sub-screens) | `web/src/screens/` | Users, backup, audit log, packs, data protection | Firm administration | Planned (SPEC-022, 036, 037, 039, 041) |
| `FindingCard.tsx` | `web/src/components/` | One finding with values, difference, arithmetic, sources, law reference | The core display rule of the product | Planned (SPEC-026) |
| `shortcuts.ts` | `web/src/lib/` | Keyboard shortcut map and `?` overlay | Keyboard-first operation | Planned (SPEC-026) |
| `en.json`, `hi.json` | `web/src/i18n/` | English and Hindi UI labels | Hindi/English label toggle | Planned (SPEC-026) |
| `web/dist/*` | `web/dist/` | Built UI bundle | Embedded into `recon-server` | Generated (not committed) |

## 8. Configuration, build and packaging

| File | Location | Purpose | Contribution to the application | Status |
|---|---|---|---|---|
| `go.mod`, `go.sum` | `/` | Go module and pinned dependency versions | Reproducible backend builds | Planned (SPEC-001) |
| `.gitignore` | `/` | Excludes DBs, vault, build output, `node_modules`, Word lock files, private samples | Keeps client data and build junk out of git | Planned (SPEC-001) |
| CI workflow (e.g. `.github/workflows/ci.yml`) | CI folder | Runs gofmt, vet, tests, archtest, benchmarks | Automatic quality gate on every change | Planned (SPEC-001, SPEC-017) |
| `recon.toml` (example: `recon.example.toml`) | `/` (example); data directory (real) | Server config: bind address, port, workers, timeouts, backup path | Per-office settings | Planned (SPEC-022) |
| `dev.toml` | `/` | Developer config for running the server locally | Local development | Planned (SPEC-022) |
| `pdftotext.exe` + licence | `third_party/poppler/` | Vendored Poppler PDF text extractor | Layout-preserving PDF reading | Planned (SPEC-030) |
| installer script and assets | `installer/` | Windows installer and portable-zip setup | Zero-install-prerequisite deployment | Planned (SPEC-040) |
| `TESTING.md` | `installer/` | Manual install test steps on clean VMs | Proves the installer works for a non-IT office | Planned (SPEC-040) |
| `engine-deps.txt` | `build/` | Full dependency list of `recon-engine` | Proof that the engine contains no network code | Generated (SPEC-001) |
| `*.reconpack` | release output | Signed content packs | Rule/profile/law updates delivered by USB | Generated (SPEC-041) |

## 9. Testing

| File | Location | Purpose | Contribution to the application | Status |
|---|---|---|---|---|
| `*_test.go` | beside the code in every package | Unit and table-driven tests | Proves each package behaves as its spec says | Planned (every spec) |
| `archtest_test.go` | `internal/archtest/` | No-network check on the engine; import-direction rules; no floats, clocks or randomness in engine packages | Enforces the non-negotiable rules automatically | Planned (SPEC-001) |
| `packs_test.go` | `testdata/` | Runs every synthetic filing pack end to end | Proves R1–R10 are caught with zero false alarms | Planned (SPEC-016) |
| `ref-01/`, `clean-01/` | `testdata/packs/` | Phase 1 synthetic pack with the depreciation mismatch, and a clean pack | The Phase 1 gate | Planned (SPEC-011) |
| `r01/` … `r10/`, `ref-full/`, `clean-full/` | `testdata/packs/` | One synthetic pack per defect category, plus combined and clean packs | Phase 2 gate; regression protection | Planned (SPEC-016) |
| GST / TDS / payroll packs | `testdata/packs/` | Synthetic statutory packs with planted gaps | Phase 5 gate | Planned (SPEC-031–033) |
| `*.txt`, `*.json` | `testdata/golden/` | Expected rendered output (e.g. `RECON-DEP-002.txt`) | Locks finding wording and format | Planned (SPEC-009) |
| `testdata/large/` | `testdata/` | Synthetic full-volume clients from `recon-dev synth` | Performance and scale tests | Generated (SPEC-017) |
| UI tests (`*.test.tsx`) | `web/src/` | Component and screen tests, missing-translation check | Proves the UI shows arithmetic and enforces roles | Planned (SPEC-026) |

## 10. Files that must never be committed

| Pattern | Why |
|---|---|
| `*.db`, `*.db-wal`, `*.db-shm`, `data/`, `vault/` | Contain client data |
| `samples-private/` | Real anonymised samples received from the firm, kept off git |
| `~$*.docx`, `~$*.xlsx` | Microsoft Office lock files |
| `build/` (except published release artefacts), `web/dist/`, `node_modules/` | Build output |
| Any signing private key | Must stay offline on the vendor side (SPEC-041) |
