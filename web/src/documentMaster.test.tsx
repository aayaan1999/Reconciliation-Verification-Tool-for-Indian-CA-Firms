import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import type { DocColumn, DocDetail, DocRow } from './docMasterTypes.ts'
import { PermissionContext, canFrom } from './permissions.ts'
import { Admin } from './screens/Admin.tsx'
import { DocumentMasterPanel } from './screens/DocumentMaster.tsx'
import type { Account } from './types.ts'

// SPEC-022 part A4: the Document Master tab of the Admin screen. The server is a small in-memory
// stand-in for /api/v1/document-types; every name is invented.

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })

const col = (name: string, label: string, type: DocColumn['type'], key = false, required = false): DocColumn => ({ name, label, type, key, required })
const salesColumns = [col('ref_no', 'Reference', 'text', true, true), col('doc_date', 'Date', 'date', true), col('net_amount', 'Net amount', 'amount')]

const row = (code: string, name: string, extra: Partial<DocRow> = {}): DocRow => ({
  code, origin: 'office', name, category: 'Books', description: `${name}, described for the test`, applicable_for: ['xlsx', 'csv'], status: 'active',
  record_type: `${code}_row`, updated_by: 'priya', updated_at: '2026-11-05T09:00:00Z', referenced_by: { formats: 0, rules: 0 }, ...extra,
})

const gstr1 = row('gstr1', 'Gstr1', { origin: 'builtin', category: 'GST', applicable_for: ['json', 'xlsx', 'csv'], record_type: 'gstr1_b2b_invoice' })
const salesReturn = row('sales_return', 'Sales return register')
const debitNotes = row('debit_notes', 'Debit notes', { status: 'draft', record_type: '', applicable_for: ['xlsx'] })
const bankExtra = row('bank_extra', 'Bank extra lines', { category: 'Bank', status: 'inactive', applicable_for: ['json'], referenced_by: { formats: 1, rules: 0 } })

function detailOf(r: DocRow, extra: Partial<DocDetail> = {}): DocDetail {
  return {
    ...r,
    structure: r.record_type ? { name: r.record_type, description: 'Rows of the register', copied_from: '', locked: false, columns: salesColumns } : null,
    usage: { formats: [], rules: [], active_rules: [], records: 0 },
    history: [{ at: '2026-11-05T09:00:00Z', by: 'priya', action: 'document.add', details: '{}' }],
    ...extra,
  }
}

interface Call { url: string; method: string; body: unknown }

// A stand-in server. docs is the list it holds; details answers GET of one document; extra routes
// are tried first.
function stubServer(docs: DocRow[], details: Record<string, DocDetail> = {}, ...routes: ((c: Call) => Response | undefined)[]) {
  const calls: Call[] = []
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    const body = init?.body ? JSON.parse(String(init.body)) : undefined
    const call = { url, method, body }
    calls.push(call)
    for (const r of routes) {
      const a = r(call)
      if (a) return a
    }
    if (url === '/api/v1/document-types' && method === 'GET') return json({ documents: docs })
    const one = /^\/api\/v1\/document-types\/([a-z_]+)$/.exec(url)
    if (one && method === 'GET') {
      const d = details[one[1]] ?? (docs.find((r) => r.code === one[1]) ? detailOf(docs.find((r) => r.code === one[1]) as DocRow) : undefined)
      return d ? json(d) : json({ error_code: 'NOT_FOUND', message: 'no such document' }, 404)
    }
    throw new Error(`unexpected ${method} ${url}`)
  }))
  return calls
}

const settle = async () => {
  await act(async () => {})
  await act(async () => {})
}

async function open(onChanged?: () => void) {
  render(<DocumentMasterPanel onChanged={onChanged} />)
  await settle()
}

const table = () => screen.getByRole('table', { name: 'Documents' })
const rowOf = (name: string) =>
  within(table()).getAllByRole('row').find((r) => within(r).queryAllByRole('cell')[0]?.textContent?.startsWith(name)) as HTMLElement
const names = () => within(table()).getAllByRole('row').slice(1).map((r) => within(r).getAllByRole('cell')[0].textContent)

