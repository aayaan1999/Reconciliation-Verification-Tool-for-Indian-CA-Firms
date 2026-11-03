import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import type { FormatsList } from './formatsTypes.ts'
import { Formats } from './screens/Formats.tsx'

// The Formats screen (SPEC-016): formats and parameters listed with where each is used, standard
// ones read only, an office format switched off and back with a reason, a parameter added, and
// the gaps. The server is stubbed; every name is invented.

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const list = (over: Partial<FormatsList> = {}): FormatsList => ({
  formats: [
    {
      last_used: '2026-11-02T09:00:00Z', needed: 2, needed_read: 2, samples: [], id: 'generic.gstr1', name: 'GSTR-1 (portal JSON)', file_type: 'json', document: 'gstr1', source: 'builtin', version: 'v1', revision: 1,
      status: 'active', disabled_reason: '', published_by: '', reads_facts: ['gstr1.total.taxable'], reads_records: ['gst_invoice'],
      versions: [{ revision: 1, version: 'v1', status: 'active', by: 'recon-engine', at: '2026-11-03T09:00:00Z', note: 'Profile version 1', published: true }],
    },
    {
      last_used: '', needed: 3, needed_read: 1, samples: [], id: 'office.sales', name: 'Office sales export', file_type: 'xlsx', document: 'sales_register', source: 'office', version: 'v2', revision: 2,
      status: 'testing', disabled_reason: '', published_by: '', reads_facts: [], reads_records: ['gst_invoice'],
      versions: [
        { revision: 1, version: 'v1', status: 'active', by: 'A. Partner', at: '2026-11-01T10:00:00Z', note: 'Taught from sales.xlsx', published: true },
        { revision: 2, version: 'v2', status: 'testing', by: 'A. Partner', at: '2026-11-02T10:00:00Z', note: 'Taught from sales2.xlsx', published: true },
      ],
    },
  ],
  parameters: [
    {
      key: 'gstr1.total.taxable', name: 'GSTR-1 total taxable value', kind: 'money', document: 'gstr1', description: 'The total.', aliases: [], source: 'builtin',
      status: 'active', disabled_reason: '', deprecated: false, version: 'v1', used_by_formats: ['generic.gstr1'],
      used_by_rules: [{ id: 'RECON-GST-001', title: 'Sales equal GSTR-1', status: 'active' }],
    },
    {
      key: 'office.advance_received', name: 'Advance received', kind: 'money', document: 'ledger', description: '', aliases: ['Advance'], source: 'office',
      status: 'active', disabled_reason: '', deprecated: false, version: 'v1', used_by_formats: [], used_by_rules: [],
    },
  ],
  unsupplied: ['office.advance_received'],
  uncovered: [{ id: 'RECON-LED-001', title: 'New heads', status: 'testing', missing: ['office.advance_received'] }],
  unrecognised: [],
  ...over,
})

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })
}

// stub answers the list and records every other call.
function stub(data: FormatsList = list(), reply: (url: string) => Response | null = () => null, get: (url: string) => unknown = () => null) {
  const calls: { method: string; url: string; body: unknown }[] = []
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (method === 'GET') return json(get(String(url)) ?? data)
    calls.push({ method, url, body: init?.body ? JSON.parse(String(init.body)) : null })
    return reply(url) ?? json({ ok: true })
  }))
  return calls
}

