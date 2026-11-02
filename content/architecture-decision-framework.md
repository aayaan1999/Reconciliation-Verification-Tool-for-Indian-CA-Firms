# APPLICATION ARCHITECTURE DECISION FRAMEWORK

**A requirements-driven standard for selecting, documenting, and governing application architecture and technology stacks**

| Attribute | Value |
|---|---|
| Document | `architecture-decision-framework.md` (referred to in the standard set as the Architecture Decision Framework) |
| Version | 1.1 |
| Status | Proposed — change to Approved on sign-off (agents treat unapproved documents as guidance only, SDLC Rules G-12) |
| Issue date | 2026-10-23 (v1.1 and v1.0; v1.0 converted from the earlier unversioned Word draft) |
| Document owner | *To be assigned by the adopting organization (see AGOV-003)* |
| Review cycle | At least annually, and when a trigger in AREV-003 occurs |
| Audience | Architects, tech leads, developers, product owners, security and operations engineers, approvers, and AI coding agents used for architecture and implementation work |
| Related standards | `security-privacy-policy.md`, `ai-usage-policy.md`, `sdlc-rules.md`, `it-standard-rules.md` v1.3, `testing-rules.md` v1.2, `code-review-rules.md`, `release-rules.md` |

---

## Table of Contents

- [0. How to Use This Standard](#0-how-to-use-this-standard)
- [A. Principles and Governance (AGOV)](#a-principles-and-governance-agov)
- [B. Decision Drivers (ADRV)](#b-decision-drivers-adrv)
- [C. Architecture Alternatives (AALT)](#c-architecture-alternatives-aalt)
- [D. Objective Evaluation (AEVL)](#d-objective-evaluation-aevl)
- [E. Pattern, Deployment Model, and Integration Style (APAT)](#e-pattern-deployment-model-and-integration-style-apat)
- [F. Technology Stack Selection (ASTK)](#f-technology-stack-selection-astk)
- [G. Architecture Description (ADSN)](#g-architecture-description-adsn)
- [H. Architecture Decision Records (AADR)](#h-architecture-decision-records-aadr)
- [I. AI-Assisted Architecture Work (AAI)](#i-ai-assisted-architecture-work-aai)
- [J. Architecture Conformance and Fitness Functions (AFIT)](#j-architecture-conformance-and-fitness-functions-afit)
- [K. Review, Approval, and Reassessment (AREV)](#k-review-approval-and-reassessment-arev)
- [Annex A. ADR Template](#annex-a-adr-template)
- [Annex B. Scoring Sheet Template and Worked Example](#annex-b-scoring-sheet-template-and-worked-example)
- [Annex C. Architecture Review Checklist](#annex-c-architecture-review-checklist)
- [Annex D. Glossary](#annex-d-glossary)
- [Annex E. References](#annex-e-references)
- [Annex F. Rule Prefix Index](#annex-f-rule-prefix-index)

---

## 0. How to Use This Standard

### 0.1 Purpose

This standard defines how an application's architecture and technology stack are chosen, documented, approved, enforced, and revisited. Its aim is to select the architecture that best satisfies defined requirements at an acceptable level of risk and total cost, **not** the most advanced or fashionable architecture.

### 0.2 Normative language

The key words below are interpreted as described in BCP 14 (IETF RFC 2119 and RFC 8174) **only when they appear in capital letters**:

| Term | Meaning in this standard |
|---|---|
| **MUST** / **MUST NOT** | Mandatory. Non-conformance requires an approved exception (AREV-006). |
| **SHOULD** / **SHOULD NOT** | Recommended. Deviation is permitted when the reason is recorded in the architecture description or an ADR. |
| **MAY** | Optional practice. |

Lowercase "must", "should", and "may" in explanatory text are not normative. Numeric values in this standard (weights, margins, horizons) are **illustrative** unless a rule states otherwise.

### 0.3 Applicability and risk tiers

Rules are labeled **Universal**, **Conditional** (with the condition), or **Architecture-dependent** (with the architectural feature). "Tier" refers to the organization's risk-tier model, which is shared across the standard set and currently defined in Annex 2 of `testing-rules.md` (Tier 1 = Critical to Tier 4 = Low). Using the same tier for architecture and testing keeps the depth of design and the depth of verification consistent.

### 0.4 Rule format

Each rule has a stable identifier (`PREFIX-NNN`) that is never reused or renumbered. Tables use four columns: **ID**, **Requirement**, **Applicability**, and **Verification and evidence**.

### 0.5 Relationship to other standards

- This standard decides **structure and technology**. It references the other standards where their topics meet architecture rather than repeating them. In particular, verification of the decisions made here is governed by `testing-rules.md`.
- **Conflict order:** law and regulation → client contracts → Security & Privacy Policy → AI Usage Policy → SDLC Rules → Architecture Decision Framework / Code Review Rules / Release Rules / IT Standard Rules / Testing Rules → approved project ADRs and project rules → stack-specific rules → tool defaults. A higher document wins when two documents conflict. A lower document MAY add stricter requirements; that is not a conflict. Within the same level, the stricter rule applies. Report conflicts; never choose silently. A project deviation needs a recorded exception (owner, reason, expiry).
- **Tags.** Rules are not tagged [H]/[A]/[B]; they apply to humans and AI agents alike, except Section I (AAI), which applies to AI agents.
- **Data classes** are those of the Security & Privacy Policy §1.
- **Phase use (SDLC Rules).** Phase 1: ADRV-002 and ADRV-008 (NFR checklist). Phase 2: the full document. Phase 3: ADSN-006 and Section J. Phase 4: Section I in the always-loaded agent instructions, and Section J checks in CI. Phase 5: Section I and the project's ADR index. Phase 7: AREV-003 and AREV-004.
- Conformance with this standard does not by itself demonstrate conformance with ISO/IEC/IEEE 42010, ISO/IEC 25010, TOGAF, NIST SSDF, or any other external framework (Annex E).

### 0.6 Use with AI coding agents

This standard is short enough to be consulted in full during architecture and specification sessions. For implementation sessions, the project's always-loaded agent instructions SHOULD contain:

1. the rules in Section I (AAI), and
2. a pointer to the project's ADR index and architecture description (AADR-006, AAI-007),

so that every session works within the accepted architecture rather than re-deciding it.

---

## A. Principles and Governance (AGOV)

**Intent and rationale.** Architecture is the set of decisions that are expensive to reverse. These rules ensure those decisions are made deliberately, by accountable people, for documented reasons, and with effort proportional to risk.

| ID | Requirement | Applicability | Verification and evidence |
|---|---|---|---|
| AGOV-001 | Architecture decisions MUST be justified by documented business requirements, non-functional requirements, security and compliance obligations, and constraints; personal preference, popularity, novelty, or "industry trend" MUST NOT be recorded as the rationale for a decision. | Universal | ADR rationale sections reviewed against drivers (Section B). |
| AGOV-002 | Each application MUST be assigned a risk tier before architecture evaluation begins, and the depth of evaluation, documentation, and review MUST follow that tier as set out in this standard. | Universal | Risk-tier record referenced in the architecture description. |
| AGOV-003 | The organization MUST assign a named owner for this standard, and each application MUST have a named architecture owner (role and person) accountable for its decisions and for keeping its architecture records current. | Universal | Ownership records. |
| AGOV-004 | Approval authority for architecture decisions MUST be defined per risk tier; for Tier 1–2 decisions, the proposer MUST NOT be the sole approver. | Universal | Approval authority matrix; ADR approval fields. |
| AGOV-005 | When alternatives satisfy the mandatory gates and drivers comparably, the lower-complexity alternative SHOULD be chosen; each increase in complexity (distribution, asynchronous processing, additional datastores, additional languages or frameworks) MUST be justified by a named requirement. | Universal | ADR rationale linking each complexity source to a requirement. |
| AGOV-006 | Architecture decisions affecting a part of the system MUST be accepted before implementation of that part begins; implementation started earlier MUST be flagged as at risk in the project plan. | Universal | ADR acceptance dates compared with implementation start. |
| AGOV-007 | Projects MUST record which version of this standard they follow. | Universal | Architecture description header. |

---

## B. Decision Drivers (ADRV)

**Intent and rationale.** An architecture can only be judged against what it must achieve. Drivers that are vague or missing produce decisions that cannot be evaluated and are later found to be wrong in production.

| ID | Requirement | Applicability | Verification and evidence |
|---|---|---|---|
| ADRV-001 | The business context MUST be documented: purpose, user groups and expected numbers, business processes supported, functional scope, and what is explicitly out of scope. | Universal | Context section of the architecture description. |
| ADRV-002 | Non-functional requirements MUST be stated as measurable, project-specific targets (for example, security, performance, availability, reliability, recovery time and recovery point, scalability, maintainability, observability, and cost) and MUST be recorded in the project's NFR register so the same targets are later verified (testing standard REQ-005, PERF-001, RES-001). | Universal | NFR register entries with targets, measurement method, and owner. |
| ADRV-003 | Mandatory obligations (security, privacy, regulatory, contractual, data residency, accessibility, organizational policy) MUST be identified and expressed as pass/fail gates before alternatives are scored (Security & Privacy Policy SP-1.4, SP-23.7). | Universal | Gate list with source of each obligation. |
| ADRV-004 | Constraints MUST be documented: budget, timeline, team size and skills, approved or prohibited technologies, hosting options, existing platforms and licenses, and existing integrations. | Universal | Constraints section. |
| ADRV-005 | Data characteristics MUST be documented: data class (Security & Privacy Policy §1), volumes and growth, retention, residency, and the system of record for each major data entity. | Universal | Data profile. |
| ADRV-006 | Each external integration MUST be inventoried with direction, protocol, volume, availability expectation, data exchanged, and owning party. | Conditional — external integrations exist | Integration inventory. |
| ADRV-007 | Assumptions that influence the decision MUST be recorded explicitly, each with an owner and the means and date by which it will be validated. | Universal | Assumptions log. |
| ADRV-008 | The quality characteristics of ISO/IEC 25010 SHOULD be used as a checklist to confirm no relevant quality concern has been missed, and exclusions SHOULD be justified. | Universal | Completed quality-characteristic checklist. |
| ADRV-009 | A total cost of ownership estimate MUST be prepared over a stated horizon, covering build, infrastructure, licenses and subscriptions, monitoring, maintenance, support, training, and exit or migration costs. | Conditional — Tier 1–3 | TCO estimate with horizon and assumptions. |

---

## C. Architecture Alternatives (AALT)

**Intent and rationale.** A decision with only one option considered is not a decision. Comparing genuinely different alternatives, including the simplest one and the option of not building custom software at all, is the main protection against over-engineering.

| ID | Requirement | Applicability | Verification and evidence |
|---|---|---|---|
| AALT-001 | A build, buy, or configure decision MUST be made and recorded before custom architecture alternatives are evaluated, considering at least: an existing SaaS product, configuration of a platform the organization already operates (for example, an existing low-code or workflow platform), custom development, and a hybrid of these. | Universal | ADR for the sourcing decision. |
| AALT-002 | At least two viable alternatives MUST be evaluated for Tier 1–3 applications; Tier 4 applications MAY record a single option with its rationale. | Universal | Options section of the ADR. |
| AALT-003 | The alternatives evaluated MUST include the simplest viable option (for a new custom application, typically a modular monolith with a single primary datastore), unless a mandatory gate rules it out, in which case the gate MUST be named. | Conditional — Tier 1–3 | Options list; elimination reason where applicable. |
| AALT-004 | Alternatives MUST be described at comparable depth (structure, deployment model, data approach, key technologies, operational model) so that comparison is fair. | Universal | Option descriptions in the ADR or scoring sheet. |
| AALT-005 | Alternatives that fail any mandatory gate (ADRV-003) MUST be eliminated before scoring, and the failed gate MUST be recorded. | Universal | Elimination record. |
| AALT-006 | Where a more complex target architecture is anticipated, the chosen option SHOULD describe its evolution path (for example, extracting a module into a service) and the measurable triggers that would justify each step. | Conditional — growth anticipated | Evolution path in the ADR. |

---

## D. Objective Evaluation (AEVL)

**Intent and rationale.** Weighted scoring makes trade-offs explicit and reviewable. It supports judgment; it does not replace it. Scores without defined anchors, weights chosen after seeing results, and narrow margins presented as clear wins are the common ways it is misused.

### D.1 Rules

| ID | Requirement | Applicability | Verification and evidence |
|---|---|---|---|
| AEVL-001 | Viable alternatives MUST be scored against weighted criteria (D.2 or an approved equivalent), and the weights MUST be set and approved before any alternative is scored. | Conditional — Tier 1–3 | Dated weight approval preceding scores. |
| AEVL-002 | Each score MUST use the anchored scale in D.3 and MUST cite the evidence or recorded assumption on which it rests. | Conditional — Tier 1–3 | Scoring sheet (Annex B) with evidence column. |
| AEVL-003 | For Tier 1–2 applications, alternatives SHOULD be scored independently by at least two people, and material differences SHOULD be discussed and resolved before the final score is recorded. | Conditional — Tier 1–2 | Individual and reconciled scores. |
| AEVL-004 | A sensitivity check MUST be performed for Tier 1–2 decisions and SHOULD be performed for Tier 3: where the leading margin is small (illustratively, under 10 points) or a plausible change to any single weight changes the winner, the decision MUST be recorded as a close call and justified qualitatively. | Conditional — Tier 1–3 | Sensitivity note in the ADR. |
| AEVL-005 | A weighted score MUST NOT override a failed mandatory gate; gates are evaluated first and are not part of the score. | Universal | Gate results recorded before scores. |
| AEVL-006 | Tier 1 decisions SHOULD use a recognized scenario-based evaluation method (for example, an ATAM-style analysis of quality-attribute scenarios) in addition to weighted scoring. | Conditional — Tier 1 | Scenario analysis record. |
| AEVL-007 | High-risk assumptions MUST be validated before the decision is accepted for Tier 1–2, and SHOULD be for Tier 3, using a spike, prototype, proof of concept, benchmark, or security assessment; results MUST be recorded as evidence. | Conditional — Tier 1–3 | Validation results linked from the ADR. |

### D.2 Evaluation criteria (illustrative weights)

Weights below are a starting point. Final weights MUST reflect the application's requirements and risk profile (AEVL-001). Security and compliance obligations that are mandatory are handled as gates (AEVL-005); the criterion below scores how well an option goes **beyond** the gate.

| Criterion | Example weight | Key question |
|---|---|---|
| Security and compliance | 15 | How robustly can the option protect data and satisfy obligations beyond the minimum gate? |
| Maintainability and testability | 15 | Can developers safely understand, test, and change it, with clear module boundaries and seams for testing? |
| Team capability and operability | 15 | Can **this** team build, debug, deploy, operate, and support it, including without AI assistance? |
| Reliability and availability | 10 | Can it tolerate failures and meet availability and recovery targets? |
| Performance | 10 | Can it meet response-time and throughput targets under expected workload? |
| Scalability | 10 | Can it accommodate anticipated growth in users, data, and workload? |
| Total cost of ownership | 10 | Is the cost over the stated horizon acceptable (ADRV-009)? |
| Integration flexibility | 10 | Can it integrate reliably with the required external systems? |
| Portability and exit | 5 | How costly would it be to change vendor, platform, or hosting later? |
| **Total** | **100** | |

### D.3 Score anchors

| Score | Meaning |
|---|---|
| 1 | Does not meet the requirement; no credible mitigation. |
| 2 | Meets the requirement only with significant mitigation, cost, or risk. |
| 3 | Meets the requirement adequately with normal effort. |
| 4 | Meets the requirement well, with evidence and some margin. |
| 5 | Exceeds the requirement with strong evidence and clear margin. |

### D.4 Weighted score

```
Weighted score = Σ [ weightᵢ × (scoreᵢ − 1) / 4 ]
```

Weights sum to 100 and scores range from 1 to 5, so the result ranges from 0 (all criteria scored 1) to 100 (all criteria scored 5). Annex B contains a worked example.

---

## E. Pattern, Deployment Model, and Integration Style (APAT)

**Intent and rationale.** "Architecture" is often reduced to one label such as "microservices" or "serverless". These are different kinds of decisions: how the code is structured, how it is deployed and run, and how parts communicate. Deciding them separately avoids false choices; for example, a modular monolith can run on serverless infrastructure.

### E.1 Structural patterns

| Pattern | Suitable when | Main trade-offs |
|---|---|---|
| Layered | Clear separation of presentation, business logic, and data access is useful. | Layers become entangled if boundaries are not enforced. |
| Modular monolith | A cohesive application needs clear internal modules and simple deployment. | Modules deploy together; boundaries need automated enforcement (AFIT-001). |
| Microservices | Distinct capabilities need independent deployment, ownership, or scaling, and the organization can operate distributed systems. | Higher infrastructure, observability, testing, and operational complexity; distributed data consistency. |
| Event-driven | Workloads need asynchronous processing, notifications, or loose coupling between producers and consumers. | Harder tracing, failure handling, ordering, and consistency. |
| Pipeline or batch | Work is processed in stages over data sets, on schedule or in bulk. | Latency; restart, re-run, and reconciliation logic. |

### E.2 Deployment models

| Model | Suitable when | Main trade-offs |
|---|---|---|
| Managed platform (PaaS) or containers | General-purpose services with steady or predictable load. | Platform operations and configuration skills required. |
| Serverless functions | Event-triggered, spiky, or intermittent workloads that fit execution limits. | Execution-time and resource limits, cold starts, cost variability, provider dependency. |
| Virtual machines | Specific OS-level control or legacy constraints. | Highest operational burden. |
| Existing low-code or SaaS platform | Requirements fit the platform's capabilities (see AALT-001). | Platform limits, licensing, and portability. |

### E.3 Integration styles

| Style | Suitable when | Main trade-offs |
|---|---|---|
| Synchronous API | Caller needs an immediate answer. | Temporal coupling; cascading failure risk. |
| Asynchronous messaging or events | Work can complete later, or many consumers react to one event. | Delivery guarantees, duplicates, ordering, and monitoring. |
| File or bulk transfer | Partners exchange data periodically in bulk. | Latency; file validation and reconciliation. |

### E.4 Rules

| ID | Requirement | Applicability | Verification and evidence |
|---|---|---|---|
| APAT-001 | The structural pattern, deployment model, and integration style MUST each be decided and recorded explicitly. | Universal | ADR or architecture description sections. |
| APAT-002 | For new custom applications, a modular monolith with enforced module boundaries SHOULD be the first candidate evaluated; it is a starting point, not a mandatory outcome. | Conditional — new custom applications | Options list. |
| APAT-003 | Microservices MUST NOT be adopted unless the ADR records all three of: (a) a requirement for independent deployment, ownership, or scaling of specific capabilities; (b) teams able to own and operate each service; and (c) existing or funded capability for CI/CD, observability, and contract testing across services. | Conditional — microservices proposed | ADR showing (a), (b), and (c). |
| APAT-004 | Event-driven or asynchronous designs MUST define delivery guarantees, ordering assumptions, idempotency, failure handling (for example, dead-letter handling), and event schema versioning before implementation. | Architecture-dependent — asynchronous messaging | Messaging design section; verified later under testing standard DIST rules. |
| APAT-005 | Serverless designs MUST evaluate execution limits, start-up latency, concurrency limits, cost under expected and peak load, and provider dependency. | Architecture-dependent — serverless | Evaluation notes in the ADR. |
| APAT-006 | Each additional datastore beyond the primary one MUST be justified by a named requirement, and a single system of record MUST be defined for every major data entity. | Universal | Data architecture section. |
| APAT-007 | System boundaries SHOULD align with team ownership and communication paths (Conway's Law); a boundary that no single team owns SHOULD be avoided. | Universal | Ownership mapping per component. |

---

## F. Technology Stack Selection (ASTK)

**Intent and rationale.** Technology choices outlive the people who make them. They must be current, supported, licensed appropriately, and supportable by the team that will own them. Facts about versions, support dates, and licenses change frequently and must be checked, not recalled.

| ID | Requirement | Applicability | Verification and evidence |
|---|---|---|---|
| ASTK-001 | The stack decision MUST list each selected language and runtime, framework, datastore, messaging technology, hosting platform, significant library, and CI/CD and observability tool, with the version selected. | Universal | Stack table in the architecture description. |
| ASTK-002 | For each stack component, the supported version, end-of-support or long-term-support date, license, and recent security advisory history MUST be verified from primary sources (vendor or project documentation) at decision time, with the source and date recorded. | Universal | Stack table with source links and verification dates. |
| ASTK-003 | Components MUST NOT be past end of support at the time of decision, and SHOULD have support covering the planned operating period; otherwise an upgrade or migration plan MUST be recorded. | Universal | Support dates versus planned operating period. |
| ASTK-004 | Component licenses MUST be checked for compatibility with the application's distribution model and organizational policy. | Universal | License review record. |
| ASTK-005 | The team's current proficiency in each primary technology MUST be recorded, with a training or support plan for any gap; for Tier 1–2 applications, a technology that the team could not support without AI assistance MUST NOT be selected unless a support plan is approved. | Universal; stricter for Tier 1–2 | Skills assessment; training or support plan. |
| ASTK-006 | The number of languages, frameworks, and datastores SHOULD be minimized, and each addition MUST be justified by a requirement. | Universal | Stack rationale. |
| ASTK-007 | Where the organization maintains an approved technology list, selections MUST come from it or be approved as an exception. | Conditional — approved list exists | Comparison against the list; exception record. |
| ASTK-008 | Mature, widely adopted, well-documented technologies with strong testing tooling and static type checking SHOULD be preferred over less established alternatives, unless a requirement justifies otherwise. | Universal | Stack rationale. |
| ASTK-009 | Where provider or platform lock-in is material, an exit strategy (data export, portability approach, estimated migration effort) SHOULD be documented for Tier 1–2 applications. | Conditional — Tier 1–2 with material lock-in | Exit strategy section. |
| ASTK-010 | Components added during implementation beyond the accepted stack MUST follow AAI-006. | Universal | ADRs for additions; dependency checks (AFIT-004). |

---

## G. Architecture Description (ADSN)

**Intent and rationale.** The architecture description is the shared reference for developers, reviewers, operators, and AI agents. It must be complete enough to build and operate from, kept close to the code, and kept current.

### G.1 Required content by tier

| Component | Content | Tier 1–2 | Tier 3 | Tier 4 |
|---|---|---|---|---|
| System context | Users, external systems, integrations, boundaries | Required | Required | Required |
| Logical structure | Modules, responsibilities, dependencies, allowed and forbidden dependencies | Required | Required | Brief |
| Data architecture | Entities and ownership, system of record, schema and migration approach, validation, retention, backup and restore | Required | Required | Brief |
| Integration architecture | Interfaces and contracts, protocols, timeouts, retries, idempotency, error handling | Required | Required | If any |
| Deployment architecture | Environments, hosting, networking, promotion, rollback, configuration and secrets management | Required | Required | Brief |
| Security architecture | Authentication, authorization, least privilege, trust boundaries, data flows, encryption, audit trail, threat model | Required | Required | Brief |
| Technology stack | ASTK-001 to ASTK-004 | Required | Required | Required |
| Quality requirements | Measurable targets from the NFR register (ADRV-002) | Required | Required | If any |
| Testability and observability | How the design supports testing and diagnosis (ADSN-005) | Required | Required | Brief |
| Testing approach | Pointer to the project test strategy under the testing standard | Required | Required | Required |
| Operations | Monitoring, alerting, logging, incident response, backup, disaster recovery | Required | Required | Brief |
| Failure modes | Key failure modes and degraded behavior | Required | Recommended | — |
| Cost model | TCO summary (ADRV-009) | Required | Required | — |
| Decisions | ADR index (AADR-006) | Required | Required | Required |

### G.2 Rules

| ID | Requirement | Applicability | Verification and evidence |
|---|---|---|---|
| ADSN-001 | Each application MUST have an architecture description containing the components in G.1 for its tier. | Universal | Architecture description reviewed against G.1. |
| ADSN-002 | Diagrams SHOULD follow the C4 model: Context and Container diagrams are required for Tier 1–3, Component diagrams for the critical parts of Tier 1–2 systems, and Code-level diagrams are optional. | Conditional — Tier 1–3 | Diagrams in the repository. |
| ADSN-003 | The architecture description and diagrams MUST be stored in the application's repository as text (for example, Markdown with Mermaid, PlantUML, or Structurizr DSL diagram sources) where feasible, so that they are versioned with the code and readable by people and AI agents. | Universal | Files in the repository. |
| ADSN-004 | The security architecture MUST identify trust boundaries and sensitive data flows, and threat modeling MUST be performed for Tier 1–2 applications and wherever Security & Privacy Policy SP-23.5 requires it (testing standard SEC-055). | Universal; threat model per SP-23.5 and for Tier 1–2 | Security section; threat model. |
| ADSN-005 | The design MUST state how it supports testing and diagnosis: seams for replacing dependencies in tests, externalized configuration, structured logging, correlation identifiers, and health checks (testing standard PRIN-010, OPS-002). | Universal | Testability and observability section. |
| ADSN-006 | Integration designs MUST define versioned contracts and the behavior for timeouts, retries, duplicates, and dependency failure (testing standard API-001, DIST-010). | Conditional — integrations exist | Integration section; contract files. |
| ADSN-007 | For Tier 1–2 applications, key failure modes and the intended degraded behavior MUST be documented (testing standard RES-002). | Conditional — Tier 1–2 | Failure mode section. |
| ADSN-008 | The architecture description MUST be updated in the same change that alters the architecture, so that it never describes a superseded design. | Universal | Change history of the description alongside code changes. |

---

## H. Architecture Decision Records (AADR)

**Intent and rationale.** An ADR preserves why a decision was made, so that later teams and agents neither repeat settled debates nor unknowingly violate the reasoning. ADRs are short and kept with the code.

| ID | Requirement | Applicability | Verification and evidence |
|---|---|---|---|
| AADR-001 | Every significant decision MUST be recorded as an ADR. A decision is significant when it affects structure, a quality target, the technology stack, integrations, data ownership, security, or cost, or when it would be costly to reverse. | Universal | ADR set compared with the architecture description. |
| AADR-002 | ADRs MUST be stored as Markdown files in the application repository (for example, `docs/adr/0001-short-title.md`) using the template in Annex A. | Universal | Files in the repository. |
| AADR-003 | Each ADR MUST carry a status from: Proposed, Accepted, Rejected, Superseded, or Deprecated. | Universal | Status fields. |
| AADR-004 | Accepted ADRs MUST NOT be rewritten; a changed decision MUST be recorded as a new ADR that supersedes the earlier one, and the earlier ADR's status and link MUST be updated. | Universal | Supersession links; file history. |
| AADR-005 | Each ADR MUST state the triggers that would require the decision to be reconsidered. | Universal | Reconsideration section. |
| AADR-006 | An ADR index (for example, `docs/adr/README.md`) listing each ADR with number, title, status, and date MUST be maintained. | Universal | Index file. |

---

## I. AI-Assisted Architecture Work (AAI)

**Intent and rationale.** AI agents are useful for exploring options, drafting ADRs, and checking designs, but they produce confident, polished recommendations whose facts may be outdated and whose assumptions may be invented. During implementation, an agent can also change the architecture silently by adding a library, service, or datastore to solve a local problem. These rules keep the agent useful and the humans accountable.

| ID | Requirement | Applicability | Verification and evidence |
|---|---|---|---|
| AAI-001 | Recommendations, scores, and stack proposals produced by AI agents are input to human decisions; ADRs MUST be accepted only by the human approval authority defined under AGOV-004. | Conditional — AI agents used | ADR approval fields name a human approver. |
| AAI-002 | When asked for an architecture or stack recommendation, an agent MUST present the alternatives required by AALT-002 and AALT-003 with their trade-offs, and MUST NOT present a single option as the only reasonable choice. | Conditional — AI agents used | Agent output reviewed against AALT rules. |
| AAI-003 | An agent MUST distinguish verified facts (with their source) from assumptions and from recalled knowledge; versions, support dates, licenses, prices, and service limits MUST be verified from current primary sources or explicitly marked as unverified. | Conditional — AI agents used | Source links and "unverified" labels in agent output. |
| AAI-004 | An agent MUST NOT fill in missing requirements, NFR targets, or constraints with invented values; it MUST ask for them or record them as assumptions requiring approval (ADRV-007). | Conditional — AI agents used | Assumptions log entries attributed to the agent. |
| AAI-005 | Agent-produced scoring MUST show each criterion's score, the evidence or assumption behind it, and the calculation, so that a human can check it. | Conditional — AI agents used | Scoring sheet produced by the agent. |
| AAI-006 | During implementation, an agent MUST NOT introduce a new framework, service, datastore, messaging technology, external integration, or significant library, and MUST NOT change module boundaries or the deployment model, beyond what accepted ADRs allow; it MUST stop and propose an ADR instead. | Conditional — AI agents used for implementation | Dependency diffs compared with ADRs; AFIT-004 results. |
| AAI-007 | Before design or implementation tasks, an agent MUST read the ADR index and the relevant accepted ADRs and architecture description sections; the project's agent instruction files MUST point to them. | Conditional — AI agents used | Agent instruction files; session logs. |
| AAI-008 | For Tier 1–2 applications, architecture descriptions and diagrams produced or updated by agents MUST be reviewed by a human against the actual code before each release. | Conditional — Tier 1–2 with AI agents | Review record per release. |
| AAI-009 | Agent instruction files derived from this standard MUST be version-controlled, MUST record the version of this standard they derive from, and this standard prevails where they differ. | Conditional — AI agents used | Instruction files with version references. |

---

## J. Architecture Conformance and Fitness Functions (AFIT)

**Intent and rationale.** An architecture that exists only in documents decays, especially when much of the code is written by agents. Automated checks that fail the build when the architecture is violated ("fitness functions") are what keep a modular design modular.

| ID | Requirement | Applicability | Verification and evidence |
|---|---|---|---|
| AFIT-001 | Architectural constraints that can be checked automatically (module dependency direction, layering rules, forbidden imports, cross-module data access) MUST be enforced by automated checks in CI for Tier 1–3 applications. | Conditional — Tier 1–3 | CI configuration; check results. |
| AFIT-002 | Fitness functions for key quality targets (for example, response-time budgets, start-up time, package or bundle size, interface compatibility) SHOULD run in CI or on a schedule, using targets from the NFR register. | Conditional — Tier 1–2 | Fitness function results. |
| AFIT-003 | A failed architecture conformance check MUST block merge unless an exception is approved (AREV-006). | Conditional — AFIT-001 checks exist | Branch protection settings; exception records. |
| AFIT-004 | CI SHOULD detect new third-party dependencies and new infrastructure resources that are not covered by the accepted stack or ADRs, and route them for architecture review. | Universal | Dependency check configuration and results. |

---

## K. Review, Approval, and Reassessment (AREV)

**Intent and rationale.** Review before implementation catches decisions that are poorly justified while they are still cheap to change. Reassessment keeps decisions valid as conditions change.

| ID | Requirement | Applicability | Verification and evidence |
|---|---|---|---|
| AREV-001 | Before implementation begins, the architecture MUST be reviewed against the checklist in Annex C, and the review outcome and any conditions MUST be recorded. | Universal | Completed checklist with reviewer and date. |
| AREV-002 | For Tier 1–2 applications, the review MUST include security and operations representation and at least one reviewer independent of the proposing team. | Conditional — Tier 1–2 | Reviewer list. |
| AREV-003 | The architecture MUST be reassessed when any of the following occurs: a material change in business requirements, workload, security risk, compliance obligations, team, or cost; a stack component approaching end of support; repeated incidents attributed to the architecture; or a reconsideration trigger in an ADR. | Universal | Reassessment records with trigger. |
| AREV-004 | Tier 1–2 architectures MUST be reviewed at least annually even without a trigger. | Conditional — Tier 1–2 | Annual review record. |
| AREV-005 | Conditions attached to an architecture approval MUST be tracked to closure. | Universal | Condition tracking records. |
| AREV-006 | Deviations from a MUST rule in this standard MUST be approved through the organization's exception process (for example, Section X of the testing standard), with a reason, risk assessment, compensating controls, and expiry date. | Universal | Exception register entries. |

---

## Annex A. ADR Template

File name: `docs/adr/NNNN-short-title.md`

```markdown
# ADR-NNNN: <Short title of the decision>

| Field | Value |
|---|---|
| Status | Proposed / Accepted / Rejected / Superseded by ADR-XXXX / Deprecated |
| Date | YYYY-MM-DD |
| Risk tier | Tier 1 / 2 / 3 / 4 |
| Deciders | <names and roles> |
| Approver | <human approver per AGOV-004> |
| Supersedes | ADR-XXXX (if any) |
| Standard version | ARCHITECTURE_DECISION_FRAMEWORK.md vX.Y |

## Context
What requirement, problem, or constraint requires a decision? Link the drivers
(requirements, NFR targets, gates, constraints, assumptions).

## Options considered
Each viable option, described at comparable depth. Options eliminated by a
mandatory gate, with the gate named.

## Decision
The option selected, stated in one or two sentences.

## Rationale
Scores and evidence (link to the scoring sheet), sensitivity check result,
and qualitative reasoning. Link each source of added complexity to a requirement.

## Consequences
Benefits, limitations, costs, risks, and what becomes easier or harder.

## Validation
Spikes, prototypes, benchmarks, or assessments performed or required, with results.

## Reconsideration triggers
Conditions that would require this decision to be revisited.

## Approval
Approver, role, date, and any conditions.
```

---

## Annex B. Scoring Sheet Template and Worked Example

### B.1 Template

| Criterion | Weight | Option A score | Evidence or assumption (A) | Option B score | Evidence or assumption (B) |
|---|---|---|---|---|---|
| *criterion* | *w* | *1–5* | *source* | *1–5* | *source* |
| **Weighted score** | **100** | | | | |
| **Gates passed?** | | Yes / No | | Yes / No | |

### B.2 Worked example (illustrative only)

Scenario: an internal case-management application, Tier 3, for about 300 users, with two external integrations. Both options passed all mandatory gates.

| Criterion | Weight | A: Modular monolith | B: Microservices |
|---|---|---|---|
| Security and compliance | 15 | 4 | 4 |
| Maintainability and testability | 15 | 4 | 3 |
| Team capability and operability | 15 | 4 | 2 |
| Reliability and availability | 10 | 4 | 4 |
| Performance | 10 | 4 | 4 |
| Scalability | 10 | 3 | 5 |
| Total cost of ownership | 10 | 4 | 2 |
| Integration flexibility | 10 | 4 | 4 |
| Portability and exit | 5 | 4 | 3 |
| **Weighted score** | **100** | **72.5** | **60.0** |

Calculation for option A: 15×0.75 + 15×0.75 + 15×0.75 + 10×0.75 + 10×0.75 + 10×0.50 + 10×0.75 + 10×0.75 + 5×0.75 = 72.5.

Sensitivity: B's only advantage is scalability. Doubling the scalability weight (moving 10 points from total cost of ownership) gives A 70.0 and B 67.5, so A stays ahead but its lead narrows to 2.5 points. The base margin (12.5) is above the illustrative 10-point threshold and no plausible single-weight change flips the winner, so this is not a close call under AEVL-004. The ADR should still state that A's advantage rests mainly on team capability and cost, and should include an evolution trigger (AALT-006) naming the workload level at which extracting a module into a service would be reconsidered.

---

## Annex C. Architecture Review Checklist

Reviewers confirm each item, or record why it does not apply.

1. Risk tier assigned and recorded (AGOV-002).
2. Business context, scope, and out-of-scope documented (ADRV-001).
3. NFR targets measurable and recorded in the NFR register (ADRV-002).
4. Mandatory gates identified with sources (ADRV-003).
5. Constraints, data profile, integration inventory, and assumptions documented (ADRV-004 to ADRV-007).
6. TCO estimate prepared over a stated horizon (ADRV-009).
7. Build, buy, or configure decision recorded (AALT-001).
8. Required alternatives evaluated, including the simplest viable option (AALT-002, AALT-003).
9. Weights approved before scoring; scores anchored and evidenced; sensitivity checked (AEVL-001 to AEVL-004).
10. High-risk assumptions validated (AEVL-007).
11. Pattern, deployment model, and integration style each decided (APAT-001); microservices preconditions met if applicable (APAT-003).
12. Stack components verified for version, support, and license from primary sources (ASTK-001 to ASTK-004).
13. Team capability and support plan recorded (ASTK-005).
14. Architecture description complete for the tier, stored as text in the repository (ADSN-001, ADSN-003).
15. Threat model completed where required (ADSN-004).
16. Testability and observability described (ADSN-005).
17. ADRs recorded, indexed, and approved by a human authority (AADR-001 to AADR-006, AAI-001).
18. Automated conformance checks defined (AFIT-001).
19. Agent instruction files point to the ADR index and architecture description (AAI-007).

---

## Annex D. Glossary

| Term | Definition in this standard |
|---|---|
| Architecture description | The documented set of views, decisions, and rationale describing an application's architecture. |
| Architecture Decision Record (ADR) | A short document recording one significant decision, its context, options, rationale, and consequences. |
| Fitness function | An automated check that measures whether the system still meets an architectural constraint or quality target. |
| Mandatory gate | A pass/fail requirement that every alternative must satisfy before it is scored. |
| Modular monolith | A single deployable application divided into modules with explicit, enforced boundaries. |
| NFR register | The project's list of non-functional requirements with measurable targets, owners, and measurement methods. |
| Risk tier | The criticality classification shared across the organization's standards (Tier 1 = Critical to Tier 4 = Low). |
| System of record | The single authoritative source for a given data entity. |
| Total cost of ownership (TCO) | All costs of building, running, supporting, and eventually exiting a solution over a stated horizon. |

---

## Annex E. References

This standard is informed by the sources below. It does not reproduce them or claim conformance with them. Editions are those known at the issue date and MUST be re-verified at each review of this standard.

| Reference | Edition | Relevance |
|---|---|---|
| ISO/IEC/IEEE 42010, Software, systems and enterprise — Architecture description | 2022 | Concepts for describing architecture: stakeholders, concerns, views, rationale. |
| ISO/IEC 25010, Systems and software Quality Requirements and Evaluation (SQuaRE) — Product quality model | 2023 | Quality characteristics checklist (ADRV-008). |
| The TOGAF Standard (The Open Group) | 10th Edition | Aligning architecture with business goals and governance; mainly relevant at enterprise level. |
| NIST SP 800-218, Secure Software Development Framework (SSDF) | Version 1.1; Rev. 1 in draft at issue | Secure development practices that architecture must support. |
| C4 model for visualizing software architecture | Current published model | Context, Container, Component, and Code diagrams (ADSN-002). |
| Architecture Tradeoff Analysis Method (ATAM), Software Engineering Institute, Carnegie Mellon University | Published method | Scenario-based evaluation of quality-attribute trade-offs (AEVL-006). |
| Architecture Decision Records (format popularized by Michael Nygard) | Community practice | ADR structure (Annex A). |
| `testing-rules.md` | v1.2 | Verification of the decisions made under this standard; shared risk-tier model. |
| `security-privacy-policy.md` | Current | Data classes, ASVS levels, threat-model scope, and security gates used in Sections B and G. |

---

## Annex F. Rule Prefix Index

| Prefix | Section | Topic | Rules | Range |
|---|---|---|---|---|
| AGOV | A | Principles and governance | 7 | AGOV-001 to AGOV-007 |
| ADRV | B | Decision drivers | 9 | ADRV-001 to ADRV-009 |
| AALT | C | Architecture alternatives | 6 | AALT-001 to AALT-006 |
| AEVL | D | Objective evaluation | 7 | AEVL-001 to AEVL-007 |
| APAT | E | Pattern, deployment model, integration style | 7 | APAT-001 to APAT-007 |
| ASTK | F | Technology stack selection | 10 | ASTK-001 to ASTK-010 |
| ADSN | G | Architecture description | 8 | ADSN-001 to ADSN-008 |
| AADR | H | Architecture decision records | 6 | AADR-001 to AADR-006 |
| AAI | I | AI-assisted architecture work | 9 | AAI-001 to AAI-009 |
| AFIT | J | Conformance and fitness functions | 4 | AFIT-001 to AFIT-004 |
| AREV | K | Review, approval, and reassessment | 6 | AREV-001 to AREV-006 |
| **Total** | | **11 prefixes** | **79** | |

---

## Revision History

| Version | Date | Summary | Approver |
|---|---|---|---|
| 1.1 | 2026-10-23 | Renamed to `architecture-decision-framework.md` and aligned with the eight-document standard set: set-wide conflict order, tags, data classes and phase use in Section 0.5; ADRV-003, ADRV-005 and ADSN-004 cross-referenced to the Security & Privacy Policy (threat model now also required wherever SP-23.5 requires it); status set to Proposed. | *To be recorded by the adopting organization* |
| 1.0 | 2026-10-23 | First versioned issue. Converted from the unversioned Word draft "Application Architecture Decision Framework" into rule format with identifiers and normative language. Added: build, buy, or configure decision; separation of pattern, deployment model, and integration style; technology stack verification rules; anchored scoring scale, 0–100 formula, and sensitivity check; team capability and portability criteria; tiered architecture description requirements; ADR lifecycle and storage; AI-assisted architecture rules; fitness functions; review checklist; references with editions; cross-references to the testing standard. | *To be recorded by the adopting organization* |

*End of document.*
