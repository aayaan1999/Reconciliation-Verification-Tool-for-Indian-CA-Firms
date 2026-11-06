// The shapes of the engine's JSON output (recon-engine run --json and facts --json) and of
// the sample data the screens use until they are connected to the engine. Money is always
// an integer number of paise; the screens never do arithmetic on it, only display it.

import type { DryRun } from './formatsTypes.ts'

export type Value =
  | { kind: 'money'; paise: number }
  | { kind: 'int'; int: number }
  | { kind: 'text'; text: string }
  | { kind: 'date'; date: string }
  | { kind: 'bool'; bool: boolean }
  | { kind: 'rate'; rate_bp: number }

export interface Locator {
  document_version_id?: number
  file_name: string
  kind: string
  path: string
}

export interface Subject {
  period: string
  scope: string
  match_key: string
}

export interface LabeledValue {
  input: string
  label: string
  value: Value
  locator: Locator
}

export interface LawRef {
  act: string
  section: string
  reference: string
}

export interface Finding {
  finding_id: string // the stable ID a decision is kept against (SPEC-006 part 3)
  value_fingerprint: string
  rule_id: string
  rule_version: string
  title: string
  category: string
  code: string // row matching: "ONLY_IN_A", "AMOUNT_DIFF" ...; "" for a comparison of two figures
  code_text: string // the code in words, such as "In Sales register only"
  subject: Subject
  values: LabeledValue[]
  difference_paise: number
  show_difference: boolean // false for "found together" and law-table findings (SPEC-006)
  law_refs: LawRef[]
  // Set only for a Partner, on a finding someone marked deleted (SPEC-013 §4). The other roles
  // are not sent the finding at all.
  deleted?: { by: string; at: string; reason: string }
}

export interface FindingGroup {
  review_document: string
  heading: string
  findings: Finding[]
}

export interface MissingFigure {
  fact: string
  label: string
  document_type: string
  document: string
  rows: boolean // a record type (rows), not a single figure
}

export interface CannotCheck {
  rule_id: string
  rule_version: string
  title: string
  category: string
  subject: Subject
  missing: MissingFigure[]
}

export interface Issue {
  code: string
  message: string
  path: string
}

export interface FileReport {
  file: string
  file_type: string
  document_type: string // a file that holds several documents (a bundle) has one report per document
  profile_id: string
  profile_version: string
  source_system: string
  tax_year: string
  client_match: string
  pan_last4: string
  fact_count: number
  record_count: number
  unread: Unread[]
  warnings: Issue[]
  errors: Issue[]
}

export interface Passed {
  rule_id: string
  rule_version: string
  title: string
  category: string
  review_document: string
  code: string
  subject: Subject
  // What the check compared and why it agreed. Absent in a comparison kept before these were
  // stored, which then shows only that the check passed.
  code_text?: string
  values?: LabeledValue[]
  difference_paise?: number
  show_difference?: boolean
  tolerance?: { kind: 'none' | 'paise' | 'bp'; value: number } // paise, or basis points of the larger figure
}

// One part of a file that was not read, and why: in a row the format leaves out, a column
// of a row that was read which no check uses yet, or another part of the file.
export interface Unread {
  path: string
  kind: 'row_left_out' | 'column_not_used' | 'not_read'
  row: string // the row the value is in; '' for another part of the file
  reason: string
}

export interface RunReport {
  tax_year: string
  includes_testing: boolean
  counts: { checks: number; pass: number; fail: number; cannot_check: number }
  files: FileReport[]
  findings: FindingGroup[]
  cannot_check: CannotCheck[]
  passed: Passed[]
  comparison: ReportComparison | null
  totals: DocTotal[]
  not_selected: { rule_id: string; title: string }[] // rules of the type the CA left out (SPEC-004 part 9)
}

export interface ReportComparison {
  id: string
  name: string
  description: string
  totals: boolean
  documents: { document_type: string; name: string; present: boolean }[]
}

export interface DocTotal {
  document_type: string
  document: string
  rows: number
  amounts: Record<string, number>
}

