# SPEC-002 — Developer tools setup

| | |
|---|---|
| **Status** | Done (one licence decision open on WiX; see Progress) |
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

- [x] AC-1 — `go version`, `git --version`, `node --version`, `pdftotext -v` and `wix --version` work in a fresh shell.
- [x] AC-2 — `go run ./cmd/recon-dev rulestore-check` runs and passes.
- [x] AC-3 — `go mod download` fetches every Go library; the Windows build succeeds with cgo off.
- [x] AC-4 — `go test -count=1 ./...` passes locally.
- [x] AC-5 — CI runs the Go checks (including the rule-store check) on every push.
- [x] AC-6 — Every library's licence is recorded; no commercial library (such as UniOffice) is used.

## Verification

```
go version && git --version && node --version && pdftotext -v && wix --version
go mod download
go test -count=1 ./...
```

## Progress (14 October 2026)

- AC-1: in a fresh shell, `go version` (1.27.0), `git --version` (2.55.0), `node --version` (24.19.0), `pdftotext -v` (26.09.0) and `wix --version` (7.0.0) all work. WiX was installed with `winget install Microsoft.DotNet.SDK.10` (SDK 10.0.402) and then `dotnet tool install --global wix`.
- **Open licence decision (WiX):** WiX's source is MS-RL, but since v6 its released binaries also carry the Open Source Maintenance Fee EULA, which can require a fee from organisations that earn revenue. Building an installer with v7 may also require accepting that EULA (`wix eula accept`). The firm decides whether to pay the fee, pin v5 (`dotnet tool install --global wix --version 5.*`, MS-RL only) or build from source. Nothing is built with WiX yet; the installer is Module 14.
- AC-2: `rulestore-check` passes (304 rules, 63 documents, 0 problems).
- AC-3: `go mod download` succeeds, and `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags ui ./cmd/...` builds all four programs.
- AC-4: `go test -count=1 ./...` passes.
- AC-5: CI runs gofmt, vet (host and Windows), `rulestore-check`, a check that the store still matches the workbook, `rule-lint`, `profile-test`, all Go tests with the architecture guard, the web lint, tests and build, and the Windows build on every push.
- AC-6: `docs/licences.md` records every linked module with its version and licence, plus the UI libraries, Poppler and the developer tools. `TestEveryLinkedModuleHasALicenceRecord` fails on any unrecorded module. No commercial library is used.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| 14 October 2026 | Unnati (with Claude Code) | not committed yet | All 6 criteria met; WiX licence decision open (see Progress). |
