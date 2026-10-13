# SPEC-032 — TDS reconciliation module

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 5 |
| **Depends on** | SPEC-021, SPEC-029, SPEC-030 |
| **Requirements** | RC-04, IN-06, RU-31, RU-33 |
| **Technical spec** | §A6.4, §B8 (`tds`); v0.2 spec §8 |
| **Code / content paths** | `internal/modules/tds`, `content/profiles/traces/**`, `content/rules/<tax-year>/tds/**` |

## Goal

TDS payable (books vs challans vs quarterly statements vs TRACES) and TDS receivable (credits vs 26AS/AIS/Form 168 vs turnover) are reconciled, for both the 1961-Act and the 2025-Act form sets.

## Scope

- Profiles for challan files, quarterly statement summaries, TRACES conso/justification text files, and 26AS/AIS/Form 168 (text/JSON/PDF), using synthetic samples until real ones arrive.
- Vocabulary: `tds_entry`, `tds_challan` and `tds_credit` record types, plus facts per section per quarter.
- Rules:
  - section-wise base and TDS in books vs quarterly statements vs challans;
  - deposit date vs due date (from a law table);
  - expense heads classified under a TDS section (via the SPEC-021 worklist) that have no TDS entries → the computed base is shown;
  - receivable: books TDS credit vs 26AS/AIS/Form 168, and the R8 receipts-vs-turnover gap.
- Regime handling: rules are tagged by tax year, and section numbers are mapped via `section_map_1961_2025` for display (RU-33).

## Out of scope

- Applicability judgments ("TDS should have been deducted"). Rules report computed figures and user-confirmed classifications, nothing more.

## Acceptance criteria

- [ ] AC-1 — Synthetic payable and receivable packs produce exactly the expected findings, and the clean packs produce none.
- [ ] AC-2 — A head classified under a section with zero TDS entries yields a finding showing the head's total and the zero, with no "should" wording.
- [ ] AC-3 — A 2025-26 pack and a 2026-27 pack each run with their own regime's rules and forms.
- [ ] AC-4 — Late-deposit findings show the deposit date, the due date and the days late.
- [ ] AC-5 — Profile tests pass for every TDS profile.

## Verification

```
go test ./internal/modules/tds/ -v
go test ./testdata/... -run TestPacks/tds -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
