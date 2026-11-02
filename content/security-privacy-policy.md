# Security and Privacy Policy

Version: 1.0 | Updated: 2026-10-23 | Owner: <to be assigned> | Status: Proposed — change to Approved on sign-off (agents treat unapproved documents as guidance only, SDLC Rules G-12)

Applies to: all products, environments, employees, contractors and AI agents, on every project and stack.
MUST = mandatory. SHOULD = default, deviate only with a stated reason. MAY = optional. MUST NOT = prohibited.
Tags: [H] = humans, [A] = AI agents, [B] = both.
Conflict order: law and regulation → client contracts → Security & Privacy Policy → AI Usage Policy → SDLC Rules → Architecture Decision Framework / Code Review Rules / Release Rules / IT Standard Rules / Testing Rules → approved project ADRs and project rules → stack-specific rules → tool defaults. A higher document wins when two documents conflict. A lower document MAY add stricter requirements; that is not a conflict. Within the same level, the stricter rule applies. Report conflicts; never choose silently. A project deviation needs a recorded exception (owner, reason, expiry). When unsure, choose the more protective option and ask.

Related documents (not repeated here):
- ai-usage-policy.md — sharing data with AI development tools; what agents may do
- sdlc-rules.md — phases, gates, approvals
- code-review-rules.md — review process and finding severity
- release-rules.md — release, deploy, rollback
- it-standard-rules.md — coding-level practices (IT Standard Rules §4 Security, §9 Logging) — this policy states WHAT must be achieved; the IT Standard Rules state HOW in code
- testing-rules.md — test standards (Testing Rules); Section J verifies the security controls in this policy and Section U the AI features in §21
- architecture-decision-framework.md — architecture options, scoring, ADRs and technology stack selection

Legal note: legal references (DPDP Act 2023 and DPDP Rules 2025, CERT-In Directions, GDPR, PCI DSS) are summaries for engineering use, not legal advice. Legal MUST confirm obligations per product. Most DPDP Rules obligations apply from 13 May 2027; products built now will be live then, so treat them as in force.

## Values to set

| Value | Default (editable) | Used in |
|---|---|---|
| Security contact / channel | <name, email, chat channel> | SP-0.5, §26 |
| Privacy contact / DPO / grievance officer | <name, email> | SP-28.6 |
| Access review frequency | Quarterly | SP-7.12 |
| Admin / Restricted-data session idle timeout | 15 minutes | SP-7.8 |
| Standard session idle timeout | 30 minutes | SP-7.8 |
| Absolute session lifetime | 12 hours | SP-7.8 |
| Secret rotation | Every 90 days, and on exposure or staff exit | SP-10.6 |
| Fix deadline: Critical / High / Medium / Low | 7 / 30 / 90 / 180 days | SP-24.3 |
| Security log retention | At least 1 year | SP-17.4 |
| Backup restore test | Every 6 months | SP-25.3 |
| Incident tabletop exercise | Yearly | SP-27.1 |
| Penetration test (Restricted data or public exposure) | Before first release, then yearly and after major changes | SP-27.3 |
| Data subject request response time | <per applicable law; confirm with legal> | SP-3.3 |
| Internal incident report time | Immediately, at most 1 hour after discovery | SP-26.1 |

## When to use this file

| Phase | Sections |
|---|---|
| 0 Readiness | 0, 1, 31 |
| 1 Requirements | 0, 1–6, 19–22 (if applicable), 23.1, 28 |
| 2 Design | 0, 7–22, 23.2, 29 |
| 3 Plan | 23.3, 24, 27 |
| 4 Configure | 10, 16, 23.4 |
| 5 Build & Verify | 0, 7–22, 23.5, 24 |
| 6 Release | 23.6, 25, 27 |
| 7 Operate & Learn | 17, 24–27, 30 |

## 0. Always (agent quick rules)

