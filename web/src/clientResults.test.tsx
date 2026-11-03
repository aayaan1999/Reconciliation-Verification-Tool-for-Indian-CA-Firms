import { act, fireEvent, render, screen, within } from '@testing-library/react'
import type { ComponentProps } from 'react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { ClientFile } from './screens/ClientFile.tsx'
import { sampleRules, sampleRuns } from './sample/index.ts'
import { showFinding, showResults, showView } from './testFindings.ts'
import type { ComparisonEntry, FindingDecision } from './types.ts'
import { bannedIn } from './wording.ts'

// The Client file's result, redesigned after the Recon Screen Document v2 (§6.2 upload and
// run, §7.1 result summary, §7.2 findings by document, §7.3 one finding, §7.4 deciding, §8.1
// documents, §8.2 run history). The client is the fictional Test Pack A company.

const pack = sampleRuns['filing-pack']
const findings = pack.report.findings.flatMap((g) => g.findings)
const client = { id: 'c7', name: 'Nimbus Logic Systems Private Limited (Test Pack A)', pan_last4: '821K', runs: ['filing-pack'], last_run_on: '2026-10-16' }
const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status, headers: { 'content-type': 'application/json' } })

function show(props: Partial<ComponentProps<typeof ClientFile>> = {}) {
  return render(
    <ClientFile client={client} runs={[{ ...pack, last_run_on: '2026-11-02 12:10' }]} rules={sampleRules.rules} live={false}
      fromUpload={false} taxYears={['2025-26']} onRun={() => {}} {...props} />,
  )
}

const panelSteps = () => screen.getByRole('group', { name: 'Move between findings' })

afterEach(() => vi.unstubAllGlobals())

describe('7.1 Result summary', () => {
  test('the card names the run, its date and tax year, the counts, what is still open and the testing note', () => {
    show({ decisions: {}, onDecide: vi.fn() })
    const summary = screen.getByLabelText('Result summary')
    const c = pack.report.counts
    for (const text of ['Latest run', '2026-11-02 12:10 · tax year 2025-26', `${c.checks} Checks`, `${c.pass} Pass`, `${c.fail} Findings`,
      `${c.cannot_check} Cannot check`, `${findings.length} open, no decision yet`, '0 decided']) {
      expect(summary).toHaveTextContent(text)
    }
    expect(within(summary).getByRole('note')).toHaveTextContent('This run includes rules not yet approved by the CA')
    expect(summary.querySelector('.stack-bar')).not.toBeNull()
  })
})

describe('7.2 Findings, grouped by the document to fix', () => {
  test('the result filter shows counts; one compact filter line with Clear; groups show open and total and fold', () => {
    show({ decisions: {}, onDecide: vi.fn() })
    const filter = screen.getByRole('group', { name: 'Result filter' })
    const c = pack.report.counts
    expect(within(filter).getAllByRole('button').map((b) => b.textContent)).toEqual([
      `Findings ${c.fail}`, `Cannot check ${c.cannot_check}`, `Passed ${c.pass}`, 'All',
    ])
    expect(within(filter).getByRole('button', { name: /^Findings/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByText(`${c.fail} of ${c.checks} results`)).toBeInTheDocument()

    const index = screen.getByRole('navigation', { name: 'Findings by document to fix' })
    const group = within(index).getByText('Computation of total income prepared in the tax suite').closest('details') as HTMLDetailsElement
    expect(group.querySelector('summary')).toHaveTextContent('5 open · 5')
    expect(group.open).toBe(true)
    fireEvent.click(group.querySelector('summary') as HTMLElement)
    expect(group.open).toBe(false)
    // Each line: rule ID, the difference, the name and Open.
    const line = within(index).getByRole('button', { name: /RECON-DEP-001/ })
    for (const text of ['RECON-DEP-001', '2,448.00', 'Depreciation', 'Open']) expect(line).toHaveTextContent(text)
  })

  test('search, decision, document and category filters sit on one line, with Clear when any is set', () => {
    show({ decisions: {}, onDecide: vi.fn() })
    const c = pack.report.counts
    for (const name of ['Search results', 'Decision filter', 'Document filter', 'Category filter']) {
      expect(within(screen.getByRole('group', { name: 'Result filters' })).getByLabelText(name)).toBeInTheDocument()
    }
    expect(screen.queryByRole('button', { name: 'Clear' })).toBeNull()
    fireEvent.change(screen.getByLabelText('Search results'), { target: { value: 'RECON-MAT' } })
    const mat = findings.filter((f) => f.rule_id.startsWith('RECON-MAT')).length
    expect(screen.getByText(`${mat} of ${c.checks} results`)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }))
    expect(screen.getByLabelText('Search results')).toHaveValue('')
    expect(screen.getByText(`${c.fail} of ${c.checks} results`)).toBeInTheDocument()

    showResults('Passed')
    expect(screen.getByText(`${c.pass} of ${c.checks} results`)).toBeInTheDocument()
    expect(panelSteps()).toHaveTextContent(`Result 1 of ${c.pass}`)
    expect(screen.queryByRole('article')).toBeNull()
  })
})