// A type of check the CA picks before uploading (content/comparisons/comparisons.yaml).
export interface Comparison {
  id: string
  name: string
  description: string
  documents: string[]
  rules: string[]
  totals: boolean
  worklist: boolean // this year's and last year's document of one type give the Worklist (SPEC-006 part 3)
  document_names: { document_type: string; name: string }[]
}

// All the details of a client. The full PAN is sent when adding or changing it, and never
// comes back: only pan_last4.
export interface ClientDetails {
  legal_name: string
  trade_name: string
  pan?: string
  pan_last4: string
  constitution: string
  date_of_incorporation: string
  cin: string
  tan: string
  gstins: { gstin: string; registration_type: string; primary: boolean }[]
  address: { line1: string; line2: string; city: string; state: string; pin: string }
  contact_person: string
  email: string
  phone: string
  accounting_software: string
  notes: string
}

export interface ClientProblem {
  field: string
  code: string
  message: string
}

export interface StoredFact {
  name: string
  period: string
  scope: string
  value: Value
  locator: Locator
}

export interface StoredDocument {
  document_type: string
  period: string
  scope: string
  part: string
  file_name: string
  file_sha256: string
  draft_no: number
  profile_id: string
  profile_version: string
  facts: StoredFact[]
  records: StoredRecord[] // rows such as invoices, for row documents (GST registers and returns)
}

// One stored row, with its columns: text fields and amounts in paise.
export interface StoredRecord {
  record_type: string
  match_key: string
  period: string
  scope: string
  fields: Record<string, string>
  amounts: Record<string, number>
  locator: { file_name: string; kind: string; path: string }
}

export interface FactList {
  engagements: { tax_year: string; documents: StoredDocument[] }[]
}

// One stored rule, as recon-engine rules --json lists it.
export interface RuleInfo {
  id: string
  version: string
  revision: number
  source: string // rulebook | builtin | office
  status: string // draft | testing | active | disabled
  runnable: boolean
  type: string
  domain: string
  category: string // the main reconciliation type
  also_categories?: string[] // further reconciliation types; absent when the rule has none
  title: string
  tax_years: string[]
  valid_from: string
  valid_to: string
  file_path: string
  approvable: boolean // a testing rule the Partner may approve now (SPEC-010 §4)
  approval_note: string // why it may not be approved yet, when approvable is false
  reads_office_documents?: boolean // an input is the rows of a document the Partner added (SPEC-022); absent from an older server
}

export interface RuleList {
  rules: RuleInfo[]
  tax_years: string[]
}

// A client as the server lists it (recon-engine clients --json). The full PAN is never sent.
export interface ServerClient {
  id: number
  name: string
  pan_last4: string
  last_run_on: string
  documents: number
  counts: RunReport['counts'] | null
  // SPEC-004 part 10: a disabled client is read only; missing from an older server (active).
  status?: 'active' | 'disabled'
  disabled_on?: string // YYYY-MM-DD
  disabled_by?: string
  disabled_reason?: string
}

export interface ServerClientList {
  tax_year: string
  clients: ServerClient[]
}

// A client's latest result (recon-engine result --json).
// Decisions on findings (SPEC-006 part 3): what staff decided, by whom and when.
export type DecisionKind = 'fix' | 'not_an_issue' | 'ask_client'

export interface DecisionEntry {
  decision: DecisionKind
  note: string
  decided_by: string
  decided_at: string // UTC
  on_other_values: boolean // the finding's figures changed after this decision
}

export interface FindingDecision {
  state: DecisionKind | 'open' // open again when the figures changed after the decision
  current: DecisionEntry
  history: DecisionEntry[] // oldest first
}

// TypeResult is a client's latest result of one type of check (SPEC-004 part 9).
export interface TypeResult {
  comparison_id: string // '' for a run of every rule
  last_run_on: string
  report: RunReport
}

// One run of a type of check for a client, numbered Comparison 1, 2, 3… in run order (SPEC-009).
export interface ComparisonEntry {
  no: number
  finished_at: string // YYYY-MM-DD HH:MM, the office computer's local time
  includes_testing: boolean
  kept: boolean // false for a run made before reports were kept: it cannot be opened
  counts: RunReport['counts'] | null
  files: { document_type: string; file_name: string; draft_no: number }[]
}

