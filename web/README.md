# web

The browser UI for Tool 1: React + TypeScript, built with Vite. `npm run build` writes `web/dist/`, which `recon-server` embeds with `go:embed` (SPEC-026). Everything is bundled; the UI must never load anything from the internet, because the tool runs offline on the office LAN.

| Task | Command (run inside `web/`) |
|---|---|
| Install exact locked dependencies | `npm ci` |
| Dev server with hot reload | `npm run dev` |
| Tests (Vitest, once) | `npm test` |
| Lint (oxlint) | `npm run lint` |
| Type-check and production build | `npm run build` |
