# SPEC-011 — Engine CLI and Phase 1 end to end

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 1 (Phase 1 gate) |
| **Depends on** | SPEC-006, SPEC-008, SPEC-009, SPEC-010 |
| **Requirements** | IN-01, IN-10, IN-14, RU-50, RU-51, RU-52; requirements §13 Phase 1 "done when" |
| **Technical spec** | §A3 (engine CLI mode), §A7, §B15 |
| **Code / content paths** | `cmd/recon-engine`, `testdata/packs/ref-01`, `testdata/packs/clean-01`, `content/rules/2025-26/tieout/RECON-DEP-002/` |

## Goal

From the command line, the engine reads a folder of files, extracts facts, runs RECON-DEP-002, and prints the §6.9 finding. A clean pack gives zero findings.

## Scope

- `recon-engine run --input <dir> --rules <dir> --tax-year "2025-26" --db <path> --out <file> [--format text|json]`.
- Pipeline: detect → extract → resolve current view → snapshot → evaluate → render (§A7 steps 1–7, 10).
- Prints validation reports for each file (IN-14).
- Exit codes: `0` no findings; `1` findings present; `2` input or rule error; `3` internal error.
- Synthetic packs: `ref-01` (computation and tax audit report with the depreciation mismatch 1,12,450 vs 1,15,200) and `clean-01` (same figures agree).
- Rule `RECON-DEP-002` with pass and fail fixtures.

## Out of scope

- Daemon/IPC mode (SPEC-023). Findings lifecycle (SPEC-025).

## Acceptance criteria

- [ ] AC-1 — `ref-01` produces exactly one finding, RECON-DEP-002, and the text output matches the golden file from SPEC-009.
- [ ] AC-2 — `clean-01` produces zero findings and exit code 0.
- [ ] AC-3 — Removing the tax audit report from `ref-01` produces a "Cannot check" item, not a finding.
- [ ] AC-4 — An unrecognised file in the input folder is reported with `PROFILE_NO_MATCH` and exit code 2; it is not guessed.
- [ ] AC-5 — Two runs on `ref-01` give byte-identical JSON output.
- [ ] AC-6 — `build/engine-deps.txt` for this build contains no `net` package.
- [ ] AC-7 — The Windows `.exe` runs on the Windows test PC with the same output (blocked until Q5 is answered).

## Verification

```
go run ./cmd/recon-engine run --input testdata/packs/ref-01 --rules content/rules/2025-26 --tax-year "2025-26" --db scratch.db --out report.txt
go run ./cmd/recon-engine run --input testdata/packs/clean-01 --rules content/rules/2025-26 --tax-year "2025-26" --db scratch2.db --out clean.txt
go test ./cmd/recon-engine/ -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
