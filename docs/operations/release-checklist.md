# Release checklist

Purpose: the gates every release to the firm passes, including the 30 October 2026 demo build and the 6 November 2026 second release. It summarises the CI workflow (`.github/workflows/ci.yml`), the architecture guard in [CLAUDE.md](../../CLAUDE.md), the acceptance lists in the [implementation plan](../Reconciliation%20%26%20Verification%20Tool%20-%20Implementation%20plan.md) §10, and the technical spec ([tool1-technical-spec.md](../../tool1-technical-spec.md) §B10, §B14, §B15). Items marked **Planned** cannot be ticked until the feature exists; record them as not applicable with the reason.

Copy this list into the release notes and tick each item with who checked it and when.

## Release details

| | |
|---|---|
| Release | e.g. demo build / second release |
| Commit | |
| Built by | |
| Checked by (not the builder) | |
| Date | |

## 1. CI and local checks

- [ ] CI is green on the release commit: format check, vet (host and Windows), rule store check, rule store matches the workbook, rule lint, profile tests, all tests with the architecture guard, web lint, tests and build, embedded UI test, Windows build.
- [ ] `gofmt -l .` prints nothing.
- [ ] `go vet ./...` and `GOOS=windows GOARCH=amd64 go vet ./...` pass.
- [ ] `go test -count=1 ./...` passes locally.
- [ ] `cd web && npm ci && npm run lint && npm test && npm run build` passes.
- [ ] The Poppler test has been run on Windows (from PowerShell if Bash cannot start `pdftotext`).

## 2. Engine network proof

- [ ] `go test -count=1 ./internal/archtest/ -v` passes.
- [ ] `build/engine-deps-windows-amd64.txt` and `build/engine-deps-linux-amd64.txt` reviewed: no new package compared with the last release, or each new one explained.
- [ ] `build/engine-symbols-windows-amd64.txt` and `-linux-amd64.txt` reviewed: only parse-only network functions and the initialisers of `net` and `dnsmessage` appear ([ADR 0001](../architecture/decisions/ADR-0001-engine-network-proof.md)).
- [ ] Any new entry in `allowedNetworkImports` has a reason and was reviewed.
- [ ] The CI artefact `engine-deps` for the release commit is kept with the release notes.

## 3. Results

- [ ] Test Pack A, Pass 1: `go test -count=1 ./cmd/recon-engine -run TestTestPackAPass1 -v`. Record the score (currently 22 of 30, critical 4 of 4, 0 false positives). A drop in score or any false positive blocks the release.
- [ ] The corrected copy of Test Pack A gives no finding; no negative control is flagged.
- [ ] The Worklist on Test Pack A gives exactly F27 to F30.
- [ ] Clean set gives zero findings; same files run twice give identical output (plan §10).
- [ ] Screens show the same figures as the engine (`TestWebSamplesMatchEngine` passes without `-update`).
- [ ] The plan §7 demo steps run in the browser on the release build (for the demo release).

## 4. Rules

- [ ] Every rule switched on has CA approval recorded (reviewer, date, decision). No rule is enabled by an agent.
- [ ] Every runnable rule has pass and fail fixtures (`go run ./cmd/recon-dev rule-lint`).
- [ ] Rules disabled after a field incident stay disabled unless the CA re-approved a new version ([incident response](incident-response.md)).
- [ ] **Planned:** rule pack versioned by assessment year and signed with Ed25519; manifest SHA-256 values checked; changelog written; signing key kept offline (§B10). Until then, rules ship as the content folders of the build.

## 5. Build

- [ ] Windows build with the UI embedded: `cd web && npm ci && npm run build && cd ..` then `GOOS=windows GOARCH=amd64 go build -tags ui -o build/windows/ ./cmd/...`.
- [ ] The build starts on a Windows test machine; `recon-server` serves the screens; `recon-engine run` gives the expected exit code on `testdata/run/clean` (0) and `testdata/run/differs` (1).
- [ ] **Planned:** installer tested on a clean Windows PC (Module 14); WiX licence decision recorded (SPEC-002, tech spec T1b).
- [ ] **Planned:** backup restored with the same results (Module 14).

## 6. Licences

- [ ] [docs/licences.md](../licences.md) lists every linked module with version and licence (`TestEveryLinkedModuleHasALicenceRecord` passes).
- [ ] Poppler GPL obligations checked if the tool goes to anyone other than the firm ([ADR 0003](../architecture/decisions/ADR-0003-bundle-poppler.md)).
- [ ] No commercial library added.

## 7. Documents

- [ ] Every file created since the last release is in `docs/Tool1-File-Guide.docx`; renames, moves and deletions are reflected.
- [ ] `specs/README.md` statuses match the spec files.
- [ ] `tool1-technical-spec.md` reflects any design change.
- [ ] CLAUDE.md "Current state" and "Commands" are current.

## 8. Data and security

- [ ] No real client data in the repository, the build or the release notes.
- [ ] No private key (`*.key`, `*.pem` other than `*.pub.pem`) in the repository.
- [ ] Security review done for changes to ingest, store, server or updater code (`security-review` skill).

## 9. Release notes

- [ ] Written: what changed, rules added or disabled, Test Pack A score, known limits (for example, no logins or encryption yet if still true), open items.
- [ ] Go/no-go recorded (30 October) or sign-off recorded (6 November) by the firm.