// ComparisonHistory lists a type's comparisons; opening one adds its report as it was and
// the figures of the drafts it read.
export interface ComparisonHistory {
  comparison_id: string
  tax_year: string
  comparisons: ComparisonEntry[]
  comparison?: ComparisonEntry
  report?: RunReport
  facts?: FactList
}

export interface ClientResult {
  client: ServerClient
  tax_year?: string // the tax year this result is for
  run_tax_years?: string[] // every tax year in which the client has a run, oldest first
  report: RunReport | null // the latest run of any type
  results?: TypeResult[] // the latest run of each type, sorted by type; missing from an older server
  decisions?: Record<string, FindingDecision> // by finding ID, for the findings that have one
  facts: FactList
}

export interface Client {
  id: string
  name: string
  pan_last4: string
  runs: string[] // the sample runs shown for this client, one per type of check; [] when none yet
  last_run_on: string
}

// The Worklist (SPEC-006 part 3): a ledger head that is new, vanished, missing months or
// much changed from last year, as recon-engine worklist --json gives it. It is for staff to
// classify; it is never a finding, and the tool never classifies it.
export type WorklistKind = 'new_head' | 'vanished' | 'broken_recurrence' | 'step_change'

export interface WorklistClassification {
  text: string
  by: string
  at: string // UTC, RFC 3339
}

export interface WorklistItem {
  client_id: number
  client_name: string
  key: string // "kind|ledger": names the item for its classification
  kind: WorklistKind
  kind_text: string
  ledger: string
  group: string
  text: string // what was counted, in words
  prior_total: number // paise
  current_total: number // paise
  prior_months: number
  current_months: number
  missing_months: string[]
  times: string // this year over last year, step changes only
  prior_rows: string[]
  current_rows: string[]
  classification: WorklistClassification | null
}

export interface WorklistList {
  tax_year: string
  items: WorklistItem[]
}

// The mapping screen (SPEC-004 part 8.4): what a file holds, and what the user picks.
export interface InspectedField {
  path: string // "$.rows[*].inv_no"
  kind: string
  example: string
  count: number
  values?: string[] // the different values, when there are few enough to choose among
}

export interface TeachableType {
  name: string
  description: string
  record_type: string
  // label, type and required come from the server (SPEC-022); a fixture without them reads as before.
  columns: { name: string; amount: boolean; key: boolean; label?: string; type?: 'text' | 'date' | 'amount'; required?: boolean }[]
  origin?: 'builtin' | 'office'
}

export interface Inspection {
  file: string
  recognised_as: string
  tax_year?: string // the tax year a recognised file is for; '' when not known
  fields: InspectedField[]
  lists: { path: string; count: number }[]
  truncated: boolean
  document_types: TeachableType[]
}

export interface LayoutMapping {
  name: string
  document_type: string
  rows: string
  where: { path: string; equals: string; in?: string[] }[]
  unit: 'rupees' | 'paise' | 'lakhs' | 'crores'
  negative?: '' | 'drcr' // '' reads a minus sign or brackets as written; 'drcr' reads Dr or Cr after the amount (SPEC-016)
  columns: { name: string; path: string; date_format: string }[]
  period: { path: string; format: string; const: string }
  tax_year: string
  gstin_path: string
  identify: { path: string; equals: string }[]
}

export interface TeachResult {
  profile_id: string
  profile_version: string
  document_type: string
  saved: boolean
  rows: number
  tax_year: string
  period: string
  pan_last4: string
  warnings: number
  preview: { match_key: string; locator: string; fields: Record<string, string>; amounts: Record<string, string> }[]
  profile_yaml: string
  dry_run?: DryRun // which rules can run on the layout (SPEC-016)
  draft?: boolean // an edit of a format: saved as a draft, not yet in use
  format_revision?: number
}

// The Rule Builder (SPEC-010). The lists the window offers (recon-engine rule-options).
export interface RuleCategory {
  code: string
  name: string
  description: string
}

