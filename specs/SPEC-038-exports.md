# SPEC-038 — Exports: Excel, watermarked PDF, client query letter

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 6 |
| **Depends on** | SPEC-025, SPEC-036 |
| **Requirements** | FD-08, NF-04 (export control), requirements §3 (who may export) |
| **Technical spec** | §A6.5, §B13 |
| **Code / content paths** | `internal/export` (server only) |

## Goal

Managers and Partners can export an engagement's findings to Excel or a watermarked PDF, and generate a client query letter. Every export is logged.

## Scope

- The `Exporter` interface (§A6.5) with three implementations.
- Excel: one sheet per review document, a cannot-check sheet and a worklist sheet. Values are numbers in rupees with paise, in Indian number format.
- PDF: the same content, with a watermark on every page showing the firm name, user, timestamp and export ID. The PDF library must be chosen and licence-reviewed (server side only).
- Client query letter: lists items in `decided_ask_client`, phrased as questions with the computed values. It uses a fixed template, and the banned-word test applies.
- Each export ID is recorded in the audit log with the list of findings included.

## Acceptance criteria

- [ ] AC-1 — Staff and Read-only users get 403 on export.
- [ ] AC-2 — Excel totals equal the findings' values exactly (paise-level check).
- [ ] AC-3 — Every PDF page carries the watermark.
- [ ] AC-4 — The query letter passes the banned-word test and lists only ask-client items.
- [ ] AC-5 — Each export creates an audit entry with its export ID.

## Verification

```
go test ./internal/export/... -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
