# SPEC-004 — Fact vocabulary

| | |
|---|---|
| **Status** | Done |
| **Phase** | 1 |
| **Depends on** | SPEC-003 |
| **Requirements** | FV-01, FV-02, FV-03, FV-04 |
| **Technical spec** | §A2, §B3 |
| **Code / content paths** | `internal/vocab`, `internal/strictyaml`, `cmd/recon-dev` (`vocab-doc`), `content/vocabulary/facts.yaml`, `content/vocabulary/records.yaml`, `docs/fact-vocabulary.md` (generated) |

## Goal

There is one controlled list of fact and record names. Profiles and rules are validated against it, and the human-readable document is generated from it.

## Scope

As built (the original draft is superseded where it differs):

- **`internal/strictyaml`** (shared with SPEC-006, 007 and 013): `Decode(file, data, &struct)` on `go.yaml.in/yaml/v3` v3.0.4, the maintained continuation of the archived `gopkg.in/yaml.v3`, with the same code and API. It is the first outside dependency, and it passes the engine network guard.
  - **Refuses:** unknown keys, duplicate keys, unquoted text (every Go `string` field), anything but plain `true`/`false` for a yes/no field, numbers not written in plain digits, anchors, aliases, merge keys, tags, and more than one document per file.
  - **Errors:** every problem is reported at once, as `strictyaml.Errors` with file and line. `Positions` lets loaders point their own checks at a line.
- **`content/vocabulary/facts.yaml`** (`schema: "recon.vocab.facts/1"`, `vocab_version`):
  - **`document_types`:** `name`, `prefix`, `description`, `since_vocab`. Every fact name starts with its document type's `prefix` (`tax_audit_report` → `tar.`).
  - **`facts`:** `name`, `kind` (`money|int|text|date|bool|rate`), `periodicity` (`FY|quarter|month`), `document_type`, `description`, `since_vocab`, optional `deprecated_in`.
- **`content/vocabulary/records.yaml`** (`schema: "recon.vocab.records/1"`, same `vocab_version`):
  - **`record_types`:** `name`, `description`, `match_key`, `amounts`, `fields`, `since_vocab`, optional `deprecated_in`.
  - **Rules:** each `match_key` entry must be one of the record's `fields`, and a column may sit in `amounts` or `fields`, not both.
  - **Shipped empty:** GST, TDS and payroll rows arrive in SPEC-031..033, Tally rows in SPEC-020.
- **Naming:** facts match `^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+$`. Document types, prefixes, record types and columns match `^[a-z][a-z0-9_]*$`. Descriptions are required and fit on one line.
- **`Vocab` API:** `Version()`, `ContentHash()`, `Lookup(name) (Fact, bool)`, `IsFact`, `RecordType(name)`, `IsRecordType`, `DocumentType(name)`, `DocumentTypes()`, `Facts()`, `RecordTypes()`. Every list is in name order, and returned slices are copies. `LoadDir(dir)` and `Load(...)` report all problems in both files, each with file and line.
- **Generator:** `go run ./cmd/recon-dev vocab-doc` writes `docs/fact-vocabulary.md`, grouped by document type and showing the version and a content hash.
  - `CheckDoc` / `CheckVersionBumped` refuse a content change whose version did not increase.
  - `TestCommittedDocIsUpToDate` fails when the committed document differs from the generated one (CRLF line endings are ignored).
- **Initial entries:** document types `computation` and `tax_audit_report`, and the three SPEC-011 depreciation facts.
- **Added to `internal/domain`:** `ParseValueKind`.

## Out of scope

- Facts for GST/TDS/payroll (added by SPEC-031..033), Tally records (SPEC-020).

## Design notes

- A name is never reused with a different meaning. Changing a meaning means a new name, and the old one gets `deprecated_in`.
- The vocabulary version is an integer that increases on every change, and is recorded in each run (§A10). The content hash in the generated document enforces this: it hashes meaning, not file bytes, so comments and entry order are not changes.
- `internal/strictyaml` is listed in the archtest `engineSidePatterns` and `deterministicDirs`. Technical spec §A4, §B3 and §B4.1, and SPEC-007 (which reuses the helper), were updated in the same change.

## Acceptance criteria

- [x] AC-1 — Loader rejects: duplicate names, invalid names, unknown keys, unknown `kind`, unquoted text fields, a record type whose `match_key` names a field it does not declare.
- [x] AC-2 — Loader accepts the shipped `facts.yaml` and `records.yaml`.
- [x] AC-3 — `docs/fact-vocabulary.md` is generated, and a CI test fails if the committed file differs from the generated output.
- [x] AC-4 — The vocabulary version is read and exposed by `Vocab.Version()`.
- [x] AC-5 — The SPEC-011 facts `tar.depreciation_schedule.total`, `computation.allowed_depreciation` and `computation.fixed_asset_schedule.total` are present.

## Verification

```
go test ./internal/vocab/ -v
go run ./cmd/recon-dev vocab-doc && git diff --exit-code docs/fact-vocabulary.md
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| 13 Oct 2026 | Claude Code, for the project owner | not yet committed | AC-1: `TestLoadRefuses` (25 cases at exact file:line, including duplicates, bad and non-ASCII names, wrong prefix, unknown keys/kind/periodicity/document type, unquoted text, version errors, `match_key` naming a missing field or an amount) plus `strictyaml` tests (27 refusal cases). AC-2, AC-4, AC-5: `TestShippedVocabulary`. AC-3: `TestCommittedDocIsUpToDate`, proven by break-and-restore. A description edit without a version increase failed the test and the generator; raising the version in one file only was refused; raising it in both regenerated the document. All files were then restored. Full suite, `gofmt -l`, `go vet` (host and Windows) and the Windows build pass. File Guide section 8 added. |
