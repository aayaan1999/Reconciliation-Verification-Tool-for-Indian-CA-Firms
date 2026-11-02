# Feature spec template

Purpose: an expanded template for new build units. It builds on [specs/TEMPLATE.md](../../specs/TEMPLATE.md) and the rules in [specs/README.md](../../specs/README.md), and adds dependencies, traceability IDs and the File Guide step.

New specs still live in `specs/` as `SPEC-NNN-short-title.md`, numbered after the last one, and get a row in the index table of [specs/README.md](../../specs/README.md). This file is guidance only; `specs/TEMPLATE.md` stays the minimum. Copy the block below into the new spec file and delete the guidance lines in italics.

## Rules for using it

- A spec says what will be built and how we prove it is done. It does not repeat the design; it points to [tool1-technical-spec.md](../../tool1-technical-spec.md).
- If building the spec changes the design, update `tool1-technical-spec.md` in the same commit. If it changes a requirement in [requirements-spec-tool1.md](../../requirements-spec-tool1.md), stop and ask.
- A human (the developer, and the CA for rule content) approves the spec before building starts. Agents may draft it; see [operating model](../agents/operating-model.md).
- Done is set by the builder when every criterion is ticked and verification passes. Verified is set only by a second person who re-ran verification.
- Illustrative data only. Never put real client data in a spec, fixture or sample.

## Template

````markdown
# SPEC-NNN — Title

| | |
|---|---|
| **Status** | Not started |
| **Phase** | n (requirements §13) |
| **Build Plan modules** | module numbers from the implementation plan §4 |
| **Depends on** | SPEC-xxx (must be Done) |
| **Requirements** | IDs from `requirements-spec-tool1.md` (ENV, IN, FV, RU, RC, FD, SC, NF) |
| **Technical spec** | § references in `tool1-technical-spec.md` |
| **Rules** | RECON-* IDs added or changed in `content/rules/` (or "none") |
| **Test Pack A** | finding and control IDs (F01–F30, N01–N16) this spec affects (or "none") |
| **Code / content paths** | where this spec's work lives |

## Goal

One or two sentences: what exists when this spec is done.

## Background

_Optional. Why now: a failed run, a review, a request (with its date)._

## Scope

- What is built, in numbered parts if large.

## Out of scope

- What is deliberately left to another spec (name it, or say "not yet specced").

## Dependencies

| Kind | Item | State |
|---|---|---|
| Spec | SPEC-xxx | Done |
| Firm / CA | e.g. approval, sample file, decision | Open / received (date) |
| Tool or library | e.g. Poppler, a new Go module (licence recorded in `docs/licences.md`) | |

## Design references

- Technical spec sections this spec follows, and ADRs in `docs/architecture/decisions/`.
- New packages: classified in `internal/archtest/archtest_test.go` (and `deterministicDirs` if deterministic).

## Design notes

Only what this spec needs beyond the technical spec.

## Invariant check

- [ ] Findings show both values, the difference and both sources; no "should", no judgment.
- [ ] Money in integer paise; no floats; no clock or randomness in rules.
- [ ] No network code on the engine side; no LLM in the finding path.
- [ ] Rules are YAML data with at least one pass and one fail fixture.
- [ ] A manual file-drop path exists for any new data source.

## Acceptance criteria

- [ ] AC-1 — a testable statement.
- [ ] AC-n — `go test -count=1 ./...`, `gofmt -l .` (prints nothing), `go vet` (host and Windows), and, if `web/` changed, `npm run lint` and `npm test` pass. The architecture guard passes.
- [ ] Every new rule is added to `testdata/testpack_nimbus/score.yaml` where it maps to a finding or control.
- [ ] Every file this spec created is added to `docs/Tool1-File-Guide.docx` in plain language.

## Verification

```
commands that prove each criterion, with expected exit codes
```

## Files created

| File | What happens in it | In File Guide |
|---|---|---|
| | | [ ] |

## Traceability

| Requirement | Tech spec | Rule | Test / fixture | Test Pack A |
|---|---|---|---|---|
| | | | | |

## Build notes

_Dated notes written while building: decisions, surprises, results._

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |

## Verification record

| Date | Verified by (not the builder) | Result |
|---|---|---|
| | | |
````

## Notes on the sections

- **Traceability** feeds [traceability.md](traceability.md). Fill it from the header IDs.
- **Files created** must match what goes into `docs/Tool1-File-Guide.docx`. Renames, moves and deletions update the guide in the same commit (CLAUDE.md).
- **Verification record** is new compared with `specs/TEMPLATE.md`. It records the second person's re-run that allows status Verified. Agents never fill it.
- **Commit** in the completion record may read "not committed yet" (as in SPEC-001 to SPEC-006) until the developer commits.
