# SPEC-026 — Web UI: Season board, Client file, Worklist

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 4 (Phase 4 gate) |
| **Depends on** | SPEC-024, SPEC-025 |
| **Requirements** | FD-02, FD-06, FD-07, NF-08, ENV-02, ENV-03 |
| **Technical spec** | §B11; v0.2 spec §11.2 |
| **Code / content paths** | `web/` (React + TypeScript), embedded by `cmd/recon-server` |

## Goal

Staff run the season from a browser: they see every client, open a client's findings grouped by the document to fix, decide them, and classify new heads.

## Scope

- **Season board:** one row per engagement showing status, open findings by severity, cannot-check count, oldest open finding and last run time, with filter and sort.
- **Client file:**
  - an upload area with a validation report per file;
  - findings grouped by review document. Each shows both values, the difference, the arithmetic (sum parts, Starlark steps), every source locator and the law reference;
  - decision buttons with keyboard shortcuts;
  - a cannot-check section;
  - a draft list with a draft-to-draft diff view (RC-06);
  - a finding history panel.
- **Worklist:** items grouped by kind; a classification form; confirm / ask client.
- Login screen, user menu and a job progress indicator.
- Keyboard-first: documented shortcuts to move, open, decide and go to the next client; a `?` overlay lists them.
- Hindi/English toggle for UI labels. Finding values and rule text stay as stored.
- No external fonts, CDNs or analytics; everything is bundled.

## Out of scope

- Rule Builder, Law tables, Format Register and Admin screens (SPEC-027, SPEC-028, SPEC-035–040).

## Acceptance criteria

- [ ] AC-1 — Using only the keyboard, a staff user can upload, run, open a finding, decide it, upload a corrected file and see the finding resolved.
- [ ] AC-2 — Every finding view shows both values, the difference, the locators and the law reference; no view states a "correct" value.
- [ ] AC-3 — Findings are grouped by review document.
- [ ] AC-4 — The client file for a client with 500 findings opens in under 2 s on a staff PC.
- [ ] AC-5 — The UI works with the main desktop disconnected from the internet (manual check).
- [ ] AC-6 — The Hindi/English toggle switches all UI labels, and a test fails on any missing translation.
- [ ] AC-7 — Read-only users see no decision controls.

## Verification

```
cd web && npm ci && npm test && npm run build
go run ./cmd/recon-server --config dev.toml   # manual walkthrough of AC-1
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
