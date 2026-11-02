# SPEC-008 — Excel files read by header text

| | |
|---|---|
| **Status** | In progress |
| **Phase** | 3 |
| **Depends on** | SPEC-007 |
| **Requirements** | IN-02, IN-04, IN-22, IN-23, NF-01 |
| **Technical spec** | §A6.1, §A6.2, §B2.2, §B2.3, §B16 (phase 3), risk T1 |
| **Code / content paths** | `internal/ingest/adapters/xlsx/`, `internal/profile` (sheet tables), `internal/api` (upload types), `web/src` (file picker), `cmd/recon-engine` (adapter registry), `content/profiles/generic/`, `docs/fixtures.md`, `docs/contracts/integrations.md` |

## Goal

A CA can upload a spreadsheet as `.xlsx` or `.csv`, the same way as a JSON file. CSV is already read (SPEC-006 part 3); this spec adds `.xlsx`. Its figures and rows are found by sheet name and column header text, never by column letter, and are stored in paise with the sheet and cell they came from. The proof is that the GST sales register read from Excel gives exactly the same findings as the same register read from JSON.

## Library

Excel is read with **`github.com/xuri/excelize/v2` v2.11.0**, already in `go.mod` and checked by the architecture guard since 14 October 2026. It reaches only the parse-only packages `net/url` and `golang.org/x/net/html/charset`, which are reviewed and listed in `allowedNetworkImports` (ADR 0001). It is pure Go, so the single binary and zero-install deployment are unchanged.

It is already used in the developer tool: `cmd/recon-dev/rulestore_import.go` reads the rule book, and `cmd/recon-dev/rulestore_problems.go` writes the CA's problem list. `internal/ingest/adapters/xlsx/excelize.go` holds only a blank import today.

The functions this spec uses, and why:

| Need | Function | Why |
|---|---|---|
| Open the upload | `excelize.OpenReader(r, excelize.Options{...})` | Reads from the uploaded bytes, like the JSON and CSV adapters, never from a path. |
| Defensive limits | `Options{UnzipSizeLimit, UnzipXMLSizeLimit}` | An `.xlsx` is a zip file. The caps stop a zip bomb from exhausting memory. |
| Money without rounding | `Options{RawCellValue: true}` | Gives the value the workbook stores, not the display text (`112450.5`, not `1,12,450.50` or a rounded figure). That value then goes to `internal/money`, never through `float64`. |
| Find sheets | `f.GetSheetList()` | Supports detect tests on sheet names. |
| Read rows | `f.Rows(sheet)`, then `rows.Next()`, `rows.Columns()`, `rows.Close()` | Reads one row at a time, so a large trial balance or register does not load into memory at once. |
| Cell names in locators | `excelize.CoordinatesToCellName` | Turns a row and column into `C14` for the locator. |
| Formulas | The value the workbook saved | Never recompute (`CalcCellValue` is not used). The tool reports what the file says. |

## Scope

### 1. The adapter (`internal/ingest/adapters/xlsx`)

- `Sniff` recognises a workbook by its content (a zip file holding `xl/workbook.xml`), not by its extension.
- `Open` reads the workbook with the options above and a default limit of 64 MB, like CSV.
- Each sheet becomes a list of rows of cell text in the tree the profiles already address. Every cell keeps its place, so the locator is the file, the sheet and the cell (`Sales Register!C14`).
- Numbers are kept as exact decimal text. A stored value in exponent form (`1.2345E5`) is converted to plain decimal text with integer arithmetic only.
- Dates stored as serial numbers are turned into `YYYY-MM-DD` with integer arithmetic. Both the 1900 and the 1904 date systems are handled.
- A formula cell with no saved value is a missing value, never a zero.
- Merged cells give their value once, at the top-left cell. Hidden sheets, rows and columns are read like visible ones and the validation report says they were hidden.
- A password-protected workbook is refused with a clear message, and no password is ever asked for or stored.

### 2. Profiles that read sheets by header text (`internal/profile`)

