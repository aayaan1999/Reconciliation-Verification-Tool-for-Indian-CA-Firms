import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { filterRules, noRuleFilter } from './filters.ts'
import { blankForm } from './ruleForm.ts'
import { RuleBuilder } from './screens/RuleBuilder.tsx'
import { Rules } from './screens/Rules.tsx'
import type { RuleInfo, RuleOptions, RuleRecordOption } from './types.ts'

// SPEC-022 part B3: the Rule Builder offers the rows of the documents the Partner added beside the
// built-in ones, shows their columns with their labels, offers only the check types that read rows,
// and the Rules tab marks the rules that read them. The server is stubbed; every name is invented.

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const columns: NonNullable<RuleRecordOption['columns']> = [
  { name: 'ref_no', label: 'Reference', type: 'text', key: true, required: true },
  { name: 'customer', label: 'Customer', type: 'text', key: false, required: false },
  { name: 'net_amount', label: 'Net amount', type: 'amount', key: false, required: false },
  { name: 'tax_amount', label: 'Tax amount', type: 'amount', key: false, required: false },
]

const gstInvoice: RuleRecordOption = {
  name: 'gstr1_b2b_invoice', description: 'GSTR-1 invoices', document_type: 'gstr1', match_key: ['invoice_no'],
  amounts: ['taxable_value'], fields: ['invoice_no'], origin: 'builtin', active: true,
}
const salesReturn: RuleRecordOption = {
  name: 'sales_return_row', description: 'Rows of the sales return register', document_type: 'sales_return', match_key: ['ref_no'],
  amounts: ['net_amount', 'tax_amount'], fields: ['ref_no', 'customer'], origin: 'office', active: true, columns,
}
const switchedOff: RuleRecordOption = {
  ...salesReturn, name: 'old_returns_row', document_type: 'old_returns', description: 'A document switched off', active: false,
}

const options = (records: RuleRecordOption[]): RuleOptions => ({
  modules: ['gst', 'office', 'tieout'],
  severities: ['high', 'medium', 'low'],
  categories: [{ code: 'R1', name: 'Same figure in two places', description: '' }],
  check_types: ['equal', 'gte', 'match_records', 'percent_of', 'present_together', 'sum_equals', 'table_lookup'],
  row_check_types: ['match_records', 'present_together', 'sum_equals'],
  facts: [{ name: 'computation.allowed_depreciation', kind: 'money', periodicity: 'FY', document_type: 'computation', description: '' }],
  record_types: records,
  document_types: [{ name: 'computation', description: 'Computation' }, { name: 'sales_return', description: 'Sales return register' }],
})

function open(o: RuleOptions) {
  render(<RuleBuilder mode="create" current={null} options={o} taxYears={['2026-27']} onSaved={() => {}} onClose={() => {}} />)
  return screen.getByRole('form', { name: /rule/i })
}

// addRows adds an input of the kind "Rows of a record type" and returns the form.
function addRows(form: HTMLElement) {
  fireEvent.click(within(form).getByRole('button', { name: 'Add a figure or rows' }))
  const kinds = within(form).getAllByLabelText(/^Kind \d+$/)
  fireEvent.change(kinds[kinds.length - 1], { target: { value: 'record_type' } })
  const names = within(form).getAllByLabelText(/^Name in the rule \d+$/)
  fireEvent.change(names[names.length - 1], { target: { value: 'a' } })
}

