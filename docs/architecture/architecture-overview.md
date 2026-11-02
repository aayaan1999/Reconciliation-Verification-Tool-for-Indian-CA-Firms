# Architecture overview

This page describes the processes, the pipeline, the package layout, the domain model, the extension points, data storage and the architecture guard, and marks what is built and what is planned. It summarises the technical spec Part A and §B1, §B6, §B8 and §B16 ([tool1-technical-spec.md](../../tool1-technical-spec.md)), the product spec v0.2 §5 and §6 ([recon-tool-spec.md](../../recon-tool-spec.md)), requirements v0.3 §11 and §12 ([requirements-spec-tool1.md](../../requirements-spec-tool1.md)), the `doc.go` of each package under `internal/`, and the ADRs in [decisions/](decisions/). For actors and external systems see [system-context.md](system-context.md); for the constraints behind each choice see [constraints-and-assumptions.md](constraints-and-assumptions.md).

## Stack

Go, one static binary per program, cgo disabled. SQLite through the pure-Go `ncruces/go-sqlite3` driver, explicit SQL, no ORM. React + TypeScript UI built with Vite and embedded with `go:embed` in builds with `-tags ui`. `excelize` for Excel (linked, adapter planned). Bundled Poppler 26.09.0 `pdftotext -layout` for text PDFs, run as a separate program. Rules in strict YAML, with Starlark planned as the escape hatch (tech spec §A4, requirements §11).

## Processes (tech spec §A3)

| Process | Responsibility | Network | Status |
|---|---|---|---|
| `recon-engine` | Format detection, extraction, fact and record writes, rule evaluation, decisions and worklist classifications; also the command line | **None** (project invariant 5) | Built (SPEC-003 to SPEC-006): `load-rules`, `run`, `facts`, `rules`, `client-add`, `client-update`, `client-get`, `clients`, `result`, `comparisons`, `inspect`, `profile-create`, `decide`, `worklist`, `classify` |
| `recon-server` | Serves the embedded UI and the API; runs `recon-engine` for each request; never evaluates a rule | Listens on `127.0.0.1:8080` by default; office LAN with `--listen` | Test API built (SPEC-004 part 6 onwards). Logins, job queue, pack install, exports, backups planned |
| `recon-dev` | Developer tools: `rulestore-import`, `rulestore-check`, `rulestore-problems`, `rule-lint`, `profile-test`, `vocab-doc`. Never shipped to firms | — | Built |
| `recon-updater` | Verifies and swaps binaries from a USB update, keeps the previous version for rollback | None | Placeholder (SPEC-042) |
| `recon-collector` | Optional Tally pull on the LAN; writes files into the upload inbox | LAN to Tally only | Not built (SPEC-043) |

Planned server-engine link: one long-lived engine child, DB key as the first line on stdin, JSON-lines control messages; bulk data goes through the database (tech spec §A3.1, §A3.2, `internal/ipc` empty). Today the server starts a fresh engine process per request and the engine is the database's single writer.

## Pipeline (tech spec §A7)

```
files ──► ingest adapter ──► format profile ──► facts + records ──► rule engine ──► evaluations ──► findings ──► UI
          (syntax only)      (detect: exactly       (SQLite, with      (sorted rules,    (Pass / Fail /    (stable ID,
                              one; extract)          locators)          pure checks)      Cannot check)     decisions)
                                                          │
                                                          └──► newheads module ──► Worklist (never findings)
```

`recon-engine run` detects and extracts every input before writing anything, then stores drafts, takes the engagement's current view (latest draft per document), snapshots the rule and document versions used, evaluates and renders, in one transaction. Exit codes: 0 no findings, 1 findings, 2 input or rule error, 3 internal (tech spec §B6). Incremental runs and the worker pool are planned (§A7 step 5, §A9).

## The four contracts (tech spec §A2)

The fact vocabulary (`content/vocabulary/`), format profiles (`content/profiles/`), rules (`content/rules/<tax-year>/` plus the descriptive rule store in `docs/Recon_Tool_YAML_Rule_Store/recon_yaml/`) and law tables (planned) are data, versioned with the code. A profile never names a rule and a rule never names a format; the vocabulary is the only shared contract (FV-04).

## Package layout of `internal/`

Responsibilities from each package's `doc.go`. "Built" means the package holds working code today.

| Package | Responsibility | Status |
|---|---|---|
| `money` | Every amount as whole paise (`Paise int64`); overflow reported, never wrapped | Built |
| `period` | Tax years, FY periods, quarters, months; FY starts 1 April; no clock | Built |
| `domain` | Pure types: Fact, Record, Locator, Evaluation, Finding, run snapshots; no I/O | Built |
| `strictyaml` | Strict YAML decoding: quoted text, no unknown or duplicate keys, no anchors | Built |
| `vocab` | Loads and validates the fact vocabulary | Built |
| `ingest` | Chooses the one adapter for a file; builds the validation report | Built |
| `ingest/adapters/json`, `csv` | JSON with a fixed JSON-path subset, numbers as exact text; CSV as the same tree | Built |
| `ingest/adapters/xlsx`, `docx`, `tallyxml`, `pdftext`, `fixedwidth` | Excel by header text; Word tables; Tally XML streaming; layout-preserving PDF; TRACES/EPFO/ESIC text | Planned (`xlsx` links excelize only, for the guard) |
| `profile` | Loads profiles, detects exactly one match, extracts facts and records; office profiles from the mapping screen | Built |
| `rules` | Strict rule loader, validation against vocabulary and check types | Built (YAML generation for the Rule Builder planned) |
| `rulestore` | Reads the descriptive YAML rule store (304 rules, never evaluated) | Built |
| `engine`, `engine/checks/*` | Selects rules, evaluates in sorted order; check types `equal`, `sum_equals`, `gte`, `percent_of`, `present_together`, `table_lookup`, `match_records` | Built. `threshold`, `within_due_date`, `all_periods_present`, `field_present`, `domain`, `co_movement`, `draft_regression`, `formula` planned |
| `findings` | Wording templates, stable IDs, lifecycle | Built (wording, IDs, decisions); full state machine planned (§B7) |
| `client` | Client details and identity-number checks (PAN, TAN, CIN, GSTIN check character) | Built |
| `comparison` | Reads the comparison types (`content/comparisons/comparisons.yaml`) | Built |
| `modules/newheads` | Two years of ledgers → new, vanished, broken-recurrence, step-change Worklist items | Built |
| `modules/tieout`, `drafts`, `gst`, `tds`, `payroll` | Reconciliation areas on the same store and engine | Planned (GST invoice matching today runs as the `match_records` check type) |
| `store` | The only package that talks to SQLite; explicit SQL; migrations `0001` to `0006` | Built |
| `api` | HTTP handlers of `recon-server` (server side only) | Built (test API) |
| `lawtable`, `starhost`, `vault`, `jobs`, `ipc`, `pack`, `audit`, `authz`, `export` | Dated law values; Starlark sandbox; encrypted file vault; DB-backed queue; server-engine messages; signed packs; hash-chained audit log; roles; exports | Planned (`doc.go` only) |
| `archtest` | The architecture guard (below) | Built |

