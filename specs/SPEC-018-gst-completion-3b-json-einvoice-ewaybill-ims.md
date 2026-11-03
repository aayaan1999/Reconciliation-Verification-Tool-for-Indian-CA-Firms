# SPEC-018 — GST completion: GSTR-3B JSON, e-invoice register, e-way bill register and IMS actions

| | |
|---|---|
| **Status** | In progress |
| **Phase** | 3 |
| **Depends on** | SPEC-017 (GSTR-3B record type, totals of rows in `sum_equals`), SPEC-015 (B2C and HSN record types, `match_records`), SPEC-008 (CSV and Excel by header text), SPEC-016 (formats) |
| **Requirements** | RU-01, RU-02, RU-40, IN-10..14, IN-22 |
| **Technical spec** | §B2.2 (profile language), §B4.2 (rule files), §6.4 (fixtures) |
| **Code / content paths** | `content/vocabulary/records.yaml`, `content/profiles/generic_gstn/gstr3b`, `content/profiles/generic_portal/{einvoice,ewaybill,ims}`, `content/rules/2026-27/gst/`, `internal/engine/checks/matchrecords` (one-sided reporting), `cmd/recon-engine/gstcompletion_test.go`, `testdata/gst_completion_sep2026/` |

## Goal

The GST part of the Sep 2026 sample pack (`Reconciliation_Sample_Pack_Sep2026`) is read by the tool and compared: GSTR-3B (JSON as the GST portal gives it), the e-invoice IRN register, the e-way bill register and the IMS actions, each against the document it must agree with, in one upload with the files the tool already reads (sales register, purchase register, GSTR-1, GSTR-2B). On the pack, every difference the tool can compute is reported with both values, the difference and both sources.

This follows the 3 November 2026 trial: of 25 pack files, 4 were read; the Tally sales CSV could be read once a layout was taught, and the tool then reproduced the pack's R01 results except two that need a judgment (see Out of scope). The sample pack's expected outputs (`outputs/R0x_*.csv`) are the answer key for comparison; the Python engine that made them is not run (no Python in this project) and its recommended actions and legal advice are not reproduced.

## Scope

**Vocabulary (version 12).** Document types `einvoice`, `ewaybill`, `ims`; record types:
- `einvoice_irn` (key document number; fields IRN, buyer GSTIN, date, document type, IRN status; amounts taxable value, IGST, CGST, SGST, cess, total value).
- `ewaybill_row` (key document number; fields e-way bill number, date, from and to GSTIN, HSN, status; amount total value).
- `ims_action` (key supplier GSTIN and document number; fields document type, date, IMS action, in GSTR-2B; amount taxable value).
- `gstr3b_table_row` (from SPEC-017) is now also filled from the GST portal JSON.

**Formats shipped with the tool**, each with two samples and expected rows:
- `generic.gstn.gstr3b` (JSON): `sup_details.osup_det` is Table 3.1(a), `osup_zero` 3.1(b), `osup_nil_exmp` 3.1(c), `isup_rev` 3.1(d), `itc_elg.itc_avl` (type `OTH`) 4(A)(5), `itc_elg.itc_rev` 4(B). Read as the same `gstr3b_table_row` records as the CSV and Excel layouts of SPEC-017, so every rule that reads 3B reads both.
- CSV layouts for the e-invoice register, the e-way bill register and the IMS actions (columns found by header text; Excel versions as SPEC-017 did).
- The sales register and purchase register as Tally CSV exports, with the column names of the pack, as a shipped format (today they need a layout taught by hand). The client's GSTIN is taken from the client when a file has none.

**One small addition to `match_records`:** a rule may say a side is one-sided (`report_missing: ["a"]`): rows only in that side are reported and rows only in the other are not. Needed because an e-way bill exists only for movements above a value, so "sales invoice without an e-way bill" is not a computable difference, while "e-way bill without a sales invoice" is.

