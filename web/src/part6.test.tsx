import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import App from './App.tsx'
import { filterResults, filterRules, noResultFilter, noRuleFilter, resultRows, ruleYearMatch, taxYearOf } from './filters.ts'
import { sampleRules, sampleRuns } from './sample/index.ts'
import type { RuleInfo } from './types.ts'
import { bannedIn } from './wording.ts'
import { openBoardClient, openFirstClient, railTo, showFinding, showResults, showView } from './testFindings.ts'

// SPEC-004 part 6: filters (AC-17) and the upload panel (AC-15, AC-16) in the browser.

const rules = sampleRules.rules

const rule = (over: Partial<RuleInfo>): RuleInfo => ({
  id: 'X-001', version: '1', revision: 1, source: 'rulebook', status: 'disabled', runnable: false, type: 'statutory',
  domain: 'gst', category: 'C', title: 'T', tax_years: [], valid_from: '', valid_to: '', file_path: '', approvable: false, approval_note: '', ...over,
})

// The engine's own rules (source builtin): every one loads as testing until the CA approves it.
const builtin = rules.filter((r) => r.source === 'builtin').map((r) => r.id).sort()

describe('rule filters', () => {
  test('no filter shows every rule', () => {
    expect(rules.length).toBeGreaterThan(304)
    expect(filterRules(rules, noRuleFilter)).toHaveLength(rules.length)
  })

  test('each filter narrows, and together they combine', () => {
    const gst = filterRules(rules, { ...noRuleFilter, domain: ['gst'] })
    expect(gst.length).toBeGreaterThan(0)
    expect(gst.every((r) => r.domain === 'gst')).toBe(true)
    const recon = filterRules(rules, { ...noRuleFilter, domain: ['gst'], type: ['reconciliation'] })
    expect(recon.length).toBeLessThan(gst.length)
    expect(recon.every((r) => r.domain === 'gst' && r.type === 'reconciliation')).toBe(true)
    const cat = recon[0].category
    expect(filterRules(rules, { ...noRuleFilter, category: [cat] }).every((r) => r.category === cat)).toBe(true)
    expect(builtin).toContain('RECON-DEP-001')
    expect(filterRules(rules, { ...noRuleFilter, status: ['testing'] }).map((r) => r.id).sort()).toEqual(builtin)
    expect(filterRules(rules, { ...noRuleFilter, source: ['builtin'] })).toHaveLength(builtin.length)
    expect(filterRules(rules, { ...noRuleFilter, search: 'rc-gst-001' }).map((r) => r.id)).toEqual(['RC-GST-001'])
  })

  test('tax year: named years, validity dates, and rules that are not dated', () => {
    expect(ruleYearMatch(rule({ tax_years: ['2025-26'] }), '2025-26')).toBe('named')
    expect(ruleYearMatch(rule({ tax_years: ['2025-26'] }), '2024-25')).toBe('no')
    expect(ruleYearMatch(rule({ valid_from: '2025-04-01' }), '2025-26')).toBe('dated')
    expect(ruleYearMatch(rule({ valid_from: '2026-04-01' }), '2025-26')).toBe('no')
    expect(ruleYearMatch(rule({ valid_to: '2025-03-31' }), '2025-26')).toBe('no')
    expect(ruleYearMatch(rule({}), '2025-26')).toBe('not_dated')
    expect(taxYearOf('2026-03-31')).toBe('2025-26')
    expect(taxYearOf('2025-04-01')).toBe('2025-26')
    // RECON rules name 2025-26 only; for 2024-25 they drop out, undated rule-store rules stay.
    const y2425 = filterRules(rules, { ...noRuleFilter, taxYear: ['2024-25'] })
    expect(y2425.some((r) => r.id === 'RECON-DEP-001')).toBe(false)
    expect(y2425.length).toBeGreaterThan(200)
  })
})