export interface RuleFactOption {
  name: string
  kind: string // money, int, text, date, bool or rate
  periodicity: string
  document_type: string
  description: string
}

export interface RuleRecordOption {
  name: string
  description: string
  document_type: string
  match_key: string[] | null
  amounts: string[] | null
  fields: string[] | null
  // SPEC-022: a record type of a document the Partner added has its labelled columns and is offered for a new rule only while its
  // document is active. A server without the Document Master sends none of these: the record type is built-in and active.
  origin?: 'builtin' | 'office'
  active?: boolean
  columns?: { name: string; label: string; type: 'text' | 'date' | 'amount'; key: boolean; required: boolean }[]
}

export interface RuleDocumentOption {
  name: string
  description: string
}

export interface RuleOptions {
  modules: string[]
  severities: string[]
  categories: RuleCategory[]
  check_types: string[]
  row_check_types?: string[] // the check types that read rows: the only ones that can take an office record type (SPEC-022)
  facts: RuleFactOption[]
  record_types: RuleRecordOption[]
  document_types: RuleDocumentOption[]
}

// A rule as the window edits it: the same fields the engine writes (recon-engine rule-save).
// The engine writes null for an empty list; fetchRuleCurrent and fetchRuleVersions give empty lists.

export interface RuleFormInput {
  name: string
  kind: 'fact' | 'record_type'
  ref: string
  label: string
  optional: boolean
  default: string
}

export interface RuleFormPart {
  input: string
  field: string
  sign: string // "+" or "-"
}

export interface RuleFormCondition {
  input: string
  field: string
  is: string
  value: string
  values: string[]
}

export interface RuleFormTableRow {
  keys: string[]
  value: string
  source: string
}

export interface RuleFormCheck {
  type: string
  tolerance: string // rupees as typed, "" for exact
  compare_amounts: string[]
  compare_fields: string[]
  duplicates_in: string[]
  invoice_amounts: string[]
  pair_unmatched_on: string[]
  parts: RuleFormPart[]
  total: string
  total_field: string
  base: string
  rate: string
  value: string
  keys: string[]
  when: RuleFormCondition[]
  table: RuleFormTableRow[]
}

export interface RuleFormLaw {
  act: string
  section: string
  reference: string
}

export interface RuleFormNamed {
  name: string
  value: string
}

export interface RuleFormFact {
  fact: string
  value: string
  period: string
  scope: string
  file: string
  locator: string
}

export interface RuleFormRecord {
  record_type: string
  period: string
  scope: string
  file: string
  locator: string
  fields: RuleFormNamed[]
  amounts: RuleFormNamed[]
}

export interface RuleFormFixture {
  kind: 'pass' | 'fail'
  name: string
  description: string
  facts: RuleFormFact[]
  records: RuleFormRecord[]
}

export interface RuleForm {
  mode: 'create' | 'edit'
  id: string
  title: string
  rule_text: string
  notes: string
  module: string
  category: string // the main reconciliation type, the one reports and exports show
  more_categories?: string[] // further reconciliation types (optional)
  tax_years: string[]
  valid_from: string
  valid_to: string
  inputs: RuleFormInput[]
  check: RuleFormCheck
  severity: string
  review_document: string
  law: RuleFormLaw[]
  fixtures: RuleFormFixture[]
}

export interface RuleFixtureInfo {
  kind: string
  name: string
  description: string
}

// A rule as recon-engine rule-get returns it: its form when the window can edit it, and the
// fixtures the rule keeps.
export interface RuleCurrent {
  rule_id: string
  version: string
  source: string
  status: string
  disabled_reason: string
  editable: boolean
  reason: string
  approvable: boolean
  approval_note: string
  form?: RuleForm
  fixtures: RuleFixtureInfo[]
  rulebook?: RulebookRule | null // the whole rule-book rule, for a rule-book rule (SPEC-010 scope 8)
  has_rulebook_version?: boolean // the rule-book version is stored, so an office edit can be reset
}

export interface RuleVersionRow {
  id: number
  revision: number
  version: string
  source: string
  tax_years: string
  file_path: string
  created_by: string
  created_at: string
  active: boolean
}

