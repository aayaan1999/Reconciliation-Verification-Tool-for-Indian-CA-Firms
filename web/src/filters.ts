// The filters of the Rules view and of a run's results (SPEC-004 part 6). They are plain
// functions, so the tests check them directly. A list filter that is empty means "all".
import type { CannotCheck, Finding, Passed, RuleInfo, RunReport } from './types.ts'

// ---------- tax years ----------

// fyStart returns the start year of the tax year "2025-26" (2025), or NaN.
export function fyStart(taxYear: string): number {
  const m = /^(\d{4})-(\d{2})$/.exec(taxYear)
  return m ? Number(m[1]) : Number.NaN
}

// taxYearOf returns the tax year an ISO date falls in: "2025-07-31" → "2025-26".
export function taxYearOf(iso: string): string {
  const m = /^(\d{4})-(\d{2})-\d{2}$/.exec(iso)
  if (!m) return ''
  const y = Number(m[1]) - (Number(m[2]) < 4 ? 1 : 0)
  return `${y}-${String((y + 1) % 100).padStart(2, '0')}`
}

// How a rule relates to a tax year.
export type YearMatch = 'named' | 'dated' | 'not_dated' | 'no'

// ruleYearMatch: a rule naming tax years applies to those; a rule with validity dates applies
// to every tax year the dates overlap (1 April to 31 March); a rule with neither is "not
// dated" and is shown for every year, labelled as such.
export function ruleYearMatch(r: RuleInfo, taxYear: string): YearMatch {
  if (r.tax_years.length > 0) return r.tax_years.includes(taxYear) ? 'named' : 'no'
  if (!r.valid_from && !r.valid_to) return 'not_dated'
  const y = fyStart(taxYear)
  const fyFrom = `${y}-04-01`
  const fyTo = `${y + 1}-03-31`
  const from = r.valid_from || '0000-01-01'
  const to = r.valid_to || '9999-12-31'
  return from <= fyTo && to >= fyFrom ? 'dated' : 'no'
}

// allTaxYears lists every tax year the year pickers offer, 1999-00 to 2046-47, oldest first.
// A year no rule names can still be chosen; its run simply has no rules to check.
export const firstTaxYear = 1999
export const lastTaxYear = 2046
export function allTaxYears(): string[] {
  const out: string[] = []
  for (let y = firstTaxYear; y <= lastTaxYear; y++) out.push(`${y}-${String((y + 1) % 100).padStart(2, '0')}`)
  return out
}

// taxYearOptions lists the tax years worth offering: those the rules name, and those their
// validity dates fall in, newest first.
export function taxYearOptions(rules: RuleInfo[], named: string[]): string[] {
  const set = new Set(named)
  for (const r of rules) {
    r.tax_years.forEach((t) => set.add(t))
    for (const d of [r.valid_from, r.valid_to]) {
      const t = taxYearOf(d)
      if (t) set.add(t)
    }
  }
  return [...set].sort().reverse()
}

// ---------- rules ----------

// Each list filter holds the chosen values: empty means no filter, several mean "any of".
export interface RuleFilter {
  taxYear: string[]
  type: string[]
  domain: string[]
  category: string[]
  status: string[]
  source: string[]
  search: string
}

export const noRuleFilter: RuleFilter = { taxYear: [], type: [], domain: [], category: [], status: [], source: [], search: '' }

// anyOf is true when nothing is chosen or the value is one of the chosen.
export function anyOf(chosen: string[], value: string): boolean {
  return chosen.length === 0 || chosen.includes(value)
}

function matchesText(search: string, ...texts: string[]): boolean {
  const q = search.trim().toLowerCase()
  return q === '' || texts.some((t) => t.toLowerCase().includes(q))
}

