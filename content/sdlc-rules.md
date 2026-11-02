# SDLC Rules

Version: 1.0 | Updated: 2026-10-23 | Owner: <to be assigned> | Status: Proposed — change to Approved on sign-off (agents treat unapproved documents as guidance only, SDLC Rules G-12)

Applies to: every product and change built with AI coding agents (Claude Code and similar), on every project and stack, using spec-driven development.
MUST = mandatory. SHOULD = default, deviate only with a stated reason. MAY = optional. MUST NOT = prohibited.
Tags: [H] = humans, [A] = AI agents, [B] = both. Untagged rules apply to both.
Conflict order: law and regulation → client contracts → Security & Privacy Policy → AI Usage Policy → SDLC Rules → Architecture Decision Framework / Code Review Rules / Release Rules / IT Standard Rules / Testing Rules → approved project ADRs and project rules → stack-specific rules → tool defaults. A higher document wins when two documents conflict. A lower document MAY add stricter requirements; that is not a conflict. Within the same level, the stricter rule applies. Report conflicts; never choose silently. A project deviation needs a recorded exception (owner, reason, expiry).

Related documents (not repeated here):
- security-privacy-policy.md — data classes, security controls, security work per phase (§23)
- ai-usage-policy.md — what agents may access and do
- code-review-rules.md — review process, checklist, finding severity
- release-rules.md — release, deploy, rollback, hotfix
- it-standard-rules.md — coding practices (IT Standard Rules)
- testing-rules.md — test standards (Testing Rules); the §5 TS rules are the agent quick set
- architecture-decision-framework.md — how architecture options are scored and recorded

## Values to set

| Value | Default (editable) | Used in |
|---|---|---|
| Product owner / tech lead / security reviewer / release approver | <names> | RD-2, gates |
| Documents folder | `docs/` | RD-5 |
| Specs folder | `docs/specs/` | §SD |
| ADR folder | `docs/adr/` | DS-5 |
| Max attempts at the same fix before escalating | 2 | G-13 |
| Clarifying questions at start of requirements | 5–8 | RQ-1 |
| Agent session cost limit | <amount> | CF-15 |
| Milestone length | <weeks> | PL-18 |

## When to use this file

Apply §0 and §SD always, plus only the section for the current phase.

| Phase | Section | Give the agent these documents | Do NOT give yet |
|---|---|---|---|
| 0 Readiness | §RD | This file, AI Usage Policy, Security & Privacy Policy | — |
| 1 Requirements | §1 | Context note; policy constraint lines; Security & Privacy §1–6, §23.1; NFR checklist from the architecture framework (ADRV-002, ADRV-008); Testing Rules Section C (requirement testability) and Annex 2 (risk tiers) | Coding rules, other testing rules, review rules, architecture patterns, tech stack |
| 2 Design | §2 | Approved requirements; architecture framework (full); Security & Privacy (full relevant parts); IT Standard Rules §4, §9 | Testing rules, review rules, release rules |
| 3 Plan | §3 | Approved design and ADRs; IT Standard Rules §5, §6; Testing Rules (Annex 8 core profile for the risk tier, plus the sections the design needs); DoR / DoD | Release rules (except migration and rollout planning) |
| 4 Configure | §4 | IT Standard Rules (full); Testing Rules digest and path-scoped extracts (Testing Rules §0.6); Code Review Rules; AI Usage Policy §6 | — |
| 5 Build & Verify | §5 | Task contract, linked specs and ADRs; rules already loaded from Phase 4 | Unrelated specs |
| 6 Release | §6 | Release Rules; release scope; test evidence | — |
| 7 Operate & Learn | §7 | Release Rules; incident sections of Security & Privacy Policy | — |

Reason: give a document in the phase where the decision it governs is made. Given earlier, it biases decisions it was never meant to make (for example, coding rules turning into product requirements). Given later, its guidance is missed.

## 0. Always (every phase)

