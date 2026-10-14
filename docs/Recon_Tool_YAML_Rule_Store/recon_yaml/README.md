# Recon Tool – YAML rule store

Generated 14 Oct 2026 from `Recon_Tool_Rulebook_v2.xlsx` (research date 13 Oct 2026).

## Folder layout

```
RULE_TEMPLATE.yaml          annotated structure every rule follows (read this first)
DOCUMENT_TEMPLATE.yaml      annotated structure every document definition follows
rules/
  index.yaml                list of all rules (id, type, domain, documents, status, file)
  reconciliation/<domain>/RC-*.yaml      90 document-vs-document rules
  statutory/<domain>/*.yaml              214 law / rate / computation rules
documents/
  index.yaml                list of all document types
  <DOC_CODE>.yaml           63 document types with their fields
```

Every text value is in double quotes (requirement RU-02), so every file loads through the
tool's strict YAML reader. Parameter values, `min` and `max` are quoted text too; the
parameter's `type` says how the value is read. An empty value is `""`, never `null`.

## Commands (Go only; run from the repository root)

```
go run ./cmd/recon-dev rulestore-import     # rewrite every file from docs/Recon_Tool_Rulebook_v2.xlsx
go run ./cmd/recon-dev rulestore-check      # check every file; also runs in CI through go test
```

`rulestore-check` checks: every file loads strictly (all text quoted, no unknown keys);
file name = rule id and the rule sits in `rules/<type>/<domain>/`; no duplicate ids; every
document a rule uses exists; every `DOC.field` in keys/comparisons is defined in that
document; outcome codes and parameter keys are unique; parameter values match their
declared type and min/max (exact decimals, no floating point); a rule cannot be
`enabled: true` without a CA reviewer, review date and an approved decision.

## Points to know

* All rules are imported with `enabled: false`. Switch a rule on only after CA sign-off.
* `valid_from` is filled for reconciliation rules; statutory rules keep their effective-date
  wording in `validity.effective_text` and need a CA to set `valid_from` / `valid_to`.
* `implementation.handler` is empty – developers assign the function that runs each rule.
* Some fields in `documents/` are marked `origin: new (design)`: rules need them but they were
  not confirmed in a real file. Check them against the first real downloads.
* When the law changes: move the current file to `rules/_history/<ID>.v<version>.yaml` after setting
  its `valid_to`; then edit the live file, raise `version` and set the new `valid_from`. The live
  file always holds the latest version; `_history` keeps earlier ones for audit.
* Exception records (the results of a run) are data, not rules – store them in the app's
  database, not in YAML.
