import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { TeachLayout } from './screens/TeachLayout.tsx'
import type { InspectedField, Inspection } from './types.ts'

// SPEC-022 part A4: the layout wizard's Document list comes from the server (it holds the
// documents the Partner added, filtered by Active and by the uploaded file's format), and an
// office document's columns show the labels, the date checkbox and the required mark it was given.

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const f = (path: string, example: string): InspectedField => ({ path, kind: 'text', example, count: 1 })
const json = (b: unknown) => new Response(JSON.stringify(b), { status: 200, headers: { 'content-type': 'application/json' } })

// An invented return register (synthetic, no real client).
const base: Inspection = {
  file: 'sales_returns.json', recognised_as: '', truncated: false,
  fields: [
    f('$.gstin', '27ZLVCD9565Z1ZF'), f('$.fp', '082026'),
    f('$.rows[*].ref', 'SR-0001'), f('$.rows[*].raised', '12-08-2026'), f('$.rows[*].net', '1000'),
  ],
  lists: [{ path: '$.rows[*]', count: 3 }],
  document_types: [],
}

const office = {
  name: 'sales_return', description: 'Credit notes issued to customers', record_type: 'sales_return_row', origin: 'office' as const,
  columns: [
    { name: 'ref_no', label: 'Reference number', type: 'text' as const, amount: false, key: true, required: true },
    { name: 'doc_date', label: 'Date raised', type: 'date' as const, amount: false, key: false, required: false },
    { name: 'memo', label: 'Memo', type: 'text' as const, amount: false, key: false, required: true },
    { name: 'raised_on', label: 'Raised on text', type: 'text' as const, amount: false, key: false, required: false }, // text: no date checkbox
    { name: 'net_amount', label: 'Net amount', type: 'amount' as const, amount: true, key: false, required: false },
  ],
}

async function open(inspection: Inspection) {
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url === '/api/v1/inspect') return json(inspection)
    throw new Error('unexpected ' + url)
  }))
  const file = new File(['{}'], 'sales_returns.json', { type: 'application/json' })
  await act(async () => {
    render(<TeachLayout file={file} onSaved={() => {}} onCancel={() => {}} />)
  })
  return screen.getByRole('form', { name: 'Teach this layout' })
}

describe('the Document list', () => {
  test('lists the documents the server offers, an office one beside the built-in ones', async () => {
    const form = await open({
      ...base,
      document_types: [
        { name: 'sales_register', description: 'Sales register from the client\'s books', record_type: 'sales_b2b_invoice', columns: [{ name: 'invoice_no', amount: false, key: true }] },
        office,
      ],
    })
    const options = within(within(form).getByLabelText('Document')).getAllByRole('option').map((o) => o.textContent)
    expect(options).toEqual(['Choose…', 'Sales register from the client\'s books', 'Credit notes issued to customers'])
    expect(within(form).queryByText(/No active document accepts a file of this format/)).toBeNull()
  })

  test('says so, and where to add one, when no active document accepts the file\'s format', async () => {
    const form = await open(base)
    expect(within(form).getByLabelText('Document')).toBeInTheDocument()
    expect(within(within(form).getByLabelText('Document')).getAllByRole('option')).toHaveLength(1)
    expect(within(form).getByText('No active document accepts a file of this format; the Partner can add one in Document Master.')).toBeInTheDocument()
    expect(within(form).getByRole('button', { name: 'Next' })).toBeDisabled()
  })
})

describe('the columns of an office document', () => {
  async function toColumns() {
    const form = await open({ ...base, document_types: [office] })
    fireEvent.change(within(form).getByLabelText('Document'), { target: { value: 'sales_return' } })
    fireEvent.click(within(form).getByRole('button', { name: 'Next' }))
    return form
  }

  test('show the labels the Partner gave, not the column names', async () => {
    const form = await toColumns()
    expect(within(form).getByLabelText('Reference number')).toBeInTheDocument()
    expect(within(form).getByLabelText('Date raised')).toBeInTheDocument()
    expect(within(form).getByLabelText('Net amount')).toBeInTheDocument()
    expect(within(form).queryByLabelText('ref_no')).toBeNull()
  })

  test('mark a key as needed to match rows, and a required column as marked in the Document Master', async () => {
    const form = await toColumns()
    const rowOf = (label: string) => within(form).getByLabelText(label).closest('tr') as HTMLElement
    expect(rowOf('Reference number')).toHaveTextContent('Needed: rows are matched on it')
    expect(rowOf('Memo')).toHaveTextContent('Marked required in the Document Master')
    expect(rowOf('Date raised')).not.toHaveTextContent('Needed')
    expect(rowOf('Net amount')).not.toHaveTextContent('required')
  })

  test('offer the dd-mm-yyyy checkbox for a date column, by its type and not by its name', async () => {
    const form = await toColumns()
    // doc_date is a date column; raised_on and net_amount are not.
    const dd = within(form).getByRole('checkbox', { name: 'Date raised is written dd-mm-yyyy' })
    expect(dd).toBeInTheDocument()
    expect(within(form).queryByRole('checkbox', { name: 'Raised on text is written dd-mm-yyyy' })).toBeNull()
    expect(within(form).queryByRole('checkbox', { name: 'Net amount is written dd-mm-yyyy' })).toBeNull()
  })

  test('a date column that does not end in _date still gets the checkbox, and a built-in one that does still gets it', async () => {
    const form = await open({
      ...base,
      document_types: [
        { ...office, columns: [{ name: 'ref_no', label: 'Reference number', type: 'text', amount: false, key: true, required: false },
          { name: 'raised', label: 'Raised', type: 'date', amount: false, key: false, required: false }] },
        { name: 'sales_register', description: 'Sales register', record_type: 'sales_b2b_invoice',
          columns: [{ name: 'invoice_no', amount: false, key: true }, { name: 'invoice_date', amount: false, key: false }] }, // no type: the _date name rule
      ],
    })
    fireEvent.change(within(form).getByLabelText('Document'), { target: { value: 'sales_return' } })
    fireEvent.click(within(form).getByRole('button', { name: 'Next' }))
    expect(within(form).getByRole('checkbox', { name: 'Raised is written dd-mm-yyyy' })).toBeInTheDocument()
    fireEvent.click(within(form).getByRole('button', { name: 'Back' }))
    fireEvent.change(within(form).getByLabelText('Document'), { target: { value: 'sales_register' } })
    fireEvent.click(within(form).getByRole('button', { name: 'Next' }))
    expect(within(form).getByRole('checkbox', { name: /is written dd-mm-yyyy/ })).toBeInTheDocument()
  })
})
