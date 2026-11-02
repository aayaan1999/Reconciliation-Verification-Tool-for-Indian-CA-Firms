# Release Rules

Version: 1.0 | Updated: 2026-10-23 | Owner: <to be assigned> | Status: Proposed — change to Approved on sign-off (agents treat unapproved documents as guidance only, SDLC Rules G-12)

Applies to: every release, deployment, hotfix, configuration change and data migration to any shared environment (test, staging, production), on every project and stack.
MUST = mandatory. SHOULD = default, deviate only with a stated reason. MAY = optional. MUST NOT = prohibited.
Tags: [H] = humans, [A] = AI agents, [B] = both. Untagged rules apply to both.
Conflict order: law and regulation → client contracts → Security & Privacy Policy → AI Usage Policy → SDLC Rules → Architecture Decision Framework / Code Review Rules / Release Rules / IT Standard Rules / Testing Rules → approved project ADRs and project rules → stack-specific rules → tool defaults. A higher document wins when two documents conflict. A lower document MAY add stricter requirements; that is not a conflict. Within the same level, the stricter rule applies. Report conflicts; never choose silently. A project deviation needs a recorded exception (owner, reason, expiry).

Related documents (not repeated here):
- security-privacy-policy.md — data classes, security controls, incident and breach notification
- ai-usage-policy.md — what AI agents may access and do
- sdlc-rules.md — phases, gates, approvals
- code-review-rules.md — review process and finding severity (Critical / High / Medium / Low / Info)
- it-standard-rules.md — coding practices (IT Standard Rules)
- architecture-decision-framework.md — architecture options, scoring, ADRs and technology stack selection
- testing-rules.md — test standards (Testing Rules)

Note for teams coming from low-code (for example Appian): the platform's packages, import/export and deployment tooling handled versioning, environment promotion and much of rollback for you. On a code stack the team owns all of it. These rules make that ownership explicit.

## Values to set

| Value | Default (editable) | Used in |
|---|---|---|
| Lead time: standard / normal / major change | 0 / 2 / 5 working days | RR-1 |
| Approvers: standard / normal / major / emergency | Pre-approved / tech lead + release approver / + product owner + security reviewer / on-call lead + release approver | RR-1, RR-8 |
| Canary / staged rollout steps | 5% → 25% → 50% → 100% | RR-7 |
| Minimum time at each rollout step | 30 minutes | RR-7 |
| Release window | Mon–Thu, business hours IST, support available | RR-7 |
| Freeze periods | Month-end/quarter-end close, major client events, public holidays | RR-7, RR-17 |
| Post-deploy watch window | 60 minutes (major: 24 hours) | RR-10 |
| Rollback trigger: error rate | > 2× baseline or > 1% of requests for 5 minutes | RR-11 |
| Rollback trigger: latency | p95 > NFR target for 10 minutes | RR-11 |
| Rollback trigger: business KPI | < 80% of baseline (orders, logins, submissions) | RR-11 |
| Maximum time to decide on rollback | 15 minutes after a trigger fires | RR-11 |
| Hotfix full review after deploy | 1 working day | RR-12.8 |
| Hotfix retro-documentation deadline | 2 working days | RR-12.6 |
| Hypercare period: normal / major | 3 / 10 working days | RR-13 |
| Release record retention | 3 years, or longer if law or contract requires | RR-16 |

## When to use this file

| Phase | Sections |
|---|---|
| 0 Readiness | 3, 16, 17 |
| 1 Requirements | 1 (expected change classes), 7 (availability and downtime NFRs) |
| 2 Design | 3, 5, 6, 7, 11 (environments, deployment strategy, migration and rollback approach) |
| 3 Plan | 1, 4, 5, 6, 11 (release scope, readiness criteria, migration and rollback tasks) |
| 4 Configure | 0, 3, 6, 9 (pipeline, environments, flags) |
| 5 Build & Verify | 0, 2, 4, 5, 6 (release notes, evidence, rehearsals) |
| 6 Release | All sections |
| 7 Operate & Learn | 10, 11, 12, 13, 15, 16 |

## 0. Always (agent quick rules)

