---
paths:
  - "cmd/**/*.go"
  - "internal/**/*.go"
  - "go.mod"
---

# Backend (Go)

Architecture: `docs/architecture/architecture-overview.md`. Design detail: `tool1-technical-spec.md` Part A (structure) and Part B (implementation).

## Layout

- Programs live in `cmd/` (`recon-engine`, `recon-server`, `recon-dev`, and later `recon-updater`). Libraries live in `internal/`, one responsibility per package, stated in its `doc.go`. A new package gets a `doc.go`.
- Every package must be classified in exactly one of `engineSidePatterns`, `serverSidePatterns` or `otherPatterns` in `internal/archtest/archtest_test.go`; the guard fails on an unclassified package. Deterministic packages also go in `deterministicDirs`.
- Engine-side packages never import `net`, `net/*`, `crypto/tls` or `golang.org/x/net/*`. A new third-party edge into a network package needs an entry in `allowedNetworkImports` with a reason (ADR 0001).

## Code rules

- Money is `int64` paise. Parse amounts from text with `internal/money` (`parse.go`); never through `float64` or `strconv.ParseFloat`.
- No `time.Now`, `math/rand` or map iteration in anything that produces output. Sort keys before iterating. Timestamps, where needed, come in from the caller.
- Rules are data. Do not add per-rule or per-client logic in Go; add or extend a check type (`internal/engine/checks`) and express the rule in YAML under `content/rules/`.
- A figure that cannot be found gives "cannot check", never a zero and never a mismatch.
- Figures in a list are read from the entry that meets conditions, never by position.
- Errors are wrapped with context (`fmt.Errorf("...: %w", err)`) and never swallowed. CLI exit codes: 0 no findings, 1 findings, 2 input or rule error, 3 internal.
- Explicit SQL, no ORM (ADR 0002). Schema changes are new numbered files in `internal/store/migrations`; never edit an applied migration.
- Follow the surrounding style: comment density, naming, table-driven tests.

## Before finishing

`gofmt -l .` prints nothing, `go vet ./...` and `GOOS=windows GOARCH=amd64 go vet ./...` pass, and `go test -count=1 ./...` passes (it includes the architecture guard and Test Pack A scoring). If the change alters engine output the screens show, regenerate the samples with `go test ./cmd/recon-engine -run TestWebSamplesMatchEngine -update` and review the diff.
