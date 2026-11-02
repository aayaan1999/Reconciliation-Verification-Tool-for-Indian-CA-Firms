# Incident response

Purpose: what to do when something goes wrong with Tool 1 at the firm. It draws on the invariants in [.claude/rules/project-invariants.md](../../.claude/rules/project-invariants.md) (invariants 1, 5 and 6), the security controls and pack rollback in [tool1-technical-spec.md](../../tool1-technical-spec.md) (§B7, §B10, §B12) and the risks in the [implementation plan](../Reconciliation%20%26%20Verification%20Tool%20-%20Implementation%20plan.md) §12. Steps that depend on features not yet built are marked **Planned**.

## Contacts

All Open; fill in before the pilot.

| Role | Name | How to reach |
|---|---|---|
| Developer (first contact) | <name> | <phone / email> |
| CA who approves rules | <name> (approver name pending) | <phone / email> |
| Firm Partner (decides on client notice) | <name> | <phone / email> |
| Office IT contact (machine, network, backup drive) | <name> | <phone / email> |

## Always

1. **Write it down first.** Open an incident log entry (template below) before changing anything.
2. **Keep the evidence.** Copy the database file (with `-wal` and `-shm`), the input files involved and any console output to a safe folder on the office machine. Never copy client files into this repository, an email or a chat.
3. **Do not edit figures or findings by hand.** Decisions and findings are append-only (SPEC-006 part 3).
4. Tell the developer the same day.

## A wrong finding in the field

A rule that fires wrongly is **disabled, not tuned** (invariant 6; plan §11, rule 6).

1. Record the finding ID, rule ID, client (internal number only), tax year, both values and both sources shown.
2. Check the arithmetic shown on the finding against the source documents. If the figures were read from the wrong place (a profile or extraction problem, not the rule), say so in the log; the developer fixes the profile with a new hand-checked sample.
3. If the rule itself is wrong: stop it running. Today's build has no disable command, so keep it out of runs: a rule in `testing` runs only with `--include-testing`, and within a comparison type `--comparison <type> --rules <other IDs>` runs only the rules named. How to stop one rule in a full run with `--include-testing` is Open until the disable command exists; ask the developer. Rule-store files are not edited by hand (CI checks they still match the workbook). A one-click disable is **Planned** (§B9, §B12.1; who may disable is Open, T-Q3).
4. Mark the finding "Not an issue" with a note that names the incident, so the decision is kept.
5. The developer adds the field case as a failing fixture (with synthetic figures in the same shape, never the client's data) and writes a new rule version. The CA approves it before it is switched on again. The old version is never edited.
6. Re-run Test Pack A and the other client runs that used the rule.

A finding that states a "should", a correct value or a judgment is a defect in its wording (invariant 1). Treat it the same way and report it.

## A suspected data leak

1. Disconnect the office machine from the network if the leak may be ongoing. Stop `recon-server`.
2. Tell the Partner at once. Whether clients or any authority must be told is the firm's decision; the DPDP legal opinion is Open (requirements Q7).
3. Note what may have left: which clients, which files, how (network port open without logins, a copied database, a USB drive, a backup).
4. Today the database is not encrypted at rest and the server has no logins or HTTPS; check whether the port was reachable beyond the office (see the [runbook](runbook.md), step 3).
5. Check the engine network proof for the build in use (`build/engine-symbols-*.txt` from its CI artefact). The engine has no network code by design (invariant 5); a doubt here is a release-blocking defect.
6. **Planned:** verify the audit hash chain (§B12.2) to see who viewed or exported which client file.

## A corrupted database

1. Stop `recon-server`. Do not run `load-rules` or any command that writes to the file.
2. Keep the damaged file and its `-wal` and `-shm` files as they are, in a separate folder.
3. Restore the latest backup to a scratch folder, check a few clients with `recon-engine result`, then put it in place and restart.
4. Work done since that backup (uploads, decisions, classifications) has to be redone from the original files. List it in the log.
5. **Planned:** start-up integrity check, signed backups and the recovery code for new hardware (§B12.2, ADR 0002).

## A failed update

1. Stop `recon-server`. Put back the previous programs and content folders kept from before the update, and the backup taken before it.
2. Run `load-rules` again only if the rules changed; loading adds versions and never overwrites one.
3. Re-run Test Pack A on the restored build to confirm it behaves as before.
4. **Planned:** a pack install rejects a bad signature, a SHA-256 mismatch or a failing fixture as a whole, and rollback re-activates the previous versions recorded by the install (§B10). `recon-updater` keeps the previous binaries for rollback.

## Incident log template

| Field | Entry |
|---|---|
| Date and time found | |
| Found by | |
| Type | wrong finding / data leak / corrupted DB / failed update / other |
| Build and commit in use | |
| Rules involved | |
| What happened | |
| Evidence kept (where, not the data itself) | |
| Actions taken, with times | |
| Who was told, when | |
| Follow-up (new fixture, rule version, spec) | |
| Closed by, date | |

Keep the log on the office machine with the firm's records. It may name internal client numbers but not PANs or figures.
