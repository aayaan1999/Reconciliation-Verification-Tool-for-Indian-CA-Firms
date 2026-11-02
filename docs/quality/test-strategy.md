# Test strategy

Purpose: the layers of automatic testing in Tool 1, what each proves and how to run it. It summarises the technical spec §B14 (testing strategy) and §B2.3 (profile testing), the commands in [CLAUDE.md](../../CLAUDE.md), and the tests as they exist in the code after SPEC-006.

Go is at `C:\Program Files\Go\bin`; put it on `PATH` if `go` is not found. Node.js 24 is used only in `web/`. No test, fixture or sample may contain real client data: every name, PAN, GSTIN and figure is invented (§B14).

## What the tests must protect

The tests exist to hold the project invariants (`.claude/rules/project-invariants.md`): assert only what can be computed, no LLM in the finding path, money as integer paise, determinism, no network in the engine, rules as data with fixtures, and a manual file path for every source. A false positive is worse than a miss: on Test Pack A, zero false positives matters more than the hit rate.

## Layers

| Layer | What it proves | Where | Status |
|---|---|---|---|
| Unit tests | Each package does its job: money parsing and Indian grouping, periods, canonical JSON, stable IDs, adapters (JSON, CSV), profile detection and extraction, strict YAML, check types | `*_test.go` beside the code, mostly table-driven | Built |
| Rule golden fixtures | Every runnable rule has at least one **pass** and one **fail** fixture at fact level, with the expected evaluations (RU-43) | `content/rules/<tax year>/<area>/<RULE-ID>/fixtures/pass/*.yaml` and `fail/*.yaml`, run by `TestRuleFixtures` in `internal/engine` | Built (SPEC-003 onward) |
| Rule lint | Every rule loads against the vocabulary and has both kinds of fixture; problems printed with file and line | `go run ./cmd/recon-dev rule-lint` | Built |
| Rule store check | The YAML rule store decodes strictly and its cross-references hold; the store matches the workbook (no hand edits) | `go run ./cmd/recon-dev rulestore-check`; CI also re-imports and runs `git diff --exit-code` | Built (SPEC-001) |
| Profile tests | Every shipped profile's samples are detected as that profile against every profile and give exactly the hand-verified facts and locators | `go run ./cmd/recon-dev profile-test`, `TestShippedProfiles` in `internal/profile` | Built (SPEC-004) |
| Engine behaviour | Missing input is Cannot check, two values for one figure are refused, evaluation order is deterministic | `internal/engine` (`TestMissingInputIsCannotCheck`, `TestConflictingFactsAreRefused`, `TestEvaluateIsDeterministic`) | Built |
| End-to-end runs | `recon-engine run` gives the expected exit code and report on each input folder (clean, differs, missing, unknown, damaged, GST pack and more) | `cmd/recon-engine` tests over `testdata/run/` and `testdata/gst_pack_aug2026/` | Built |
| Test Pack A scoring | Pass 1 (JSON only) of the fictional Nimbus Logic Systems pack: each answer-key finding is detected by the rules listed, with the stated difference; no rule fires on a negative control | `TestTestPackAPass1` in `cmd/recon-engine`, answer key `testdata/testpack_nimbus/10_EXPECTED_FINDINGS.csv`, map `score.yaml` | Built (SPEC-005, SPEC-006) |
| Negative controls as checks | Each of the 16 controls is checked by a rule that passes on the pack, and each planted defect fires only its own rules | `testdata/testpack_nimbus/11_NEGATIVE_CONTROLS.csv` | Planned (SPEC-007, not started) |
| Worklist | The year-on-year ledger comparison gives exactly F27 to F30 on Test Pack A | `internal/modules/newheads`, `cmd/recon-engine` | Built (SPEC-006 part 3) |
| Architecture guard | Engine has no network code (import graph and linked symbols, windows/amd64 and linux/amd64), import rules hold, every package is classified, no clock or randomness in deterministic packages, no floating point in money packages, every linked module has a licence record | `internal/archtest` | Built (SPEC-002, SPEC-003) |
| Web samples golden check | The screens' sample data equals real engine `--json` output byte for byte | `TestWebSamplesMatchEngine` in `cmd/recon-engine`, files in `web/src/sample/` | Built (SPEC-004) |
| Embedded UI | The built UI is served by the Go package `web` | `go test -tags ui ./web/` (after `npm run build`) | Built |
| Web unit tests | Screens render from the engine's shape, filters and keyboard navigation work, banned wording never appears and the screens' banned list equals `rules.BannedPhrases` | Vitest, `web/src/*.test.tsx` | Built (SPEC-004 onward) |
| Poppler | The vendored `pdftotext` runs and keeps column layout | `internal/ingest/adapters/pdftext/poppler_test.go`, Windows only | Built (SPEC-002) |

