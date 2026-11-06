// The connection to recon-server's API (SPEC-004 part 6). When no server answers (the Vite
// dev server on its own, or the tests), the screens use the sample data instead and say so.
import type {
  AdminList, AuditList, Account, ClientDetails, ClientProblem, ClientResult, Comparison, ComparisonHistory, FactList, Inspection, LayoutMapping, Lockout, RuleList, RunReport, ServerClient, ServerClientList, Session, TeachResult,
  CurrencyInput, CurrencyState, TypeResult, DecisionKind, FindingDecision, WorklistItem, WorklistKind, WorklistList,
  RuleCurrent, RuleForm, RuleOptions, RuleSaved, RuleVersionList, RulebookRule,
} from './types.ts'
import { normaliseForm } from './ruleForm.ts'
import type { DryRun, FormatMapping, FormatsList, ImportPlan, NewParameter } from './formatsTypes.ts'
import type { DocDeleted, DocDetail, DocInput, DocRow, DocSaved, DocValidation } from './docMasterTypes.ts'

export interface RunResult {
  exit_code: number
  report: RunReport
  results?: TypeResult[] // one per type of check run; for a client, the latest of each of its types
  decisions?: Record<string, FindingDecision> // a client's decisions, by finding ID
  facts: FactList
}

// A RuleChoice is one type of check to run and the rules chosen for it (SPEC-004 part 9);
// rules is null for a type that has no rules.
export interface RuleChoice {
  comparison: string
  rules: string[] | null
}

// An ApiError is a refusal from the server or the engine, with its stable code
// (for example PROFILE_NO_MATCH), its HTTP status (0 when no server answered) and the message to show.
export class ApiError extends Error {
  code: string
  problems: ClientProblem[]
  status: number
  constructor(code: string, message: string, problems: ClientProblem[] = [], status = 0) {
    super(message)
    this.code = code
    this.problems = problems
    this.status = status
  }
}

// sessionEnded is told when a request is refused with 401: the sign-in has ended (or was never
// made), so the app shows the sign-in screen again (SPEC-012).
let sessionEnded: (() => void) | null = null

export function onSessionEnded(handler: (() => void) | null) {
  sessionEnded = handler
}

async function readError(resp: Response): Promise<ApiError> {
  try {
    const body = (await resp.json()) as { error_code?: string; message?: string; problems?: ClientProblem[] }
    return new ApiError(body.error_code ?? `HTTP_${resp.status}`, body.message ?? resp.statusText, body.problems ?? [], resp.status)
  } catch {
    return new ApiError(`HTTP_${resp.status}`, resp.statusText, [], resp.status)
  }
}

// refusal reads a refused answer, and tells the app when the sign-in has ended.
async function refusal(resp: Response): Promise<ApiError> {
  if (resp.status === 401) sessionEnded?.()
  return readError(resp)
}

// fetchRules returns the stored rules, or null when no server is running.
export async function fetchRules(): Promise<RuleList | null> {
  try {
    const resp = await fetch('/api/v1/rules')
    if (resp.status === 401) sessionEnded?.()
    if (!resp.ok || !(resp.headers.get('content-type') ?? '').includes('application/json')) return null
    return (await resp.json()) as RuleList
  } catch {
    return null
  }
}

async function call<T>(url: string, init?: RequestInit): Promise<T> {
  let resp: Response
  try {
    resp = await fetch(url, init)
  } catch {
    throw new ApiError('NO_SERVER', 'The server is not running. Start recon-server.')
  }
  if (!resp.ok) throw await refusal(resp)
  return (await resp.json()) as T
}

// fetchClients returns the clients in the database for a tax year, or null when no server
// is running.
export async function fetchClients(taxYear: string): Promise<ServerClientList | null> {
  try {
    // Disabled clients too (SPEC-004 part 10): the board hides them unless asked, and their files open read only.
    return await call<ServerClientList>(`/api/v1/clients?tax_year=${encodeURIComponent(taxYear)}&all=1`)
  } catch {
    return null
  }
}

// addClient adds a client with all its details. The PAN is sent once and never comes back.
export async function addClient(details: ClientDetails, taxYear: string): Promise<ServerClient> {
  const out = await call<{ client: ServerClient }>('/api/v1/clients', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ tax_year: taxYear, details }),
  })
  return out.client
}

