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

- A sheet-per-document workbook (one sheet per form, labelled rows). It needs a real export to design against (IN-26).
- Old `.xls` workbooks, which stay refused (SPEC-008).
- New rules or check types.

## Acceptance criteria

- [x] AC-1 — Each CSV and Excel pack (01, 02, 03) gives the same exit code and the same report as its JSON twin, apart from the places: 28, 8 and 0 failed checks. (`TestFlatFilingPacks`)
- [x] AC-2 — Each figure's source is its Excel cell (`Filing pack!B…`) or CSV line (`line …`). (`TestFlatFilingPacks`)
- [x] AC-3 — A contradictory file (a path twice, a value inside a value, a skipped list number, `[*]`, a path without `$`) is refused, naming the line. (`TestFlatFileConflicts`)
- [x] AC-4 — Every existing format, sample and test still passes: `recon-dev profile-test` (42 samples), the Excel and CSV tests of SPEC-008, Test Pack A, and the architecture guard.
- [ ] AC-5 — Manual: the steps of `testdata/manual_filing_pack/README.md` give the same results with the `.xlsx` files and with the `.csv` files as with the JSON files, and the comparison list shows each file's name.
- [x] Every file this spec created is added to `docs/Tool1-File-Guide.docx` in plain language.

## Verification

1. `go test -count=1 ./cmd/recon-engine -run 'TestFlatFilingPacks|TestFlatFileConflicts' -v`.
2. `go run ./cmd/recon-dev profile-test` and `go test -count=1 ./...`.
3. Manual (AC-5): restart with `stop-local.cmd` and `start-local.cmd`, then follow `testdata/manual_filing_pack/README.md` with the `.xlsx`, then the `.csv`, files.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
