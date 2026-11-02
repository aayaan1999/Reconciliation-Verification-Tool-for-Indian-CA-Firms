# testdata/run

Inputs for `recon-engine run` (SPEC-003). Each folder is one `--input`. The files are synthetic, in the shape of the generic tax-suite samples in `content/profiles/generic/` (see `docs/fixtures.md`). Every name, PAN and figure is invented.

| Folder | What it holds | Expected result with `--tax-year 2025-26 --include-testing` |
|---|---|---|
| `clean` | The computation sample whose allowed depreciation agrees with its fixed-asset schedule | Exit 0: no findings. RECON-DEP-002 is Cannot check because no tax audit report is given |
| `differs` | The same computation with the schedule total raised by Rs 2,750 (a planted difference) | Exit 1: one RECON-DEP-001 finding with both figures, the difference and both locators |
| `missing` | The clean computation, plus a tax audit report with its depreciation total removed | Exit 0: the report's validation shows `REQUIRED_FIELD_MISSING`, and RECON-DEP-002 is Cannot check naming `tar.depreciation_schedule.total` |
| `taxsuite` | A fuller tax-suite export of the computation (generic computation v3) with a planted Rs 90 transposition: allowed depreciation 98,760, schedule total 98,670 | Exit 1: one RECON-DEP-001 finding showing the Rs 90 difference |
| `compare-clean` | The client's accounts and our computation for the same company, agreeing (financial statements v1 and computation v3) | Exit 0: RECON-PL-001 and RECON-PL-002 pass; RECON-DEP-002 is Cannot check (no tax audit report) |
| `compare-differs` | The client's accounts and our computation for Sample Engineering Works: profit before tax 12,63,540 in the accounts but 12,36,540 in the computation | Exit 1: a RECON-PL-001 finding of Rs 27,000, plus the Rs 90 RECON-DEP-001 finding in the computation |
| `unknown` | A JSON file that no profile recognises | Exit 2: `PROFILE_NO_MATCH` with the file name |
| `damaged` | A computation with a stray comma, so it is not valid JSON | Exit 2: `PARSE_ERROR` with the file name, line and column |

Every file directly inside the folder is read, in name order, so do not add notes or other files to these folders.
