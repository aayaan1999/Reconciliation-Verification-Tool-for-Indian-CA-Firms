import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { blankForm, findingSentence, requiredLeft, ruleReadiness } from './ruleForm.ts'
import { canFrom, PermissionContext } from './permissions.ts'
import { Modal } from './screens/Modal.tsx'
import { RuleBuilder } from './screens/RuleBuilder.tsx'
import { Rules } from './screens/Rules.tsx'
import type { RuleForm, RuleInfo, RuleList, RuleOptions } from './types.ts'
import { bannedIn } from './wording.ts'

// The Rules screen and the Rule Builder as the Recon Screen Document v2 §10 lays them out: five
// status tiles that filter the table, a sortable table, one rule in a drawer with a readiness
// note and its actions in the footer, and the Rule Builder page with section navigation,
// toggle chips, the live sentence and the save bar. The rules and figures are illustrative.

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status, headers: { 'content-type': 'application/json' } })

function rule(over: Partial<RuleInfo>): RuleInfo {
  return {
    id: 'RECON-XYZ-001', version: '1', revision: 1, source: 'builtin', status: 'testing', runnable: true, type: 'tie-out', domain: '',
    category: 'R1', title: 'Illustrative rule', tax_years: ['2025-26'], valid_from: '2025-04-01', valid_to: '',
    file_path: 'rules/2025-26/tieout/RECON-XYZ-001/rule.yaml', approvable: false, approval_note: '', ...over,
  }
}

// One rule of each kind the tiles count: approved, testing (twice), described only (twice), disabled.
const book: RuleList = {
  tax_years: ['2025-26', '2026-27'],
  rules: [
    rule({ id: 'RECON-DEP-003', title: 'Charlie rule', status: 'active' }),
    rule({ id: 'RECON-DEP-001', title: 'Alpha rule', status: 'testing', approvable: true }),
    rule({ id: 'RECON-DEP-002', title: 'Bravo rule', status: 'testing', approval_note: 'the rule has not been run on a client with testing rules included' }),
    rule({ id: 'RC-GST-001', title: 'Delta rule', status: 'disabled', runnable: false, source: 'rulebook', type: 'statutory', domain: 'gst' }),
    rule({ id: 'RC-GST-002', title: 'Echo rule', status: 'disabled', runnable: false, source: 'rulebook', type: 'statutory', domain: 'gst' }),
    rule({ id: 'RECON-DEP-004', title: 'Foxtrot rule', status: 'disabled', source: 'office' }),
  ],
}

const partner = canFrom(['activate_rules', 'disable_rules', 'draft_rules', 'view'])
const staff = canFrom(['draft_rules', 'view'])

function stubVersions() {
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url.endsWith('/versions')) {
      return json({ rule_id: 'x', status: 'testing', versions: [
        { id: 1, revision: 1, version: '1', source: 'builtin', tax_years: '2025-26', file_path: 'a', created_by: 'engine', created_at: '', active: true },
      ] })
    }
    return json({ error_code: 'NOT_FOUND', message: 'no such endpoint' }, 404)
  }))
}

// A rule-book rule as rule-get returns it (illustrative), and the save the editor sends.
const rulebookRule = {
  schema_version: '1.0',
  rule: { id: 'RC-GST-001', version: '1', type: 'statutory', domain: 'gst', category: 'GST', name: 'Delta rule', purpose: 'Invoice-level match of books with GSTR-1.' },
  governance: { enabled: false, verification_status: 'verified', verification_label: 'Verified', reviewed_by: '', reviewed_on: '', review_decision: '', last_modified_by: 'import', last_modified_on: '2026-10-14', change_note: 'Imported' },
  validity: { valid_from: '2025-04-01', valid_to: '', effective_text: '2025-04-01', period_basis: 'tax period' },
  legal: { provisions: 'CGST Act s.37', applicability: 'Registered persons', exemptions_exceptions: '', thresholds_rates: '', rounding: '', due_dates: '' },
  inputs: { documents: [{ role: 'A', doc_code: 'SALES', required: true }], fields: null, prerequisites: '', input_validations: '' },
  matching: { keys: [{ left: 'SALES.invoice_no', right: 'GSTR1.invoice_no' }], key_normalisation: '', aggregation: '', comparisons: null, tolerance: 'Rs 1' },
  calculation: { formula: 'diff = books - gstr1', steps: ['Match by invoice.'] },
  parameters: [{ key: 'tolerance', value: '1', type: 'integer', unit: 'rupees', label: 'Tolerance', editable: true }],
  outcomes: { categories: [{ code: 'MATCHED', description: 'matched', severity: 'Low' }], default_severity: '', expected_output: '', recommended_action: '', common_discrepancies: '' },
  tests: { worked_example: 'Example', positive: 'All match', negative: 'One missing' },
  implementation: { handler: '', pseudocode: 'for inv in books: match(inv)' },
  sources: [{ title: 'Illustrative source', url: 'https://example.invalid/gst', ref: '', official: false }],
  notes: 'Illustrative note.',
}

