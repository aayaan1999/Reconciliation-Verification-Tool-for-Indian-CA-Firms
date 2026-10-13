# SPEC-041 — Content packs: sign, verify, install, rollback

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 7 (Phase 7 gate) |
| **Depends on** | SPEC-027, SPEC-036 |
| **Requirements** | ENV-05, RU-13, RU-41, NF-06, SC-01, SC-02, SC-03 |
| **Technical spec** | §A10, §B10 |
| **Code / content paths** | `internal/pack`, `cmd/recon-dev` (`pack build`, `pack sign`), `internal/api` (packs), Packs screen in `web/` |

## Goal

Updates to rules, profiles, law tables and the vocabulary arrive on USB as signed packs. They are verified offline, never overwrite office rules, and can be rolled back.

## Scope

- The pack format of §B10: a zip with `manifest.json`, `manifest.sig`, `content/` and `CHANGELOG.md`.
- `recon-dev pack build` collects the content, runs all profile tests and rule fixtures, and records the results in the manifest. `recon-dev pack sign` signs the pack with an offline Ed25519 key. Both are vendor-side tools and are never shipped.
- Public key(s) embedded in `recon-server`, with support for a second key for rotation.
- Install (Partner only), in order:
  1. verify the signature and file hashes;
  2. check version compatibility;
  3. strict-load every item;
  4. run the pack's fixtures in the engine;
  5. show the office-rule conflict screen, where the CA chooses per rule;
  6. show the changelog;
  7. install in one transaction.
- Rollback to the previous pack state; nothing is deleted.
- Packs list screen: installed packs, versions, who installed them and when.

## Acceptance criteria

- [ ] AC-1 — A pack with one changed byte, or one signed with an unknown key, is rejected before anything is loaded.
- [ ] AC-2 — A pack with a failing fixture is rejected as a whole.
- [ ] AC-3 — A pack that changes a built-in rule the CA has edited shows both versions, and the office version is kept unless the CA chooses otherwise.
- [ ] AC-4 — Install followed by rollback returns active rules, profiles and law rows to exactly the previous state (compared by hash).
- [ ] AC-5 — A run before the install and a run after install and rollback give identical results.
- [ ] AC-6 — Phase 7 gate: a signed pack is installed from a USB drive and rolled back on the test machine (recorded).

## Verification

```
go test ./internal/pack/... -v
go run ./cmd/recon-dev pack build --content content --out test.reconpack
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
