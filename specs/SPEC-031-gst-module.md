# SPEC-031 — GST reconciliation module

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 5 |
| **Depends on** | SPEC-018, SPEC-029 |
| **Requirements** | RC-03, IN-06, IN-11, RU-30 |
| **Technical spec** | §A6.4, §B8 (`gst`); v0.2 spec §8 |
| **Code / content paths** | `internal/modules/gst`, `content/profiles/gstportal/**`, `content/rules/<tax-year>/gst/**`, `content/vocabulary/*` (GST entries) |

## Goal

Books, GSTR-1/1A, GSTR-3B, GSTR-2B/IMS and GSTR-9 are reconciled per GSTIN and per month, with every gap reported to the rupee.

## Scope

- Profiles for GST portal downloads (JSON and Excel) of GSTR-1/1A, 3B, 2B, 9 and IMS, using synthetic samples until real downloads are collected.
- Vocabulary: `gst_invoice` and `gst_summary` record types, plus summary facts per return table.
- `Prepare`: per-GSTIN, per-month aggregates, and supplier-wise summaries of 2B vs the purchase register.
- Rules (each with a law reference and fixtures):
  - outward: books vs GSTR-1/1A vs 3B (taxable value, tax by head, invoice count, month by month);
  - inward: purchase register vs 2B/IMS vs 3B ITC; supplier-wise gaps; ITC at risk as a computed amount;
  - control checks via law tables: Rule 37/37A timing, the e-invoice reporting window, the 3-year filing bar. These are reported as computed dates and days, never as "ineligible";
  - annual: GSTR-9 vs the sum of the monthly returns.

## Out of scope

- Any portal connection (requirements §1.2). Advice on ITC eligibility.

## Acceptance criteria

- [ ] AC-1 — A synthetic GSTIN-year pack with planted gaps produces exactly the expected findings, and the clean pack produces none.
- [ ] AC-2 — Two GSTINs for one client are reconciled separately.
- [ ] AC-3 — A missing month's return produces a cannot-check item for that month, not a mismatch.
- [ ] AC-4 — All GST rule wording passes the banned-word test, and ITC items show computed amounts only.
- [ ] AC-5 — Profile tests pass for every GST portal profile.

## Verification

```
go test ./internal/modules/gst/ -v
go test ./testdata/... -run TestPacks/gst -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
