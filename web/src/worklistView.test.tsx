import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { Worklist, type ClassifyFn } from './screens/Worklist.tsx'
import { sampleWorklist } from './sample/index.ts'
import type { WorklistItem } from './types.ts'
import { bannedIn } from './wording.ts'

// Recon Screen Document v2 §9: the Worklist's progress card, change-type chips, dense rows
// with a 12-month strip, inline classification with suggestions, bulk classify, a toast with
// Undo, and "Only items not classified" remembered. The items are the engine's Worklist on
// Test Pack A (fictional).

const table = () => screen.getByRole('table', { name: 'Worklist items' })
const rowOf = (ledger: string) => within(table()).getByText(ledger).closest('tr') as HTMLElement
const bodyRows = () => within(table()).getAllByRole('row').slice(1)

function showAll() {
  fireEvent.change(screen.getByLabelText('Items per page'), { target: { value: '15' } })
}

function classified(it: WorklistItem, text: string): WorklistItem {
  return { ...it, classification: { text, by: 'B. Staff', at: '2026-10-20T09:00:00Z' } }
}

describe('Worklist screen (design §9)', () => {
  beforeEach(() => localStorage.clear())

  test('top card: how many are left, a progress bar and a chip per change type with its count', () => {
    const items = [classified(sampleWorklist[0], 'Clause 44: GST registered supplier'), ...sampleWorklist.slice(1)]
    render(<Worklist items={items} taxYear="2025-26" live onClassify={vi.fn()} />)
    const card = screen.getByRole('group', { name: 'Worklist summary' })
    expect(card).toHaveTextContent('11 left to classify')
    const bar = within(card).getByRole('progressbar', { name: 'Items classified' })
    expect(bar).toHaveAttribute('aria-valuenow', '1')
    expect(bar).toHaveAttribute('aria-valuemax', '12')
    const chips = within(card).getByRole('group', { name: 'Show items by what changed' })
    expect(within(chips).getAllByRole('button').map((b) => b.textContent)).toEqual([
      'All 12',
      'New head 7',
      'Vanished 2',
      'Missing months 2',
      'Step change 1',
    ])

    fireEvent.click(within(chips).getByRole('button', { name: 'Vanished 2' }))
    expect(within(chips).getByRole('button', { name: 'Vanished 2' })).toHaveAttribute('aria-pressed', 'true')
    expect(bodyRows().map((r) => within(r).getByText(/Courier|Office rent - Malviya/).textContent)).toEqual([
      'Courier and postage',
      'Office rent - Malviya Nagar',
    ])
    // Choosing the chip again shows every type.
    fireEvent.click(within(chips).getByRole('button', { name: 'Vanished 2' }))
    expect(bodyRows()).toHaveLength(10)
  })

  test('a row: ledger head with client and group, what changed, both years with months, and the month strip', () => {
    render(<Worklist items={sampleWorklist} taxYear="2025-26" live={false} />)
    showAll()
    const step = rowOf('Travelling expenses - overseas')
    for (const text of ['Nimbus Logic Systems Private Limited (Test Pack A) · Travel', 'Step change', '3.84 times last year',
      '9,83,627.00', '37,72,925.00', '12 months']) expect(step).toHaveTextContent(text)
    expect(within(step).getByRole('img')).toHaveAccessibleName('April to March, entry both years: every month')

    // A missing month is dashed: an entry last year only.
    const gap = rowOf('Electricity charges')
    const strip = within(gap).getByRole('img')
    expect(strip).toHaveAccessibleName('April to March, entry both years: 11 months; entry last year only: November')
    expect(strip.querySelectorAll('i.last')).toHaveLength(1)
    expect(strip.children[7]).toHaveClass('last')

    // A new head with entries in 3 of 12 months: which months is not given, so no strip.
    const partial = rowOf('Conference sponsorship')
    expect(within(partial).queryByRole('img')).toBeNull()
    expect(partial).toHaveTextContent('Months not listed')
    expect(partial).toHaveTextContent('3 months')
    expect(bannedIn(document.body.textContent ?? '')).toEqual([])
  })

  test('the tool never fills a classification in; suggestions are only classifications already written', () => {
    const items = [
      classified(sampleWorklist[7], 'Stopped: documents now sent by email'),
      classified(sampleWorklist[8], 'Office moved'),
      ...sampleWorklist.slice(0, 7),
    ]
    render(<Worklist items={items} taxYear="2025-26" live onClassify={vi.fn()} />)
    for (const it of sampleWorklist.slice(0, 7)) expect(screen.getByLabelText(`Classification of ${it.ledger}`)).toHaveValue('')
    const input = screen.getByLabelText('Classification of Cloud hosting - overseas')
    const list = document.getElementById(input.getAttribute('list') ?? '') as HTMLDataListElement
    expect([...list.options].map((o) => o.value)).toEqual(['Office moved', 'Stopped: documents now sent by email'])
  })

  test('Enter saves; the row turns green with who saved it and Edit; the toast offers Undo where the server can put it back', async () => {
    const onClassify = vi.fn<ClassifyFn>(async () => {})
    const before = classified(sampleWorklist[7], 'Old text')
    const { rerender } = render(<Worklist items={[before]} taxYear="2025-26" live onClassify={onClassify} />)
    const row = rowOf('Courier and postage')
    expect(row).toHaveClass('done')
    expect(row).toHaveTextContent('by B. Staff, 2026-10-20')
    expect(within(row).queryByRole('textbox')).toBeNull()

    fireEvent.click(within(row).getByRole('button', { name: 'Edit the classification of Courier and postage' }))
    const input = screen.getByLabelText('Classification of Courier and postage')
    expect(document.activeElement).toBe(input)
    fireEvent.change(input, { target: { value: 'New text' } })
    await act(async () => {
      fireEvent.keyDown(input, { key: 'Enter' })
    })
    expect(onClassify).toHaveBeenCalledWith(before, 'New text')
    const after = classified(sampleWorklist[7], 'New text')
    rerender(<Worklist items={[after]} taxYear="2025-26" live onClassify={onClassify} />)
    const toast = screen.getByRole('status')
    expect(toast).toHaveTextContent('Saved "New text" for Courier and postage.')
    await act(async () => {
      fireEvent.click(within(toast).getByRole('button', { name: 'Undo' }))
    })
    // Undo names the item as it was saved; the server knows it by client, kind and ledger.
    expect(onClassify).toHaveBeenLastCalledWith(before, 'Old text')
    expect(screen.queryByRole('status')).toBeNull()
  })

  test('a first classification on the server has no Undo: the server refuses an empty classification', async () => {
    const onClassify = vi.fn<ClassifyFn>(async () => {})
    render(<Worklist items={[sampleWorklist[7]]} taxYear="2025-26" live onClassify={onClassify} />)
    const input = screen.getByLabelText('Classification of Courier and postage')
    fireEvent.change(input, { target: { value: 'Stopped' } })
    await act(async () => {
      fireEvent.click(within(input.closest('td') as HTMLElement).getByRole('button', { name: 'Save' }))
    })
    const toast = screen.getByRole('status')
    expect(toast).toHaveTextContent('Use Edit on the row to change it.')
    expect(within(toast).queryByRole('button', { name: 'Undo' })).toBeNull()
    expect(onClassify).toHaveBeenCalledTimes(1)
  })

  test('ticked rows open a bulk bar at the bottom; it classifies each ticked item', async () => {
    const onClassify = vi.fn<ClassifyFn>(async () => {})
    render(<Worklist items={sampleWorklist} taxYear="2025-26" live onClassify={onClassify} />)
    expect(screen.queryByRole('group', { name: 'Classify the ticked items' })).toBeNull()
    fireEvent.click(screen.getByLabelText('Tick Courier and postage'))
    fireEvent.click(screen.getByLabelText('Tick Office rent - Malviya Nagar'))
    const bar = screen.getByRole('group', { name: 'Classify the ticked items' })
    expect(bar).toHaveTextContent('2 ticked')
    const input = within(bar).getByLabelText('Classification for the ticked items')
    fireEvent.change(input, { target: { value: 'Discontinued' } })
    await act(async () => {
      fireEvent.click(within(bar).getByRole('button', { name: 'Save for 2 items' }))
    })
    expect(onClassify.mock.calls.map((c) => [c[0].ledger, c[1]])).toEqual([
      ['Courier and postage', 'Discontinued'],
      ['Office rent - Malviya Nagar', 'Discontinued'],
    ])
    expect(screen.queryByRole('group', { name: 'Classify the ticked items' })).toBeNull()
    expect(screen.getByRole('status')).toHaveTextContent('Saved "Discontinued" for 2 items.')
  })

  test('sample data: Undo puts the row back to not classified', async () => {
    render(<Worklist items={sampleWorklist} taxYear="2025-26" live={false} />)
    const input = screen.getByLabelText('Classification of Cloud hosting - overseas')
    fireEvent.change(input, { target: { value: 'TDS 195' } })
    await act(async () => {
      fireEvent.keyDown(input, { key: 'Enter' })
    })
    expect(rowOf('Cloud hosting - overseas')).toHaveTextContent('this screen only')
    await act(async () => {
      fireEvent.click(within(screen.getByRole('status')).getByRole('button', { name: 'Undo' }))
    })
    expect(screen.getByLabelText('Classification of Cloud hosting - overseas')).toHaveValue('')
  })

  test('"Only items not classified" hides finished rows and is remembered', () => {
    const items = [classified(sampleWorklist[0], 'Done'), ...sampleWorklist.slice(1, 3)]
    const { unmount } = render(<Worklist items={items} taxYear="2025-26" live onClassify={vi.fn()} />)
    expect(bodyRows()).toHaveLength(3)
    fireEvent.click(screen.getByLabelText('Only items not classified'))
    expect(bodyRows()).toHaveLength(2)
    unmount()
    render(<Worklist items={items} taxYear="2025-26" live onClassify={vi.fn()} />)
    expect(screen.getByLabelText('Only items not classified')).toBeChecked()
    expect(bodyRows()).toHaveLength(2)
  })

  test('keyboard: j and k move, Enter edits, Esc returns, / searches', () => {
    render(<Worklist items={sampleWorklist} taxYear="2025-26" live={false} />)
    const t = table()
    expect(document.activeElement).toBe(t)
    fireEvent.keyDown(t, { key: 'j' })
    fireEvent.keyDown(t, { key: 'j' })
    fireEvent.keyDown(t, { key: 'k' })
    expect(bodyRows()[1]).toHaveAttribute('aria-selected', 'true')
    fireEvent.keyDown(t, { key: 'Enter' })
    const input = screen.getByLabelText('Classification of Conference sponsorship')
    expect(document.activeElement).toBe(input)
    // Typing "/" in a field types it; it does not jump to the search.
    fireEvent.keyDown(input, { key: '/' })
    expect(document.activeElement).toBe(input)
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(document.activeElement).toBe(t)

    fireEvent.keyDown(t, { key: '/' })
    const search = screen.getByLabelText('Search the Worklist')
    expect(document.activeElement).toBe(search)
    fireEvent.change(search, { target: { value: 'rent' } })
    expect(bodyRows()).toHaveLength(2)
    fireEvent.keyDown(search, { key: 'Escape' })
    expect(search).toHaveValue('')
    expect(document.activeElement).toBe(t)
  })
})