- SP-0.1 [A] MUST treat unclassified data as Restricted.
- SP-0.2 [A] MUST NOT use real personal data or secrets in development, tests, prompts, logs, examples or documentation.
- SP-0.3 [A] MUST flag any requirement, design or change that collects, stores, shares or exposes personal or Restricted data.
- SP-0.4 [A] MUST NOT weaken or disable a security control (authentication, authorisation, validation, encryption, TLS verification, logging, permissions, security tests), even temporarily. Stop and ask.
- SP-0.5 [A] MUST report suspected vulnerabilities or data exposure to the human immediately.
- SP-0.6 [A] When two options meet the requirements equally, MUST recommend the more privacy-protective one.
- SP-0.7 [A] MUST NOT suggest storing data "just in case"; ask for the purpose.
- SP-0.8 [A] MUST include a "Security & privacy impact" section in every spec, design and completion report (see Appendix A).
- SP-0.9 [A] MUST apply the OWASP ASVS level required by the data class (SP-1.4) when designing, building or reviewing.

---

# Part A — Foundations

## 1. Data classification

This is the single source of data classes. All other documents use these names.

| Class | Examples | Minimum protection | OWASP ASVS level |
|---|---|---|---|
| Public | Marketing pages, public docs | Integrity only | Level 1 |
| Internal | Internal docs, non-sensitive code | Company access only | Level 1 |
| Confidential | Client code, contracts, pricing, business plans | Need-to-know, encrypted in transit and at rest | Level 2 |
| Restricted | Personal data, financial, health, credentials, keys, national IDs, production data | Strict need-to-know, encryption, audit logging, masking, no use in tests | Level 2 + Level 3 controls for authentication, access control, cryptography and logging |

- SP-1.1 [B] Every data item a product stores or processes MUST be classified in Phase 1.
- SP-1.2 [B] A dataset takes the class of its most sensitive field.
- SP-1.3 [H] Data owners MUST be named for Confidential and Restricted data.
- SP-1.4 [B] The product's security level MUST follow the highest data class it handles, using the OWASP ASVS level in the table (current ASVS version).
- SP-1.5 [B] Payment card data MUST also follow §19 and PCI DSS scope rules.
- SP-1.6 [B] Whether data of each class may be sent to AI development tools (coding assistants and agents) is governed by AI Usage Policy §3 (single source; Restricted data is never allowed). AI features inside the product follow §21.

## 2. Privacy principles (privacy by design)

- SP-2.1 [B] Purpose: MUST state why each personal data item is collected. No collection "for later use".
- SP-2.2 [B] Minimisation: MUST collect, store, show and share only what the stated purpose needs.
- SP-2.3 [B] Lawful basis and consent: MUST record the legal basis for processing. Consent MUST be clear, specific, informed, recorded, and as easy to withdraw as to give.
- SP-2.4 [B] Notice: users MUST be told in plain language what is collected, why, how long it is kept, who receives it, and how to exercise their rights.
- SP-2.5 [B] Accuracy: SHOULD let users view and correct their data.
- SP-2.6 [B] Privacy-friendly defaults: optional data sharing, tracking and marketing MUST be off by default.
- SP-2.7 [B] Purpose change: using data for a new purpose MUST be treated as a new requirement with fresh legal review and, where needed, fresh consent.

## 3. Data subject rights

- SP-3.1 [B] Products holding personal data MUST support, or have a documented process for: access, correction, erasure, consent withdrawal, grievance handling and nomination, as required by applicable law.
- SP-3.2 [B] Erasure MUST cover all copies the product controls (database, files, caches, search indexes, analytics, third-party processors, and backups per retention schedule).
- SP-3.3 [H] Requests MUST be answered within the time limit in Values to set.
- SP-3.4 [B] Identity MUST be verified before acting on a rights request.

## 4. Retention and deletion

