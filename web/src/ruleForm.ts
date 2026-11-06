// The Rule Builder's pure helpers (SPEC-010): the form's shape, which fields each check type uses,
// the words for each field the server reports, and the lists typed with commas. No rule is
// decided here: the engine checks and writes every rule.
import type { RuleForm, RuleFormFixture, RuleFormInput, RuleInfo, RuleOptions } from './types.ts'

// ruleReadiness says plainly whether a rule can be approved and, if not, why (Recon Screen
// Document v2 §10.2). It reads only what the server listed for the rule: its status, whether it
// runs, and the engine's approval gate (SPEC-010 §4). live is false for the sample data.
export function ruleReadiness(r: RuleInfo, live: boolean): { tone: 'pass' | 'warn' | 'muted'; text: string } {
  if (!r.runnable) {
    return r.status === 'disabled'
      ? { tone: 'muted', text: 'Described only and switched off: the rule book describes this rule, but it has no check the engine can run yet, so it is never run.' }
      : { tone: 'muted', text: 'Described only and switched on: the rule book describes this rule, but it has no check the engine can run yet, so it is never run.' }
  }
  switch (r.status) {
    case 'active':
      return { tone: 'pass', text: 'Approved: it runs in every run.' }
    case 'disabled':
      return { tone: 'muted', text: 'Switched off: it does not run. Enabled again, it comes back as testing and waits for approval.' }
    case 'testing':
      if (!live) return { tone: 'muted', text: 'Testing: waiting for approval. Start recon-server to see whether it can be approved.' }
      return r.approvable
        ? { tone: 'pass', text: 'Ready for approval: its fixtures pass and it has run on a client with testing rules included.' }
        : { tone: 'warn', text: `Not ready for approval: ${r.approval_note || 'the engine gave no reason.'}` }
    default:
      return { tone: 'muted', text: 'Draft: it does not run yet, so it cannot be approved.' }
  }
}

// The check types the engine runs, with what each one compares, in the words of the window.
export const checkTypeText: Record<string, string> = {
  equal: 'Two figures must agree, within the tolerance.',
  sum_equals: 'The figures added up (and taken away) must equal the total.',
  gte: 'One figure must be at least another, within the tolerance.',
  percent_of: 'A rate of a base (or of parts) must equal the figure given.',
  present_together: 'Conditions that hold together are reported as found, with no difference.',
  table_lookup: 'The figure must equal the value in the small table of the rule for its keys.',
  match_records: 'Rows of two record types are matched by their key; a key on one side only, or amounts that differ, are reported.',
}

// Which fields of the window each check type uses. The window shows only these.
export interface CheckFields {
  tolerance: boolean
  parts: boolean // figures added or taken away, each with its sign
  total: boolean // sum_equals: the input the sum is compared with, and the column of the total
  base: boolean // percent_of: the input the rate is applied to
  rate: boolean
  value: boolean // percent_of: the figure compared; table_lookup: the figure looked up
  when: boolean // present_together: the conditions
  keys: boolean // table_lookup: the inputs whose values select a row
  table: boolean // table_lookup: the rows of the table
  matching: boolean // match_records: the lists of columns
}

export function checkFields(type: string): CheckFields {
  const none: CheckFields = {
    tolerance: false, parts: false, total: false, base: false, rate: false, value: false, when: false, keys: false, table: false, matching: false,
  }
  switch (type) {
    case 'equal':
    case 'gte':
      return { ...none, tolerance: true }
    case 'sum_equals':
      return { ...none, tolerance: true, parts: true, total: true }
    case 'percent_of':
      return { ...none, tolerance: true, parts: true, base: true, rate: true, value: true }
    case 'present_together':
      return { ...none, when: true }
    case 'table_lookup':
      return { ...none, keys: true, value: true, table: true }
    case 'match_records':
      return { ...none, tolerance: true, matching: true }
    default:
      return none
  }
}

// blankForm is a new rule with nothing filled in, ready for the window.
export function blankForm(options: RuleOptions, taxYear: string): RuleForm {
  return {
    mode: 'create',
    id: '',
    title: '',
    rule_text: '',
    notes: '',
    module: options.modules[0] ?? 'tieout',
    category: options.categories[0]?.code ?? 'R1',
    more_categories: [],
    tax_years: taxYear ? [taxYear] : [],
    valid_from: '',
    valid_to: '',
    inputs: [],
    check: {
      type: options.check_types.includes('equal') ? 'equal' : (options.check_types[0] ?? ''),
      tolerance: '',
      compare_amounts: [],
      compare_fields: [],
      duplicates_in: [],
      invoice_amounts: [],
      pair_unmatched_on: [],
      parts: [],
      total: '',
      total_field: '',
      base: '',
      rate: '',
      value: '',
      keys: [],
      when: [],
      table: [],
    },
    severity: options.severities[0] ?? 'high',
    review_document: options.document_types[0]?.name ?? '',
    law: [{ act: '', section: '', reference: '' }],
    fixtures: [],
  }
}

