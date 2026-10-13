# SPEC-009 — Finding wording and report rendering

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 1 |
| **Depends on** | SPEC-008 |
| **Requirements** | FD-02, NF-08, requirements §6.9; CLAUDE.md invariant 1 |
| **Technical spec** | §B4.6 |
| **Code / content paths** | `internal/findings/templates`, `internal/findings/render` |

## Goal

Every finding is rendered from a fixed template that shows both values, the difference, every source and the law reference, and never says what a value "should" be.

## Scope

- One text template per check type; `equal` first, others added with their check types.
- Text report: findings grouped by `review_document`, then by rule ID; a separate "Cannot check" section listing missing figures and the document they were expected in.
- Canonical JSON report (same content, machine-readable) for golden tests.
- Indian digit grouping via `money.FormatIndian`.
- Banned-word test: renders every template and every rule title with fixture data and fails on `should`, `incorrect`, `must be`, `wrong`, `correct value`, `ought`, `required to` (case-insensitive, whole words).

## Out of scope

- HTML/UI rendering (SPEC-026), Excel/PDF exports (SPEC-038), Hindi labels (SPEC-026).

## Design notes

- `rule_text_en` is a description shown on the rule screen. It is not part of a finding's text, so its "must equal" wording is allowed.

## Acceptance criteria

- [ ] AC-1 — The `equal` finding for the requirements §6.9 example renders byte-for-byte as the golden file `testdata/golden/RECON-DEP-002.txt` (which reproduces the §6.9 block).
- [ ] AC-2 — The banned-word test fails when a banned word is placed in a template or a rule title (checked once, then reverted).
- [ ] AC-3 — A cannot-check item renders as `Cannot check — figure not found in <document name>`.
- [ ] AC-4 — Findings are grouped by review document in a stable order.
- [ ] AC-5 — The JSON report for the same run is byte-identical across two runs.

## Verification

```
go test ./internal/findings/... -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