- RR-0.1 [A] MUST NOT deploy to production, run scripts or migrations against production, or change production configuration, secrets or feature flags.
- RR-0.2 [A] MUST NOT approve a release, a go/no-go decision, an exception or a rollback.
- RR-0.3 [A] MAY prepare, for human review: release notes, checklists, rollback plans, migration scripts, runbooks, change records and communication drafts.
- RR-0.4 [A] MUST stop and report if asked to skip, weaken or work around a release gate, test, review or approval.
- RR-0.5 [A] MUST mark anything it could not verify (environment state, test results, metrics) as unverified.
- RR-0.6 [A] MUST include a rollback or recovery step for every deployment or migration step it writes.
- RR-0.7 [A] MUST NOT include secrets, personal data or production data in release notes, runbooks, scripts or change records.

## 1. Change classes

- RR-1.1 [B] Every change to a shared environment MUST be classified before planning: standard, normal, major or emergency.
- RR-1.2 [B] Standard: low-risk, repeatable, pre-approved change with a tested procedure (for example a routine dependency patch with no behaviour change). MUST follow its pre-approved procedure exactly.
- RR-1.3 [B] Normal: any change that is not standard, major or emergency. MUST follow the full process in this file.
- RR-1.4 [B] Major: new product or module, breaking API or contract change, schema change on large or Restricted data, infrastructure or authentication change, or change affecting several services or clients.
- RR-1.5 [B] Emergency: fixes an active production incident, security vulnerability under exploitation, or legal/compliance breach. MUST follow section 12.
- RR-1.6 [H] Approvals and lead times MUST follow the values table. Approvers are named people, not roles alone.
- RR-1.7 [B] When unsure, classify as major. Use emergency only for the cases in RR-1.5.
- RR-1.8 [H] A standard change that fails or causes an incident MUST lose its pre-approved status until reviewed.

## 2. Versioning, tagging and release notes

- RR-2.1 [B] Releases MUST use a documented versioning scheme; semantic versioning (MAJOR.MINOR.PATCH) SHOULD be the default: MAJOR for breaking changes, MINOR for compatible features, PATCH for compatible fixes.
- RR-2.2 [B] Each release MUST have an immutable tag in version control pointing to the exact commit built. Tags MUST NOT be moved or reused.
- RR-2.3 [B] The deployed artifact MUST be traceable to its tag, commit, build and pipeline run.
- RR-2.4 [B] Every release MUST have release notes (Appendix D) listing: version, date, changes, linked tasks and requirement IDs, breaking changes, migrations, configuration and flag changes, known issues, rollback notes.
- RR-2.5 [B] A changelog MUST be kept in the repository and updated with every release.
- RR-2.6 [B] Release scope MUST be traceable: every change in the release maps to a task, and every task to a requirement (see SDLC Rules traceability).
- RR-2.7 [B] Changes not listed in the release scope MUST NOT be included.

## 3. Environments and promotion

- RR-3.1 [B] Changes MUST be promoted in order: dev → test → staging → production. No environment may be skipped for normal or major changes.
- RR-3.2 [B] Build once: the same artifact MUST be promoted through every environment. Rebuilding per environment is not allowed.
- RR-3.3 [B] Staging MUST match production in versions, configuration shape, infrastructure type and data volume order, as far as cost allows. Known differences MUST be documented.
- RR-3.4 [B] Environment-specific configuration MUST live outside the artifact (see IT Standard Rules CDEP-008, CDEP-009).
- RR-3.5 [B] Production changes MUST go through the pipeline or infrastructure as code. Manual changes in production MUST NOT be made, except approved emergency actions that are recorded and back-ported the same day.
- RR-3.6 [B] Production data MUST NOT be copied to lower environments unless masked or anonymised as required by the Security & Privacy Policy.
- RR-3.7 [B] Each environment MUST have its own credentials, secrets and access list.
- RR-3.8 [H] Configuration drift between environments SHOULD be detected and fixed regularly.

## 4. Release readiness (entry criteria)

A release MUST NOT go to production until all of these are true, or formally accepted as exceptions:

- RR-4.1 [B] Definition of Done is met for every item in scope.
- RR-4.2 [B] Traceability is complete: requirement → task → test → result.
- RR-4.3 [B] Test evidence exists for functional, regression, performance against NFR targets, security and accessibility tests, as required by the test plan.
- RR-4.4 [B] No open Critical or High review findings, unless accepted under Code Review Rules CR-9.5.
- RR-4.5 [B] No open Critical or High vulnerabilities, unless accepted under Security & Privacy Policy SP-24.4; all other vulnerabilities are within the fix deadlines (SP-24.3).
- RR-4.6 [B] Documentation, runbooks, API contracts and user guides are updated.
- RR-4.7 [B] Monitoring, logs, dashboards and alerts for new features exist and have been checked in staging.
- RR-4.8 [B] The rollback plan (Appendix C) is written and tested in staging.
- RR-4.9 [B] Data migrations are rehearsed in staging (section 5).
- RR-4.10 [H] Support and operations teams are briefed; known issues are shared.
- RR-4.11 [H] Legal, compliance, privacy or client sign-offs are obtained where required.
- RR-4.12 [B] The release checklist (Appendix A) is complete and attached to the change record.