export interface RuleVersionList {
  rule_id: string
  status: string
  versions: RuleVersionRow[]
}

// What a save returns (recon-engine rule-save).
export interface RuleSaved {
  rule_id: string
  version: string
  source: string
  status: string
  action: string // "created" or "edited"
  file: string
}

// The sign-in and the users (SPEC-012). An account never carries a password or its hash.
export interface Account {
  id: number
  login_name: string
  display_name: string
  role: string // partner, manager, staff or read_only
  active: boolean
  must_change_password: boolean
  failed_logins: number
  locked: boolean
  locked_until: string // UTC, RFC 3339; "" when not locked
  last_login_at: string // UTC, RFC 3339; "" when never
  created_at: string
  updated_at: string
}

// A Session is the signed-in user: the account, the permissions its role holds (from the
// server, never a copy kept here), and whether a password change is pending.
export interface Session {
  user: Account
  permissions: string[]
  must_change_password: boolean
}

export interface Lockout {
  max_failed_logins: number
  lock_minutes: number
}

export interface AdminList {
  users: Account[]
  roles: string[]
  lockout: Lockout
}

export interface AuditEntry {
  seq: number
  at: string
  actor: string
  action: string
  object_type: string
  object_id: string
  details: Record<string, string>
}

export interface AuditList {
  entries: AuditEntry[]
  total: number
  chain: { ok: boolean; message: string }
}

// A rule-book rule as its file holds it (docs/Recon_Tool_YAML_Rule_Store, RULE_TEMPLATE.yaml).
// The names are the file's. It is described only: shown in full, edited by a Partner, never run.
export interface RulebookPair {
  left?: string
  right?: string
  expression?: string
}

export interface RulebookDocUse {
  role: string
  doc_code: string
  required: boolean
}

export interface RulebookParam {
  key: string
  value: string
  type: string
  unit: string
  label: string
  editable: boolean
  min?: string
  max?: string
  allowed_values?: string[]
  note?: string
}

export interface RulebookOutcome {
  code: string
  description: string
  severity: string
  action?: string
}

export interface RulebookSource {
  title: string
  url: string
  ref: string
  official: boolean
}

export interface RulebookRule {
  schema_version: string
  rule: { id: string; version: string; type: string; domain: string; category: string; name: string; purpose: string }
  governance: {
    enabled: boolean
    verification_status: string
    verification_label: string
    reviewed_by: string
    reviewed_on: string
    review_decision: string
    last_modified_by: string
    last_modified_on: string
    change_note: string
  }
  validity: { valid_from: string; valid_to: string; effective_text: string; period_basis: string }
  legal: {
    provisions: string
    applicability: string
    exemptions_exceptions: string
    thresholds_rates: string
    rounding: string
    due_dates: string
  }
  inputs: { documents: RulebookDocUse[]; fields: string[]; prerequisites: string; input_validations: string }
  matching: { keys: RulebookPair[]; key_normalisation: string; aggregation: string; comparisons: RulebookPair[]; tolerance: string }
  calculation: { formula: string; steps: string[] }
  parameters: RulebookParam[]
  outcomes: {
    categories: RulebookOutcome[]
    default_severity: string
    expected_output: string
    recommended_action: string
    common_discrepancies: string
  }
  tests: { worked_example: string; positive: string; negative: string }
  implementation: { handler: string; pseudocode: string }
  sources: RulebookSource[]
  notes: string
}

// The dollar rate of the Admin tab's Currency page (SPEC-021). The rates are texts with four
// decimal places ("83.2500") as the server writes them; the screen never turns them into money.
export interface CurrencyChange {
  at: string
  by: string
  usd_rate: string
  previous: string // "" when there was no rate before
  effective_from: string
  note: string
}

export interface CurrencyState {
  set: boolean
  usd_rate: string // "" when no rate is set
  effective_from: string
  note: string
  problem: string // "" when the rate is usable; else "dollar rate not set" or "dollar rate not valid"
  detail: string
  history: CurrencyChange[] // newest first
}

export interface CurrencyInput {
  usd_rate: string
  effective_from: string
  note: string
}