// blankFixture is a pass or a fail fixture with no figures yet.
export function blankFixture(kind: 'pass' | 'fail'): RuleFormFixture {
  return { kind, name: '', description: '', facts: [], records: [] }
}

// blankInput is a figure or rows the rule reads, with nothing chosen yet.
export function blankInput(): RuleFormInput {
  return { name: '', kind: 'fact', ref: '', label: '', optional: false, default: '' }
}

// nn replaces the null the engine writes for an empty list with an empty list.
function nn<T>(x: T[] | null | undefined): T[] {
  return x ?? []
}

// normaliseForm gives every list a value, so the window can always map and append to it.
export function normaliseForm(f: RuleForm): RuleForm {
  const c = f.check
  return {
    ...f,
    more_categories: nn(f.more_categories),
    tax_years: nn(f.tax_years),
    inputs: nn(f.inputs),
    law: nn(f.law),
    fixtures: nn(f.fixtures).map((fx) => ({
      ...fx,
      facts: nn(fx.facts),
      records: nn(fx.records).map((r) => ({ ...r, fields: nn(r.fields), amounts: nn(r.amounts) })),
    })),
    check: {
      ...c,
      compare_amounts: nn(c.compare_amounts),
      compare_fields: nn(c.compare_fields),
      duplicates_in: nn(c.duplicates_in),
      invoice_amounts: nn(c.invoice_amounts),
      pair_unmatched_on: nn(c.pair_unmatched_on),
      parts: nn(c.parts),
      keys: nn(c.keys),
      when: nn(c.when).map((w) => ({ ...w, values: nn(w.values) })),
      table: nn(c.table).map((t) => ({ ...t, keys: nn(t.keys) })),
    },
  }
}

// parseList reads a list typed with commas: "taxable_value, igst" gives two names.
export function parseList(text: string): string[] {
  return text
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s !== '')
}

// forSave is the form as the engine is sent it: only the parameters the check type uses (the
// engine refuses any other, so a field left from an earlier choice is cleared), and rows the CA
// added but left empty are dropped.
export function forSave(form: RuleForm): RuleForm {
  const f = normaliseForm(form)
  const c = f.check
  const use = checkFields(c.type)
  return {
    ...f,
    inputs: nn(f.inputs).filter((i) => i.name !== '' || i.ref !== '' || i.label !== ''),
    law: nn(f.law).filter((l) => l.act !== '' || l.section !== '' || l.reference !== ''),
    fixtures: f.fixtures.map((fx) => ({
      ...fx,
      facts: fx.facts.filter((x) => Object.values(x).some((v) => v !== '')) ?? [],
      records: fx.records.filter((x) => x.record_type !== '' || x.period !== '' || x.file !== '' || x.locator !== '' || x.fields.length > 0 || x.amounts.length > 0) ?? [],
    })),
    check: {
      type: c.type,
      tolerance: use.tolerance ? c.tolerance : '',
      compare_amounts: use.matching ? c.compare_amounts : [],
      compare_fields: use.matching ? c.compare_fields : [],
      duplicates_in: use.matching ? c.duplicates_in : [],
      invoice_amounts: use.matching ? c.invoice_amounts : [],
      pair_unmatched_on: use.matching ? c.pair_unmatched_on : [],
      parts: use.parts ? nn(c.parts).filter((p) => p.input !== '' || p.field !== '') : [],
      total: use.total ? c.total : '',
      total_field: use.total ? c.total_field : '',
      base: use.base ? c.base : '',
      rate: use.rate ? c.rate : '',
      value: use.value ? c.value : '',
      keys: use.keys ? c.keys : [],
      when: use.when ? c.when : [],
      table: use.table ? nn(c.table).filter((t) => t.value !== '' || t.source !== '' || nn(t.keys).length > 0) : [],
    },
  }
}

// The sections of the Rule Builder page, in the order of its navigation (Recon Screen Document
// v2 §10.3).
export type BuilderSection = 'who' | 'where' | 'reads' | 'check' | 'examples'