// updateClient replaces a client's details; an empty PAN keeps the PAN.
export async function updateClient(id: number, details: ClientDetails): Promise<ClientDetails> {
  const out = await call<{ details: ClientDetails }>(`/api/v1/clients/${id}`, {
    method: 'PUT',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ details }),
  })
  return out.details
}

// fetchClientDetails returns all of a client's details (never the full PAN).
export async function fetchClientDetails(id: number): Promise<ClientDetails> {
  const out = await call<{ details: ClientDetails }>(`/api/v1/clients/${id}`)
  return out.details
}

// decideFinding records a decision on one of a client's findings and returns the finding's
// decisions (SPEC-006 part 3). The server records the signed-in user as the decider (SPEC-012).
export async function decideFinding(
  clientId: number, taxYear: string, findingId: string, decision: DecisionKind, note: string,
): Promise<FindingDecision> {
  const out = await call<{ decision: FindingDecision }>(`/api/v1/clients/${clientId}/decisions`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ tax_year: taxYear, finding_id: findingId, decision, note }),
  })
  return out.decision
}

// exportClient asks the server for a client's Excel export of a tax year (SPEC-013 §4). The file
// is the server's and carries the name the server gave it. A refusal is an ApiError that names
// the permission the server needs.
export async function exportClient(clientId: number, taxYear: string): Promise<{ blob: Blob; filename: string }> {
  const resp = await fetch(`/api/v1/clients/${clientId}/exports`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ tax_year: taxYear, format: 'xlsx' }),
  })
  if (!resp.ok) throw await refusal(resp)
  const named = /filename="([^"]+)"/.exec(resp.headers.get('content-disposition') ?? '')
  return { blob: await resp.blob(), filename: named?.[1] ?? 'recon-export.xlsx' }
}

// setClientDisabled disables a client or enables it again, with the reason (SPEC-004 part 10; Partner only).
export async function setClientDisabled(clientId: number, disable: boolean, reason: string): Promise<void> {
  await send('POST', `/api/v1/clients/${clientId}/${disable ? 'disable' : 'enable'}`, { reason })
}

// deleteFinding marks a finding deleted, with the reason (SPEC-013 §4). The finding is kept: a
// Partner still lists it, marked deleted, and the other roles stop seeing it.
export async function deleteFinding(clientId: number, taxYear: string, findingId: string, reason: string): Promise<void> {
  await send('POST', `/api/v1/clients/${clientId}/findings/${findingId}/delete`, { tax_year: taxYear, reason })
}

// fetchWorklist returns every client's Worklist items for a tax year, or null when no server
// is running (SPEC-006 part 3).
export async function fetchWorklist(taxYear: string): Promise<WorklistList | null> {
  try {
    return await call<WorklistList>(`/api/v1/worklist?tax_year=${encodeURIComponent(taxYear)}`)
  } catch {
    return null
  }
}

// classifyItem records the classification staff wrote for one of a client's Worklist items
// and returns the client's items. The server records the signed-in user.
export async function classifyItem(
  clientId: number, taxYear: string, kind: WorklistKind, ledger: string, classification: string,
): Promise<WorklistItem[]> {
  const out = await call<WorklistList>(`/api/v1/clients/${clientId}/worklist`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ tax_year: taxYear, kind, ledger, classification }),
  })
  return out.items
}

// fetchComparisons returns the types of check, or null when no server is running.
export async function fetchComparisons(): Promise<Comparison[] | null> {
  try {
    return (await call<{ comparisons: Comparison[] }>('/api/v1/comparisons')).comparisons
  } catch {
    return null
  }
}

// fetchClientResult returns a client's latest result for a tax year.
export async function fetchClientResult(clientId: number, taxYear: string): Promise<ClientResult> {
  return call<ClientResult>(`/api/v1/clients/${clientId}/result?tax_year=${encodeURIComponent(taxYear)}`)
}

// fetchHistory lists a client's runs of one type of check; with no, it also gives that run's
// report as it was and the figures it read (SPEC-009).
export async function fetchHistory(clientId: number, taxYear: string, comparison: string, no?: number): Promise<ComparisonHistory> {
  const q = new URLSearchParams({ tax_year: taxYear, comparison })
  if (no !== undefined) q.set('no', String(no))
  return call<ComparisonHistory>(`/api/v1/clients/${clientId}/comparisons?${q}`)
}

