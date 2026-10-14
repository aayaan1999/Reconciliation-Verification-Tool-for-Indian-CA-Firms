# SPEC-002 — Developer tools setup

| | |
|---|---|
| **Status** | In progress |
| **Requirements** | NF-04 |
| **Technical spec** | §B15 |
| **Paths** | developer machine; `.github/workflows/ci.yml` |

## Goal

Every tool needed to build, test and validate the project is installed on the developer machine and in CI, so all of them can run side by side.

## Scope

All tools are free. Install on the developer machine:

| Tool | Licence | Purpose |
|---|---|---|
| Go 1.27 (`C:\Program Files\Go\bin`, on `PATH`) | BSD | Language for every program |
| Git | GPL (tool only) | Version control, GitHub |
| Node.js + npm | MIT | Builds the React + TypeScript UI |
| Poppler `pdftotext.exe` | GPL | PDF text; shipped as a separate program, never linked in. Check the licence before sharing the tool with other firms |
| WiX Toolset | MS-RL | Builds the Windows installer |
| Microsoft Word | Already licensed | Read-only checks of `docs/Tool1-File-Guide.docx` |

Downloaded by Go itself (`go.mod`), no manual install:

| Library | Licence | Purpose |
|---|---|---|
| SQLite via `ncruces/go-sqlite3` (SQLite compiled to WebAssembly, run in pure Go) | Public domain / MIT | Database; pure Go, so no C compiler and no cgo. Encryption at rest is designed without SQLCipher |
| `go.yaml.in/yaml/v3` | MIT / Apache-2.0 | YAML |
| `go.starlark.net` | BSD | Rule scripts |
| `excelize` | BSD | Excel files |
| Open-source `.docx` reader (not UniOffice, which is commercial) | MIT / BSD | Word tables |
| Ed25519 (`crypto/ed25519`) | Built into Go | Signing packs and updates |

React and TypeScript come through npm with the UI.

The project uses one language, Go. Python is not used; the rule-store tools are Go commands (`rulestore-import`, `rulestore-check`).

CI: Go checks as now (they include the rule-store check through `go test`). No C compiler: the build stays pure Go (cgo off).

## Acceptance criteria

- [ ] AC-1 — `go version`, `git --version`, `node --version`, `pdftotext -v` and `wix --version` work in a fresh shell.
- [ ] AC-2 — `go run ./cmd/recon-dev rulestore-check` runs and passes.
- [ ] AC-3 — `go mod download` fetches every Go library; the Windows build succeeds with cgo off.
- [ ] AC-4 — `go test -count=1 ./...` passes locally.
- [ ] AC-5 — CI runs the Go checks (including the rule-store check) on every push.
- [ ] AC-6 — Every library's licence is recorded; no commercial library (such as UniOffice) is used.

## Verification

```
go version && git --version && node --version && pdftotext -v && wix --version
go mod download
go test -count=1 ./...
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