describe('7.3 One finding', () => {
  test('figures are rows A and B with where each comes from, then Difference (A − B); Next and Previous move', () => {
    show()
    expect(panelSteps()).toHaveTextContent(`Finding 1 of ${findings.length}`)
    const card = screen.getByRole('article', { name: /RECON-DEP-001/ })
    const rows = within(within(card).getByRole('table', { name: 'Figures of this finding' })).getAllByRole('row')
    expect(rows[0]).toHaveTextContent(/Figure.*Where it comes from.*Amount \(₹\)/)
    expect(rows[1]).toHaveTextContent(/^A.*Computation, "Allowed depreciation".*09_filing_pack_structured\.json \$\.computation\.less\.allowed_depreciation.*1,38,310\.00$/)
    expect(rows[2]).toHaveTextContent(/^B.*Computation, fixed-asset schedule total.*\$\.computation\.fixed_asset_schedule\.total\.depreciation.*1,35,862\.00$/)
    expect(rows[3]).toHaveTextContent(/Difference \(A − B\).*2,448\.00$/)
    expect(rows[3]).toHaveClass('cf-diff')
    expect(card).toHaveTextContent('Rule basis: Income-tax Act, 1961 – Sec 32')

    fireEvent.click(within(panelSteps()).getByRole('button', { name: 'Next' }))
    expect(panelSteps()).toHaveTextContent(`Finding 2 of ${findings.length}`)
    fireEvent.click(within(panelSteps()).getByRole('button', { name: 'Previous' }))
    expect(panelSteps()).toHaveTextContent(`Finding 1 of ${findings.length}`)
    fireEvent.click(within(panelSteps()).getByRole('button', { name: 'Next' }))
    expect(panelSteps()).toHaveTextContent(`Finding 2 of ${findings.length}`)
    expect(bannedIn(document.body.textContent ?? '')).toEqual([])
  })
})