describe('the list', () => {
  test('shows each document with its origin, formats, structure and status, and the actions its kind allows', async () => {
    stubServer([gstr1, salesReturn, debitNotes, bankExtra])
    await open()
    expect(screen.getByText('4 documents')).toBeInTheDocument()
    const builtin = rowOf('Gstr1')
    expect(within(builtin).getByText('Built-in')).toBeInTheDocument()
    expect(within(builtin).getByText('JSON, XLSX, CSV')).toBeInTheDocument()
    expect(within(builtin).getByRole('button', { name: 'View Gstr1' })).toBeInTheDocument()
    expect(within(builtin).getByRole('button', { name: 'Deactivate Gstr1' })).toBeInTheDocument()
    expect(within(builtin).queryByRole('button', { name: 'Edit Gstr1' })).toBeNull()
    expect(within(builtin).queryByRole('button', { name: 'Delete Gstr1' })).toBeNull()

    const office = rowOf('Sales return register')
    expect(within(office).getByText('Active')).toBeInTheDocument()
    expect(within(office).getByRole('button', { name: 'Edit Sales return register' })).toBeInTheDocument()
    expect(within(office).getByRole('button', { name: 'Delete Sales return register' })).toBeInTheDocument()

    const draft = rowOf('Debit notes')
    expect(within(draft).getByText('Draft')).toBeInTheDocument()
    expect(within(draft).getByText('No structure yet')).toBeInTheDocument()
    expect(within(draft).queryByRole('button', { name: /^(Activate|Deactivate) Debit notes$/ })).toBeNull()

    const used = rowOf('Bank extra lines')
    expect(within(used).getByText('Inactive')).toBeInTheDocument()
    expect(within(used).getByRole('button', { name: 'Activate Bank extra lines' })).toBeInTheDocument()
    expect(within(used).queryByRole('button', { name: 'Delete Bank extra lines' })).toBeNull()
    expect(within(used).getByText(/Used by 1 layout, 0 rules/)).toBeInTheDocument()
  })

  test('search, the four filters and the sort narrow and order it', async () => {
    stubServer([gstr1, salesReturn, debitNotes, bankExtra])
    await open()
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search documents' }), { target: { value: 'return' } })
    expect(names()).toEqual(['Sales return register'])
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search documents' }), { target: { value: '' } })

    fireEvent.change(screen.getByRole('combobox', { name: 'Origin' }), { target: { value: 'builtin' } })
    expect(names()).toEqual(['Gstr1Built-in'])
    fireEvent.change(screen.getByRole('combobox', { name: 'Origin' }), { target: { value: '' } })
    fireEvent.change(screen.getByRole('combobox', { name: 'Category' }), { target: { value: 'Bank' } })
    expect(names()).toEqual(['Bank extra lines'])
    fireEvent.change(screen.getByRole('combobox', { name: 'Category' }), { target: { value: '' } })
    fireEvent.change(screen.getByRole('combobox', { name: 'File format' }), { target: { value: 'json' } })
    expect(names()).toEqual(['Gstr1Built-in', 'Bank extra lines'])
    fireEvent.change(screen.getByRole('combobox', { name: 'File format' }), { target: { value: '' } })
    fireEvent.change(screen.getByRole('combobox', { name: 'Status' }), { target: { value: 'draft' } })
    expect(names()).toEqual(['Debit notes'])
    expect(screen.getByText('1 document of 4')).toBeInTheDocument()

    fireEvent.change(screen.getByRole('searchbox', { name: 'Search documents' }), { target: { value: 'zzz' } })
    expect(screen.getByText('No document matches these filters.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }))
    expect(names()).toHaveLength(4)

    const sortName = screen.getByRole('button', { name: 'Sort by name' })
    fireEvent.click(sortName)
    expect(sortName.closest('th')).toHaveAttribute('aria-sort', 'ascending')
    expect(names()[0]).toBe('Bank extra lines')
    fireEvent.click(sortName)
    expect(sortName.closest('th')).toHaveAttribute('aria-sort', 'descending')
    expect(names()[0]).toBe('Sales return register')
  })

  test('says so when the list cannot be read', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json({ error_code: 'FORBIDDEN', message: 'your role does not allow this' }, 403)))
    await open()
    expect(screen.getByRole('alert')).toHaveTextContent('your role does not allow this (FORBIDDEN)')
  })
})

