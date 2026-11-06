# SPEC-022 — Document Master: document types, record types and rules over them, managed by the Partner

| | |
|---|---|
| **Status** | In progress: Parts A and B built and verified except AC-15 (a look in the browser), 6 November 2026; waiting for that look and a second person to verify |
| **Phase** | 3 |
| **Depends on** | SPEC-016 (formats and the mapping screen), SPEC-012 and SPEC-013 (users, roles, audit log), SPEC-004 (vocabulary, record types, taught layouts), SPEC-010 (Rule Builder, office rules, rule versions and lifecycle), SPEC-007 (rule fixtures, `rule-lint`), SPEC-009 (comparison history) |
| **Requirements** | Rules are data (C5); determinism (C4); no network (C3); every change is audited (RU-42); a rule keeps the version it ran with (RU-41, RU-53); a manual file-drop path for every source; no real client data |
| **Technical spec** | §A (vocabulary), §B3 (record types, `rule_versions`), §B9 (Rule Builder, audit), §B12 (roles) |
| **Code / content paths** | `internal/vocab`, `internal/store/migrations/0015_document_master.sql`, `internal/store/docmaster.go`, `cmd/recon-engine/docmaster.go`, `cmd/recon-engine/teach.go`, `cmd/recon-engine/officerules.go`, `cmd/recon-engine/run.go`, `internal/rules`, `internal/api/documents.go`, `internal/auth/roles.go`, `web/src/screens/DocumentMaster.tsx`, `web/src/screens/TeachLayout.tsx`, `web/src/screens/RuleBuilder.tsx` |

## Goal

The Partner adds, edits, deactivates and (when nothing uses it) deletes the kinds of document the tool can read, without a code change. A new document is defined in two steps that can be done on different days: the **document** (name, code, category, applicable file formats) and its **record structure** (the columns of one row, their data types, and which identify a row). The document appears in the layout wizard only when its structure is complete and valid, and only for files of a format it supports. Once a layout reads its rows, staff and the Partner write **reconciliation rules over those rows** in the existing Rule Builder, with the existing versions, testing-to-active lifecycle and audit, and the engine reports their findings like any other. The Master never creates a file format, a list detection rule, a column mapping or a rule by itself; each stays a separate step.

The work is delivered in two parts that share one spec: **Part A** the Document Master and the layout wizard, **Part B** rules over office record types. Part A is complete and usable on its own before Part B starts.

## Decisions made (5 November 2026)

1. **Reuse is a copy.** A new document may start from an existing record type (built-in or office) by copying its columns into an **independent** office record type with its own columns, labels, data types, key flags and required flags. A later change to either never affects the other. There is no shared record type.
2. **Applicable for = file formats.** The Partner picks one or more of JSON, XLSX and CSV. The layout wizard lists only documents compatible with the uploaded file's format. It is not about client types and never gates which rules run.
3. **Rules over office record types are in this spec** (Part B), added to the existing Rule Builder rather than built again.
4. **Row-based rules only in the first version.** `match_records`, `sum_equals` and `present_together`. Office facts, derived values and advanced calculations are a separate future spec, designed once the row-based work is stable.
5. **A new `office` module.** Rules over office documents belong to a new `office` module in the Rule Builder, kept apart from the built-in modules so the Partner can find and manage them. It reuses the existing Rule Builder, versions, testing, approval, audit, fixtures and permissions; there is no second Rule Builder and no duplicated rule-management logic.
6. **Deactivation warns and then proceeds.** Deactivating a document that formats or active rules use shows a warning before confirmation and is then allowed. Existing formats, stored records and active rules keep working. Deactivation never disables or edits a rule on its own.
7. **Phased delivery.** Part A first; Part B starts after Part A's regression run passes.

## Verification of risk 1 against the code (5 November 2026)

Checked by reading the implementation, before any build. Result: **no change to extraction or to the three checks is needed**; one small, optional display point is noted.

| Place | Finding |
|---|---|
| `profile.Extract` / `extraction.record` (`internal/profile/extract.go`) | Generic. Reads each column named by the mapping, `rm.fields` as text (with the optional `dd-mm-yyyy` conversion) and `rm.amounts` as money, and builds the match key from `rm.rt.MatchKey`. No built-in column name appears. |
| Mapping and `RecordTypeFor` (`internal/profile/mapping.go`) | Generic: finds the record type by `document_type`, and validates the chosen columns against that record type. The client GSTIN is a file-level setting (`gstin_path`, read as PAN and scope), not a column of the record type, so an office document without a GSTIN column works. |
| `match_records` (`matchrecords.go`) | Compares the columns the rule names, checked against the two record types' `Fields` and `Amounts`. The only built-in name is `invoice_value`, used to show a matched key and only when it is among the compared amounts; otherwise the first compared amount is used. Generic. |
| `sum_equals` rows and totals, `present_together` | They check column names with `slices.Contains` on the record type's `Amounts` and `Fields`. Generic. |
| Engine (`engine.go`) | Groups rows by record type name; no built-in assumption. A check type that cannot evaluate rows is refused already, which the Rule Builder uses to offer office inputs only for row-reading checks. |
| `findings.Totals` and `totalColumns` | The totals table lists built-in amount names, but only for documents named in `comparisons.yaml`. Office documents are in none, so no totals table is affected. |
| `matchrecords.ColumnLabel` | A display-only function with a few built-in names (IGST, SGST/UTGST, GSTIN); every other column is shown as its name with underscores turned into spaces. Office columns therefore read as their names in **finding text**. This is acceptable for the first version; the Partner can name columns so they read well. Column labels from the Master are used in the wizard, the Rule Builder and the Document Master. Carrying a label into finding text would need the label to reach the engine (a later change). |
| Hash safety | `contentHash` marshals `vocab.RecordType` to JSON. Office labels therefore live in the office layer (`vocab.Office`), not as a new field on `RecordType`; any field added to `RecordType` itself must be tagged `json:"-"` or the base hash would change. |
| `rules.Modules` (`internal/rules/rule.go:124`) | A list of seven names, checked by the loader and the builder. Adding `"office"` is a one-line change; the only other use of a module name in code is a display mapping (`moduleType`, "tieout" to "tie-out"), which leaves `office` as it is. The office rules folder layout `<tax-year>/<module>/<ID>` works with it unchanged. |