// filterRules applies every filter; `except` leaves one out, to work out that filter's own
// options from the others.
export function filterRules(rules: RuleInfo[], f: RuleFilter, except?: keyof RuleFilter): RuleInfo[] {
  return rules.filter(
    (r) =>
      (except === 'taxYear' || f.taxYear.length === 0 || f.taxYear.some((y) => ruleYearMatch(r, y) !== 'no')) &&
      (except === 'type' || anyOf(f.type, r.type)) &&
      (except === 'domain' || anyOf(f.domain, r.domain)) &&
      (except === 'category' || anyOf(f.category, r.category)) &&
      (except === 'status' || anyOf(f.status, r.status)) &&
      (except === 'source' || anyOf(f.source, r.source)) &&
      (except === 'search' || matchesText(f.search, r.id, r.title, r.category)),
  )
}

// distinct returns the sorted distinct non-empty values of one field.
export function distinct<T>(items: T[], pick: (t: T) => string): string[] {
  return [...new Set(items.map(pick).filter((v) => v !== ''))].sort()
}

// ---------- results of a run ----------

export type ResultStatus = 'fail' | 'cannot_check' | 'pass'

// A ResultRow is one result of a run, whatever its status, for filtering.
export interface ResultRow {
  status: ResultStatus
  ruleId: string
  title: string
  category: string
  documents: string[] // the document to review, or the documents a missing figure comes from
  key: string
  finding?: Finding
  cannot?: CannotCheck
  passed?: Passed
}

export interface ResultFilter {
  status: string
  ruleId: string
  category: string
  document: string
  search: string
}

export const noResultFilter: ResultFilter = { status: '', ruleId: '', category: '', document: '', search: '' }

// resultRows lists every result of a report: findings, Cannot check and passes.
export function resultRows(r: RunReport): ResultRow[] {
  const rows: ResultRow[] = []
  const subjectKey = (s: { period: string; scope: string; match_key: string }) => `${s.period}|${s.scope}|${s.match_key}`
  for (const g of r.findings) {
    for (const f of g.findings) {
      rows.push({ status: 'fail', ruleId: f.rule_id, title: f.title, category: f.category, documents: [g.heading],
        key: `fail|${f.rule_id}|${subjectKey(f.subject)}`, finding: f })
    }
  }
  for (const c of r.cannot_check) {
    rows.push({ status: 'cannot_check', ruleId: c.rule_id, title: c.title, category: c.category,
      documents: [...new Set(c.missing.map((m) => m.document))], key: `cc|${c.rule_id}|${subjectKey(c.subject)}`, cannot: c })
  }
  for (const p of r.passed) {
    rows.push({ status: 'pass', ruleId: p.rule_id, title: p.title, category: p.category,
      documents: [documentHeading(r, p.review_document)], key: `pass|${p.rule_id}|${subjectKey(p.subject)}`, passed: p })
  }
  return rows
}

// documentHeading names a review document the way the findings groups do, when known.
function documentHeading(r: RunReport, docType: string): string {
  return r.findings.find((g) => g.review_document === docType)?.heading ?? docType
}

export function filterResults(rows: ResultRow[], f: ResultFilter, except?: keyof ResultFilter): ResultRow[] {
  return rows.filter(
    (r) =>
      (except === 'status' || !f.status || r.status === f.status) &&
      (except === 'ruleId' || !f.ruleId || r.ruleId === f.ruleId) &&
      (except === 'category' || !f.category || r.category === f.category) &&
      (except === 'document' || !f.document || r.documents.includes(f.document)) &&
      (except === 'search' || matchesText(f.search, r.ruleId, r.title, r.category, ...r.documents,
        ...(r.finding?.values.map((v) => v.label) ?? []), ...(r.cannot?.missing.map((m) => m.label) ?? []))),
  )
}

// builderTaxYears lists the tax years a new rule can name: 2017-18 to 2030-31.
export function builderTaxYears(): string[] {
  const out: string[] = []
  for (let y = 2017; y <= 2030; y++) out.push(`${y}-${String((y + 1) % 100).padStart(2, '0')}`)
  return out
}
