import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { guessColumn, rowOptions, singleFields } from './layout.ts'
import { ClientFile } from './screens/ClientFile.tsx'
import { sampleRules } from './sample/index.ts'
import type { InspectedField, Inspection, LayoutMapping } from './types.ts'

// SPEC-004 part 8.4 (AC-29): the mapping screen, for a file no known format matches.

const f = (path: string, example: string): InspectedField => ({ path, kind: 'text', example, count: 1 })

// A GSTR-1-like layout: invoices inside each customer.
const nested = [
  f('$.gstin', '27ZLVCD9565Z1ZF'), f('$.fp', '082026'), f('$.b2b[*].ctin', '27BMSCV1962U1ZB'),
  f('$.b2b[*].inv[*].inum', 'DBS/26-27/0451'), f('$.b2b[*].inv[*].idt', '03-08-2026'),
  f('$.b2b[*].inv[*].itms[*].itm_det.txval', '325000'),
]

describe('mapping helpers', () => {
  test('a row offers its own fields, and the fields around its list with ^', () => {
    expect(rowOptions(nested, '$.b2b[*].inv[*]').map((o) => o.path)).toEqual(['.inum', '.idt', '.itms[*].itm_det.txval', '^.ctin'])
    expect(rowOptions(nested, '$.b2b[*]').map((o) => o.path)).toEqual(['.ctin', '.inv[*].inum', '.inv[*].idt', '.inv[*].itms[*].itm_det.txval'])
  })

  test('columns are guessed from usual field names, the most telling first', () => {
    const opts = rowOptions(nested, '$.b2b[*].inv[*]')
    expect(guessColumn('invoice_no', opts)).toBe('.inum')
    expect(guessColumn('counterparty_gstin', opts)).toBe('^.ctin')
    expect(guessColumn('taxable_value', opts)).toBe('.itms[*].itm_det.txval')
    expect(guessColumn('cess', opts)).toBe('')
    const register = [f("$.rows[*]['Bill No']", 'A/1'), f('$.rows[*].rate', '65000'), f('$.rows[*].gst_rate', '18')]
    expect(guessColumn('invoice_no', rowOptions(register, '$.rows[*]'))).toBe("['Bill No']")
    expect(guessColumn('rate', rowOptions(register, '$.rows[*]'))).toBe('.gst_rate') // the item price is not the GST rate
  })

  test('in a workbook the period and GSTIN are columns of a sheet, read from its first row', () => {
    const sheets = [f("$['Details'][*]['Return Period']", '082026'), f("$['Sales Register'][*]['Voucher No']", 'INV-1'), f('$.meta.fp', '082026')]
    expect(singleFields(sheets, true).map((x) => x.path)).toEqual(["$['Details'][*]['Return Period']", "$['Sales Register'][*]['Voucher No']"])
    expect(singleFields(sheets, false).map((x) => x.path)).toEqual(['$.meta.fp'])
  })
})

const inspection: Inspection = {
  file: 'office_sales_Aug2026.json', recognised_as: '', truncated: false,
  fields: [
    f('$._synthetic', 'SYNTHETIC TEST DATA'), f("$.Header['Company GSTIN']", '27ZLVCD9565Z1ZF'), f('$.Header.Month', '2026-08'),
    f('$.Vouchers[*].Type', 'Sales'), f("$.Vouchers[*]['Bill No']", 'DBS/26-27/0451'), f("$.Vouchers[*]['Bill Date']", '03-08-2026'),
    f("$.Vouchers[*]['Buyer GSTIN']", '27BMSCV1962U1ZB'), f('$.Vouchers[*].Amounts.Taxable', '325000'),
    f('$.Vouchers[*].Amounts.IGST', '0'), f('$.Vouchers[*].Amounts.Total', '383500'),
  ],
  lists: [{ path: '$.Vouchers[*]', count: 5 }],
  document_types: [{
    name: 'sales_register', description: 'Sales register from the client\'s books', record_type: 'sales_b2b_invoice',
    columns: [
      { name: 'invoice_no', amount: false, key: true }, { name: 'counterparty_gstin', amount: false, key: false },
      { name: 'invoice_date', amount: false, key: false }, { name: 'taxable_value', amount: true, key: false },
      { name: 'igst', amount: true, key: false }, { name: 'invoice_value', amount: true, key: false },
    ],
  }],
}

const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status, headers: { 'content-type': 'application/json' } })