## Scope

### Part A: Document Master

1. A **Document Master** tab (Admin area, Partner only) listing every document type: name, code, category, description, origin (Built-in or Office), applicable formats, structure (record type) and status. Actions: Add, View, Edit, Activate / Deactivate, Delete (office documents only, when unused).
2. **Add / Edit document** form: Document name, Unique document code, Category, Description, Applicable for (JSON, XLSX, CSV; one or more), Status, and the record structure (see 3).
3. **Record structure** for a document, either:
   - **Copy** the columns of an existing record type into a new, independent office record type; or
   - **Define new columns**.

   In both cases the office record type has a name, a description and a list of columns. Each column has: name (the key in files, lower snake case), label (shown in the wizard), type (`text`, `date`, `amount`), and flags `key` (part of the match key) and `required`. At least one key column is needed; keys are text or date columns, never amounts.
4. **Readiness.** A document is **Draft** until its structure passes every check in "Validation"; then it can be **Active** or **Inactive**. Only Active documents are offered in the wizard.
5. The layout wizard (`TeachLayout`) gets its Document list, and each document's columns (label, type, key flag), from the server, which merges the built-in and the office definitions and filters by Active and by the uploaded file's format.
6. **Permissions:** a new permission `manage_documents`, held by the Partner only. Reading the list (to fill the wizard) needs only sign-in.
7. **Audit:** every add, edit, activate, deactivate and delete writes one entry (`document.*`, `record_type.*`).

### Part B: Rules over office record types

8. The **Rule Builder** (SPEC-010) lists office record types, with their columns, as inputs beside the built-in ones; the rule form, the fixture editor and the generated YAML use their column names, types and labels.
9. The **rule loader, `rule-lint`-style fixture checks, the Rule Builder's save, `load-rules` and `run`** read rules against the **merged vocabulary** (a strict superset of the base one), so a rule that reads an office record type loads, is tested and runs.
10. **Which checks.** An office record type is rows only (it has no facts), so a rule can read it in the check types that read rows: `match_records` (office with office, or office with a built-in record type), `sum_equals` (per row, or totals of rows), and `present_together` (per row). `equal`, `gte`, `percent_of` on a fact and `table_lookup` keep reading base facts only; naming an office record type as their input is refused with a message, as the loader already refuses a figure that is not there.
11. **Lifecycle, versions, permissions and audit are the existing ones** (SPEC-010, SPEC-013): a saved rule starts as testing and runs only with testing rules included; the Partner approves it once its pass and fail fixtures pass and it has run (RU-40, RU-43); versions are kept and can be switched back (RU-41); drafting needs `draft_rules`, approving `activate_rules`, disabling `disable_rules`. Nothing in this spec adds a second rule lifecycle.
12. **Built-in rules and the built-in vocabulary are not changed.** Built-in rules keep loading from `content/rules` with the same results; the Rules tab marks a rule that reads an office record type with an "Office document" chip.

## Out of scope

- **Facts for office documents** (single figures such as a total read from a document) and therefore `equal`, `gte`, `percent_of` and `table_lookup` over office documents. They would need an office fact vocabulary and fact periodicity; a later spec.
- New check types, and editing built-in rules, record types or documents.
- Creating a format, mapping or list detection from the Master (the wizard does it).
- Sharing a record type between documents, importing or exporting definitions between offices.
- Renaming a document code or a record type name after creation.

## Architecture

### 1. Two layers: the base vocabulary stays exactly as it is

- `content/vocabulary/facts.yaml` and `records.yaml` remain the **base vocabulary**: versioned, hashed, matched by `docs/fact-vocabulary.md`. Nothing in them changes, and `vocab.LoadDir` returns the same value and the same `ContentHash()` as today.
- A new **office vocabulary** layer holds the Partner's document types and record types in the database. It is not part of `vocab_version`, `docs/fact-vocabulary.md` or the rule-store checks.
- `vocab.Vocab` gets `WithOffice(Office) (*Vocab, error)`, which returns a **merged copy**: the base entries plus the office entries that are ready. The base `Vocab` is never modified. The merged vocabulary is a strict superset of the base, so a built-in rule validates and runs exactly as before when read against it.
- One helper `loadVocab(db, dir)` in `cmd/recon-engine` returns the merged vocabulary. It is used by `teach`, `formatadmin`, `formatedit`, `formatextra`, `run`, `facts`, `comparisons`, `history` (Part A) and by `load-rules`, `rules`, `officerules`, the Rule Builder commands and `rule-save` (Part B). `recon-dev` (`rule-lint`, `rulestore-check`, `vocab-doc`) and the tests that call `vocab.LoadDir` directly stay on the base, so built-in content is always checked against built-in vocabulary only.
- Office entries are checked by the **same checker** as the YAML files: `checkRecords` and the name rules are refactored into functions that both paths call. One definition of a valid record type.

