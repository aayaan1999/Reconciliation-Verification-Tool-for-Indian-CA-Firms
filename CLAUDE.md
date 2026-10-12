# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Current state

There is no code yet. This repo has two documents and nothing else:

- `recon-tool-spec.md`: the product and engineering spec (v0.2, draft). It is the source of truth for every design decision. Section references below (§n) point into it.
- `Recon-Tool-Understanding-and-Gaps.docx`: a plain-language summary of the spec plus a list of open gaps, for example undefined finding states (Open/Fixed/Dismissed/Reopened), what happens when a fixed defect reappears in a later draft, and clients whose Tally books are kept off-site.

There are no build, lint or test commands yet. When code is scaffolded, add them here. The spec gates building: Phase 1 (§14) is a CLI-only spike that parses one reference filing pack with 80–120 rules, and it should not start until Phase 0 validation clears.

All figures, PANs and entity names in the spec are illustrative. Never treat them as real client data, and never put real client data in this repo.

## What the product is

An offline consistency layer for Indian CA firms. It reads what the accounting package (Tally), the tax suite, the GST portal, TRACES and EPFO/ESIC produce, and reports every place where they disagree, to the rupee. It also flags expense and income heads that are new or missing compared with the prior year. It does not prepare filings, keep books or give tax advice (§3.2).

## Non-negotiable invariants

These come from the spec and should constrain every piece of code:

1. **Assert only what can be computed (§2).** A finding shows both values, the difference and both source locators (file + page/locator). Output never says "should", never states a correct value, and never makes an applicability or characterisation judgment. Judgment findings were wrong 6 times out of 9 in the reference review.
2. **No LLM in the finding decision path (§5.4).** An optional local model may only suggest a classification for a new ledger head (the user confirms it) or restate a finding in plain language. It may never create, suppress or alter a finding.
3. **Money is integer paise everywhere.** No floating point anywhere in money arithmetic (C4).
4. **Determinism (C4).** The same input must give the same output: no dependence on map-iteration order and no clocks or randomness in rules.
5. **The engine binary has no network capability (§4.1, C3).** The reconciliation engine must never import `net`, `net/http` or anything that pulls them in transitively. The updater is a separate process. Generating the import-graph proof is meant to be a build artefact.
6. **Rules are data, not code (§5.2.4, C5).** Rules are declarative YAML, with Starlark as a sandboxed escape hatch. A CA who is not an engineer must be able to read them. Each rule ships with golden-file fixtures: at least one passing and one failing (§6.4, §9). A rule that fires wrongly in the field gets disabled, not tuned. Rule packs are versioned by assessment year and signed with Ed25519.
7. **Every data source needs a manual file-drop path (§8.1).** API or ODBC access is an optimisation, never a dependency, so the tool has to work fully air-gapped.
8. **Zero-install deployment (C2, C7).** One binary on one office machine, with staff using it from a browser over the LAN. No Docker, no external services (Redis and similar), no runtime prerequisites. The job queue runs in-process with its state stored in the DB.

## Planned architecture (§5, §6)

- **Stack:** Go single static binary. SQLite + SQLCipher (explicit SQL, no ORM). A React + TypeScript UI embedded with `go:embed`. `excelize` for Excel. Vendored Poppler `pdftotext -layout` for PDFs. Where the tax suite emits JSON, prefer parsing that over its PDFs. A Python `pdfplumber` sidecar is a last resort only.
- **Pipeline:** ingestion adapters → normalised encrypted fact store → deterministic rule engine → findings with lifecycle and audit trail → LAN web UI.
- **Tie-out model (M2):** a *Fact* is `(artefact, locator, label, value, period)`. An *Edge* asserts that facts must be equal or must satisfy a relation. A *Finding* is a broken edge. Rule categories R1–R10 (§6.2) are the reference defect catalogue, and the Phase 1 gate is reproducing all of them with zero false positives.
- **New-head detection (M3, §7):** a year-on-year ledger diff (new, vanished, broken recurrence, step change, unmapped head). Its output is a classification *worklist*, which is separate from findings.
- **PDF extraction must preserve column layout.** Naive text extraction reorders statutory table columns and produces confident false findings. Validate every extractor against known-good fixtures.
- **UI:** exactly three screens: Season board, Client file and Worklist (§11.2). Group findings by the document that needs fixing, not by rule category. Always show the arithmetic. Keyboard-first.