- A profile with `file_type: "xlsx"` names each sheet it reads and the labels its header row must contain. The header row is the first row holding all those labels. Rows below it are read as objects keyed by header text, and reading stops at the first blank row.
- Detect tests can require a sheet to exist and a header row to contain given labels.
- A column is found by its header text. A moved or inserted column changes nothing. A header label missing from the sheet is an error that names the label. A label found twice in the header row is an error, never a guess.
- Everything else (`where`, `fields`, `amounts`, `unit`, `rounding`, `required`, entries that meet conditions, the validation report of values not read) works as it does for JSON and CSV.

### 3. Upload and run

- The server accepts `.xlsx` beside `.json` and `.csv`, with the same size limits and name checks.
- `recon-engine run`, `inspect` and `profile-create`, and `recon-dev profile-test`, register the Excel adapter. (`facts` reads the database, not files.) The mapping screen does not teach Excel layouts: `inspect` refuses a workbook with a clear message, and an Excel layout is written as a profile.
- An `.xls` file (Excel 97–2003) is refused with `UNSUPPORTED_FILE_TYPE` and a message asking for the file to be saved as `.xlsx` or `.csv` first (see the decision below).
- The upload screen's file picker offers `.json`, `.xlsx` and `.csv`, and its help text says `.xls` is to be saved as `.xlsx` or `.csv`.

### 4. The proof profile

- A synthetic `.xlsx` sales register in `content/profiles/generic_xlsx/sales_register/` (a new profile, `generic.books.sales_register_xlsx`, beside the JSON one), holding the same invoices as the JSON sample, with its expected facts and records. Its columns are in a different order from the JSON one, and it has a title row and a blank column before the header row.
- An `.xlsx` copy of the expense ledger (`content/profiles/generic_xlsx/ledger/`, a new profile, `generic.books.expense_ledger_xlsx`, beside the CSV one), holding the same rows as the CSV sample, so `.xlsx` and `.csv` are shown to give the same result.
- `docs/fixtures.md` describes both layouts.

## Out of scope

- **Excel export of findings** (FD-08, §B12): a later spec. It uses the same library (`NewFile`, `NewSheet`, `SetSheetRow`, `NewStyle`, `SetCellStyle`, `Write`).
- **Reading `.xls`**: not supported in Tool 1 (decision below).
- **Real Tally, GST portal or CA-staff Excel layouts**: their profiles come when real samples arrive (IN-26, Q1, Q2).
- **Coded fields and law tables**, which SPEC-005 had reserved for SPEC-008, move to SPEC-010 (SPEC-009 is comparison history).
- The Word, Tally XML, PDF and fixed-width adapters.

## Design notes

- The adapter knows syntax and the profile knows meaning (§A6). The header labels come from the profile, because only the profile knows which labels a layout has; the adapter's `View` finds the row that holds them and keys the rows below it by header text.
- No floating point anywhere. excelize gives cell text; the adapter never calls `strconv.ParseFloat` and never uses `excelize.ExcelDateToTime`, which works in floating point. Money goes through `money.Parse`, so a stored value with more than two decimal places (`0.30000000000000004`) is an error unless the profile names a `rounding`.
- Determinism: sheets are read in workbook order and cells in row and column order. No map is iterated to produce output.
- When this spec is built, §B2.2 of the technical spec is updated to the profile syntax as built, in the same commit.

## Decision: `.xlsx` and `.csv`, not `.xls`

Decided 28 October 2026: Tool 1 reads spreadsheets as **`.xlsx` and `.csv`**. Old Excel files (`.xls`, Excel 97–2003) are not read; staff save them as `.xlsx` or `.csv` first, which keeps to the manual file-drop path (invariant 7). excelize reads only `.xlsx`, so no second library is added. Requirement IN-02 in `requirements-spec-tool1.md` is updated to match.

## Acceptance criteria

