import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import App from './App.tsx'
import { sampleRules, sampleRuns } from './sample/index.ts'
import { bannedIn } from './wording.ts'
import { showFinding, showView, openBoardClient } from './testFindings.ts'

// SPEC-004 part 9: choosing the rules of a type of check, and several types per client.

function key(k: string) {
  fireEvent.keyDown(document.activeElement ?? document.body, { key: k })
}

const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status, headers: { 'content-type': 'application/json' } })
const client = { id: 1, name: 'Deccan Byte Systems Pvt Ltd', pan_last4: '565Z', last_run_on: '', documents: 0, counts: null }

// openLiveClient renders the screens against a mocked recon-server with one client and no
// result yet, opens the client and chooses a file; sent collects each run's form.
async function openLiveClient(runReply: (form: FormData) => unknown) {
  const sent: FormData[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      if (url === '/api/v1/rules') return json(sampleRules)
      if (url.startsWith('/api/v1/clients?')) return json({ tax_year: '2025-26', clients: [client] })
      if (url.startsWith('/api/v1/clients/1/result')) return json({ client, report: null, results: [], facts: { engagements: [] } })
      if (url === '/api/v1/clients/1') return json({ error_code: 'NOT_HERE', message: '' }, 404)
      if (url === '/api/v1/runs') {
        const form = init?.body as FormData
        sent.push(form)
        return json(runReply(form))
      }
      throw new Error('unexpected ' + url)
    }),
  )
  render(<App />)
  await act(async () => {})
  await act(async () => {})
  await act(async () => {
    key('Enter')
  })
  fireEvent.change(screen.getByLabelText('Files to check'), { target: { files: [new File(['{}'], 'a.json', { type: 'application/json' })] } })
  return sent
}

