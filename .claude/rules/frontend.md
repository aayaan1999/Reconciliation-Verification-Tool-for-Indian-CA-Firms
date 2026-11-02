---
paths:
  - "web/**"
---

# Frontend (web/)

React + TypeScript, built with Vite, tested with Vitest, linted with oxlint. Node.js 24 and npm are development tools only; the built UI is embedded in `recon-server` with `go:embed` (`-tags ui`). Users reach it from a browser on the office LAN.

## Rules

- **Offline only.** No CDNs, web fonts, remote images, analytics or online lookups. Every asset is bundled.
- **The UI never decides a finding.** It shows what the engine returned: both values, the difference, both source locators and the arithmetic. It never computes a different result, hides a finding or rewords it to say "should" or state a correct value. Wording helpers live in `web/src/wording.ts`.
- **Group findings by the document that needs fixing**, not by rule category (§11.2).
- **Keyboard-first.** Every action on the Season board, Client file, Worklist and Rules screens is reachable from the keyboard (see `web/src/useRowNav.ts`).
- Money arrives as integer paise and is formatted for display only (`web/src/format.ts`, Indian digit grouping). Never do money arithmetic in floating point in the UI.
- Sample data in `web/src/sample/` is real engine `--json` output. Do not hand-edit it; regenerate it with `go test ./cmd/recon-engine -run TestWebSamplesMatchEngine -update`.
- Match the existing style in `web/src/index.css` and the screens in `web/src/screens/`.

## Before finishing

Inside `web/`: `npm run lint`, `npm test` and `npm run build` all pass. If the build changed what is embedded, `go test -count=1 -tags ui ./web/` passes too.
