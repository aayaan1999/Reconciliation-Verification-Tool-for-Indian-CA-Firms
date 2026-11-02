---
name: security-review
description: Review a change, package or spec for security and privacy problems specific to this offline tool that holds client PANs and financial statements - network capability in the engine, client data leaks, SQL and path injection, unsafe parsing, LAN exposure, key handling. Use before a release, after changes to ingest, store, server or updater code, or when asked for a security review.
---

# Security review

Checks a change against `.claude/rules/security-and-privacy.md` and the threats in `docs/security/data-and-threat-model.md`. Read-only: report findings, do not fix them.

## Scope

Use the diff (`git diff` and `git status`) unless files, a package or a spec are named. Read each changed file in full and grep for its callers.

## Checklist

**Network (invariant 5)**
- No new import of `net`, `net/*`, `crypto/tls` or `golang.org/x/net/*` in engine-side code. Any new entry in `allowedNetworkImports` has a reason and is parse-only.
- `go test -count=1 ./internal/archtest/ -v` passes; compare `build/engine-deps-*.txt` and `build/engine-symbols-*.txt` with the previous run if available.
- `recon-server` binds only to the address given; no default of `0.0.0.0` or `:port`.
- The UI loads no remote resource (grep `web/` for `http://`, `https://`, `//cdn`, `fonts.googleapis`).

**Client data**
- No real PANs, GSTINs, names or figures in code, tests, fixtures, samples or docs. A PAN-shaped string in new content is checked against the fictional packs.
- Logs and error messages do not print PANs or amounts beyond what a finding shows.
- No database file or private data folder is added to git.

**Injection and parsing**
- SQL is parameterised; no string-built queries.
- File paths from uploads or input cannot escape their folder; uploaded names are not used as paths.
- `pdftotext` and other programs are run with argument lists, never a shell.
- Parsers of untrusted files limit size and depth and reject malformed input rather than guessing.

**Storage, keys and packs**
- Encryption at rest (ADR 0002) is not wired in yet; flag any new place client data is written outside the database, and once SPEC-034 lands, check the database opens only through the encrypted VFS outside tests.
- No private keys or secrets committed; only `*.pub.pem`.
- Rule packs and updates are signature-checked (Ed25519) before use, and a failed check refuses the pack.

**Server and access**
- Each API handler checks the caller's role (`internal/authz`) where the tech spec requires it.
- Decisions and audit records stay append-only.

## Report

Summary and verdict first. Then each finding, most severe first: severity (Critical / High / Medium / Low), `path:line`, the problem and a concrete scenario, the current code and a minimal fix. Critical is any breach of an invariant, a data leak or a remote-reachable hole. End with checks that passed and anything not checked.