describe('7.4 Deciding a finding', () => {
  test('three decision cards say what they mean; a card decides and moves to the next open finding, unless that is turned off', async () => {
    const onDecide = vi.fn(async () => {})
    show({ decisions: {}, onDecide })
    const first = findings[0]
    const panel = screen.getByRole('group', { name: `Decision on ${first.rule_id}` })
    expect(within(panel).getByRole('button', { name: 'Fix' })).toHaveAccessibleDescription('The document will be corrected')
    expect(within(panel).getByRole('button', { name: 'Not an issue' })).toHaveAccessibleDescription('The difference is explained')
    expect(within(panel).getByRole('button', { name: 'Ask client' })).toHaveAccessibleDescription('Information is needed from the client')
    expect(panel).toHaveTextContent('Decisions are added on top and never overwritten. They follow the finding into later runs.')

    await act(async () => {
      fireEvent.click(within(panel).getByRole('button', { name: 'Fix' }))
    })
    expect(onDecide).toHaveBeenCalledWith(first.finding_id, 'fix', '')
    expect(screen.getByRole('status')).toHaveTextContent(`Fix is recorded for ${first.rule_id}.`)
    expect(panelSteps()).toHaveTextContent(`Finding 2 of ${findings.length}`)

    fireEvent.click(screen.getByRole('checkbox', { name: 'Go to the next open finding after deciding' }))
    await act(async () => {
      fireEvent.click(within(screen.getByRole('article')).getByRole('button', { name: 'Ask client' }))
    })
    expect(onDecide).toHaveBeenLastCalledWith(findings[1].finding_id, 'ask_client', '')
    expect(panelSteps()).toHaveTextContent(`Finding 2 of ${findings.length}`)
  })

  test('the history shows the newest decision first, with who, when and the note', () => {
    const entry = (decision: 'fix' | 'ask_client', at: string, note: string) =>
      ({ decision, note, decided_by: 'B. Manager', decided_at: at, on_other_values: false })
    const d: FindingDecision = {
      state: 'ask_client', current: entry('ask_client', '2026-11-02T09:00:00Z', 'Asked for the asset register'),
      history: [entry('fix', '2026-11-01T09:00:00Z', 'First look'), entry('ask_client', '2026-11-02T09:00:00Z', 'Asked for the asset register')],
    }
    show({ decisions: { [findings[0].finding_id]: d }, onDecide: vi.fn() })
    const panel = screen.getByRole('group', { name: `Decision on ${findings[0].rule_id}` })
    expect(within(panel).getByRole('button', { name: 'Ask client' })).toHaveAttribute('aria-pressed', 'true')
    const items = within(panel).getAllByRole('listitem')
    expect(items[0]).toHaveTextContent(/^Ask client by B\. Manager · 2026-11-02 09:00 UTC\s*Current\s*Asked for the asset register$/)
    expect(items[1]).toHaveTextContent(/^Fix by B\. Manager · 2026-11-01 09:00 UTC\s*First look$/)
  })

  test('a finding is deleted from the … menu with a reason', async () => {
    const onDeleteFinding = vi.fn(async () => {})
    show({ decisions: {}, onDecide: vi.fn(), onDeleteFinding })
    expect(screen.queryByRole('button', { name: 'Delete finding' })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'More actions on this finding' }))
    fireEvent.click(screen.getByRole('button', { name: 'Delete finding' }))
    const dialog = screen.getByRole('dialog', { name: `Delete ${findings[0].rule_id}?` })
    expect(dialog).toHaveTextContent('kept in the audit trail')
    const confirm = within(dialog).getByRole('button', { name: 'Delete finding' })
    expect(confirm).toBeDisabled()
    fireEvent.change(within(dialog).getByLabelText(/Reason for deleting/), { target: { value: 'Duplicate of RECON-DEP-002' } })
    await act(async () => {
      fireEvent.click(confirm)
    })
    expect(onDeleteFinding).toHaveBeenCalledWith(findings[0], 'Duplicate of RECON-DEP-002')
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(screen.getByRole('article')).toHaveTextContent('Deleted. The finding is kept; the other roles no longer see it.')
  })

  test('a Cannot check result says what was missing and what to upload', () => {
    const differs = sampleRuns.differs
    show({ runs: [differs] })
    showResults('Cannot check')
    const cannot = differs.report.cannot_check.find((c) => c.rule_id === 'RECON-DEP-002')!
    fireEvent.click(within(screen.getByRole('navigation', { name: 'Findings by document to fix' })).getByRole('button', { name: /RECON-DEP-002/ }))
    const detail = screen.getByRole('region', { name: `RECON-DEP-002 ${cannot.title}` })
    expect(detail).toHaveTextContent('Why')
    expect(detail).toHaveTextContent(`Cannot check: figure not found in ${cannot.missing[0].document}: ${cannot.missing[0].label}`)
    expect(detail).toHaveTextContent(`What to upload${cannot.missing[0].document} for tax year ${differs.report.tax_year}`)
    expect(bannedIn(detail.textContent ?? '')).toEqual([])
  })
})

