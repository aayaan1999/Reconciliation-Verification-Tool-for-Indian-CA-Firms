import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { TeachLayout } from './screens/TeachLayout.tsx'
import type { InspectedField, Inspection, LayoutMapping, TeachResult } from './types.ts'

// SPEC-016: the mapping screen opened on an existing office format. The stored mapping fills the
// screen, the name and the document cannot change, a clean preview shows which rules can run, and
// saving makes a draft by naming the format to the server. The server is stubbed; the file and
// its figures are invented.

const f = (path: string, example: string): InspectedField => ({ path, kind: 'text', example, count: 1 })

const inspection: Inspection = {
  file: 'office_billing_export.json', recognised_as: 'office.office_billing.sales_register v1', truncated: false,
  fields: [
    f('$.gstin', '27ZLVCD9565Z1ZF'), f('$.fp', '082026'),
    f('$.bills[*].bill_no', 'NB-0001'), f('$.bills[*].bill_date', '2026-08-12'), f('$.bills[*].taxable', '100000'), f('$.bills[*].igst', '18000'),
  ],
  lists: [{ path: '$.bills[*]', count: 3 }],
  document_types: [{
    name: 'sales_register', description: "Sales register from the client's books", record_type: 'sales_b2b_invoice',
    columns: [
      { name: 'invoice_no', amount: false, key: true }, { name: 'invoice_date', amount: false, key: false },
      { name: 'taxable_value', amount: true, key: false }, { name: 'igst', amount: true, key: false },
    ],
  }],
}

const stored: LayoutMapping = {
  name: 'office billing export', document_type: 'sales_register', rows: '$.bills[*]', where: [], unit: 'paise',
  columns: [
    { name: 'invoice_no', path: '.bill_no', date_format: '' }, { name: 'invoice_date', path: '.bill_date', date_format: '' },
    { name: 'taxable_value', path: '.taxable', date_format: '' }, { name: 'igst', path: '.igst', date_format: '' },
  ],
  period: { path: '$.fp', format: 'mmyyyy', const: '' }, tax_year: '', gstin_path: '$.gstin', identify: [],
}

const result: TeachResult = {
  profile_id: 'office.office_billing.sales_register', profile_version: '2', document_type: 'sales_register', saved: false, rows: 3,
  tax_year: '2026-27', period: '2026-08', pan_last4: '565Z', warnings: 0, profile_yaml: '',
  preview: [{ match_key: 'NB-0001', locator: '$.bills[0]', fields: { invoice_no: 'NB-0001', invoice_date: '2026-08-12' }, amounts: { taxable_value: '100000.00', igst: '18000.00' } }],
  dry_run: {
    can_run: [{ id: 'RECON-GST-001', title: 'Sales equal GSTR-1' }],
    cannot_check: [{ id: 'RECON-GST-002', title: 'Invoice tax', missing: ['sreg.row.cgst'] }],
  },
}

const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status, headers: { 'content-type': 'application/json' } })

function stubServer() {
  const sent: { checkOnly: string | null; format: string | null; mapping: LayoutMapping }[] = []
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    if (url === '/api/v1/inspect') return json(inspection)
    if (url === '/api/v1/profiles' && init?.body instanceof FormData) {
      const b = init.body
      sent.push({ checkOnly: b.get('check_only') as string | null, format: b.get('format') as string | null, mapping: JSON.parse(String(b.get('mapping'))) })
      return json(b.get('check_only') === 'true' ? result : { ...result, saved: true, draft: true, format_revision: 2 })
    }
    throw new Error('unexpected ' + url)
  }))
  return sent
}

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

async function open(onSaved: (r: TeachResult) => void = () => {}) {
  const file = new File(['{}'], 'office_billing_export.json', { type: 'application/json' })
  await act(async () => {
    render(<TeachLayout file={file} onSaved={onSaved} onCancel={() => {}} edit={{ formatId: 'office.office_billing.sales_register', mapping: stored }} />)
  })
  return screen.getByRole('form', { name: 'Edit this layout' })
}