- SP-4.1 [B] Every personal and Restricted data item MUST have a retention period and a deletion method.
- SP-4.2 [B] Data MUST be deleted or anonymised when its purpose ends or the retention period expires, unless a law requires longer retention.
- SP-4.3 [B] Deletion SHOULD be automated, not manual.
- SP-4.4 [H] Legal holds override deletion and MUST be documented.
- SP-4.5 [B] Minimum retention required by law (for example 1 year for certain logs under DPDP Rules 2025) MUST be designed in; confirm with legal.

## 5. Data residency and transfers

- SP-5.1 [B] Data location requirements MUST be recorded as constraints in Phase 1 (for example: client data stored in India).
- SP-5.2 [B] Hosting regions, backups, logs, analytics, AI providers and third-party processors MUST all meet the residency requirement.
- SP-5.3 [H] Cross-border transfers MUST be approved by legal.

## 6. Regulated and special data

- SP-6.1 [B] Aadhaar numbers, PAN, passport, bank account and other national or financial identifiers MUST be flagged in Phase 1 for legal review; they have specific storage and masking rules (for example Aadhaar storage restrictions). Collect them only when a law or contract requires it.
- SP-6.2 [B] Display of national identifiers MUST be masked by default (for example show only the last 4 digits).
- SP-6.3 [B] Health, biometric, sexual orientation, religious, caste, and similar sensitive data MUST be treated as Restricted, with a privacy impact assessment (SP-23.6).
- SP-6.4 [B] Children's data MUST follow applicable law (for example verifiable parental consent under DPDP Act). Flag any product that may be used by minors.
- SP-6.5 [B] Data about persons with disabilities acting through a lawful guardian MUST follow the applicable consent rules.

---

# Part B — Technical controls

## 7. Identity and access control

- SP-7.1 [B] Deny by default: every endpoint, screen, file and record starts with no access; access is granted explicitly per role.
- SP-7.2 [B] Least privilege MUST apply to users, admins, services, agents and database accounts.
- SP-7.3 [B] Authorisation MUST be enforced on the server for every protected operation, including object-level checks (a user can access only records they are allowed to).
- SP-7.4 [B] Roles and permissions MUST come from the roles and permissions matrix approved in Phase 1.
- SP-7.5 [B] Company staff and all admin access (including product admin consoles) MUST use SSO and multi-factor authentication.
- SP-7.6 [B] End-user authentication MUST use proven frameworks or identity providers. Step-up authentication SHOULD protect high-risk actions (payments, data export, profile or password changes).
- SP-7.7 [B] Passwords, if stored, MUST use Argon2id, scrypt or bcrypt with settings per current OWASP guidance.
- SP-7.8 [B] Sessions and tokens MUST have idle and absolute timeouts (Values to set), be revocable, and be invalidated on logout, password change and role change.
- SP-7.9 [B] Login, password reset and OTP endpoints MUST have rate limiting and lockout or step-up protection against brute force.
- SP-7.10 [B] Service-to-service calls MUST be authenticated (for example managed identities, mutual TLS, signed tokens), not trusted by network location alone.
- SP-7.11 [B] Shared accounts MUST NOT be used; every action MUST be traceable to a person or service identity.
- SP-7.12 [H] Access MUST be reviewed at the frequency in Values to set and removed on role change or exit.
- SP-7.13 [H] Emergency (break-glass) access MUST be time-limited, logged and reviewed after use.
- SP-7.14 [B] Role and permission changes MUST go through the product's approved process, never directly in the database.

## 8. Tenant and client data isolation

Applies when one deployment serves more than one client, organisation or tenant.

- SP-8.1 [B] The isolation model (separate database, separate schema, or shared tables with tenant ID) MUST be chosen in Design and recorded in an ADR.
- SP-8.2 [B] Every query, cache key, file path, search index and message MUST be scoped to the tenant.
- SP-8.3 [B] Tenant scoping MUST be enforced centrally (data access layer, row-level security or equivalent), not left to each developer.
- SP-8.4 [B] Automated tests MUST prove that one tenant cannot read or change another tenant's data.
- SP-8.5 [B] Admin and support tools that cross tenants MUST be separately authorised and logged.
- SP-8.6 [B] Tenant data MUST be exportable and deletable per tenant at contract exit.

