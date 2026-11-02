# Traceability

Purpose: how a requirement is followed from [requirements-spec-tool1.md](../../requirements-spec-tool1.md) through [tool1-technical-spec.md](../../tool1-technical-spec.md) (§B18), the build units in [specs/](../../specs/README.md), the rules in `content/rules/`, the tests and fixtures, and the Test Pack A answer key in [score.yaml](../../testdata/testpack_nimbus/score.yaml). The table for SPEC-001 to SPEC-007 is copied from the spec file headers; the spec files win if they differ.

## The chain

```
Requirement ID ──► Tech spec § ──► SPEC-NNN ──► Rule (RECON-*) ──► Fixtures / Go tests ──► Test Pack A finding (F..) or control (N..)
```

| Link | Where it is recorded |
|---|---|
| Requirement → tech spec | §B18 of the technical spec (by requirement family) |
| Requirement, tech spec → spec | The **Requirements** and **Technical spec** rows of each spec header |
| Spec → rule | The spec's scope and paths; each rule folder `content/rules/<tax year>/<area>/RECON-*/rule.yaml` |
| Rule → fixtures | `fixtures/pass/` and `fixtures/fail/` beside each `rule.yaml`, checked by `go run ./cmd/recon-dev rule-lint` and `go test ./internal/engine/...` |
| Rule → Test Pack A | `detected_by` (findings) and `rules` (negative controls) in `testdata/testpack_nimbus/score.yaml`, scored by `TestTestPackAPass1` |
| Rule-book rule → engine rule | Open. The 304 rule-store files (`docs/Recon_Tool_YAML_Rule_Store/recon_yaml/`, IDs such as `RC-*`) are descriptive; the runnable `recon.rule/1` rules use `RECON-*` IDs. No field links one to the other yet |

## Requirement ID families

| Prefix | Area (requirements section) | Tech spec (§B18) |
|---|---|---|
| ENV-01..07 | Operating environment (§2) | A3, B10, B12 |
| IN-01..06, IN-10..14, IN-20..26 | Inputs, multiple files, format library (§4) | A6.1, A6.2, A7, B2.1–B2.4 |
| FV-01..04 | Standard fact vocabulary (§5) | A2, A5, B3 |
| RU-01..04, 10..13, 20..25, 30..34, 40..44, 50..53 | Rules: storage, Rule Builder, law tables, governance, execution (§6) | B4, B5, B6, B9, B10, B14 |
| RC-01..06 | Reconciliation functions (§7) | A6.4, B4.3, B8 |
| FD-01..08 | Findings and workflow (§8) | B4.6, B7, B11, B13 |
| SC-01..08 | Scalability (§9) | A1, A6, A9 |
| NF-01..09 | Non-functional (§10) | A8, B1.3, B12, B14 |

## Specs built so far