## Domain model (spec v0.2 §6.1, tech spec §A5)

- **Fact**: one scalar figure: vocabulary name, period, scope (GSTIN, branch), typed value (money in paise, int, text, date, bool, rate in basis points) and a **locator** (file plus JSON path, sheet cell, table cell or PDF position).
- **Record**: one row of a tabular source (invoice, ledger month, challan) with a match key, named amounts and fields, and a locator. Row-level matching needs records.
- **Edge**: a rule; an assertion that facts are equal or satisfy a relation (check type plus tolerance).
- **Evaluation**: the outcome of one rule for one subject: Pass, Fail or Cannot check, with every value, its locator and the computed difference.
- **Finding**: a failed evaluation with a stable ID (hash of rule, engagement, period, scope, match key; no version or run) and a value fingerprint, so re-runs update it and decisions survive unchanged values (FD-01, FD-04).

Evaluations and findings have a canonical JSON encoding, compared byte for byte in golden files and determinism tests.

## Extension points (tech spec §A6)

| Grows by | Extension | Needs a new binary? |
|---|---|---|
| New market format or version | Format profile (data), or taught on the mapping screen | No |
| New check | Rule (data) | No |
| New tax year or law change | Tax-year-tagged rules, dated law-table rows | No |
| New figure | Vocabulary entry plus profile plus rule | No |
| New comparison a CA picks | A few lines in `comparisons.yaml` | No |
| New file type | Adapter (code) | Yes, rare |
| New comparison shape | Check type (code), or Starlark `formula` once built | Check type: yes |
| New reconciliation area | Module (code, same store and engine) | Yes |
| New export format | Exporter (server side) | Yes |

Registries are iterated in sorted key order.

## Data storage

- One SQLite file (`recon.db`, `build/recon.db` in development), WAL mode, foreign keys on, explicit SQL, numbered migrations in `internal/store/migrations` (tech spec §B1). Money columns are `INTEGER` paise.
- Tables built: rules and rule state, runs and run snapshots, documents and drafts, facts, records, evaluations, clients and GSTINs, engagements, `finding_decisions` and `worklist_classifications` (append-only, enforced by triggers).
- **Encryption at rest:** the driver's `xts` VFS with a 64-byte key (AES-256-XTS) is decided and proven by `internal/store/encryption_test.go` ([ADR-0002](decisions/ADR-0002-database-encryption.md)). The store still opens a **plain** file today; key handling, DPAPI sealing and the recovery code are planned (SPEC-034, tech spec §B12.2).
- Office profiles taught on the mapping screen are YAML files in an office profiles folder, each with its sample file beside it.
- Planned: content-addressed encrypted vault for original files (`internal/vault`), law tables, packs, audit log.

## Architecture guard (`internal/archtest`)

Runs with `go test -count=1 ./internal/archtest/ -v` and in CI, for windows/amd64 and linux/amd64 with cgo disabled ([ADR-0001](decisions/ADR-0001-engine-network-proof.md)).

- `TestEngineHasNoNetwork`: no network package in the engine's import graph, except reviewed parse-only edges in `allowedNetworkImports`. Writes `build/engine-deps-<goos>-<goarch>.txt`.
- `TestEngineBinaryHasNoNetworkCode`: builds the network-free programs and fails on any linked connection, socket or resolver code. Writes `build/engine-symbols-<goos>-<goarch>.txt`.
- `TestEngineSideAvoidsServerPackages`, `TestFoundationPackagesImportOnlyStandardLibrary`: the import rules of tech spec §A4.1.
- `TestDeterministicPackagesHaveNoClockOrRandomness`, `TestMoneyPackagesUseNoFloatingPoint`: determinism and integer money.
- `TestEveryPackageIsClassified`: every package is in exactly one of `engineSidePatterns`, `serverSidePatterns`, `otherPatterns`.
- `TestEveryLinkedModuleHasALicenceRecord`: every linked module is in [docs/licences.md](../licences.md).

## Decisions

- [ADR-0001: how the engine proves it has no network code; SQLite driver and excelize](decisions/ADR-0001-engine-network-proof.md)
- [ADR-0002: database encryption with AES-256-XTS instead of SQLCipher](decisions/ADR-0002-database-encryption.md)
- [ADR-0003: bundle Poppler pdftotext (GPL)](decisions/ADR-0003-bundle-poppler.md)