**Rules** (`recon.rule/1`, loaded as testing, naming their rule-book rule; two passing and two failing fixtures each, with tolerance cases where one applies):
- `RECON-GST-030` (RC-GST-001 books side of R02): sales register invoices added up per tax head against GSTR-3B 3.1(a). Both values and the difference are shown; no explanation of the difference is given.
- `RECON-GST-031` (RC-GST-007 for JSON): GSTR-1 B2B, B2CL and B2CS invoices added up against GSTR-3B 3.1(a), per head. Uses the totals-of-rows of SPEC-017.
- `RECON-GST-032` (RC-GST-014, R05): GSTR-2B ITC for invoices with ITC available (credit notes negative) added up per head against GSTR-3B Table 4(A)(5); reports the difference, not which side is right.
- `RECON-GST-033` (RC-GST-001 e-invoice side, R07): sales register against the e-invoice register by document number: in both or one, and taxable value, tax and total value. Rows whose IRN status is cancelled are listed with that status shown as written.
- `RECON-GST-034` (R08): e-way bills against the sales register by document number: the e-way bill's total value against the invoice's, and e-way bills with no sales invoice (`report_missing: ["a"]`). No finding for an invoice that has no e-way bill.
- `RECON-GST-035` (R04 IMS part): purchase register against the IMS actions by supplier GSTIN and document number: in both or one, taxable value, and the IMS action shown as written (Accepted, Rejected, Pending, deemed accepted) so the reader sees which documents IMS holds back.

## Out of scope

- Everything in the pack's outputs that is a judgment about law or about which side is right: `ITC_BLOCKED_17_5`, `ITC_TIME_BARRED_16_4`, `ITC_180_DAY_REVERSAL`, `NOT_APPLICABLE` (e-way bill thresholds), `NON_SUPPLY_MOVEMENT`, `TIMING_DIFFERENCE`, and every "recommended action" and legal reference. Invariant 1: the tool reports what it computes.
- Matching two invoice numbers that are written differently (`INV/26-27/0107` and `INV-26-27-107`, `UE/1010` and `UE-1010/26`): the tool reports each as in one document only. A fuzzy match is a guess and is not a computed fact. If the CA wants it, it is a later spec with an explicit, visible normalisation parameter.
- The pack's liability bridge (R02/R03 "explained by R01 items"): the explanation is an attribution, not a computation. The two totals and the difference are shown.
- The Tally sales register XML (a new file type) and the other pack areas: TDS (R11, R12, R14), payroll (R15, R16) and bank (R18) are separate specs.
- The GSTN JSON of the e-invoice and e-way bill (no sample); only the CSV downloads are read.
- Making a rule active: the CA approves each one (SPEC-010).

## Design notes

