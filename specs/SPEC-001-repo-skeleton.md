# SPEC-001 — Repository skeleton and architecture guards

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 1 |
| **Depends on** | — (start blocked on T-Q4: does a Phase 0 still gate Phase 1?) |
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

- [ ] AC-1 — `go build ./cmd/...` succeeds for `windows/amd64` and for the developer's host OS.
- [ ] AC-2 — `go test ./internal/archtest/` passes on the clean skeleton.
- [ ] AC-3 — Temporarily adding `import _ "net"` to any engine-side package makes the archtest fail with a message that names the package and the import path chain. (Done once; result recorded in the completion record; change reverted.)
- [ ] AC-4 — `build/engine-deps.txt` is produced by the test run and lists every package linked into `recon-engine`.
- [ ] AC-5 — An import from `internal/domain` to any non-standard-library package fails the archtest.
- [ ] AC-6 — `math/rand` or `time.Now` inside an engine-side package fails the archtest.
- [ ] AC-7 — CI fails on unformatted code, `go vet` warnings, or failing tests.
- [ ] AC-8 — `.gitignore` covers the paths above; `git status` after a local run shows no DB or vault files.
- [ ] AC-9 — `CLAUDE.md` lists the working build and test commands.

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
| | | | |
