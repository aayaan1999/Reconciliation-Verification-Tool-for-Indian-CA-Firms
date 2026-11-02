---
name: test-and-verify
description: Run this project's full verification for a change or a spec and report each acceptance criterion with evidence. Use after implementing a change, before marking a spec Done, or when asked to test, verify or check a spec.
---

# Test and verify

Proves a change is done against `docs/quality/definition-of-done.md`. Report results faithfully: a failure is reported with its output, a skipped step is named as skipped.

## Steps

1. **Read the spec** in `specs/` (Acceptance criteria and Verification sections) if one applies.
2. **Run the gates in this order**, narrowest first, and stop to diagnose at the first failure. If `go` is not found, put `C:\Program Files\Go\bin` on `PATH`.

   | Gate | Command |
   |---|---|
   | Formatting (must print nothing) | `gofmt -l .` |
   | Vet, host and Windows | `go vet ./...` and `GOOS=windows GOARCH=amd64 go vet ./...` |
   | Rule store | `go run ./cmd/recon-dev rulestore-check` |
   | Rule lint (pass and fail fixture per rule) | `go run ./cmd/recon-dev rule-lint` |
   | Profile tests | `go run ./cmd/recon-dev profile-test` |
   | Changed package | `go test -count=1 ./internal/<pkg>/ -v` |
   | Architecture guard | `go test -count=1 ./internal/archtest/ -v` |
   | Test Pack A, Pass 1 | `go test -count=1 ./cmd/recon-engine -run TestTestPackAPass1 -v` |
   | All Go tests | `go test -count=1 ./...` |
   | Web (inside `web/`, if UI changed) | `npm run lint`, `npm test`, `npm run build` |
   | Embedded UI | `go test -count=1 -tags ui ./web/` |
   | Windows build | `GOOS=windows GOARCH=amd64 go build -tags ui -o build/windows/ ./cmd/...` |

3. **Run the spec's own verification steps** exactly as written, including any end-to-end engine run, and keep the output.
4. **Check the invariants** on the diff: no `float` in money code, no `time.Now` or `math/rand` in engine packages, no new network import, no finding text with "should" or a stated correct value, no real client data.
5. **Check Test Pack A**: the score did not drop and false positives stay at 0. Note the score.
6. **Check the paperwork**: every new file is in `docs/Tool1-File-Guide.docx`; new rules are in `testdata/testpack_nimbus/score.yaml`; `specs/README.md` matches the spec's status.

## Report

| Criterion | Evidence (test name or command) | Result |
|---|---|---|

Then each failure with the command, the trimmed output and the root cause (or "root cause not found" with what was ruled out). End with what remains untested. Never set a spec to Verified; that needs a second person.
