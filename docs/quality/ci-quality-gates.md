# CI and quality gates

Purpose: exactly what runs on every push, what to run locally before pushing, and what blocks a merge. It summarises [.github/workflows/ci.yml](../../.github/workflows/ci.yml), the Commands section of [CLAUDE.md](../../CLAUDE.md) and the technical spec §B14 and §B15.

## What CI runs

The workflow `CI` runs on every `push` and `pull_request`, with read-only repository permissions. It has two jobs that run in parallel on `ubuntu-latest`.

### Job `check` (Go, then the embedded UI)

Go version comes from `go.mod`. Steps, in order; any failing step fails the job:

| # | Step | Command | Fails when |
|---|---|---|---|
| 1 | Formatting | `gofmt -l .` must print nothing | Any file is not gofmt-formatted (the list is printed) |
| 2 | Vet | `go vet ./...` | Vet reports a problem on the host platform |
| 3 | Vet for Windows | `GOOS=windows GOARCH=amd64 go vet ./...` | Vet reports a problem in Windows-only code |
| 4 | Rule store check | `go run ./cmd/recon-dev rulestore-check` | A rule-store file fails strict decoding or a cross-reference |
| 5 | Rule store matches the workbook | `go run ./cmd/recon-dev rulestore-import && git diff --exit-code -- docs/Recon_Tool_YAML_Rule_Store` | The YAML store was hand-edited or not regenerated after the workbook changed |
| 6 | Rule lint | `go run ./cmd/recon-dev rule-lint` | A rule does not load or lacks a pass or a fail fixture |
| 7 | Profile tests | `go run ./cmd/recon-dev profile-test` | A profile's sample does not give exactly its expected facts |
| 8 | Tests | `go test -count=1 ./...` | Any Go test fails, including the architecture guard, rule fixtures, Test Pack A scoring and the web samples check. Never from cache |
| 9 | Set up Node 24 | `actions/setup-node`, npm cache from `web/package-lock.json` | |
| 10 | Build the browser UI | `npm ci && npm run build` in `web/` | Locked install, type check or Vite build fails |
| 11 | Test the embedded UI | `go test -count=1 -tags ui ./web/` | The built UI cannot be embedded or served |
| 12 | Windows build | `GOOS=windows GOARCH=amd64 go build -tags ui -o build/windows/ ./cmd/...` | Any program fails to build for Windows with the UI embedded |
| 13 | Keep the engine dependency proof | uploads `build/engine-deps-*.txt` and `build/engine-symbols-*.txt` as artefact `engine-deps` | The files are missing (runs whenever step 8 ran, even if it failed) |

Steps 4 to 7 also run inside `go test`; they run first so that a broken rule, profile or rule-store file is named in the log before the full test run.

### Job `web` (inside `web/`)

| Step | Command |
|---|---|
| Install locked dependencies | `npm ci` |
| Lint | `npm run lint` (oxlint) |
| Tests | `npm test` (Vitest, `vitest run`) |
| Type-check and build | `npm run build` (`tsc -b && vite build`) |

## Local gates before pushing

Run these from the repository root. They mirror CI; the first four are the minimum for any Go change.

| Gate | Command | Expect |
|---|---|---|
| Format | `gofmt -l .` | No output |
| Vet, host | `go vet ./...` | No output |
| Vet, Windows | `GOOS=windows GOARCH=amd64 go vet ./...` | No output |
| All tests | `go test -count=1 ./...` | `ok` for every package |
| Architecture guard (on its own, verbose) | `go test -count=1 ./internal/archtest/ -v` | Pass; `build/engine-*.txt` written |
| Rule store | `go run ./cmd/recon-dev rulestore-check` | No problems |
| Rule lint | `go run ./cmd/recon-dev rule-lint` | No problems |
| Profiles | `go run ./cmd/recon-dev profile-test` | Every sample passes |
| Test Pack A | `go test -count=1 ./cmd/recon-engine -run TestTestPackAPass1 -v` | Pass, 0 false positives |
| Web (in `web/`) | `npm ci`, `npm run lint`, `npm test`, `npm run build` | All pass |

In PowerShell, set the target for the Windows vet with `$env:GOOS='windows'; $env:GOARCH='amd64'; go vet ./...` and clear the variables afterwards. Run the Poppler test from PowerShell on Windows, because some sandboxed Bash shells cannot start it.

If a change alters engine output the screens show, regenerate the samples with `go test ./cmd/recon-engine -run TestWebSamplesMatchEngine -update` and review the diff. If the workbook changed, run `go run ./cmd/recon-dev rulestore-import` and commit the regenerated store.

## What the guard enforces

The architecture guard in `internal/archtest` fails the test run when:

- `recon-engine` (or another network-free command) links connection, socket or resolver code, or our own code imports `net`, `net/*`, `crypto/tls` or `golang.org/x/net/*`;
- a third-party library adds an import edge into a network package that is not listed with a reason in `allowedNetworkImports` ([ADR-0001](../architecture/decisions/ADR-0001-engine-network-proof.md));
- a package is not classified in exactly one of `engineSidePatterns`, `serverSidePatterns` or `otherPatterns`;
- a deterministic package (listed in `deterministicDirs`) uses a clock or randomness, or a money package uses floating point;
- a linked module has no licence record in [docs/licences.md](../licences.md).

## What blocks a merge

- Any failing step in either CI job. Both jobs must be green.
- A change that breaks a non-negotiable invariant (`.claude/rules/project-invariants.md`), even if CI passes (for example finding text that states a correct value, or per-rule logic written in Go instead of YAML). Code review rejects it.
- A change to the YAML rule store that does not come from the workbook (step 5).
- A new rule without pass and fail fixtures, or a rule detecting a Test Pack A finding without its entry in `testdata/testpack_nimbus/score.yaml`.
- A new package without a classification in the guard, or a new network import edge without a reviewed entry.

Branch protection settings on the hosting side are not recorded in this repository: whether CI is a required check is **Open**. A spec is not Done until its own verification also passes; see [definition-of-done.md](definition-of-done.md).
