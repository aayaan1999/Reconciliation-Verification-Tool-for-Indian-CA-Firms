# SPEC-007 — Rule format and strict loader

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 1 |
| **Depends on** | SPEC-004 |
| **Requirements** | RU-01, RU-02, RU-03, RU-30, RU-43, FV-03 |
| **Technical spec** | §B4.1, §B4.2 |
| **Code / content paths** | `internal/rules`, `content/rules/<tax-year>/<module>/<RULE-ID>/`, `cmd/recon-dev` (`rule-lint`) |

## Goal

Rules are YAML files that a CA can read, loaded through a strict parser that refuses anything ambiguous, and validated before they can run.

## Scope

- Rule schema `recon.rule/1` exactly as §B4.2: `id`, `version`, `source`, `module`, `category`, `title`, `rule_text_en`, `notes`, `law[]`, `applies`, `inputs`, `check`, `severity`, `review_document`.
- Strict YAML (§B4.1): `gopkg.in/yaml.v3` with `KnownFields(true)`; walk the node tree and reject unquoted scalars in text fields (IDs, versions, tax years, dates, sections); reject anchors, aliases, merge keys, custom tags and multi-document files.
- Validation:
  - `law` has at least one entry with `act` and `section` (RU-30);
  - every input names a vocabulary fact or record type (FV-03);
  - `check.type` exists in the check-type registry and its params pass `ValidateParams` (registry interface from SPEC-008);
  - `review_document` is a vocabulary document type;
  - `title` passes the banned-word test of SPEC-009.
- Fixture format per rule: `fixtures/pass/*.yaml` and `fixtures/fail/*.yaml`, each holding input facts and the expected evaluation(s).
- `recon-dev rule-lint <dir>`: loads every rule, checks that each has at least one pass and one fail fixture (RU-43), and prints errors with file and line.
- Shared strict-YAML helper used by vocabulary, profiles, law tables and rules.

## Out of scope

- Lifecycle status, approvals and office versions (stored in the DB; SPEC-027).
- Running fixtures (SPEC-008).

## Acceptance criteria

- [ ] AC-1 — The §B4.2 example rule loads.
- [ ] AC-2 — Each of these fails with file:line and a plain message: `tax_years: [2025-26]`; `id: RECON-DEP-002` unquoted is accepted only if it is a string, and `version: 1` unquoted is rejected; an unknown key; an anchor `&a`; two documents in one file; missing `law`; an input name not in the vocabulary; an unknown check type.
- [ ] AC-3 — `yes`, `no`, `on`, `off` are never read as booleans.
- [ ] AC-4 — `rule-lint` fails a rule that has no `fail` fixture.
- [ ] AC-5 — The YAML library passes the engine archtest.

## Verification

```
go test ./internal/rules/ -v
go run ./cmd/recon-dev rule-lint ./content/rules/...
go test ./internal/archtest/
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