function stubRulebook() {
  const calls: { saved: unknown } = { saved: null }
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith('/versions')) {
      return json({ rule_id: 'RC-GST-001', status: 'disabled', versions: [
        { id: 1, revision: 1, version: '1', source: 'rulebook', tax_years: '', file_path: 'a', created_by: 'load-rules', created_at: '', active: true },
      ] })
    }
    if (url.endsWith('/rulebook') && init?.method === 'PUT') {
      calls.saved = JSON.parse(String(init.body))
      return json({ rule_id: 'RC-GST-001', status: 'disabled' })
    }
    if (url.endsWith('/rules/RC-GST-001')) {
      return json({ rule_id: 'RC-GST-001', version: '1', source: 'rulebook', status: 'disabled', disabled_reason: 'not enabled in the rule store (governance.enabled is false)',
        editable: true, reason: '', approvable: false, approval_note: '', form: null, fixtures: null, rulebook: rulebookRule, has_rulebook_version: true })
    }
    return json({ error_code: 'NOT_FOUND', message: 'no such endpoint' }, 404)
  }))
  return calls
}

function showRules(can = partner, live = true) {
  stubVersions()
  render(
    <PermissionContext.Provider value={can}>
      <Rules list={book} live={live} />
    </PermissionContext.Provider>,
  )
}

const tile = (name: RegExp) => within(screen.getByRole('group', { name: /Rules by status/ })).getByRole('button', { name })
const ids = () => within(screen.getByRole('table', { name: 'Rules list' })).getAllByRole('row').slice(1).map((r) => r.querySelector('td')?.textContent)

describe('the rule book (§10.1)', () => {
  test('five tiles count the rules in the book, approved, testing, switched off (described only) and disabled', () => {
    showRules()
    expect(tile(/^Rules in the book/)).toHaveTextContent('6')
    expect(tile(/^Rules in the book/)).toHaveTextContent('4 can run as checks')
    expect(tile(/^Approved/)).toHaveTextContent('1')
    expect(tile(/^Testing/)).toHaveTextContent('2')
    expect(tile(/^Switched off/)).toHaveTextContent('2')
    expect(tile(/^Switched off/)).toHaveTextContent('described only')
    expect(tile(/^Disabled/)).toHaveTextContent('1')
  })

  test('a tile filters the table, and pressing it again shows every rule', () => {
    showRules()
    fireEvent.click(tile(/^Testing/))
    expect(tile(/^Testing/)).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByText('2 of 6 rules')).toBeInTheDocument()
    expect(ids()).toEqual(['RECON-DEP-001', 'RECON-DEP-002'])
    fireEvent.click(tile(/^Switched off/))
    expect(ids()).toEqual(['RC-GST-001', 'RC-GST-002'])
    expect(within(screen.getByRole('table', { name: 'Rules list' })).getAllByText('Described only, not run yet')).toHaveLength(2)
    fireEvent.click(tile(/^Disabled/))
    expect(ids()).toEqual(['RECON-DEP-004'])
    fireEvent.click(tile(/^Disabled/))
    expect(tile(/^Rules in the book/)).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByText('6 of 6 rules')).toBeInTheDocument()
  })

  test('the filter row has search, tax year, type, subject, category and source', () => {
    showRules()
    const row = screen.getByRole('group', { name: 'Rule filters' })
    for (const name of ['Search rules', 'Tax year', 'Type', 'Subject', 'Category', 'Source']) {
      expect(within(row).getByLabelText(name)).toBeInTheDocument()
    }
  })

  test('column headings sort the table; a third click returns to the book\'s order', () => {
    showRules()
    const byName = screen.getByRole('button', { name: /^Name/ })
    fireEvent.click(byName)
    expect(byName.closest('th')).toHaveAttribute('aria-sort', 'ascending')
    expect(ids()).toEqual(['RECON-DEP-001', 'RECON-DEP-002', 'RECON-DEP-003', 'RC-GST-001', 'RC-GST-002', 'RECON-DEP-004'])
    fireEvent.click(byName)
    expect(byName.closest('th')).toHaveAttribute('aria-sort', 'descending')
    expect(ids()?.[0]).toBe('RECON-DEP-004')
    fireEvent.click(byName)
    expect(byName.closest('th')).toHaveAttribute('aria-sort', 'none')
    expect(ids()).toEqual(book.rules.map((r) => r.id))
  })

  test('the columns are rule, name, type and subject, category, tax years, status and source', () => {
    showRules(partner, false)
    const heads = within(screen.getByRole('table', { name: 'Rules list' })).getAllByRole('columnheader').map((h) => h.textContent?.trim())
    expect(heads).toEqual(['Rule', 'Name', 'Type and subject', 'Category', 'Tax years', 'Status', 'Source'])
  })
})

