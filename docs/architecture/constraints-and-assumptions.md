# Constraints and assumptions

This page lists the fixed constraints C1 to C8, the non-negotiable invariants that follow from them, and the working assumptions, each with its source and how it is enforced today. It summarises the product spec v0.2 §2, §4.1, §5.0, §5.4 and §8.1 ([recon-tool-spec.md](../../recon-tool-spec.md)), requirements v0.3 §2, §10 and §14 ([requirements-spec-tool1.md](../../requirements-spec-tool1.md)), the technical spec §A4.1 and §A8 ([tool1-technical-spec.md](../../tool1-technical-spec.md)), the invariants in [.claude/rules/project-invariants.md](../../.claude/rules/project-invariants.md) and the ADRs in [decisions/](decisions/).

## Constraints C1 to C8 (spec v0.2 §5.0)

| # | Constraint | Consequence | Source |
|---|---|---|---|
| C1 | Runs entirely on firm premises; no cloud | No managed services, cloud database or SaaS in the data path | Practitioner requirement (§0); ENV-03, ENV-04 |
| C2 | Installs on a Windows machine in an office with no IT staff | No runtime prerequisites, no Docker, no DBA | Observed reality of the target firm |
| C3 | The engine is provably unable to reach the network | Capability absence is demonstrable, not configured | §4.1; NF-04 |
| C4 | Deterministic and reproducible | No floating-point money, no map-order dependence, no LLM in the decision path | §2, §6.4; NF-01, NF-03, RU-51 |
| C5 | Rules readable by a CA and shippable without recompiling | Rules are data, not code | §4.2, §9; RU-01 to RU-04 |
| C6 | 50 concurrent client jobs on office hardware | Real concurrency, modest memory | §11.1; SC-04 to SC-06 |
| C7 | Multi-user over the office LAN, no per-workstation install | Server process plus browser | ENV-02 |
| C8 | Small team in a tier-2 Indian city | Mainstream languages | Practical |

Requirements v0.3 tightens C1: there is no internet at any time, and updates arrive by USB (ENV-03, ENV-05). The v0.2 outbound licence heartbeat and HTTPS update pull (§4.1, §9) do not apply to Tool 1.

## Non-negotiable invariants

From CLAUDE.md, each traced to the constraint and how it is held.

| # | Invariant | Source | How it is enforced today | Planned |
|---|---|---|---|---|
| 1 | **Assert only what can be computed.** A finding shows both values, the difference and both locators; it never says "should", never states a correct value, never judges applicability | §2; requirements §6.9; FD-02 | Wording templates in `internal/findings`; banned-phrase tests in `internal/rules`, `internal/findings` and `web/src/App.test.tsx` (tech spec §B4.6, §B14) | Same test for exports and the query letter (§B13) |
| 2 | **No LLM in the finding decision path.** A local model may only suggest a worklist classification (user confirms) or restate a finding | §5.4; NF-03 | No ML or LLM client in the engine; worklist classifications are typed by staff (SPEC-006 part 3) | Archtest allow-list against ML clients (tech spec §A8) |
| 3 | **Money is integer paise.** No floating point in money arithmetic | C4; NF-01 | `internal/money` (`Paise int64`, overflow reported); JSON numbers kept as exact text; `TestMoneyPackagesUseNoFloatingPoint` in `internal/archtest` | Property tests on money (§B14) |
| 4 | **Determinism.** Same input, same output; no map-order dependence, no clock or randomness in rules | C4; RU-51 | Registries iterated in sorted order; results sorted before write; `TestDeterministicPackagesHaveNoClockOrRandomness`; reports hold no run ID or time (tech spec §B6) | CI double-run with shuffled order and different `GOMAXPROCS` (§A8) |
| 5 | **The engine has no network capability.** Own code never imports `net`, `net/*`, `crypto/tls`, `golang.org/x/net/*`; third-party parse-only edges are listed and justified | §4.1, C3; NF-04 | `TestEngineHasNoNetwork` (import graph) and `TestEngineBinaryHasNoNetworkCode` (linked symbols), for windows/amd64 and linux/amd64; artefacts `build/engine-deps-*.txt`, `build/engine-symbols-*.txt` ([ADR-0001](decisions/ADR-0001-engine-network-proof.md)) | — |
| 6 | **Rules are data, not code.** Strict YAML, Starlark as a sandboxed escape hatch; each rule has pass and fail fixtures; a rule that fires wrongly is disabled, not tuned; packs versioned by tax year and signed with Ed25519 | §5.2.4, C5; RU-01 to RU-04, RU-43, RU-44, NF-06 | `internal/strictyaml`, `internal/rules`; `recon-dev rule-lint`; `recon-dev rulestore-check` in `go test` | Starlark (`internal/starhost`, empty) and signed packs (`internal/pack`, empty) |
| 7 | **Every source has a manual file-drop path.** API or ODBC access is an optimisation, never a dependency | §8.1 | All input today is uploaded files (JSON, CSV) | The optional Tally collector writes ordinary files into the inbox (`cmd/recon-collector`, SPEC-043) |
| 8 | **Zero-install deployment.** One binary on one machine, browsers over the LAN; no Docker, no external services; in-process job queue with state in the DB | C2, C7 | Pure-Go build with cgo disabled; the UI is embedded with `-tags ui` | Installer, Windows service and DB-backed job queue (tech spec §B15, `internal/jobs` empty) |

