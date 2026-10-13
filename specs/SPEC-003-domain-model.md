# SPEC-003 — Core domain model

| | |
|---|---|
| **Status** | Done |
| **Phase** | 1 |
| **Depends on** | SPEC-002 |
| **Requirements** | FV-01, FD-01, FD-02, RU-52, RU-53 |
| **Technical spec** | §A5, §A8, §B7.1 |
| **Code / content paths** | `internal/domain` |

## Goal

The pure types that cross package boundaries exist, with canonical encodings that make output byte-stable.

## Scope

**`internal/domain`** (as built)

- `value.go`: `ValueKind` (`KindMoney`, `KindInt`, `KindText`, `KindDate`, `KindBool`, `KindRate`; the zero value is invalid) and `Value`, the one figure shared by `Fact` and `LabeledValue` (embedded, so `f.Money`, `f.Text` and so on read it). Constructors `MoneyValue`, `IntValue`, `TextValue`, `DateValue`, `BoolValue`, `RateValue`.
- `locator.go`: `Locator` and the kind constants `json_path`, `sheet_cell`, `table_cell`, `pdf_pos`, `xml_path`, `line`. `Locator.Display()`: `comp_20261008.json $.business_income.allowed_depreciation`, `TAR_v3.pdf p.7` (a `pdf_pos` path `p7:l23:c41-52` shows only its page; a path without a page number is shown as stored), `TB.xlsx Trial Balance!D42`.
- `fact.go`: `Fact`, `Record`.
- `evaluation.go`: `Subject` (period, scope, match key; the engagement is passed to `StableID` separately), `Subject.Key()` = `period|scope|match_key` with `|` and `\` escaped in period and scope so keys never collide (the match key is last and may contain `|`), `LabeledValue` (input name, label, value, locator), `LawRef`, `EvalStatus` (`pass`, `fail`, `cannot_check`; zero invalid), `Evaluation`.
- `finding.go`: `FindingState` with the eight states of §B7.2 (`withdrawn` is left out until T6 is decided), `Finding`.
- `snapshot.go`: `RunSnapshot` with `DocumentUse` (document version, type, SHA-256, profile ID and version), `RuleUse` and `LawRowUse` (table, row ID, content hash), plus engine and vocabulary versions.
- `identity.go`: `StableID(ruleID, engagementID, subject)` = first 16 characters of standard base32 of SHA-256 over the escaped key. `ValueFingerprint(values)` = lowercase hex SHA-256 of the canonical JSON of the values only (no locators, labels or input names), in the given order; errors on an invalid kind or non-UTF-8 text.
- `canonical.go`: `Evaluation.CanonicalJSON()` and `Finding.CanonicalJSON()`: sorted keys, no whitespace, integers only, no HTML escaping, nil and empty lists both `[]`, a value writes only the field its kind names, `missing_facts` sorted (a set), values and law refs in rule order. Invalid kinds, statuses, states and non-UTF-8 text are refused rather than encoded lossily.

## Out of scope

- Persistence (SPEC-010), lifecycle transitions (SPEC-025).

## Design notes

- No I/O and no imports outside the standard library and `money`/`period`.
- `internal/domain` was added to the archtest no-floating-point list (`moneyDirs`), since it carries money.
- `ValueFingerprint` must ignore locators, so the same figure on a different page of a new draft does not count as a change (FD-04).

## Acceptance criteria

- [x] AC-1 — All types in §A5 exist with doc comments.
- [x] AC-2 — `StableID` does not change when the rule version, run ID or locator changes; it changes when rule ID, engagement, period, scope or match key changes (table test).
- [x] AC-3 — `ValueFingerprint` changes when any value changes and does not change when only locators change.
- [x] AC-4 — Canonical JSON of the same `Evaluation` built in two different field orders is byte-identical.
- [x] AC-5 — `Locator.Display()` produces the formats in the §6.9 example.
- [x] AC-6 — The archtest confirms `internal/domain` imports only the standard library, `money` and `period`.

## Verification

```
go test ./internal/domain/ -v
go test ./internal/archtest/
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| 13 Oct 2026 | Claude Code, for the project owner | not yet committed | AC-1: all §A5 types plus `Value`, `Subject`, `LabeledValue`, `LawRef`, `FindingState`, `RunSnapshot`, each with doc comments; §A5 and §B7.1 of the technical spec updated to match. AC-2: `TestStableID` (4 unchanged, 5 changed cases) and `TestSubjectKeyIsUnambiguous`. AC-3: `TestValueFingerprint` (page, file, label, input name and off-kind fields leave it unchanged; a one-paisa change, swap, kind change, removal or addition, and a change of every kind alter it). AC-4: `TestEvaluationCanonicalJSONIgnoresFieldOrder` (two struct literals in different orders and one decoded from JSON with shuffled keys give identical bytes) plus an exact-bytes golden test. AC-5: `TestLocatorDisplay`. AC-6: proven by break-and-restore (an import of `internal/vocab` and a `0.18` float added to `internal/domain` were both reported, then removed). Full suite, `gofmt -l`, `go vet` (host and Windows) and `go build ./cmd/...` pass. File Guide section 7 added. |