- [x] AC-1 — The adapter recognises an `.xlsx` workbook by its content and refuses a renamed non-workbook, a password-protected workbook and a workbook over the size or unzip limits, each with a clear error naming the file.
- [x] AC-2 — Money cells are read as exact decimal text and stored in paise, including values in exponent form. A test confirms no `float64` money path (no `ParseFloat`, no `ExcelDateToTime`) exists in the adapter.
- [x] AC-3 — Date serial numbers in both the 1900 and 1904 date systems become `YYYY-MM-DD`, with tests on known dates.
- [x] AC-4 — A profile finds the header row by its labels and each column by header text. Moving or inserting a column gives the same facts and records. A missing label and a label found twice are errors naming the label.
- [x] AC-5 — Every fact and record from Excel carries a locator with file, sheet and cell, and the validation report lists every value not read, with its sheet and cell.
- [x] AC-6 — A formula cell with no saved value is a missing value (`REQUIRED_FIELD_MISSING` when required), never a zero.
- [x] AC-7 — The server and the upload screen accept `.json`, `.xlsx` and `.csv`, and refuse `.xls` with the message asking for `.xlsx` or `.csv`. CSV uploads work exactly as before (the existing CSV tests still pass).
- [x] AC-8 — The `gst_sales_vs_gstr1` check gives the same findings with the sales register from the `.xlsx` sample as from the JSON sample, apart from the locators.
- [x] AC-8a — The same holds for the expense ledger: last year's and this year's ledger as `.xlsx` give exactly the same Worklist items as the existing `.csv` files.
- [x] AC-9 — `recon-dev profile-test` passes for the new profile; the same file read twice gives byte-identical output.
- [x] AC-10 — The architecture guard passes: excelize reaches only the network edges already allowed, and the linked `recon-engine` binary still has no connection, socket or resolver code.
- [x] AC-11 — `docs/contracts/integrations.md`, `docs/fixtures.md` and §B2.2 of the technical spec describe what was built.
- [x] AC-12 — Every file this spec created is added to `docs/Tool1-File-Guide.docx` in plain language.

### Part 2: what real exports do (added 28 October 2026)

A manual test of the Excel reader through the running server, with workbooks made from the synthetic samples the way real exports look, found ten gaps; the user asked for them to be covered. The worst gave false findings without a warning.

- [x] AC-13 — An empty row inside a table does not end it: every row below with a value under the table's headings is read, and the report warns `EMPTY_ROWS`, naming the empty rows. (Before: one blank row in the register dropped 21 of 26 invoices and gave 18 false "In GSTR-1 only" findings.)
- [x] AC-14 — A profile may list `total_rows` ("Total", "Grand Total"): such a row is left out, and the report lists its cells under "Not read" with the reason "a total row". (Before: a Grand Total row refused the whole ledger.)
- [x] AC-15 — A profile may set `any_name: true` on a sheet: the table is read from whichever one sheet holds its header row ("Sheet1"); two sheets holding it are an error, never a guess. Locators name the real sheet.
- [x] AC-16 — Header labels match without regard to capitals, for every label a profile reads, header list or not; columns are keyed by the profile's spelling.
- [x] AC-17 — A workbook no format reads gets a message saying, for each Excel format, the sheet or the header labels it did not find ("the nearest, row 3, lacks "Taxable Value"").
- [x] AC-18 — A problem in an Excel row names its sheet row (`Ledger!286:286`) in the message, not a JSON path.
- [x] AC-19 — An Excel layout can be taught on the mapping screen: the inspection lists each sheet's columns by header text, and the profile written from the choices reads the sample and the same layout on a sheet of another name with headers in other capitals.
- [x] AC-20 — A GST document whose layout does not say whose GSTIN it is, read for a client with exactly one GSTIN, is given that GSTIN, and the report warns `GSTIN_FROM_CLIENT` naming its last four characters. A client with several GSTINs is not guessed for.
- [x] AC-21 — In row matching, an amount that no row of a document holds (its layout does not read that column) is missing, never zero: a row that agrees on everything else is "Cannot check", naming the amount and the document. An amount only some rows lack stays zero in them, as GST portal files leave zeros out.
- [x] AC-22 — A 200,000-row ledger workbook is read in about 17 s with a peak of about 1.3 GB (before: 23 s, 1.9 GB); a workbook of more than 1,000,000 filled cells is refused as `FILE_TOO_LARGE` with a message to split it or save it as `.csv`.

## Verification

```
go run ./cmd/recon-dev profile-test
go test -count=1 ./internal/ingest/... ./internal/profile/... ./internal/api/...
go test -count=1 ./internal/archtest/ -v
go test -count=1 ./...
gofmt -l .
go vet ./... && GOOS=windows GOARCH=amd64 go vet ./...
```

