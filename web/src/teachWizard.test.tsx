import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { columnsOf } from './layout.ts'
import { TeachLayout } from './screens/TeachLayout.tsx'
import type { InspectedField, Inspection, TeachResult } from './types.ts'

// Recon Screen Document v2 §6.3: Teach this layout as a three-step wizard (About the file, Map
// the columns, Preview and save), columns marked Suggested until changed or confirmed, the
// file's values beside the mapping, a preview that runs on its own, and Save only after it reads.

const f = (path: string, example: string): InspectedField => ({ path, kind: 'text', example, count: 1 })

// An invented billing export (synthetic, no real client).
const inspection: Inspection = {
  file: 'office_billing_export.json', recognised_as: '', truncated: false,
  fields: [
    f('$.gstin', '27ZLVCD9565Z1ZF'), f('$.fp', '082026'),
    f('$.bills[*].bill_no', 'NB-0001'), f('$.bills[*].bill_date', '2026-08-12'), f('$.bills[*].party_name', 'Orion Retail'),
    f('$.bills[*].taxable', '100000'), f('$.bills[*].igst', '18000'),
  ],
  lists: [{ path: '$.bills[*]', count: 3 }],
  document_types: [{
    name: 'sales_register', description: 'Sales register from the client\'s books', record_type: 'sales_b2b_invoice',
    columns: [
      { name: 'invoice_no', amount: false, key: true }, { name: 'invoice_date', amount: false, key: false },
      { name: 'counterparty_gstin', amount: false, key: false }, { name: 'taxable_value', amount: true, key: false },
      { name: 'igst', amount: true, key: false },
    ],
  }],
}

const result: TeachResult = {
  profile_id: 'office.office_billing.sales_register', profile_version: '1', document_type: 'sales_register',
  saved: false, rows: 3, tax_year: '2026-27', period: '2026-08', pan_last4: '565Z', warnings: 0, profile_yaml: '',
  preview: [
    { match_key: 'NB-0001', locator: '$.bills[0]', fields: { invoice_no: 'NB-0001', invoice_date: '2026-08-12' }, amounts: { taxable_value: '100000.00', igst: '18000.00' } },
    { match_key: 'NB-0002', locator: '$.bills[1]', fields: { invoice_no: 'NB-0002', invoice_date: '2026-08-19' }, amounts: { taxable_value: '50000.00', igst: '9000.00' } },
  ],
}

const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status, headers: { 'content-type': 'application/json' } })

// stubServer answers inspect with the billing export and each teach call with the next answer.
function stubServer(...answers: Response[]) {
  const sent: string[] = []
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    if (url === '/api/v1/inspect') return json(inspection)
    if (url === '/api/v1/profiles') {
      const body = init?.body
      if (!(body instanceof FormData)) throw new Error('profiles takes a form')
      sent.push(String(body.get('check_only')))
      const next = answers.shift()
      if (!next) throw new Error('no answer left')
      return next
    }
    throw new Error('unexpected ' + url)
  }))
  return sent
}

async function open(props: { onSaved?: () => void; onCancel?: () => void } = {}) {
  const file = new File(['{}'], 'office_billing_export.json', { type: 'application/json' })
  await act(async () => {
    render(<TeachLayout file={file} onSaved={props.onSaved ?? (() => {})} onCancel={props.onCancel ?? (() => {})} />)
  })
  return screen.getByRole('form', { name: 'Teach this layout' })
}

const current = () => within(screen.getByRole('list', { name: 'Steps' })).getAllByRole('listitem').find((li) => li.getAttribute('aria-current') === 'step')

function toColumns(form: HTMLElement) {
  fireEvent.change(within(form).getByLabelText('Document'), { target: { value: 'sales_register' } })
  fireEvent.click(within(form).getByRole('button', { name: 'Next' }))
}

