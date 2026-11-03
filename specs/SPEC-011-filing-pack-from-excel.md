# SPEC-011 — Tax suite filing pack read from Excel and CSV

| | |
|---|---|
| **Status** | In progress |
| **Phase** | 3 |
| **Depends on** | SPEC-008, SPEC-009 |
| **Requirements** | IN-02, IN-04, IN-22, IN-23, NF-01, NF-03 |
| **Technical spec** | §A6.1, §A6.2, §B2.2 |
| **Code / content paths** | `internal/ingest/adapters/json/tree.go`, `internal/ingest/adapter.go` (`Flattened`), `internal/ingest/adapters/csv`, `internal/ingest/adapters/xlsx/table.go` (`JSONTree`), `internal/profile` (`reads`, `read`, `relocate`), `testdata/manual_filing_pack/`, `cmd/recon-engine/flatpack_test.go` |

## Goal

A CA can upload the tax suite filing pack as one Excel workbook (`.xlsx`) or one CSV file instead of JSON, under "Tax suite filing pack: audit report, Form 29B, Form 67, Form 26AS, computation and accounts", and get exactly the same checks and findings as from the JSON pack. Each figure's source is its cell ("Filing pack!B271") or its CSV line ("line 271").

## The layout

The pack is the JSON pack flattened into two columns, one value per row:

| Path | Value |
|---|---|
| `$.assessee.pan` | `AABCN4821K` |
| `$.form_3cd.clause_18_depreciation.total.depreciation_allowable` | `135862` |
| `$.form_67[0].country` | `…` |

- **CSV:** the first row is exactly `Path,Value`.
- **Excel:** one sheet, of any name, whose header row holds `Path` and `Value` (capitals do not matter; other columns are ignored).
- **Path** is the value's JSON path in the pack, in the syntax profiles use (`$`, `.key`, `['key']`, `[n]`). List items are numbered from 0 in order.
- **Value** is read as JSON would write it: `true` and `false` are yes/no, `null` is no value, `[]` and `{}` an empty list or object, a number is a number, and anything else is text. An empty value is empty text (a blank PAN or UDIN stays blank). In Excel, number and yes/no cells keep their type.
- A path given twice, a value inside something another row gives as a single value, a skipped list number or a path with `[*]` is refused (`PARSE_ERROR`), naming the line or cell: a figure is never read from one of two conflicting copies.

This is a synthetic stand-in. When the firm's tax suite gives a real Excel or CSV export, its own profiles are added for it.

## How it works (as built, 29 October 2026)

- No new or copied format files. The six filing-pack formats (`content/profiles/taxsuite_pack/`, unchanged) read the flattened file as the JSON document it describes.
- `jsonadapter.BuildTree` rebuilds the document from the rows and remembers each value's place in the file.
- The CSV and Excel readers implement `ingest.Flattened` (`JSONTree`). CSV collects the rows when the header is `Path,Value`; Excel reads the sheet through the existing table reader (SPEC-008), so the header row may sit anywhere and the sheet may have any name.
- A JSON profile also tries a CSV or Excel file that holds such a table (`Profile.reads`). Every other profile is unaffected: a file without the table is not a flattened document, and a CSV or Excel profile still reads only its own tables.
- After reading, every locator is turned from the JSON path into the cell or line (`relocate`, shared with the Excel sheet-cell mapping of SPEC-008). The validation report gives the real file type (`csv` or `xlsx`).
- The upload already accepted `.xlsx` and `.csv`, so the server and screens are unchanged.

## Scope

- Reading the flattened layout from CSV and Excel, for any JSON format. The filing pack is the case built and proven.
- CSV and Excel copies of the three manual-test packs in `testdata/manual_filing_pack/`, written from the JSON by `go test ./cmd/recon-engine -run TestFlatFilingPacks -update`, never by hand.

## Out of scope

- ~~A sheet-per-document workbook (one sheet per form, labelled rows). It needs a real export to design against (IN-26).~~ Brought into scope on 2 November 2026 as part 2 below, on the user's request, against the synthetic Test Pack A workbooks.
- Old `.xls` workbooks, which stay refused (SPEC-008).
- New rules or check types.

## Part 2: one workbook per form (added 2 November 2026)

Requested by the user: test the tool on `testdata/testpack_nimbus/01_Form_3CD.xlsx` to `06_Financial_Statements.xlsx` and, where the tool has no format, add it. None of the six was recognised (`recon-engine inspect` gave no format). They are synthetic (fictional Nimbus Logic Systems, the same figures as `09_filing_pack_structured.json`), one workbook per document: an `Assessee` sheet (Particulars, Value) and one sheet per clause or part, either labelled rows (Particulars, Value) or a table with its own headings. Dates are date cells. This is still a stand-in: when the firm's tax suite gives a real Excel export, its own formats are added for it.

**The layout's two needs.**
- **A blank cell is blank text.** The UDIN rows of Form 3CD and Form 29B have an empty Value cell. A blank UDIN is a planted defect (F24), so it must be read as blank text, not as a figure that is not there. New sheet option `blank_as_text: ["Value"]`: in a row that is read, an empty cell under one of these headings is read as empty text. Only that sheet's named columns change; an empty amount cell elsewhere stays missing ("cannot check", never zero), as SPEC-008 part 2 says.
- **The heading a row sits under.** The Computation sheet indents the rows of each block under a heading row (`Add Back`, `Less`, `Normal Tax`, `Book Profit 115Jb`), and three of those rows are each labelled `Total`. New sheet option `headings: "Heading"`: a row whose first column is filled and whose other columns are all empty is a heading; each indented row under it (its first cell starts with spaces) gets the heading's label in a column of that name, and its own label without the leading spaces. A row that is not indented has no heading. So the Normal Tax total is the row where `Particulars` is `Total` and `Heading` is `Normal Tax`: read by its conditions, never by position.