Also `go test -count=1 ./cmd/recon-engine -run TestExcel -v` and, in `web/`, `npm test` and `npm run lint`.

Manual: build the screens (`start-local.cmd`), upload the `.xlsx` sales register and GSTR-1 under "GST: Sales register vs GSTR-1", and check that the findings match the JSON run and that each locator names the sheet and cell.

## What was built

- **The adapter** (`internal/ingest/adapters/xlsx`): `excelize.go` opens the workbook from the uploaded bytes and reads every non-empty cell of every sheet; `values.go` turns stored numbers into plain decimal text and date serials into `YYYY-MM-DD` with integers only, and recognises date formats; `table.go` is `View`, which reads a workbook as the tables of one profile (the header row found by its labels, rows keyed by header text) and gives each value its sheet and cell.
- **Profiles** (`internal/profile`): an Excel profile has `sheets: [{name, header}]`; every path from the top must start in a named sheet; bundles are refused. Detection and extraction read the workbook through the profile's tables; a sheet or header row that is not there means no match. After extraction every locator becomes `Sheet!C14` (a value) or `Sheet!14:14` (a row). Hidden sheets, rows and columns give the warning `HIDDEN_CONTENT`.
- **Upload**: the server accepts `.json`, `.xlsx` and `.csv` and refuses `.xls` with the re-save message; the upload screen's picker and help text say the same.
- **Samples**: `content/profiles/generic_xlsx/sales_register/v1` and `generic_xlsx/ledger/v1`, each with two workbooks and their expected files, written from the JSON and CSV samples by `go test ./cmd/recon-engine -run TestExcelSamples -update`.
- **Code review fixes (28 October 2026)**: a sheet is walked by its filled cells only, so a sparse workbook cannot stall a run (`TestSparseWorkbook`); a workbook's table view is built once per profile and reused (`TestViewIsKept`). In the profile code from earlier specs: `combine: latest` needs real `yyyy-mm-dd` dates, otherwise no figure (`TestLatestNeedsRealDates`); a period not in its format is reported once (`TestUnreadablePeriodIsOneError`); values joined from a `[*]` column are escaped (`TestJoinedFieldsAreEscaped`); `^` goes up only from a list entry (`TestParentPath`); the `equal` check uses the one shared tolerance function.
- **Second code review fixes (28 October 2026)**: a header label found twice in a sheet one format names no longer stops another format from matching; the label error is reported only when no format matches (`TestHeaderErrorDoesNotStopOtherFormats`). In the profile and engine code from earlier specs: with `combine: latest`, a latest entry without the figure gives no figure rather than an older entry's, and a tie names how many entries share the date (`TestLatestEntryWithoutFigure`); a `required` figure that cannot be read without guessing is a `REQUIRED_FIELD_MISSING` error, not a warning (same test); the reason a row was left out names the column and the condition but never the row's own value, and the GSTIN warning names its place, not the GSTIN (`TestMessagesHoldNoClientValues`; the screens now show `.supply_type is another value; this format reads only rows where .supply_type is "B2B"`); the place an `equals_path` condition compares with counts as read (same test); a fixed period with a `format` is checked when the profile loads (`TestFixedPeriodFormatCheckedAtLoad`); the engine keeps what a row-matching check could not find (`MissingFacts`); record match keys are built by one shared function, `domain.JoinMatchKey`.

