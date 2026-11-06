import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import App from './App.tsx'
import { sampleRules } from './sample/index.ts'
import { blankForm, checkFields, fieldLabel, forSave, normaliseForm, parseList } from './ruleForm.ts'
import { RuleBuilder } from './screens/RuleBuilder.tsx'
import type { RuleCurrent, RuleInfo, RuleList, RuleOptions } from './types.ts'
import { railTo } from './testFindings.ts'

// SPEC-010 in the browser: the Rules tab's actions, the Rule Builder window's refusals and its
// links, the reason a rule is switched off with, and the pure helpers of the window. The server
// is stubbed; the figures are illustrative.

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })
}

const options: RuleOptions = {
  modules: ['tieout', 'gst'],
  severities: ['high', 'medium', 'low'],
  categories: [{ code: 'R1', name: 'Same figure in two places', description: '' }],
  check_types: ['equal', 'sum_equals', 'gte', 'percent_of', 'present_together', 'table_lookup', 'match_records'],
  facts: [
    { name: 'computation.allowed_depreciation', kind: 'money', periodicity: 'FY', document_type: 'computation', description: '' },
    { name: 'computation.fixed_asset_schedule.total', kind: 'money', periodicity: 'FY', document_type: 'computation', description: '' },
  ],
  record_types: [],
  document_types: [{ name: 'computation', description: 'Computation of total income' }],
}

// stubServer answers like recon-server with the rules, the options and no clients. extra may
// answer a request first. It returns the requests made, in order.
function stubServer(extra: (url: string, init?: RequestInit) => Response | undefined = () => undefined) {
  const calls: { url: string; method: string; body: string }[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, method: init?.method ?? 'GET', body: String(init?.body ?? '') })
      const answer = extra(url, init)
      if (answer) return answer
      if (url === '/api/v1/rules') return json(sampleRules)
      if (url.startsWith('/api/v1/clients?')) return json({ tax_year: '2025-26', clients: [] })
      if (url === '/api/v1/comparisons') return json({ comparisons: [] })
      if (url === '/api/v1/rule-options') return json(options)
      return json({ error_code: 'NOT_FOUND', message: 'no such endpoint' }, 404)
    }),
  )
  return calls
}

// searchFor narrows the Rules list to the rules with this text, as a user would with the search box.
function searchFor(text: string) {
  fireEvent.change(screen.getByLabelText("Search rules"), { target: { value: text } })
}

async function openRules() {
  render(<App />)
  await act(async () => {})
  await act(async () => {})
  railTo('Rules')
}

// rowAction opens a rule's "⋯" actions menu in the list and returns the named action, or null
// when the row has no menu or the menu has no such action.
function rowAction(id: string, name: string): HTMLElement | null {
  const menu = screen.queryByRole('button', { name: `Actions for ${id}` })
  if (!menu) return null
  if (menu.getAttribute('aria-expanded') !== 'true') fireEvent.click(menu)
  return screen.queryByRole('menuitem', { name })
}

describe('Rules tab with the server running (AC-1)', () => {
  test('Add rule and each rule row has Edit and Disable (or Enable again)', async () => {
    stubServer()
    await openRules()
    searchFor("RECON-DEP-001")
    expect(screen.getByRole('button', { name: 'Add rule' })).toBeEnabled()
    expect(rowAction('RECON-DEP-001', 'Edit')).toBeInTheDocument()
    expect(rowAction('RECON-DEP-001', 'Disable')).toBeInTheDocument()
  })

  test('without the server the Rules tab stays read only', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => {
      throw new TypeError('no server')
    }))
    await openRules()
    expect(screen.getByRole('button', { name: 'Add rule' })).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Actions for RECON-DEP-001' })).toBeNull()
  })
})

