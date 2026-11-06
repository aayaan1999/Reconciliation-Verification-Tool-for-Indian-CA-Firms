# SPEC-020 — TDS and bank documents: TDS ledger, Form 140, challans, OLTAS, Form 168, bank book and bank statement

| | |
|---|---|
| **Status** | In progress |
| **Phase** | 3 |
| **Depends on** | SPEC-018 (the `match_records` options `report_missing` and `report_fields`, CSV formats shipped as profiles; its AC-10 is still open), SPEC-008 (CSV and Excel by header text), SPEC-016 (formats), SPEC-010 (rules approved by the CA) |
| **Requirements** | RC-04 (TDS reconciliation), RU-01, RU-02, RU-10, IN-10..14 |
| **Technical spec** | §B2.2 (profile language), §B3 (vocabulary), §B4.2 (rule files), §6.4 (fixtures) |
| **Code / content paths** | `content/vocabulary/facts.yaml` and `records.yaml`, `content/profiles/generic_tally/{tds_ledger,tds_challan,tds_receivable,bank_book}`, `content/profiles/generic_portal/{form140,oltas,form168,bank_statement}`, `content/rules/2026-27/tds/`, `content/rules/2026-27/bank/`, `content/comparisons/comparisons.yaml`, `cmd/recon-engine/tdsbank_test.go`, `testdata/tds_bank_sep2026/` |

## Goal

The TDS and bank files of the Sep 2026 sample pack (`Reconciliation_Sample_Pack_Sep2026`) are recognised and read by the tool, and compared in one upload with the GST files it already reads (SPEC-018): the TDS ledger against the Form 140 deductee rows, the books' challans against the OLTAS challan status, the books' TDS receivable against Form 168, and the bank book against the bank statement. Every difference the tool can compute is reported with both values, the difference and both sources.

This follows the 4 November 2026 trial: of the 25 pack files, 8 were read as they are; the 5 TDS files, the bank book, the bank statement and `bank_meta.json` were refused with `PROFILE_NO_MATCH` because the tool has no document type or record type for them. Teaching a layout cannot fix that, since a layout only points a file at an existing type. The pack's answer files (`outputs/R11`, `R12`, `R14`, `R18`) are the answer key for what can be computed; the Python engine that made them is not run, and its recommended actions, law references, interest figures and reason codes that need a judgment are not reproduced.

## Scope

**Vocabulary (version 13).** Document types `tds_ledger`, `form140`, `tds_challan`, `oltas_status`, `tds_receivable`, `form168`, `bank_book`, `bank_statement` (facts.yaml), and record types (records.yaml):

- `tds_ledger_row`: one deduction in the books. Key: deductee and payment date. Fields: voucher number, deductee, PAN, section, nature, payment date, challan CIN, deposit date. Amounts: amount paid, TDS amount.
- `form140_deductee_row`: one deductee row of the TDS statement. Same key. Fields: PAN, deductee, section code, payment date, challan CIN. Amounts: amount paid, TDS deducted, TDS deposited.
- `tds_challan_books`: one challan in the books. Key: CIN. Fields: BSR code, deposit date, serial. Amount: amount.
- `oltas_challan`: one challan as the bank reported it. Key: CIN (BSR code, date and serial put together the same way as the books). Fields: status. Amounts: amount, consumed, available.
- `tds_receivable_row`: TDS a customer is expected to have deducted from us. Key: deductor TAN and section. Fields: customer. Amounts: amount credited, TDS expected.
- `form168_row`: one deductor and section in Form 168 (the former Form 26AS part 1). Key: deductor TAN and section. Fields: deductor name, booking status. Amounts: amount paid, TDS deposited.
- `bank_book_entry`: one line of the bank ledger in Tally. Key: instrument reference. Fields: date, voucher number, particulars. Amounts: receipt, payment.
- `bank_statement_line`: one line of the statement. Key: reference number. Fields: transaction date, value date, description. Amounts: credit, debit.

`bank_meta.json` (opening balances of book and statement) is read as facts `bank_opening_balance_books` and `bank_opening_balance_statement`, with the account name and period.

