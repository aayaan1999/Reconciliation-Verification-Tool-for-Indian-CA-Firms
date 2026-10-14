# SPEC-001 — Convert the rule book into the YAML rule store

| | |
|---|---|
| **Status** | Done |
| **Requirements** | RU-01, RU-02, RU-03, RU-30 |
| **Technical spec** | §B4.1, §B4.2 |
| **Paths** | `docs/Recon_Tool_YAML_Rule_Store/recon_yaml/`, `cmd/recon-dev` (`rulestore-import`, `rulestore-check`), `internal/strictyaml` |

## Goal

Every rule in `docs/Recon_Tool_Rulebook_v2.xlsx` is a YAML file of one common structure, grouped by type and domain, and passes the strict-YAML rule RU-02 (all text quoted).

## Scope

- `RULE_TEMPLATE.yaml` and `DOCUMENT_TEMPLATE.yaml`: the annotated common structure.
- `rules/reconciliation/<domain>/RC-*.yaml` (90 rules) and `rules/statutory/<domain>/*.yaml` (214 rules); `rules/index.yaml`.
- `documents/<DOC_CODE>.yaml` (63 document types with their fields); `documents/index.yaml`.
- Go only, no Python: `go run ./cmd/recon-dev rulestore-import` writes the store from the workbook, and `go run ./cmd/recon-dev rulestore-check` validates it (replacing `excel_to_yaml.py`, `validate.py` and the two JSON schemas, which are removed).
- Every text value is quoted (RU-02), including `id`, `version`, dates, codes and parameter values, so every file loads through `internal/strictyaml`. An empty value is `""`, never `null`; long text is a `|` block, which the strict reader now accepts as text. Parameter values, `min` and `max` are text read by their declared `type`, compared as exact decimals (no floats). A document field's `required` is `"yes"`, `"no"` or `""`.

## Acceptance criteria

- [x] AC-1 — Every workbook rule is one YAML file named after its rule ID, in `rules/<type>/<domain>/`.
- [x] AC-2 — Every file follows `RULE_TEMPLATE.yaml`; sections that do not apply are present and empty.
- [x] AC-3 — All rules are imported with `governance.enabled: false`; a rule can be enabled only with a CA reviewer, review date and approved decision.
- [x] AC-4 — `go run ./cmd/recon-dev rulestore-check` passes: 304 rules, 63 documents, 0 problems.
- [x] AC-5 — Every text value is quoted, so each rule and document file, and both templates, decode through `internal/strictyaml` (RU-02).
- [x] AC-6 — The Go importer loses no text: every file it writes equals the earlier Python output except for quoting and `null` → `""`.

## Verification

```
go run ./cmd/recon-dev rulestore-import
go run ./cmd/recon-dev rulestore-check
go test -count=1 ./cmd/recon-dev/
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| 14 Oct 2026 | Project owner | not yet committed | 304 rule files (90 reconciliation, 214 statutory) and 63 document files generated from the workbook. validate.py not yet run: pyyaml missing. Re-quoting for RU-02 open. |
| 14 Oct 2026 | Claude Code, on the project owner's instruction | not yet committed | Python replaced by Go: `rulestore-import` (ports excel_to_yaml.py) and `rulestore-check` (ports validate.py and both JSON schemas, adding folder and document file-name checks). Store regenerated with every text value quoted; `rulestore-check`: 304 rules, 63 documents, 0 problems. All 369 files (rules, documents, two indexes) compared with the earlier Python output: 0 differ apart from quoting and null → "". Each written file is read back through `internal/strictyaml` and compared before it is saved. Templates quoted and loading strictly. `internal/strictyaml` now accepts `|`/`>` block scalars as text. Tests in `cmd/recon-dev/rulestore_test.go` (shipped store, templates, five refusals, parameter checks, importer helpers) run in CI with `go test`. tools/*.py and schema/*.json removed. |