describe('one rule in a popup (§10.2)', () => {
  test('a rule ready for approval: the note says so, and a Partner\'s Approve is enabled', async () => {
    showRules()
    fireEvent.click(screen.getByText('Alpha rule'))
    const drawer = screen.getByRole('dialog', { name: 'RECON-DEP-001' })
    // The rule opens as a popup in the middle of the screen, not a side drawer.
    expect(drawer.closest('.drawer-backdrop')).toBeNull()
    expect(drawer.closest('.sheet-backdrop')).not.toBeNull()
    expect(within(drawer).getByRole('note')).toHaveTextContent(/^Ready for approval/)
    const foot = within(drawer).getByRole('group', { name: 'Actions on this rule' })
    expect(within(foot).getByRole('button', { name: 'Approve' })).toBeEnabled()
    expect(within(foot).getByRole('button', { name: 'Disable' })).toBeInTheDocument()
    expect(within(foot).getByRole('button', { name: 'Edit this rule' })).toBeInTheDocument()
    expect(await within(drawer).findByRole('table', { name: 'Versions' })).toHaveTextContent('engine')
  })

  test('a rule not ready: the note gives the engine\'s reason, and Approve is disabled', () => {
    showRules()
    fireEvent.click(screen.getByText('Bravo rule'))
    const drawer = screen.getByRole('dialog', { name: 'RECON-DEP-002' })
    expect(within(drawer).getByRole('note')).toHaveTextContent('Not ready for approval: the rule has not been run on a client with testing rules included')
    expect(within(drawer).getByRole('button', { name: 'Approve' })).toBeDisabled()
  })

  test('Staff see no Approve and no Disable in the footer', () => {
    showRules(staff)
    fireEvent.click(screen.getByText('Alpha rule'))
    const drawer = screen.getByRole('dialog', { name: 'RECON-DEP-001' })
    expect(within(drawer).queryByRole('button', { name: 'Approve' })).toBeNull()
    expect(within(drawer).queryByRole('button', { name: 'Disable' })).toBeNull()
    expect(within(drawer).getByRole('button', { name: 'Edit this rule' })).toBeInTheDocument()
  })

  test('Disable opens the confirmation that needs a reason', () => {
    showRules()
    fireEvent.click(screen.getByText('Alpha rule'))
    fireEvent.click(within(screen.getByRole('dialog', { name: 'RECON-DEP-001' })).getByRole('button', { name: 'Disable' }))
    expect(screen.getByRole('form', { name: 'Switch off RECON-DEP-001' })).toBeInTheDocument()
  })

  test('a described-only rule shows the whole rule, and a Partner may edit it and switch it on (SPEC-010 scope 8)', async () => {
    const calls = stubRulebook()
    render(
      <PermissionContext.Provider value={partner}>
        <Rules list={book} live />
      </PermissionContext.Provider>,
    )
    fireEvent.click(screen.getByText('Delta rule'))
    const drawer = screen.getByRole('dialog', { name: 'RC-GST-001' })
    expect(within(drawer).getByRole('note')).toHaveTextContent('Described only and switched off')
    // Every section of the rule-book file is in the summary.
    for (const name of ['Purpose', 'Governance', 'Validity', 'Legal basis', 'Inputs', 'Matching', 'Calculation', 'Parameters', 'Outcomes', 'Tests', 'Pseudocode', 'Sources', 'Notes']) {
      expect(await within(drawer).findByRole('region', { name })).toBeInTheDocument()
    }
    expect(within(drawer).getByText('Invoice-level match of books with GSTR-1.')).toBeInTheDocument()
    expect(within(drawer).getByText('Switched off because: not enabled in the rule store (governance.enabled is false)')).toBeInTheDocument()
    // Sources are text, never links: the tool is offline.
    expect(within(drawer).queryByRole('link')).toBeNull()
    const actions = within(drawer).getByRole('group', { name: 'Actions on this rule' })
    expect(within(actions).getByRole('button', { name: 'Enable again' })).toBeInTheDocument()
    expect(within(actions).queryByRole('button', { name: 'Approve' })).toBeNull()

    // Edit opens the rule-book editor; saving needs a reason and sends the whole rule.
    fireEvent.click(within(actions).getByRole('button', { name: 'Edit this rule' }))
    const form = await screen.findByRole('form', { name: 'Edit rule-book rule RC-GST-001' })
    fireEvent.change(within(form).getByLabelText('Purpose *'), { target: { value: 'Changed purpose.' } })
    fireEvent.click(within(form).getByRole('button', { name: 'Save as office version' }))
    expect(await within(form).findByRole('alert')).toHaveTextContent('Say why the rule is changed')
    fireEvent.change(within(form).getByLabelText('Reason for the change *'), { target: { value: 'agreed with the CA' } })
    fireEvent.click(within(form).getByRole('button', { name: 'Save as office version' }))
    await vi.waitFor(() => expect(calls.saved).not.toBeNull())
    expect(calls.saved).toMatchObject({ reason: 'agreed with the CA', rule: { rule: { id: 'RC-GST-001', purpose: 'Changed purpose.' } } })
  })

  test('staff see the whole rule-book rule but cannot edit, switch on or switch off it', async () => {
    stubRulebook()
    render(
      <PermissionContext.Provider value={staff}>
        <Rules list={book} live />
      </PermissionContext.Provider>,
    )
    fireEvent.click(screen.getByText('Delta rule'))
    const drawer = screen.getByRole('dialog', { name: 'RC-GST-001' })
    expect(await within(drawer).findByRole('region', { name: 'Purpose' })).toBeInTheDocument()
    expect(within(drawer).queryByRole('group', { name: 'Actions on this rule' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Edit RC-GST-001' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Enable again RC-GST-001' })).toBeNull()
  })

  test('every readiness note is free of banned phrases', () => {
    for (const r of book.rules) {
      for (const live of [true, false]) expect(bannedIn(ruleReadiness(r, live).text)).toEqual([])
    }
  })
})