describe('View', () => {
  test('shows the columns, what reads the document and its history, read only', async () => {
    stubServer([salesReturn], {
      sales_return: detailOf(salesReturn, {
        structure: { name: 'sales_return_row', description: 'Rows', copied_from: 'gstr1_b2b_invoice', locked: true, columns: salesColumns },
        usage: { formats: ['sr-xlsx'], rules: ['RECON-OFF-001'], active_rules: [], records: 12 },
      }),
    })
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'View Sales return register' }))
    await settle()
    const dialog = screen.getByRole('dialog', { name: 'Sales return register' })
    const cols = within(dialog).getByRole('table', { name: 'Columns' })
    expect(within(cols).getByText('net_amount')).toBeInTheDocument()
    expect(within(cols).getAllByRole('row')).toHaveLength(4)
    expect(within(dialog).getByText(/copied from/)).toBeInTheDocument()
    expect(within(dialog).getByText('sr-xlsx')).toBeInTheDocument()
    expect(within(dialog).getByText('12 stored rows')).toBeInTheDocument()
    expect(within(dialog).getByText('document.add')).toBeInTheDocument()
    expect(within(dialog).queryByRole('textbox')).toBeNull()
    fireEvent.click(within(dialog).getByRole('button', { name: 'Close' }))
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})

describe('Deactivate, Activate and Delete', () => {
  test('a document nothing reads is switched off at once; Activate switches it back on', async () => {
    const calls = stubServer([salesReturn, bankExtra], {}, (c) => {
      if (c.method === 'POST' && c.url.endsWith('/status')) return json(detailOf(salesReturn))
    })
    await open(vi.fn())
    fireEvent.click(screen.getByRole('button', { name: 'Deactivate Sales return register' }))
    await settle()
    expect(screen.queryByRole('dialog')).toBeNull()
    const post = calls.find((c) => c.method === 'POST')
    expect(post).toMatchObject({ url: '/api/v1/document-types/sales_return/status', body: { active: false } })
    expect(screen.getByRole('status')).toHaveTextContent('Sales return register is switched off.')
    fireEvent.click(screen.getByRole('button', { name: 'Activate Bank extra lines' }))
    await settle()
    expect(calls.filter((c) => c.method === 'POST')[1]).toMatchObject({ url: '/api/v1/document-types/bank_extra/status', body: { active: true } })
  })

  test('a document that layouts, stored rows or rules read warns first, lists them, and then proceeds on confirmation', async () => {
    const detail = detailOf(salesReturn, { usage: { formats: ['sr-xlsx', 'sr-csv'], rules: ['RECON-OFF-001', 'RECON-OFF-002'], active_rules: ['RECON-OFF-001'], records: 40 } })
    const calls = stubServer([{ ...salesReturn, referenced_by: { formats: 2, rules: 2 } }], { sales_return: detail }, (c) => {
      if (c.method === 'POST' && c.url.endsWith('/status')) return json(detail)
    })
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Deactivate Sales return register' }))
    await settle()
    const dialog = screen.getByRole('dialog', { name: 'Deactivate Sales return register' })
    expect(within(dialog).getByRole('alert')).toHaveTextContent(/no longer be offered for new layouts or new rules/)
    expect(within(dialog).getByText(/keeps working; nothing is changed or switched off/)).toBeInTheDocument()
    expect(within(dialog).getByText(/2 layouts/)).toBeInTheDocument()
    expect(within(dialog).getByText('sr-xlsx, sr-csv')).toBeInTheDocument()
    expect(within(dialog).getByText('40 stored rows')).toBeInTheDocument()
    expect(within(dialog).getByText(/1 active rule/)).toBeInTheDocument()
    expect(calls.some((c) => c.method === 'POST')).toBe(false) // nothing happens until it is confirmed
    fireEvent.click(within(dialog).getByRole('button', { name: 'Deactivate' }))
    await settle()
    expect(calls.find((c) => c.method === 'POST')).toMatchObject({ body: { active: false } })
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  test('Cancel on the warning changes nothing', async () => {
    const detail = detailOf(salesReturn, { usage: { formats: ['sr-xlsx'], rules: [], active_rules: [], records: 0 } })
    const calls = stubServer([{ ...salesReturn, referenced_by: { formats: 1, rules: 0 } }], { sales_return: detail })
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Deactivate Sales return register' }))
    await settle()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(calls.some((c) => c.method === 'POST')).toBe(false)
  })

  test('Delete asks first, then deletes an unused office document', async () => {
    const calls = stubServer([salesReturn], {}, (c) => (c.method === 'DELETE' ? json({ deleted: true }) : undefined))
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Delete Sales return register' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete Sales return register' })
    expect(calls.some((c) => c.method === 'DELETE')).toBe(false)
    fireEvent.click(within(dialog).getByRole('button', { name: 'Delete document' }))
    await settle()
    expect(calls.find((c) => c.method === 'DELETE')?.url).toBe('/api/v1/document-types/sales_return')
    expect(screen.getByRole('status')).toHaveTextContent('Sales return register is deleted.')
  })

  test('a delete the server refuses is shown in its own words', async () => {
    stubServer([salesReturn], {}, (c) =>
      c.method === 'DELETE' ? json({ error_code: 'IN_USE', message: '"sales_return" is read by 12 stored rows; deactivate it instead' }, 409) : undefined)
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Delete Sales return register' }))
    fireEvent.click(screen.getByRole('button', { name: 'Delete document' }))
    await settle()
    expect(screen.getByRole('alert')).toHaveTextContent('deactivate it instead (IN_USE)')
  })
})