describe('Upload and run with no type of check (part 9, types removed 30 October 2026)', () => {
  afterEach(() => vi.unstubAllGlobals())

  test('no type of check is chosen: once a file is chosen every rule can run, and the panel says so', async () => {
    await openLiveClient(() => ({}))
    expect(screen.queryByRole('group', { name: 'Types of check to run' })).toBeNull()
    expect(screen.queryByRole('checkbox', { name: 'Tax suite: computation checks' })).toBeNull()
    expect(screen.getByRole('note', { name: 'What runs' })).toHaveTextContent('Every rule in the system runs on the files uploaded')
    expect(screen.getByRole('note', { name: 'What runs' })).toHaveTextContent('recognised by what it contains, not by its name')
    expect(screen.getByRole('button', { name: 'Run checks' })).toBeEnabled()
  })

  test('the run sends no type of check and no rule choice, only the files, the year and the testing switch', async () => {
    const all = { ...sampleRuns.differs.report, comparison: null }
    const sent = await openLiveClient(() => ({
      exit_code: 1, report: all, facts: sampleRuns.differs.facts, results: [{ comparison_id: '', last_run_on: '', report: all }],
    }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Include rules not yet approved by the CA' }))
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Run checks' }))
    })
    expect(sent).toHaveLength(1)
    expect(sent[0].getAll('comparison')).toEqual([])
    expect([...sent[0].keys()].filter((k) => k.startsWith('rules:'))).toEqual([])
    expect(sent[0].get('include_testing')).toBe('false')
    expect(sent[0].get('client_id')).toBe('1')
  })

  test('a run of every rule shows one result, with no tabs, and its findings', async () => {
    const all = { ...sampleRuns['gst-sales'].report, comparison: null }
    await openLiveClient(() => ({
      exit_code: 1, report: all, facts: sampleRuns['gst-sales'].facts,
      results: [{ comparison_id: '', last_run_on: '2026-10-30', report: all }],
    }))
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Run checks' }))
    })
    expect(screen.queryByRole('tablist', { name: 'Types of check' })).toBeNull()
    expect(screen.getByLabelText('Result summary')).toHaveTextContent('tax year 2026-27')
    expect(screen.getByLabelText('Result summary')).toHaveTextContent(`${all.counts.fail} Findings`)
    expect(showFinding(/DBS\/26-27\/0462/)).toBeInTheDocument()
    expect(bannedIn(document.body.textContent ?? '')).toEqual([])
  })

  test('a run of every rule replaces the results of the old types of check on screen', async () => {
    const all = { ...sampleRuns['gst-sales'].report, comparison: null }
    const old = sampleRuns['gst-purchase'].report
    await openLiveClient(() => ({
      exit_code: 1, report: all, facts: sampleRuns['gst-sales'].facts,
      results: [
        { comparison_id: '', last_run_on: '2026-10-30', report: all },
        { comparison_id: 'gst_purchase_vs_gstr2b', last_run_on: '2026-10-15', report: old },
      ],
    }))
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Run checks' }))
    })
    expect(screen.queryAllByRole('tab')).toHaveLength(0)
    expect(screen.queryByRole('article', { name: /KPS\/UPS\/0231/ })).toBeNull()
  })

  test('AC-33: the sample company shows its two types as tabs, moved between with the arrows', async () => {
    render(<App />)
    openBoardClient('Deccan Byte Systems Pvt Ltd')
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Deccan Byte Systems Pvt Ltd')
    const tabs = screen.getAllByRole('tab')
    expect(tabs).toHaveLength(2)
    expect(tabs[1]).toHaveTextContent('GST: Purchase register vs GSTR-2B')
    fireEvent.keyDown(screen.getByRole('tablist', { name: 'Types of check' }), { key: 'ArrowRight' })
    expect(screen.getByRole('tab', { selected: true })).toHaveTextContent('GST: Purchase register vs GSTR-2B')
    expect(showFinding(/KPS\/UPS\/0231/)).toHaveTextContent('In Purchase register only')
    fireEvent.keyDown(screen.getByRole('tablist', { name: 'Types of check' }), { key: 'ArrowRight' })
    expect(screen.getByRole('tab', { selected: true })).toHaveTextContent('GST: Sales register vs GSTR-1')
  })

  test('AC-34: the Season board adds up the types of a sample client', () => {
    render(<App />)
    const sales = sampleRuns['gst-sales'].report.counts
    const purchase = sampleRuns['gst-purchase'].report.counts
    const row = screen.getByRole('row', { name: /Deccan Byte Systems Pvt Ltd/ })
    expect(row).toHaveTextContent(String(sales.fail + purchase.fail))
  })
})