describe('6.2 Upload and run', () => {
  test('each file shows what it was recognised as before the run; an unknown file can be taught; an .xls is refused', async () => {
    const sent: FormData[] = []
    let finish: (r: Response) => void = () => {}
    vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
      const form = init?.body as FormData
      if (url === '/api/v1/inspect') {
        const name = (form.get('file') as File).name
        return name === 'pack.json'
          ? json({ file: name, recognised_as: 'generic.taxsuite.filing_pack v1', tax_year: '2025-26', fields: [], lists: [], truncated: false, document_types: [] })
          : json({ file: name, recognised_as: '', fields: [], lists: [], truncated: false, document_types: [] })
      }
      if (url === '/api/v1/runs') {
        sent.push(form)
        return new Promise<Response>((resolve) => {
          finish = resolve
        })
      }
      throw new Error('unexpected ' + url)
    }))
    show({ runs: [], live: true })
    const files = [new File(['{}'], 'pack.json'), new File(['{}'], 'office_billing.json'), new File(['x'], 'old_ledger.xls')]
    await act(async () => {
      fireEvent.change(screen.getByLabelText('Files to check'), { target: { files } })
    })
    const list = screen.getByRole('list', { name: 'Files chosen' })
    const [pack1, unknown, xls] = within(list).getAllByRole('listitem')
    expect(pack1).toHaveTextContent('generic.taxsuite.filing_pack v1 · 2025-26')
    expect(pack1).toHaveTextContent('Recognised')
    expect(unknown).toHaveTextContent('The file matches no known format.')
    expect(within(unknown).getByRole('button', { name: 'Teach this layout (office_billing.json)' })).toBeInTheDocument()
    expect(xls).toHaveTextContent('Save it as .xlsx or .csv first')

    // The switch sits next to the button, which says how many files will be read.
    expect(screen.getByRole('checkbox', { name: 'Include rules not yet approved by the CA' })).toBeChecked()
    const run = screen.getByRole('button', { name: 'Run checks' })
    expect(run).toHaveTextContent('Run checks on 2 files')
    await act(async () => {
      fireEvent.click(run)
    })
    expect(sent[0].getAll('files').map((f) => (f as File).name)).toEqual(['pack.json', 'office_billing.json'])
    // While the run happens: the four steps, with no claim of which one is under way.
    expect(screen.getByRole('progressbar', { name: 'Running the checks' })).not.toHaveAttribute('aria-valuenow')
    expect(within(screen.getByRole('dialog', { name: 'Upload and run' })).getByRole('list').textContent).toBe('Reading the filesRecognising each documentRunning every ruleSaving the result')
    expect(screen.getByRole('form', { name: 'Upload and run' })).toHaveTextContent('on 2 files')
    await act(async () => {
      finish(json({ exit_code: 1, report: pack.report, facts: pack.facts, results: [{ comparison_id: '', last_run_on: '', report: pack.report }] }))
    })
  })
})