- G-1 [A] MUST ask which phase we are in if it is not stated.
- G-2 [A] MUST follow only the current phase's rules and MUST NOT jump ahead (no code in requirements; no tech stack in design until options are compared).
- G-3 MUST mark statements as Verified fact, Assumption or Open question.
- G-4 [A] MUST NOT invent numbers, facts, requirements, packages, APIs or approvals. Unknown = Open question.
- G-5 [A] MUST NOT claim done, tested or secure without evidence (command run + result).
- G-6 [A] MUST stop and ask when requirements are unclear, rules conflict, or an action is irreversible.
- G-7 [A] MUST follow the AI Usage Policy and the Security & Privacy Policy in every phase.
- G-8 [A] MUST NOT touch production, delete data, force-push or change permissions.
- G-9 [A] MUST work only within the given task scope; report out-of-scope issues instead of fixing them.
- G-10 [A] MUST preserve work it did not create; never overwrite, revert or reformat it.
- G-11 SHOULD prefer the simplest solution that meets stated requirements.
- G-12 [A] MUST treat documents marked draft as guidance, not policy.
- G-13 [A] MUST stop after the maximum attempts (Values to set) at the same fix and report what was tried.
- G-14 SHOULD verify external facts (library versions, APIs, laws, prices) against current official sources.
- G-15 MUST record decisions in project documents, not only in chat.
- G-16 [A] MUST point out contradictions between documents, specs or instructions before continuing.
- G-17 [A] MUST update the existing source-of-truth document instead of creating a new one; ask before creating new documents.
- G-18 [A] SHOULD cite the rule ID when a rule drives a decision or a refusal.
- G-19 SHOULD start a fresh agent session for each new phase or unrelated task.
- G-20 SHOULD summarise session outcomes into project documents before the session ends.
- G-21 MUST prefer reversible actions when options are otherwise equal.
- G-22 [H] A human MUST approve every phase gate, merge and release.

## SD. Spec-driven development (every phase)

- SD-1 The approved spec is the source of truth for behaviour. Code MUST conform to the spec, not the other way round.
- SD-2 Behaviour changes MUST change the spec first, get approval, then change the code.
- SD-3 [A] MUST generate implementation only from specs with status Approved.
- SD-4 Every spec MUST have: ID, status, owner, linked requirements, scope, out of scope, main flow, failure and abuse cases, acceptance criteria, data and Security & privacy impact (Security & Privacy Policy Appendix A), observability, rollout and rollback, open questions.
- SD-5 Spec status lifecycle: Draft → In review → Approved → Implemented → Superseded. Only a human moves a spec to Approved.
- SD-6 Specs MUST live in version control (Values to set) and be reviewed like code (Code Review Rules §8.1).
- SD-7 [A] MUST report drift when code and spec disagree, and MUST NOT silently "fix" either one.
- SD-8 Each spec SHOULD be small enough to implement and review in a few tasks; split large specs.
- SD-9 Acceptance criteria MUST be written so they can become automated tests (Given / When / Then).

## RD. Phase 0 — Readiness

- RD-1 [H] MUST approve the AI Usage Policy before project data or code is shared with an agent.
- RD-2 [H] MUST name roles: product owner, tech lead, security reviewer, release approver. One person MAY hold several roles on a small team, but the same person SHOULD NOT both write and be the sole approver of a high-risk change.
- RD-3 [H] MUST give the agent development access only, never production credentials.
- RD-4 [H] MUST have a secrets manager or approved alternative before the first line of code.
- RD-5 [H] SHOULD set up the repository with: documents folder, short `CLAUDE.md`, `.gitignore` for secrets, branch protection, basic CI.
- RD-6 [H] SHOULD have dev and test environments separate from production.
- RD-7 [B] MUST list missing prerequisite documents as risks with an owner and due phase.
- RD-8 [H] MUST agree an escalation path for requirements, security and release questions.
- Done when: policies approved or risks logged; roles named; access confirmed.

## 1. Phase 1 — Requirements

Purpose: understand the problem, users, scope and constraints before any solution is discussed.