**Formats shipped with the tool** (CSV, columns found by header text, each with two samples and expected rows; an Excel version of each as SPEC-018 did): TDS ledger, challans of the books, TDS receivable, bank book, Form 140 deductee records, OLTAS challan status, Form 168 extract, bank statement, and the `bank_meta.json` format. Amounts with paise (`250000.6`) are read exactly to the paise. The client's TAN and the period are taken from the client or from the file's own dates, as SPEC-018 did for GSTIN and month, and no value is invented when a file has none.

**Rules** (`recon.rule/1`, loaded as testing, each naming its rule-book rule; two passing and two failing fixtures each, with a tolerance case where one applies). The rule-book rules these come from are RC-TDS-001, RC-TDS-003, RC-IT-002 and RC-BNK-001; the CA confirms the mapping when approving.

- `RECON-TDS-010` (R11, rule-book RC-TDS-001): TDS ledger rows against Form 140 rows by deductee and payment date: in books only, in statement only, and where both exist the differences in amount paid, TDS deducted and TDS deposited. PAN and section are shown from both sides and a difference in either is reported as a difference of the text value, with no statement of which is correct.
- `RECON-TDS-011` (R12, rule-book RC-TDS-003): challans in the books against the OLTAS status by CIN: in books only, in OLTAS only, and the difference in amount; the OLTAS status, consumed and available amounts are shown as written.
- `RECON-TDS-012` (R12, second part): the TDS the Form 140 rows say was deposited against each challan, added up by CIN, against the OLTAS amount of that challan. Both values and the difference are shown; nothing says which is wrong.
- `RECON-TDS-013` (R14, rule-book RC-IT-002): TDS receivable by deductor TAN and section against Form 168: in books only, in Form 168 only (shown with the amount, no remark about unbooked income), and the difference in TDS. The booking status is shown as written.
- `RECON-BNK-010` (R18, rule-book RC-BNK-001): bank book against the bank statement by reference: in book only, in statement only, and where both exist the difference in amount (receipt less payment against credit less debit). Dates of both sides are shown.
- `RECON-BNK-011`: the opening balance in `bank_meta.json` for the books against the statement, and the closing balance each side gets from its own opening balance and lines; both values and the difference.

**Comparison types** in `content/comparisons/comparisons.yaml`: `tds_books_vs_statement` (010, 011, 012), `tds_credit_vs_form168` (013), `bank_book_vs_statement` (010, 011), and the new documents join the all-rules upload (no type picker since 30 October 2026).

## Out of scope