## 5. Database and data migrations

- RR-5.1 [B] Schema changes MUST follow expand → migrate → contract: add new structures, move data, remove old structures only in a later release.
- RR-5.2 [B] Each migration MUST be backward compatible with the previous application version, so the app can be rolled back without rolling back the schema.
- RR-5.3 [B] A verified backup or snapshot MUST be taken before any production migration.
- RR-5.4 [B] Migrations MUST be rehearsed in staging on production-like data volume, and the run time recorded.
- RR-5.5 [B] The migration plan MUST state expected duration, lock and performance impact, and whether users are affected.
- RR-5.6 [B] Every migration MUST be reversible, or have a documented recovery path (see IT Standard Rules CDB-007, CDB-008).
- RR-5.7 [B] Schema changes and data changes SHOULD be separate migrations.
- RR-5.8 [B] Long-running or large data migrations MUST run online in batches, be restartable, and be monitored.
- RR-5.9 [B] Migrations MUST be version-controlled, run by the pipeline, and recorded in the change record.
- RR-5.10 [A] MUST write migration scripts with a matching rollback or recovery script and a verification query.

## 6. Configuration, secrets and feature flags

- RR-6.1 [B] Required configuration MUST be validated at startup; the app MUST fail fast with a clear message that reveals no secrets.
- RR-6.2 [B] Secrets and configuration MUST exist in the target environment before deployment starts.
- RR-6.3 [B] Every feature flag MUST have an owner, a purpose, a default (off for new features), and a removal date.
- RR-6.4 [B] Risky features MUST have a kill switch that can turn them off without a deployment.
- RR-6.5 [B] Changing a flag or configuration in production is a change: it MUST be approved, logged and reversible.
- RR-6.6 [B] Flags past their removal date MUST be removed in the next release.
- RR-6.7 [B] Flag combinations used in production MUST have been tested.

## 7. Deployment strategy

- RR-7.1 [B] The deployment strategy MUST be chosen per risk in Design and recorded: rolling, blue/green, canary or staged percentage rollout.
- RR-7.2 [B] Major changes SHOULD use canary or staged rollout with the steps and minimum time per step in the values table.
- RR-7.3 [B] Zero-downtime deployment MUST be used where the availability NFR requires it.
- RR-7.4 [B] Planned downtime MUST be agreed with stakeholders and announced in advance.
- RR-7.5 [B] Releases MUST happen within the release window and MUST NOT happen in freeze periods, except emergency changes or an exception signed off by the release approver (RR-17.2).
- RR-7.6 [B] Releases SHOULD NOT happen when the team, on-call or support is unavailable to respond.
- RR-7.7 [B] When several services or components change, the deployment order and compatibility between versions MUST be documented.
- RR-7.8 [B] Each rollout step MUST meet the watch criteria (section 10) before moving to the next.

## 8. Go / no-go decision

- RR-8.1 [H] Every normal and major release MUST have a go/no-go decision by the named release approver.
- RR-8.2 [H] The decision MUST be based on the release checklist and readiness criteria (section 4).
- RR-8.3 [H] Known risks and open issues MUST be listed and accepted explicitly, by name.
- RR-8.4 [H] The decision MAY be made in a meeting or asynchronously, but MUST be recorded (Appendix B).
- RR-8.5 [H] Any missing criterion means no-go, unless covered by an approved exception.
- RR-8.6 [H] A no-go MUST record the reason and what is needed to proceed.

## 9. Deployment execution

- RR-9.1 [B] Deployments MUST run through the approved automated pipeline.
- RR-9.2 [H] Each deployment MUST have a named deployer and a named backup.
- RR-9.3 [B] Each deployment MUST follow a written runbook with steps, checks, and rollback points.
- RR-9.4 [B] Each deployment MUST have a change record linking version, scope, approvals, checklist and evidence.
- RR-9.5 [H] Stakeholders MUST be told before, during (if long or user-visible) and after the deployment.
- RR-9.6 [B] The pipeline MUST keep an audit log of who deployed what, where and when.
- RR-9.7 [H] Deployers MUST stop at the first unexpected result and follow the rollback criteria; they MUST NOT improvise fixes in production.