## 9. Business audit trail

- SP-9.1 [B] Changes to Confidential and Restricted business records MUST be audited: who, when, what changed (old and new values or a reference), and from where.
- SP-9.2 [B] Reads of Restricted data SHOULD be audited where the risk or law requires it.
- SP-9.3 [B] Audit records MUST be append-only and protected from change by application users and admins.
- SP-9.4 [B] Audit records MUST NOT store secrets; Restricted values in audit records MUST be masked or referenced.
- SP-9.5 [B] Audit retention MUST be defined with the data owner and legal.
- Note: low-code platforms such as Appian provide record history automatically; on a code stack it MUST be designed and built.

## 10. Encryption and secrets

- SP-10.1 [B] Data in transit MUST use TLS 1.2 or higher (TLS 1.3 preferred) for all external and internal traffic.
- SP-10.2 [B] Confidential and Restricted data MUST be encrypted at rest. Highly sensitive fields (national IDs, health, bank details) SHOULD also use field-level encryption or tokenisation.
- SP-10.3 [B] MUST use well-known, maintained cryptographic libraries and current algorithms; MUST NOT write custom cryptography.
- SP-10.4 [B] Keys and secrets MUST be stored in an approved key or secrets manager.
- SP-10.5 [B] Secrets MUST NOT appear in code, config files, container images, tickets, chat, prompts or logs.
- SP-10.6 [B] Secrets MUST be rotated per Values to set; any exposed secret MUST be treated as compromised and rotated immediately.
- SP-10.7 [B] Restricted fields SHOULD be masked in UIs, exports and support tools by default.
- SP-10.8 [B] TLS certificates MUST be monitored for expiry and renewed automatically where possible.
- SP-10.9 [B] SHOULD use pseudonymisation (tokens instead of real identifiers) where systems do not need the real value.
- SP-10.10 [B] Anonymised data MUST NOT be re-identifiable by combining it with other datasets; if it can be, treat it as personal data.

## 11. API security

- SP-11.1 [B] Every API MUST be listed in an API inventory with owner, purpose, data class and consumers.
- SP-11.2 [B] Every non-public API MUST require authentication; no hidden unauthenticated endpoints.
- SP-11.3 [B] APIs that can be abused (public, authentication, search, export, write operations) MUST apply rate limiting; all APIs MUST apply request size limits. (IT Standard Rules CAPI-003, CAPI-004 describes how.)
- SP-11.4 [B] CORS MUST allow only known origins; wildcard origins MUST NOT be used with credentials.
- SP-11.5 [B] API keys MUST be scoped, rotatable and never placed in client-side code or URLs.
- SP-11.6 [B] Debug, test and admin endpoints MUST be disabled or separately protected in production.
- SP-11.7 [B] Responses MUST NOT return more fields than the caller needs (IT Standard Rules CAPI-006).
- SP-11.8 [B] Old API versions MUST be retired on a schedule, not left running.

## 12. File handling and uploads

- SP-12.1 [B] Uploads MUST be checked for type (by content, not only extension), size and count.
- SP-12.2 [B] Uploaded files MUST be stored outside the web root with generated names, never with user-supplied paths.
- SP-12.3 [B] Uploads from untrusted users SHOULD be malware-scanned before use or sharing.
- SP-12.4 [B] Storage buckets MUST be private by default; access MUST use short-lived signed links.
- SP-12.5 [B] Downloads and bulk exports of Confidential or Restricted data MUST be authorised and logged.
- SP-12.6 [B] Document metadata (author, location, device) SHOULD be stripped where not needed.

## 13. Web and mobile clients

