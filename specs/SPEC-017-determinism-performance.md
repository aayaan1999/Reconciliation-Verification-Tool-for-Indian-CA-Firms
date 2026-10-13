# SPEC-017 — Determinism, incremental re-run and performance harness

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 2 |
| **Depends on** | SPEC-011 |
| **Requirements** | RU-51, NF-03, SC-04, SC-05, SC-06, SC-07 |
| **Technical spec** | §A7 (incremental), §A8, §A9, §B14 |
| **Code / content paths** | `internal/engine` (incremental planner), `cmd/recon-dev` (`synth`), CI jobs |

## Goal

CI proves three things: results never depend on order, timing or parallelism; an incremental run equals a full run; and the performance targets hold.

## Scope

- Incremental run: `recon-engine run ... --base-run <id>`. Changed document versions → changed fact names → affected rules (via the dependency index); evaluations of unaffected rules are copied forward by reference.
- Determinism test: each pack is run twice, with shuffled file order and `GOMAXPROCS` of 1 and 8, and the canonical JSON is compared byte for byte.
- Incremental ≡ full test: for each pack, change one file, then run incremental and full and compare.
- `recon-dev synth`: a seeded, deterministic generator of synthetic clients, up to 200k vouchers, 12 GST months, 4 TDS quarters and 12 payroll months. It never uses real names or PANs.
- Benchmarks, plus a CI job that fails on a regression of more than 20% against the stored baseline.

## Out of scope

- 50-client concurrency through the server (load test in SPEC-023).

## Acceptance criteria

- [ ] AC-1 — The determinism test passes on every pack in `testdata/packs`.
- [ ] AC-2 — Incremental ≡ full passes for every pack, for each single-file change.
- [ ] AC-3 — One synthetic client at full volume: a full run takes less than 3 min on the 8-core/16 GB reference machine (SC-06).
- [ ] AC-4 — A re-run after changing one file takes less than 20 s (SC-06).
- [ ] AC-5 — 600 active rules on a full-volume client stay within the AC-3 budget (SC-07).
- [ ] AC-6 — `synth` with the same seed produces byte-identical files.

## Verification

```
go test ./... -run 'TestDeterminism|TestIncrementalEqualsFull' -v
go run ./cmd/recon-dev synth --seed 1 --vouchers 200000 --out testdata/large
go test ./internal/engine/ -bench BenchmarkFullClient -run ^$
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