const options: RuleOptions = {
  modules: ['tieout'],
  severities: ['high'],
  categories: [{ code: 'R1', name: 'Same figure in two places', description: '' }],
  check_types: ['equal', 'gte', 'sum_equals', 'percent_of', 'present_together', 'table_lookup', 'match_records'],
  facts: [
    { name: 'computation.allowed_depreciation', kind: 'money', periodicity: 'FY', document_type: 'computation', description: '' },
    { name: 'computation.fixed_asset_schedule.total', kind: 'money', periodicity: 'FY', document_type: 'computation', description: '' },
  ],
  record_types: [
    { name: 'sales_b2b_invoice', description: '', document_type: 'sales_register', match_key: null, amounts: null, fields: null },
    { name: 'gstr1_b2b_invoice', description: '', document_type: 'gstr1', match_key: null, amounts: null, fields: null },
  ],
  document_types: [{ name: 'computation', description: 'Computation of total income' }],
}

function equalForm(over: Partial<RuleForm['check']> = {}): RuleForm {
  const f = blankForm(options, '2025-26')
  return {
    ...f,
    inputs: [
      { name: 'a', kind: 'fact', ref: 'computation.allowed_depreciation', label: 'Allowed depreciation', optional: false, default: '' },
      { name: 'b', kind: 'fact', ref: 'computation.fixed_asset_schedule.total', label: '', optional: false, default: '' },
    ],
    check: { ...f.check, ...over },
  }
}