- SP-13.1 [B] Secrets MUST NOT be embedded in web front-ends or mobile apps; anything shipped to a client is public.
- SP-13.2 [B] Security decisions MUST NOT rely on client-side checks; hiding a button is not access control.
- SP-13.3 [B] Mobile apps MUST store tokens and sensitive data only in the platform's secure storage.
- SP-13.4 [B] Sensitive data MUST NOT be stored in browser local storage, URLs, or client logs.
- SP-13.5 [B] Web apps MUST set security headers (Content-Security-Policy, HSTS, X-Content-Type-Options, frame protection, Referrer-Policy).
- SP-13.6 [B] Session cookies MUST be Secure, HttpOnly and use an appropriate SameSite setting; cookie-based sessions MUST have CSRF protection.
- SP-13.7 [B] App permissions (camera, location, contacts) MUST be requested only when needed and explained to the user.

## 14. Network

- SP-14.1 [B] Networks MUST be segmented: public entry points, application tier and data tier separated.
- SP-14.2 [B] Databases, caches and queues MUST NOT be reachable from the public internet.
- SP-14.3 [B] SHOULD use private endpoints for cloud services that carry Confidential or Restricted data.
- SP-14.4 [B] Outbound (egress) traffic from servers SHOULD be limited to required destinations, to reduce data leaks and server-side request forgery.
- SP-14.5 [B] Only required ports and services may be exposed; default credentials MUST be removed.
- SP-14.6 [B] Admin access to servers MUST go through a controlled path (bastion, managed access service), never open SSH/RDP to the internet.

## 15. Cloud and containers

- SP-15.1 [B] Each environment (dev, test, staging, production) MUST use a separate cloud account, subscription or project.
- SP-15.2 [B] Cloud IAM MUST follow least privilege; no wildcard permissions in production.
- SP-15.3 [B] Storage, databases and queues MUST NOT be publicly accessible unless a requirement says so and it is approved.
- SP-15.4 [B] Infrastructure MUST be defined as code and reviewed like code; manual changes in production MUST NOT be made outside the approved process.
- SP-15.5 [B] Cloud configuration SHOULD be scanned continuously against a secure baseline (for example CIS Benchmarks).
- SP-15.6 [B] Container images MUST come from trusted base images, be scanned, kept minimal, and run as non-root.
- SP-15.7 [B] Systems and images MUST be patched within the fix deadlines in Values to set.

## 16. Software supply chain

- SP-16.1 [B] Dependencies MUST be pinned through lockfiles and installed from trusted registries.
- SP-16.2 [B] New dependencies MUST be checked for maintenance, licence, known vulnerabilities, and typosquatting before adoption (IT Standard Rules CDEP-001, CDEP-002).
- SP-16.3 [B] SHOULD produce a software bill of materials (SBOM) for each release.
- SP-16.4 [B] Build pipelines MUST be protected: limited access, reviewed changes, no secrets in build logs, secrets injected at run time.
- SP-16.5 [B] Release artifacts SHOULD be signed and verified before deployment.
- SP-16.6 [B] The main branch MUST be protected: required review, required CI, no direct pushes, no force-push.

## 17. Logging and monitoring

- SP-17.1 [B] MUST log security events: login success and failure, authorisation failures, admin actions, permission changes, data exports, configuration changes, and security control failures.
- SP-17.2 [B] Logs MUST NOT contain passwords, tokens, keys, session IDs, full card data or unmasked Restricted data (IT Standard Rules CLOG-001).
- SP-17.3 [B] Logs MUST be centralised, protected from tampering, and access-controlled.
- SP-17.4 [B] Security logs MUST be kept for the retention in Values to set, then deleted unless a law requires longer.
- SP-17.5 [B] Alerts MUST exist for suspicious activity: repeated failed logins, unusual exports, privilege changes, access from unusual locations.
- SP-17.6 [B] System clocks MUST be synchronised so logs can be correlated.

## 18. Notifications (email, SMS, push)

