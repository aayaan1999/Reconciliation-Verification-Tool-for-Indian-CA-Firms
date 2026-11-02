# Integrations

Purpose: every outside system Tool 1 reads from, the file format, the code that reads it, and what is built today. It summarises requirements §4 (IN-01..26) in [requirements-spec-tool1.md](../../requirements-spec-tool1.md), the technical spec §A6.1 (adapters), §A6.2 and §B2 (format profiles) and §B16 (build order), checked against `internal/ingest/adapters/*`, `content/profiles/` and [docs/fixtures.md](../fixtures.md).

## Principles

- **Files only, always offline.** Every source arrives as a file a staff member downloads or exports and uploads (the manual file-drop path, project invariant 7). API or ODBC access may later be an optimisation, never a dependency. There are no direct government-portal connections: the tool has no internet (requirements §1.2, §2).
- **Adapters know syntax, profiles know meaning (§A2, §A6).** An adapter turns bytes into a navigable document, one per file *type*. A format profile, one per market format *and version*, says where each figure sits and writes vocabulary facts and records. A profile never names a rule and a rule never names a format.
- **Structured before PDF.** Where a source gives JSON, Excel or XML, that is read; PDF is a fallback and must keep the column layout.
- **Identified by content, not extension.** `ingest.Registry.ForFile` needs exactly one adapter to recognise the file; otherwise `UNSUPPORTED_FILE_TYPE`.
- **All formats here are synthetic** until real samples arrive (IN-26, open question Q1). They are defined in [docs/fixtures.md](../fixtures.md) and are not the output of any real product.

## Adapters (`internal/ingest/adapters/`)

| Adapter | File type | Status | Notes |
|---|---|---|---|
| `json` | JSON | **Built** (SPEC-003) | Numbers kept as exact text, never through `float64`; strict (repeated keys, invalid UTF-8, trailing text refused); a small fixed JSON-path subset; default 64 MB limit (§A6.1) |
| `csv` | CSV | **Built** (SPEC-006 part 3) | UTF-8, first row the column names; read into the same tree as JSON, so profiles use JSON paths (`$[*]`, `.amount_inr`) |
| `xlsx` | Excel (.xlsx) | **Built** ([SPEC-008](../../specs/SPEC-008-excel-files-by-header-text.md)) | Read with `excelize` v2.11.0 from the uploaded bytes, stored values only (no rounding, formulas never recalculated); numbers as exact decimal text, date cells as `YYYY-MM-DD` (1900 and 1904 date systems), a formula with no saved value is missing. A profile names its sheets and the labels of each header row (`sheets:`); rows below it are keyed by header text, so profiles use JSON paths (`$['Sales Register'][*]`, `.['Voucher No']`) and locators show the sheet and cell (`Sales Register!C14`). Hidden sheets, rows and columns are read and reported (`HIDDEN_CONTENT`). Tables run past empty rows (`EMPTY_ROWS`), total rows can be left out (`total_rows`), a sheet can have any name (`any_name`), header labels match ignoring capitals, and Excel layouts can be taught on the mapping screen (SPEC-008 part 2); at most 1,000,000 filled cells. `.xls` and password-protected workbooks are refused with what to do instead. Default 64 MB limit; unzipped size at most four times that |
| `docx` | Word tables | Planned (Phase 3) | Package stub only; paragraph text is never interpreted |
| `tallyxml` | Tally XML export | Planned (Phase 3) | Package stub only; to read as a stream for large Day Books |
| `pdftext` | Text-based PDF | Planned (Phase 5) | Runs the vendored Poppler `pdftotext -layout`; only a test exists today |
| `fixedwidth` | Delimited and fixed-width text (TRACES, EPFO, ESIC) | Planned (Phase 5) | Package stub only |

The server accepts JSON, Excel (`.xlsx`) and CSV uploads (requirement IN-02). An old Excel file (`.xls`) is refused with a message to save it as `.xlsx` or `.csv`; anything else is `UNSUPPORTED_FILE_TYPE`. The mapping screen teaches JSON and CSV layouts only; an Excel layout is written as a profile.

## Sources