## 10. Post-deploy verification

- RR-10.1 [B] Smoke tests MUST run immediately after deployment and cover the critical user journeys.
- RR-10.2 [B] Health checks MUST pass on all instances.
- RR-10.3 [B] Key metrics MUST be watched for the watch window: error rate, latency, saturation (CPU, memory, connections, queues) and business KPIs.
- RR-10.4 [B] Metrics MUST be compared with a pre-release baseline.
- RR-10.5 [B] Logs and alerts MUST be confirmed working for new features.
- RR-10.6 [H] The deployer MUST record the verification result in the change record before closing it.

## 11. Rollback and roll-forward

- RR-11.1 [B] Rollback criteria MUST be defined before release, using the thresholds in the values table or stricter project values.
- RR-11.2 [H] A named person MUST own the rollback decision, and MUST decide within the maximum decision time.
- RR-11.3 [B] Rollback MUST be tested in staging before a normal or major release.
- RR-11.4 [B] The rollback plan MUST state what happens to data written by the new version and to migrations already run.
- RR-11.5 [B] When a trigger fires and the cause is not understood, the default is rollback.
- RR-11.6 [H] Roll-forward (fixing forward) MAY be used only for small, understood fixes, with release approver approval, and MUST still follow review and pipeline rules.
- RR-11.7 [B] Every rollback MUST be followed by a post-release review (section 15).

## 12. Hotfix and emergency changes

- RR-12.1 [B] Emergency changes are allowed only for the cases in RR-1.5.
- RR-12.2 [B] Scope MUST be the minimum change that fixes the problem.
- RR-12.3 [B] Review MAY be expedited but MUST NOT be skipped: at least one qualified reviewer approves before deploy. If the fix is security-sensitive (Code Review Rules CR-8.9), that reviewer MUST be a security reviewer.
- RR-12.4 [B] The fix MUST have a test that proves it, and the pipeline's automated checks MUST pass.
- RR-12.5 [B] The fix MUST be merged back to main so the next release includes it.
- RR-12.6 [H] The emergency record (Appendix E) MUST be completed within the retro-documentation deadline.
- RR-12.7 [H] Repeated emergency changes in the same area MUST trigger a root-cause review.
- RR-12.8 [H] A full review under Code Review Rules MUST follow within the deadline in the values table.

## 13. Hypercare and handover

- RR-13.1 [H] Every normal and major release MUST have a hypercare period (values table) with named on-call owners.
- RR-13.2 [B] A known-issues list MUST be kept and shared with support during hypercare.
- RR-13.3 [B] The support knowledge base and runbooks MUST be updated before hypercare ends.
- RR-13.4 [H] Handover to steady-state support MUST be signed off by the support lead.

## 14. Communication

- RR-14.1 [H] Each release MUST have a communication plan: who is told, when, and through which channel.
- RR-14.2 [B] Release notes SHOULD exist in two versions: technical (for teams and support) and user-facing (plain language, no internal details).
- RR-14.3 [H] Planned downtime, user-visible changes and breaking API changes MUST be announced in advance to affected users and integrators.
- RR-14.4 [H] A status page or channel SHOULD be updated during incidents and rollbacks.

## 15. Release-related incidents and learning

- RR-15.1 [H] Operational incidents (outage, degraded service, failed release) MUST follow: detect → name an incident lead → contain (rollback, flag off) → communicate (status page or channel) → resolve → blameless review. Security incidents and data breaches MUST also follow Security & Privacy Policy §26 (Incidents and breach notification).
- RR-15.2 [H] Every failed, rolled-back or emergency release MUST have a blameless post-release review.
- RR-15.3 [H] The review MUST name the gate, test, check or rule that missed the problem, and an owner to fix it.
- RR-15.4 [B] Lessons MUST be fed back into this file, the release checklist, tests or monitoring.
- RR-15.5 [H] SHOULD track release metrics: deployment frequency, change failure rate, time to restore, rollbacks.

## 16. Records and audit

