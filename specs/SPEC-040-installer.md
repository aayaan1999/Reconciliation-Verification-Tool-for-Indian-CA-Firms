# SPEC-040 — Installer and packaging

| | |
|---|---|
| **Status** | Not started (licence check: Q4) |
| **Phase** | 6 (Phase 6 gate) |
| **Depends on** | SPEC-034, SPEC-035 |
| **Requirements** | ENV-01, ENV-02, ENV-03, ENV-06, requirements §11 (packaging); CLAUDE.md invariant 8 |
| **Technical spec** | §B15 |
| **Code / content paths** | `installer/`, CI release job |

## Goal

A partner with no IT help can install Tool 1 on the main desktop from a Windows installer or a portable zip, and staff can then reach it from their browsers.

## Scope

- Windows installer:
  - copies the binaries and the bundled Poppler;
  - creates the data directory;
  - generates the master key and prints the recovery code (SPEC-034);
  - generates certificates (SPEC-035);
  - registers `recon-server` as a Windows service with auto-start;
  - opens the LAN port in Windows Firewall, for the private network profile only;
  - creates the first Partner account.
- Portable zip: the same binaries, with a first-run setup command that does the same steps without an installer.
- Uninstall keeps the data directory unless the Partner explicitly chooses to delete it.
- No runtime prerequisites: the installer test runs on a clean Windows VM without Go, Python, extra .NET runtimes or Docker.
- Licence check: implemented or removed, depending on the answer to Q4.
- Release artefacts: binaries, installer, zip, the `engine-deps.txt` proof, SHA-256 sums and Ed25519 signatures (signing key from SPEC-041).

## Acceptance criteria

- [ ] AC-1 — On a clean Windows VM: install, open the UI from a second VM on the same network, and log in as Partner.
- [ ] AC-2 — After a reboot, the service starts and the UI is reachable without anyone logging in on the main desktop.
- [ ] AC-3 — The portable zip reaches the same state as the installer.
- [ ] AC-4 — Uninstall leaves the data directory, and a reinstall picks it up.
- [ ] AC-5 — Release artefacts include the engine dependency proof and the signatures.
- [ ] AC-6 — Phase 6 gate: the security checklist (SPEC-035) passes on an installed system.

## Verification

Run the manual install test script in `installer/TESTING.md` on clean VMs, and record the results below.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
