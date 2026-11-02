# Data model

Purpose: one place for the shapes Tool 1 stores and passes around. It summarises the technical spec §A5 (domain model), §B1 (schema), §B3 (vocabulary) and §B7 (findings lifecycle), checked against `internal/domain` and the migrations in `internal/store/migrations` as they stand after SPEC-006.

Where this page and the technical spec differ, the code and migrations describe what is built and [tool1-technical-spec.md](../../tool1-technical-spec.md) describes the design. Planned tables are marked **planned**.

## Ground rules

- **Money is integer paise everywhere** (`domain.Paise`, an `int64`; Rs 1 = 100 paise). Money columns are `INTEGER` and named `*_paise`. No floating point anywhere in money arithmetic; `internal/money` parses amounts from text and the architecture guard refuses float use in the money packages (`TestMoneyPackagesUseNoFloatingPoint`). Rates are basis points (`rate_bp`, 1% = 100).
- **Every value keeps its source.** A fact or record always carries a locator: file name, kind (`json_path`, `sheet_cell`, `table_cell`, `pdf_pos`, `xml_path`, `line`) and the concrete path, for example `$.business_income.allowed_depreciation` or `$[3].amount_inr` (fourth CSV data row).
- **Text columns are `NOT NULL`**, with `''` meaning "none", so no query has to tell `NULL` from `''` (migration 0001).
- **Names come from the vocabulary.** Fact names, record types and document types are listed in [content/vocabulary/facts.yaml](../../content/vocabulary/facts.yaml) and [records.yaml](../../content/vocabulary/records.yaml). [docs/fact-vocabulary.md](../fact-vocabulary.md) is generated from them (`go run ./cmd/recon-dev vocab-doc`) and never hand-edited (§B3).
- **Determinism.** Lists are stored and returned in a fixed order (evaluations by `(rule_id, subject_key)`), and the report holds no run ID, database ID or time.

## Domain entities (`internal/domain`, §A5)