| Source | Documents | Format read | Profiles | Status |
|---|---|---|---|---|
| Tax suite, single documents | Computation of income (v1, v2, v3), tax audit report, financial statements | JSON | `content/profiles/generic/computation`, `tax_audit_report`, `financial_statements` | **Built** (SPEC-003, SPEC-004), synthetic |
| Tax suite filing pack | One JSON file holding the computation, tax audit report, Form 29B (MAT report), financial statements, Form 67 and Form 26AS | JSON bundle: each profile reads its own section | `content/profiles/taxsuite_pack/*`, `bundle: "taxsuite.filing_pack"` | **Built** (SPEC-005; Form 67 and 26AS in SPEC-006), synthetic Test Pack A |
| GST portal | GSTR-1 B2B invoices, GSTR-2B B2B invoices | JSON | `content/profiles/generic/gstr1`, `gstr2b` | **Built** (SPEC-004 part 8), synthetic GST pack; matched row by row with `match_records` |
| Books (registers) | Sales register, purchase register | JSON; the sales register also as Excel | `content/profiles/generic/sales_register`, `purchase_register`; `content/profiles/generic_xlsx/sales_register` | **Built** (SPEC-004 part 8; Excel SPEC-008), synthetic |
| Books (ledgers) | Monthly expense ledger, this year and last year | CSV (`period`, `ledger_head`, `group`, `amount_inr`) or Excel (sheet `Ledger`) | `content/profiles/generic/ledger`, `content/profiles/generic_xlsx/ledger` | **Built** (SPEC-006 part 3; Excel SPEC-008); feeds the Worklist only, never a rule |
| TRACES / income-tax portal | Form 26AS | JSON, inside the tax suite filing pack | `taxsuite_pack/form_26as` | **Built** as part of the pack (SPEC-006). TRACES text and PDF downloads, AIS, conso and justification files are planned |
| Form 67 (foreign tax credit) | Form 67 | JSON, inside the filing pack | `taxsuite_pack/form_67` | **Built** (SPEC-006) |
| Tally | Trial balance, ledger master, monthly summary, Day Book, registers | XML preferred, Excel accepted (IN-04) | none yet | Planned (Phase 3). Optional live pull by a separate `recon-collector`, Phase 3+, which still drops a file (§A3) |
| GST portal, other returns | GSTR-1A, 3B, 9, IMS | Excel / JSON | none yet | Planned (Phase 5, `gst` module) |
| EPFO / ESIC | ECR, contribution files | Text / Excel | none yet | Planned (Phase 5, `payroll` module) |
| Working schedules, client sheets | Any | Excel / Word | none yet | Planned (Phase 3) |

Which documents are compared together is set by the types of check in [content/comparisons/comparisons.yaml](../../content/comparisons/comparisons.yaml) (SPEC-004 part 8), for example `gst_sales_vs_gstr1` or `ledger_year_on_year`. The source formats the firm uses are tracked in requirements §4.3 and are still to be confirmed.

## Format profiles

- **Shipped profiles** live in [content/profiles](../../content/profiles/) as `<family>/<document>/v<n>/profile.yaml` (schema `recon.profile/1`, §B2.2), each with `samples/` and hand-verified `expected/*.facts.yaml`. A changed format gets a **new version folder**; a published profile is never edited in place.
- **Detection.** A profile's `detect` block (for example `json_exists`) must match. No match is `PROFILE_NO_MATCH`; more than one, outside a single bundle, is `PROFILE_AMBIGUOUS`.
- **Bundles (SPEC-005).** Profiles that share a `bundle` value each read their own section of one file, so one file can give several documents.
- **Context.** A profile reads the file's tax year, period, PAN (only the last four are reported) and scope (for example the GSTIN). A file for another tax year, or whose PAN is not the client's, gets a validation error and its figures are not used. Last year's ledger is accepted for the year-on-year comparison.
- **Lists.** A figure in a list is read from the entry that meets conditions (`rows`, `where`, `combine`), never by position (SPEC-006).
- **Testing.** `go run ./cmd/recon-dev profile-test` (and `TestShippedProfiles`) requires each sample to be detected as its own profile against every installed profile and to match its expected facts and locators exactly (§B2.3).

### Office profiles (layouts taught on the mapping screen)

A layout the shipped profiles do not know can be taught without code (SPEC-004 part 8): `recon-engine inspect` lists every field of a JSON file with an example value, and `profile-create` turns the user's mapping (JSON on standard input) into an office profile, or with `--check-only` previews the rows it would read. Office profiles are written to the folder given by `--office-profiles` (for `recon-server`, a `profiles` folder beside the database unless set) and runs read them with the same flag. The screen uses `POST /api/v1/inspect` and `POST /api/v1/profiles`. The Format Register over all profiles (§B2.4) is planned.

## Vendored Poppler for PDF

PDF text comes from Poppler 26.09.0 `pdftotext -layout`, vendored in [third_party/poppler](../../third_party/poppler/README.md) and run as a separate program on local files only (GPL; no Tool 1 code links it). Decision: [ADR-0003](../architecture/decisions/ADR-0003-bundle-poppler.md). Its test runs only on Windows, and some sandboxed Bash shells cannot start it, so run it from PowerShell there. Rules for PDF (§B17 T4, T7):

- column layout must be preserved, and every extractor validated against known-good fixtures;
- scanned PDFs are rejected, and there is no OCR in Tool 1;
- image pages of mixed PDFs are listed as not read, and figures on them become Cannot check (SPEC-044, planned).

## Engine network proof

Every adapter runs inside `recon-engine`, which must have no network capability. A third-party library used by an adapter may reach a parse-only network package only through a reviewed edge in `allowedNetworkImports` ([ADR-0001](../architecture/decisions/ADR-0001-engine-network-proof.md)). See [../quality/test-strategy.md](../quality/test-strategy.md).
