# Tool 1 — Build specifications

Each file in this folder is one **build unit**: a piece of Tool 1 small enough to build, test and sign off on its own. The design lives in `../tool1-technical-spec.md` and the requirements in `../requirements-spec-tool1.md`. A spec file says *what will be built and how we prove it is done*. It does not repeat the design.

## How to use

1. Pick the lowest-numbered spec whose dependencies are **Done**.
2. Set its status to **In progress**, in the spec file and in the table below.
3. Build until every acceptance criterion is ticked and its verification steps pass.
4. Add every file the spec created to `../docs/Tool1-File-Guide.docx`, in plain language that someone who does not read code can follow: the file, where it is, and what happens in it.
5. Fill in the **Completion record** (date, who, commit), then set the status to **Done**.
6. **Verified** is set only by a second person who re-ran the verification steps. For rule and law-table content, that person is the CA.

If building a spec changes the design, update `tool1-technical-spec.md` in the same commit. If it changes a requirement, stop and ask.

### Status values

| Status | Meaning |
|---|---|
| Not started | Nothing built |
| In progress | Work started; some criteria still open |
| Blocked | Waiting on an open question or a missing sample (the spec says which) |
| Done | All acceptance criteria ticked, verification passes, completion record filled in |
| Verified | Someone other than the builder re-ran verification and agreed |

The status in each spec file is the source of truth. Keep this table in step with it.

## Index

### Phase 1 — Skeleton, fact store, YAML rule engine, one rule end to end (CLI)

| ID | Spec | Depends on | Status |
|---|---|---|---|
| SPEC-001 | [Repository skeleton and architecture guards](SPEC-001-repo-skeleton.md) | — | Done |
| SPEC-002 | [Money and period primitives](SPEC-002-money-period.md) | 001 | Blocked (waiting for CA confirmation of ₹10 rounding for negatives) |
| SPEC-003 | [Core domain model](SPEC-003-domain-model.md) | 002 | Done |
| SPEC-004 | [Fact vocabulary](SPEC-004-fact-vocabulary.md) | 003 | Done |
| SPEC-005 | [Ingestion framework and JSON adapter](SPEC-005-ingestion-json.md) | 003 | Not started |
| SPEC-006 | [Format profiles](SPEC-006-format-profiles.md) | 004, 005 | Not started |
| SPEC-007 | [Rule format and strict loader](SPEC-007-rule-format.md) | 004 | Not started |
| SPEC-008 | [Rule engine core and `equal` check](SPEC-008-rule-engine-core.md) | 007 | Not started |
| SPEC-009 | [Finding wording and report rendering](SPEC-009-finding-wording.md) | 008 | Not started |
| SPEC-010 | [Fact store (SQLite)](SPEC-010-fact-store.md) | 003 | Not started |
| SPEC-011 | [Engine CLI and Phase 1 end to end](SPEC-011-engine-cli.md) | 006, 008, 009, 010 | Not started |

### Phase 2 — First format profiles and 80–120 rules

| ID | Spec | Depends on | Status |
|---|---|---|---|
| SPEC-012 | [Scalar check types](SPEC-012-scalar-checks.md) | 008 | Not started |
| SPEC-013 | [Law tables: format, loader, as-of lookup](SPEC-013-law-tables.md) | 007, 010 | Not started |
| SPEC-014 | [Starlark formula rules](SPEC-014-starlark-rules.md) | 008, 013 | Not started |
| SPEC-015 | [Draft history and draft comparison](SPEC-015-drafts.md) | 008, 010 | Not started |
| SPEC-016 | [Starter rule library: tie-out R1–R10](SPEC-016-starter-rules.md) | 012, 013, 014, 015 | Not started |
| SPEC-017 | [Determinism, incremental re-run and performance harness](SPEC-017-determinism-performance.md) | 011 | Not started |

### Phase 3 — Excel, Word and Tally; new-head check

| ID | Spec | Depends on | Status |
|---|---|---|---|
| SPEC-018 | [Excel and CSV adapter](SPEC-018-excel-csv-adapter.md) | 006 | Not started |
| SPEC-019 | [Word (.docx) table adapter](SPEC-019-word-adapter.md) | 006 | Not started |
| SPEC-020 | [Tally XML adapter and record store](SPEC-020-tally-records.md) | 006, 010 | Not started |
| SPEC-021 | [New-head detection and worklist](SPEC-021-new-heads-worklist.md) | 020 | Not started |

### Phase 4 — Server and UI