// runChecks uploads files and runs the checks on them; with a client, the files are added
// to that client's documents and the result covers all of them.
export async function runChecks(
  files: File[],
  taxYear: string,
  includeTesting: boolean,
  clientId?: number,
  choices: RuleChoice[] = [],
): Promise<RunResult> {
  const form = new FormData()
  if (clientId !== undefined) form.append('client_id', String(clientId))
  for (const c of choices) {
    form.append('comparison', c.comparison)
    if (c.rules !== null) form.append(`rules:${c.comparison}`, c.rules.join(','))
  }
  form.append('tax_year', taxYear)
  form.append('include_testing', String(includeTesting))
  for (const f of files) form.append('files', f, f.name)
  let resp: Response
  try {
    resp = await fetch('/api/v1/runs', { method: 'POST', body: form })
  } catch {
    throw new ApiError('NO_SERVER', 'The server is not running. Start recon-server to upload files.')
  }
  if (!resp.ok) throw await refusal(resp)
  return (await resp.json()) as RunResult
}

async function postForm<T>(url: string, form: FormData): Promise<T> {
  let resp: Response
  try {
    resp = await fetch(url, { method: 'POST', body: form })
  } catch {
    throw new ApiError('NO_SERVER', 'The server is not running. Start recon-server.')
  }
  if (!resp.ok) throw await refusal(resp)
  return (await resp.json()) as T
}

// inspectFile lists the fields of a file, for the mapping screen.
export async function inspectFile(file: File): Promise<Inspection> {
  const form = new FormData()
  form.append('file', file, file.name)
  return postForm<Inspection>('/api/v1/inspect', form)
}

// teachLayout sends a mapping and a file in its layout. With checkOnly the server only
// previews the rows; otherwise it saves the layout, and later files in it are recognised.
export async function teachLayout(
  file: File, mapping: LayoutMapping, checkOnly: boolean, formatId?: string, keepSample?: boolean,
): Promise<TeachResult> {
  const form = new FormData()
  form.append('mapping', JSON.stringify(mapping))
  form.append('check_only', String(checkOnly))
  if (formatId) form.append('format', formatId) // an edit of this office format: saved as a draft (SPEC-016)
  if (keepSample && !checkOnly) form.append('keep_sample', 'true') // the Partner keeps the sample to read again on publish
  form.append('file', file, file.name)
  return postForm<TeachResult>('/api/v1/profiles', form)
}

// The Rule Builder (SPEC-010). Each change is checked and written by recon-engine; a refused
// save is an ApiError with the problems and their fields. The server records the signed-in user
// as the one who made the change (SPEC-012), so no name is sent.
async function send<T>(method: string, url: string, body: unknown = {}): Promise<T> {
  return call<T>(url, {
    method,
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(body),
  })
}

// fetchRuleOptions returns the lists the Rule Builder window offers.
export async function fetchRuleOptions(): Promise<RuleOptions> {
  return call<RuleOptions>('/api/v1/rule-options')
}

// fetchRuleCurrent returns a rule as the window edits it, or throws when it is not found.
export async function fetchRuleCurrent(id: string): Promise<RuleCurrent> {
  const cur = await call<RuleCurrent>(`/api/v1/rules/${encodeURIComponent(id)}`)
  return {
    ...cur,
    form: cur.form ? normaliseForm(cur.form) : undefined,
    fixtures: cur.fixtures ?? [],
    rulebook: cur.rulebook ? normaliseRulebook(cur.rulebook) : null,
  }
}

// saveRule creates a rule (mode create) or saves a changed one (mode edit). It returns what was
// written: the version and that the rule is testing.
export async function saveRule(form: RuleForm): Promise<RuleSaved> {
  if (form.mode === 'edit') return send<RuleSaved>('PUT', `/api/v1/rules/${encodeURIComponent(form.id)}`, { rule: form })
  return send<RuleSaved>('POST', '/api/v1/rules', { rule: form })
}

// switchRule switches a rule off or on again, with the reason kept in the audit log.
export async function switchRule(id: string, action: 'disable' | 'enable', reason: string): Promise<void> {
  await send<unknown>('POST', `/api/v1/rules/${encodeURIComponent(id)}/${action}`, { reason })
}

