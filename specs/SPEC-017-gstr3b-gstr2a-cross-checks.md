# SPEC-017 — GSTR-3B and GSTR-2A read from CSV and Excel, and GSTR-1, 2A and 3B compared when uploaded together

| | |
|---|---|
| **Status** | In progress |
| **Phase** | 3 |
| **Depends on** | SPEC-008 (Excel and CSV read by header text), SPEC-015 (GST record types, `match_records`), SPEC-006 (`sum_equals`), SPEC-016 (formats) |
| **Requirements** | RU-01, RU-02, RU-40, IN-10..14, IN-22 |
| **Technical spec** | §B2.2 (profile language), §B4.2 (rule files), §6.4 (fixtures) |
| **Code / content paths** | `content/vocabulary/records.yaml`, `content/profiles/generic_portal/`, `content/rules/2026-27/gst/`, `internal/engine/checks/sumequals`, `internal/profile`, `cmd/recon-engine/gstreturns_test.go`, `testdata/gst_returns_apr2026/` |

## Goal

A user uploads the GSTR-1, GSTR-2A and GSTR-3B files of one month together, as three CSV files or as one Excel workbook with one sheet each, and the tool compares them and reports each place they differ, to the rupee, with both values and both sources. It works with no change to how the user uploads: every upload runs every rule and files are recognised by their contents.

The trial on 3 November 2026 (sample files in `GST_Return_Sample_Files_Excel_and_CSV`) showed the gap: GSTR-1 could only be read after a layout was taught by hand, and GSTR-3B and GSTR-2A could not be read at all.

## Scope

**Vocabulary (version 11).** Three record types and two document types:
- `gstr3b_table_row` (document `gstr3b`): one row of GSTR-3B, key `table` (3.1(a), 3.1(b), 3.1(c), 3.1(d), 4(A)(5), 4(B)(2) and so on); amounts taxable value, IGST, CGST, SGST, cess.
- `gstr2a_b2b_invoice` (document `gstr2a`): one invoice in GSTR-2A, key supplier GSTIN and invoice number; fields date, place of supply, reverse charge, ITC availability, filing period; the same amounts. GSTR-2A is its own document, not GSTR-2B (it is live and changes as suppliers file; 2B is fixed), so reports name the right one.
- `gstr1_outward_line` (document `gstr1`): one row of a flat GSTR-1 table with its `category` (B2B, B2CL, B2CS, EXP), so a total of B2B, B2CS and exports can be taken from one list.

**Formats shipped with the tool** (`generic_portal`, CSV and Excel, each with two samples and expected rows): GSTR-1, GSTR-2A and GSTR-3B as the sample files lay them out, found by header text, any sheet name, labels matched ignoring capitals. A taught layout (SPEC-016) still wins over a shipped one for the same file.

**One new capability in `sum_equals`: totals of rows.** A part may be `{input, field, sign, rows_where: [...]}`: the sum of that amount over every row of the record type that meets the conditions, instead of one fact. `total` may name one row of another record type by its key (`gstr3b_table_row`, table `3.1(a)`). A figure that has no rows is Cannot check, never zero. Money stays in integer paise and rows are added in a fixed order.

**Rules** (`recon.rule/1`, loaded as testing, naming their rule-book rule; two passing and two failing fixtures each, including a tolerance case where one applies):
- `RECON-GST-020` (RC-GST-007): GSTR-1 outward rows in categories B2B, B2CL and B2CS, added up per tax head and taxable value, against GSTR-3B 3.1(a).
- `RECON-GST-021` (RC-GST-007, RC-GST-004): GSTR-1 export rows against GSTR-3B 3.1(b).
- `RECON-GST-022` (RC-GST-016 inputs, 3.1(d)): GSTR-2A invoices marked reverse charge, added up per head, against GSTR-3B 3.1(d). Reports each head so a split between IGST and CGST/SGST shows.
- `RECON-GST-023`: GSTR-2A invoices with ITC availability "Available" against GSTR-3B 4(A)(5), each head. Worded as "differs"; it does not say which side is right (4(A)(5) is one of several ITC rows).

**Upload together.** The three CSVs, or the workbook (each sheet read by its own format, as SPEC-005 and SPEC-011 do), in one run give one report; a missing document makes its rules "cannot check", named.

## Out of scope

- GSTR-1 against GSTR-2A (they hold different parties; nothing to match).
- GSTR-2A against GSTR-2B and ITC rules 4(A)(1)..(4), 4(B), 4(D) (RC-GST-014 and others need more documents), amendments, credit notes in the flat table, the interest and late fee rows of GSTR-3B, and any applicability or "should" judgment.
- Reading the GSTN JSON of GSTR-3B and GSTR-2A (the layouts are in the rule book; no sample yet).
- Making a rule active: the CA approves each one (SPEC-010).

## Design notes

- Expected on the sample files (April 2026): RECON-GST-020 and RECON-GST-021 pass. RECON-GST-022 reports DC-778: taxable value agrees, IGST 0 against 1,800, CGST 900 against 0, SGST 900 against 0. RECON-GST-023 reports taxable value 150,000 against 22,000, IGST 18,000 against 3,960, CGST 4,500 against 1,980, SGST 4,500 against 1,980.
- The rule wording never says a figure is wrong or what it should be.
- Because `sum_equals` and the new record types are data, the four rules need no per-rule Go.

## Acceptance criteria

- [x] AC-1 — The three shipped CSV formats and the Excel workbook formats read the sample files; `recon-dev profile-test` passes for each (two samples each, expected rows).
- [x] AC-2 — `sum_equals` on rows (`rows_where`, `total` as a keyed row) has Go tests: sums across rows, empty selection is Cannot check, one paise over tolerance, deterministic order.
- [x] AC-3 — RECON-GST-020..023 pass `recon-dev rule-lint` (two passing and two failing fixtures each) and load as testing.
- [x] AC-4 — Uploading the three sample CSVs together, and the workbook alone, gives the same results: 020 and 021 pass; 022 and 023 give exactly the differences in Design notes, each with both values, the difference and both sources; no other finding (Go test).
- [x] AC-5 — A run with only one of the three files names the missing document and finds nothing wrong.
- [x] AC-6 — A defect planted alone in the sample (one GSTR-1 CGST changed by 1 paise beyond tolerance, one 3B row removed) fires only the expected rule.
- [x] AC-7 — RECON-GST-001, 002, 003, 011, 013 give the same results as before and Test Pack A still scores 22 of 30 with 0 false positives.
- [ ] AC-8 — `go test -count=1 ./...`, `gofmt`, `go vet` (host and Windows) and the architecture guard pass; the technical spec and `docs/Tool1-File-Guide.docx` are updated with every new file.

## Verification

```
go run ./cmd/recon-dev profile-test
go run ./cmd/recon-dev rule-lint
go test -count=1 ./cmd/recon-engine -run 'GSTReturns|GSTPack|TestTestPackAPass1' -v
go test -count=1 ./...
gofmt -l .
go vet ./... && GOOS=windows GOARCH=amd64 go vet ./...
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
