# SPEC-005 — Ingestion framework and JSON adapter

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 1 |
| **Depends on** | SPEC-003 |
| **Requirements** | IN-01, IN-14, IN-21, IN-23, SC-01 |
| **Technical spec** | §A6.1, §B2.1 |
| **Code / content paths** | `internal/ingest`, `internal/ingest/adapters/json` |

## Goal

Any file can be handed to the ingestion layer, which picks the adapter for its file type and gives profiles a navigable document. JSON is the first adapter.

## Scope

- `Adapter`, `Doc`, `Selector`, `Cell` interfaces (§A6.1).
- Adapter registry: registration at start-up, lookup by file type, iteration in sorted order.
- `ValidationReport` type: recognised profile, tax year, client match, counts of facts/records, list of unread items, warnings, errors. It is filled by the profile executor (SPEC-006).
- Error codes: `UNSUPPORTED_FILE_TYPE`, `PARSE_ERROR`, `PROFILE_NO_MATCH`, `PROFILE_AMBIGUOUS`, `REQUIRED_FIELD_MISSING`, `CONFLICTING_VALUES`.
- JSON adapter:
  - decodes with `json.Decoder.UseNumber()`, so numbers are never parsed as floats;
  - supports a documented JSON-path subset: `$`, `.key`, `['key with spaces']`, `[n]`, `[*]` (for record rows);
  - every returned `Cell` carries its exact path for the locator;
  - rejects files over a configurable size limit with a clear error.

## Out of scope

- Detection logic and fact mapping (SPEC-006).
- Other adapters (SPEC-018, 019, 020, 030).

## Design notes

- Adapters know file *syntax* only. An adapter never produces a fact and never contains a figure name.
- Document the supported JSON-path subset in the package doc; anything outside it is a profile load error, not a run-time surprise.

## Acceptance criteria

- [ ] AC-1 — Registry returns adapters in sorted order and rejects duplicate registration.
- [ ] AC-2 — JSON adapter: a number like `112450.50` is returned as the exact text, never via `float64` (test with a value that a float cannot represent exactly, e.g. `90071992547409.93`).
- [ ] AC-3 — All path forms in the subset work; unsupported syntax returns an error naming the path.
- [ ] AC-4 — Every `Cell` has a locator of kind `json_path` with the full concrete path (e.g. `$.rows[3].amount`).
- [ ] AC-5 — Malformed JSON returns `PARSE_ERROR` with line/offset.
- [ ] AC-6 — The engine archtest still passes.

## Verification

```
go test ./internal/ingest/... -v
go test ./internal/archtest/
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
