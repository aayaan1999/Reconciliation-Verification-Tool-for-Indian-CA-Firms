# SPEC-027 — Rule and law-table governance, Rule Builder

| | |
|---|---|
| **Status** | Not started (who may disable a rule: T-Q3) |
| **Phase** | 4b (Phase 4b gate) |
| **Depends on** | SPEC-013, SPEC-025, SPEC-026 |
| **Requirements** | RU-11, RU-12, RU-20..25, RU-31, RU-34, RU-40..44, SC-02 |
| **Technical spec** | §B9, §B5 |
| **Code / content paths** | `internal/rules` (generator), `internal/api` (rules, law tables), `web/` (Rule Builder, Law tables screens), migrations for `rule_versions`, `rule_state` |

## Goal

The CA creates and edits rules and law-table values through forms and tests a draft rule on a past client; a Partner then activates it. Every version is kept and every change is logged with a reason.

## Scope

- **Rule Builder form:** document → figure (only readable facts, RU-22), check type, tolerance, severity, law reference, tax year, valid from/to, title, plain-English text, notes, review document.
- `rules.Generate()` produces canonical YAML from the form and runs the strict loader before saving (RU-21). Free text is stored only as a description (RU-23).
- Editing a built-in rule creates an office version; "Reset to standard rule" restores the built-in one (RU-12).
- Starlark rules: only `params` are editable (RU-24).
- **Test on a client** (RU-25): runs a `rule_test` job against a chosen past engagement and shows what the rule would flag, without touching findings.
- Lifecycle: `draft → testing → active`, and `disabled` from any state, with a reason (RU-40). Only a Partner can activate; an office rule needs a passing test run first (RU-43).
- Version list, with rollback to any earlier version (RU-41).
- **Law tables screen:** view tables; add a row with a new `valid_from`, which closes the previous row's `valid_to`; a Partner approves (RU-34).
- All governance actions write to `rule_state` and the version tables and, once SPEC-036 exists, to the audit log (RU-42).

## Out of scope

- Packs from USB and conflicts with office rules (SPEC-041).

## Acceptance criteria

- [ ] AC-1 — A CA creates a new `equal` rule through the form without typing YAML. The generated YAML passes the strict loader and round-trips back to the same form.
- [ ] AC-2 — Figure dropdowns contain only facts that an installed profile can produce.
- [ ] AC-3 — Test on a client shows the would-be findings and leaves the client's real findings unchanged.
- [ ] AC-4 — Staff cannot activate a rule; a Partner can. An office rule without a passing test cannot be activated.
- [ ] AC-5 — Editing a built-in rule and then resetting it restores the built-in version, and both versions stay in history.
- [ ] AC-6 — Disabling a rule takes one action with a reason, and the rule stops running from the next run.
- [ ] AC-7 — Adding a law-table row from a date changes results only for periods after that date.
- [ ] AC-8 — Phase 4b gate: a CA creates or edits a rule and tests it on a client, then a Partner activates it (manual walkthrough recorded).

## Verification

```
go test ./internal/rules/... ./internal/api/... -run 'Rule|Law' -v
cd web && npm test
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