describe('Tax year of a client file (fix of 15 October 2026)', () => {
  afterEach(() => vi.unstubAllGlobals())

  // A client checked only for 2026-27; the board's tax year is 2025-26.
  function serverWithResultIn2026(sent: FormData[], asked: string[], runYears = ['2026-27']) {
    const sales = sampleRuns['gst-sales']
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string, init?: RequestInit) => {
        if (url === '/api/v1/rules') return json(sampleRules)
        if (url.startsWith('/api/v1/clients?')) return json({ tax_year: '2025-26', clients: [client] })
        if (url.startsWith('/api/v1/clients/1/result')) {
          const ty = new URL(url, 'http://x').searchParams.get('tax_year') ?? ''
          asked.push(ty)
          const results = ty === '2026-27' ? [{ comparison_id: 'gst_sales_vs_gstr1', last_run_on: '2026-10-15', report: sales.report }] : []
          return json({ client, tax_year: ty, run_tax_years: runYears, report: results[0]?.report ?? null, results, facts: sales.facts })
        }
        if (url === '/api/v1/clients/1') return json({ error_code: 'NOT_HERE', message: '' }, 404)
        if (url === '/api/v1/inspect') return json({ file: 'x', recognised_as: 'generic.gstn.gstr1 v1', tax_year: '2026-27', fields: [], lists: [], truncated: false, document_types: [] })
        if (url === '/api/v1/runs') {
          sent.push(init?.body as FormData)
          return json({ exit_code: 1, report: sales.report, results: [{ comparison_id: 'gst_sales_vs_gstr1', last_run_on: '2026-10-15', report: sales.report }], facts: sales.facts })
        }
        throw new Error('unexpected ' + url)
      }),
    )
  }

  test('a client opens on the latest tax year with a run, and the result names the year', async () => {
    const asked: string[] = []
    serverWithResultIn2026([], asked)
    render(<App />)
    await act(async () => {})
    await act(async () => {})
    await act(async () => {
      key('Enter')
    })
    await act(async () => {})
    expect(asked).toEqual(['2025-26', '2026-27'])
    expect(screen.getByLabelText('Results for tax year')).toHaveValue('2026-27')
    expect(screen.getByLabelText('Result summary')).toHaveTextContent(/tax year 2026-27/)
    expect(screen.getByLabelText('Result summary')).toHaveTextContent(/\d+ Findings/)
    fireEvent.click(screen.getByRole('button', { name: 'Upload and run' }))
    expect(screen.getByLabelText('Tax year of the run')).toHaveValue('2026-27')
    fireEvent.keyDown(screen.getByLabelText('Tax year of the run'), { key: 'Escape' })

    // Choosing 2025-26 shows that year, which has no run.
    await act(async () => {
      fireEvent.change(screen.getByLabelText('Results for tax year'), { target: { value: '2025-26' } })
    })
    await act(async () => {})
    expect(asked[asked.length - 1]).toBe('2025-26')
    expect(screen.getByText(/No run yet for this client in tax year 2025-26/)).toBeInTheDocument()
  })

  test('a client with runs in both years opens on the later one', async () => {
    const asked: string[] = []
    serverWithResultIn2026([], asked, ['2025-26', '2026-27'])
    render(<App />)
    await act(async () => {})
    await act(async () => {})
    await act(async () => {
      key('Enter')
    })
    await act(async () => {})
    expect(asked).toEqual(['2025-26', '2026-27'])
    expect(screen.getByLabelText('Results for tax year')).toHaveValue('2026-27')
  })

  test("choosing files sets the run's tax year to the files' own year", async () => {
    const sent: FormData[] = []
    serverWithResultIn2026(sent, [])
    render(<App />)
    await act(async () => {})
    await act(async () => {})
    await act(async () => {
      key('Enter')
    })
    await act(async () => {})
    await act(async () => {
      fireEvent.change(screen.getByLabelText('Results for tax year'), { target: { value: '2025-26' } })
    })
    await act(async () => {})
    expect(screen.getByLabelText('Tax year of the run')).toHaveValue('2025-26')
    await act(async () => {
      fireEvent.change(screen.getByLabelText('Files to check'), { target: { files: [new File(['{}'], 'GSTR1_Aug2026.json', { type: 'application/json' })] } })
    })
    expect(screen.getByLabelText('Tax year of the run')).toHaveValue('2026-27')
    expect(screen.getByRole('status')).toHaveTextContent('The files are for tax year 2026-27, so the tax year was set to 2026-27.')
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Run checks' }))
    })
    expect(sent[0].get('tax_year')).toBe('2026-27')
    expect(screen.getByLabelText('Results for tax year')).toHaveValue('2026-27')
  })
})

