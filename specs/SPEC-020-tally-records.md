# SPEC-020 — Tally XML adapter and record store

| | |
|---|---|
| **Status** | Not started (Tally profiles blocked on Q3) |
| **Phase** | 3 |
| **Depends on** | SPEC-006, SPEC-010 |
| **Requirements** | IN-04, IN-11, SC-05 |
| **Technical spec** | §A5 (`Record`), §A9, §B1 (`records`), §B3 (record types) |
| **Code / content paths** | `internal/ingest/adapters/tallyxml`, `internal/store` (records), `content/profiles/tally/**`, `content/vocabulary/records.yaml` |

## Goal

Tally exports (XML preferred, Excel accepted) become row-level records: trial balance lines, monthly ledger totals and vouchers. They are stored so that 200,000 vouchers per client can be queried and matched quickly.

## Scope

- Tally XML adapter: a streaming parse (`encoding/xml` token reader, not a whole-document load). Encoding is detected from the BOM or the XML declaration, including UTF-16. XML-path selectors; the locator is `xml_path` plus the element index.
- Record mapping in profiles: record type, row selector, field and amount selectors, `match_key`.
- Records table and indexes per §B1; batch insert. The repository call `Records(engagement, type, period, scope)` returns rows sorted by `match_key`.
- Record types: `tb_line`, `ledger_master`, `ledger_month`, `voucher`, `register_line` (sales/purchase/payroll).
- Multiple Tally companies or branches: `scope` is the Tally company identifier (IN-11).
- Tally Excel exports use the SPEC-018 adapter with Tally Excel profiles.
- Staff export guide: a "Tally" section in `docs/export-guide.md` (A3).

## Out of scope

- Live pull from Tally (SPEC-043).

## Acceptance criteria

- [ ] AC-1 — Synthetic Tally XML (UTF-8 and UTF-16) with a TB, ledger master, monthly summary and Day Book extracts the expected records.
- [ ] AC-2 — A 200,000-voucher synthetic Day Book parses and stores in under 60 s, using under 500 MB of memory on the reference machine.
- [ ] AC-3 — Two Tally companies for one client are stored with distinct scopes and can be read separately or summed.
- [ ] AC-4 — In the synthetic sample, the TB debit and credit totals computed from the records equal the TB's own totals (an internal consistency check of the profile).
- [ ] AC-5 — Profile tests pass for at least one real Tally version (blocked on Q3; record which version was tested).

## Verification

```
go test ./internal/ingest/adapters/tallyxml/ ./internal/store/... -v
go test ./internal/ingest/adapters/tallyxml/ -bench . -run ^$
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
