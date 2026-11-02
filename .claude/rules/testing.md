---
paths:
  - "**/*_test.go"
  - "testdata/**"
  - "content/rules/**"
  - "web/src/**/*.test.tsx"
---

# Testing

Strategy: `docs/quality/test-strategy.md`. Gates: `docs/quality/ci-quality-gates.md`.

## Rules

- **Synthetic data only.** Never use real client names, PANs, GSTINs or figures.
- **Every rule has at least one passing and one failing fixture** under `content/rules/<year>/<category>/<RULE-ID>/fixtures/{pass,fail}/`. `go run ./cmd/recon-dev rule-lint` checks this. Add near-boundary cases (`within_tolerance`, `just_over_tolerance`) where a tolerance applies.
- **Each new rule goes in `testdata/testpack_nimbus/score.yaml`** so Test Pack A scoring maps findings to it. Test Pack A must keep 0 false positives against its negative controls.
- **Map acceptance criteria to tests.** Each criterion in a spec is proven by at least one named test or verification command.
- **Deterministic tests.** No sleeps for timing, no clock, no randomness without a fixed seed, no dependence on map order. Run twice and compare bytes where output is produced.
- **Cover the project's edge cases:** paise rounding and overflow, Indian digit grouping, negatives in parentheses, missing figures ("cannot check", never zero), 31 March / 1 April period boundaries, strict YAML (unquoted years and dates, unknown keys).
- **Golden files change only on purpose.** Regenerate with the documented `-update` flag and review the diff.
- Run with `-count=1` so caching cannot hide a failure. The architecture guard (`internal/archtest`) is part of every full test run.
- Poppler tests run only on Windows; run them from PowerShell if a sandboxed Bash shell cannot start `pdftotext.exe`.