describe('the live sentence (§10.3)', () => {
  test('equal and gte state the two figures and the tolerance', () => {
    expect(findingSentence(equalForm({ type: 'equal', tolerance: '1.00' }))).toBe(
      'A finding is raised when Allowed depreciation and computation.fixed_asset_schedule.total differ by more than ₹1.00.',
    )
    expect(findingSentence(equalForm({ type: 'equal' }))).toBe(
      'A finding is raised when Allowed depreciation and computation.fixed_asset_schedule.total differ.',
    )
    expect(findingSentence(equalForm({ type: 'gte' }))).toBe(
      'A finding is raised when Allowed depreciation is lower than computation.fixed_asset_schedule.total.',
    )
  })

  test('sum_equals and percent_of name the parts, the total, the rate and the base', () => {
    const parts = [{ input: 'a', field: '', sign: '+' }, { input: 'b', field: '', sign: '-' }]
    expect(findingSentence({ ...equalForm({ type: 'sum_equals', parts: [parts[0]], total: 'b' }) })).toBe(
      'A finding is raised when Allowed depreciation differs from computation.fixed_asset_schedule.total.',
    )
    expect(findingSentence(equalForm({ type: 'percent_of', parts, rate: '4.00', value: 'a', tolerance: '1.00' }))).toBe(
      'A finding is raised when 4.00% of the sum of Allowed depreciation minus computation.fixed_asset_schedule.total, rounded to the rupee, differs from Allowed depreciation by more than ₹1.00.',
    )
  })

  test('until the check has its figures, the sentence says what is missing', () => {
    expect(findingSentence(blankForm(options, '2025-26'))).toBe('Add the figures a and b in What it reads to see when a finding is raised.')
  })

  test('no sentence uses a banned phrase', () => {
    const forms = [
      equalForm({ type: 'equal', tolerance: '1.00' }), equalForm({ type: 'gte' }),
      equalForm({ type: 'present_together', when: [{ input: 'a', field: '', is: 'nonzero', value: '', values: [] }] }),
      equalForm({ type: 'table_lookup', value: 'a', keys: ['b'] }), equalForm({ type: 'match_records', compare_amounts: ['igst'] }),
    ]
    for (const f of forms) expect(bannedIn(findingSentence(f))).toEqual([])
  })
})

describe('required fields left in each section (§10.3)', () => {
  test('a blank new rule counts what each section still needs', () => {
    const left = requiredLeft(blankForm(options, '2025-26'), 'create')
    expect(left).toEqual({ who: 3, where: 2, reads: 1, check: 0, examples: 2 })
  })

  test('a pass and a fail fixture are both required; an edited rule\'s kept fixtures count', () => {
    const f = { ...blankForm(options, '2025-26'), fixtures: [{ kind: 'pass' as const, name: 'agree', description: '', facts: [], records: [] }] }
    expect(requiredLeft(f, 'create').examples).toBe(2) // the fail fixture, and the pass fixture's figures
    expect(requiredLeft({ ...f, fixtures: [] }, 'edit', ['pass', 'fail']).examples).toBe(0)
  })
})