**The formats.** Six built-in formats in `content/profiles/taxsuite_xlsx/` (tax audit report, MAT report, Form 67, computation, Form 26AS, financial statements). Each reads the same figures and rows, under the same names, as its JSON twin in `content/profiles/taxsuite_pack/`, and is recognised by its own sheets, so a workbook of another layout is not taken for it. The tax year and PAN come from the `Assessee` sheet. Each figure's source is its cell (`Form 29B!B15`).

## Acceptance criteria

- [x] AC-1 — Each CSV and Excel pack (01, 02, 03) gives the same exit code and the same report as its JSON twin, apart from the places: 28, 8 and 0 failed checks. (`TestFlatFilingPacks`)
- [x] AC-2 — Each figure's source is its Excel cell (`Filing pack!B…`) or CSV line (`line …`). (`TestFlatFilingPacks`)
- [x] AC-3 — A contradictory file (a path twice, a value inside a value, a skipped list number, `[*]`, a path without `$`) is refused, naming the line. (`TestFlatFileConflicts`)
- [x] AC-4 — Every existing format, sample and test still passes: `recon-dev profile-test` (42 samples), the Excel and CSV tests of SPEC-008, Test Pack A, and the architecture guard.
- [ ] AC-5 — Manual: the steps of `testdata/manual_filing_pack/README.md` give the same results with the `.xlsx` files and with the `.csv` files as with the JSON files, and the comparison list shows each file's name.
- [x] Every file this spec created is added to `docs/Tool1-File-Guide.docx` in plain language.

Part 2:

- [x] AC-6 — `recon-engine inspect` recognises each of the six workbooks as its document type, and `recon-dev profile-test` passes for the six new formats, each figure with its cell.
- [x] AC-7 — The six workbooks uploaded together for a client give the same findings, figure for figure, as `09_filing_pack_structured.json`: the same rules fail with the same values and differences; only the sources differ (cells instead of JSON paths). (Go test.)
- [x] AC-8 — The blank UDIN is read as blank text (F24 is reported); the Normal Tax total is read from the row under its heading; an empty amount cell outside `blank_as_text` columns is still missing, never zero. (Go tests of the reader.)
- [x] AC-9 — Every existing format, sample and test still passes, and Test Pack A Pass 1 still scores 22 of 30 with 0 false positives.

## Verification

1. `go test -count=1 ./cmd/recon-engine -run 'TestFlatFilingPacks|TestFlatFileConflicts' -v`.
2. `go run ./cmd/recon-dev profile-test` and `go test -count=1 ./...`.
3. Manual (AC-5): restart with `stop-local.cmd` and `start-local.cmd`, then follow `testdata/manual_filing_pack/README.md` with the `.xlsx`, then the `.csv`, files.

## Build notes, part 2 (2 November 2026)

- **Tested first, as asked:** `recon-engine inspect` recognised none of the six workbooks. All six now are, each as its document (`taxsuite.xlsx.tax_audit_report`, `mat_report`, `form_67`, `computation`, `form_26as`, `financial_statements`).
- **What the layout needed beyond the two planned options.** Building against the workbooks found three more gaps, each closed in the profile language rather than by reading a position: the tax year and PAN are labelled rows of the Assessee sheet, so a context value may now be read from `rows` + `where` like a figure (two matching rows are refused, once); the MAT credit rows are chosen by "the row of the file's assessment year", which the JSON did with `equals_path`, so a condition may be `equals_year: tax_year | assessment_year`; the clause 18 blocks share their table with a `Total` row that facts read, so a condition may be `not_equals: "Total"`; and clause 34(c) writes `No` as text, so a yes or no fact may take `yes_no: true`. A blank PAN (clause 31(c)) is also read as blank (`blank_as_text: ["Pan"]`), as in the JSON.
- **Result.** The six workbooks give 69 checks: 39 pass, 28 fail, 2 cannot check, exactly as `09_filing_pack_structured.json` does: all 127 figures and rows are equal, and the reports are equal apart from the places (`TestTestPackAWorkbooksGiveTheJSONFindings`). Each figure names its cell (`Form 29B!B15`); a sum names its rows, as in JSON.
- **Samples.** Each format has two samples: the workbook, and a copy with its sheets and table rows in reverse order (the Computation sheet keeps its row order, which its headings depend on), written by `go test ./cmd/recon-engine -run TestTestPackAWorkbookSamples -update`; the same test checks every expected file against the JSON format's expected file for the same document. `recon-dev profile-test`: 54 samples.
- **Tests:** `TestLabelledRowsBlanksAndHeadings` (reader), `TestLabelledRows` and `TestLabelledRowsRefusals` (profile language), `TestTestPackAWorkbookSamples`, `TestTestPackAWorkbooksGiveTheJSONFindings`, and `TestShippedProfiles` lists the six formats.
- **Not yet done:** the new files are not in `docs/Tool1-File-Guide.docx`, and the upload was not checked by eye in the running app.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