Further rules that follow from these:

- **Layout-preserving PDF extraction.** Naive extraction reorders statutory table columns and produces confident false findings (§5.2.5). Poppler `pdftotext -layout` is bundled as a separate program ([ADR-0003](decisions/ADR-0003-bundle-poppler.md)); the PDF adapter is planned.
- **No OCR.** Scanned PDFs and images are out of scope for Tool 1 (IN-05, A5; tech spec §B17 T7).
- **The decoupling rule.** A profile never names a rule and a rule never names a format; the fact vocabulary is the only shared contract (FV-04, tech spec §A2).
- **Missing is not a mismatch.** A missing figure gives "Cannot check", never a finding (RU-52, tech spec §B4.4).
- **No real client data in the repository.** All test data is synthetic (A1, tech spec §B14).

## Assumptions (requirements v0.3 §14)

| # | Assumption | Status |
|---|---|---|
| A1 | Development uses synthetic data only until anonymised real samples are provided | Holds: `testdata/` packs are fictional |
| A2 | The first format profile is built against a generic tax-suite JSON format | Holds: `content/profiles/generic/`, `content/profiles/taxsuite_pack/` |
| A3 | CA staff can prepare and export JSON, Excel and Word consistently using a written export guide | **Open**: the export guide is not written |
| A4 | Word files carry figures in tables; free text is not read | Design only (Word adapter planned) |
| A5 | PDFs are text-based; scans are out of scope | Design only (PDF adapter planned) |
| A6 | The firm's CA approves the starter rule library and law-table values before go-live | **Open**: the rule-book problem list is with the CA (SPEC-004 part 4) |

Assumptions carried from spec v0.2 and marked there as unverified: GSP/ASP licensing cost and eligibility (§8.1); the DPDP characterisation of an on-premise licensor (§10.2). Both are **Open** (requirements Q7; gaps doc Part B).

## Decided changes to the original constraints

| Decision | Replaces | Record |
|---|---|---|
| SQLite encrypted with the pure-Go driver's AES-256-XTS VFS | SQLCipher (v0.2 §5.2.2, requirements NF-04 and §11) | [ADR-0002](decisions/ADR-0002-database-encryption.md); trade-off: no page-level tamper detection |
| Parse-only network edges from third-party libraries allowed when reviewed, with a linked-symbol proof | "No network package anywhere in the graph" | [ADR-0001](decisions/ADR-0001-engine-network-proof.md) |
| Poppler (GPL) bundled and run as a separate program | Open licence question | [ADR-0003](decisions/ADR-0003-bundle-poppler.md) |

## Open

- Licence check: keep or remove (requirements Q4).
- HTTPS on the LAN: import a firm CA on staff PCs, or accept a browser warning (tech spec §B17 T-Q2). ENV-02 says nothing is installed on staff PCs.
- WiX installer licence (tech spec §B17 T1b).
- Number of concurrent staff users (Q6).