export const builderSections: { key: BuilderSection; title: string }[] = [
  { key: 'who', title: 'Who and what' },
  { key: 'where', title: 'Where it applies' },
  { key: 'reads', title: 'What it reads' },
  { key: 'check', title: 'The check' },
  { key: 'examples', title: 'Examples' },
]

const blank = (s: string) => s.trim() === ''
const count = (...missing: boolean[]) => missing.filter(Boolean).length

// requiredLeft counts, for each section, the required fields not yet filled in. It only guides
// the CA through the page: the engine still checks the rule when it is saved. keptKinds are the
// kinds of the fixtures an edited rule keeps ("pass", "fail"), which count as given.
export function requiredLeft(form: RuleForm, mode: 'create' | 'edit', keptKinds: string[] = []): Record<BuilderSection, number> {
  const use = checkFields(form.check.type)
  const law = form.law.filter((l) => !blank(l.act) && !blank(l.section)).length === 0
    ? 1
    : form.law.filter((l) => (!blank(l.act) || !blank(l.section) || !blank(l.reference)) && (blank(l.act) || blank(l.section))).length
  const kinds = new Set([...keptKinds, ...form.fixtures.map((f) => f.kind)])
  return {
    who: count(mode === 'create' && blank(form.id), blank(form.title), blank(form.rule_text)),
    where: count(blank(form.module), blank(form.category), form.tax_years.length === 0, blank(form.valid_from)) + law,
    reads: form.inputs.length === 0 ? 1 : form.inputs.filter((i) => blank(i.name) || blank(i.ref)).length,
    check:
      count(blank(form.check.type), blank(form.severity), blank(form.review_document)) +
      count(use.rate && blank(form.check.rate), use.value && blank(form.check.value), use.base && blank(form.check.base) && form.check.parts.length === 0),
    examples:
      count(!kinds.has('pass'), !kinds.has('fail')) +
      form.fixtures.filter((f) => blank(f.name)).length +
      form.fixtures.filter((f) => f.facts.length === 0 && f.records.length === 0).length,
  }
}

// The words for one test of a present_together condition.
function conditionText(is: string, value: string, values: string[]): string {
  switch (is) {
    case 'blank': return 'is blank'
    case 'not_blank': return 'is not blank'
    case 'zero': return 'is zero'
    case 'nonzero': return 'is not zero'
    case 'equals': return `is "${value}"`
    case 'matches': return `matches the pattern "${value}"`
    case 'in': return `is one of ${values.join(', ')}`
    case 'not_in': return `is none of ${values.join(', ')}`
    case 'true': return 'is yes'
    case 'false': return 'is no'
    case 'same_as': return `is the same as ${value}`
    default: return is
  }
}

