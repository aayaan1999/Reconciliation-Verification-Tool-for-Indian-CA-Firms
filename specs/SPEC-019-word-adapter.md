# SPEC-019 — Word (.docx) table adapter

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 3 |
| **Depends on** | SPEC-006 |
| **Requirements** | IN-03, A4 |
| **Technical spec** | §A6.1, §B2.2 |
| **Code / content paths** | `internal/ingest/adapters/docx` |

## Goal

Profiles can read figures from tables inside Word documents. Free-running text is never interpreted.

## Scope

- Read `.docx` with the standard library (`archive/zip` + `encoding/xml`) over `word/document.xml`. Use no third-party library unless this proves insufficient, and record the reason in `docs/adr/` if so.
- Extract tables (`w:tbl`) in document order, including nested tables. Resolve merged cells (`w:gridSpan`, `w:vMerge`) so that each logical cell has a row and column index.
- Cell text is the concatenated runs, with whitespace normalised.
- Selectors: `table_index`, `table_anchor` (text in the first row, or in the paragraph immediately before the table), `row_label`, `column_header`.
- Detect predicates: `docx_table_anchor_exists`, `docx_heading_contains`. Headings may be used for detection, never for figures.
- Locator: `T3!R7C4`, plus the anchor text for display.
- Old `.doc` files are rejected with "save as .docx" guidance.

## Out of scope

- Reading figures from paragraphs (A4). Real staff documents (blocked on Q2).

## Acceptance criteria

- [ ] AC-1 — A synthetic document with three tables, one of them with merged header cells, extracts the expected facts.
- [ ] AC-2 — A figure that appears only in paragraph text is never extracted.
- [ ] AC-3 — Inserting a new table before the target table does not change the results when `table_anchor` is used.
- [ ] AC-4 — `.doc` input gives a clear rejection message.
- [ ] AC-5 — The engine archtest passes.

## Verification

```
go test ./internal/ingest/adapters/docx/ -v
go run ./cmd/recon-dev profile-test ./content/profiles/...
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