// switchRules switches several rules off or on again with one reason, all or none (SPEC-010 scope 9).
export async function switchRules(ids: string[], action: 'disable' | 'enable', reason: string): Promise<void> {
  await send<unknown>('POST', `/api/v1/rules/bulk/${action}`, { rule_ids: ids, reason })
}

// approveRule makes a testing rule active, with a reason, kept in the audit log. The server
// refuses it, with the reason, when the gate is not met (SPEC-010 §4).
export async function approveRule(id: string, reason: string): Promise<void> {
  await send<unknown>('POST', `/api/v1/rules/${encodeURIComponent(id)}/approve`, { reason })
}

// fetchRuleVersions lists a rule's stored versions, oldest first.
export async function fetchRuleVersions(id: string): Promise<RuleVersionList> {
  const out = await call<RuleVersionList>(`/api/v1/rules/${encodeURIComponent(id)}/versions`)
  return { ...out, versions: out.versions ?? [] }
}

// pickRuleVersion makes another stored version the one in use (recorded in the audit log).
export async function pickRuleVersion(id: string, versionId: number): Promise<void> {
  await send<unknown>('POST', `/api/v1/rules/${encodeURIComponent(id)}/versions/${versionId}/switch`)
}

// saveRulebookRule saves a Partner's edit of a rule-book rule, with the reason kept in the audit
// log (SPEC-010 scope 8). The server stores it as an office version; the rule-book file is not changed.
export async function saveRulebookRule(id: string, reason: string, rule: RulebookRule): Promise<void> {
  await send<unknown>('PUT', `/api/v1/rules/${encodeURIComponent(id)}/rulebook`, { reason, rule })
}

// normaliseRulebook turns the engine's null lists into empty ones, so the screens can map them.
export function normaliseRulebook(r: RulebookRule): RulebookRule {
  return {
    ...r,
    inputs: { ...r.inputs, documents: r.inputs.documents ?? [], fields: r.inputs.fields ?? [] },
    matching: { ...r.matching, keys: r.matching.keys ?? [], comparisons: r.matching.comparisons ?? [] },
    calculation: { ...r.calculation, steps: r.calculation.steps ?? [] },
    parameters: r.parameters ?? [],
    outcomes: { ...r.outcomes, categories: r.outcomes.categories ?? [] },
    sources: r.sources ?? [],
  }
}

// resetRule makes the standard (built-in) version the one in use again.
export async function resetRule(id: string): Promise<void> {
  await send<unknown>('POST', `/api/v1/rules/${encodeURIComponent(id)}/reset`)
}

// The sign-in and the session (SPEC-012). fetchSession says who is signed in: a session, no
// session (the sign-in screen), or no server at all (the sample data, as before).
export type SessionState = { kind: 'signed_in'; session: Session } | { kind: 'signed_out' } | { kind: 'unavailable' }

export async function fetchSession(): Promise<SessionState> {
  try {
    const resp = await fetch('/api/v1/me')
    if (resp.status === 401) return { kind: 'signed_out' }
    if (!resp.ok || !(resp.headers.get('content-type') ?? '').includes('application/json')) return { kind: 'unavailable' }
    const body = (await resp.json()) as Partial<Session>
    if (!body.user || !Array.isArray(body.permissions)) return { kind: 'unavailable' }
    return { kind: 'signed_in', session: body as Session }
  } catch {
    return { kind: 'unavailable' }
  }
}

// postPlain sends a JSON body and does not treat a 401 as an ended session: the sign-in and the
// first-start setup answer 401 for a wrong password, which is not a session ending.
async function postPlain<T>(url: string, body: unknown): Promise<T> {
  let resp: Response
  try {
    resp = await fetch(url, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body) })
  } catch {
    throw new ApiError('NO_SERVER', 'The server is not running. Start recon-server.')
  }
  if (!resp.ok) throw await readError(resp)
  return (await resp.json()) as T
}

// signIn checks the login name and password and starts a session. A wrong password gets the
// same message for an unknown name and a known one.
export async function signIn(loginName: string, password: string): Promise<Session> {
  return postPlain<Session>('/api/v1/login', { login_name: loginName, password })
}

