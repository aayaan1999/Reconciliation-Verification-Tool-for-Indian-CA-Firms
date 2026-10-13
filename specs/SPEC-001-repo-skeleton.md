# SPEC-001 — Repository skeleton and architecture guards

| | |
|---|---|
| **Status** | In progress (waiting for the first GitHub CI run, AC-7) |
| **Phase** | 1 |
| **Depends on** | — (T-Q4 answered 13 Oct 2026: Phase 0 cleared, building may start) |
| **Requirements** | NF-03, NF-04, SC-08 |
| **Technical spec** | §A4, §A4.1, §A8, §B15 |
| **Code / content paths** | `go.mod`, `cmd/*`, `internal/*` (empty packages), `internal/archtest`, `.gitignore`, CI config |

## Goal

The Go module and folder layout of §A4 exist, every binary builds, and automated tests prove that the engine cannot reach the network and that package import rules hold.

## Scope

- `go.mod` (module path to be agreed; Go version pinned).
- Stub `main` packages for `recon-engine`, `recon-server`, `recon-updater`, `recon-dev` (and an empty `recon-collector` folder with a README only).
- Empty `internal/*` packages with a `doc.go` stating each package's responsibility, copied from §A4.
- `content/`, `docs/`, `testdata/`, `third_party/` folders with README files.
- `internal/archtest`:
  - engine dependency check: `go list -deps ./cmd/recon-engine` contains no `net`, `net/*` or `crypto/tls`;
  - writes the full dependency list to `build/engine-deps.txt` (the "no network" proof artefact);
  - import-direction rules of §A4.1 (e.g. `domain`, `money`, `period` import only the standard library; engine-side packages never import `api`, `authz`, `export`);
  - no `math/rand` and no `time.Now` in engine-side packages.
- `.gitignore` excluding `*.db`, `data/`, `vault/`, `build/`, `web/dist/`, `node_modules/`, and a `samples-private/` folder that must never be committed.
- CI pipeline: `gofmt -l` (must be empty), `go vet ./...`, `go test ./...`.
- `CLAUDE.md`: replace "no build commands yet" with the commands in §B15 that now work.

## Out of scope

- Any real logic (SPEC-002 onwards).
- Windows installer (SPEC-040).

## Design notes

- The archtest runs `go list` through `os/exec`; it lives in a test file, so it never ships inside a binary.
- Keep the forbidden-package list in one place so later specs (SPEC-014 Starlark, SPEC-018 excelize, SPEC-034 SQLCipher) can rely on it.

## Acceptance criteria

- [x] AC-1 — `go build ./cmd/...` succeeds for `windows/amd64` and for the developer's host OS.
- [x] AC-2 — `go test ./internal/archtest/` passes on the clean skeleton.
- [x] AC-3 — Temporarily adding `import _ "net"` to any engine-side package makes the archtest fail with a message that names the package and the import path chain. (Done once; result recorded in the completion record; change reverted.)
- [x] AC-4 — `build/engine-deps-windows-amd64.txt` (and `-linux-amd64`) is produced by the test run and lists every package linked into `recon-engine` for that target.
- [x] AC-5 — An import from `internal/domain` to any non-standard-library package fails the archtest.
- [x] AC-6 — `math/rand` or `time.Now` inside an engine-side package fails the archtest.
- [ ] AC-7 — CI fails on unformatted code, `go vet` warnings, or failing tests.
- [x] AC-8 — `.gitignore` covers the paths above; `git status` after a local run shows no DB or vault files.
- [x] AC-9 — `CLAUDE.md` lists the working build and test commands.

## Verification

```
go build ./cmd/...
GOOS=windows GOARCH=amd64 go build ./cmd/...
go test ./internal/archtest/ -v
gofmt -l . ; go vet ./...
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| 13 Oct 2026 | Claude Code, approved step by step by the project owner | not yet committed | AC-1..6, 8, 9 proven locally. AC-3, AC-5, AC-6 proven by deliberate break-and-restore (network import in internal/money; non-standard import in internal/domain; time.Now in internal/engine). The network guard also covers recon-updater and every engine-side package directly. AC-7 pending first CI run on GitHub. |
| 13 Oct 2026 | Claude Code, after code-reviewer and test-engineer reports, approved by the project owner | not yet committed | Fixes: .gitignore patterns anchored to the root (internal/vault was being ignored) and exceptions for vendored .exe and public keys; guard evaluates windows/amd64 and linux/amd64 with cgo off; guard reads all module sources so test caching cannot hide a violation, CI uses -count=1; clock check also catches dot imports, Since, Until, Sleep, timers, hash/maphash and cgo; new tests require every package to be classified and the module path to match go.mod. Each fix proven by break-and-restore (Linux-only net import in internal/store, unclassified internal/zzdemo, dot-import of time in internal/engine), all caught without -count=1. Helper unit tests (helpers_test.go, about 70 cases) added by the test engineer; previously skipped known-gap cases now pass. |
