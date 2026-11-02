# Reconciliation & Verification Tool for Indian CA Firms (Tool 1)

An offline consistency layer for Indian CA firms. It reads what the accounting package (Tally), the tax suite, the GST portal, TRACES and EPFO/ESIC produce, and reports every place where they disagree, to the rupee, showing both values, the difference and where each came from. It also flags expense and income heads that are new or missing compared with the prior year. It does not prepare filings, keep books or give tax advice.

It runs as one program on one office Windows machine, with staff using it from a browser on the office LAN. It needs no internet connection, and the reconciliation engine contains no network code at all.

> All figures, PANs and entity names in this repository are fictional. Never commit real client data.

## Status

SPEC-001 to SPEC-006 are done; SPEC-007 (Test Pack A negative controls) is next. See [`specs/README.md`](specs/README.md). JSON and CSV inputs are read today; PDF and Excel follow.

## Quick start

Requires Go (see `go.mod`) and, for the web UI only, Node.js 24.

```sh
go build ./cmd/...
go test -count=1 ./...

# Load the rules and run the engine on a sample
go run ./cmd/recon-engine load-rules --db build/recon.db --store docs/Recon_Tool_YAML_Rule_Store/recon_yaml --rules content/rules
go run ./cmd/recon-engine run --db build/recon.db --input testdata/run/differs --tax-year 2025-26 --include-testing

# The screens
cd web && npm ci && npm run build && cd ..
go build -tags ui -o build/ ./cmd/recon-engine ./cmd/recon-server
build/recon-server --listen 127.0.0.1:8080 --db build/ui.db   # then open http://127.0.0.1:8080
```

On Windows, double-click `start-local.cmd` instead: it builds the screens and programs the first time, and again whenever the Go code or the screens have changed since the last build, starts the server on 127.0.0.1:8080 in its own window (no internet needed) and opens the browser. `stop-local.cmd` stops it; the data in `build/ui.db` is kept.

Every command is listed in [`CLAUDE.md`](CLAUDE.md#commands).

## Repository layout

| Path | Contents |
|---|---|
| `cmd/` | Programs: `recon-engine` (network-free engine CLI), `recon-server` (LAN web server), `recon-dev` (developer tools) |
| `internal/` | Go packages, one responsibility each (see each `doc.go`) |
| `content/` | Rules, profiles, vocabulary, comparison types and thresholds, all as data |
| `web/` | React + TypeScript UI, embedded in `recon-server` |
| `testdata/` | Synthetic filing packs, including Test Pack A (fictional Nimbus Logic Systems) |
| `third_party/` | Vendored Poppler `pdftotext` |
| `specs/` | Build units with acceptance criteria |
| `docs/` | Documentation by topic — start at [`docs/README.md`](docs/README.md) |
| `.claude/` | Rules, skills and agents for Claude Code |
| `CLAUDE.md` | Working guide: current state, commands, invariants |
| `flow.md` | How a change goes from requirement to release |

## Source documents

- [`recon-tool-spec.md`](recon-tool-spec.md) — product and engineering spec v0.2
- [`requirements-spec-tool1.md`](requirements-spec-tool1.md) — requirements v0.3 (wins for Tool 1)
- [`tool1-technical-spec.md`](tool1-technical-spec.md) — technical spec