- RQ-1 [A] MUST start by asking the highest-impact clarifying questions (Values to set).
- RQ-2 MUST describe what and why, never how. No stack, pattern or code unless it is a stated constraint.
- RQ-3 Each requirement MUST have: ID (REQ-xxx), priority (Must / Should / Could / Won't), owner, testable acceptance criteria.
- RQ-4 MUST make each requirement atomic and unambiguous; replace vague words ("fast", "easy", "secure") with measurable criteria.
- RQ-5 User journeys MUST include failure paths: invalid input, no permission, external system down, duplicate action, timeout.
- RQ-6 MUST list abuse cases (misuse, fraud, scraping, privilege escalation) next to journeys.
- RQ-7 NFRs MUST cover: security, performance, availability, reliability, maintainability, scalability, recovery (RPO / RTO), accessibility. Number or Open question; never invented.
- RQ-8 MUST list constraints separately: budget, deadline, team skills, mandated tech, hosting, integrations, compliance, data location.
- RQ-9 MUST build a data inventory: each data item, class (Security & Privacy Policy §1), purpose, retention.
- RQ-10 MUST produce a roles and permissions matrix: what each user role may see and do.
- RQ-11 MUST list external systems and integrations, with owner and availability.
- RQ-12 MUST list compliance obligations and the required security level (Security & Privacy Policy §1, §28).
- RQ-13 MUST list stakeholders, decision makers and approvers.
- RQ-14 MUST define success measures that can be measured after launch.
- RQ-15 MUST list non-goals.
- RQ-16 MUST split scope into first release (MVP) and later releases.
- RQ-17 MUST keep assumptions, risks, open questions and dependencies in separate lists, each with an owner.
- RQ-18 SHOULD describe the current (as-is) process if the product replaces or changes one.
- RQ-19 SHOULD keep a glossary of business terms so names stay consistent in specs and code.
- RQ-20 SHOULD capture localisation needs: languages, time zones, currencies, date and number formats.
- RQ-21 MUST NOT turn coding rules into product requirements.
- RQ-22 [A] MUST NOT add requirements the stakeholder did not state; propose them as suggestions.
- RQ-23 [B] MUST assign the product's risk tier (Testing Rules GOV-009 and Annex 2); the tier sets the depth of design, testing and review in later phases.
- Output: product brief, requirements, NFRs, constraints, journeys, abuse cases, data inventory, roles matrix, risk and question log.
- Done when: product owner approves; every Must has testable criteria; NFRs measurable or marked open; data classes known; risk tier assigned.

## 2. Phase 2 — Design

Purpose: choose the architecture and security approach that best meets the approved requirements.

- DS-1 MUST derive design from requirements, NFRs and constraints, not preference, trend or agent defaults.
- DS-2 MUST compare at least two viable options for each significant decision (architecture style, database, hosting, authentication, key integration, tenant isolation).
- DS-3 MUST score options with the architecture framework's weighted criteria; security, privacy and compliance are pass/fail gates.
- DS-4 SHOULD start from the simplest viable architecture (often a modular monolith); microservices, event-driven or serverless need a stated requirement.
- DS-5 MUST write an ADR for each significant decision: context, options, decision, rationale, consequences, validation, approver, revisit conditions. Record rejected options and why.
- DS-6 MUST cover: system context, modules and boundaries, data model and ownership, integrations, deployment and environments, security, quality targets, operations.
- DS-7 MUST define integration behaviour: protocol, auth, timeouts, retries, idempotency, error handling, fallback.
- DS-8 MUST define failure modes: what happens when each dependency is slow or down, and how the product degrades.
- DS-9 MUST define backup approach and recovery targets (RPO / RTO).
- DS-10 MUST define observability: logs, metrics, alerts, health checks, tracing, audit trail.
- DS-11 MUST define environments (dev, test, staging, production) and how configuration differs.
- DS-12 MUST include the threat model and privacy impact assessment where required (Security & Privacy Policy §23.2).
- DS-13 MUST define tenant isolation if the product serves several clients (Security & Privacy Policy §8).
- DS-14 MUST include accessibility in UI design, using the standard named in requirements.
- DS-15 SHOULD include UX flows or wireframes for key journeys.
- DS-16 SHOULD estimate running cost, vendor lock-in and exit cost.
- DS-17 SHOULD use C4-style diagrams: context, container, and component where useful.
- DS-18 SHOULD define a feature flag and configuration strategy.
- DS-19 SHOULD define the deployment strategy and migration approach early (Release Rules §5, §7).
- DS-20 MUST list high-risk assumptions with a validation plan (prototype, proof of concept, performance or security test).
- DS-21 MUST map every Must requirement to a design element.
- DS-22 [A] MUST NOT write implementation code.
- DS-23 MUST record the build, buy or configure decision (for example SaaS, an existing low-code platform such as Appian, custom build, or a hybrid) before comparing custom architectures (Architecture Decision Framework AALT-001).
- Output: context diagram, architecture description, ADRs, data model, threat model, validation plan.
- Done when: tech lead approves ADRs; security reviewer approves threat model where required.

## 3. Phase 3 — Plan

Purpose: turn the design into specs, contracts, small tasks and a test plan.

- PL-1 MUST write feature specs per §SD and get them approved.
- PL-2 MUST define contracts before code: APIs, schemas, events, error formats.
- PL-3 MUST split work into tasks small enough to review in one sitting (Code Review Rules, Values to set).
- PL-4 Each task MUST have: ID, linked spec and requirement, acceptance criteria, file or module scope, dependencies, verification steps, owner (human or agent role).
- PL-5 MUST keep traceability: requirement → spec → task → test.
- PL-6 MUST plan tests per the Testing Rules, including failure, abuse, security, performance, accessibility and recovery tests as the NFRs require.
- PL-7 MUST define CI gates before build: build, lint, tests, secret and dependency scans, static analysis.
- PL-8 MUST give parallel tasks non-overlapping file ownership or a stated integration order.
- PL-9 MUST plan migration, rollout and rollback for any change to stored data (Release Rules §5).
- PL-10 SHOULD plan vertical slices that deliver working user value end to end.
- PL-11 SHOULD schedule the highest-risk tasks first.
- PL-12 MUST time-box research spikes and state the question each spike answers.
- PL-13 MUST list environment, test data and access needs for each task.
- PL-14 MUST identify the critical path and blocking dependencies.
- PL-15 MUST mark tasks that need human-only actions (approvals, credentials, production steps).
- PL-16 SHOULD state estimate confidence (low / medium / high) and the unknowns behind it.
- PL-17 MUST plan documentation updates inside tasks, not as a later task.
- PL-18 SHOULD define milestones with a demo or review at each one.
- PL-19 MUST agree Definition of Ready and Definition of Done (Appendix).
- PL-20 [A] MUST NOT write implementation code.
- Output: approved specs, contracts, task list, traceability matrix, test plan, DoR, DoD.
- Done when: tech lead approves; every task meets Definition of Ready.

## 4. Phase 4 — Configure Claude Code

Purpose: set up the agent so it works inside the rules automatically, not by memory.

- CF-1 Root `CLAUDE.md` MUST be short: purpose, sources of truth, exact build / test / lint / run commands, working rules, completion report format. It points to documents; it does not copy them.
- CF-2 MUST put coding, testing and review rules where the agent loads them (`.claude/rules/`), split by area. Always-loaded content MUST stay short; large standards (for example the full Testing Rules) MUST be loaded as path-scoped rules, skills or on-demand references, not in full every session (Testing Rules §0.6).
- CF-3 Stack-specific rules MAY extend generic rules but MUST NOT override them.
- CF-4 SHOULD create skills or commands only for workflows already done manually at least once.
- CF-5 SHOULD enforce non-negotiable rules with hooks, CI and permission settings, not text alone.
- CF-6 Permissions MUST be least privilege: deny production, destructive commands and secret files (AI Usage Policy §6).
- CF-7 Each sub-agent MUST have one role, scope, file ownership and output format. The reviewer agent MUST be independent of the writer agent.
- CF-8 SHOULD keep one file describing all agent roles and how they hand off work.
- CF-9 MUST use only approved MCP servers and connectors, with the narrowest scope.
- CF-10 MUST NOT put secrets, personal data or client-confidential details in agent files.
- CF-11 SHOULD add hooks that run the formatter and fast tests after edits.
- CF-12 Agent configuration MUST be version-controlled and reviewed (Code Review Rules §8.6).
- CF-13 MUST update `CLAUDE.md` and rules when commands, structure or rules change.
- CF-14 SHOULD review agent configuration at each milestone; remove unused rules, skills and agents.
- CF-15 SHOULD set session cost and usage limits (Values to set).
- Done when: configuration reviewed; one trial task runs end to end within the rules.

## 5. Phase 5 — Build and Verify

Purpose: implement approved specs in small, tested, reviewed steps.

### Build
- BD-1 [A] MUST read the task, linked spec, requirements and existing code before changing anything.
- BD-2 [A] MUST state the plan before large changes.
- BD-3 One task per change; no unrelated refactoring (IT Standard Rules CCLN-014).
- BD-4 MUST follow existing project patterns, naming and structure.
- BD-5 [A] MUST stop and escalate before touching security controls, data migrations, contracts or production configuration outside the plan.
- BD-6 MUST NOT add a dependency without stating why and checking maintenance, licence and security (IT Standard Rules CDEP-001, CDEP-002); significant dependencies also need an ADR (IT Standard Rules CDEP-003).
- BD-7 MUST NOT hardcode configuration, URLs, credentials or environment-specific values.
- BD-8 SHOULD hide incomplete features behind feature flags.
- BD-9 MUST make small commits with clear messages that include the task ID.
- BD-10 MUST update affected specs, docs, contracts and traceability in the same change.
- BD-11 MUST NOT leave TODOs without a linked task ID.
- BD-12 MUST keep the branch up to date with main and resolve conflicts without dropping others' work.

### Test (agent quick rules; the full standard is testing-rules.md)
- TS-1 MUST run the planned tests and show results (Testing Rules AIQA-027).
- TS-2 MUST test behaviour, not implementation details; tests MUST fail if the feature breaks, and expected results MUST come from the spec (Testing Rules AIQA-024).
- TS-3 MUST cover failure and abuse cases.
- TS-4 MUST NOT weaken, skip or delete tests or assertions to make them pass (Testing Rules AUTO-037, AIQA-007).
- TS-5 MUST NOT use real personal data or secrets in tests.
- TS-6 Tests MUST be deterministic and independent (Testing Rules AUTO-013, AUTO-015).
- TS-7 SHOULD place each test at the lowest level that reliably detects the defect; the mix of unit, integration and end-to-end tests follows the project's risks, not a fixed pyramid (Testing Rules LVL-002, AUTO-004).
- TS-8 MUST run the full relevant suite before reporting done.
- TS-9 MUST report flaky tests instead of retrying until green (Testing Rules AUTO-029 to AUTO-032).
- TS-10 MUST add a regression test for every fixed bug (Testing Rules REG-010).
- TS-11 MUST add accessibility checks for UI changes and performance checks for NFR-critical paths.

### Review and CI
- RV-1 Every change MUST be reviewed per code-review-rules.md before merge.
- RV-2 [H] A human MUST approve every merge; AI-created changes MUST NOT be auto-merged.
- CI-1 Every change MUST pass CI gates: build, lint, tests, secret and dependency scans.
- CI-2 MUST NOT bypass a failing gate without a recorded human decision.
- CI-3 MUST build once and promote the same artifact through environments.
- CI-4 [A] MUST NOT change CI pipelines without an approved task.

### Completion report (every task)
- Summary and linked task / spec / requirement
- Files changed
- Verification run and results
- Review findings and how they were resolved
- Specs, docs and traceability updated
- Risks, limitations and what was not checked
- Approval needed
- Done when: Definition of Done met, CI green, human approval recorded.

## 6. Phase 6 — Release

- RL-1 MUST follow release-rules.md for every release, deployment, hotfix and migration.
- RL-2 [A] MUST NOT deploy to production, run scripts on production data, or approve releases. It MAY prepare release notes, checklists, rollback plans and migration scripts for human review.
- RL-3 MUST confirm the release checklist, rollback plan, monitoring and go / no-go record exist before release.
- Done when: human go / no-go recorded; post-deploy verification passed; hypercare owner named.

## 7. Phase 7 — Operate and Learn

- OP-1 MUST follow the incident process in the Security & Privacy Policy §26 and Release Rules §15.
- OP-2 Incident and escaped-defect reviews MUST be blameless and name the rule, spec, test or gate that failed, with an owner to fix it.
- OP-3 MUST put each lesson into the right document or rule file, not only meeting notes.
- OP-4 MUST revisit ADRs when their revisit conditions are met.
- OP-5 MUST monitor service levels against NFR targets and alert on breaches.
- OP-6 MUST keep runbooks for common failures and on-call steps.
- OP-7 MUST measure the success measures from Phase 1 and report against them.
- OP-8 MUST keep a technical debt register with owner and impact.
- OP-9 SHOULD update dependencies regularly, not only when forced.
- OP-10 SHOULD remove unused features, flags, code and data.
- OP-11 SHOULD track delivery metrics: lead time, deployment frequency, change failure rate, time to restore, escaped defects.
- OP-12 SHOULD hold a retrospective at each milestone and update rules from it.

## 8. Organisational controls (humans only; agents skip)

- OR-1 [H] Each phase gate MUST be approved by the named approver and recorded.
- OR-2 [H] Team members MUST complete training on AI tools and these rule files before using agents.
- OR-3 [H] This file MUST be reviewed every 6 months and after major incidents or tool changes.
- OR-4 [H] Project teams MAY add project rules; they MUST NOT weaken a MUST rule without a recorded exception.

## Appendix — Definition of Ready and Definition of Done

Definition of Ready (task):
- Linked spec Approved; requirement accepted; acceptance criteria testable
- Contracts and design decisions approved
- Dependencies known and available
- File scope and owner assigned
- Verification steps and test data defined
- Small enough to review in one sitting

Definition of Done (task):
- Acceptance criteria met; tests pass with evidence
- Review done per Code Review Rules; no open Critical or High findings
- CI gates green
- Specs, docs, contracts and traceability updated
- No secrets, debug code, unlinked TODOs or unrelated changes
- Human approval recorded

## Appendix — Gate approvals

| Gate | Approver | Evidence |
|---|---|---|
| Phase 0 → 1 | Delivery lead | Policies approved or risks logged; roles named; access confirmed |
| Phase 1 → 2 | Product owner | Approved requirements, NFRs, data classes, roles matrix |
| Phase 2 → 3 | Tech lead + security reviewer | Approved ADRs; threat model where required |
| Phase 3 → 4 | Tech lead | Approved specs, contracts, tasks, traceability, test plan, DoR / DoD |
| Phase 4 → 5 | Tech lead | Agent configuration reviewed; trial task passed |
| Each merge | Human reviewer | CI green; findings resolved; completion report |
| Release | Release approver | Release Rules checklist and go / no-go record |

---

## Revision history

| Version | Date | Summary |
|---|---|---|
| 1.0 | 2026-10-23 | First versioned issue. Set-wide conflict order (adds the Architecture Decision Framework; lower documents may be stricter); Testing Rules now written, so §5 TS rules become the agent quick set with cross-references and TS-7 no longer prescribes a fixed test pyramid; phase table loads Testing Rules by section; added RQ-23 (risk tier) and DS-23 (build, buy or configure); CF-2 limits always-loaded content; IT Standard Rules references updated to v1.3 identifiers. |
