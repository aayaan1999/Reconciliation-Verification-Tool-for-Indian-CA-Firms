# SPEC-007 — Test Pack A, Pass 1: every negative control checked, and one defect at a time

| | |
|---|---|
| **Status** | Done (27 October 2026) |
| **Phase** | 1 |
| **Depends on** | SPEC-006 |
| **Requirements** | RU-43, NF-01, NF-03 |
| **Technical spec** | §B4.3, §B14 |
| **Code / content paths** | `content/rules/2025-26/tieout/`, `testdata/testpack_nimbus/`, `cmd/recon-engine/testpack_test.go`; also `internal/engine/checks/{sumequals,percentof}`, `internal/profile` (`where … in`), `content/vocabulary`, `content/profiles/taxsuite_pack` |

## Goal

Prove the zero false positives of Test Pack A, Pass 1 (JSON only), instead of inferring it. Today the 16 negative controls pass because no rule looks at them; after this spec a rule checks each one and passes on it, and each planted defect is shown to fire its own rules and nothing else. The README of the pack: "this matters more than the hit rate".

## Scope

### 1. Negative controls as checks

Rules that check each control, must pass on the pack, and catch the same defect in client files. Each has two pass and two fail fixtures and loads as testing.

| Control | Check |
|---|---|
| N01 | clause 18: opening + additions − depreciation = closing, for each block and in total |
| N03 | clause 27(a) ITC: opening + availed − utilised = closing |
| N04 | clause 44 adds up (v2): exempt + composition + other registered = total registered; + unregistered = total. No footing error on v2 (the README's trap) |
| N05 | normal tax: 25 per cent of total income, plus cess |
| N06 | the computation's 115JB block adds up, and MAT = 15 per cent with surcharge and cess |
| N07 | refund = tax liability − TDS − advance tax |
| N08, N09 | TDS at exactly 10 per cent under 194J and 194A |
| N10, N11 | the section 192 and 194J payment bases cover the related-party payments of clause 23 |
| N15 | the MAT credit table carries forward from one year to the next |

N02 (half-rate depreciation), N12 (24Q due dates), N13 (net profit ratios), N14 (115BAA not opted) and N16 (interest under 234A/B/C) are listed against the rules that read their figures, or recorded as having none.

`testdata/testpack_nimbus/score.yaml` lists each rule against its control; `TestTestPackAPass1` requires each to pass on the pack and on the corrected copy.

### 2. One defect at a time

Starting from the corrected copy of the pack, each of the 22 defects the JSON can show is planted on its own (the figures of the answer key), and exactly the rules mapped to it in `score.yaml` must fire: no other rule, and none of the negative-control rules.

## Out of scope

- Findings that need the PDFs, the two drafts of the tax audit report or the ledger CSVs (F11, F12, F25 to F30).
- Switching any rule on: the CA approves first.

## Acceptance criteria

- [x] AC-1 — Every control in the table has a rule that checks it, with two pass and two fail fixtures, and each passes on the pack and on the corrected copy.
- [x] AC-2 — `score.yaml` maps every negative control to its rules, or records that none reads its figures; the test fails if a mapped rule fails on its control.
- [x] AC-3 — The one-defect-at-a-time test passes for every defect the JSON can show.
- [x] AC-4 — Pass 1 still scores the result of SPEC-006 (22 or 21 of 30), critical 4 of 4, 0 false positives.
- [x] AC-5 — All tests and the architecture guard pass; the file guide and the technical spec are updated.

## Verification

```
go run ./cmd/recon-dev rule-lint
go test -count=1 ./cmd/recon-engine -run 'TestTestPackA' -v
go test -count=1 ./...
```

## What was built

- **19 rules**, each with two pass and two fail fixtures, loading as testing:

  | Control | Rules |
  |---|---|
  | N01 (and N02) | RECON-TAR-018 (each block of clause 18), RECON-TAR-019 (the total row) |
  | N03 | RECON-TAR-027 |
  | N04 | RECON-TAR-045 (registered columns), RECON-TAR-046 (registered + unregistered = total) |
  | N05 | RECON-TAX-001 (25 per cent), RECON-TAX-002 (cess), RECON-TAX-003 (total) |
  | N06 | RECON-MAT-005 (book profit), RECON-MAT-006 (15 per cent), RECON-MAT-007 (7 per cent surcharge), RECON-MAT-008 (cess), RECON-MAT-009 (total) |
  | N07 | RECON-TAX-004 |
  | N08, N09 | RECON-TDS-001 (194J, clause 34(a)), RECON-TDS-002 (194A, Form 26AS) |
  | N10, N11 | RECON-TDS-003, RECON-TDS-004 |
  | N15 | RECON-MAT-010 |

  N12, N14 and N16 have no rule; `score.yaml` says why. N13 stays with RECON-PL-001.
- **Check types:** `sum_equals` can test each row of one record type (one result per row, Cannot check when a row lacks an amount). `percent_of` can take its base as a sum of `parts` (cess on tax plus surcharge). Profiles accept `where … in: [values]`.
- **Vocabulary 9:** 38 figures and the record type `tar_depreciation_block`, read by the filing-pack profiles. Each was checked by hand against the pack.
- **Error in the pack itself (decided 27 October 2026):** N15 says the MAT credit table rolls forward, but for AY 2026-27 it is Rs 10 apart (6061420 + 5540210 − 755049 = 10846581 against 10846571). The AY 2025-26 row is also Rs 9,00,000 apart.
  - RECON-MAT-010 checks strictly and fails on the pack by exactly Rs 10.
  - `score.yaml` records that under `pack_errors`. It is not counted as a false positive, and the rule is not loosened.
  - The corrected copy now carries the carry-forward that follows from correcting F08 (10028309), and the rule passes there.
- **One defect at a time:** `plant` in `score.yaml` puts each defect's figures from the pack into the corrected copy. `TestTestPackAOneDefectAtATime` requires exactly the mapped rules to fire, for all 22.
  - The figures come from the pack, except where the pack mixes two defects. F03 plants Sl.19 at 15 per cent of the understated book profit (5568690, as in the answer key for F04). F08 plants the carry-forward that follows (10846581).

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| 27 October 2026 | Unnati (with Claude Code) | not committed yet | All 5 criteria met. Pass 1: 22 of 30, critical 4 of 4, 0 false positives. 13 of 16 negative controls are checked by rules; the other three are recorded. 1 error in the pack itself (N15, Rs 10). All 22 one-defect plants fire exactly their own rules. |