- Everything in the pack's outputs that is a judgment or needs a law table: `SHORT_DEDUCTION` and `expected_tds` (the rate comes from the Income-tax Act and the nature of the payment, RU-31 law tables), `LATE_DEPOSIT` and every interest figure (RC-TDS-004 and 005, a later spec with a rate table), `INCOME_NOT_BOOKED?`, `UNPRESENTED_CHEQUE`, `UNCLEARED_DEPOSIT`, `UNIDENTIFIED_RECEIPT`, `INTEREST_UNBOOKED`, `BANK_CHARGE_UNBOOKED` (classifying a bank line is a judgment; the tool reports the line as in one side only), `TIMING_DIFFERENCE`, and every "recommended action" and legal reference. Invariant 1: the tool reports what it computes.
- Matching two references written differently, and many-to-one matching of bank lines (RC-BNK-001's tolerance windows and UTR grouping): not a computed fact; a later spec with a visible parameter if the CA wants it.
- Bank charges and GST on them, interest against 26AS, cash thresholds, year-end bank confirmation (RC-BNK-002..006).
- Payroll, EPF ECR and ESIC (RC-05) and the Tally sales register XML: the next spec (new reader for the `#~#` text file and the XML).
- Reading TDS data from FVU or TRACES text files, PDFs of Form 140 or Form 168, and bank statements as PDF.
- Making a rule active: the CA approves each one (SPEC-010).

## Design notes

- **Rule numbers.** `RECON-TDS-001..004` are already taken by the Form 67 and 26AS checks, so these start at 010. Rules for TDS and bank go in their own folders beside `gst/`.
- **Match keys are a proposal.** The pack's R11 matches a row whose PAN differs (Beta Consultants, `AAFFB2345D` against `...E`) and one whose section differs (Eta Printers), so PAN and section cannot be in the key; deductee name and payment date can, because both files carry them. A deductee whose name is spelt differently on the two sides is reported as in one side only (as SPEC-018 does for invoice numbers). The CA confirms the key before the rules are approved.
- **Signed bank amounts.** The book has receipt and payment columns, the statement credit and debit. If `match_records` cannot compare `receipt − payment` against `credit − debit` as it is, add the smallest change: a profile field that gives one signed amount from two columns (integer paise arithmetic). The first task of the build is to check this and record the choice here.
- **One-sided reporting** reuses `report_missing` and `report_fields` from SPEC-018 (status shown as written), so no new check type is needed.
- **Fixtures** under `testdata/tds_bank_sep2026/` copy the pack's inputs. PANs, TANs and names in the pack are invented; no real data enters the repo. The company GSTIN problem noted in SPEC-018 does not apply (no GSTIN is used here). The pack's own files are never edited.
- **Expected on the fixtures** (only what is computable, from the answer key): 010 reports Beta (PAN differs), Epsilon (in books only), Eta (section differs) and Zeta (in statement only), and passes Alpha, Gamma and Delta as matching figures; 011 reports challan `...00021` (books 38,000, OLTAS 30,800, difference 7,200) and `...00041` (in books only), and passes `...00011` and `...00034`; 012 reports `...00021` (statement rows deposit more than OLTAS shows) and `...00041`; 013 reports Mehta Retail (in books only, 2,500), Gujarat Infotech (10,000 against 8,000) and Noida Networks (in Form 168 only, 300), and passes Bhavani; 010-bank reports PYMT-202 (−38,000 against −30,800), RCPT-102 (295,000 against 294,700), the cheques 004512 and 778899 in the book only, and the three statement lines `UTIBN26301122`, `INT-OCT26` and `CHG-OCT26` in the statement only, each with no classification. These are examples for the build to confirm against the real fixtures, not promised outputs.

## Acceptance criteria

- [x] AC-1 — Vocabulary version 13 holds the eight document types and eight record types above and loads (`recon-dev` vocabulary check and `go test ./internal/vocab`).
- [x] AC-2 — Each of the eight CSV formats (and Excel for each) reads the pack's file to the paise, and `bank_meta.json` is read as facts; `recon-dev profile-test` passes with two samples each.
- [x] AC-3 — Every one of the pack's 8 TDS and bank files is recognised (no `PROFILE_NO_MATCH`); the 8 GST files read before are unchanged.
- [x] AC-4 — RECON-TDS-010..013 and RECON-BNK-010..011 pass `recon-dev rule-lint` (two passing and two failing fixtures each) and load as testing.
- [x] AC-5 — On the pack fixtures each rule gives the results in Design notes after the CA's confirmation of the keys, each finding with both values, the difference and both sources, and no other finding (Go test against a checked-in answer list derived from `outputs/R11`, `R12`, `R14`, `R18`, with every excluded judgment named).
- [x] AC-6 — A bank line or TDS row that exists on one side only is reported with its values and no classification, no reason code and no "should" (checked on every rule's wording and every finding in AC-5).
- [x] AC-7 — A run with one of the two documents of a rule missing names that document as missing and finds nothing wrong.
- [x] AC-8 — Uploading the whole pack `inputs` folder with `--skip-unread` reads every GST, TDS and bank file and lists only the payroll, EPF, ESIC and XML files as not read.
- [x] AC-9 — The signed-amount question in Design notes is settled and written here, with a Go test for any new profile field.
- [x] AC-10 — RECON-GST rules give the same results as before and Test Pack A still scores 22 of 30 with 0 false positives.
- [ ] AC-11 — `go test -count=1 ./...`, `gofmt -l .`, `go vet ./...` (host and Windows) and the architecture guard pass (new packages classified in `archtest_test.go` if any); the technical spec and `docs/Tool1-File-Guide.docx` list every new file.

## Verification

- `go run ./cmd/recon-dev rulestore-check`, `go run ./cmd/recon-dev rule-lint`, `go run ./cmd/recon-dev profile-test`.
- `go test -count=1 ./cmd/recon-engine -run TestTDSBank -v` (answer list) and `go test -count=1 ./cmd/recon-engine -run TestTestPackAPass1 -v`.
- `go run ./cmd/recon-engine run --db build/recon.db --input <pack inputs> --tax-year 2026-27 --include-testing --skip-unread`.
- `go test -count=1 ./...`, `gofmt -l .`, `go vet ./...` and `GOOS=windows GOARCH=amd64 go vet ./...`.

## Open questions for the user

1. Is deductee name plus payment date the right key for the ledger against Form 140, or should the CA name another (for example challan CIN plus deductee)?
2. May interest and expected-TDS checks wait for a later spec with law tables (RU-31)?
3. Should the bank lines of the pack that are timing items, interest and charges be shown only as "in one side only", as written above?

## As built (deviations from the text above, 4 November 2026)

- **Tax year from the run (`context.tax_year: { from_run: true }`).** The TDS files hold no tax year the profile language can read (a date is not a month, and two files have no date), so the plan to take it "from the client" was not possible. One small, deterministic addition to the profile language (`internal/profile`): the run's `--tax-year` is passed in `Source.TaxYear`; a file read with no run year is refused ("the tax year is not in the file and the run gave none"), never read under a guessed year. The sample test reads each sample for the tax year its expected file names. Go test: `internal/profile/spec020_test.go`. The six TDS formats use the whole year (`FY`) as their period so the files of a quarter pair up.
- **AC-9 (signed amounts) settled with no profile change.** Both sides keep two amounts, `receipt` and `payment` (a statement's credit is a receipt, its debit a payment), and `match_records` compares both.
- **RECON-TDS-012 compares different things from the plan.** OLTAS has no CIN column (BSR code, date and serial are separate), so Form 140's TDS cannot be added up by OLTAS challan. It compares the TDS the Form 140 rows deposit per CIN with the books' challan of that CIN. It finds challan `...00041` (7,000 against 6,000, which the pack lists as "claimed in statement") and `...00034` (15,000 against 15,675), which the pack does not list. TDS-011 carries the books-against-OLTAS comparison.
- **RECON-BNK-011 compares the opening balances only** (not the closing balances each side works out): that needs sums of rows from two record types in one check, which `sum_equals` does not do. The month comes from the file's `Sep-2026` (`period.format: "mon-yyyy"`).
- **`bank` is a new rule module** (`internal/rules.Modules`), as the bank rules do not belong to `tds`. The Rule Builder screen reads its module list from the engine.
- **Match keys.** Both sides of a rule must use the same key and amount names, so the vocabulary names them alike: `deductee`+`payment_date`, the challan identifiers, `deductor_tan`+`section`, and `reference`. `form140_challan_line` and `tds_challan_cin` are two extra record types (the same files read a second time by CIN).
- **Excel versions (added 4 November 2026):** eight formats (`generic_tally_xlsx/{tds_ledger,tds_challan,tds_receivable,bank_book}`, `generic_portal_xlsx/{form140,oltas_status,form168,bank_statement}`), each with two workbooks (the second on a sheet called Sheet1 under a title row, headings in capitals) that read to the same rows as the CSV samples (`TestTDSBankExcelSamples`). `bank_meta.json` is a small JSON file and has no Excel version.
- **File Guide updated (4 November 2026):** section 22 of `docs/Tool1-File-Guide.docx` lists every file this spec created (the old section 22 is now 23). Not done: the technical spec's list of new files and the traceability rows (added when the spec is Done). AC-11 stays open also because `TestExcelGivesTheSameFindings` still fails (see above).
- **Changed tests of earlier specs**, because this spec makes more of the pack readable: `TestGSTCompletionWholePackFolder` (4 files not read instead of 13), `TestTeachLayout` (more teachable document types), `TestRunSnapshot` (counts the rules of the run's tax year), `TestShippedProfiles` and `TestShippedComparisons` (counts).
- **Sample data regenerated (4 November 2026, with the user's approval)** with `TestWebSamplesMatchEngine -update`; this also brought in engine changes of SPEC-015 that the committed samples lacked, so `part9.test.tsx` (row counts 67 and 62, no rows left out in the sales register, the GSTR-1 tables `$.exp`, `$.cdnr`, `$.doc_issue`) and `part6.test.tsx` (rules of the sample's tax year only) were updated to the engine's output. Still failing, and not caused by this spec: `TestExcelGivesTheSameFindings` (the Excel sales register has 26 rows, the JSON one 67) and two tests in `web/src/spec013.test.tsx` (the Rules screen shows no Edit or Disable buttons after searching RECON-DEP).|

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
