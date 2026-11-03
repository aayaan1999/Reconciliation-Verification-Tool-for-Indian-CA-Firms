import { fireEvent, render, screen, within } from '@testing-library/react'
import { afterAll, beforeAll, describe, expect, test } from 'vitest'
import App from './App.tsx'
import { features } from './features.ts'
import { formatPaise } from './format.ts'
import { sampleFiles } from './sample/index.ts'
import { clientsSpec, comparisonListSpec, factListSpec, ruleListSpec, runReportSpec, shapeProblems, worklistSpec } from './shape.ts'
import { bannedIn } from './wording.ts'
import { openFirstClient, railTo, showResults, showView } from './testFindings.ts'

// The Worklist is hidden for now (features.ts); the tests here still cover it.
beforeAll(() => {
  features.worklist = true
})
afterAll(() => {
  features.worklist = false
})

describe('formatPaise', () => {
  test.each([
    [11245050, '1,12,450.50'],
    [275000, '2,750.00'],
    [5, '0.05'],
    [-123456789, '-12,34,567.89'],
    [100000000000, '1,00,00,00,000.00'],
  ])('%i paise is %s', (paise, text) => expect(formatPaise(paise)).toBe(text))

  test('refuses a fraction of a paisa', () => expect(() => formatPaise(1.5)).toThrow())
})

// AC-9: every sample file has exactly the shape the screens read (the engine files are kept
// equal to real engine output by a Go test).
describe('sample data shape', () => {
  const specs = { run: runReportSpec, facts: factListSpec, clients: clientsSpec, worklist: worklistSpec, rules: ruleListSpec, comparisons: comparisonListSpec }
  for (const [name, data] of Object.entries(sampleFiles)) {
    test(name, () => {
      const kind = name.split(/[-.]/)[0] as keyof typeof specs
      expect(shapeProblems(data, specs[kind])).toEqual([])
    })
  }

  test('a changed engine output is caught', () => {
    const broken = structuredClone(sampleFiles['run-differs.json']) as { counts: Record<string, unknown> }
    broken.counts.fail = '1'
    delete broken.counts.pass
    expect(shapeProblems(broken, runReportSpec)).toEqual(['$.counts.pass: missing', '$.counts.fail: want a whole number'])
  })
})

// AC-6 and AC-8: the three screens open with sample data and work with clicks and the
// browser's own keys (Tab, Enter on a button), with no keys of the tool's own.
describe('screen use', () => {
  test('Season board: open a client, go back', () => {
    render(<App />)
    const clients = screen.getByRole('table', { name: 'Clients' })
    fireEvent.click(within(clients).getAllByRole('row')[1])
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Illustrative Traders Private Limited')
    railTo('Season board')
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Season board')
  })

  test('Client file: open a document to see its figures with their source', () => {
    render(<App />)
    openFirstClient()
    showView('Documents')
    const docs = screen.getByRole('table', { name: 'Documents' })
    fireEvent.click(within(docs).getAllByRole('row')[1])
    const panel = screen.getByLabelText('Read from comp_2025-26_differs.json')
    expect(panel).toHaveTextContent('1,15,200.50')
    expect(panel).toHaveTextContent('comp_2025-26_differs.json $.fixed_assets.schedule_total')
  })

  test('Client file with results: upload closed, findings first, the Documents table in its own view; upload opens over the screen', () => {
    render(<App />)
    openFirstClient()
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(screen.getByRole('navigation', { name: 'Findings by document to fix' })).toBeInTheDocument()
    expect(screen.queryByRole('table', { name: 'Documents' })).toBeNull()
    showView('Documents')
    expect(screen.getByRole('table', { name: 'Documents' })).toBeInTheDocument()
    showView('Findings')
    fireEvent.click(screen.getByRole('button', { name: 'Upload and run' }))
    const dialog = screen.getByRole('dialog', { name: 'Upload and run' })
    expect(within(dialog).getByRole('form', { name: 'Upload and run' })).toBeInTheDocument()
    fireEvent.click(within(dialog).getByRole('button', { name: 'Close upload and run' }))
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  test('Worklist: go to an item and classify it', () => {
    render(<App />)
    railTo('Worklist')
    expect(screen.getByRole('table', { name: 'Worklist items' })).toBeInTheDocument()
    // 12 items, 10 a page: the last two are on page 2.
    fireEvent.click(within(screen.getByRole('group', { name: 'Pages of items' })).getByRole('button', { name: 'Next' }))
    const input = screen.getByLabelText('Classification of Travelling expenses - overseas')
    fireEvent.change(input, { target: { value: 'Clause 44: registered and unregistered split' } })
    expect(input).toHaveValue('Clause 44: registered and unregistered split')
    railTo('Season board')
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Season board')
  })
})

// AC-7: the Client file shows each finding with both figures, the difference and both
// sources, grouped by document, plus the Cannot check list; no screen has a banned phrase.
describe('Client file content and wording', () => {
  test('finding with both figures, difference and sources', () => {
    render(<App />)
    openFirstClient()
    const card = screen.getByRole('article', { name: /RECON-DEP-001/ })
    for (const text of [
      'Computation, "Allowed depreciation"',
      '1,12,450.50',
      'Computation, fixed-asset schedule total',
      '1,15,200.50',
      'Difference',
      '2,750.00',
      'comp_2025-26_differs.json $.business_income.allowed_depreciation',
      'comp_2025-26_differs.json $.fixed_assets.schedule_total',
      'Rule basis: Income-tax Act, 1961 – Sec 32',
    ]) {
      expect(card).toHaveTextContent(text)
    }
    // The finding is listed under the document to fix, whose group says how many it holds.
    const index = screen.getByRole('navigation', { name: 'Findings by document to fix' })
    const group = within(index).getByText('Computation of total income prepared in the tax suite').closest('summary') as HTMLElement
    expect(group).toHaveTextContent(/1$/)
    // Cannot check and Passed are their own kinds of result in the result filter, each with its count.
    const filter = screen.getByRole('group', { name: 'Result filter' })
    expect(within(filter).getByRole('button', { name: /^Cannot check \d+$/ })).toBeInTheDocument()
    expect(within(filter).getByRole('button', { name: /^Passed \d+$/ })).toBeInTheDocument()
    showResults('Cannot check')
    fireEvent.click(within(screen.getByRole('navigation', { name: 'Findings by document to fix' })).getByRole('button', { name: /RECON-DEP-002/ }))
    expect(document.body).toHaveTextContent(
      'Cannot check: figure not found in Tax audit report (Form 3CA/3CB with 3CD): Tax audit report, depreciation schedule total',
    )
  })

  test('no banned phrase on any screen', () => {
    const { container } = render(<App />)
    const texts = [container.textContent ?? '']
    for (const row of [1, 2, 3, 4]) {
      fireEvent.click(within(screen.getByRole('table', { name: 'Clients' })).getAllByRole('row')[row])
      texts.push(container.textContent ?? '')
      railTo('Season board')
    }
    railTo('Worklist')
    texts.push(container.textContent ?? '')
    for (const t of texts) expect(bannedIn(t)).toEqual([])
  })
})