describe('result filters', () => {
  const rows = resultRows(sampleRuns.differs.report)

  // The sample computations hold the depreciation figures only, so one rule fails (differs)
  // or passes (clean) and every other engine rule cannot be checked.
  test('every result is a row: fail, cannot check and pass', () => {
    const others = builtin.filter((id) => id !== 'RECON-DEP-001').map((id) => `cannot_check:${id}`)
    expect(rows.map((r) => `${r.status}:${r.ruleId}`).sort()).toEqual([...others, 'fail:RECON-DEP-001'].sort())
    const clean = resultRows(sampleRuns.clean.report)
    expect(clean.map((r) => `${r.status}:${r.ruleId}`).sort()).toEqual([...others, 'pass:RECON-DEP-001'].sort())
  })

  test('status, rule, category, document and search', () => {
    expect(filterResults(rows, { ...noResultFilter, status: 'fail' }).map((r) => r.ruleId)).toEqual(['RECON-DEP-001'])
    expect(filterResults(rows, { ...noResultFilter, ruleId: 'RECON-DEP-002' }).map((r) => r.status)).toEqual(['cannot_check'])
    const r1 = filterResults(rows, { ...noResultFilter, category: 'R1' })
    expect(r1.length).toBeGreaterThan(0)
    expect(r1.length).toBeLessThan(rows.length)
    expect(r1.every((r) => r.category === 'R1')).toBe(true)
    const tar = filterResults(rows, { ...noResultFilter, document: 'Tax audit report (Form 3CA/3CB with 3CD)' })
    expect(tar.map((r) => r.ruleId)).toContain('RECON-DEP-002')
    expect(tar.every((r) => r.documents.includes('Tax audit report (Form 3CA/3CB with 3CD)'))).toBe(true)
    expect(filterResults(rows, { ...noResultFilter, search: 'fixed-asset' }).map((r) => r.ruleId)).toEqual(['RECON-DEP-001'])
    expect(filterResults(rows, { ...noResultFilter, status: 'pass' })).toHaveLength(0)
  })
})

describe('Rules view in the browser', () => {
  test('pages: 10 at a time by default; 5, 10, 15 or 30 can be chosen; a filter goes back to page 1', () => {
    render(<App />)
    railTo('Rules')
    const list = screen.getByRole('table', { name: 'Rules list' })
    const pages = screen.getByRole('group', { name: 'Pages of rules' })
    expect(within(list).getAllByRole('row')).toHaveLength(11)
    // The page bar sits under the list, as on every screen.
    expect(list.compareDocumentPosition(pages) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(pages).toHaveTextContent(`1–10 of ${rules.length}`)
    const size = screen.getByLabelText('Rules per page')
    expect([...(size as HTMLSelectElement).options].map((o) => o.value)).toEqual(['5', '10', '15', '30'])
    fireEvent.change(size, { target: { value: '5' } })
    expect(within(list).getAllByRole('row')).toHaveLength(6)
    fireEvent.click(within(pages).getByRole('button', { name: 'Next' }))
    expect(pages).toHaveTextContent(`6–10 of ${rules.length}`)
    expect(within(list).getAllByRole('row')[1]).toHaveTextContent(rules[5].id)
    fireEvent.click(within(pages).getByRole('button', { name: 'Next' }))
    expect(pages).toHaveTextContent(`11–15 of ${rules.length}`)
    fireEvent.change(screen.getByLabelText('Search rules'), { target: { value: 'ADV' } })
    expect(pages).toHaveTextContent('1–5 of')
    fireEvent.change(size, { target: { value: '10' } })
  })

  test('filters, the search box, and Clear filters', () => {
    render(<App />)
    railTo('Rules')
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Rules')
    expect(screen.getByText(`${rules.length} of ${rules.length} rules`)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /^Subject:/ }))
    fireEvent.click(screen.getByRole('checkbox', { name: /payroll/i }))
    fireEvent.keyDown(screen.getByRole('group', { name: 'Subject' }), { key: 'Escape' })
    const payroll = filterRules(rules, { ...noRuleFilter, domain: ['payroll'] }).length
    expect(screen.getByText(`${payroll} of ${rules.length} rules`)).toBeInTheDocument()
    const search = screen.getByLabelText('Search rules')
    fireEvent.change(search, { target: { value: 'no-such-rule' } })
    expect(screen.getByText(`0 of ${rules.length} rules`)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }))
    expect(screen.getByText(`${rules.length} of ${rules.length} rules`)).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Rules')
    const list = screen.getByRole('table', { name: 'Rules list' })
    fireEvent.click(screen.getByRole('button', { name: /^Status:/ }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Switched off' }))
    expect(screen.getByText(`304 of ${rules.length} rules`)).toBeInTheDocument()
    expect(within(list).getAllByText('Described only, not run yet')).toHaveLength(10) // 10 a page
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }))
    // The screen's own wording has no banned phrase. Rule-book names are shown as the CA wrote
    // them; the two that use one are on the CA's problem list (rulestore-problems).
    const page = document.body.cloneNode(true) as HTMLElement
    page.querySelector('table[aria-label="Rules list"]')?.remove()
    expect(bannedIn(page.textContent ?? '')).toEqual([])
  }, 20_000) // renders the 312-rule list several times; slow when the whole suite runs at once
})

