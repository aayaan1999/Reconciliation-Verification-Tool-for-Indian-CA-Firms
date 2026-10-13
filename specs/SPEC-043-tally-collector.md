# SPEC-043 — Tally live collector (optional)

| | |
|---|---|
| **Status** | Not started (optional; build only if the firm asks for it) |
| **Phase** | Later |
| **Depends on** | SPEC-020, SPEC-024 |
| **Requirements** | IN-04 ("optional live pull later"); CLAUDE.md invariant 7 (manual file-drop path always remains) |
| **Technical spec** | §A3 (`recon-collector`) |
| **Code / content paths** | `cmd/recon-collector`, `internal/collector` |

## Goal

Staff can pull Tally data on the office LAN with one click instead of exporting it by hand. The pulled data arrives as an ordinary file, so the rest of Tool 1 cannot tell the difference and the manual export path keeps working.

## Scope

- `recon-collector` is a separate process. It is the only Tool 1 process that talks to Tally, and only to a configured Tally host and port on the LAN.
- It requests the same reports as the manual export guide (trial balance, ledger master, monthly summary, Day Book, registers) in Tally's XML format, for a chosen company and period.
- Each response is written as an XML file into the engagement's upload inbox and goes through normal upload handling (SPEC-024): the same detection, the same profiles and the same validation report.
- Started from the Client file (Manager or Staff) for one engagement; the request and its result are written to the audit log.
- Clear errors when Tally is not running, the company is not open, or the port is closed. Nothing is retried in the background.

## Out of scope

- Any internet access. Writing anything back to Tally. Clients whose Tally books are kept off-site (they keep using the manual file-drop path).

## Acceptance criteria

- [ ] AC-1 — A pull against a test Tally instance produces files that give exactly the same facts and records as the same reports exported manually.
- [ ] AC-2 — With the collector switched off or not installed, every Tally workflow still works through manual upload.
- [ ] AC-3 — The engine and server binaries still pass their archtests; network code exists only in `recon-collector`.
- [ ] AC-4 — The collector connects only to the configured LAN host; any other address is refused.
- [ ] AC-5 — Each pull is in the audit log with user, engagement, reports and result.

## Verification

```
go test ./internal/collector/... -v
go test ./internal/archtest/
```

Plus a manual pull against the firm's Tally version (Q3), recorded below.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
