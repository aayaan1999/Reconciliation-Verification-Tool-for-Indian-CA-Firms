# SPEC-004 — Fact vocabulary

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 1 |
| **Depends on** | SPEC-003 |
| **Requirements** | FV-01, FV-02, FV-03, FV-04 |
| **Technical spec** | §A2, §B3 |
| **Code / content paths** | `internal/vocab`, `content/vocabulary/facts.yaml`, `content/vocabulary/records.yaml`, `docs/fact-vocabulary.md` (generated) |

## Goal

There is one controlled list of fact and record names. Profiles and rules are validated against it, and the human-readable document is generated from it.

## Scope

- YAML schemas for fact entries (`name`, `kind`, `periodicity`, `document_type`, `description`, `since_vocab`, optional `deprecated_in`) and record types (`name`, `match_key`, `amounts`, `fields`).
- Loader using the strict YAML rules shared with SPEC-007 (unknown keys rejected, text fields quoted).
- Naming check: `^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+$`, ASCII only.
- `Vocab` API: `Lookup(name)`, `IsFact`, `IsRecordType`, `DocumentTypes()`, `Version()`.
- Document-type list (e.g. `computation`, `tax_audit_report`, `trial_balance`, `gstr1`, …) is part of the vocabulary.
- Generator: `go run ./cmd/recon-dev vocab-doc` writes `docs/fact-vocabulary.md`, grouped by document type.
- Initial entries: enough for SPEC-011 (depreciation facts in the computation and the tax audit report) plus the document types used by synthetic fixtures.

## Out of scope

- Facts for GST/TDS/payroll (added by SPEC-031..033), Tally records (SPEC-020).

## Design notes

- A name is never reused with a different meaning. Changing a meaning means a new name, and the old one gets `deprecated_in`.
- The vocabulary version is an integer that increases on every change, and is recorded in each run (§A10).

## Acceptance criteria

- [ ] AC-1 — Loader rejects: duplicate names, invalid names, unknown keys, unknown `kind`, unquoted text fields, a record type whose `match_key` names a field it does not declare.
- [ ] AC-2 — Loader accepts the shipped `facts.yaml` and `records.yaml`.
- [ ] AC-3 — `docs/fact-vocabulary.md` is generated, and a CI test fails if the committed file differs from the generated output.
- [ ] AC-4 — The vocabulary version is read and exposed by `Vocab.Version()`.
- [ ] AC-5 — The SPEC-011 facts `tar.depreciation_schedule.total`, `computation.allowed_depreciation` and `computation.fixed_asset_schedule.total` are present.

## Verification

```
go test ./internal/vocab/ -v
go run ./cmd/recon-dev vocab-doc && git diff --exit-code docs/fact-vocabulary.md
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