| Criterion | Proved by |
|---|---|
| AC-1 | `TestRefusals` (xlsx adapter) |
| AC-2 | `TestPlainDecimal`, `TestNoFloatingPoint`; `TestExcelProfileReadsByHeader` (15000.50 is 1500050 paise) |
| AC-3 | `TestSerialDate`, `TestDate1904`, `TestViewReadsByHeaderText` |
| AC-4 | `TestMovedColumnReadsTheSame`, `TestHeaderProblems`, `TestExcelProfileReadsByHeader`, `TestExcelProfileProblems`, `TestExcelProfileRefusals` |
| AC-5 | `TestViewReadsByHeaderText`, `TestExcelProfileReadsByHeader`, `TestExcelSamples` |
| AC-6 | `TestViewReadsByHeaderText`, `TestExcelProfileProblems` |
| AC-7 | `TestUploadExcel`, `TestUploadChecks` (API); the existing CSV and Worklist tests |
| AC-8 | `TestExcelGivesTheSameFindings` |
| AC-8a | `TestExcelGivesTheSameWorklist` |
| AC-9 | `recon-dev profile-test` (42 samples), `TestShippedProfiles`, `TestExcelProfileDeterministic` |
| AC-10 | `internal/archtest` (no new network edge; binary symbol check unchanged) |
| AC-11 | `docs/contracts/integrations.md`, `docs/contracts/error-model.md`, `docs/fixtures.md`, technical spec §B2.2 |
| AC-12 | `docs/Tool1-File-Guide.docx` |
| AC-13 | `TestViewReadsByHeaderText`, `TestRealExportShape` |
| AC-14 | `TestRealExportShape`, `TestExcelTotalRowAndRowNames` |
| AC-15 | `TestRealExportShape` (two sheets refused), `TestTeachPlainExcelExport` (sheet "Sheet1") |
| AC-16 | `TestRealExportShape`, `TestTeachExcelLayout` |
| AC-17 | `TestNotFoundSaysWhatIsMissing`, `TestExcelNoMatchSaysWhy` |
| AC-18 | `TestExcelTotalRowAndRowNames` |
| AC-19 | `TestGuess`, `TestTeachExcelLayout`, `TestTeachPlainExcelExport`; web test "in a workbook the period and GSTIN are columns of a sheet" |
| AC-20 | `TestTeachPlainExcelExport` (23 checks: 17 pass, 6 fail, as from JSON) |
| AC-21 | `TestAmountNoRowHoldsIsMissing`; the GST answer key unchanged (`TestGSTPackMatchesTheAnswerKey`) |
| AC-22 | `TestAttributePassMatchesExcelize`, `TestCellLimitAndPlainErrors`; measured by hand (below) |

**Part 2 as built.** `table.go` reads to the end of the sheet, leaves total rows out (`View.LeftOut`), finds a sheet by its header when `any_name` is set and matches labels ignoring capitals (`Table.Labels`, collected by `labelsOf` from every path of the profile); `NotFoundError` says what the nearest row lacks. `attrs.go` reads each sheet's cell types, styles and hidden rows and columns in one pass over its XML, instead of excelize loading the whole sheet again for every cell; a workbook whose parts cannot be found that way falls back to excelize. The JSON path code builds its escaper once (it was built for every quoted key, 800,000 times for the large ledger). `Inspect` lists a workbook through `Doc.Guess` (a header row per sheet), the mapping writes an Excel profile (`Mapping.FileType`, `sheets` from the paths chosen), `profile-create` takes the file type from the sample and the server accepts `.xlsx` for teaching. `scopeFromClient` in `run.go` gives the client's only GSTIN; `matchrecords` marks an amount no row holds as `amount:<record type>.<amount>`, which the report words as "Cannot check: … is not in …". Both shipped Excel profiles now use `any_name` and `total_rows` (they were not yet published).

**Measured by hand (28 October 2026)**, a 200,000-row, 4-column ledger: 22.6 s and 1.86 GB peak before, 16.8 s and 1.31 GB after; through the server, 30 s (the server reads the files a second time for the figures panel). The 14 test workbooks of the manual test (in the git-ignored `build/xltest/`) now give the JSON register's 23 checks (17 pass, 6 fail) for every register variant, and 12 Worklist items for the ledger with a total row.

**Known after part 2.** A taught Excel layout and a shipped one can both match a file (the shipped synthetic sample with its Details sheet matches a taught layout of the same register); the tool then refuses as `PROFILE_AMBIGUOUS` rather than guess, and the taught layout needs an identifying value. The mapping screen offers one row filter, so a register that needs several (Doc Type and Is Cancelled as well as Supply Type) cannot yet be taught in full. Reading still holds every cell of a workbook in memory several times over; a reader that streams rows into records would cut the 1.6 KB per cell further.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