describe('the rows a rule can read', () => {
  test('are shown in two groups, Built-in and Office documents, once a document has been added', () => {
    const form = open(options([gstInvoice, salesReturn]))
    addRows(form)
    const builtin = within(form).getByRole('group', { name: 'Built-in' })
    const office = within(form).getByRole('group', { name: 'Office documents' })
    expect(within(builtin).getAllByRole('button').map((b) => b.textContent)).toEqual(['gstr1_b2b_invoice'])
    expect(within(office).getAllByRole('button').map((b) => b.textContent)).toEqual(['sales_return_row'])
    expect(within(form).queryByText(/once the Partner has added one in Document Master/)).toBeNull()
  })

  test('are shown as they always were, with a line saying how to add a document, when none is ready', () => {
    const form = open(options([gstInvoice]))
    addRows(form)
    expect(within(form).queryByRole('group', { name: 'Built-in' })).toBeNull()
    expect(within(form).getByRole('button', { name: 'gstr1_b2b_invoice' })).toBeInTheDocument()
    expect(within(form).getByText(/once the Partner has added one in Document Master \(Admin\)/)).toBeInTheDocument()
  })

  test('leave out a document that is switched off, which a rule already reading it keeps', () => {
    const form = open(options([gstInvoice, salesReturn, switchedOff]))
    addRows(form)
    const office = within(form).getByRole('group', { name: 'Office documents' })
    expect(within(office).queryByRole('button', { name: 'old_returns_row' })).toBeNull()

    cleanup()
    const reading = { ...blankForm(options([gstInvoice, salesReturn, switchedOff]), '2026-27'), module: 'office',
      inputs: [{ name: 'a', kind: 'record_type' as const, ref: 'old_returns_row', label: 'Old returns', optional: false, default: '' }] }
    render(<RuleBuilder mode="edit" current={{ rule_id: 'RECON-OFF-9', version: '1', source: 'office', status: 'testing', disabled_reason: '', editable: true, reason: '',
      approvable: false, approval_note: '', form: reading, fixtures: [] }}
      options={options([gstInvoice, salesReturn, switchedOff])} taxYears={['2026-27']} onSaved={() => {}} onClose={() => {}} />)
    const kept = within(screen.getByRole('group', { name: 'Office documents' })).getByRole('button', { name: 'old_returns_row' })
    expect(kept).toHaveAttribute('aria-pressed', 'true')
  })
})

describe('choosing the rows of an office document', () => {
  function chooseOffice() {
    const form = open(options([gstInvoice, salesReturn]))
    addRows(form)
    fireEvent.click(within(within(form).getByRole('group', { name: 'Office documents' })).getByRole('button', { name: 'sales_return_row' }))
    return form
  }

  test('shows its columns with the labels the Partner gave them', () => {
    const form = chooseOffice()
    const table = within(form).getByRole('table', { name: 'Columns of sales_return_row' })
    const rows = within(table).getAllByRole('row').slice(1).map((r) => within(r).getAllByRole('cell').map((c) => c.textContent))
    expect(rows).toEqual([
      ['Reference', 'ref_no', 'text', 'Yes'], ['Customer', 'customer', 'text', 'No'],
      ['Net amount', 'net_amount', 'amount', 'No'], ['Tax amount', 'tax_amount', 'amount', 'No'],
    ])
  })

  test('puts the rule in the office module, and says why', () => {
    const form = chooseOffice()
    expect(within(form).getByLabelText('Module')).toHaveValue('office')
    expect(within(form).getByText('A rule over a document added in the Document Master is kept in the office module.')).toBeInTheDocument()
  })

  test('offers only the check types that read rows, and marks one already chosen that cannot', () => {
    const form = chooseOffice()
    const types = () => within(within(form).getByLabelText('Check type')).getAllByRole('option').map((o) => o.textContent)
    // the type the form opened with (the first in the list) cannot read rows, so it stays visible, marked and not choosable
    expect(types()).toEqual(['equal (reads single figures, not rows of an office document)', 'match_records', 'present_together', 'sum_equals'])
    expect(within(within(form).getByLabelText('Check type')).getByRole('option', { name: /^equal/ })).toBeDisabled()
    fireEvent.change(within(form).getByLabelText('Check type'), { target: { value: 'sum_equals' } })
    expect(types()).toEqual(['match_records', 'present_together', 'sum_equals'])
  })

  test('leaves every check type on offer while the rule reads built-in rows only', () => {
    const form = open(options([gstInvoice, salesReturn]))
    addRows(form)
    fireEvent.click(within(within(form).getByRole('group', { name: 'Built-in' })).getByRole('button', { name: 'gstr1_b2b_invoice' }))
    expect(within(within(form).getByLabelText('Check type')).getAllByRole('option')).toHaveLength(7)
    expect(within(form).getByLabelText('Module')).not.toHaveValue('office')
  })

  test('shows the labels of its columns in the fixture editor', () => {
    const form = chooseOffice()
    fireEvent.click(within(form).getByRole('button', { name: 'Add a pass fixture' }))
    fireEvent.click(within(form).getByRole('button', { name: 'Add a row to this fixture' }))
    fireEvent.change(within(form).getByLabelText('Record type of row 1, fixture 1'), { target: { value: 'sales_return_row' } })
    fireEvent.click(within(form).getAllByRole('button', { name: 'Add a column' })[0])
    const amounts = within(form).getAllByLabelText(/^Column 1 of amounts of the row$/)[0]
    expect(within(amounts).getAllByRole('option').map((o) => o.textContent)).toEqual(['(choose)', 'Net amount (net_amount)', 'Tax amount (tax_amount)'])
  })

  test('a built-in record type in the fixture editor keeps showing its plain names', () => {
    const form = open(options([gstInvoice, salesReturn]))
    addRows(form)
    fireEvent.click(within(within(form).getByRole('group', { name: 'Built-in' })).getByRole('button', { name: 'gstr1_b2b_invoice' }))
    fireEvent.click(within(form).getByRole('button', { name: 'Add a pass fixture' }))
    fireEvent.click(within(form).getByRole('button', { name: 'Add a row to this fixture' }))
    fireEvent.change(within(form).getByLabelText('Record type of row 1, fixture 1'), { target: { value: 'gstr1_b2b_invoice' } })
    fireEvent.click(within(form).getAllByRole('button', { name: 'Add a column' })[0])
    const amounts = within(form).getAllByLabelText(/^Column 1 of amounts of the row$/)[0]
    expect(within(amounts).getAllByRole('option').map((o) => o.textContent)).toEqual(['(choose)', 'taxable_value'])
  })
})