describe('Upload and run', () => {
  afterEach(() => vi.unstubAllGlobals())

  const json = (body: unknown, status = 200) =>
    new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })
  const client1 = { id: 1, name: 'Sample Engineering Works', pan_last4: '678M', last_run_on: '', documents: 0, counts: null }

  // serverWith answers like recon-server with one client in the database.
  function serverWith(runResponse: () => Response) {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) => {
        if (url === '/api/v1/rules') return json(sampleRules)
        if (url.startsWith('/api/v1/clients?')) return json({ tax_year: '2025-26', clients: [client1] })
        if (url.startsWith('/api/v1/clients/1/result')) return json({ client: client1, report: null, facts: { engagements: [] } })
        return runResponse()
      }),
    )
  }

  async function upload(name: string) {
    render(<App />)
    await act(async () => {}) // the rules and clients requests
    await act(async () => {})
    await act(async () => {
      openFirstClient() // open the client, which loads its latest result
    })
    const input = screen.getByLabelText('Files to check')
    fireEvent.change(input, { target: { files: [new File(['{}'], name, { type: 'application/json' })] } })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Run checks' }))
    })
  }

  test('AC-15: the result of an upload is shown with both figures and sources', async () => {
    const report = sampleRuns.differs.report
    const result = { exit_code: 1, report, results: [{ comparison_id: '', last_run_on: '', report }], facts: sampleRuns.differs.facts }
    serverWith(() => json(result))
    await upload('comp.json')
    expect(screen.getByText(/Result of the files you uploaded/)).toBeInTheDocument()
    const card = showFinding(/RECON-DEP-001/)
    expect(card).toHaveTextContent('1,15,200.50')
    expect(card).toHaveTextContent('2,750.00')
    showResults('Cannot check')
    expect(screen.queryByRole('article', { name: /RECON-DEP-001/ })).toBeNull()
    const n = resultRows(sampleRuns.differs.report).length
    expect(screen.getByText(`${n - 1} of ${n} results`)).toBeInTheDocument()
  })

  test('AC-16: a refused file shows its code and the file name', async () => {
    serverWith(
      () =>
        new Response(JSON.stringify({ error_code: 'PROFILE_NO_MATCH', message: 'PROFILE_NO_MATCH: balance_sheet.json: the file matches no known json format' }), {
          status: 422,
          headers: { 'content-type': 'application/json' },
        }),
    )
    await upload('balance_sheet.json')
    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('PROFILE_NO_MATCH')
    expect(alert).toHaveTextContent('balance_sheet.json')
  })

  test('without a server the panel says so and stays off', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => Promise.reject(new Error('offline'))))
    render(<App />)
    await act(async () => {})
    openFirstClient()
    fireEvent.click(screen.getByRole('button', { name: 'Upload and run' }))
    expect(screen.getByLabelText('Files to check')).toBeDisabled()
    expect(screen.getByText(/start recon-server to upload files/)).toBeInTheDocument()
  })
})