### 2. Names cannot collide

An office document code and record type name must not equal any base or office name (case-insensitive), must match `^[a-z][a-z0-9_]*$`, are 3 to 40 characters, and an office document code must not equal a base prefix. Office documents have no `prefix` (no facts): the merged `DocumentType` carries `Office: true` and an empty prefix, and the fact checks never see it.

### 3. Hashing

- `ContentHash()` stays the hash of the base vocabulary only. Existing runs, golden files and Test Pack A scores are unchanged.
- `OfficeHash()` is the SHA-256 of a fixed JSON form of every **ready** office document and record type in name order (same method as `contentHash`: struct fields in declaration order, no maps). It is `""` when there are no office definitions, so a database with an empty Master hashes exactly as before.
- It covers meaning only (name, description, category, formats, column names, types, key and required flags), not the Active flag, so deactivating a document does not change the hash of later runs.
- `runs` gets `office_vocab_hash TEXT NOT NULL DEFAULT ''`; a run's vocabulary identity is the pair (base hash, office hash).

### 4. Which definitions are in the merged vocabulary

| State | In merged vocab (extraction, rules, runs) | In wizard list | In Rule Builder input list |
|---|---|---|---|
| Draft (incomplete) | no | no | no |
| Ready and Active | yes | yes | yes |
| Ready and Inactive | yes | no | no for a new rule; yes for a rule that already reads it |

An Inactive document stays in the merged vocabulary, so existing formats keep reading and existing rules keep running. Deactivating only hides it from **new** layouts and **new** rules.

### 5. Reproducing an old run (versioning)

- Rule files are stored per version in `rule_versions` (SPEC-010), and a run already records the rule versions it used. That is unchanged.
- **New:** each run also stores the office vocabulary it used. `office_vocab_snapshots (hash PRIMARY KEY, body_json)` holds the canonical JSON that `OfficeHash()` hashes, written once per distinct hash (append-only, never updated). `runs.office_vocab_hash` points to it.
- Reading an earlier comparison (SPEC-009, read-only "exactly as it was reported") shows the rows, columns and labels from the snapshot, not from the current Master, so editing or deactivating a document later cannot change an old report.
- Re-running an old run's rules (when the Rules tab switches back to an earlier version) validates against the **current** merged vocabulary; if the switch would use a column the record type no longer has, it is refused with the column named. Locking (next section) makes that case impossible for a referenced record type.

### 6. Editing and locking

- A record type is **locked** once any format version names its document, any `records` row has its name, **or any office rule version (any status) reads it**. A locked record type allows only: description, column labels, and adding **non-key, non-required** columns. It refuses removing, renaming or retyping a column, and changing a key or a required flag.
- An unlocked record type may be edited freely until it is first used.
- The document's name, category, description and applicable formats are always editable (removing a format that an existing format uses is refused); the code is never.
- A column added to a locked record type changes nothing for existing formats and rules (they do not name it); a later format version or rule version may use it.

### 7. Deletion and deactivation

- Only office documents can be deleted, and only when **no format version references them**, **no `records` row uses their record type** and **no office rule version reads it**. A format reference is read from the new `format_versions.document_type` column (see Database), not from the free-text `document` label; a rule reference is read from the rule's inputs.
- The record type goes with the document if no other document uses it.
- Built-in documents are never deleted.
- A refused delete says which formats and rules use it and offers Deactivate.
- **Deactivation is always allowed and preserves everything.** When formats, stored records or active rules reference the document, the confirmation first shows a warning: the document will no longer be available for new layouts or new rules, and the existing formats, stored records and active rules that reference it (listed by name, with counts) will continue to work. It then proceeds on confirmation. Deactivation never disables, edits or re-versions a rule, and writes one audit entry (`document.deactivate`) that records the counts.

### 8. Rules over office record types (Part B)

- The rule loader (`rules.Load`) already resolves an input's `record_type` through `Vocab.RecordType`; it needs no change except the vocabulary it is given. Each check type's `ValidateParams` already reads columns from the record type it is given; the three row-reading check types are verified generic over column names in part 1 (risk 1).
- `matchrecords` compares columns by name. A rule between an office record type and a built-in one chooses compare fields, key and amounts from both record types, exactly as for two built-in types; a mismatch is refused by the existing validation with the column named.
- **Column types.** The check types read the vocabulary's two lists, `amounts` (money) and `fields` (everything else). An office column of type `amount` goes to `amounts`; `text` and `date` go to `fields`. The wizard's date handling reads `date`. No check type changes.
- **Fixtures.** `fixture.go` builds fixture rows from the record type; the fixture editor of the Rule Builder lists the office columns and their types. A rule must have at least one pass and one fail fixture (existing rule), which run in the engine before approval.
- **Findings.** The engine, the findings report and the screens already handle any record type name; a finding shows both values, the difference and both source locators. In finding text an office column appears as its name with underscores shown as spaces (see "Verification of risk 1"); the Master's labels are used in the wizard, the Rule Builder and the Document Master.
- **Module.** Rules over office record types use the new `office` module (`"office"` added to `rules.Modules`). A rule that reads an office record type must be in the `office` module, and an `office` rule may read built-in record types too (for example office rows against GSTR-1 rows).
- **Where office rules live.** In the office rules folder (`--office-rules`) as today, with `source: office`. The Master adds no rule files.
- **Order of work for a Partner:** add the document, teach a layout, then write a rule. The Rule Builder offers only ready office record types; it says so, and links to the Master, when the list is empty.

## Database

Migration `0015_document_master.sql` (new, append-only; no existing migration is edited):

