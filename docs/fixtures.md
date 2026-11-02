# Synthetic fixture formats

Until real tax-suite samples arrive (requirement IN-26, open question Q1), Tool 1 is built and tested against
**generic synthetic formats** defined on this page. They are not the output of any real product. Every
figure, PAN and name in them is invented; none belongs to a real client.

The formats are read by the profiles under `content/profiles/generic/`, and each profile folder holds its
samples and their hand-verified expected facts. `go run ./cmd/recon-dev profile-test` checks them all, and
CI runs the same check (`TestShippedProfiles` in `internal/profile`).

When real samples arrive, each real format gets its own profile folder beside these. The generic formats stay,
because the rule fixtures are built on them.

## Conventions shared by every generic format

- A JSON file in UTF-8, one object at the top.
- `meta.form` says which document the file is, and `meta.schema_version` says which version of the format it
  is. Together they are what detection tests, so exactly one profile matches each file.
- `meta.tax_year` is the assessment's financial year written `YYYY-YY` (`"2025-26"`). Every document in these
  formats covers a whole tax year (the profile's period is `const: "FY"`).
- `assessee.pan` is the PAN as text. Only its last four characters ever appear in a report.
- Amounts are rupees. They are normally JSON numbers with two decimal places (`112450.50`). A file may instead
  write an amount as text with Indian digit grouping (`"1,20,000.00"`); both read to the same paise.
- Fields not listed below may appear. The tool does not read them, and the upload's validation report lists
  them as "not read".

## Generic computation of income, version 1

`meta.form` = `"COMPUTATION"`, `meta.schema_version` = `"1.0"`.
Profile: `generic.taxsuite.computation` v1 (`content/profiles/generic/computation/v1`).

| Field | Type | Meaning | Read as |
|---|---|---|---|
| `meta.form` | text | Always `"COMPUTATION"` | detection |
| `meta.schema_version` | text | Always `"1.0"` | detection |
| `meta.tax_year` | text | Financial year, `"2025-26"` | tax year; period `FY2025-26` |
| `meta.software` | text | Name of the producing software | not read |
| `assessee.pan` | text | PAN of the assessee | client match |
| `assessee.name` | text | Name of the assessee | not read |
| `assessee.status` | text | Company, firm, LLP and so on | not read |
| `business_income.net_profit_as_per_pl` | amount | Net profit as per the profit and loss account | not read (yet) |
| `business_income.depreciation_as_per_books` | amount | Depreciation charged in the books | not read (yet) |
| `business_income.allowed_depreciation` | amount | Depreciation allowed under the Income-tax Act | `computation.allowed_depreciation` (required) |
| `fixed_assets.blocks[]` | list | One row per block of assets | not read |
| `fixed_assets.blocks[].block` | text | Block name and rate | not read |
| `fixed_assets.blocks[].depreciation` | amount | Depreciation on the block | not read |
| `fixed_assets.schedule_total` | amount | Total of the computation's own fixed-asset schedule | `computation.fixed_asset_schedule.total` (required) |
| `annexure.notes` | text | Free notes | not read |

Samples:

- `comp_2025-26_agrees.json`: allowed depreciation and the schedule total agree (₹1,12,450.50).
- `comp_2024-25_differs.json`: allowed depreciation is ₹87,300.00 but the schedule total is ₹87,030.00. This
  ₹270 difference is planted on purpose, for the rules built in later SPECs to find.

## Generic computation of income, version 2

`meta.form` = `"COMPUTATION"`, `meta.schema_version` = `"2.0"`.
Profile: `generic.taxsuite.computation` v2 (`content/profiles/generic/computation/v2`).

This is version 1 with one change: the business-income block moved from `business_income` to
`income_heads.business`, and it no longer carries `depreciation_as_per_books`. Supporting it took only a new
profile folder; no program code changed (requirement IN-24). Every other field is as in version 1, and
`annexure` is absent.

| Field | Type | Meaning | Read as |
|---|---|---|---|
| `income_heads.business.net_profit_as_per_pl` | amount | Net profit as per the profit and loss account | not read (yet) |
| `income_heads.business.allowed_depreciation` | amount | Depreciation allowed under the Income-tax Act | `computation.allowed_depreciation` (required) |

Samples:

- `comp_v2_2025-26.json`: amounts as JSON numbers.
- `comp_v2_text_amounts.json`: every amount written as text with Indian digit grouping (`"1,20,000.00"`).

## Generic computation of income, version 3 (full tax-suite export)

`meta.form` = `"COMPUTATION"`, `meta.schema_version` = `"3.0"`.
Profile: `generic.taxsuite.computation` v3 (`content/profiles/generic/computation/v3`).

A dummy document shaped like a full computation export from a tax suite: every head of income, the
additions and deductions that turn book profit into business income, gross total income, Chapter VI-A
deductions, total income (computed and rounded), the tax liability, taxes paid and the fixed-asset
schedule. Every figure is invented, and each sample's own arithmetic adds up (each total equals the sum of
its parts). The tax figures are illustrative only and are not computed under any real rate.

