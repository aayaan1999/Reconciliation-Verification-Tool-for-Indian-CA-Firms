# SPEC-015 — Rule-book GST rules made runnable, batch 1: B2C large, B2C small and HSN summary

| | |
|---|---|
| **Status** | In progress |
| **Phase** | 3 |
| **Depends on** | SPEC-004 part 8 (GST invoice matching, `match_records`), SPEC-010 (rule lifecycle) |
| **Requirements** | RU-01, RU-02, RU-40, IN-10..14, IN-22 |
| **Technical spec** | §B2.2 (profile language), §B4.2 (rule files), §B4.7 (rule store), §6.4 (fixtures) |
| **Code / content paths** | `content/vocabulary/records.yaml`, `content/profiles/generic/gstr1/v2`, `content/profiles/generic/sales_register/v2`, `content/rules/2026-27/gst/`, `internal/profile` (`supersedes`, record `sign`, column `const`), `cmd/recon-engine/gstpack_test.go`, `testdata/gst_pack_aug2026/` |

## Goal

Three rules of the firm's rule book, which today are described only, run as checks: RC-GST-002 (B2C large invoices, Table 5), RC-GST-003 (B2C small summary by place of supply and rate, Table 7) and RC-GST-011 (HSN summary, Table 12). On the synthetic GST pack they give the results its answer key expects.

## Why only these three (decided 2 November 2026)

The user asked to make all 304 rule-book rules runnable. Of those, 214 statutory rules name no document to read, and 83 reconciliation rules need a document the tool does not read yet (general ledger, trial balance, GSTR-3B, TDS returns, payroll, bank statements, GSTR-9, AIS, EPFO and others). Seven need only documents the tool reads, but of those, RC-GST-006 needs amendment sections no sample has and the filing date for the s.37(3) limit, RC-GST-033 needs a GST rate master by HSN and date (a law table not built yet), and RC-GST-012 and RC-GST-034 each need a new check type. The user chose to build the three that today's `match_records` check can run.

## Scope

**Vocabulary (version 10).** Six record types:
- `sales_b2cl_invoice` and `gstr1_b2cl_invoice`: one B2C large invoice (key invoice number; place of supply, rate, date; taxable value, IGST, cess, invoice value).
- `sales_b2cs_line` and `gstr1_b2cs_line`: B2C small supplies by place of supply and rate (key place of supply and rate; taxable value, IGST, CGST, SGST, cess). The books' lines of one bucket are added together, as `match_records` does for the lines of one invoice.
- `sales_hsn_line` and `gstr1_hsn_line`: supplies by tab (B2B or B2C), HSN and rate (key tab, HSN, rate; taxable value, IGST, CGST, SGST, cess).

**Formats.** `generic.gstn.gstr1` v2 reads, besides the B2B invoices of v1, the `b2cl`, `b2cs`, `hsn.hsn_b2b` and `hsn.hsn_b2c` sections. `generic.books.sales_register` v2 reads, besides the B2B invoices of v1, the B2C large invoices, the B2C small lines and the HSN lines: invoices (B2B, B2C small, B2C large, exports) as they are, and credit notes negated, so the HSN lines are net of credit notes as Table 12 is. Each v2 has two samples with expected rows.

**Three additions to the profile language,** each small and declared in the file:
- `supersedes: "<version>"` on a profile: when a file matches both this profile and the version it supersedes, this one reads it. GSTN's GSTR-1 JSON carries no schema version, so v1 and v2 could otherwise not both be installed; v1 stays as published, so drafts it read are still explained by it.
- `sign: "negate"` on a record block: its amounts are negated (credit notes). Default `as_is`.
- `const: "<text>"` on a record field instead of `path`: the field has that value for every row of the block (the tab of a GSTR-1 HSN list, which the file shows only by the list's name).

**Rules,** each `recon.rule/1`, `match_records`, loaded as testing, with at least two passing and two failing fixtures, naming its rule-book rule:
- `RECON-GST-002` (RC-GST-002): `sales_b2cl_invoice` against `gstr1_b2cl_invoice`; compares taxable value, IGST, cess, invoice value, place of supply, rate and date.
- `RECON-GST-003` (RC-GST-003): `sales_b2cs_line` against `gstr1_b2cs_line`, by place of supply and rate; compares taxable value and each tax head. Tolerance Rs 10 per bucket, as the rule book says.
- `RECON-GST-011` (RC-GST-011): `sales_hsn_line` against `gstr1_hsn_line`, by tab, HSN and rate; compares taxable value and each tax head. Tolerance Rs 1 per line, as the rule book says.

## Out of scope

- The rule book's result codes that are judgments or need data the tool does not have: `REPORTED_IN_B2CS` and `WRONG_SECTION` (RC-GST-002) say where else an invoice went; the tool reports only what it computes, that the invoice is in the books and not in Table 5. `DIGITS_SHORT` and `HSN_INVALID` (RC-GST-011) need the client's turnover and an HSN master; the tool reports the two HSN rows that do not match. `TAB_TOTAL_DIFF` (Table 12 against GSTR-1's own invoice sections) and quantity checks (`QTY_DIFF`: quantities are not money, and the tool compares money only).
- RC-GST-006, RC-GST-012, RC-GST-033, RC-GST-034 (see above) and every other rule-book rule.
- Making a rule active: the CA approves each one (SPEC-010).

## Design notes

- A rule-book rule stays in the rule book, described only. The new engine rule names it (`law.reference`), as `RECON-GST-001` names RC-GST-001.
- Expected on the GST pack (`ANSWER_KEY.xlsx`): RECON-GST-002 reports invoice DBS/26-27/0463 as in the books and not in Table 5 (E09). RECON-GST-003 reports POS 27 at 18% with different amounts (E08), POS 30 at 18% as matched, and POS 33 at 18% as in GSTR-1 only (consequential to E09). RECON-GST-011 reports B2C, HSN 847130 at 18% as in the books only and B2C, HSN 8471 at 18% as in GSTR-1 only (E11 and its two consequential results); every other HSN line matches.

## Acceptance criteria

- [ ] AC-1 — `generic.gstn.gstr1` v2 and `generic.books.sales_register` v2 read their new sections; `recon-dev profile-test` passes for both (two samples each); a GSTR-1 file is read by v2, not refused as ambiguous, and v1 is unchanged.
- [ ] AC-2 — `supersedes`, record `sign` and column `const` are checked when a profile loads (a superseded version that is not installed, a sign other than `as_is` or `negate`, `const` with `path`) and each has a Go test.
- [ ] AC-3 — RECON-GST-002, RECON-GST-003 and RECON-GST-011 pass `recon-dev rule-lint` (two passing and two failing fixtures each) and load as testing.
- [ ] AC-4 — On the GST pack each gives exactly the results in Design notes, each finding with both values, the difference and both sources, and no other finding (Go test against the answer key).
- [ ] AC-5 — RECON-GST-001 and RECON-GST-013 give the same results as before, and Test Pack A still scores 22 of 30 with 0 false positives.
- [ ] AC-6 — `go test -count=1 ./...`, `gofmt`, `go vet` (host and Windows) pass; the technical spec and the file guide are updated.

## Verification

```
go run ./cmd/recon-dev profile-test
go run ./cmd/recon-dev rule-lint
go test -count=1 ./cmd/recon-engine -run 'GSTPack|TestTestPackAPass1' -v
go test -count=1 ./...
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