describe('8.1 Documents', () => {
  test('four totals, each document with a coverage bar, and a drawer of what was read with a filter and search', () => {
    show()
    showView('Documents')
    const files = pack.report.files
    const totals = screen.getByRole('list', { name: 'What was read' })
    const read = files.reduce((n, f) => n + f.fact_count + f.record_count, 0)
    const unread = files.reduce((n, f) => n + f.unread.length, 0)
    expect(within(totals).getAllByRole('listitem').map((li) => li.textContent)).toEqual([
      '1 Files uploaded', `${files.length} Documents recognised`, `${read} Figures and rows read by the checks`, `${unread} Values no check uses yet`,
    ])
    const table = screen.getByRole('table', { name: 'Documents' })
    expect(within(table).getByRole('columnheader', { name: 'Coverage' })).toBeInTheDocument()

    fireEvent.click(within(table).getAllByRole('row')[1])
    const drawer = screen.getByRole('dialog', { name: 'Read from 09_filing_pack_structured.json, computation' })
    const figures = () => within(drawer).queryByRole('table', { name: 'Figures read' })
    const all = within(figures()!).getAllByRole('row').length
    fireEvent.change(within(drawer).getByLabelText('Search what was read'), { target: { value: 'allowed_depreciation' } })
    expect(within(figures()!).getAllByRole('row').length).toBeLessThan(all)
    expect(figures()).toHaveTextContent('$.computation.less.allowed_depreciation')
    fireEvent.click(within(drawer).getByRole('button', { name: 'Not read' }))
    expect(figures()).toBeNull()
    expect(drawer).toHaveTextContent('Nothing that was not read matches the search.')
    fireEvent.change(within(drawer).getByLabelText('Search what was read'), { target: { value: '' } })
    expect(within(drawer).getByLabelText('Not read, and why')).toHaveTextContent(`Not read, and why (${files[0].unread.length} values)`)
    fireEvent.click(within(drawer).getByRole('button', { name: 'Close read from 09_filing_pack_structured.json, computation' }))
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})

describe('8.2 Run history', () => {
  test('each run is a card with its counts and the change in findings; an earlier one opens read only', async () => {
    const counts = (fail: number) => ({ checks: 67, pass: 67 - fail, fail, cannot_check: 0 })
    const entry = (no: number, fail: number): ComparisonEntry => ({
      no, finished_at: `2026-11-0${no} 12:10`, includes_testing: no === 2, kept: true, counts: counts(fail),
      files: [{ document_type: 'computation', file_name: '09_filing_pack_structured.json', draft_no: no }],
    })
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url === '/api/v1/clients/1') return json({ details: null })
      if (url.startsWith('/api/v1/clients/1/comparisons?')) {
        const list = { comparison_id: '', tax_year: '2025-26', comparisons: [entry(1, 30), entry(2, 28)] }
        return json(url.includes('no=1') ? { ...list, comparison: entry(1, 30), report: pack.report, facts: pack.facts } : list)
      }
      throw new Error('unexpected ' + url)
    }))
    show({ clientId: 1, viewYear: '2025-26', decisions: {}, onDecide: vi.fn() })
    await act(async () => {
      showView('Run history')
    })
    const [latest, earlier] = within(screen.getByRole('list', { name: 'Runs of this client' })).getAllByRole('listitem')
    expect(latest).toHaveTextContent('39 pass · 28 findings · 0 cannot check · 2 fewer findings than Run 1')
    expect(latest).toHaveTextContent('Included rules not yet approved by the CA')
    expect(latest.querySelector('.stack-bar')).not.toBeNull()
    expect(earlier).not.toHaveTextContent('than Run')
    await act(async () => {
      fireEvent.click(within(earlier).getByRole('button', { name: 'Open Run 1 read only' }))
    })
    expect(screen.getByText(/Run 1 of 2, run on 2026-11-01 12:10\. Read only/)).toBeInTheDocument()
    expect(screen.getByLabelText('Result summary')).toHaveTextContent('Run 1 · read only')
    expect(screen.queryAllByRole('group', { name: /^Decision on / })).toHaveLength(0)
    fireEvent.click(screen.getByRole('button', { name: 'Back to latest' }))
    expect(screen.getByLabelText('Result summary')).toHaveTextContent('Latest run')
    expect(showFinding(/RECON-DEP-001/)).toBeInTheDocument()
  })
})