describe('Clients (part 7)', () => {
  afterEach(() => vi.unstubAllGlobals())
  const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status, headers: { 'content-type': 'application/json' } })

  test('AC-21: add a client from the Season board', async () => {
    const added = { id: 7, name: 'New Client LLP', pan_last4: '234F', last_run_on: '', documents: 0, counts: null }
    let clients: unknown[] = []
    let posted: { tax_year?: string; details?: Record<string, unknown> } = {}
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string, init?: RequestInit) => {
        if (url === '/api/v1/rules') return json(sampleRules)
        if (url.startsWith('/api/v1/clients?')) return json({ tax_year: '2025-26', clients })
        if (url === '/api/v1/clients' && init?.method === 'POST') {
          posted = JSON.parse(String(init.body))
          clients = [added]
          return json({ client: added }, 201)
        }
        if (url.startsWith('/api/v1/clients/7/result')) return json({ client: added, report: null, results: [], facts: { engagements: [] } })
        throw new Error('unexpected ' + url)
      }),
    )
    render(<App />)
    await act(async () => {})
    await act(async () => {})
    expect(screen.getByText(/No clients yet/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '+ Add client' }))
    fireEvent.change(screen.getByLabelText('Legal name'), { target: { value: 'New Client LLP' } })
    fireEvent.change(screen.getByLabelText('PAN'), { target: { value: 'abcde1234f' } })
    expect(screen.getByLabelText('PAN')).toHaveValue('ABCDE1234F')
    fireEvent.change(screen.getByLabelText('TAN'), { target: { value: 'PNEA12345B' } })
    fireEvent.click(screen.getByRole('button', { name: '+ Add GSTIN' }))
    fireEvent.change(screen.getByLabelText('GSTIN 1'), { target: { value: '27abcde1234f1zx' } })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Save client' }))
    })
    await act(async () => {})
    expect(posted.tax_year).toBe('2025-26')
    expect(posted.details).toMatchObject({
      legal_name: 'New Client LLP', pan: 'ABCDE1234F', tan: 'PNEA12345B',
      gstins: [{ gstin: '27ABCDE1234F1ZX', registration_type: 'regular', primary: true }],
    })
    expect(screen.getByRole('heading', { level: 1, name: /New Client LLP/ })).toBeInTheDocument()
    // A new client has no run yet, so the upload form opens over its file.
    expect(screen.getByRole('dialog', { name: 'Upload and run' })).toBeInTheDocument()
    expect(document.body.textContent).not.toContain('ABCDE1234F')
  })

  test('a refused client shows its code', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string, init?: RequestInit) => {
        if (url === '/api/v1/rules') return json(sampleRules)
        if (url.startsWith('/api/v1/clients?')) return json({ tax_year: '2025-26', clients: [] })
        if (init?.method === 'POST') return json({ error_code: 'CLIENT_EXISTS', message: 'a client with this PAN already exists (client 1)' }, 409)
        throw new Error('unexpected ' + url)
      }),
    )
    render(<App />)
    await act(async () => {})
    await act(async () => {})
    fireEvent.click(screen.getByRole('button', { name: '+ Add client' }))
    fireEvent.change(screen.getByLabelText('Legal name'), { target: { value: 'Again' } })
    fireEvent.change(screen.getByLabelText('PAN'), { target: { value: 'AAKFS5678M' } })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Save client' }))
    })
    expect(screen.getByRole('alert')).toHaveTextContent('CLIENT_EXISTS')
  })

  test('the client form opens over the screen with focus in it; its close button closes it', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) => {
        if (url === '/api/v1/rules') return json(sampleRules)
        if (url.startsWith('/api/v1/clients?')) return json({ tax_year: '2025-26', clients: [] })
        throw new Error('unexpected ' + url)
      }),
    )
    render(<App />)
    await act(async () => {})
    await act(async () => {})
    const button = screen.getByRole('button', { name: '+ Add client' })
    button.focus()
    fireEvent.click(button)
    const dialog = screen.getByRole('dialog', { name: 'Add client' })
    expect(within(dialog).getByRole('form', { name: 'Add client' })).toBeInTheDocument()
    expect(document.activeElement).toBe(screen.getByLabelText('Legal name'))
    fireEvent.click(within(dialog).getByRole('button', { name: 'Close add client' }))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(screen.getByRole('heading', { name: /Season board/ })).toBeInTheDocument()
  })

  test("AC-22: the client's accounts compared with our computation (sample data)", () => {
    render(<App />)
    openBoardClient('Sample Engineering Works')
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Sample Engineering Works')
    const card = showFinding(/RECON-PL-001/)
    for (const text of ['Accounts, profit before tax', '12,63,540.00', 'Computation, "Net profit as per P&L"', '12,36,540.00', '27,000.00',
      'fs_2025-26_differs.json $.profit_and_loss.profit_before_tax', 'comp_v3_2025-26_differs.json $.income.business_profession.net_profit_as_per_pl']) {
      expect(card).toHaveTextContent(text)
    }
  })
})