| Spec | Requirements | Tech spec | Rules added or changed | Main tests and checks | Test Pack A |
|---|---|---|---|---|---|
| SPEC-001 Rule store in YAML | RU-01, RU-02, RU-03, RU-30 | §B4.1, §B4.2 | 304 rule-store files, all `governance.enabled: false` | `recon-dev rulestore-import`, `rulestore-check`; `go test ./cmd/recon-dev/` | — |
| SPEC-002 Developer tools | NF-04 | §B15 | — | CI workflow; `TestEveryLinkedModuleHasALicenceRecord` | — |
| SPEC-003 Rules into DB, one run | RU-01, RU-02, RU-51, RU-52, RU-53, IN-11, IN-14, NF-01, NF-03 | §A7, §A8, §A10, §B1, §B2.2, §B4.2, §B4.4, §B4.6, §B6, §B15 | RECON-DEP-001, RECON-DEP-002 | `recon-engine run` on `testdata/run/clean` (exit 0), `differs` (exit 1), `unknown` (exit 2); archtest | F01 (later scored) |
| SPEC-004 JSON files, screens, rule problems | IN-01, IN-10..14, IN-20..23, FV-01, FD-02, FD-06, RU-01, RU-02, NF-01, NF-03, NF-04 | §A6.1, §A6.2, §B1.1, §B2.1–§B2.3, §B4.2, §B4.7, §B11, §B14, §B15 | RECON-GST-001, RECON-GST-013, RECON-PL-001, RECON-PL-002 | `recon-dev profile-test`, `rule-lint`, `rulestore-problems`; `go test ./internal/api/`; `TestWebSamplesMatchEngine`; `npm test` | — (N13 maps to RECON-PL-001 in `score.yaml`) |
| SPEC-005 Filing pack, Test Pack A Pass 1 | IN-10..14, IN-20..23, FV-01, RU-01, RU-02, NF-01, NF-03 | §B2.2, §B2.3, §B3, §B4.2, §B4.3, §B14 | RECON-MAT-001, RECON-MAT-002 | `TestTestPackAPass1`; `profile-test` | F01, F03; scoring of F01–F30, N01–N16 introduced |
| SPEC-006 Check types, Form 67 and 26AS, decisions, Worklist | RU-01, RU-02, RU-43, RU-52, FD-02, FV-01, NF-01, NF-03 | §B4.2, §B4.3, §B4.6, §B3, §B2.2 | RECON-PGBP-001, RECON-MAT-003, RECON-MAT-004, RECON-FTC-001..004, RECON-REC-001, RECON-TAR-008, -020..024, -026, -031..035, -040, -044, -050 | `TestTestPackAPass1`; `TestTestPackAWorklist`, `TestWorklistNeedsBothYears`, `TestFindingDecisions`; `go test ./internal/modules/newheads ./internal/ingest/adapters/csv ./internal/api` | Pass 1: 22 of 30, critical 4 of 4, 0 false positives; Worklist gives F27–F30 |
| SPEC-007 Negative controls, one defect at a time | RU-43, NF-01, NF-03 | §B4.3, §B14 | New control rules (IDs not yet assigned) | `TestTestPackA*` (planned) | N01–N16; one-defect test for the 22 JSON findings |

## Test Pack A findings to rules

From [score.yaml](../../testdata/testpack_nimbus/score.yaml). Differences are in rupees; "0.00" means the finding reports values found together, not a difference.

| Finding | Rules | Finding | Rules |
|---|---|---|---|
| F01 | RECON-DEP-001, RECON-DEP-002 | F16 | RECON-TAR-031 |
| F02 | RECON-PGBP-001 | F17 | RECON-TAR-032, RECON-TAR-033 |
| F03 | RECON-MAT-001, RECON-MAT-002 | F18 | RECON-REC-001 |
| F04 | RECON-MAT-003 | F19 | RECON-TAR-021 |
| F05 | RECON-FTC-001 | F20 | RECON-TAR-035 |
| F06 | RECON-FTC-002 | F21 | RECON-TAR-022 |
| F07 | RECON-FTC-003 | F22 | RECON-TAR-026 |
| F08 | RECON-FTC-004 | F23 | RECON-TAR-023, RECON-TAR-024 |
| F09 | RECON-TAR-040 | F24 | RECON-TAR-050, RECON-MAT-004 |
| F10 | RECON-TAR-008 | F11, F12 | Not in the JSON: Pass 3 (two 3CD drafts) |
| F13 | RECON-TAR-044 | F25, F26 | Not in the JSON: Pass 2 (PDFs) |
| F14 | RECON-TAR-020 | F27–F30 | Not in the JSON: Worklist from the ledger CSVs (`internal/modules/newheads`) |
| F15 | RECON-TAR-034 | | |

## Keeping it current

- When a spec adds a rule, add it to `score.yaml` if it maps to a finding or control (CLAUDE.md), and add a row or rule IDs here.
- When a spec is finished, add its row to the table above from its header.
- Requirement IDs not yet covered by any spec (for example ENV-05 packs, RU-10..13 Rule Builder, FD-08 exports, NF-05..09 security) are planned in the technical spec §B16; no spec exists for them yet.