```
document_types (
  code TEXT PRIMARY KEY,                -- unique, lower snake case
  origin TEXT NOT NULL CHECK (origin IN ('builtin','office')),
  name TEXT NOT NULL, category TEXT NOT NULL, description TEXT NOT NULL,
  applicable_for TEXT NOT NULL,          -- JSON array of 'json','xlsx','csv', in that order
  record_type TEXT NOT NULL DEFAULT '',  -- name of its record type; '' while Draft
  active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0,1)),
  created_by TEXT NOT NULL, created_at TEXT NOT NULL, updated_by TEXT NOT NULL, updated_at TEXT NOT NULL)

office_record_types (
  name TEXT PRIMARY KEY, description TEXT NOT NULL,
  copied_from TEXT NOT NULL DEFAULT '',  -- the record type it was copied from, for information only
  created_by TEXT NOT NULL, created_at TEXT NOT NULL)

office_record_columns (
  record_type TEXT NOT NULL REFERENCES office_record_types(name),
  position INTEGER NOT NULL,             -- the order shown in the wizard
  name TEXT NOT NULL, label TEXT NOT NULL,
  type TEXT NOT NULL CHECK (type IN ('text','date','amount')),
  is_key INTEGER NOT NULL DEFAULT 0, required INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (record_type, name), UNIQUE (record_type, position))

office_vocab_snapshots (
  hash TEXT PRIMARY KEY, body_json TEXT NOT NULL)   -- append-only: update and delete triggers refuse

ALTER TABLE runs ADD COLUMN office_vocab_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE format_versions ADD COLUMN document_type TEXT NOT NULL DEFAULT '';
```

- `copied_from` is information only; after the copy no query follows it.
- Built-in documents are rows with `origin='builtin'`, filled **from the YAML on every start** (idempotent upsert of name, description, record type; the `active` flag and the default category are kept). Their definition stays authoritative in YAML. A trigger refuses deleting a `builtin` row.
- Office documents and record types are definitions, deleted for real when unused; the audit log keeps the history. Snapshots are never deleted, so an old run can always be shown.
- `format_versions.document_type` is filled for new versions at save time and for existing rows by an idempotent step run once after the migration (it reads the `document_type:` line of `body_yaml`). The existing `format_versions_no_update` trigger does not list this column, so the fill is allowed.
- Rule references to a record type are read from the rule bodies already stored in `rule_versions`; no new rule table.
- Reads that reach output have a total `ORDER BY` (category, name, code). No client data is stored in these tables.

## Validation

A document can be made Ready only when all hold; the screen runs the same checks for early feedback and the engine decides:

1. Code, record type name and column names are valid names, unique, not reserved, and 3 to 40 characters (columns 2 to 40).
2. Name 3 to 60 characters, one line; description 10 to 300 characters, one line; category from the fixed list (Books, GST, Income tax, TDS, Payroll, Bank, Other); applicable for has at least one of json, xlsx, csv.
3. The record type has 1 to 40 columns, at least one key column, every key column is `text` or `date`, and at least one column that is not a key (a structure of keys only reads nothing useful).
4. Exactly one non-deprecated record type has this document as its main record type (the existing `RecordTypeFor` rule). Because reuse **copies**, the new document always has its own record type.
5. A failed validation returns every problem together, each naming the field, with the `BAD_*` codes the layout wizard already uses.

Part B validation is the Rule Builder's own (SPEC-010): the same `Problems` and the strict loader, run against the merged vocabulary, plus: an office record type may be an input only of a row-reading check type; a rule that names a column the record type does not have is refused with the column named.

## APIs

Same split as formats and rules: engine commands do the work and write the audit entry in one transaction; the server forwards and checks the permission.

| Method and path | Needs | What |
|---|---|---|
| `GET /api/v1/document-types` | sign-in | The merged list: code, origin, name, category, description, applicable_for, status, record type, `referenced_by` counts (formats, rules). `?active=1&file_type=xlsx` gives the wizard's list |
| `GET /api/v1/document-types/{code}` | sign-in | One document with its columns, and the formats and rules that reference it |
| `POST /api/v1/document-types` | `manage_documents` | Add (Draft or, when valid, Active or Inactive) |
| `PUT /api/v1/document-types/{code}` | `manage_documents` | Edit, within the locking rules |
| `POST /api/v1/document-types/{code}/status` | `manage_documents` | `{active: true/false}` |
| `DELETE /api/v1/document-types/{code}` | `manage_documents` | Delete, or `409 IN_USE` with the formats and rules that use it |
| `POST /api/v1/document-types/validate` | `manage_documents` | Validates a definition without saving it |

- `/api/v1/inspect` keeps its `document_types` field, built from the merged vocabulary, filtered by Active and by the uploaded file's format, with each column gaining `label`, `type` and `required`. Existing clients ignore the new fields.
- **Part B:** the existing rule endpoints (SPEC-010, SPEC-013) are unchanged in path, method and permission. The vocabulary they return to the Rule Builder includes ready office record types with their columns; saving, testing, approving and listing versions use the merged vocabulary. A rule row gains `reads_office_documents` for the Rules tab chip.
- Engine commands: `document-list`, `document-get`, `document-save`, `document-status`, `document-delete`, `document-validate` (JSON on standard input, `--json` out, exit codes as the other commands).

## UI workflow

