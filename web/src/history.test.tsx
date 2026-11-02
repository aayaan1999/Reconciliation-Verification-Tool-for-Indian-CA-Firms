import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import App from './App.tsx'
import { sampleRules, sampleRuns } from './sample/index.ts'
import type { ComparisonEntry } from './types.ts'
import { showView } from './testFindings.ts'

// SPEC-009: a client's runs of every rule listed as Run 1, 2…, and an earlier one explored
// read only, as it was reported.

const json = (b: unknown) => new Response(JSON.stringify(b), { status: 200, headers: { 'content-type': 'application/json' } })
const client = { id: 1, name: 'Deccan Byte Systems Pvt Ltd', pan_last4: '565Z', last_run_on: '2026-10-29', documents: 2, counts: null }
const sales = { ...sampleRuns['gst-sales'], report: { ...sampleRuns['gst-sales'].report, comparison: null } }
const first = { ...sales.report, findings: sales.report.findings.slice(0, 1) } // what Run 1 reported
const entry = (no: number, draft: number): ComparisonEntry => ({
  no, finished_at: `2026-10-2${7 + no} 16:10`, includes_testing: true, kept: true, counts: sales.report.counts,
  files: [{ document_type: 'sales_register', file_name: 'SALES_REG_Aug2026.xlsx', draft_no: draft }],
})

test('earlier runs are listed and open read only', async () => {
  const asked: string[] = []
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url === '/api/v1/rules') return json(sampleRules)
    if (url.startsWith('/api/v1/clients?')) return json({ tax_year: '2026-27', clients: [client] })
    if (url.startsWith('/api/v1/clients/1/result')) {
      return json({ client, tax_year: '2026-27', run_tax_years: ['2026-27'], report: sales.report,
        results: [{ comparison_id: '', last_run_on: '2026-10-29', report: sales.report }], decisions: {}, facts: sales.facts })
    }
    if (url.startsWith('/api/v1/clients/1/comparisons?')) {
      asked.push(url)
      const list = { comparison_id: '', tax_year: '2026-27', comparisons: [entry(1, 1), entry(2, 2)] }
      return json(url.includes('no=1') ? { ...list, comparison: entry(1, 1), report: first, facts: sales.facts } : list)
    }
    if (url === '/api/v1/clients/1') return json({ details: null })
    throw new Error('unexpected ' + url)
  }))
  render(<App />)
  await act(async () => {})
  await act(async () => {})
  await act(async () => {
    fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Enter' })
  })
  await act(async () => {})
  await act(async () => {})
  const decisionPanels = () => screen.queryAllByRole('group', { name: /^Decision on / }).length
  expect(decisionPanels()).toBeGreaterThan(0)
  await act(async () => {
    showView('Run history')
  })
  expect(asked[0]).toBe('/api/v1/clients/1/comparisons?tax_year=2026-27&comparison=')

  const list = screen.getByRole('list', { name: 'Runs of this client' })
  expect(within(list).getAllByRole('listitem').map((li) => li.textContent)).toEqual([
    expect.stringMatching(/^Run 2Latest2026-10-29 16:10/),
    expect.stringContaining('sales register: SALES_REG_Aug2026.xlsx, draft 1'),
  ])

  await act(async () => {
    fireEvent.click(within(list).getByRole('button', { name: 'Open Run 1 read only' }))
  })
  expect(asked[1]).toContain('no=1')
  expect(screen.getByText(/Run 1 of 2, run on 2026-10-28 16:10\. Read only/)).toBeInTheDocument()
  expect(decisionPanels()).toBe(0)
  const index = screen.getByRole('navigation', { name: 'Findings by document to fix' })
  expect(within(index).getAllByRole('button')).toHaveLength(first.findings[0].findings.length)
  await act(async () => {
    showView('Run history')
  })
  expect(screen.getByRole('button', { name: 'Open Run 1 read only' })).toHaveAttribute('aria-current', 'true')
  showView('Findings')

  fireEvent.click(screen.getByRole('button', { name: 'Back to latest' }))
  expect(screen.queryByText(/Read only/)).toBeNull()
  expect(decisionPanels()).toBeGreaterThan(0)
})

afterEach(() => vi.unstubAllGlobals())