// The Add and Edit popup.
// The popup sits over the list, which has a Category filter of its own: look inside the open dialog.
const fieldByLabel = (label: string) => within(screen.getByRole('dialog')).getByLabelText(label) as HTMLInputElement

describe('Add document', () => {
  function addServer(extra: ((c: Call) => Response | undefined)[] = []) {
    return stubServer([gstr1, salesReturn], {}, ...extra, (c) => {
      if (c.url.startsWith('/api/v1/document-types/validate')) return json({ valid: true, problems: [], record_type: null })
      if (c.method === 'POST' && c.url === '/api/v1/document-types') return json({ saved: true, problems: [], document: detailOf(row('purchase_return', 'Purchase return')) }, 201)
    })
  }

  async function startAdd() {
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Add document' }))
    await settle()
  }

  function fillAbout() {
    fireEvent.change(fieldByLabel('Document name'), { target: { value: 'Purchase return' } })
    fireEvent.change(fieldByLabel('Unique document code'), { target: { value: 'purchase_return' } })
    fireEvent.change(fieldByLabel('Category'), { target: { value: 'Books' } })
    fireEvent.click(screen.getByRole('checkbox', { name: 'Excel (XLSX)' }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'CSV' }))
    fireEvent.change(fieldByLabel('Description'), { target: { value: 'Debit notes raised on suppliers' } })
  }

  test('three steps: about the document, the record structure, then the checks and the save', async () => {
    const calls = addServer()
    await startAdd()
    const steps = () => within(screen.getByRole('list', { name: 'Steps' })).getAllByRole('listitem').map((li) => `${li.textContent}${li.getAttribute('aria-current') ? '*' : ''}`)
    expect(steps()).toEqual(['1About the document*', '2Record structure', '3Check and save'])
    fillAbout()
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await settle()
    expect(steps()[1]).toContain('*')

    // define new columns
    expect(screen.getByRole('radio', { name: 'Define new columns' })).toBeChecked()
    fireEvent.change(fieldByLabel('Record type name'), { target: { value: 'purchase_return_row' } })
    fireEvent.change(fieldByLabel('Record type description'), { target: { value: 'Rows of purchase returns' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Column 1 name' }), { target: { value: 'ref_no' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Column 1 label' }), { target: { value: 'Reference' } })
    fireEvent.click(screen.getByRole('checkbox', { name: 'Column 1 is part of the key' }))
    fireEvent.click(screen.getByRole('button', { name: 'Add column' }))
    fireEvent.change(screen.getByRole('textbox', { name: 'Column 2 name' }), { target: { value: 'net_amount' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Column 2 label' }), { target: { value: 'Net amount' } })
    fireEvent.change(screen.getByRole('combobox', { name: 'Column 2 type' }), { target: { value: 'amount' } })
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await settle()

    // step 3 lists the checks and the note that nothing else is created
    const checks = within(screen.getByRole('list', { name: 'Checks' })).getAllByRole('listitem')
    expect(checks).toHaveLength(7)
    expect(checks.every((li) => li.className === 'good')).toBe(true)
    expect(within(screen.getByRole('dialog')).getByText(/creates no layout, list detection, column mapping or rule/)).toBeInTheDocument()
    expect(screen.getByText(/Ready\. This document will be offered in the layout wizard/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Save document' }))
    await settle()
    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/v1/document-types')
    expect(post?.body).toEqual({
      code: 'purchase_return', name: 'Purchase return', category: 'Books', description: 'Debit notes raised on suppliers',
      applicable_for: ['xlsx', 'csv'], status: 'active',
      record_type: {
        name: 'purchase_return_row', description: 'Rows of purchase returns',
        columns: [col('ref_no', 'Reference', 'text', true), col('net_amount', 'Net amount', 'amount')],
      },
    })
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(screen.getByRole('status')).toHaveTextContent('Purchase return is saved and offered in the layout wizard.')
  })

  test('Enter moves to the next step and Back returns, from the keyboard', async () => {
    addServer()
    await startAdd()
    fillAbout()
    fireEvent.submit(screen.getByRole('form', { name: 'Add document' }))
    await settle()
    expect(screen.getByRole('radio', { name: 'Define new columns' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Back' }))
    expect(fieldByLabel('Document name')).toHaveValue('Purchase return')
  })

  test('a problem in the document\'s own fields shows beside the field and keeps the user on step 1', async () => {
    addServer([(c) => c.url.startsWith('/api/v1/document-types/validate')
      ? json({ valid: false, record_type: null, problems: [
        { field: 'code', code: 'BAD_DOCUMENT_CODE', message: 'the code "purchase_return" is already used by a built-in document, a prefix or another document' },
        { field: 'description', code: 'BAD_DESCRIPTION', message: 'the description must be 10 to 300 characters' }] })
      : undefined])
    await startAdd()
    fillAbout()
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await settle()
    expect(screen.queryByRole('radio', { name: 'Define new columns' })).toBeNull()
    const code = fieldByLabel('Unique document code')
    expect(code).toHaveAttribute('aria-invalid', 'true')
    expect(code.closest('.form-group-premium')).toHaveTextContent('is already used by a built-in document')
    expect(fieldByLabel('Description').closest('.form-group-premium')).toHaveTextContent('must be 10 to 300 characters')
  })

  test('with the structure left for later, the check says so and the document saves as a Draft', async () => {
    const calls = addServer([(c) => c.method === 'POST' && c.url === '/api/v1/document-types'
      ? json({ saved: true, problems: [], document: detailOf(row('purchase_return', 'Purchase return', { status: 'draft', record_type: '' })) }, 201)
      : undefined])
    await startAdd()
    fillAbout()
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await settle()
    fireEvent.click(screen.getByRole('radio', { name: 'Add the structure later (saves a Draft)' }))
    expect(screen.queryByRole('table', { name: 'Columns' })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await settle()
    expect(screen.getAllByText('Not added yet')).toHaveLength(2)
    expect(screen.getByText(/This saves a Draft/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Save as Draft' }))
    await settle()
    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/v1/document-types')
    expect(post?.body).not.toHaveProperty('record_type')
    expect(screen.getByRole('status')).toHaveTextContent(/saved as a Draft.*not offered anywhere/)
  })

  test('a structure that is not valid yet is marked on step 3 and saves a Draft', async () => {
    addServer([(c) => c.url.startsWith('/api/v1/document-types/validate')
      ? json({ valid: false, record_type: null, problems: [{ field: 'columns', code: 'BAD_KEY', message: 'mark at least one column as part of the key' }] })
      : undefined])
    await startAdd()
    fillAbout()
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await settle()
    fireEvent.change(screen.getByRole('textbox', { name: 'Column 1 name' }), { target: { value: 'ref_no' } })
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await settle()
    const columns = within(screen.getByRole('list', { name: 'Checks' })).getByText('Columns').closest('li') as HTMLElement
    expect(columns).toHaveClass('bad')
    expect(columns).toHaveTextContent('mark at least one column as part of the key')
    expect(screen.getByRole('button', { name: 'Save as Draft' })).toBeEnabled()
  })

  test('copying the columns of an existing document asks the engine for them and fills the table', async () => {
    const calls = addServer([(c) => {
      const record = (c.body as { record_type?: { copy_from?: string } } | undefined)?.record_type
      return c.url.startsWith('/api/v1/document-types/validate') && record?.copy_from === 'sales_return_row'
        ? json({ valid: true, problems: [], record_type: { name: 'purchase_return_row', description: '', copied_from: 'sales_return_row', columns: salesColumns } })
        : undefined
    }])
    await startAdd()
    fillAbout()
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await settle()
    fireEvent.click(screen.getByRole('radio', { name: 'Copy the columns of an existing document' }))
    expect(screen.getByText(/changing it never changes the document it came from/)).toBeInTheDocument()
    fireEvent.change(fieldByLabel('Copy the columns of'), { target: { value: 'sales_return_row' } })
    await settle()
    expect(screen.getByRole('textbox', { name: 'Column 3 name' })).toHaveValue('net_amount')
    expect(screen.getByRole('textbox', { name: 'Column 1 label' })).toHaveValue('Reference')
    expect(fieldByLabel('Record type name')).toHaveValue('purchase_return_row')
    // the sources are the documents that have a structure
    const sources = within(fieldByLabel('Copy the columns of')).getAllByRole('option').map((o) => o.textContent)
    expect(sources).toEqual(['Choose…', 'Gstr1 (gstr1_b2b_invoice)', 'Sales return register (sales_return_row)'])
    // the columns are the copy's own: editing one changes only this form
    fireEvent.change(screen.getByRole('textbox', { name: 'Column 1 label' }), { target: { value: 'Return number' } })
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await settle()
    fireEvent.click(screen.getByRole('button', { name: 'Save document' }))
    await settle()
    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/v1/document-types')
    expect(post).toBeDefined()
    const record = ((post as Call).body as { record_type: { copy_from: string; columns: DocColumn[] } }).record_type
    expect(record.copy_from).toBe('sales_return_row')
    expect(record.columns[0].label).toBe('Return number')
  })

  test('a refusal from the save is shown with its problems and returns to the step they belong to', async () => {
    addServer([(c) => c.method === 'POST' && c.url === '/api/v1/document-types'
      ? json({ error_code: 'INVALID_DOCUMENT', message: 'the name must be 3 to 60 characters', problems: [{ field: 'name', code: 'BAD_NAME', message: 'the name must be 3 to 60 characters' }] }, 422)
      : undefined])
    await startAdd()
    fillAbout()
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await settle()
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await settle()
    fireEvent.click(screen.getByRole('button', { name: /^Save/ }))
    await settle()
    expect(screen.getByRole('alert')).toHaveTextContent('INVALID_DOCUMENT')
    expect(fieldByLabel('Document name').closest('.form-group-premium')).toHaveTextContent('the name must be 3 to 60 characters')
  })
})

describe('Edit document', () => {
  const locked = detailOf(salesReturn, {
    structure: { name: 'sales_return_row', description: 'Rows', copied_from: '', locked: true, columns: salesColumns },
    usage: { formats: ['sr-xlsx'], rules: [], active_rules: [], records: 0 },
  })

  async function startEdit(detail: DocDetail, extra: ((c: Call) => Response | undefined)[] = []) {
    const calls = stubServer([salesReturn], { sales_return: detail }, ...extra, (c) => {
      if (c.url.startsWith('/api/v1/document-types/validate')) return json({ valid: true, problems: [], record_type: null })
      if (c.method === 'PUT') return json({ saved: true, problems: [], document: detail })
    })
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Edit Sales return register' }))
    await settle()
    return calls
  }

  test('the code cannot be changed, and a structure something reads is locked but for labels and new optional columns', async () => {
    const calls = await startEdit(locked)
    expect(fieldByLabel('Unique document code')).toHaveAttribute('readonly')
    expect(fieldByLabel('Unique document code')).toHaveValue('sales_return')
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await settle()
    expect(screen.queryByRole('radiogroup')).toBeNull() // the structure exists: no choice of where the columns come from
    expect(screen.getByRole('note')).toHaveTextContent(/cannot change/)
    expect(fieldByLabel('Record type name')).toHaveAttribute('readonly')
    // existing columns: name, type, key and required are fixed, the label is not, none can be removed
    expect(screen.getByRole('textbox', { name: 'Column 1 name' })).toBeDisabled()
    expect(screen.getByRole('combobox', { name: 'Column 1 type' })).toBeDisabled()
    expect(screen.getByRole('checkbox', { name: 'Column 1 is part of the key' })).toBeDisabled()
    expect(screen.getByRole('checkbox', { name: 'Column 1 is required' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Remove column 1' })).toBeDisabled()
    expect(screen.getByRole('textbox', { name: 'Column 1 label' })).toBeEnabled()
    // a new column may be added if it is neither a key nor required
    fireEvent.click(screen.getByRole('button', { name: 'Add column' }))
    expect(screen.getByRole('textbox', { name: 'Column 4 name' })).toBeEnabled()
    expect(screen.getByRole('checkbox', { name: 'Column 4 is part of the key' })).toBeDisabled()
    expect(screen.getByRole('checkbox', { name: 'Column 4 is required' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Remove column 4' })).toBeEnabled()
    fireEvent.change(screen.getByRole('textbox', { name: 'Column 4 name' }), { target: { value: 'memo' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Column 4 label' }), { target: { value: 'Memo' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Column 1 label' }), { target: { value: 'Reference number' } })
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await settle()
    fireEvent.click(screen.getByRole('button', { name: 'Save document' }))
    await settle()
    const put = calls.find((c) => c.method === 'PUT')
    expect(put?.url).toBe('/api/v1/document-types/sales_return')
    expect(put).toBeDefined()
    const record = ((put as Call).body as { record_type: { columns: DocColumn[]; copy_from?: string } }).record_type
    expect(record.columns.map((c) => c.name)).toEqual(['ref_no', 'doc_date', 'net_amount', 'memo'])
    expect(record.columns[0].label).toBe('Reference number')
    expect(record).not.toHaveProperty('copy_from')
    // the edit asked the engine to check it as an edit
    expect(calls.some((c) => c.url === '/api/v1/document-types/validate?edit=1')).toBe(true)
  })

  test('an unlocked structure can be changed freely', async () => {
    await startEdit(detailOf(salesReturn))
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await settle()
    expect(screen.queryByRole('note')).toBeNull()
    expect(screen.getByRole('textbox', { name: 'Column 1 name' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Remove column 1' })).toBeEnabled()
    fireEvent.click(screen.getByRole('button', { name: 'Move column 2 up' }))
    expect(screen.getByRole('textbox', { name: 'Column 1 name' })).toHaveValue('doc_date')
  })

  test('a Draft with no structure offers the choice of where its columns come from', async () => {
    stubServer([debitNotes], { debit_notes: detailOf(debitNotes) }, (c) =>
      c.url.startsWith('/api/v1/document-types/validate') ? json({ valid: true, problems: [], record_type: null }) : undefined)
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Edit Debit notes' }))
    await settle()
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await settle()
    expect(screen.getByRole('radiogroup', { name: 'Where the columns come from' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'Add the structure later (saves a Draft)' })).toBeChecked()
  })
})

// The tab is for the Partner who holds manage_documents; the server refuses everyone else anyway.
describe('the Admin tab', () => {
  const me: Account = {
    id: 1, login_name: 'priya', display_name: 'Priya Partner', role: 'partner', active: true, must_change_password: false, failed_logins: 0,
    locked: false, locked_until: '', last_login_at: '', created_at: '2026-10-01T09:00:00Z', updated_at: '2026-10-01T09:00:00Z',
  }

  function adminServer() {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      if (url === '/api/v1/admin/users') return json({ users: [me], roles: ['partner'], lockout: { max_failed_logins: 5, lock_minutes: 15 } })
      if (url === '/api/v1/admin/audit') return json({ entries: [], total: 0, chain: { ok: true, message: 'ok' } })
      if (url === '/api/v1/document-types') return json({ documents: [salesReturn] })
      throw new Error('unexpected ' + url)
    }))
  }

  test('is shown with manage_documents and opens the list', async () => {
    adminServer()
    render(<PermissionContext.Provider value={canFrom(['manage_users', 'manage_documents'])}><Admin me={me} /></PermissionContext.Provider>)
    await settle()
    fireEvent.click(screen.getByRole('tab', { name: 'Document Master' }))
    await settle()
    expect(screen.getByRole('table', { name: 'Documents' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add document' })).toBeInTheDocument()
  })

  test('is hidden without manage_documents', async () => {
    adminServer()
    render(<PermissionContext.Provider value={canFrom(['manage_users'])}><Admin me={me} /></PermissionContext.Provider>)
    await settle()
    expect(screen.queryByRole('tab', { name: 'Document Master' })).toBeNull()
    expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['Users', 'Sign-in security', 'Currency', 'Audit log'])
  })
})
