# Runbook

Purpose: how to build Tool 1, put it on one office Windows machine, serve it on the office network and run it day to day. It summarises the "Commands" section of [CLAUDE.md](../../CLAUDE.md), the technical spec ([tool1-technical-spec.md](../../tool1-technical-spec.md) §B10, §B12, §B15, §B16) and the program flags in `cmd/recon-server`. Items the technical spec plans but the code does not yet have are marked **Planned**.

All names, PANs and figures below are illustrative. Never use real client data outside the firm's own machine.

## What exists today and what is planned

| Item | State |
|---|---|
| `recon-engine` (CLI: load rules, run, facts, clients, decide, worklist) | Built |
| `recon-server` (browser screens and test API, starts `recon-engine`) | Built; no logins yet |
| Logins and roles (Partner, Manager, Staff, Read-only) | **Planned**, Module 16 (2–3 Nov), §B12.1 |
| Encryption at rest (AES-256-XTS VFS, ADR 0002) | Driver linked and tested; **today `internal/store` opens a plain SQLite file**. Wiring with key handling is **Planned**, Module 14 |
| HTTPS on the LAN | **Planned**, §B12.2; T-Q2 Open |
| Windows installer and Windows service | **Planned**, Module 14, §B15; WiX licence decision Open (SPEC-002) |
| Daily encrypted backup with restore test | **Planned**, §B12.2 (NF-07) |
| Signed rule packs by USB, rollback | **Planned**, Module 19, §B10 |
| `recon-updater` for binary updates | **Planned**, Phase 7, §B10 |

## 1. Build (developer machine)

Go is at `C:\Program Files\Go\bin`; put it on `PATH` if `go` is not found. Node.js 24 is needed only for the UI.

```
cd web && npm ci && npm run build && cd ..
GOOS=windows GOARCH=amd64 go build -tags ui -o build/windows/ ./cmd/...
```

The build is pure Go with cgo off, so it cross-compiles from any OS. Before copying a build to the office, pass the [release checklist](release-checklist.md).

## 2. Install on the office machine

Until the installer exists (**Planned**), install by copying:

1. Create a folder, for example `C:\ReconTool\`.
2. Copy `build/windows/recon-engine.exe` and `recon-server.exe` into it. `recon-server` looks for `recon-engine` in its own folder.
3. Copy the content folders it reads: `content/` (rules, vocabulary, profiles, comparisons, worklist thresholds) and `docs/Recon_Tool_YAML_Rule_Store/recon_yaml/`. Either keep the same relative paths under the install folder and start the server from there, or pass the paths with the flags in step 3.
4. For PDF reading, copy `third_party/poppler/` (GPL, run as a separate program; [ADR 0003](../architecture/decisions/ADR-0003-bundle-poppler.md)).
5. Create a data folder outside the install folder, for example `D:\ReconData\`. Keep it off any synced or shared drive.

Nothing is installed on staff PCs; they use a browser (C2, C7).

## 3. Start recon-server on the LAN

```
recon-server.exe --listen :8080 --db D:\ReconData\recon.db ^
  --store docs\Recon_Tool_YAML_Rule_Store\recon_yaml --rules content\rules ^
  --vocab content\vocabulary --profiles content\profiles ^
  --comparisons content\comparisons\comparisons.yaml
```

- `--listen 127.0.0.1:8080` (the default) serves this machine only; `:8080` serves the office network. Staff open `http://<office-machine-name>:8080/`.
- `--office-profiles` sets where layouts taught on the mapping screen are kept (default: a `profiles` folder beside the database).
- Until logins (Module 16) and HTTPS are built, anyone on the office network who can reach the port can use the tool, over plain HTTP. For the 30 October demo and pilot the plan runs on one computer without logins (plan §5). Allow the port in Windows Firewall for the office subnet only, or keep the default `127.0.0.1`.
- Stop with Ctrl+C. Running as a Windows service is **Planned** (installer).

## 4. Load rules

```
recon-engine load-rules --db D:\ReconData\recon.db --store docs\Recon_Tool_YAML_Rule_Store\recon_yaml --rules content\rules
recon-engine rules --db D:\ReconData\recon.db
```

Loading again with no change changes nothing; a changed file adds a new version. Rule-book rules load `disabled` and engine rules load `testing` until the CA approves them (SPEC-001, SPEC-003). Runs include testing rules only with `--include-testing`.

## 5. Add clients

```
echo ABCDE1234F | recon-engine client-add --db D:\ReconData\recon.db --name "Example Traders" --tax-year 2025-26
recon-engine clients --db D:\ReconData\recon.db --tax-year 2025-26
recon-engine client-add --db D:\ReconData\recon.db --tax-year 2026-27 < details.json
```

The PAN goes on standard input so it is not left in the command history. Clients can also be added on the screens.

## 6. Run checks

```
recon-engine run --db D:\ReconData\recon.db --input <folder or file> --tax-year 2025-26 --client 1 [--include-testing] [--json report.json]
recon-engine result --db D:\ReconData\recon.db --client 1 --tax-year 2025-26 --json out.json
```

Exit codes: 0 no findings, 1 findings, 2 input or rule error, 3 internal error. `--comparison <type>` runs one comparison type (list them with `recon-engine comparisons`); with it, `--rules <IDs>` runs only the chosen rules of that type. Decisions and the Worklist: `decide`, `worklist`, `classify` (see CLAUDE.md).

## 7. Back up the database

Today (plain SQLite in WAL mode):

1. Stop `recon-server` and make sure no `recon-engine` run is active.
2. Copy `recon.db` together with any `recon.db-wal` and `recon.db-shm` beside it, and the office `profiles` folder, to the backup drive.
3. Restart the server. Keep backups on an encrypted drive in a locked place: the file holds client PANs and figures and is not yet encrypted.
4. Test a restore now and then: copy the backup to a scratch folder, run `recon-engine result` for a client and compare with the live result.

**Planned** (§B12.2): a daily encrypted copy made with the SQLite online backup API to a configured path, N days kept, a monthly restore test reported in Admin, and a recovery code printed once at install for restore on new hardware.

## 8. Where things live

| What | Where |
|---|---|
| Programs | Install folder, for example `C:\ReconTool\` |
| Database | The `--db` path, plus `-wal` and `-shm` files beside it |
| Office format profiles | `--office-profiles`, default a `profiles` folder beside the database |
| Logs | `recon-server` and `recon-engine` write messages to standard error (the console). No log file is kept today; redirect the console to a file if one is needed. The audit hash chain is **Planned** (§B12.2) |
| Rule content | `content/rules/` and the YAML rule store |

## 9. Updates

Today: replace the programs and content folders by copying a new tested build, after a backup, then run `load-rules` again. **Planned**: signed `.reconpack` files from USB with signature and SHA-256 checks, conflict check against office rules and rollback (§B10); binary updates by `recon-updater`, which keeps the previous binaries.