describe('GST invoice matching and the client form (part 8)', () => {
  afterEach(() => vi.unstubAllGlobals())

  test('AC-27: each invoice difference shows both values, the difference and the sources (sample data)', () => {
    render(<App />)
    openBoardClient('Deccan Byte Systems Pvt Ltd')
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Deccan Byte Systems Pvt Ltd')
    expect(screen.getByRole('tab', { selected: true })).toHaveTextContent('GST: Sales register vs GSTR-1')
    const card = showFinding(/RECON-GST-001 .* DBS\/26-27\/0462/)
    for (const text of ['Amounts differ', 'taxable value', '4,50,000.00', '4,05,000.00', '45,000.00',
      'SALES_REG_Aug2026.json $.rows[14]', 'GSTR1_Aug2026.json $.b2b[6].inv[0]']) {
      expect(card).toHaveTextContent(text)
    }
    expect(showFinding(/DBS\/26-27\/0458/)).toHaveTextContent('In Sales register only')
    expect(showFinding(/DBS\/26-27\/0453/)).toHaveTextContent('More than once in GSTR-1')
    expect(screen.getByLabelText('Totals')).toHaveTextContent('Taxable value')
    showView('Documents')
    expect(screen.getByLabelText('Documents of the comparison')).toHaveTextContent('Read')
    showView('Findings')
    expect(bannedIn(document.body.textContent ?? '')).toEqual([])
  })

  test('AC-25: a refused client shows each field and its reason', async () => {
    const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status, headers: { 'content-type': 'application/json' } })
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string, init?: RequestInit) => {
        if (url === '/api/v1/rules') return json(sampleRules)
        if (url.startsWith('/api/v1/clients?')) return json({ tax_year: '2025-26', clients: [] })
        if (url === '/api/v1/comparisons') return json({ comparisons: [] })
        if (init?.method === 'POST') {
          return json({ error_code: 'INVALID_CLIENT', message: 'the check character is wrong', problems: [
            { field: 'gstins[0]', code: 'GSTIN_CHECK_DIGIT', message: 'the check character of 27ZLVCD9565Z1ZG is wrong; the GSTIN is mistyped' },
            { field: 'tan', code: 'BAD_TAN', message: 'a TAN is four letters, five digits and one letter' },
          ] }, 422)
        }
        throw new Error('unexpected ' + url)
      }),
    )
    render(<App />)
    await act(async () => {})
    await act(async () => {})
    fireEvent.click(screen.getByRole('button', { name: '+ Add client' }))
    fireEvent.change(screen.getByLabelText('Legal name'), { target: { value: 'Deccan Byte Systems Pvt Ltd' } })
    fireEvent.change(screen.getByLabelText('PAN'), { target: { value: 'ZLVCD9565Z' } })
    fireEvent.click(screen.getByRole('button', { name: '+ Add GSTIN' }))
    fireEvent.change(screen.getByLabelText('GSTIN 1'), { target: { value: '27ZLVCD9565Z1ZG' } })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Save client' }))
    })
    expect(screen.getByRole('alert')).toHaveTextContent('INVALID_CLIENT')
    expect(screen.getByRole('alert')).toHaveTextContent('tan: a TAN is four letters')
    expect(document.body).toHaveTextContent('the check character of 27ZLVCD9565Z1ZG is wrong')
  })
})

