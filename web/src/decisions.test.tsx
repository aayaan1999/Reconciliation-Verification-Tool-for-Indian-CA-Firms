import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import App from './App.tsx'
import { sampleComparisons, sampleRules, sampleRuns } from './sample/index.ts'
import type { FindingDecision } from './types.ts'
import { bannedIn } from './wording.ts'
import { openFirstClient, showFinding } from './testFindings.ts'

// SPEC-006 part 3, AC-11 and AC-12: decisions on findings, on screen.

const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status, headers: { 'content-type': 'application/json' } })
const client = { id: 1, name: 'Deccan Byte Systems Pvt Ltd', pan_last4: '565Z', last_run_on: '2026-10-16', documents: 2, counts: null }
const sales = sampleRuns['gst-sales']
const findings = sales.report.findings.flatMap((g) => g.findings)

async function openWith(decisions: Record<string, FindingDecision>, sent: unknown[] = []) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      if (url === '/api/v1/rules') return json(sampleRules)
      if (url === '/api/v1/comparisons') return json({ comparisons: sampleComparisons })
      if (url.startsWith('/api/v1/clients?')) return json({ tax_year: '2026-27', clients: [client] })
      if (url.startsWith('/api/v1/clients/1/result')) {
        return json({ client, tax_year: '2026-27', run_tax_years: ['2026-27'], report: sales.report,
          results: [{ comparison_id: 'gst_sales_vs_gstr1', last_run_on: '2026-10-16', report: sales.report }], decisions, facts: sales.facts })
      }
      if (url === '/api/v1/clients/1') return json({ error_code: 'NOT_HERE', message: '' }, 404)
      if (url === '/api/v1/clients/1/decisions') {
        const body = JSON.parse(String(init?.body))
        sent.push(body)
        return json({ finding_id: body.finding_id, decision: {
          state: body.decision, current: { decision: body.decision, note: body.note, decided_by: 'A. Partner', decided_at: '2026-10-16T10:00:00Z', on_other_values: false },
          history: [{ decision: body.decision, note: body.note, decided_by: 'A. Partner', decided_at: '2026-10-16T10:00:00Z', on_other_values: false }] } })
      }
      throw new Error('unexpected ' + url)
    }),
  )
  render(<App />)
  await act(async () => {})
  await act(async () => {})
  await act(async () => {
    openFirstClient()
  })
  await act(async () => {})
}

describe('Decisions on findings (SPEC-006 part 3)', () => {
  afterEach(() => vi.unstubAllGlobals())

  test('AC-11: decide a finding with no typed name (the server records the signed-in user); Not an issue needs a note', async () => {
    const sent: unknown[] = []
    await openWith({}, sent)
    const first = findings[0]
    const name = (f: (typeof findings)[number]) => [f.rule_id, f.subject.match_key].filter(Boolean).join(' ')
    const panel = screen.getByRole('group', { name: `Decision on ${name(first)}` })
    expect(screen.getByLabelText('Result summary')).toHaveTextContent(`${findings.length} open`)
    // There is no name field: the decider is the signed-in user (SPEC-012).
    expect(screen.queryByLabelText('Your name')).not.toBeInTheDocument()

    await act(async () => {
      fireEvent.click(within(panel).getByRole('button', { name: 'Not an issue' }))
    })
    expect(within(panel).getByRole('alert')).toHaveTextContent('Write a note saying why this is not an issue')
    expect(sent).toHaveLength(0)
    fireEvent.click(within(panel).getByRole('button', { name: 'Go to the note' }))
    expect(document.activeElement).toBe(within(panel).getByLabelText(`Note on ${name(first)}`))

    fireEvent.change(within(panel).getByLabelText(`Note on ${name(first)}`), { target: { value: 'GSTR-1 to be amended next month' } })
    await act(async () => {
      fireEvent.click(within(panel).getByRole('button', { name: 'Fix' }))
    })
    expect(sent).toEqual([{ tax_year: '2026-27', finding_id: first.finding_id, decision: 'fix', note: 'GSTR-1 to be amended next month' }])
    // The screen moves on to the next open finding; chosen again, the decided one shows its decision.
    const esc = (s: string) => s.replace(/[.*+?^${}()|[\]\\/]/g, '\\$&')
    showFinding(new RegExp(`${esc(first.rule_id)} .*${esc(first.subject.match_key)}`))
    const decided = screen.getByRole('group', { name: `Decision on ${name(first)}` })
    expect(decided).toHaveTextContent('Fix by A. Partner · 2026-10-16 10:00 UTC')
    expect(decided).toHaveTextContent('GSTR-1 to be amended next month')
    expect(screen.getByLabelText('Result summary')).toHaveTextContent(`${findings.length - 1} open`)
    expect(bannedIn(document.body.textContent ?? '')).toEqual([])
  })

  test('the Decision filter, the history, and figures that changed after a decision', async () => {
    const [a, b] = findings
    const entry = (decision: 'fix' | 'ask_client', on_other_values = false) => ({ decision, note: 'n', decided_by: 'B. Manager', decided_at: '2026-10-15T09:00:00Z', on_other_values })
    await openWith({
      [a.finding_id]: { state: 'ask_client', current: entry('ask_client'), history: [entry('fix'), entry('ask_client')] },
      [b.finding_id]: { state: 'open', current: entry('fix', true), history: [entry('fix', true)] },
    })
    const listed = () => within(screen.getByRole('navigation', { name: 'Findings by document to fix' })).getAllByRole('button')
    fireEvent.change(screen.getByLabelText('Decision filter'), { target: { value: 'ask_client' } })
    expect(listed()).toHaveLength(1)
    expect(within(screen.getByRole('article')).getByText('History (2 decisions)')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Decision filter'), { target: { value: 'open' } })
    expect(listed()).toHaveLength(findings.length - 1)
    showFinding(new RegExp(b.subject.match_key.replace(/[.*+?^${}()|[\]\\/]/g, '\\$&')))
    const reopened = screen.getByRole('group', { name: `Decision on ${b.rule_id} ${b.subject.match_key}` })
    expect(reopened.closest('article')).toHaveTextContent('The figures changed after this decision, so the finding is open again.')
  })
})
