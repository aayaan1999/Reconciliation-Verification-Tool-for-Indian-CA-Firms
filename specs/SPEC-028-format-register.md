# SPEC-028 — Format Register and vocabulary browser

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 4b |
| **Depends on** | SPEC-026 |
| **Requirements** | IN-25, FV-02, FD-07 |
| **Technical spec** | §B2.4, §B3 |
| **Code / content paths** | `internal/api` (formats, vocabulary), `web/` (Format Register screen), migration for `profiles` |

## Goal

Anyone can see which market formats the tool supports, in which versions, and which figures it can read from each.

## Scope

- `profiles` table, populated from the installed profiles.
- `GET /formats`: source system, document, format version, file type, profile version, date added, pack, and number of samples tested.
- `GET /vocabulary?readable=true`: facts grouped by document type, with the profiles that produce each one.
- Format Register screen with filters, and a drill-down showing which figures a profile reads and where (the selector).
- The format collection checklist (requirements §4.3) shown as a status view: which source systems have samples and profiles.

## Out of scope

- Installing profiles from packs (SPEC-041).

## Acceptance criteria

- [ ] AC-1 — Every installed profile appears, with all the register columns.
- [ ] AC-2 — The vocabulary browser shows which profiles produce each fact; facts with no producer are marked "not readable yet".
- [ ] AC-3 — The same data feeds the Rule Builder dropdowns (SPEC-027 AC-2).
- [ ] AC-4 — The screen is visible to all roles and has no edit actions.

## Verification

```
go test ./internal/api/... -run 'Format|Vocabulary' -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