### Planned layers (§B14)

Determinism across shuffled upload order and different `GOMAXPROCS`, incremental re-run equal to a full run, money property tests, lifecycle transitions (§B7.3), "every API handler has a permission", and performance benchmarks from Phase 2 (1 client with 200k vouchers under 3 minutes). Synthetic end-to-end packs in `testdata/packs/` reproducing R1 to R10 are described in §B14; today that role is taken by `testdata/run/`, the GST pack and Test Pack A.

## Rules for writing tests

- **Each rule ships with fixtures**: at least one passing and one failing. A boundary case (just within and just over tolerance) is encouraged; see `content/rules/2025-26/tieout/RECON-DEP-002/fixtures/`.
- **Each new rule goes into `testdata/testpack_nimbus/score.yaml`** when it detects a Test Pack A finding; the test then requires it. A rule that fires on a negative control fails the test.
- A rule that fires wrongly in the field is disabled, not tuned.
- A changed format gets a new profile version folder with its own samples and expected facts; a published profile is never edited.
- When engine output the screens show changes on purpose, regenerate the samples and review the diff before committing.
- The architecture guard reads all module sources, so caching cannot hide a violation, but run it with `-count=1`.

## How to run each layer

| Task | Command |
|---|---|
| All Go tests (includes fixtures, profile tests, Test Pack A, web samples check and the architecture guard) | `go test -count=1 ./...` |
| One package or test | `go test ./internal/archtest/ -run TestEngineHasNoNetwork -v` |
| Rule fixtures only | `go test ./internal/engine/ -run TestRuleFixtures -v` |
| Architecture guard | `go test -count=1 ./internal/archtest/ -v` |
| Rule store check | `go run ./cmd/recon-dev rulestore-check` |
| Rule lint | `go run ./cmd/recon-dev rule-lint` |
| Profile tests | `go run ./cmd/recon-dev profile-test` |
| Test Pack A, Pass 1 | `go test -count=1 ./cmd/recon-engine -run TestTestPackAPass1 -v` |
| Web samples check | `go test ./cmd/recon-engine -run TestWebSamplesMatchEngine` |
| Regenerate web samples (intended output change only) | `go test ./cmd/recon-engine -run TestWebSamplesMatchEngine -update` |
| Web tests and lint (inside `web/`) | `npm ci`, `npm test`, `npm run lint`, `npm run build` |
| Embedded UI | `cd web && npm run build && cd ..`, then `go test -count=1 -tags ui ./web/` |
| Poppler test | run from PowerShell on Windows: `go test ./internal/ingest/adapters/pdftext/` |

## Artefacts

The architecture guard writes `build/engine-deps-<goos>-<goarch>.txt` (every package linked into `recon-engine`) and `build/engine-symbols-<goos>-<goarch>.txt` (network-package functions actually linked; only parse-only ones and the initialisers of `net` and `dnsmessage` may appear). Together they are the proof that the engine has no network code ([ADR-0001](../architecture/decisions/ADR-0001-engine-network-proof.md)). CI keeps them as the `engine-deps` artefact.

## Current score

Test Pack A Pass 1 scores 22 of 30, the most the JSON can reach (critical 4 of 4, 0 false positives); the remaining findings need data that is not in the JSON (draft versions, signing date, ledgers) and are marked `in_json: false` in `score.yaml`. F27 to F30 come from the ledger CSVs through the Worklist, not from findings. See [ci-quality-gates.md](ci-quality-gates.md) for what runs in CI and [definition-of-done.md](definition-of-done.md) for when a spec is finished.