// The Rules tab.
const rule = (id: string, extra: Partial<RuleInfo> = {}): RuleInfo => ({
  id, version: '1', revision: 1, source: 'builtin', status: 'active', runnable: true, type: 'tie-out', domain: '', category: 'R1', title: `Rule ${id}`,
  tax_years: ['2026-27'], valid_from: '2026-04-01', valid_to: '', file_path: `${id}.yaml`, approvable: false, approval_note: '', ...extra,
})

describe('the Rules tab', () => {
  const list = [rule('RECON-GST-001'), rule('RECON-OFF-101', { source: 'office', status: 'testing', reads_office_documents: true, title: 'Sales returns add up' })]

  function show(rules: RuleInfo[]) {
    render(<Rules list={{ rules, tax_years: ['2026-27'] }} live={false} />)
  }

  test('marks a rule that reads an office document with a chip, and no other', () => {
    show(list)
    const office = screen.getByText('Sales returns add up').closest('tr') as HTMLElement
    expect(within(office).getByText('Office document')).toBeInTheDocument()
    const built = screen.getByText('Rule RECON-GST-001').closest('tr') as HTMLElement
    expect(within(built).queryByText('Office document')).toBeNull()
  })

  test('has a filter for them, shown only when some rule reads one', () => {
    show(list)
    expect(screen.getByRole('button', { name: /^Reads/ })).toBeInTheDocument()
    cleanup()
    show([rule('RECON-GST-001')])
    expect(screen.queryByRole('button', { name: /^Reads/ })).toBeNull()
  })

  test('the filter keeps the rules that read office documents', () => {
    expect(filterRules(list, { ...noRuleFilter, reads: ['office'] }).map((r) => r.id)).toEqual(['RECON-OFF-101'])
    expect(filterRules(list, noRuleFilter).map((r) => r.id)).toEqual(['RECON-GST-001', 'RECON-OFF-101'])
    // an older server sends no flag: no rule is taken to read an office document
    expect(filterRules([rule('RECON-X-001')], { ...noRuleFilter, reads: ['office'] })).toEqual([])
  })

  test('choosing the filter narrows the table', async () => {
    show(list)
    fireEvent.click(screen.getByRole('button', { name: /^Reads/ }))
    await act(async () => {})
    fireEvent.click(screen.getByRole('checkbox', { name: 'Office documents' }))
    expect(screen.queryByText('Rule RECON-GST-001')).toBeNull()
    expect(screen.getByText('Sales returns add up')).toBeInTheDocument()
  })
})