- SP-18.1 [B] Notifications MUST NOT contain passwords, full national IDs, full account numbers or other Restricted data.
- SP-18.2 [B] Links in notifications MUST NOT carry long-lived tokens; one-time links MUST expire quickly and work once.
- SP-18.3 [B] Sending domains MUST have SPF, DKIM and DMARC configured.
- SP-18.4 [B] Marketing messages MUST respect consent and opt-out, and follow applicable telecom and messaging rules.

---

# Part C — Specialised areas (apply when relevant)

## 19. Payments and financial data

- SP-19.1 [B] MUST NOT store full card numbers, CVV or PINs. Use a PCI-compliant payment provider and tokens to keep the product out of PCI scope where possible.
- SP-19.2 [B] Amounts, prices, taxes and discounts MUST be calculated and verified on the server.
- SP-19.3 [B] Payment and refund operations MUST be idempotent and fully audited (§9).
- SP-19.4 [B] Payment webhooks and callbacks MUST be verified (signature check) before being trusted.
- SP-19.5 [B] Financial records MUST follow statutory retention rules; confirm with legal and finance.

## 20. Cookies, tracking and analytics

- SP-20.1 [B] Non-essential cookies and trackers MUST NOT load before consent where the law requires consent.
- SP-20.2 [B] Analytics MUST NOT receive personal data, Restricted data or URLs containing identifiers.
- SP-20.3 [B] Every third-party script MUST be approved and listed; unknown scripts MUST NOT be added.
- SP-20.4 [B] SHOULD prefer privacy-friendly analytics with IP anonymisation and short retention.

## 21. AI and LLM features inside the product

Applies when the product itself calls an AI model. AI development tools are covered by the AI Usage Policy.

- SP-21.1 [B] Design MUST address the current OWASP Top 10 for LLM Applications.
- SP-21.2 [B] AI providers MUST pass vendor review (§29), including training on customer data, retention and residency terms.
- SP-21.3 [B] MUST send the minimum data to the model; mask or remove Restricted data where the purpose allows.
- SP-21.4 [B] User input and retrieved content (documents, web pages, emails) MUST be treated as untrusted; design against prompt injection.
- SP-21.5 [B] Model output MUST NOT be executed, run as queries or commands, or rendered as HTML without validation and encoding.
- SP-21.6 [B] AI features MUST NOT get more data access or tool permissions than the user they act for.
- SP-21.7 [B] Actions with external effect (sending, paying, deleting, changing records) MUST require user confirmation.
- SP-21.8 [B] Output guardrails SHOULD filter disallowed content and sensitive data leaks; usage limits SHOULD prevent cost abuse.
- SP-21.9 [B] Users MUST be told when they are interacting with AI or receiving AI-generated content, where required.
- SP-21.10 [B] High-impact decisions (credit, hiring, medical, legal, eligibility) MUST have human review.
- SP-21.11 [B] Prompt and output logs MUST NOT store Restricted data unmasked, and follow the same retention rules as other personal data.

## 22. Abuse, fraud and bot protection

- SP-22.1 [B] Abuse cases (fake accounts, scraping, spam, enumeration, coupon and referral abuse) MUST be listed in Phase 1.
- SP-22.2 [B] Public forms and sign-up flows SHOULD have bot protection and rate limiting.
- SP-22.3 [B] Error messages and timings MUST NOT reveal whether an account, email or record exists.
- SP-22.4 [B] SHOULD monitor for unusual patterns (mass downloads, rapid account creation, repeated failed payments).
- SP-22.5 [B] Public-facing services SHOULD have DDoS protection or a web application firewall where the risk justifies it.

---

# Part D — Lifecycle

## 23. Security and privacy by SDLC phase

### 23.1 Requirements
- SP-23.1 [B] MUST identify data classes, regulated data (§6), compliance obligations and residency.
- SP-23.2 [B] MUST write security and privacy requirements (authentication, roles, audit, retention, consent, tenant isolation) as testable requirements with IDs.
- SP-23.3 [B] MUST list abuse cases next to user journeys.
- SP-23.4 [B] MUST record the required ASVS level (SP-1.4).