describe('teaching a layout (AC-29)', () => {
  afterEach(() => vi.unstubAllGlobals())

  test('a file no format matches is taught: fields guessed, rows previewed, layout saved', async () => {
    const sent: { mapping: LayoutMapping; checkOnly: string }[] = []
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string, init?: RequestInit) => {
        const form = init?.body as FormData
        if (url === '/api/v1/runs') {
          return json({ error_code: 'PROFILE_NO_MATCH', message: 'PROFILE_NO_MATCH: office_sales_Aug2026.json: the file matches no known json format' }, 422)
        }
        if (url === '/api/v1/inspect') return json(inspection)
        if (url === '/api/v1/profiles') {
          const checkOnly = String(form.get('check_only'))
          sent.push({ mapping: JSON.parse(String(form.get('mapping'))), checkOnly })
          return json({
            profile_id: 'office.office_sales.sales_register', profile_version: '1', document_type: 'sales_register',
            saved: checkOnly !== 'true', rows: 4, tax_year: '2026-27', period: '2026-08', pan_last4: '565Z', warnings: 0, profile_yaml: '',
            preview: [{ match_key: 'DBS/26-27/0451', locator: '$.Vouchers[0]', fields: { invoice_no: 'DBS/26-27/0451', invoice_date: '2026-08-03' },
              amounts: { taxable_value: '325000.00', igst: '0.00', invoice_value: '383500.00' } }],
          })
        }
        throw new Error('unexpected ' + url)
      }),
    )
    render(<ClientFile client={{ id: '1', name: 'Deccan Byte', pan_last4: '565Z', runs: [], last_run_on: '' }} runs={[]}
      rules={sampleRules.rules} live fromUpload taxYears={['2026-27']} onRun={() => {}} />)
    const file = new File(['{}'], 'office_sales_Aug2026.json', { type: 'application/json' })
    fireEvent.change(screen.getByLabelText('Files to check'), { target: { files: [file] } })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Run checks' }))
    })
    expect(screen.getByRole('alert')).toHaveTextContent('PROFILE_NO_MATCH')
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Teach this layout (office_sales_Aug2026.json)' }))
    })
    const form = screen.getByRole('form', { name: 'Teach this layout' })
    // The list of rows, the period (written 2026-08) and the client's GSTIN are guessed. The
    // wizard goes on to the columns (and so to the preview) only once a document is chosen.
    expect(within(form).getByLabelText('List of rows')).toHaveValue('$.Vouchers[*]')
    expect(within(form).getByRole('button', { name: 'Next' })).toBeDisabled()

    fireEvent.change(within(form).getByLabelText('Document'), { target: { value: 'sales_register' } })
    fireEvent.click(within(form).getByRole('button', { name: 'Next' }))
    expect(within(form).getByLabelText('Period field')).toHaveValue('$.Header.Month')
    expect(within(form).getByLabelText("Client's GSTIN field")).toHaveValue("$.Header['Company GSTIN']")
    expect(within(form).getByLabelText('Invoice number')).toHaveValue("['Bill No']")
    expect(within(form).getByLabelText('Customer GSTIN')).toHaveValue("['Buyer GSTIN']")
    expect(within(form).getByLabelText('Taxable value')).toHaveValue('.Amounts.Taxable')
    expect(within(form).getByLabelText('Invoice value')).toHaveValue('.Amounts.Total')
    expect(within(form).getByLabelText('Invoice date is written dd-mm-yyyy')).toBeChecked()
    // Nothing can be saved before the rows are previewed: Save layout is only on the preview step.
    expect(within(form).queryByRole('button', { name: 'Save layout' })).toBeNull()
    fireEvent.change(within(form).getByLabelText('Row filter field'), { target: { value: '.Type' } })
    fireEvent.change(within(form).getByLabelText('Row filter value'), { target: { value: 'Sales' } })

    await act(async () => {
      fireEvent.click(within(form).getByRole('button', { name: 'Preview rows' }))
    })
    expect(sent[0].checkOnly).toBe('true')
    expect(sent[0].mapping).toMatchObject({
      document_type: 'sales_register', rows: '$.Vouchers[*]', unit: 'rupees', where: [{ path: '.Type', equals: 'Sales' }],
      period: { path: '$.Header.Month', format: '' }, gstin_path: "$.Header['Company GSTIN']",
    })
    expect(sent[0].mapping.columns).toContainEqual({ name: 'invoice_date', path: "['Bill Date']", date_format: 'dd-mm-yyyy' })
    const rows = screen.getByLabelText('Rows read')
    expect(rows).toHaveTextContent('4 rows read · period 2026-08 · tax year 2026-27 · PAN ending 565Z')
    expect(rows).toHaveTextContent('325000.00')

    await act(async () => {
      fireEvent.click(within(form).getByRole('button', { name: 'Save layout' }))
    })
    expect(sent[1].checkOnly).toBe('false')
    expect(screen.queryByRole('form', { name: 'Teach this layout' })).toBeNull()
    expect(document.body).toHaveTextContent('Layout saved as office.office_sales.sales_register. Press Run checks again.')
  })

  test('a refused mapping shows each problem', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) => {
        if (url === '/api/v1/runs') return json({ error_code: 'PROFILE_NO_MATCH', message: 'PROFILE_NO_MATCH: x.json: no match' }, 422)
        if (url === '/api/v1/inspect') return json(inspection)
        return json({ error_code: 'INVALID_MAPPING', message: 'the file is already read as generic.x v1', problems: [
          { field: 'identify', code: 'PROFILE_AMBIGUOUS', message: 'the file is already read as generic.x v1' },
          { field: 'columns', code: 'MISSING_KEY_COLUMN', message: 'choose the field that holds invoice_no' },
        ] }, 422)
      }),
    )
    render(<ClientFile client={{ id: '1', name: 'C', pan_last4: '565Z', runs: [], last_run_on: '' }} runs={[]}
      rules={sampleRules.rules} live fromUpload taxYears={['2026-27']} onRun={() => {}} />)
    fireEvent.change(screen.getByLabelText('Files to check'), { target: { files: [new File(['{}'], 'x.json')] } })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Run checks' }))
    })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Teach this layout (x.json)' }))
    })
    const form = screen.getByRole('form', { name: 'Teach this layout' })
    fireEvent.change(within(form).getByLabelText('Document'), { target: { value: 'sales_register' } })
    fireEvent.click(within(form).getByRole('button', { name: 'Next' }))
    await act(async () => {
      fireEvent.click(within(form).getByRole('button', { name: 'Preview rows' }))
    })
    const alert = within(form).getByRole('alert')
    expect(alert).toHaveTextContent('INVALID_MAPPING')
    expect(alert).toHaveTextContent('columns: choose the field that holds invoice_no')
    expect(within(form).getByRole('button', { name: 'Save layout' })).toBeDisabled()
  })
})