// findingSentence states, from the form as it is filled in, exactly when the rule raises a
// finding. It describes the comparison only: it never names a correct value (invariant 1).
// Until the check has what it needs, it says what is missing.
export function findingSentence(form: RuleForm): string {
  const c = form.check
  const named = (name: string) => form.inputs.find((i) => i.name === name && name !== '')
  const text = (name: string) => {
    const i = named(name)
    return i ? (i.label.trim() || i.ref || i.name) : name
  }
  const column = (name: string, field: string) => (field ? `${field} of ${text(name)}` : text(name))
  const tolerance = blank(c.tolerance) ? '' : ` by more than ₹${c.tolerance.trim()}`
  const partsText = () =>
    c.parts
      .filter((p) => p.input !== '')
      .map((p, i) => `${i === 0 ? (p.sign === '-' ? 'minus ' : '') : p.sign === '-' ? ' minus ' : ' plus '}${column(p.input, p.field)}`)
      .join('')
  const needAB = 'Add the figures a and b in What it reads to see when a finding is raised.'
  switch (c.type) {
    case 'equal':
      if (!named('a') || !named('b')) return needAB
      return `A finding is raised when ${text('a')} and ${text('b')} differ${tolerance}.`
    case 'gte':
      if (!named('a') || !named('b')) return needAB
      return `A finding is raised when ${text('a')} is lower than ${text('b')}${tolerance}.`
    case 'sum_equals': {
      if (partsText() === '' || !named(c.total)) return 'Choose the figures added up and the total they are compared with to see when a finding is raised.'
      const rows = c.total_field !== '' || c.parts.some((p) => p.field !== '')
      return `A finding is raised ${rows ? 'for each row where' : 'when'} ${partsText()} differs from ${column(c.total, c.total_field)}${tolerance}.`
    }
    case 'percent_of': {
      const base = named(c.base) ? text(c.base) : partsText() !== '' ? `the sum of ${partsText()}` : ''
      if (base === '' || blank(c.rate) || !named(c.value)) return 'Choose the base, the rate and the figure compared to see when a finding is raised.'
      return `A finding is raised when ${c.rate.trim()}% of ${base}, rounded to the rupee, differs from ${text(c.value)}${tolerance}.`
    }
    case 'present_together': {
      const conds = c.when.filter((w) => w.input !== '' && w.is !== '').map((w) => `${column(w.input, w.field)} ${conditionText(w.is, w.value, w.values)}`)
      if (conds.length === 0) return 'Add a condition to see when a finding is raised.'
      return conds.length === 1
        ? `A finding is raised when ${conds[0]}.`
        : `A finding is raised when all of these hold together: ${conds.join('; ')}.`
    }
    case 'table_lookup':
      if (!named(c.value) || c.keys.length === 0) return 'Choose the figure looked up and the keys to see when a finding is raised.'
      return `A finding is raised when ${text(c.value)} differs from the value in the rule's table for ${c.keys.map(text).join(' and ')}. A key with no row in the table is Cannot check, not a finding.`
    case 'match_records': {
      if (!named('a') || !named('b')) return needAB
      const amounts = c.compare_amounts.length > 0 ? `, for ${c.compare_amounts.join(', ')} that differ${tolerance}` : ''
      const fields = c.compare_fields.length > 0 ? `, or for ${c.compare_fields.join(', ')} that differ` : ''
      return `Rows of ${text('a')} and ${text('b')} are matched by their key. A finding is raised for a key found on one side only${amounts}${fields}.`
    }
    default:
      return 'Choose a check type to see when a finding is raised.'
  }
}

// The words for a field the server reports, such as "inputs[1].ref" or "fixtures[0].facts[2].value".
const topLevel: Record<string, string> = {
  id: 'ID',
  title: 'Title',
  rule_text: 'Rule text',
  module: 'Module',
  category: 'Reconciliation type',
  severity: 'Severity',
  review_document: 'Review document',
  tax_years: 'Tax year',
  valid_from: 'Valid from',
  valid_to: 'Valid to',
  inputs: 'Figures and rows',
  'check.type': 'Check type',
  'check.tolerance': 'Tolerance',
  'check.rate': 'Rate',
  law: 'Legal basis',
  fixtures: 'Fixtures',
  mode: 'Mode',
  rule: 'The rule file',
}
const inputSub: Record<string, string> = { ref: 'which figure or record type', label: 'label', name: 'name in the rule', kind: 'kind', default: 'default' }
const factSub: Record<string, string> = { fact: 'figure', value: 'value', period: 'period', scope: 'scope', file: 'source file', locator: 'locator in the file' }
const recordSub: Record<string, string> = {
  record_type: 'record type', period: 'period', scope: 'scope', file: 'source file', locator: 'locator in the file', fields: 'column', amounts: 'amount',
}

export function fieldLabel(field: string): string {
  if (field in topLevel) return topLevel[field]
  let m = /^tax_years\[\d+\]$/.exec(field)
  if (m) return 'Tax year'
  m = /^inputs\[(\d+)\]\.?(\w*)$/.exec(field)
  if (m) return `Figure or row ${Number(m[1]) + 1}${m[2] ? ', ' + (inputSub[m[2]] ?? m[2]) : ''}`
  m = /^law\[(\d+)\]/.exec(field)
  if (m) return `Legal basis ${Number(m[1]) + 1}`
  m = /^fixtures\[(\d+)\]\.facts\[(\d+)\]\.(\w+)$/.exec(field)
  if (m) return `Fixture ${Number(m[1]) + 1}, figure ${Number(m[2]) + 1}, ${factSub[m[3]] ?? m[3]}`
  m = /^fixtures\[(\d+)\]\.records\[(\d+)\]\.(\w+)$/.exec(field)
  if (m) return `Fixture ${Number(m[1]) + 1}, row ${Number(m[2]) + 1}, ${recordSub[m[3]] ?? m[3]}`
  m = /^fixtures\[(\d+)\]\.(\w+)$/.exec(field)
  if (m) return `Fixture ${Number(m[1]) + 1}, ${m[2]}`
  m = /^fixtures\[(\d+)\]$/.exec(field)
  if (m) return `Fixture ${Number(m[1]) + 1}`
  return field
}