1. **Admin, Document Master tab** (Partner only; hidden for other roles, the server refuses anyway). A table with search, filters (category, origin, format, status) and sort; columns: Name, Code, Category, Description, Formats, Structure, Status; row actions View, Edit, Activate / Deactivate, Delete. Built-in rows show a "Built-in" chip and have View and Activate / Deactivate only.
2. **Add document** opens a drawer in three steps, the same pattern as the layout wizard: (1) About the document: name, code, category, applicable file formats (three checkboxes), description, status; (2) Record structure: "Copy the columns of…" (choose a record type) or "Define new columns", then a column table (name, label, type, key, required, reorder), with the note that a copy is independent; (3) Check and save: the checks with a tick or the problem beside each, and a note that no format, list, column mapping or rule is created.
3. A document that is not valid saves as **Draft** and says what is missing. It is not offered in the layout wizard or the Rule Builder.
4. **View** is read only: columns, the formats and rules that use it, and its audit history.
5. **Delete** is shown only for unused office documents and asks for confirmation; for a used one the button is replaced by the list of formats and rules and the Deactivate action.
6. **Layout wizard:** the Document dropdown is filled from the server (Active, compatible with the uploaded file's format). Column labels come from the server; the date format checkbox appears for `date` columns (the `_date` name rule stays for built-ins). An empty list says "No active document accepts a file of this format; the Partner can add one in Document Master."
7. **Rule Builder (Part B):** the input picker groups record types as "Built-in" and "Office documents"; choosing one shows its columns with their labels and types in the check's field pickers and in the fixture editor; a check type that cannot read rows is not offered for an office input. **Rules tab:** the "Office document" chip, and a filter for it.
8. Keyboard-first, labels on every control, errors beside the field, no colour alone, offline; the existing `Modal`, table and form styles are reused.

## Permissions

| Action | Permission | Who |
|---|---|---|
| See the document list, fill the wizard | sign-in | every role |
| Add, edit, activate, deactivate, delete a document or record type | `manage_documents` (new) | Partner |
| Teach a layout on a document | the existing layout permission | as today |
| Draft a rule over an office record type, run rule tests | `draft_rules` | Partner, Manager, Staff |
| Approve (testing to active) | `activate_rules` | Partner |
| Disable or enable a rule | `disable_rules` | Partner |

`PermManageDocuments = "manage_documents"` is added in `internal/auth/roles.go` for the Partner only, to `roleMatrix` and its wording in `web/src/permissions.ts` ("eleven" becomes "twelve"), to the Partner-only list in `internal/api/auth_test.go` and to the Admin tab's role summary. The rule permissions are not changed.

## Backward compatibility

- With an empty Master, the merged vocabulary equals the base: the same document list, the same hashes (`office_vocab_hash = ''`), the same findings, the same sample data. `TestWebSamplesMatchEngine` is regenerated only for the new `label`, `type` and `required` fields on columns and the new rule field.
- Built-in rules, their fixtures and `rule-lint` are untouched and still run against the base vocabulary; a built-in rule read against the merged vocabulary gives the same result.
- Existing formats, taught layouts and office profiles are not touched; their `document_type` values are base names that remain in the merged vocabulary. Existing office rules read only base record types and keep working.
- Existing runs keep their stored hashes; a run made before this spec shows an empty office hash and needs no snapshot.
- An old browser or API client that ignores new fields keeps working.
- A format or rule that reads an office document keeps working when the document is deactivated; deleting is refused while it is referenced.
- Downgrading is not supported after migration `0015` (as for every migration).

## Risks to check in the first part

1. **Resolved by reading the code (see "Verification of risk 1").** `profile.Extract`, mapping validation and the three row-reading check types are generic over column names; no backward-compatible change is needed. Part A1 still starts with a test that proves it: an office record type with column names that no built-in uses (`ref_no`, `net_amount`, `doc_date`) is taught, extracted, matched, summed and checked end to end. If that test finds an assumption the reading missed, the smallest change is made in that place and the Partner is told.
2. `records.record_type` has no foreign key (migration 0003), so office record types need no change there.
3. `internal/vocab` tests (`docagree`, `structure`, `fuzz`) and `rule-lint` stay on the base only.
4. `engine.go` refuses a rule that reads rows through a check type that cannot evaluate rows yet; the Rule Builder must offer an office record type only for the check types that can (list taken from the registered check types, not copied).
5. **Resolved (decision 5).** The `office` module is added to the module list (one line); no engine change.
6. A new package, if any, is classified in `internal/archtest/archtest_test.go`; the plan keeps the code in existing packages.

## Acceptance criteria

**Part A: Document Master**

- [x] AC-1 — Migration `0015` applies to a database at `0014` with no data change; built-in documents are present with `origin='builtin'`; `format_versions.document_type` is filled for every existing version; applying it twice does nothing.
- [x] AC-2 — With an empty Master, `ContentHash()` and the full set of findings for Test Pack A and `testdata/run/*` are byte-identical to before; `office_vocab_hash` is `''`; Test Pack A scores 22 of 30 with 0 false positives.
- [x] AC-3 — Adding a document with a copied structure and a document with new columns each saves, appears in the list, and (when Active and compatible with the file's format) appears in the layout wizard's Document dropdown and in `/api/v1/inspect`, with no code or YAML change.
- [x] AC-4 — A copied record type is independent: changing a column, label, type, key flag or required flag of the copy leaves the source unchanged, and the reverse; `copied_from` is information only.
- [x] AC-5 — The wizard lists a document only for a file whose format (json, xlsx, csv) is in its Applicable for; a document with all three shows for all three; none shows for a format it lacks.
- [x] AC-6 — A document that fails any check in "Validation" saves only as Draft, names every problem, and is absent from the wizard, the Rule Builder and the merged vocabulary.
- [x] AC-7 — A duplicate code (any case), a code equal to a base document, prefix or record type, and a duplicate column are refused.
- [x] AC-8 — A layout taught on a new office document through the wizard reads a sample file (JSON, CSV and Excel), stores rows under the office record type, and a run over it stores a non-empty `office_vocab_hash` and its snapshot; the same run with the Master emptied is refused with a message naming the missing document.
- [x] AC-9 — Deactivating a document removes it from the wizard list and the new-rule input list, but existing formats, rules and runs keep working and the office hash does not change; activating restores it. When formats, stored records or active rules reference the document, the confirmation shows the warning described under "Deletion and deactivation" with the referencing formats and rules and their counts, deactivation then proceeds on confirmation, no rule changes status or version, and the audit entry records the counts.
- [x] AC-10 — A document referenced by a format version, by `records` rows or by an office rule version cannot be deleted (`409 IN_USE`, naming the formats and rules); an unused office document can; a built-in document cannot.
- [x] AC-11 — A locked record type refuses removing, renaming or retyping a column and changing a key or required flag, and accepts a label change and a new optional column; an unlocked one accepts all edits.
- [x] AC-12 — Adding a document creates no format, no list detection rule, no column mapping and no rule (no row in `format_versions`, no profile file, no rule file).
- [x] AC-13 — `OfficeHash()` is the same for the same definitions in any insertion order, differs when a column changes, and ignores the Active flag.
- [x] AC-14 — `manage_documents` is held by the Partner only; the other roles get `403` on every write endpoint and the tab is hidden; reading the list works for every signed-in role; every write adds an audit entry with the actor.
- [ ] AC-15 — The Document Master tab lists, filters, adds (three steps), views, edits, activates, deactivates and deletes as described, works from the keyboard and follows the screen rules.
- [x] AC-16 — The layout wizard shows labels and the date checkbox for office columns from the server, and its existing tests pass unchanged.

**Part B: Rules over office record types**

- [x] AC-17 — In the Rule Builder, a Staff user drafts a `match_records` rule between an office record type and a built-in record type, and a `sum_equals` and a `present_together` rule over an office record type; each saves as testing, with generated YAML that the strict loader accepts and pass and fail fixtures that run in the engine.
- [x] AC-18 — The same rules refuse: an office record type as the input of `equal`, `gte`, `percent_of` on a fact and `table_lookup`; a column the record type does not have; an amount column used where text is needed; a Draft document's record type. Each message names the field.
- [x] AC-19 — A run with testing rules included over synthetic files for an office document and a built-in document reports findings for the office rule with both values, the difference and both source locators; the same run without testing rules, or with the rule disabled, reports none; after the Partner approves the rule, it runs without testing rules included.
- [x] AC-20 — Editing a rule saves a new version and keeps the earlier ones; switching back to an earlier version works; a record type read by any version of an office rule is locked (AC-11).
- [x] AC-21 — Reproducibility: after a run, deactivating the document, adding an optional column to its record type and editing the rule, the earlier comparison opened from the Client file (SPEC-009) shows the same rows, columns, labels, rule versions and findings as when it was run, from the stored snapshot; running its inputs again reproduces the same findings byte for byte.
- [x] AC-22 — Built-in rules are unaffected: with the Master empty and with office documents and rules present, every built-in rule gives the same status and findings for Test Pack A and `testdata/run/*`; `rule-lint` and `rulestore-check` pass unchanged.
- [x] AC-23 — Rule permissions are the existing ones: Manager and Staff can draft and test but not approve or disable; Read-only cannot change a rule; every create, edit, approve, disable and enable of a rule over an office record type adds an audit entry (SPEC-010 AC).
- [x] AC-24a — `rules.Modules` contains `office`; a rule that reads an office record type is accepted only in the `office` module and an `office` rule may also read built-in record types; the Rule Builder's module list, the office rules folder (`<tax-year>/office/<ID>`) and the Rules tab filter show it; the seven existing modules and every built-in rule are unchanged.
- [x] AC-24 — The Rule Builder groups inputs as Built-in and Office documents, shows column labels and types, offers an office input only for row-reading check types, and says how to add a document when none is ready; the Rules tab shows the "Office document" chip and its filter; the existing Rule Builder and Rules tests pass unchanged.

**Both parts**

- [x] AC-25 — `gofmt -l .` prints nothing; `go vet ./...` (host and Windows), `go test -count=1 ./...`, the architecture guard, Test Pack A; in `web/`: `npm run lint`, `npm test`, `npm run build` pass, except failures listed as already present before this spec.
- [x] AC-26 — Every file this spec created is added to `docs/Tool1-File-Guide.docx`; `tool1-technical-spec.md` and `docs/contracts/data-model.md` describe the new tables, the two-layer vocabulary and the run snapshot.

## Test cases

**Go (`internal/vocab`, `internal/store`, `internal/rules`, `cmd/recon-engine`, `internal/api`)**
- `WithOffice`: merge keeps base order and hash; collision with a base name, prefix or another office name is refused; an office document with no ready record type is left out; the base `Vocab` is not mutated; a built-in rule loads and evaluates the same against base and merged.
- `OfficeHash`: stable across insertion order (20 runs), changes on a column rename or type change, ignores `active`, `""` when empty. Snapshot written once per hash and refuses update and delete.
- Validation table: every rule in "Validation" with a pass and a fail case, including boundary lengths, a key that is an amount, no key, only keys.
- Copy independence: copy, then edit each side in turn.
- Locking: each refused and each allowed edit on a record type used by a format, by `records` rows and by an office rule version.
- Delete: unused, used by a format, used by records only, used by a rule only, built-in.
- Migration: from a populated `0014` database; the `document_type` fill against profiles with and without the line; idempotence.
- Engine end to end: teach a layout on a new office document from a synthetic JSON, CSV and Excel file; run; compare with the expected rows and locators.
- Rules end to end (synthetic data only): office versus office and office versus built-in `match_records` with a matching, an amount-differing, a missing-on-one-side and a duplicate case; `sum_equals` per row and totals; `present_together`; each with pass and fail fixtures; testing, active and disabled states; versions and switching back.
- Reproducibility: the run, then change everything in AC-21, then compare the stored report and a re-run.
- API: the permission for each endpoint and role, `409 IN_USE`, validate without saving, the `file_type` filter, the unchanged rule endpoints.
- Regression: Test Pack A (Pass 1), the negative controls and one-defect-at-a-time (SPEC-007), `TestWebSamplesMatchEngine`, the architecture guard, `rule-lint`.

**Web (Vitest)**
- Tab visible to the Partner only; list, search, filters and sort; row actions per origin and status.
- Add drawer: three steps, copy and define-new paths, formats checkboxes, every validation message beside its field, Draft save, keyboard use.
- Edit with a locked record type disables the forbidden controls and says why.
- Delete confirmation and the in-use message listing formats and rules.
- Layout wizard: the Document list comes from the server and is filtered by the uploaded file's format; a new document appears; labels and the date checkbox for office columns; the empty-list message; existing wizard tests unchanged.
- Rule Builder: office inputs grouped and labelled; column pickers and fixture editor use office columns; unsupported check types hidden for office inputs; empty-list message. Rules tab chip and filter.

**Manual**
- The Partner adds "Sales return register" (XLSX and CSV) with new columns, teaches a layout from a synthetic file; a Staff user drafts a `match_records` rule against GSTR-1 rows; the Partner approves it; a run reports a finding; the Partner then deactivates the document and an old comparison still opens exactly as it was; a Manager cannot see the Document Master tab.

## Delivery plan

| Part | Content | Estimate |
|---|---|---|
| A1 | Migration, store, `vocab.WithOffice`, `OfficeHash`, snapshots, shared checker refactor, the risk checks (including the row-reading check types) | 2.5 days |
| A2 | Engine commands, `loadVocab` helper, `teachableTypes` and run integration, audit | 1.5 days |
| A3 | API endpoints, `manage_documents`, tests | 1 day |
| A4 | Document Master screen and wizard changes | 2 days |
| A5 | Part A regression, sample regeneration, review fixes | 0.5 day |
| B1 | Merged vocabulary into the rule loader, `load-rules`, `run`, Rule Builder save and tests; fixtures over office types; locking by rule references | 2 days |
| B2 | Reproducibility: snapshot on run, comparison history reads it, tests for AC-21 | 1.5 days |
| B3 | Rule Builder and Rules tab changes | 1.5 days |
| B4 | Rules end-to-end tests, regression, docs, file guide, review fixes | 2 days |
| | **Total** | **about 14.5 working days** |

Part A is releasable on its own after A5 (about 7.5 days). Part B should start only after A5, because every Part B test needs a working Document Master. If the second release must carry something earlier, Part A is the part that fits first.

## Decisions on the earlier open questions (5 November 2026)

1. Row-based rules are enough for the first version; office facts are a separate future spec (decision 4).
2. Rules over office documents use a new `office` module in the existing Rule Builder (decision 5).
3. Deactivation shows a warning and is allowed; it never changes a rule (decision 6).

No open questions remain.

## Part A build record (6 November 2026)

What was built, and the decisions taken where the spec left room. Part B and AC-15 are open.

- **Where it is.** `0015_document_master.sql`, `internal/store/docmaster.go` (documents, record types, usage, snapshots, `DocFormatFileTypes`), `internal/vocab/office.go` and `officecheck.go` (`WithOffice`, `OfficeHash`, `ValidateOfficeDocument`, `ParseOfficeSnapshot`) with the shared `checkRecordType` in `load.go`, `cmd/recon-engine/docmaster.go` (`loadVocab`, the six `document-*` commands, `seedBuiltins`), `internal/api/documents.go`, `PermManageDocuments`, and in `web/src` the Document Master tab (`screens/DocumentMaster.tsx`, `DocumentForm.tsx`, `docMasterTypes.ts`, `styles/documents.css`) and the wizard changes in `TeachLayout.tsx`.
- **Labels are part of the office hash.** The spec lists names, types and flags; AC-21 needs an old comparison to show its labels from the snapshot, so a label change must make a new snapshot.
- **Two name spaces.** Document codes are compared with base document names, base prefixes and other codes; record type names with base and other record types. A code may equal a record type name. Names are compared ignoring capitals.
- **A Draft keeps no structure.** A document is ready when it has a record type that passes the shared checker. A structure that is not valid saves the document as a Draft with `record_type = ''` and stores no columns; the screen keeps the Partner's work until it is valid. A problem in the document's own fields (code, name, description, category, formats) refuses the save (`INVALID_DOCUMENT`, 422, every problem with its field). A ready document is never saved with a structure that is not valid.
- **Copying.** The copy's columns are filled in by `document-validate` (and so by `POST /document-types/validate`) from `copy_from`; saving stores them with `copied_from` for information only.
- **Seeding.** The built-in documents are filled by `load-rules` (the server runs it at every start) and by every `document-*` command, from the base vocabulary. Their categories come from a small table in `cmd/recon-engine/docmaster.go` (not in the vocabulary, so the content hash does not move); a built-in with no row there is "Other". They accept all three formats.
- **Not enforced yet: `required`.** A column's required flag is stored, shown in the wizard and hashed, but `profile-create` does not refuse a mapping that leaves a required (non-key) column out. Keys are enforced as before. Decide whether to enforce it before Part B.
- **The `comparisons` command** still reads the base vocabulary only: it has no `--db`, and the comparison types name built-in documents.
- **Two "never delete" tests now allow one exception each:** `TestNoCommandDeletesAUser` allows the command `document-delete`, and `TestRoutesAreOnlyRegisteredFromTheDeclaredList` allows `DELETE /api/v1/document-types/…`. Both remove only an unused office document definition. The cross-site guard now checks Origin and Sec-Fetch-Site on `DELETE` too.
- **Verification run.** Every `testdata/run/*` folder gives the same exit code, messages and report with an empty Master and with office documents present (`TestEveryRunFolderSameWithAndWithoutOfficeDocuments`); the seven sample cases and the rules list are byte-identical to the committed samples with office documents present; Test Pack A scores 22 of 30, critical 4 of 4, 0 false positives; `rule-lint` passes 69 rules and `rulestore-check` finds 0 problems. The risk-1 check types are tested over office columns in `internal/engine/checks/office_test.go`, and a document taught from a JSON, CSV and Excel file is read, run and refused on an empty Master in `TestOfficeDocumentTaughtFromJSONCSVAndExcel`.
- **AC-15** is covered by Vitest (list, filters, sort, three steps, locked controls, warnings, keyboard submit) but has not been looked at in a browser.

## Part B build record (6 November 2026)

- **Where it is.** `rules.Modules` has `office`; the loader (`internal/rules/rule.go`, `officeInputs`) refuses an office record type in any other module and under a check type that does not read rows (`rules.ReadsRows`, taken from what each check type implements, so the list is not copied); the Rule Builder's own checks (`internal/rules/builder.go`, `officeProblems`) give each such mistake, and a column that is not there or is an amount where text is needed (or the reverse), on the field of the form. `loadRuleEnv` and `load-rules` read the merged vocabulary (office rules only, in `load-rules`); `rule-options` returns each record type's origin, whether its document is active, its labelled columns and the row-reading check types; `rules` marks each rule that reads an office document (`reads_office_documents`); `history --no` returns the office vocabulary the run used (`office_vocab`, `office_vocab_hash`). In the screens: the Rule Builder groups record types as Built-in and Office documents, shows an office record type's columns with their labels, puts the rule in the office module, offers only the check types that read rows and shows labels in the fixture editor; the Rules tab has the Office document chip and a Reads filter.
- **A bug found and fixed on the way:** the usage of a record type by rules (`RecordTypeUsage`) only matched an input written on a line of its own, but the Rule Builder writes it inline (`a: { record_type: "...", label: "..." }`), so a saved office rule would not have locked or protected its document. Both forms are read now (`TestUsageReadsInlineInputs`).
- **Column checks are only for office inputs.** The builder's column problems are not applied to built-in record types, so a built-in rule's form is checked exactly as before; the loader still has the last word for every rule.
- **A check type already chosen stays visible.** When an office record type is chosen and the form holds a check type that cannot read rows, that type stays in the list, marked and not choosable, so the form never shows a type it does not hold.
- **Evidence.** AC-17: `TestOfficeRuleLifecycle` (sum_equals), `TestMatchRecordsRuleBetweenOfficeAndBuiltInRows` (office with the sales register), `TestOfficeRulesThroughServer` (present_together, drafted by Staff and Manager). AC-18: `TestOfficeRuleRefusals` (engine), `TestOfficeRulesRefusals` (`internal/engine/checks`), `TestOfficeRuleAmountAndTextColumnsAreNotMixedUp`, `TestOfficeRuleOverADraftRecordType`. AC-19, AC-20: `TestOfficeRuleLifecycle`. AC-21: `TestEarlierComparisonOpensAsReported`. AC-22: `TestBuiltinRulesUnchangedWithOfficeRulesPresent` and `TestOfficeDocumentsLeaveBuiltinOutputUnchanged`. AC-23: `TestOfficeRulesThroughServer`. AC-24a, AC-24: the engine and web tests above, `officeRules.test.tsx`, and the existing Rule Builder and Rules tests, which pass unchanged.
- **AC-25 and the CRLF files.** `gofmt -l .` is not empty in this working tree: it lists 25 files, none created by this spec, all because their working copies have CRLF line endings (their committed versions are formatted, and with the carriage returns removed gofmt has nothing to change). One real slip of this spec's own (a missing space in `internal/store/store_test.go`) was found by that gate and fixed. Every other gate passed: `go vet` on the host and Windows, `go test -count=1 ./...` (31 packages), the architecture guard, Test Pack A (22 of 30, critical 4 of 4, 0 false positives), `rule-lint` (69 rules), `rulestore-check` (0 problems), `profile-test` (126 samples), `npm run lint`, `npm test` (324 passed, 3 skipped), `npm run build`, `go test -tags ui ./web/` and the Windows build with the UI.
- **Not done, and what that leaves.** The spec's manual test (the Partner adds a document, teaches a layout, a Staff user drafts a rule, the Partner approves it, a run reports a finding, the document is deactivated, an old comparison still opens, a Manager does not see the tab) has been covered by automated tests but not walked through in a browser. AC-15 stays open for that reason. The `required` flag of a column is stored, shown and hashed but not enforced when a layout is taught (see the Part A record).

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