| Field | Type | Meaning | Read as |
|---|---|---|---|
| `meta.form`, `meta.schema_version` | text | `"COMPUTATION"`, `"3.0"` | detection |
| `meta.tax_year` | text | Financial year, `"2025-26"` | tax year; period `FY2025-26` |
| `meta.product`, `product_version`, `exported_on`, `assessment_year`, `return_form`, `notice` | text | About the export | not read |
| `assessee.pan` | text | PAN of the assessee | client match |
| `assessee.name`, `status`, `residential_status`, `date_of_incorporation`, `address.*` | text | About the assessee | not read |
| `income.house_property.*` | amounts | Annual value, standard deduction, interest, income | not read |
| `income.business_profession.net_profit_as_per_pl` | amount | Net profit as per the profit and loss account | `computation.net_profit_as_per_pl` (required) |
| `income.business_profession.additions[]`, `total_additions` | list, amount | Each addition with its section, and their total | not read |
| `income.business_profession.deductions[]`, `total_deductions` | list, amount | Each deduction with its section, and their total | not read |
| `income.business_profession.allowed_depreciation` | amount | Depreciation allowed under the Income-tax Act | `computation.allowed_depreciation` (required) |
| `income.business_profession.income` | amount | Business income | `computation.business_income` (required) |
| `income.capital_gains.*`, `income.other_sources.*` | amounts | The other heads of income | not read |
| `gross_total_income` | amount | Total of all heads | `computation.gross_total_income` (required) |
| `chapter_via.deductions[]`, `chapter_via.total` | list, amount | Chapter VI-A deductions | not read |
| `total_income.computed` | amount | Gross total income less Chapter VI-A | not read |
| `total_income.rounded_u_s_288a` | amount | Total income as rounded | `computation.total_income` (required) |
| `tax.tax_on_total_income`, `surcharge`, `health_and_education_cess` | amounts | Parts of the tax | not read |
| `tax.total_liability` | amount | Total tax liability | `computation.tax.total_liability` (required) |
| `taxes_paid.tds` | amount | TDS claimed | `computation.taxes_paid.tds` (required) |
| `taxes_paid.advance_tax` | amount | Advance tax paid | `computation.taxes_paid.advance_tax` (required) |
| `taxes_paid.tcs`, `self_assessment_tax`, `total`, `balance_payable` | amounts | The other taxes paid, and the balance | not read |
| `fixed_assets.blocks[]` | list | Each block with opening WDV, additions and depreciation | not read |
| `fixed_assets.schedule_total` | amount | Total of the fixed-asset schedule | `computation.fixed_asset_schedule.total` (required) |

Samples:

- `comp_v3_2025-26_agrees.json`: Illustrative Industries Private Limited, a company. Allowed depreciation and the
  schedule total agree (₹1,12,450.50).
- `comp_v3_2025-26_differs.json`: Sample Engineering Works, a firm. Allowed depreciation is ₹98,760.00, but the
  schedule's blocks add up to ₹98,670.00: a ₹90 transposition planted for RECON-DEP-001 to find. The same file
  is the `testdata/run/taxsuite` input.

## Generic financial statements (the client's accounts), version 1

`meta.form` = `"FINANCIAL_STATEMENTS"`, `meta.schema_version` = `"1.0"`.
Profile: `generic.accounts.financial_statements` v1 (`content/profiles/generic/financial_statements/v1`).

A dummy export of the profit and loss statement from a client's accounting software: the report the client
gives the firm (SPEC-004 part 7). Each sample's own sums add up.

| Field | Type | Meaning | Read as |
|---|---|---|---|
| `meta.form`, `meta.schema_version` | text | `"FINANCIAL_STATEMENTS"`, `"1.0"` | detection |
| `meta.tax_year` | text | Financial year, `"2025-26"` | tax year; period `FY2025-26` |
| `meta.product`, `exported_on`, `period_from`, `period_to`, `notice` | text | About the export | not read |
| `entity.pan` | text | PAN of the client | client match |
| `entity.name`, `entity.constitution` | text | About the client | not read |
| `profit_and_loss.revenue_from_operations` | amount | Revenue from operations | `fs.pl.revenue_from_operations` (required) |
| `profit_and_loss.other_income`, `total_income` | amounts | Other income, and the total | not read |
| `profit_and_loss.expenses.depreciation_and_amortisation` | amount | Depreciation and amortisation | `fs.pl.depreciation` (required) |
| `profit_and_loss.expenses.*` (the others), `total_expenses` | amounts | The other expenses, and the total | not read |
| `profit_and_loss.profit_before_tax` | amount | Profit before tax | `fs.pl.profit_before_tax` (required) |
| `profit_and_loss.tax_expense.*` | amounts | Current, deferred and total tax | not read |
| `profit_and_loss.profit_after_tax` | amount | Profit after tax | `fs.pl.profit_after_tax` (required) |
| `signatories.*` | text | Who prepared and approved it | not read |