// signOut ends the session and writes the sign-out to the audit log.
export async function signOut(): Promise<void> {
  await send<unknown>('POST', '/api/v1/logout')
}

// changeMyPassword replaces the signed-in user's password; the current one is required.
export async function changeMyPassword(current: string, next: string): Promise<Session> {
  return send<Session>('POST', '/api/v1/me/password', { current_password: current, new_password: next })
}

// fetchSetupNeeded says whether the first Partner must still be set up, or null with no server.
export async function fetchSetupNeeded(): Promise<boolean | null> {
  try {
    return (await call<{ needed: boolean }>('/api/v1/setup')).needed
  } catch {
    return null
  }
}

// setUpFirstPartner creates the first Partner, once, with the password the Partner chooses.
export async function setUpFirstPartner(loginName: string, displayName: string, password: string): Promise<Account> {
  return postPlain<Account>('/api/v1/setup', { login_name: loginName, display_name: displayName, password })
}

// The Admin tab (SPEC-012): the Partner's list of users, and the changes to them. Each change
// is one audit entry, written by the server with the Partner's login name.
export async function fetchAdminUsers(): Promise<AdminList> {
  return call<AdminList>('/api/v1/admin/users')
}

export async function adminAddUser(user: { login_name: string; display_name: string; role: string; password: string }): Promise<Account> {
  return send<Account>('POST', '/api/v1/admin/users', user)
}

export async function adminSetRole(id: number, role: string, reason: string): Promise<Account> {
  return send<Account>('POST', `/api/v1/admin/users/${id}/role`, { role, reason })
}

export async function adminResetPassword(id: number, password: string): Promise<Account> {
  return send<Account>('POST', `/api/v1/admin/users/${id}/password`, { password })
}

export async function adminSetActive(id: number, active: boolean, reason: string): Promise<Account> {
  return send<Account>('POST', `/api/v1/admin/users/${id}/active`, { active, reason })
}

export async function adminUnlock(id: number): Promise<Account> {
  return send<Account>('POST', `/api/v1/admin/users/${id}/unlock`)
}

export async function adminSetLockout(l: Lockout): Promise<Lockout> {
  return send<Lockout>('POST', '/api/v1/admin/lockout', l)
}

// fetchAudit returns the newest audit entries and whether the hash chain is intact.
export async function fetchAudit(): Promise<AuditList> {
  return call<AuditList>('/api/v1/admin/audit')
}

// The Formats screen (SPEC-016, Partner only). The server records the signed-in user as the one
// who made each change, and the engine writes one audit entry for it.
export async function fetchFormats(): Promise<FormatsList> {
  const out = await call<FormatsList>('/api/v1/formats')
  return {
    ...out,
    formats: (out.formats ?? []).map((f) => ({ ...f, samples: f.samples ?? [] })),
    parameters: out.parameters ?? [],
    unsupplied: out.unsupplied ?? [],
    uncovered: out.uncovered ?? [],
    unrecognised: out.unrecognised ?? [],
  }
}

export async function switchFormat(id: string, action: 'disable' | 'enable', reason: string): Promise<void> {
  await send<unknown>('POST', `/api/v1/formats/${encodeURIComponent(id)}/${action}`, { reason })
}

// pickFormatVersion makes an earlier version of an office format the one in use (as testing).
export async function pickFormatVersion(id: string, revision: number, reason: string): Promise<void> {
  await send<unknown>('POST', `/api/v1/formats/${encodeURIComponent(id)}/versions/${revision}/switch`, { reason })
}

// fetchFormatMapping reads the mapping of an office format, for the mapping screen to edit.
export async function fetchFormatMapping(id: string): Promise<FormatMapping> {
  return call<FormatMapping>(`/api/v1/formats/${encodeURIComponent(id)}/mapping`)
}

// publishFormat makes a draft version the one in use, as testing or active.
// A refusal with the code SAMPLES_CHANGED means a kept sample reads differently with this version:
// publish again with confirm and a reason to go ahead.
export async function publishFormat(
  id: string, revision: number, status: 'testing' | 'active', confirm?: { reason: string },
): Promise<{ dry_run?: DryRun }> {
  return send<{ dry_run?: DryRun }>('POST', `/api/v1/formats/${encodeURIComponent(id)}/publish`,
    confirm ? { revision, status, confirm: true, reason: confirm.reason } : { revision, status })
}