| ID | Spec | Depends on | Status |
|---|---|---|---|
| SPEC-022 | [Server foundation, login and roles](SPEC-022-server-auth.md) | 010 | Not started |
| SPEC-023 | [Job queue, engine supervision and IPC](SPEC-023-jobs-ipc.md) | 011, 022 | Not started |
| SPEC-024 | [Clients, engagements, uploads, vault and drafts](SPEC-024-uploads-vault.md) | 022, 023 | Not started |
| SPEC-025 | [Findings lifecycle and decisions](SPEC-025-findings-lifecycle.md) | 023, 024 | Not started |
| SPEC-026 | [Web UI: Season board, Client file, Worklist](SPEC-026-web-ui.md) | 024, 025 | Not started |

### Phase 4b — Rule Builder, law tables, Format Register

| ID | Spec | Depends on | Status |
|---|---|---|---|
| SPEC-027 | [Rule and law-table governance, Rule Builder](SPEC-027-rule-builder.md) | 013, 025, 026 | Not started |
| SPEC-028 | [Format Register and vocabulary browser](SPEC-028-format-register.md) | 026 | Not started |

### Phase 5 — GST, TDS and payroll

| ID | Spec | Depends on | Status |
|---|---|---|---|
| SPEC-029 | [Record matching and date, threshold, period checks](SPEC-029-record-checks.md) | 013, 020 | Not started |
| SPEC-030 | [PDF and text/fixed-width adapters](SPEC-030-pdf-text-adapters.md) | 006 | Not started |
| SPEC-044 | [Scanned PDFs and photos (no OCR in Tool 1)](SPEC-044-scanned-pdfs-photos.md) — build straight after 030 | 024, 030 | Not started |
| SPEC-031 | [GST reconciliation module](SPEC-031-gst-module.md) | 018, 029 | Not started |
| SPEC-032 | [TDS reconciliation module](SPEC-032-tds-module.md) | 021, 029, 030 | Not started |
| SPEC-033 | [Payroll reconciliation module](SPEC-033-payroll-module.md) | 029, 030 | Not started |

### Phase 6 — Security, audit and installer

| ID | Spec | Depends on | Status |
|---|---|---|---|
| SPEC-034 | [Encryption at rest and key management](SPEC-034-encryption-keys.md) | 024 | Not started |
| SPEC-035 | [Security hardening: HTTPS, sessions, role matrix](SPEC-035-security-hardening.md) | 022 | Not started |
| SPEC-036 | [Hash-chained audit log](SPEC-036-audit-log.md) | 022 | Not started |
| SPEC-037 | [Backup and restore](SPEC-037-backup-restore.md) | 034 | Not started |
| SPEC-038 | [Exports: Excel, watermarked PDF, client query letter](SPEC-038-exports.md) | 025, 036 | Not started |
| SPEC-039 | [Data protection: client export, deletion, retention](SPEC-039-data-protection.md) | 034, 036 | Not started |
| SPEC-040 | [Installer and packaging](SPEC-040-installer.md) | 034, 035 | Not started |

### Phase 7 — Rule packs and updates by USB

| ID | Spec | Depends on | Status |
|---|---|---|---|
| SPEC-041 | [Content packs: sign, verify, install, rollback](SPEC-041-content-packs.md) | 027, 036 | Not started |
| SPEC-042 | [Binary updater](SPEC-042-binary-updater.md) | 040, 041 | Not started |

### Later / optional

| ID | Spec | Depends on | Status |
|---|---|---|---|
| SPEC-043 | [Tally live collector (optional)](SPEC-043-tally-collector.md) | 020, 024 | Not started |
| — | Voice (requirements Phase 8) | — | Deferred, no spec yet |

## Open questions that block specs

| Question (requirements §15 / tech spec §B17) | Blocks |
|---|---|
| Q1 Which tax suite(s); JSON samples | Real profiles in SPEC-006 and SPEC-016 (synthetic profiles go ahead) |
| Q2 Which Excel/Word staff documents; samples | Real profiles in SPEC-018 and SPEC-019 |
| Q3 Tally version and export steps | SPEC-020 |
| Q4 Keep or remove the licence check | SPEC-040 |
| Q5 Windows test PC and tester | Windows verification of every spec from SPEC-011 on |
| Q6 Number of concurrent staff users | SPEC-022 sizing, SPEC-023 load test |
| Q7 DPDP legal opinion | Any compliance wording in SPEC-039 |
| T-Q2 HTTPS certificate on staff PCs | SPEC-035 |
| T-Q3 Who may disable a rule | SPEC-027 |
| T-Q4 Does a Phase 0 still gate Phase 1? | Start of SPEC-001 |
| T6 Finding states, including `withdrawn` | SPEC-025 |
| S-Q1 Searchable scans (a scan with the scanner's own invisible OCR text): refuse as scanned? | SPEC-044 AC-9 |