describe('the Rule Builder page (§10.3)', () => {
  function build(onClose = () => {}) {
    vi.stubGlobal('fetch', vi.fn(async () => json({ rule_id: 'RECON-OFF-001', version: '1', source: 'office', status: 'testing', action: 'created', file: 'x' })))
    render(<RuleBuilder mode="create" current={null} options={options} taxYears={['2025-26', '2026-27']} onSaved={() => {}} onClose={onClose} />)
  }

  test('the navigation lists the five sections with the required fields left', () => {
    build()
    const nav = screen.getByRole('navigation', { name: 'Sections of the rule' })
    const items = within(nav).getAllByRole('button').map((b) => b.textContent)
    expect(items).toEqual(['Who and what3 required left', 'Where it applies2 required left', 'What it reads1 required left', 'The checkdone', 'Examples2 required left'])
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'An illustrative rule' } })
    expect(within(nav).getAllByRole('button')[0]).toHaveTextContent('2 required left')
    expect(screen.getByRole('group', { name: 'Save the rule' })).toHaveTextContent('7 required fields left.')
  })

  test('tax years and record types are toggle chips', () => {
    build()
    const years = screen.getByRole('group', { name: 'Tax years' })
    const y2627 = within(years).getByRole('button', { name: '2026-27' })
    expect(within(years).getByRole('button', { name: '2025-26' })).toHaveAttribute('aria-pressed', 'true')
    expect(y2627).toHaveAttribute('aria-pressed', 'false')
    fireEvent.click(y2627)
    expect(y2627).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(screen.getByRole('button', { name: 'Add a figure or rows' }))
    fireEvent.change(screen.getByLabelText('Kind 1'), { target: { value: 'record_type' } })
    const rows = screen.getByRole('group', { name: 'Which rows 1' })
    const sales = within(rows).getByRole('button', { name: 'sales_b2b_invoice' })
    fireEvent.click(sales)
    expect(sales).toHaveAttribute('aria-pressed', 'true')
    expect(within(rows).getByRole('button', { name: 'gstr1_b2b_invoice' })).toHaveAttribute('aria-pressed', 'false')
  })

  test('the sentence follows the fields as they are typed', () => {
    build()
    for (const [i, name, ref] of [[1, 'a', 'computation.allowed_depreciation'], [2, 'b', 'computation.fixed_asset_schedule.total']] as const) {
      fireEvent.click(screen.getByRole('button', { name: 'Add a figure or rows' }))
      fireEvent.change(screen.getByLabelText(`Name in the rule ${i}`), { target: { value: name } })
      fireEvent.change(screen.getByLabelText(`Which figure ${i}`), { target: { value: ref } })
    }
    fireEvent.change(screen.getByLabelText('Tolerance in rupees'), { target: { value: '1.00' } })
    expect(screen.getByText(/^A finding is raised when/)).toHaveTextContent(
      'A finding is raised when computation.allowed_depreciation and computation.fixed_asset_schedule.total differ by more than ₹1.00.',
    )
  })

  test('Ctrl S saves as testing, and Esc goes back', async () => {
    const onClose = vi.fn()
    build(onClose)
    await act(async () => {
      fireEvent.keyDown(screen.getByLabelText('Title'), { key: 's', ctrlKey: true })
    })
    expect(vi.mocked(fetch)).toHaveBeenCalledWith('/api/v1/rules', expect.objectContaining({ method: 'POST' }))
    expect(await screen.findByRole('status')).toHaveTextContent('Saved RECON-OFF-001 version 1 as testing')
    fireEvent.keyDown(screen.getByRole('button', { name: 'Back to rules' }), { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  test('a key typed in the builder popup does not reach the screens behind', () => {
    // As the Rules screen shows it: the builder inside its popup, which holds the keys back.
    vi.stubGlobal('fetch', vi.fn(async () => json({})))
    render(
      <Modal title="Add rule" onClose={() => {}}>
        <RuleBuilder mode="create" current={null} options={options} taxYears={['2025-26']} onSaved={() => {}} onClose={() => {}} />
      </Modal>,
    )
    const seen = vi.fn()
    window.addEventListener('keydown', seen)
    fireEvent.keyDown(screen.getByRole('button', { name: 'Add a figure or rows' }), { key: '1' })
    window.removeEventListener('keydown', seen)
    expect(seen).not.toHaveBeenCalled()
  })
})