describe('Formats screen', () => {
  test('lists the formats with their state, and filters them', async () => {
    stub()
    render(<Formats />)
    const table = await screen.findByRole('table', { name: 'Formats' })
    expect(within(table).getByText('GSTR-1 (portal JSON)')).toBeInTheDocument()
    expect(within(table).getByText('Office sales export')).toBeInTheDocument()
    expect(screen.getByText('2 of 2 formats')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Origin'), { target: { value: 'office' } })
    expect(screen.getByText('1 of 2 formats')).toBeInTheDocument()
    expect(within(table).queryByText('GSTR-1 (portal JSON)')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }))
    expect(screen.getByText('2 of 2 formats')).toBeInTheDocument()
  })

  test('a standard format is read only', async () => {
    stub()
    render(<Formats />)
    fireEvent.click(await screen.findByRole('button', { name: 'Manage GSTR-1 (portal JSON)' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(/standard format cannot be changed/i)).toBeInTheDocument()
    expect(within(dialog).queryByRole('button', { name: 'Disable' })).not.toBeInTheDocument()
    expect(within(dialog).getByText('GSTR-1 total taxable value')).toBeInTheDocument()
  })

  test('an office format is switched off with a reason, and switched back to an earlier version', async () => {
    const calls = stub()
    render(<Formats />)
    fireEvent.click(await screen.findByRole('button', { name: 'Manage Office sales export' }))
    let dialog = await screen.findByRole('dialog')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Switch back to v1' }))
    dialog = await screen.findByRole('dialog', { name: 'Switch back to v1' })
    fireEvent.change(within(dialog).getByLabelText(/Reason/), { target: { value: 'v2 read the wrong sheet' } })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Switch back' }))
    await waitFor(() => expect(calls).toHaveLength(1))
    expect(calls[0]).toEqual({ method: 'POST', url: '/api/v1/formats/office.sales/versions/1/switch', body: { reason: 'v2 read the wrong sheet' } })

    fireEvent.click(await screen.findByRole('button', { name: 'Manage Office sales export' }))
    dialog = await screen.findByRole('dialog')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Disable' }))
    dialog = await screen.findByRole('dialog', { name: 'Disable Office sales export' })
    fireEvent.change(within(dialog).getByLabelText(/Reason/), { target: { value: 'replaced' } })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Disable' }))
    await waitFor(() => expect(calls).toHaveLength(2))
    expect(calls[1].url).toBe('/api/v1/formats/office.sales/disable')
  })

  test('a refusal is shown in the popup, and the popup stays open', async () => {
    stub(list(), () => json({ error_code: 'ALREADY_DISABLED', message: 'It is already switched off.' }, 409))
    render(<Formats />)
    fireEvent.click(await screen.findByRole('button', { name: 'Manage Office sales export' }))
    fireEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Disable' }))
    const dialog = await screen.findByRole('dialog', { name: 'Disable Office sales export' })
    fireEvent.change(within(dialog).getByLabelText(/Reason/), { target: { value: 'x' } })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Disable' }))
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('It is already switched off. (ALREADY_DISABLED)')
  })

  test('parameters: search, an office one has aliases, and a new one is added', async () => {
    const calls = stub()
    render(<Formats />)
    fireEvent.click(await screen.findByRole('tab', { name: 'Parameters' }))
    const table = await screen.findByRole('table', { name: 'Parameters' })
    expect(within(table).getByText('office.advance_received')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Search parameters'), { target: { value: 'advance' } })
    expect(screen.getByText('1 of 2 parameters')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Open Advance received' }))
    const detail = await screen.findByRole('dialog')
    expect(within(detail).getByRole('list', { name: 'Aliases' })).toHaveTextContent('Advance')
    fireEvent.click(within(detail).getByRole('button', { name: 'Close' }))

    fireEvent.click(screen.getByRole('button', { name: '+ Add a parameter' }))
    const form = await screen.findByRole('form', { name: 'Add parameter' })
    fireEvent.change(within(form).getByLabelText(/^Name/), { target: { value: 'Customer deposit' } })
    fireEvent.change(within(form).getByLabelText(/^Key/), { target: { value: 'office.customer_deposit' } })
    fireEvent.change(within(form).getByLabelText(/aliases/), { target: { value: 'Deposit, Advance deposit' } })
    fireEvent.click(within(form).getByRole('button', { name: 'Add parameter' }))
    await waitFor(() => expect(calls).toHaveLength(1))
    expect(calls[0].url).toBe('/api/v1/parameters')
    expect(calls[0].body).toMatchObject({ key: 'office.customer_deposit', kind: 'money', document: 'gstr1', aliases: ['Deposit', 'Advance deposit'] })
  })

  test('the gaps tab names the rules that cannot run', async () => {
    stub()
    render(<Formats />)
    fireEvent.click(await screen.findByRole('tab', { name: /Gaps/ }))
    const rules = await screen.findByRole('table', { name: 'Rules that cannot run' })
    expect(within(rules).getByText('RECON-LED-001')).toBeInTheDocument()
    expect(within(rules).getByText('Advance received')).toBeInTheDocument()
  })

  test('a user who may not change anything sees no actions', async () => {
    stub()
    render(<Formats canChange={false} />)
    await screen.findByRole('table', { name: 'Formats' })
    expect(screen.queryByRole('button', { name: '+ Add a format' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Manage Office sales export' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).queryByRole('button', { name: 'Disable' })).not.toBeInTheDocument()
    expect(within(dialog).queryByRole('button', { name: /Switch back/ })).not.toBeInTheDocument()
  })

  test('with no server it says so', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new Error('down') }))
    render(<Formats />)
    expect(await screen.findByRole('alert')).toHaveTextContent('NO_SERVER')
  })
})

const withDraft = (): FormatsList => {
  const l = list()
  l.formats[1].versions.push({ revision: 3, version: 'v3', status: 'draft', by: 'A. Partner', at: '2026-11-03T10:00:00Z', note: 'Edited from sales3.xlsx', published: false })
  return l
}

describe('Editing and publishing a format (SPEC-016)', () => {
  test('a draft is marked, published as testing or active, and the dry run is shown', async () => {
    const calls = stub(withDraft(), () => json({
      id: 'office.sales', version: 'v3', status: 'active',
      dry_run: {
        can_run: [{ id: 'RECON-GST-001', title: 'Sales equal GSTR-1' }],
        cannot_check: [{ id: 'RECON-GST-002', title: 'Invoice tax', missing: ['sreg.row.igst'] }],
      },
    }))
    render(<Formats />)
    fireEvent.click(await screen.findByRole('button', { name: 'Manage Office sales export' }))
    const detail = await screen.findByRole('dialog')
    const versions = within(detail).getByRole('table', { name: 'Versions' })
    expect(within(versions).getByText('Draft')).toBeInTheDocument()
    // A draft cannot be switched back to; a published version can.
    expect(within(versions).queryByRole('button', { name: 'Switch back to v3' })).not.toBeInTheDocument()
    expect(within(versions).getByRole('button', { name: 'Switch back to v1' })).toBeInTheDocument()

    fireEvent.click(within(versions).getByRole('button', { name: 'Publish v3' }))
    const dialog = await screen.findByRole('dialog', { name: 'Publish v3 of Office sales export' })
    expect(within(dialog).getByRole('radio', { name: /Testing/ })).toBeChecked()
    fireEvent.click(within(dialog).getByRole('radio', { name: /Active/ }))
    fireEvent.click(within(dialog).getByRole('button', { name: 'Publish' }))
    await waitFor(() => expect(calls).toHaveLength(1))
    expect(calls[0]).toEqual({ method: 'POST', url: '/api/v1/formats/office.sales/publish', body: { revision: 3, status: 'active' } })
    const done = await screen.findByRole('dialog', { name: 'Office sales export v3 is published' })
    expect(within(done).getByText('Published as active. Every run reads it.')).toBeInTheDocument()
    const dry = within(done).getByRole('region', { name: 'Which rules can run' })
    expect(within(dry).getByText('1 can run')).toBeInTheDocument()
    expect(within(dry).getByText('1 cannot check')).toBeInTheDocument()
    expect(within(dry).getByText(/Needs sreg\.row\.igst/)).toBeInTheDocument()
  })

  test('a refused publish says why and keeps the dialog open', async () => {
    stub(withDraft(), () => json({ error_code: 'PROFILE_NOT_LOADED', message: 'The published profile is not read by the engine.' }, 422))
    render(<Formats />)
    fireEvent.click(await screen.findByRole('button', { name: 'Manage Office sales export' }))
    fireEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Publish v3' }))
    const dialog = await screen.findByRole('dialog', { name: 'Publish v3 of Office sales export' })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Publish' }))
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('PROFILE_NOT_LOADED')
  })

  test('editing asks for a sample file, then the mapping screen opens on the stored mapping', async () => {
    stub(list(), () => null, (url) => (url.endsWith('/mapping') ? {
      last_used: '', needed: 3, needed_read: 1, samples: [], id: 'office.sales', name: 'Office sales export', editable: true, reason: '',
      in_use: { revision: 2, version: 'v2', mapping: { name: 'Office sales export', document_type: 'sales_register', rows: '$.rows[*]', where: [], unit: 'rupees', columns: [], period: { path: '', format: '', const: 'FY' }, tax_year: '', gstin_path: '', identify: [] } },
      draft: null,
    } : null))
    render(<Formats />)
    fireEvent.click(await screen.findByRole('button', { name: 'Manage Office sales export' }))
    fireEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Edit the mapping' }))
    const ask = await screen.findByRole('dialog', { name: 'Edit Office sales export' })
    expect(within(ask).getByText(/not kept/)).toBeInTheDocument()
    expect(within(ask).getByLabelText(/Sample file/)).toBeInTheDocument()
  })

  test('a format whose mapping cannot be edited says so', async () => {
    stub(list(), () => null, (url) => (url.endsWith('/mapping') ? {
      last_used: '', needed: 3, needed_read: 1, samples: [], id: 'office.sales', name: 'Office sales export', editable: false, reason: 'This format was not taught on the mapping screen, so its mapping cannot be edited.', in_use: null, draft: null,
    } : null))
    render(<Formats />)
    fireEvent.click(await screen.findByRole('button', { name: 'Manage Office sales export' }))
    fireEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Edit the mapping' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('not taught on the mapping screen')
  })

  test('a standard format offers neither edit nor publish', async () => {
    stub(withDraft())
    render(<Formats />)
    fireEvent.click(await screen.findByRole('button', { name: 'Manage GSTR-1 (portal JSON)' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).queryByRole('button', { name: 'Edit the mapping' })).not.toBeInTheDocument()
    expect(within(dialog).queryByRole('button', { name: /^Publish/ })).not.toBeInTheDocument()
  })
})