- The pack's company GSTIN (`08AAPCS9876K1ZQ`) fails the GSTIN check character, so the tool refuses it when a client is added. The pack's own files are never edited; the test fixtures under `testdata/gst_completion_sep2026/` use a valid GSTIN (`08AAPCS9876K1Z6`), recorded in the fixtures' README. Whether to relax the check for synthetic packs is a question for the user, not decided here.
- Expected on the fixtures (from the pack's answer key, only where computable): 031 and 030 show the books-against-3B differences of R02 (taxable 139,999.40 and so on) without the bridge; 032 shows IGST 9,000, CGST 2,400 and SGST 2,400 over 2B; 033 reports 0103, 0104, 0105 and 0111 as in the answer key, 0106 as in the books only, 0109 as in the register only, and 0112 with status CNL; 034 reports 0103 (value 100,300 against 94,400), 0112 (cancelled) and the e-way bill for 0113 with no invoice; 035 reports KT-901 as Rejected, SL-3301 as Pending and the credit note RS-CN-12 as in IMS only.
- The new rules are data (YAML) over existing check types; the only Go addition is the one-sided option of `match_records` and the new readers' profiles.

## Acceptance criteria

- [x] AC-1 — `generic.gstn.gstr3b` reads the pack's `gstr3b_092026.json` into `gstr3b_table_row` records for 3.1(a), 3.1(d), 4(A)(5) and 4(B) with every amount to the paise, and gives the same records as the CSV layout for the same figures; `recon-dev profile-test` passes (two samples).
- [x] AC-2 — The e-invoice, e-way bill and IMS formats (CSV, and Excel for each) and the Tally sales and purchase CSV formats read the pack's inputs; `recon-dev profile-test` passes for each.
- [x] AC-3 — `match_records` with `report_missing` reports only the chosen side's missing rows; Go tests cover each side, both, and neither (existing rules unchanged).
- [x] AC-4 — RECON-GST-030..035 pass `recon-dev rule-lint` (two passing and two failing fixtures each) and load as testing.
- [x] AC-5 — On the pack fixtures each rule gives the results in Design notes, each finding with both values, the difference and both sources, and no other finding (Go test against a checked-in answer list derived from the pack's outputs, with every excluded judgment named).
- [x] AC-6 — Uploading the whole pack folder with `--skip-unread` reads every GST file, lists the TDS, payroll, bank and XML files as not read, and the GST rules that need only the read files give results (Go test).
- [x] AC-7 — A run with the 3B JSON missing names `gstr3b` as the missing document for 030..032 and finds nothing wrong.
- [x] AC-8 — RECON-GST-001, 002, 003, 011, 013 and 020..023 give the same results as before and Test Pack A still scores 22 of 30 with 0 false positives.
- [x] AC-9 — No finding text says "should" or states a correct value (checked on every rule's wording and every finding in AC-5).
- [ ] AC-10 — `go test -count=1 ./...`, `gofmt`, `go vet` (host and Windows) and the architecture guard pass; the technical spec and `docs/Tool1-File-Guide.docx` list every new file.

## As built (deviations from the text above, 3 November 2026)

- **`match_records` got two parameters, not one.** `report_missing` is as written. The spec's
  expectations for RECON-GST-033 (IRN status CNL), 034 (e-way bill Cancelled) and 035 (KT-901
  Rejected, SL-3301 Pending shown as written) cannot be produced by `match_records` alone: a
  matched pair is a pass, and the status column exists on one side only. `report_fields` lists
  the values of a text column to report, shows the column as written, and also shows it on
  one-sided and differing rows. The Rule Builder form round-trips both (`internal/rules`).
- **Three small additions to the profile language** (`internal/profile`), needed to read the pack:
  `one_row: true` (a GSTR-3B table is an object, not a list), `period.format: "yyyy-mm-dd"` (the
  new registers have no period column; the month of the first row's date is the period, instead
  of a fixed `2026-09` that would be wrong for every other month) and the sample test accepts a
  chain of versions (3 supersedes 2 supersedes 1).
- **Two format versions beyond the spec's list.** GSTR-2B v2 (the pack puts the amounts on the
  invoice and lists credit notes under `cdnr`; v1 read neither) and GSTR-1 v3 (also gives the
  `gstr1_outward_line` rows, which RECON-GST-031 needs from the JSON).
- **RECON-GST-031 gives the same results as RECON-GST-020** once GSTR-1 v3 gives the JSON rows
  as `gstr1_outward_line`: both add up the same rows against the same 3B row. They are kept as
  the spec asks; one of them is redundant and the CA should decide which to retire.
- **E-way bill rows on a delivery challan are not read** (`doc_type` other than INV or BIL is
  listed as not read with the reason), so DC-0045 is not reported as an e-way bill with no
  invoice (the pack calls it NON_SUPPLY_MOVEMENT, a judgment). Otherwise the tool would list it.
- **The answer list also has the one-sided rows of the number variants** (INV/26-27/0107 in the
  books and INV-26-27-107 in the e-invoice register and e-way bill register; UE/1010 and
  UE-1010/26 in the purchase register and IMS) and, for 035, the purchase rows not in IMS
  (DC-778, BC-2201) and the duplicate GT/777, since all are computable.
- The pack's March GSTR-2B is of tax year 2025-26, so in the whole-folder run it is read and
  reported as `CONFLICTING_VALUES` (another tax year); the run continues.

## Verification

```
go run ./cmd/recon-dev profile-test
go run ./cmd/recon-dev rule-lint
go test -count=1 ./cmd/recon-engine -run 'GSTCompletion|GSTReturns|GSTPack|TestTestPackAPass1' -v
go test -count=1 ./...
gofmt -l .
go vet ./... && GOOS=windows GOARCH=amd64 go vet ./...
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