### 23.2 Design
- SP-23.5 [B] MUST produce a threat model when the product handles authentication, money, personal or Restricted data, or new external access. It MUST list assets, actors, trust boundaries, data flows, threats, controls and residual risks.
- SP-23.6 [H] MUST run a privacy impact assessment for high-risk processing (large scale, sensitive data, children, profiling, monitoring), and where the law requires one.
- SP-23.7 [B] Security and privacy requirements MUST be pass/fail gates in architecture selection.
- SP-23.8 [B] Every feature spec MUST include the security and privacy section in Appendix A.

### 23.3 Plan
- SP-23.9 [B] MUST plan security tests: authorisation (including object-level and tenant isolation), input validation, abuse cases, secret scanning, dependency scanning.
- SP-23.10 [B] Every threat-model control MUST trace to a task and a test.

### 23.4 Configure
- SP-23.11 [B] Agent permissions MUST deny access to secret files and production (AI Usage Policy).
- SP-23.12 [B] CI MUST include secret scanning, dependency vulnerability scanning, and static analysis where available for the stack.

### 23.5 Build and Verify
- SP-23.13 [B] MUST follow IT Standard Rules §4 (Security) and §9 (Logging).
- SP-23.14 [H] Security-sensitive changes MUST be reviewed by a qualified security reviewer (Code Review Rules).
- SP-23.15 [B] Test and development environments MUST use synthetic or masked data only; production data MUST NOT be copied to lower environments unmasked.

### 23.6 Release
- SP-23.16 [H] Critical and High vulnerabilities MUST be fixed or formally accepted (§24) before release.
- SP-23.17 [H] Penetration testing MUST follow Values to set for products with Restricted data or public exposure.
- SP-23.18 [B] Security headers, TLS, logging, monitoring and alerts MUST be confirmed in the release checklist (Release Rules).

## 24. Vulnerability management

- SP-24.1 [B] Code, dependencies, container images and cloud configuration MUST be scanned continuously.
- SP-24.2 [B] Vulnerability severity MUST use CVSS (current version) adjusted for the product's exposure and data class.
- SP-24.3 [B] Vulnerabilities MUST be fixed within the deadlines in Values to set.
- SP-24.4 [H] Accepted risks MUST have an owner, reason, compensating control and expiry date.
- SP-24.5 [B] Every fixed vulnerability MUST get a security regression test.
- SP-24.6 [H] SHOULD publish a way for external parties to report vulnerabilities (for example security.txt).

## 25. Backup and recovery

- SP-25.1 [B] Backups MUST meet the product's recovery targets (RPO / RTO from the NFRs).
- SP-25.2 [B] Backups MUST be encrypted, access-controlled, stored per residency rules, and protected from deletion or ransomware (immutable or separate account).
- SP-25.3 [H] Restores MUST be tested at the frequency in Values to set.

## 26. Incidents and breach notification

- SP-26.1 [B] Suspected security incidents and data breaches MUST be reported internally to the security contact within the time in Values to set.
- SP-26.2 [H] Steps: contain → assess impact → preserve evidence → fix → notify → review.
- SP-26.3 [H] Regulatory notifications MUST meet legal deadlines, confirmed by legal for each incident. Known references:
  - CERT-In Directions (2022): report specified cyber incidents within 6 hours of noticing.
  - DPDP Rules 2025 (from 13 May 2027): inform the Data Protection Board without delay, with a detailed report within 72 hours; inform affected users without delay.
  - GDPR (if applicable): supervisory authority within 72 hours.
- SP-26.4 [H] Client notifications MUST follow contract terms.
- SP-26.5 [H] Every incident MUST end with a blameless review and updates to this policy, rules or tests.

## 27. Security testing and readiness drills

