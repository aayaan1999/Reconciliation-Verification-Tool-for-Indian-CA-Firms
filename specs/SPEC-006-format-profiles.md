# SPEC-006 — Format profiles

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 1 (JSON); extended by later adapters |
| **Depends on** | SPEC-004, SPEC-005 |
| **Requirements** | IN-11, IN-20, IN-21, IN-22, IN-23, IN-24, IN-26, FV-03, SC-01 |
| **Technical spec** | §A2, §A6.2, §B2.2, §B2.3 |
| **Code / content paths** | `internal/profile`, `content/profiles/`, `docs/fixtures.md`, `cmd/recon-dev` (`profile-test`) |

## Goal

A market format is supported by writing a profile file, not code. Detection picks exactly one profile per file, and every profile is proven against hand-verified samples.

## Scope

- Profile schema `recon.profile/1`: identity, `detect`, `context`, `facts`, `records` (§B2.2).
- Detect predicates for JSON: `json_exists`, `json_equals`, `json_matches` (regex). Later adapters add their own predicates in their specs.
- `context`: tax year, PAN, period, scope (GSTIN/branch), document part.
- Fact mapping: `fact`, selector, `unit`, `required`, optional `sign` (`as_is` / `negate`), optional `rounding`.
- Record mapping: `record_type`, row selector, field/amount selectors, `match_key` (used from SPEC-020).
- Loader: strict YAML; every `fact`/`record_type` must exist in the vocabulary (FV-03); selectors must be valid for the declared file type.
- Detection: run every profile's `detect` on a file; zero matches → `PROFILE_NO_MATCH`; more than one → `PROFILE_AMBIGUOUS` (IN-23). Never guess.
- Executor: `(profile, Doc) → facts, records, ValidationReport`. A missing `required` figure is a report error, never a silent zero.
- Profile tests: `content/profiles/<source>/<document>/<version>/samples/*` with `expected/*.facts.yaml`; `recon-dev profile-test` compares exactly (names, values, periods, locators).
- Synthetic generic formats (IN-26): `generic.taxsuite.computation` and `generic.taxsuite.tax_audit_report` (JSON), documented in `docs/fixtures.md`, each with at least two samples.

## Out of scope

- Real tax-suite profiles (blocked on Q1; added as new profile folders when samples arrive).
- Excel/Word/Tally/PDF selectors (their adapter specs).

## Acceptance criteria

- [ ] AC-1 — Profile load fails for: unknown key, fact name not in vocabulary, invalid selector, unquoted version/tax year.
- [ ] AC-2 — A file matching no profile is rejected with `PROFILE_NO_MATCH` and a message naming the file; a file matching two profiles is rejected with `PROFILE_AMBIGUOUS` naming both.
- [ ] AC-3 — Running the same file through the executor twice gives identical facts in identical order.
- [ ] AC-4 — A missing `required` figure produces a validation-report error and no fact; it does not produce a zero.
- [ ] AC-5 — `recon-dev profile-test` passes for both generic profiles, and fails (naming the fact) when one expected value is edited.
- [ ] AC-6 — `docs/fixtures.md` describes the generic formats field by field.
- [ ] AC-7 — Adding a new version of a profile is a new folder; no Go code changes (demonstrated by adding `generic.taxsuite.computation` v2 with one moved field).

## Verification

```
go test ./internal/profile/ -v
go run ./cmd/recon-dev profile-test ./content/profiles/...
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
