import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterAll, afterEach, beforeAll, describe, expect, test, vi } from 'vitest'
import App from './App.tsx'
import { features } from './features.ts'
import { sampleComparisons, sampleRules, sampleWorklist } from './sample/index.ts'
import type { WorklistItem } from './types.ts'
import { bannedIn } from './wording.ts'
import { railTo } from './testFindings.ts'

// SPEC-006 part 3, AC-15: the Worklist screen on the database's items, with a
// classification column that staff fill in. The items are the engine's Worklist on Test
// Pack A (fictional). The Worklist is hidden for now (features.ts); these tests turn it on.
beforeAll(() => {
  features.worklist = true
})
afterAll(() => {
  features.worklist = false
})

const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status, headers: { 'content-type': 'application/json' } })

async function openWorklist(items: WorklistItem[], sent: unknown[] = []) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      if (url === '/api/v1/rules') return json(sampleRules)
      if (url === '/api/v1/comparisons') return json({ comparisons: sampleComparisons })
      if (url.startsWith('/api/v1/clients?')) return json({ tax_year: '2026-27', clients: [] })
      if (url.startsWith('/api/v1/worklist?')) return json({ tax_year: '2026-27', items })
      if (url === '/api/v1/clients/1/worklist') {
        const body = JSON.parse(String(init?.body))
        sent.push(body)
        if (body.ledger === 'Nowhere') return json({ error_code: 'WORKLIST_ITEM_NOT_FOUND', message: 'not an item' }, 404)
        return json({ tax_year: body.tax_year, items: items.map((it) => it.kind === body.kind && it.ledger === body.ledger
          ? { ...it, classification: { text: body.classification, by: 'A. Partner', at: '2026-10-16T10:00:00Z' } } : it) })
      }
      throw new Error('unexpected ' + url)
    }),
  )
  render(<App />)
  await act(async () => {})
  await act(async () => {})
  await act(async () => {
    railTo('Worklist')
  })
  await act(async () => {})
}

describe('Worklist (SPEC-006 part 3)', () => {
  afterEach(() => vi.unstubAllGlobals())

  test('AC-15: the database items, both years and what was counted; staff classify (the server records who)', async () => {
    const sent: unknown[] = []
    await openWorklist(sampleWorklist, sent)
    fireEvent.change(screen.getByLabelText('Items per page'), { target: { value: '15' } })
    const table = screen.getByRole('table', { name: 'Worklist items' })
    expect(within(table).getAllByRole('row')).toHaveLength(13)
    expect(screen.getByText('12 items, 12 not classified yet.')).toBeInTheDocument()
    const step = within(table).getByText('Travelling expenses - overseas').closest('tr') as HTMLElement
    for (const text of ['Step change', '9,83,627.00', '37,72,925.00', '3.84 times last year']) expect(step).toHaveTextContent(text)
    const gap = within(table).getByText('Electricity charges').closest('tr') as HTMLElement
    expect(gap).toHaveTextContent('every month last year; none for 2025-11')
    // The tool never fills a classification in.
    for (const it of sampleWorklist) expect(screen.getByLabelText(`Classification of ${it.ledger}`)).toHaveValue('')

    // There is no name field: the server records the signed-in user (SPEC-012).
    expect(screen.queryByLabelText('Your name')).not.toBeInTheDocument()
    const input = screen.getByLabelText('Classification of Courier and postage')
    fireEvent.change(input, { target: { value: 'Stopped: documents now sent by email' } })
    await act(async () => {
      fireEvent.submit((input as HTMLInputElement).form as HTMLFormElement) // what Enter in the field does
    })
    expect(sent).toEqual([{ tax_year: '2025-26', kind: 'vanished', ledger: 'Courier and postage', classification: 'Stopped: documents now sent by email' }])
    // A saved row shows the classification and who saved it, with Edit to change it.
    const saved = within(table).getByText('Courier and postage').closest('tr') as HTMLElement
    const cell = within(saved).getByText('Stopped: documents now sent by email').closest('td') as HTMLElement
    expect(cell).toHaveTextContent('by A. Partner, 2026-10-16')
    fireEvent.click(within(cell).getByRole('button', { name: 'Edit the classification of Courier and postage' }))
    expect(screen.getByLabelText('Classification of Courier and postage')).toHaveValue('Stopped: documents now sent by email')
    expect(screen.getByText('12 items, 11 not classified yet.')).toBeInTheDocument()

    fireEvent.click(screen.getByLabelText('Only items not classified'))
    expect(within(table).getAllByRole('row')).toHaveLength(12)
    expect(bannedIn(document.body.textContent ?? '')).toEqual([])
  })

  test('an error from the server is shown on the item', async () => {
    const odd = { ...sampleWorklist[0], ledger: 'Nowhere', key: 'new_head|Nowhere' }
    await openWorklist([odd])
    const input = screen.getByLabelText('Classification of Nowhere')
    fireEvent.change(input, { target: { value: 'x' } })
    await act(async () => {
      fireEvent.click(within(input.closest('td') as HTMLElement).getByRole('button', { name: 'Save' }))
    })
    expect(screen.getByRole('alert')).toHaveTextContent('not an item')
  })

  test('no items yet: says how to get them', async () => {
    await openWorklist([])
    expect(screen.getByText(/Upload this year's and last year's expense ledger/)).toBeInTheDocument()
  })
})