- SP-27.1 [H] Incident response SHOULD be practised with a tabletop exercise at the frequency in Values to set.
- SP-27.2 [H] Disaster recovery MUST be tested against RPO / RTO targets.
- SP-27.3 [H] Penetration tests MUST follow Values to set; findings follow §24.
- SP-27.4 [H] Security metrics SHOULD be tracked: open vulnerabilities by severity, time to fix, overdue fixes, failed access reviews, incidents.

---

# Part E — Governance

## 28. Compliance

- SP-28.1 [H] Each project MUST list applicable laws, standards and client requirements in Phase 1 (for example DPDP Act and Rules, CERT-In Directions, GDPR, PCI DSS, HIPAA, ISO 27001, SOC 2, accessibility law).
- SP-28.2 [H] Compliance obligations MUST become testable requirements, not separate paperwork.
- SP-28.3 [H] Each product MUST keep a record of processing: personal data, purpose, storage location, recipients, retention.
- SP-28.4 [H] Products that may make the organisation a Significant Data Fiduciary (large volume or sensitive data) MUST be flagged to legal for extra duties (audits, impact assessments).
- SP-28.5 [H] Consent manager integration MUST be assessed where DPDP Rules require it.
- SP-28.6 [H] A privacy contact / grievance officer MUST be published where the law requires it.

## 29. Third parties and vendors

- SP-29.1 [H] Vendors and SaaS that process Confidential or Restricted data MUST pass a security and privacy review before use.
- SP-29.2 [H] Contracts MUST cover data protection, breach notification, residency, sub-processors, audit rights and deletion at exit.
- SP-29.3 [B] Data sent to third parties MUST be the minimum needed.
- SP-29.4 [H] Vendor reviews MUST be repeated at least yearly and when terms change.

## 30. Exceptions and policy maintenance

- SP-30.1 [H] Exceptions MUST be approved by the security owner, with reason, compensating control, owner and expiry date.
- SP-30.2 [H] Expired exceptions MUST be closed or renewed; they MUST NOT stay open silently.
- SP-30.3 [H] This policy MUST be reviewed every 6 months, after major incidents, and when laws change.

## 31. Organisational controls (humans only; agents skip)

- SP-31.1 [H] Work devices MUST have disk encryption, screen lock, supported OS, automatic updates and endpoint protection.
- SP-31.2 [H] Source code and client data MUST NOT be stored on personal devices or personal cloud accounts.
- SP-31.3 [H] Phishing and suspicious messages MUST be reported, not just deleted.
- SP-31.4 [H] Access for joiners, movers and leavers MUST be granted and removed through a tracked process.
- SP-31.5 [H] Lost or stolen devices MUST be reported immediately.
- SP-31.6 [H] Everyone MUST complete security and privacy training before access and yearly.

---

## Appendix A — Security & privacy section for every spec

```text
Security & privacy impact
- Data touched: <fields>                      Class: Public / Internal / Confidential / Restricted
- Regulated data (§6): none / <list>          ASVS level: 1 / 2 / 2+3
- Purpose and legal basis:
- Who can do what (roles → actions, object-level rule):
- Tenant isolation impact: none / <how scoped>
- Abuse cases:
- Audit trail entries:
- Logging (what is logged, what is masked):
- Retention and deletion:
- External parties / AI providers receiving data:
- Threat model reference: <link or "not required because ...">
- Security tests planned:
- Open questions:
```

---

## Revision history

| Version | Date | Summary |
|---|---|---|
| 1.0 | 2026-10-23 | First versioned issue. Set-wide conflict order (adds the Architecture Decision Framework; lower documents may be stricter); IT Standard Rules references updated to v1.3 identifiers; Testing Rules now referenced as written; AI-development-tool permissions per data class moved to a single source (AI Usage Policy §3). DPDP Rules dates checked against published sources on 2026-10-23 (notified 13 November 2025; most obligations from 13 May 2027). |