describe('Mapping screen on an existing format (SPEC-016)', () => {
  test('it opens with the stored mapping; the name and the document cannot change', async () => {
    stubServer()
    const form = await open()
    expect(within(form).getByLabelText('Name of the layout')).toHaveValue('office billing export')
    expect(within(form).getByLabelText('Name of the layout')).toHaveAttribute('readonly')
    expect(within(form).getByLabelText('Document')).toHaveValue('sales_register')
    expect(within(form).getByLabelText('Document')).toBeDisabled()
    expect(within(form).getByLabelText('List of rows')).toHaveValue('$.bills[*]')
    expect(within(form).getByLabelText('Amounts are in')).toHaveValue('paise')
    fireEvent.click(within(form).getByRole('button', { name: 'Next' }))
    expect(within(form).getByLabelText('Invoice number')).toHaveValue('.bill_no')
    expect(within(form).getByLabelText('Taxable value')).toHaveValue('.taxable')
    // Nothing was guessed, so nothing waits to be confirmed.
    expect(within(form).queryAllByRole('button', { name: /^Confirm / })).toHaveLength(0)
  })

  test('the preview shows which rules can run, and saving makes a draft of the named format', async () => {
    const sent = stubServer()
    const saved = vi.fn()
    const form = await open(saved)
    fireEvent.click(within(form).getByRole('button', { name: 'Next' }))
    fireEvent.click(within(form).getByRole('button', { name: /Preview rows/ }))
    const dry = await within(form).findByRole('region', { name: 'Which rules can run' })
    expect(within(dry).getByText('1 can run')).toBeInTheDocument()
    expect(within(dry).getByText('1 cannot check')).toBeInTheDocument()
    expect(within(dry).getByText(/Needs sreg\.row\.cgst/)).toBeInTheDocument()
    expect(within(form).getByText(/The edit can be saved as a draft/)).toBeInTheDocument()

    fireEvent.click(within(form).getByRole('button', { name: /Save as draft/ }))
    await act(async () => {})
    expect(sent.map((s) => s.checkOnly)).toEqual(['true', 'false'])
    expect(sent.every((s) => s.format === 'office.office_billing.sales_register')).toBe(true)
    expect(sent[1].mapping.unit).toBe('paise')
    expect(saved).toHaveBeenCalledOnce()
    expect(saved.mock.calls[0][0]).toMatchObject({ draft: true, format_revision: 2 })
  })
})

describe('Value settings and kept samples on the mapping screen (SPEC-016)', () => {
  const stubKeep = () => {
    const sent: { checkOnly: string | null; keep: string | null; mapping: LayoutMapping }[] = []
    vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
      if (url === '/api/v1/inspect') return json(inspection)
      if (url === '/api/v1/profiles' && init?.body instanceof FormData) {
        const b = init.body
        sent.push({ checkOnly: b.get('check_only') as string | null, keep: b.get('keep_sample') as string | null, mapping: JSON.parse(String(b.get('mapping'))) })
        return json(b.get('check_only') === 'true' ? result : { ...result, saved: true })
      }
      throw new Error('unexpected ' + url)
    }))
    return sent
  }

  test('units include lakhs and crores, and a negative style can be chosen; both are sent', async () => {
    const sent = stubKeep()
    const file = new File(['{}'], 'office_billing_export.json', { type: 'application/json' })
    await act(async () => {
      render(<TeachLayout file={file} onSaved={() => {}} onCancel={() => {}} edit={{ formatId: 'office.office_billing.sales_register', mapping: stored }} />)
    })
    const form = screen.getByRole('form', { name: 'Edit this layout' })
    fireEvent.change(within(form).getByLabelText('Amounts are in'), { target: { value: 'lakhs' } })
    expect(within(form).getByLabelText('Amounts are in')).toHaveValue('lakhs')
    fireEvent.click(within(form).getByRole('radio', { name: /Dr or Cr after the amount/ }))
    fireEvent.click(within(form).getByRole('button', { name: 'Next' }))
    fireEvent.click(within(form).getByRole('button', { name: /Preview rows/ }))
    await within(form).findByRole('region', { name: 'Which rules can run' })
    expect(sent[0].mapping.unit).toBe('lakhs')
    expect(sent[0].mapping.negative).toBe('drcr')
  })

  test('the Partner may keep the sample; others are not offered it', async () => {
    const sent = stubKeep()
    const file = new File(['{}'], 'office_billing_export.json', { type: 'application/json' })
    const open = async (canKeep: boolean) => {
      await act(async () => {
        render(<TeachLayout file={file} canKeepSample={canKeep} onSaved={() => {}} onCancel={() => {}} edit={{ formatId: 'office.office_billing.sales_register', mapping: stored }} />)
      })
      const form = screen.getByRole('form', { name: 'Edit this layout' })
      fireEvent.click(within(form).getByRole('button', { name: 'Next' }))
      fireEvent.click(within(form).getByRole('button', { name: /Preview rows/ }))
      await within(form).findByRole('region', { name: 'Which rules can run' })
      return form
    }
    let form = await open(false)
    expect(within(form).queryByRole('checkbox', { name: /Keep this sample file/ })).not.toBeInTheDocument()
    cleanup()
    form = await open(true)
    const keep = within(form).getByRole('checkbox', { name: /Keep this sample file/ })
    expect(within(form).getByText(/invented or masked/)).toBeInTheDocument()
    fireEvent.click(keep)
    fireEvent.click(within(form).getByRole('button', { name: /Save as draft/ }))
    await act(async () => {})
    expect(sent.at(-1)).toMatchObject({ checkOnly: 'false', keep: 'true' })
    // The preview check never keeps the file.
    expect(sent.filter((s) => s.checkOnly === 'true').every((s) => s.keep === null)).toBe(true)
  })
})