| Entity | Key fields | Notes |
|---|---|---|
| **Fact** | engagement, `Name` (vocabulary), `Period` (`FY2025-26`, `FY2025-26:Q1`, `2025-04`), `Scope` (`''`, GSTIN, branch, Tally company), `Value`, `Locator` | One scalar figure. `Value.Kind` is one of money, int, text, date, bool, rate; only the field it names is meaningful |
| **Record** | engagement, `RecordType`, `Period`, `Scope`, `MatchKey`, `Amounts` (name to paise), `Fields` (name to text), `Locator` | One row of a tabular source (an invoice, a ledger month). Used for row-by-row matching (`match_records`) and the Worklist |
| **Rule** (the "Edge" of the tie-out model) | rule ID, version, tax years, check type, inputs, status | A declarative YAML file (`recon.rule/1`) under [content/rules](../../content/rules/) with pass and fail fixtures (§B4.2). It asserts that facts are equal or satisfy a relation. Rules are data, not code |
| **Subject** | `Period`, `Scope`, `MatchKey` | What one evaluation is about. `Key()` is `period|scope|match_key` with `|` and `\` escaped in period and scope |
| **Evaluation** | run, rule ID and version, subject, `Status` (`pass`, `fail`, `cannot_check`), `Values` (every side with label and locator), `Difference` (paise), `MissingFacts`, `LawRefs` | The result of one rule for one subject. A missing input is always `cannot_check`, never a zero and never a fail (§B4.4) |
| **Finding** | stable ID, value fingerprint, review document, latest evaluation | A failed evaluation. Grouped in the report by the document to review (`review_document`), not by rule category |
| **Decision** | finding ID, decision (`fix`, `not_an_issue`, `ask_client`), note, by, at, value fingerprint | Taken by staff (SPEC-006 part 3). A note is required for `not_an_issue` |
| **Client** | name, PAN (as keyed hash and last four only), details, GSTINs | The client master (SPEC-004 parts 7 and 8) |
| **Engagement** | client (or command-line key), tax year | One client's tax year; documents uploaded at different times are compared together |
| **Run and snapshot** | engine version, vocabulary version and hash, document versions with the profile that read each, rule versions with their status | Every versioned input of a run is recorded (RU-53, §A10) |
| **Worklist item** | kind (`new_head`, `vanished`, `broken_recurrence`, `step_change`), ledger head, both totals, both month counts, the rows read | From the year-on-year ledger comparison (`internal/modules/newheads`, SPEC-006 part 3). It is **not** a finding and never counted as one. `unmapped` is planned |

`Evaluation` and `Finding` have a canonical JSON encoding (keys sorted, integers only, empty lists as `[]`); golden files and determinism tests compare these bytes (§A5).

## Finding identity (§B7.1, `internal/domain/identity.go`)

- `StableID = base32(sha256(rule_id | engagement_id | period | scope | match_key))`, first 16 characters, standard upper-case alphabet. The rule **version**, the run and every locator are left out, so a re-run or a rule edit updates the same finding instead of duplicating it.
- `ValueFingerprint = hex(sha256(canonical JSON of the values only))`, in rule order. Labels, input names and locators are not hashed, so the same figure on a new page of a new draft is not a change of value.
- A decision is kept against the stable ID. If the fingerprint changes after the decision, the finding is Open again and the decision is shown as made on other values (§B11, SPEC-006 part 3).

## Decisions and classifications are append-only

`finding_decisions` and `worklist_classifications` have `BEFORE UPDATE` and `BEFORE DELETE` triggers that abort. A change is a new row; the latest row is the current decision or classification and earlier rows are its history. The same rule is planned for `audit_log` (§B1.3).

## SQLite tables as built

Migrations are numbered `.sql` files applied in order; an applied migration is never edited (a change is a new file). Database encryption at rest (AES-256-XTS VFS) is decided in [ADR-0002](../architecture/decisions/ADR-0002-database-encryption.md) but key handling is Module 14 (planned): today the database is a plain file.

| Migration (spec) | Table | Columns (abridged) |
|---|---|---|
| 0001 (SPEC-003) | `engagements` | id, cli_key, tax_year, unique (cli_key, tax_year); `client_id` added in 0002 (0 = a command-line run named by its `--input` folder; a client's engagement has `cli_key` `client:<id>`) |
| | `rule_versions` | id, rule_id, revision, version, source (`rulebook`/`builtin`/`office`), schema (`recon.rule/1` runnable, `rulestore/1.0` descriptive only), tax_years, file_path, body_yaml, sha256, pack_id, created_by, created_at. A changed file adds a revision; nothing is overwritten |
| | `rule_state` | rule_id, active_version_id, status (`draft`/`testing`/`active`/`disabled`), approved_by, approved_at, disabled_reason, status_set_by (who set the status by hand; a rule-book load leaves it alone, migration 0009) |
| | `documents` | id, engagement_id, document_type, period, scope, part: the document slot (IN-11) |
| | `document_versions` | id, document_id, draft_no, file_sha256, file_name, profile_id, profile_version, uploaded_by, uploaded_at, extraction_status (`ok`/`failed`), validation_report_json |
| | `facts` | id, document_version_id, engagement_id, name, period, scope, kind, money_paise, int_value, text_value, date_value, bool_value, rate_bp, locator_kind, locator_path |
| | `runs` | id, engagement_id, tax_year, engine_version, vocab_version, vocab_hash, include_testing, started_at, finished_at; later `report_json` (0002), `comparison_id` (0005), `worklist_json` (0006) |
| | `run_documents`, `run_rules` | the snapshot: document version with type, file hash and profile; rule version with the status it had |
| | `evaluations` | id, run_id, seq, rule_id, rule_version, subject_key, status, values_json, difference_paise, missing_json |
| 0002 (SPEC-004 part 7) | `settings` | name, value (for example the key that hashes PANs) |
| | `clients` | id, name, pan_hash (keyed hash), pan_last4, status (`active`/`archived`), created_at |
| 0003 (SPEC-004 part 8) | `records` | id, document_version_id, engagement_id, record_type, period, scope, match_key, amounts_json (`{column: paise}`), fields_json (`{column: text}`), locator_kind, locator_path |
| 0004 (SPEC-004 part 8) | `clients` (more columns) | trade_name, constitution, date_of_incorporation, cin, tan, address, city, state, pin, contact_person, email, phone, accounting_software, notes, updated_at |
| | `client_gstins` | client_id, gstin (unique across clients), registration_type, is_primary, position |
| 0005 (SPEC-004 part 9) | `runs.comparison_id` | the type of check run (`''` = every rule), so a client keeps its latest result per type |
| 0006 (SPEC-006 part 3) | `finding_decisions` | id, engagement_id, finding_id (stable ID), rule_id, decision, note, decided_by, decided_at (UTC, RFC 3339), value_fingerprint. Append-only |
| | `worklist_classifications` | id, engagement_id, kind, ledger, classification (free text), classified_by, classified_at. Append-only |

Indexes: `facts(engagement_id, name, period, scope)`, `records(engagement_id, record_type, period, match_key)`, `evaluations(run_id, rule_id)`, `runs(engagement_id, comparison_id, id)`, and one per decision and classification lookup.

### Planned tables (§B1.1)

`users`, `sessions`, `client_scopes`, `files` (vault), `vocab_versions`, `vocab_entries`, `profiles`, `law_tables`, `law_rows`, `packs`, `jobs`, `run_law_rows`, `findings`, `finding_events`, `worklist_items`, `audit_log`. Until §B7 is built, `finding_decisions` stands in for `findings.decision*` and `finding_events`, and `runs.worklist_json` stands in for `worklist_items`. The finding states of §B7.2 (`open`, `decided_*`, `awaiting_recheck`, `resolved`, `reopened`, `carried_forward`, proposed `withdrawn`) are **Open**: they need CA sign-off (§B17 T6).

## Data protection in the schema (§B1.3)

- A full PAN is never stored in `clients` and never returned by the API or put in a validation report; only `pan_hash` and `pan_last4`. The CLI takes a PAN on standard input, never as an argument.
- A GSTIN contains the PAN, so a client with a GSTIN has its PAN readable in the database. This is accepted and is one more reason for encryption at rest.
- All data in the repo is illustrative. Never put real client data in fixtures or samples.
