# SPEC-013 — Law tables: format, loader, as-of lookup

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 2 |
| **Depends on** | SPEC-007, SPEC-010 |
| **Requirements** | RU-31, RU-32, RU-33, RU-34, RU-53, SC-03 |
| **Technical spec** | §B5 |
| **Code / content paths** | `internal/lawtable`, `content/lawtables/`, migration adding `law_tables`, `law_rows`, `run_law_rows` |

## Goal

Values that change with law live in dated tables, and rules look them up by the date being checked. A law change is a new row, and older years keep their old values.

## Scope

- Schema `recon.lawtable/1` (§B5): `table`, `description`, `key`, `values` (typed: `money`, `rate`, `int`, `date`, `text`), `rows[]` with `id`, key values, value fields, `valid_from`, `valid_to`, `law_ref`.
- Loader (strict YAML) checks: unique row IDs; validity ranges for the same key must not overlap; `law_ref` is required.
- Lookup `Law(table, key, asOf)`: exactly one row must be valid on `asOf`. Otherwise a typed error makes the evaluation `CannotCheck` with "no law value for <table>/<key> on <date>".
- `asOf` comes from the period being checked (the rule declares `as_of: period_end | period_start | <date fact>`), never from the clock.
- Tables are loaded into the DB, and every lookup used in a run is recorded in `run_law_rows` (RU-53).
- The `approved_by` column exists; the approval workflow and editing screen are SPEC-027.
- Starter tables hold **illustrative** rows only, clearly marked: `tds.section_threshold`, `due_dates`, `gst.rate`, `section_map_1961_2025`, `regime_by_tax_year`. Real values are entered and approved by the CA (RU-34, A6).

## Out of scope

- Editing and approval UI (SPEC-027). Distribution by pack (SPEC-041).

## Acceptance criteria

- [ ] AC-1 — Overlapping ranges for the same key fail at load, naming both row IDs.
- [ ] AC-2 — Lookups on the day before `valid_from`, on `valid_from`, on `valid_to` and on the day after behave as documented (start and end both inclusive).
- [ ] AC-3 — Adding a new row with a later `valid_from` changes results for the new year only; a run for the old year is byte-identical before and after.
- [ ] AC-4 — A missing law value produces `CannotCheck`, not a failure or a zero.
- [ ] AC-5 — Each run records the law rows it used.
- [ ] AC-6 — Every starter row is marked illustrative, and a test fails if any unmarked row has no `approved_by`.

## Verification

```
go test ./internal/lawtable/ -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