describe('Editing a client, and a comparison with a document missing (part 8)', () => {
  afterEach(() => vi.unstubAllGlobals())
  const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status, headers: { 'content-type': 'application/json' } })

  test('AC-25 and AC-26: edit the client; the comparison shows the GSTR-1 as not uploaded yet', async () => {
    const client = { id: 3, name: 'Deccan Byte Systems Pvt Ltd', pan_last4: '565Z', last_run_on: '2026-10-15', documents: 1, counts: null }
    const details = {
      legal_name: client.name, trade_name: 'Deccan Byte', pan_last4: '565Z', constitution: 'company', date_of_incorporation: '',
      cin: '', tan: '', gstins: [{ gstin: '27ZLVCD9565Z1ZF', registration_type: 'regular', primary: true }],
      address: { line1: '', line2: '', city: 'Pune', state: '27', pin: '' }, contact_person: '', email: '', phone: '',
      accounting_software: '', notes: '',
    }
    // The sales register is uploaded; the GSTR-1 is not.
    const sales = sampleRuns['gst-sales']
    const report = structuredClone(sales.report)
    report.comparison!.documents = report.comparison!.documents.map((d) => ({ ...d, present: d.document_type !== 'gstr1' }))
    let put: { details?: Record<string, unknown> } = {}
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string, init?: RequestInit) => {
        if (url === '/api/v1/rules') return json(sampleRules)
        if (url === '/api/v1/comparisons') return json({ comparisons: [] })
        if (url.startsWith('/api/v1/clients?')) return json({ tax_year: '2025-26', clients: [client] })
        if (url.startsWith('/api/v1/clients/3/result')) {
          return json({ client, report, results: [{ comparison_id: 'gst_sales_vs_gstr1', last_run_on: '2026-10-15', report }], facts: sales.facts })
        }
        if (url === '/api/v1/clients/3' && init?.method === 'PUT') {
          put = JSON.parse(String(init.body))
          return json({ details: { ...details, ...put.details, pan: undefined } })
        }
        if (url === '/api/v1/clients/3') return json({ details })
        throw new Error('unexpected ' + url)
      }),
    )
    render(<App />)
    await act(async () => {})
    await act(async () => {})
    openFirstClient()
    await act(async () => {})
    await act(async () => {})
    showView('Documents')
    const docs = screen.getByLabelText('Documents of the comparison')
    const [salesDoc, gstr1Doc] = within(docs).getAllByRole('listitem')
    expect(salesDoc).toHaveTextContent(/^Read Sales register/)
    expect(gstr1Doc).toHaveTextContent(/^Not uploaded yet GSTR-1/)

    expect(screen.getByLabelText('Client details')).toHaveTextContent('PAN ending 565Z')
    fireEvent.click(screen.getByRole('button', { name: 'Edit client' }))
    expect(screen.getByRole('form', { name: 'Edit client' })).toBeInTheDocument()
    expect(screen.getByLabelText('PAN')).toHaveValue('')
    fireEvent.change(screen.getByLabelText('Trade name'), { target: { value: 'DBS' } })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    })
    await act(async () => {})
    expect(put.details).toMatchObject({ trade_name: 'DBS', legal_name: client.name })
    expect(put.details?.pan ?? '').toBe('')
    expect(screen.getByLabelText('Client details')).toHaveTextContent('DBS')
  })
})

describe('list filters take several values', () => {
  test('empty means all, several mean any of', () => {
    const rs = [rule({ id: 'A', category: 'R1', tax_years: ['2024-25'] }), rule({ id: 'B', category: 'R2', tax_years: ['2025-26'] }), rule({ id: 'C', category: 'R3', tax_years: ['2023-24'] })]
    expect(filterRules(rs, noRuleFilter)).toHaveLength(3)
    expect(filterRules(rs, { ...noRuleFilter, category: ['R1', 'R2'] }).map((r) => r.id)).toEqual(['A', 'B'])
    expect(filterRules(rs, { ...noRuleFilter, taxYear: ['2024-25', '2025-26'] }).map((r) => r.id)).toEqual(['A', 'B'])
  })
})

describe('Rules screen, two tax years', () => {
  test('choosing two tax years lists rules of either', () => {
    render(<App />)
    railTo('Rules')
    fireEvent.click(screen.getByRole('button', { name: /^Tax year:/ }))
    fireEvent.click(screen.getByRole('checkbox', { name: '2024-25' }))
    fireEvent.click(screen.getByRole('checkbox', { name: '2025-26' }))
    const n = filterRules(rules, { ...noRuleFilter, taxYear: ['2024-25', '2025-26'] }).length
    expect(document.querySelector(".result-count")?.textContent).toContain(`${n} of ${rules.length} rules`)
    expect(screen.getByRole('button', { name: 'Tax year: 2 selected' })).toBeInTheDocument()
  }, 20_000)
})