describe('Teach this layout wizard (§6.3)', () => {
  afterEach(() => vi.unstubAllGlobals())

  test('three steps are shown, the first is About the file with name, document, list of rows and unit', async () => {
    stubServer()
    const form = await open()
    const steps = within(screen.getByRole('list', { name: 'Steps' })).getAllByRole('listitem')
    expect(steps.map((s) => s.textContent)).toEqual(['1About the file', '2Map the columns', '3Preview and save'])
    expect(current()).toHaveTextContent('About the file')
    expect(within(form).getByLabelText('Name of the layout')).toHaveValue('office billing export')
    expect(within(form).getByLabelText('List of rows')).toHaveValue('$.bills[*]')
    expect(within(form).getByRole('radio', { name: 'Rupees' })).toBeChecked()
    expect(within(form).getByRole('radio', { name: 'Paise' })).not.toBeChecked()
    expect(within(form).getByRole('button', { name: 'Next' })).toBeDisabled()
  })

  test('Enter moves on to the columns, and Back returns with the choices kept', async () => {
    stubServer()
    const form = await open()
    fireEvent.change(within(form).getByLabelText('Document'), { target: { value: 'sales_register' } })
    fireEvent.submit(form) // what Enter in a field does
    expect(current()).toHaveTextContent('Map the columns')
    fireEvent.click(within(form).getByRole('button', { name: 'Back' }))
    expect(current()).toHaveTextContent('About the file')
    expect(within(form).getByLabelText('Document')).toHaveValue('sales_register')
  })

  test('columns guessed from field names are marked Suggested until changed or confirmed', async () => {
    stubServer()
    const form = await open()
    toColumns(form)
    expect(within(form).getByLabelText('Invoice number')).toHaveValue('.bill_no')
    expect(within(form).getByLabelText('Taxable value')).toHaveValue('.taxable')
    expect(within(form).getByLabelText('Customer GSTIN')).toHaveValue('')
    expect(within(form).getAllByRole('button', { name: /^Confirm / }).map((b) => b.textContent)).toEqual(['Suggested', 'Suggested', 'Suggested', 'Suggested'])
    expect(within(form).queryByRole('button', { name: /^Confirm Customer GSTIN/ })).toBeNull()

    fireEvent.click(within(form).getByRole('button', { name: 'Confirm Invoice number: .bill_no' }))
    expect(within(form).queryByRole('button', { name: /^Confirm Invoice number/ })).toBeNull()
    fireEvent.change(within(form).getByLabelText('IGST'), { target: { value: '.taxable' } })
    expect(within(form).queryByRole('button', { name: /^Confirm IGST/ })).toBeNull()
    expect(within(form).getByRole('button', { name: 'Confirm Taxable value: .taxable' })).toBeInTheDocument()
  })

  test("the file's values stay beside the mapping, each mapped field labelled with its column", async () => {
    stubServer()
    const form = await open()
    toColumns(form)
    const aside = within(form).getByRole('complementary', { name: 'First values in the file' })
    expect(aside).toHaveTextContent('$.bills[*] · 3 rows')
    const items = within(aside).getAllByRole('listitem')
    expect(items[0]).toHaveTextContent('.bill_no')
    expect(items[0]).toHaveTextContent('NB-0001')
    expect(items[0]).toHaveTextContent('Invoice number')
    const party = items.find((li) => li.textContent?.includes('.party_name'))
    expect(party).toHaveTextContent('Orion Retail')
    expect(party).not.toHaveClass('mapped')
  })

  test('the preview runs on its own when step 3 opens, shows where each row came from, and then Save is enabled', async () => {
    const sent = stubServer(json(result), json({ ...result, saved: true }))
    const onSaved = vi.fn()
    const form = await open({ onSaved })
    toColumns(form)
    await act(async () => {
      fireEvent.click(within(form).getByRole('button', { name: 'Preview rows' }))
    })
    expect(current()).toHaveTextContent('Preview and save')
    expect(sent).toEqual(['true'])
    const read = within(form).getByLabelText('Rows read')
    expect(read).toHaveTextContent('3 rows read · period 2026-08 · tax year 2026-27 · PAN ending 565Z. The layout can be saved.')
    const row = within(read).getByRole('row', { name: /NB-0002/ })
    for (const text of ['$.bills[1]', '2026-08-19', '50000.00', '9000.00']) expect(row).toHaveTextContent(text)

    await act(async () => {
      fireEvent.click(within(form).getByRole('button', { name: 'Save layout' }))
    })
    expect(sent).toEqual(['true', 'false'])
    expect(onSaved).toHaveBeenCalledWith(expect.objectContaining({ saved: true, profile_id: 'office.office_billing.sales_register' }))
  })

  test('a wrong choice gives a clear error with a link back to the columns, and Save stays off', async () => {
    stubServer(json({ error_code: 'INVALID_MAPPING', message: 'the mapping is refused', problems: [
      { field: 'columns', code: 'NOT_A_NUMBER', message: 'taxable_value: .bill_no holds NB-0001, not an amount' },
      { field: 'period', code: 'BAD_PERIOD', message: 'the period field is empty' },
    ] }, 422))
    const form = await open()
    toColumns(form)
    await act(async () => {
      fireEvent.click(within(form).getByRole('button', { name: 'Preview rows' }))
    })
    const alert = within(form).getByRole('alert')
    expect(alert).toHaveTextContent('INVALID_MAPPING')
    expect(alert).toHaveTextContent('columns: taxable_value: .bill_no holds NB-0001, not an amount')
    expect(within(form).getByRole('button', { name: 'Save layout' })).toBeDisabled()
    fireEvent.click(within(alert).getByRole('button', { name: 'Back to the columns' }))
    expect(current()).toHaveTextContent('Map the columns')
  })

  test('a preview that reads no rows keeps Save off', async () => {
    stubServer(json({ ...result, rows: 0, preview: [] }))
    const form = await open()
    toColumns(form)
    await act(async () => {
      fireEvent.click(within(form).getByRole('button', { name: 'Preview rows' }))
    })
    expect(within(form).getByRole('alert')).toHaveTextContent('No rows were read from $.bills[*].')
    expect(within(form).getByRole('button', { name: 'Save layout' })).toBeDisabled()
  })

  test('Esc closes at once when nothing is chosen, and asks first once a document is chosen', async () => {
    stubServer()
    const onCancel = vi.fn()
    let form = await open({ onCancel })
    fireEvent.keyDown(form, { key: 'Escape' })
    expect(onCancel).toHaveBeenCalledTimes(1)

    onCancel.mockClear()
    cleanup()
    form = await open({ onCancel })
    fireEvent.change(within(form).getByLabelText('Document'), { target: { value: 'sales_register' } })
    fireEvent.keyDown(form, { key: 'Escape' })
    expect(onCancel).not.toHaveBeenCalled()
    expect(within(form).getByRole('group', { name: 'Discard this mapping?' })).toBeInTheDocument()
    fireEvent.keyDown(form, { key: 'Escape' })
    expect(onCancel).toHaveBeenCalledTimes(1)
  })
})

describe('columnsOf', () => {
  test('names the columns mapped to each field, leaving out columns not in the file', () => {
    expect(columnsOf({ invoice_no: '.bill_no', igst: '.amt', cgst: '.amt', cess: '' })).toEqual({ '.bill_no': ['invoice_no'], '.amt': ['igst', 'cgst'] })
  })
})