// resolveUnrecognised closes a file no format recognised: taught, or dismissed with a reason.
export async function resolveUnrecognised(id: number, status: 'taught' | 'dismissed', note: string): Promise<void> {
  await send<unknown>('POST', `/api/v1/unrecognised/${id}/resolve`, { status, note })
}

export async function removeKeptSample(id: number): Promise<void> {
  await send<unknown>('POST', `/api/v1/format-samples/${id}/remove`, {})
}

// exportFormat downloads an office format and its parameters as a bundle file.
export async function exportFormat(id: string): Promise<{ blob: Blob; filename: string }> {
  const resp = await fetch(`/api/v1/formats/${encodeURIComponent(id)}/export`)
  if (!resp.ok) throw await refusal(resp)
  const named = /filename="([^"]+)"/.exec(resp.headers.get('content-disposition') ?? '')
  return { blob: await resp.blob(), filename: named?.[1] ?? 'format.json' }
}

// importBundle checks a bundle (checkOnly) or adds its format, as testing, and its parameters.
export async function importBundle(bundle: unknown, checkOnly: boolean): Promise<ImportPlan> {
  return send<ImportPlan>('POST', '/api/v1/formats/import', { bundle, check_only: checkOnly })
}

export async function addParameter(p: NewParameter): Promise<void> {
  await send<unknown>('POST', '/api/v1/parameters', p)
}

export async function switchParameter(key: string, action: 'disable' | 'enable', reason: string): Promise<void> {
  await send<unknown>('POST', `/api/v1/parameters/${encodeURIComponent(key)}/${action}`, { reason })
}

export async function changeAlias(key: string, alias: string, remove: boolean): Promise<void> {
  await send<unknown>('POST', `/api/v1/parameters/${encodeURIComponent(key)}/aliases`, { alias, remove })
}

// The dollar rate (SPEC-021): any signed-in user reads it; the Partner sets it on the Admin tab's
// Currency page. The server records the signed-in user as the one who changed it.
export async function fetchCurrency(): Promise<CurrencyState> {
  const out = await call<CurrencyState>('/api/v1/currency')
  return { ...out, history: out.history ?? [] }
}

export async function setCurrency(c: CurrencyInput): Promise<CurrencyState> {
  const out = await send<CurrencyState>('PUT', '/api/v1/currency', c)
  return { ...out, history: out.history ?? [] }
}

// The Document Master (SPEC-022, Partner only to change; any signed-in user reads the list). The
// engine checks every definition and writes the audit entry with the signed-in user, so no name is
// sent. A refused save is an ApiError (INVALID_DOCUMENT with each problem and its field).
export async function fetchDocuments(): Promise<DocRow[]> {
  const out = await call<{ documents: DocRow[] }>('/api/v1/document-types')
  return out.documents ?? []
}

export async function fetchDocument(code: string): Promise<DocDetail> {
  return call<DocDetail>(`/api/v1/document-types/${encodeURIComponent(code)}`)
}

// validateDocument checks a definition without saving it; with edit it checks a change to an
// existing document. A copy's columns come back filled in.
export async function validateDocument(input: DocInput, edit: boolean): Promise<DocValidation> {
  const out = await send<DocValidation>('POST', `/api/v1/document-types/validate${edit ? '?edit=1' : ''}`, input)
  return { ...out, problems: out.problems ?? [] }
}

export async function addDocument(input: DocInput): Promise<DocSaved> {
  return send<DocSaved>('POST', '/api/v1/document-types', input)
}

export async function editDocument(code: string, input: DocInput): Promise<DocSaved> {
  return send<DocSaved>('PUT', `/api/v1/document-types/${encodeURIComponent(code)}`, input)
}

// setDocumentActive switches a document on or off for new layouts and rules. Switching off is
// always allowed; the answer says what still reads the document.
export async function setDocumentActive(code: string, active: boolean): Promise<DocDetail> {
  return send<DocDetail>('POST', `/api/v1/document-types/${encodeURIComponent(code)}/status`, { active })
}

export async function deleteDocument(code: string): Promise<DocDeleted> {
  return call<DocDeleted>(`/api/v1/document-types/${encodeURIComponent(code)}`, { method: 'DELETE' })
}