describe('Clearer results (15 October 2026)', () => {
  afterEach(() => vi.unstubAllGlobals())

  test('GST files show the rows read, not 0 figures', () => {
    render(<App />)
    openBoardClient('Deccan Byte Systems Pvt Ltd')
    showView('Documents')
    const docs = screen.getByRole('table', { name: 'Documents' })
    expect(within(docs).getByRole('columnheader', { name: 'Figures / rows read' })).toBeInTheDocument()
    expect(within(docs).getByRole('row', { name: /SALES_REG_Aug2026\.json/ })).toHaveTextContent('26 rows')
    expect(within(docs).getByRole('row', { name: /GSTR1_Aug2026\.json/ })).toHaveTextContent('23 rows')
  })

  test('opening a GST file shows its rows, each with amounts and its source', () => {
    render(<App />)
    openBoardClient('Deccan Byte Systems Pvt Ltd')
    showView('Documents')
    key('Enter') // open the first document of the tab: GSTR1_Aug2026.json
    const panel = screen.getByLabelText('Read from GSTR1_Aug2026.json')
    expect(panel).toHaveTextContent('23 rows read')
    const row = within(within(panel).getByRole('table', { name: 'Rows read' })).getByRole('row', { name: /DBS\/26-27\/0451/ })
    for (const text of ['27BMSCV1962U1ZB', '2026-08-03', '3,25,000.00', '29,250.00', '3,83,500.00', 'GSTR1_Aug2026.json $.b2b[0].inv[0]']) {
      expect(row).toHaveTextContent(text)
    }
    expect(panel).not.toHaveTextContent('Nothing was stored')
  })

  test('a run with only totals to show says no checks ran, instead of empty result lists', async () => {
    const sales = sampleRuns['gst-sales']
    const summary = {
      ...sales.report,
      comparison: null,
      totals: [{ document_type: 'sales_register', document: 'Sales register', rows: 26, amounts: { taxable_value: 100 } }],
      counts: { checks: 0, pass: 0, fail: 0, cannot_check: 0 }, findings: [], cannot_check: [], passed: [], not_selected: [],
    }
    await openLiveClient(() => ({
      exit_code: 0, report: summary, facts: sales.facts,
      results: [{ comparison_id: '', last_run_on: '2026-10-30', report: summary }],
    }))
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Run checks' }))
    })
    expect(screen.getByText(/No checks ran on these files: the totals above are shown side by side/)).toBeInTheDocument()
    expect(screen.getByLabelText('Totals')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Results' })).toBeNull()
    expect(screen.queryByText('No findings.')).toBeNull()
  })
})

describe('Why parts of a file were not read, and what the words mean', () => {
  function openSampleGst() {
    render(<App />)
    openBoardClient('Deccan Byte Systems Pvt Ltd') // the sample company, on its sales vs GSTR-1 tab
    showView('Documents') // focus is on its Documents
  }

  test('a register lists the rows left out with the condition each failed, and the columns not used', () => {
    openSampleGst()
    key('ArrowDown')
    key('Enter') // SALES_REG_Aug2026.json
    const box = screen.getByLabelText('Not read, and why')
    expect(within(box).getByRole('heading', { name: 'Rows left out (10)' })).toBeInTheDocument()
    // The reason names the column and the condition, never the row's own value (client data).
    expect(box).toHaveTextContent('rows: .supply_type is another value; this format reads only rows where .supply_type is "B2B"')
    expect(box.textContent).not.toContain('"CDNR"')
    expect(box).toHaveTextContent('party_name (26)')
    expect(bannedIn(box.textContent ?? '')).toEqual([])
  })

  test('a return lists the other tables it does not read', () => {
    openSampleGst()
    key('Enter') // GSTR1_Aug2026.json
    const box = screen.getByLabelText('Not read, and why')
    expect(within(box).getByRole('heading', { name: /Other parts of the file this format does not read/ })).toBeInTheDocument()
    for (const part of ['$.hsn', '$.b2cs', '$.exp', '$.cdnr', '$.doc_issue']) expect(box).toHaveTextContent(part)
  })

  test('the results explain result, finding and cannot check, in allowed wording', () => {
    openSampleGst()
    showView('Findings')
    const help = screen.getByText('What do "result", "finding" and "cannot check" mean?').closest('details') as HTMLElement
    for (const text of ['Pass', 'Fail, a finding', 'both values, the difference', 'Cannot check', 'never counted as a pass']) {
      expect(help).toHaveTextContent(text)
    }
    expect(bannedIn(help.textContent ?? '')).toEqual([])
  })
})
