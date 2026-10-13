# SPEC-003 — Core domain model

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 1 |
| **Depends on** | SPEC-002 |
| **Requirements** | FV-01, FD-01, FD-02, RU-52, RU-53 |
| **Technical spec** | §A5, §A8, §B7.1 |
| **Code / content paths** | `internal/domain` |

## Goal

The pure types that cross package boundaries exist, with canonical encodings that make output byte-stable.

## Scope

- Types: `Locator`, `Fact`, `ValueKind`, `Record`, `Subject`, `LabeledValue`, `LawRef`, `EvalStatus`, `Evaluation`, `FindingState`, `Finding`, `RunSnapshot` (versions used: engine, vocabulary, profiles, rules, law rows, document versions).
- `Subject.Key()`: canonical string `period|scope|match_key`, used in stable IDs.
- `Locator.Display()`: short text for reports, e.g. `comp_20261008.json $.business_income.allowed_depreciation`, `TAR_v3.pdf p.7`, `TB.xlsx Trial Balance!D42`.
- Canonical JSON encoding for `Evaluation` and `Finding` (sorted keys, no insignificant whitespace, integers only) for golden files and determinism tests.
- `StableID(ruleID, engagementID, subject)` and `ValueFingerprint(values)` per §B7.1. These are pure functions here; the lifecycle using them is SPEC-025.

## Out of scope

- Persistence (SPEC-010), lifecycle transitions (SPEC-025).

## Design notes

- No I/O and no imports outside the standard library and `money`/`period`.
- `ValueFingerprint` must ignore locators, so the same figure on a different page of a new draft does not count as a change (FD-04).

## Acceptance criteria

- [ ] AC-1 — All types in §A5 exist with doc comments.
- [ ] AC-2 — `StableID` does not change when the rule version, run ID or locator changes; it changes when rule ID, engagement, period, scope or match key changes (table test).
- [ ] AC-3 — `ValueFingerprint` changes when any value changes and does not change when only locators change.
- [ ] AC-4 — Canonical JSON of the same `Evaluation` built in two different field orders is byte-identical.
- [ ] AC-5 — `Locator.Display()` produces the formats in the §6.9 example.
- [ ] AC-6 — The archtest confirms `internal/domain` imports only the standard library, `money` and `period`.

## Verification

```
go test ./internal/domain/ -v
go test ./internal/archtest/
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