- RR-16.1 [B] For every release, keep: change record, approvals, go/no-go record, checklist, test evidence, deployment logs, verification result, release notes.
- RR-16.2 [B] Records MUST be kept for the retention period in the values table.
- RR-16.3 [B] Records MUST NOT contain secrets or unmasked Restricted data.
- RR-16.4 [B] Records MUST be stored in an approved, access-controlled location.

## 17. Organisational controls (humans only; agents skip)

- RR-17.1 [H] Maintain a shared release calendar showing planned releases, windows and freeze periods.
- RR-17.2 [H] Freeze periods MUST be published in advance; exceptions need release approver sign-off.
- RR-17.3 [H] Where the organisation has a Change Advisory Board (CAB), major changes MUST go through it.
- RR-17.4 [H] Segregation of duties: for major and high-risk changes, the person who built the change MUST NOT be the only approver for production.
- RR-17.5 [H] Access to production deployment pipelines MUST be limited to named, approved people and reviewed regularly.
- RR-17.6 [H] Release approvers and on-call rotas MUST be named and kept current.

---

## Appendix A: Release checklist

```text
Release: <version>      Change class: standard / normal / major / emergency
Change record: <ID>     Planned date and window: <...>

Scope and traceability
[ ] Scope list with task and requirement IDs
[ ] Release notes (technical and user-facing) drafted
[ ] Version tag created on the built commit

Verification
[ ] Definition of Done met for all items
[ ] Functional and regression tests passed (evidence linked)
[ ] Performance tested against NFR targets
[ ] Security tests and scans passed; no open Critical / High vulnerabilities (or accepted per SP-24.4); others within deadlines
[ ] Security headers and TLS configuration verified
[ ] Penetration test done where required (Security & Privacy Policy, Values to set)
[ ] Accessibility checks passed (UI changes)
[ ] No open Critical / High findings, or exceptions approved

Data and configuration
[ ] Migrations rehearsed in staging; duration recorded
[ ] Backup / snapshot plan confirmed
[ ] Configuration and secrets present in production
[ ] Feature flags set; kill switches tested

Operations
[ ] Deployment strategy and order documented
[ ] Rollback plan written and tested in staging
[ ] Monitoring, dashboards and alerts ready
[ ] Runbooks and support knowledge base updated
[ ] Support briefed; on-call and hypercare owners named
[ ] Communication plan ready

Approvals
[ ] Required sign-offs (legal / compliance / client) obtained
[ ] Go/no-go recorded
```

## Appendix B: Go / no-go record

```text
Release: <version>      Date/time: <...>      Approver: <name>
Participants: <names>
Checklist complete: yes / no (missing items: ...)
Known risks accepted (risk — owner — mitigation):
  - ...
Exceptions (ID — owner — expiry):
  - ...
Decision: GO / NO-GO
Reason (if no-go) and what is needed to proceed:
```

## Appendix C: Rollback plan

```text
Release: <version>      Rollback owner: <name>      Backup: <name>
Rollback triggers (metric — threshold — duration):
  - Error rate:
  - Latency:
  - Business KPI:
Maximum time to decide:
Rollback method: redeploy previous version / switch blue-green / turn off flag / other
Steps (with checks after each):
  1.
Data impact: migrations already run, data written by new version, how it is handled
Tested in staging on: <date>      Result:
Verification after rollback:
```

## Appendix D: Release notes

```text
Version: <x.y.z>      Date: <...>
Summary: <one or two lines>
New features: <item — task / requirement IDs>
Changes: <...>
Fixes: <...>
Breaking changes and required actions: <...>
Migrations: <...>
Configuration and feature flag changes: <...>
Known issues: <...>
Rollback notes: <...>
```

## Appendix E: Hotfix / emergency change record

```text
Hotfix: <version>      Incident / reason: <ID and summary>
Why emergency (RR-1.5 case): <...>
Change made (minimal scope): <...>
Reviewer: <name>      Tests added: <...>      Pipeline checks passed: yes / no
Approved by: <name>   Deployed at: <date/time>   Deployed by: <name>
Verification result: <...>
Merged back to main: <commit / PR>
Follow-up actions and owner: <...>
Retro-documentation completed on: <date>
```

---

## Revision history

| Version | Date | Summary |
|---|---|---|
| 1.0 | 2026-10-23 | First versioned issue. Set-wide conflict order (adds the Architecture Decision Framework; lower documents may be stricter); IT Standard Rules references updated to v1.3 identifiers; Testing Rules now referenced as written; versioning scheme made explicit with semantic versioning as the default. |
