# SPEC-018 — Excel and CSV adapter

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 3 |
| **Depends on** | SPEC-006 |
| **Requirements** | IN-02, IN-06, IN-20..23 |
| **Technical spec** | §A6.1, §B2.2, §B17 (T1) |
| **Code / content paths** | `internal/ingest/adapters/xlsx`, `internal/ingest/adapters/csv`, Excel/CSV profile predicates in `internal/profile` |

## Goal

Profiles can read Excel workbooks and CSV files by finding columns by their header text, so a moved or inserted column never shifts values silently.

## Scope

- **First task:** confirm that `excelize` passes the engine archtest. If it does not, record the alternative chosen in `docs/adr/` before building.
- `.xlsx` via excelize: read cell *values as stored text*, never through float conversion. Dates are read using the cell's number format and converted to ISO dates.
- `.xls` (old binary format) needs a decision: find a reader that passes the archtest, or reject `.xls` with "save as .xlsx" guidance. Record the decision.
- `.csv`: the profile declares the delimiter, quote character, encoding (`utf-8`, `utf-8-bom`, `windows-1252`) and header row. Nothing is guessed.
- Selectors: `sheet`, `header_row_contains`, `header_labels` (label → field), `row_label` (for statement-style sheets), `stop_at_first_blank_row`, `skip_rows_matching`.
- Detect predicates: `sheet_exists`, `header_row_contains`, `cell_equals`.
- Locators: `Sheet!D42` for xlsx; `line 42 col "Debit"` for csv.
- One synthetic Excel profile (a working schedule) and one CSV profile, each with samples and expected facts.

## Out of scope

- Real staff schedules (blocked on Q2). Tally Excel exports use this adapter, with Tally profiles from SPEC-020.

## Acceptance criteria

- [ ] AC-1 — The engine archtest passes with the chosen Excel library.
- [ ] AC-2 — Moving a column in a sample sheet does not change the extracted facts; renaming its header produces a `REQUIRED_FIELD_MISSING` error.
- [ ] AC-3 — A cell holding `112450.1` that Excel displays as `1,12,450.10` is read as exactly 11245010 paise.
- [ ] AC-4 — Formula cells are read as their cached value; a formula with no cached value is reported, not computed.
- [ ] AC-5 — The `.xls` decision is recorded and implemented (a reader, or a clear rejection message).
- [ ] AC-6 — A CSV with a declared encoding reads non-ASCII ledger names correctly.
- [ ] AC-7 — Profile tests pass for the synthetic Excel and CSV profiles.

## Verification

```
go test ./internal/ingest/adapters/xlsx/ ./internal/ingest/adapters/csv/ -v
go run ./cmd/recon-dev profile-test ./content/profiles/...
go test ./internal/archtest/
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
