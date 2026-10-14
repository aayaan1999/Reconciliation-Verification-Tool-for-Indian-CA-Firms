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

## Expected-facts files

Each sample has a hand-verified `expected/<sample name>.facts.yaml` (schema `recon.expected/1`). It lists the
context (tax year, period, last four PAN characters, and scope and part where a format has them) and every
fact: name, value (money as rupees with two decimals), period and locator. A sample passes only if the tool
recognises it as its own profile and no other, reads it without errors, and gives exactly these facts.