Samples:

- `fs_2025-26_agrees.json`: Illustrative Industries Private Limited (PAN ending 234Q). Profit before tax
  (₹28,45,000.00) and depreciation (₹1,31,200.00) agree with `comp_v3_2025-26_agrees.json`.
- `fs_2025-26_differs.json`: Sample Engineering Works (PAN ending 678M). Profit before tax is ₹12,63,540.00 here
  but ₹12,36,540.00 in `comp_v3_2025-26_differs.json`: a ₹27,000 transposition planted for RECON-PL-001.
  Depreciation (₹1,04,300.00) agrees.

Computation v3 also gives `computation.depreciation_as_per_books` from
`income.business_profession.depreciation_as_per_books` (added for SPEC-004 part 7).

## Generic tax audit report, version 1

`meta.form` = `"TAX_AUDIT_REPORT"`, `meta.schema_version` = `"1.0"`.
Profile: `generic.taxsuite.tax_audit_report` v1 (`content/profiles/generic/tax_audit_report/v1`).

| Field | Type | Meaning | Read as |
|---|---|---|---|
| `meta.form` | text | Always `"TAX_AUDIT_REPORT"` | detection |
| `meta.schema_version` | text | Always `"1.0"` | detection |
| `meta.tax_year` | text | Financial year, `"2025-26"` | tax year; period `FY2025-26` |
| `meta.report_form` | text | `"3CA-3CD"` or `"3CB-3CD"` | not read |
| `meta.software` | text | Name of the producing software | not read |
| `assessee.pan` | text | PAN of the assessee | client match |
| `assessee.name` | text | Name of the assessee | not read |
| `clause_18.blocks[]` | list | Clause 18 depreciation table, one row per block | not read |
| `clause_18.blocks[].block` | text | Block name and rate | not read |
| `clause_18.blocks[].depreciation` | amount | Depreciation on the block | not read |
| `clause_18.total_depreciation` | amount | Total of the clause 18 depreciation table | `tar.depreciation_schedule.total` (required) |
| `clause_44` | object | Clause 44 details | not read |

Samples:

- `tar_2025-26_agrees.json`: the same assessee as `comp_2025-26_agrees.json`; the total agrees with the
  computation (₹1,12,450.50).
- `tar_2024-25_differs.json`: the same assessee as `comp_2024-25_differs.json`. The clause 18 total is
  ₹87,300.00, matching the computation's allowed depreciation but not its schedule total (₹87,030.00).

## Excel layouts (SPEC-008)

Two synthetic Excel layouts show that a workbook reads to exactly what its JSON or CSV source reads to.
They are written from those samples by `go test ./cmd/recon-engine -run TestExcelSamples -update`, which
also writes their expected files; every test run checks that each workbook still gives the same context
and rows as its source, apart from the locators.

**Sales register** (`content/profiles/generic_xlsx/sales_register/v1`, from the JSON sales register):

- Sheet `Details`: row 1 the labels `Document`, `Client GSTIN`, `Return Period`; row 2 their values
  (`SALES_REG`, the GSTIN, the period as text `082026`).
- Sheet `Sales Register`: a title in B1, row 2 empty, column A empty, and the header in row 3 from column B.
  Every JSON key is a column, in the reverse of the JSON order, with a label made from the key
  (`party_gstin` is `Party GSTIN`, `sgst_utgst` is `SGST UTGST`). Numbers are stored as numbers, written
  from the JSON text (never through floating point); the voucher date is an Excel date with the format
  `dd-mm-yyyy`; codes such as `27` or `847130` are text. An empty or null value leaves the cell empty.

**Expense ledger** (`content/profiles/generic_xlsx/ledger/v1`, from the CSV ledgers of Test Pack A):

- Sheet `Ledger`: header in row 1, `Ledger Head`, `Group`, `Month`, `Amount (Rs)`, in that order (the CSV
  has `period, ledger_head, group, amount_inr`). Months are text (`2024-04`); amounts are numbers.

A locator in these files is the sheet and row of a record (`Sales Register!4:4`) or the sheet and cell
of a value (`Sales Register!C14`).

## Expected-facts files

Each sample has a hand-verified `expected/<sample name>.facts.yaml` (schema `recon.expected/1`). It lists the
context (tax year, period, last four PAN characters, and scope and part where a format has them) and every
fact: name, value (money as rupees with two decimals), period and locator. A sample passes only if the tool
recognises it as its own profile and no other, reads it without errors, and gives exactly these facts.