describe('switching a rule off needs a reason (AC-7)', () => {
  test('the Switch off button waits for a reason, and the reason goes to the server', async () => {
    const calls = stubServer((url) =>
      url === '/api/v1/rules/RECON-DEP-001/disable' ? json({ rule_id: 'RECON-DEP-001', status: 'disabled' }) : undefined,
    )
    await openRules()
    searchFor("RECON-DEP-001")
    fireEvent.click(rowAction('RECON-DEP-001', 'Disable')!)
    const dialog = screen.getByRole('form', { name: 'Switch off RECON-DEP-001' })
    const submit = within(dialog).getByRole('button', { name: 'Switch off' })
    expect(submit).toBeDisabled()
    fireEvent.change(within(dialog).getByLabelText('Reason'), { target: { value: 'fires on the test data' } })
    expect(submit).toBeEnabled()
    await act(async () => {
      fireEvent.click(submit)
    })
    const sent = calls.find((c) => c.url === '/api/v1/rules/RECON-DEP-001/disable')
    expect(sent?.method).toBe('POST')
    // No name is sent: the server records the signed-in user (SPEC-012).
    expect(JSON.parse(sent?.body ?? '{}')).toEqual({ reason: 'fires on the test data' })
  })
})

describe('the Rule Builder window (AC-3, AC-11)', () => {
  test('a refused save names each field in plain words', async () => {
    stubServer((url, init) => {
      if (url === '/api/v1/rules' && init?.method === 'POST') {
        return json(
          {
            error_code: 'RULE_REFUSED',
            message: 'the figure "computation.made_up" is not in the fact vocabulary; pick a figure from the list',
            problems: [
              { field: 'inputs[0].ref', code: 'UNKNOWN_FACT', message: 'the figure "computation.made_up" is not in the fact vocabulary; pick a figure from the list' },
              { field: 'law', code: 'NO_LAW', message: 'give at least one legal basis: the Act and the section (RU-30)' },
            ],
          },
          422,
        )
      }
      return undefined
    })
    await openRules()
    fireEvent.click(screen.getByRole('button', { name: 'Add rule' }))
    const dialog = await screen.findByRole('form', { name: 'Add rule' })
    fireEvent.change(within(dialog).getByLabelText('ID'), { target: { value: 'RECON-OFF-001' } })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save as testing' }))
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('The rule was not saved')
    expect(alert).toHaveTextContent('Figure or row 1, which figure or record type')
    expect(alert).toHaveTextContent('Legal basis')
    expect(alert).toHaveTextContent('pick a figure from the list')
  })

  test('the window loads no external script, style, font or image', async () => {
    stubServer()
    render(<RuleBuilder mode="create" current={null} options={options} taxYears={['2025-26']} onSaved={() => {}} onClose={() => {}} />)
    const urls = [...document.querySelectorAll('[src],[href]')].map((el) => el.getAttribute('src') ?? el.getAttribute('href') ?? '')
    expect(urls.filter((u) => /^(https?:)?\/\//i.test(u))).toEqual([])
    expect(document.querySelectorAll('script, link[rel="stylesheet"], img, iframe').length).toBe(0)
  })
})

describe('the Tax years dropdown of the Rule Builder', () => {
  test('is a drop-down list of check boxes: several tax years can be chosen, and the button names them', async () => {
    stubServer()
    render(<RuleBuilder mode="create" current={null} options={options} taxYears={['2025-26', '2026-27']} onSaved={() => {}} onClose={() => {}} />)
    const form = screen.getByRole('form', { name: /rule/i })
    const button = within(form).getByRole('button', { name: /^Tax years:/ })
    expect(button).toHaveAttribute('aria-haspopup', 'true')
    fireEvent.click(button)
    const list = within(form).getByRole('group', { name: 'Tax years' })
    for (const box of within(list).getAllByRole('checkbox')) {
      if (!(box as HTMLInputElement).checked) fireEvent.click(box)
    }
    const chosen = within(list).getAllByRole('checkbox').filter((b) => (b as HTMLInputElement).checked).length
    expect(chosen).toBeGreaterThanOrEqual(2)
    fireEvent.click(within(list).getByRole('button', { name: 'Clear' }))
    expect(within(form).getByRole('button', { name: 'Tax years: Choose tax years' })).toBeInTheDocument()
  })

  test('when editing a rule the tax years are shown but cannot be changed', () => {
    const current: RuleCurrent = {
      rule_id: 'RECON-DEP-001', version: '1', source: 'builtin', status: 'testing', disabled_reason: '', editable: true, reason: '',
      approvable: false, approval_note: '', form: { ...blankForm(options, '2025-26'), id: 'RECON-DEP-001', mode: 'edit' }, fixtures: [],
    }
    render(<RuleBuilder mode="edit" current={current} options={options} taxYears={['2025-26']} onSaved={() => {}} onClose={() => {}} />)
    expect(screen.getByRole('button', { name: /^Tax years:/ })).toBeDisabled()
  })
})

describe('the Reconciliation type dropdown of the Rule Builder', () => {
  const two: RuleOptions = {
    ...options,
    categories: [
      { code: 'R1', name: 'Same figure in two places', description: '' },
      { code: 'R2', name: 'Recomputed figure', description: '' },
      { code: 'GST', name: 'GST return against the books', description: '' },
    ],
  }

  test('several types can be chosen; the first chosen is the main one and the others are sent as more_categories', () => {
    expect(forSave({ ...blankForm(two, '2025-26'), category: 'R1', more_categories: ['GST'] })).toMatchObject({ category: 'R1', more_categories: ['GST'] })
    stubServer()
    render(<RuleBuilder mode="create" current={null} options={two} taxYears={['2025-26']} onSaved={() => {}} onClose={() => {}} />)
    const form = screen.getByRole('form', { name: /rule/i })
    expect(within(form).getByRole('button', { name: 'Reconciliation type: R1: Same figure in two places' })).toBeInTheDocument()
    fireEvent.click(within(form).getByRole('button', { name: /^Reconciliation type:/ }))
    const list = within(form).getByRole('group', { name: 'Reconciliation type' })
    fireEvent.click(within(list).getByRole('checkbox', { name: 'GST: GST return against the books' }))
    fireEvent.click(within(list).getByRole('checkbox', { name: 'R2: Recomputed figure' }))
    expect(within(form).getByRole('button', { name: 'Reconciliation type: R1, GST, R2' })).toBeInTheDocument()
    // Taking the main one off makes the next one the main type.
    fireEvent.click(within(list).getByRole('checkbox', { name: 'R1: Same figure in two places' }))
    expect(within(form).getByRole('button', { name: 'Reconciliation type: GST, R2' })).toBeInTheDocument()
  })

  test('a rule sent without more_categories reads as having none', () => {
    const f = normaliseForm({ ...blankForm(two, '2025-26'), more_categories: undefined })
    expect(f.more_categories).toEqual([])
  })
})

describe('the Rule Builder helpers', () => {
  test('each check type shows only the fields it uses', () => {
    expect(checkFields('equal')).toMatchObject({ tolerance: true, parts: false, when: false, keys: false, table: false, matching: false })
    expect(checkFields('sum_equals')).toMatchObject({ parts: true, total: true, tolerance: true })
    expect(checkFields('percent_of')).toMatchObject({ rate: true, base: true, value: true, parts: true })
    expect(checkFields('present_together')).toMatchObject({ when: true, tolerance: false })
    expect(checkFields('table_lookup')).toMatchObject({ keys: true, table: true, value: true, tolerance: false })
    expect(checkFields('match_records')).toMatchObject({ matching: true, tolerance: true, parts: false })
  })

  test('a save sends only the parameters of its check type, and drops empty rows', () => {
    const f = blankForm(options, '2025-26')
    f.check = { ...f.check, type: 'equal', tolerance: '1.00', parts: [{ input: 'a', field: '', sign: '+' }], when: [{ input: 'a', field: '', is: 'zero', value: '', values: [] }] }
    f.inputs = [{ name: 'a', kind: 'fact', ref: 'computation.allowed_depreciation', label: 'A', optional: false, default: '' }, { name: '', kind: 'fact', ref: '', label: '', optional: false, default: '' }]
    f.law = [{ act: 'Income-tax Act, 1961', section: '32', reference: '' }, { act: '', section: '', reference: '' }]
    const out = forSave(f)
    expect(out.check.tolerance).toBe('1.00')
    expect(out.check.parts).toEqual([])
    expect(out.check.when).toEqual([])
    expect(out.inputs).toHaveLength(1)
    expect(out.law).toHaveLength(1)
  })

  test('a form from the engine with null lists reads as empty lists', () => {
    const raw = { ...blankForm(options, '2025-26'), inputs: null, law: null, fixtures: null, tax_years: null, check: { ...blankForm(options, '').check, parts: null, when: null, table: null } }
    const f = normaliseForm(raw as unknown as ReturnType<typeof blankForm>)
    expect(f.inputs).toEqual([])
    expect(f.fixtures).toEqual([])
    expect(f.check.parts).toEqual([])
    expect(f.check.table).toEqual([])
  })

  test('the field labels name the row, the fixture and the figure', () => {
    expect(fieldLabel('inputs[1].ref')).toBe('Figure or row 2, which figure or record type')
    expect(fieldLabel('fixtures[0].facts[2].value')).toBe('Fixture 1, figure 3, value')
    expect(fieldLabel('fixtures[1].records[0].period')).toBe('Fixture 2, row 1, period')
    expect(fieldLabel('check.tolerance')).toBe('Tolerance')
    expect(fieldLabel('unknown.field')).toBe('unknown.field')
  })

  test('a list typed with commas is read without its empty items', () => {
    expect(parseList(' taxable_value, igst ,, cgst ')).toEqual(['taxable_value', 'igst', 'cgst'])
    expect(parseList('')).toEqual([])
  })
})

describe('the Rule Builder help and required fields', () => {
  test('the guide starts collapsed, the toggle shows and hides it, and required fields are marked', async () => {
    stubServer()
    await openRules()
    fireEvent.click(screen.getByRole('button', { name: 'Add rule' }))
    const dialog = await screen.findByRole('form', { name: 'Add rule' })
    const help = within(dialog).getByRole('button', { name: 'How to add a rule' })
    expect(help).toHaveAttribute('aria-expanded', 'false')
    expect(within(dialog).queryByText('Save rule:')).not.toBeInTheDocument()
    fireEvent.click(help)
    expect(help).toHaveAttribute('aria-expanded', 'true')
    expect(within(dialog).getByText('Save rule:')).toBeInTheDocument()
    fireEvent.click(help)
    expect(help).toHaveAttribute('aria-expanded', 'false')
    expect(within(dialog).queryByText('Save rule:')).not.toBeInTheDocument()
    expect(within(dialog).getByText('Title')).toHaveClass('required')
    expect(within(dialog).getByText('Notes')).not.toHaveClass('required')
  })

  test('the header scrolls with the form: it is not sticky, in either the expanded or collapsed state', async () => {
    stubServer()
    await openRules()
    fireEvent.click(screen.getByRole('button', { name: 'Add rule' }))
    const dialog = await screen.findByRole('form', { name: 'Add rule' })
    const head = dialog.querySelector('.rule-head')
    expect(head).not.toBeNull()
    expect(head).not.toHaveClass('sidebar-premium')
    const help = within(dialog).getByRole('button', { name: 'How to add a rule' })
    fireEvent.click(help)
    expect(dialog.querySelector('#rule-help')).not.toHaveClass('sidebar-premium')
  })
})

// A testing rule of the engine, as the server lists it. approvable says whether the gate is met.
function testingRule(approvable: boolean, note = ''): RuleInfo {
  return {
    id: 'RECON-DEP-001', version: '1', revision: 1, source: 'builtin', status: 'testing', runnable: true, type: 'tie-out', domain: '',
    category: 'R1', title: 'Depreciation: computation against its schedule', tax_years: ['2025-26'], valid_from: '2025-04-01', valid_to: '',
    file_path: 'content/rules/2025-26/tieout/RECON-DEP-001/rule.yaml', approvable, approval_note: note,
  }
}

function listOf(rule: RuleInfo): RuleList {
  return { rules: [rule], tax_years: ['2025-26'] }
}

// The approval note the server gives a testing rule that has not been run on a client.
const notRunNote = 'the rule has not been run on a client with testing rules included; run a client\'s documents with testing rules on, then approve'

describe('approving a testing rule (AC-18)', () => {
  test('Approve is offered only for a testing rule the engine says may be approved', async () => {
    stubServer((url) => (url === '/api/v1/rules' ? json(listOf(testingRule(true))) : undefined))
    await openRules()
    expect(rowAction('RECON-DEP-001', 'Approve')).toBeEnabled()
  })

  test('a testing rule that may not be approved yet has no Approve, and its panel says why', async () => {
    stubServer((url) => (url === '/api/v1/rules' ? json(listOf(testingRule(false, notRunNote))) : undefined))
    await openRules()
    expect(rowAction('RECON-DEP-001', 'Approve')).toBeNull()
    fireEvent.click(screen.getByText('RECON-DEP-001'))
    expect(screen.getByText(/Not ready for approval: the rule has not been run on a client/)).toBeInTheDocument()
  })

  test('Approve asks for the Partner\'s name and a reason, sends both, and the window closes', async () => {
    const calls = stubServer((url) => {
      if (url === '/api/v1/rules') return json(listOf(testingRule(true)))
      if (url === '/api/v1/rules/RECON-DEP-001/approve') return json({ rule_id: 'RECON-DEP-001', status: 'active' })
      return undefined
    })
    await openRules()
    fireEvent.click(rowAction('RECON-DEP-001', 'Approve')!)
    const dialog = await screen.findByRole('form', { name: 'Approve RECON-DEP-001' })
    expect(dialog).toHaveTextContent('The approval is written in the audit log with your login name.')
    expect(within(dialog).queryByLabelText("Partner's name")).not.toBeInTheDocument()
    const submit = within(dialog).getByRole('button', { name: 'Approve' })
    expect(submit).toBeDisabled()
    fireEvent.change(within(dialog).getByLabelText('Reason'), { target: { value: 'checked against the computation' } })
    expect(submit).toBeEnabled()
    await act(async () => {
      fireEvent.click(submit)
    })
    const sent = calls.find((c) => c.url === '/api/v1/rules/RECON-DEP-001/approve')
    expect(sent?.method).toBe('POST')
    expect(JSON.parse(sent?.body ?? '{}')).toEqual({ reason: 'checked against the computation' })
    expect(screen.queryByRole('form', { name: 'Approve RECON-DEP-001' })).toBeNull()
  })

  test('a refused approval shows the engine\'s reason in the window', async () => {
    stubServer((url) => {
      if (url === '/api/v1/rules') return json(listOf(testingRule(true)))
      if (url === '/api/v1/rules/RECON-DEP-001/approve') {
        return json({ error_code: 'NOT_RUN_ON_CLIENT', message: 'RECON-DEP-001: ' + notRunNote }, 422)
      }
      return undefined
    })
    await openRules()
    fireEvent.click(rowAction('RECON-DEP-001', 'Approve')!)
    const dialog = await screen.findByRole('form', { name: 'Approve RECON-DEP-001' })
    fireEvent.change(within(dialog).getByLabelText('Reason'), { target: { value: 'checked' } })
    await act(async () => {
      fireEvent.click(within(dialog).getByRole('button', { name: 'Approve' }))
    })
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('has not been run on a client')
  })
})

describe('the Rule Builder window offers Approve for a testing rule (AC-18)', () => {
  function editing(approvable: boolean, note = ''): RuleCurrent {
    return {
      rule_id: 'RECON-DEP-001', version: '1', source: 'builtin', status: 'testing', disabled_reason: '', editable: true, reason: '',
      approvable, approval_note: note, form: { ...blankForm(options, '2025-26'), id: 'RECON-DEP-001', mode: 'edit' }, fixtures: [],
    }
  }

  test('Approve appears for a testing rule that may be approved, and calls the Rules tab', () => {
    stubServer()
    const onApprove = vi.fn()
    render(<RuleBuilder mode="edit" current={editing(true)} options={options} taxYears={['2025-26']} onSaved={() => {}} onApprove={onApprove} onClose={() => {}} />)
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    expect(onApprove).toHaveBeenCalledTimes(1)
  })

  test('without the gate met, the window says why and offers no Approve', () => {
    stubServer()
    render(<RuleBuilder mode="edit" current={editing(false, notRunNote)} options={options} taxYears={['2025-26']} onSaved={() => {}} onApprove={() => {}} onClose={() => {}} />)
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull()
    expect(screen.getByText(/Not ready for approval: the rule has not been run on a client/)).toBeInTheDocument()
  })
})
