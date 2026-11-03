import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import type { FormatsList } from './formatsTypes.ts'
import { Formats } from './screens/Formats.tsx'

// SPEC-016: files no format recognised, kept samples, the publish confirmation when a kept sample
// reads differently, and the import and export of a format. The server is stubbed; every name is
// invented.

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const data = (): FormatsList => ({
  formats: [{
    id: 'office.sales', name: 'Office sales export', file_type: 'xlsx', document: 'sales_register', source: 'office', version: 'v2', revision: 2,
    status: 'testing', disabled_reason: '', published_by: '', reads_facts: [], reads_records: ['gst_invoice'],
    versions: [
      { revision: 1, version: 'v1', status: 'active', by: 'A. Partner', at: '2026-11-01T10:00:00Z', note: 'Taught from sales.xlsx', published: true },
      { revision: 2, version: 'v2', status: 'draft', by: 'A. Partner', at: '2026-11-02T10:00:00Z', note: 'Edited from sales2.xlsx', published: false },
    ],
    last_used: '', needed: 0, needed_read: 0,
    samples: [{ id: 7, file_name: 'sales_sample.xlsx', size_bytes: 4096, kept_by: 'A. Partner', kept_at: '2026-11-01T10:00:00Z', has_content: true }],
  }],
  parameters: [],
  unsupplied: [],
  uncovered: [],
  unrecognised: [
    { id: 3, file_name: 'purchase_oct.csv', file_type: 'csv', sha256: 'ab'.repeat(32), size_bytes: 2048, tax_year: '2026-27', noted_at: '2026-11-02T16:40:00Z' },
  ],
})

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })

// stub answers GETs with the list (or what get gives) and records every other call.
function stub(reply: (url: string, body: unknown) => Response | null = () => null, get: (url: string) => Response | null = () => null) {
  const calls: { method: string; url: string; body: unknown }[] = []
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (method === 'GET') return get(String(url)) ?? json(data())
    const body = init?.body ? JSON.parse(String(init.body)) : null
    calls.push({ method, url, body })
    return reply(url, body) ?? json({ ok: true })
  }))
  return calls
}

describe('Files no format recognised', () => {
  test('they are listed with a count, and a file is dismissed with a reason', async () => {
    const calls = stub()
    render(<Formats />)
    const tab = await screen.findByRole('tab', { name: /Unrecognised files/ })
    expect(within(tab).getByLabelText('1 files waiting')).toBeInTheDocument()
    fireEvent.click(tab)
    const table = await screen.findByRole('table', { name: 'Unrecognised files' })
    expect(within(table).getByText('purchase_oct.csv')).toBeInTheDocument()
    expect(within(table).getByText('CSV')).toBeInTheDocument()

    fireEvent.click(within(table).getByRole('button', { name: 'Dismiss purchase_oct.csv' }))
    const dialog = await screen.findByRole('dialog', { name: 'Dismiss purchase_oct.csv' })
    fireEvent.change(within(dialog).getByLabelText(/Reason/), { target: { value: 'a one-off export' } })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Dismiss' }))
    await waitFor(() => expect(calls).toHaveLength(1))
    expect(calls[0]).toEqual({ method: 'POST', url: '/api/v1/unrecognised/3/resolve', body: { status: 'dismissed', note: 'a one-off export' } })
  })

  test('Teach this format asks for the file again, and says the server kept only its hash', async () => {
    stub()
    render(<Formats />)
    fireEvent.click(await screen.findByRole('tab', { name: /Unrecognised files/ }))
    fireEvent.click(await screen.findByRole('button', { name: 'Teach a format for purchase_oct.csv' }))
    const dialog = await screen.findByRole('dialog', { name: 'Teach a format for purchase_oct.csv' })
    expect(within(dialog).getByText(/kept only its name and hash/)).toBeInTheDocument()
    expect(within(dialog).getByLabelText(/The file/)).toHaveAttribute('accept', '.json,.xlsx,.csv')
  })

  test('with none waiting, the tab says so', async () => {
    stub(() => null, () => json({ ...data(), unrecognised: [] }))
    render(<Formats />)
    fireEvent.click(await screen.findByRole('tab', { name: /Unrecognised files/ }))
    expect(await screen.findByText(/No file is waiting for a format/)).toBeInTheDocument()
  })
})

describe('Kept samples, publishing and export', () => {
  test('a kept sample is listed and can be removed', async () => {
    const calls = stub()
    render(<Formats />)
    fireEvent.click(await screen.findByRole('button', { name: 'Manage Office sales export' }))
    const dialog = await screen.findByRole('dialog')
    const kept = within(dialog).getByRole('list', { name: 'Kept samples' })
    expect(within(kept).getByText('sales_sample.xlsx')).toBeInTheDocument()
    fireEvent.click(within(kept).getByRole('button', { name: 'Remove the kept sample sales_sample.xlsx' }))
    await waitFor(() => expect(calls).toHaveLength(1))
    expect(calls[0].url).toBe('/api/v1/format-samples/7/remove')
  })

  test('a publish refused because a kept sample reads differently is confirmed with a reason', async () => {
    let first = true
    const calls = stub((url) => {
      if (!url.endsWith('/publish')) return null
      if (first) {
        first = false
        return json({ error_code: 'SAMPLES_CHANGED', message: '1 kept sample(s) read differently with this version: sales_sample.xlsx' }, 409)
      }
      return json({ id: 'office.sales', version: 'v2', status: 'testing' })
    })
    render(<Formats />)
    fireEvent.click(await screen.findByRole('button', { name: 'Manage Office sales export' }))
    fireEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Publish v2' }))
    const dialog = await screen.findByRole('dialog', { name: 'Publish v2 of Office sales export' })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Publish' }))
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('SAMPLES_CHANGED')
    const go = within(dialog).getByRole('button', { name: 'Publish anyway' })
    expect(go).toBeDisabled()
    fireEvent.change(within(dialog).getByLabelText(/Reason for publishing it anyway/), { target: { value: 'amounts are in paise now' } })
    fireEvent.click(go)
    await waitFor(() => expect(calls).toHaveLength(2))
    expect(calls[0].body).toEqual({ revision: 2, status: 'testing' })
    expect(calls[1].body).toEqual({ revision: 2, status: 'testing', confirm: true, reason: 'amounts are in paise now' })
    expect(await screen.findByRole('dialog', { name: 'Office sales export v2 is published' })).toBeInTheDocument()
  })

  test('Export asks the server for the bundle and saves it', async () => {
    const url = vi.fn(() => 'blob:x')
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: url, revokeObjectURL: vi.fn() }))
    const seen: string[] = []
    stub(() => null, (u) => {
      if (u.endsWith('/export')) {
        seen.push(u)
        return new Response('{}', { status: 200, headers: { 'content-disposition': 'attachment; filename="office.sales.format.json"' } })
      }
      return null
    })
    render(<Formats />)
    fireEvent.click(await screen.findByRole('button', { name: 'Manage Office sales export' }))
    fireEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Export' }))
    expect(await screen.findByText('Office sales export is exported as office.sales.format.json.')).toBeInTheDocument()
    expect(seen).toEqual(['/api/v1/formats/office.sales/export'])
    expect(url).toHaveBeenCalled()
  })
})

describe('Import a format', () => {
  const bundle = { schema: 'recon.format-bundle/1', vocab_version: 10, format: { id: 'office.sales.sales_register' }, parameters: [] }
  const plan = { format: 'office.sales.sales_register', format_name: 'Office sales export', parameters_added: ['office.advance_received'], parameters_unchanged: [], applied: false, profile_folder: 'x' }

  test('a bundle is checked first, then imported on the click', async () => {
    const calls = stub((url, body) => (url === '/api/v1/formats/import'
      ? json({ ...plan, applied: (body as { check_only: boolean }).check_only === false })
      : null))
    render(<Formats />)
    fireEvent.click(await screen.findByRole('button', { name: 'Import a format' }))
    const dialog = await screen.findByRole('dialog', { name: 'Import a format' })
    const file = new File([JSON.stringify(bundle)], 'office.sales.format.json', { type: 'application/json' })
    // jsdom's File has no text(): give it one, as a browser's has.
    Object.defineProperty(file, 'text', { value: async () => JSON.stringify(bundle) })
    await act(async () => {
      fireEvent.change(within(dialog).getByLabelText(/Bundle file/), { target: { files: [file] } })
    })
    const added = await within(dialog).findByRole('region', { name: 'What the bundle adds' })
    expect(within(added).getByText('Office sales export')).toBeInTheDocument()
    expect(within(added).getByText(/1 new parameters: office\.advance_received/)).toBeInTheDocument()
    expect(calls.map((c) => (c.body as { check_only: boolean }).check_only)).toEqual([true])

    await act(async () => {
      fireEvent.click(within(dialog).getByRole('button', { name: 'Import' }))
    })
    await waitFor(() => expect(calls).toHaveLength(2))
    expect((calls[1].body as { check_only: boolean }).check_only).toBe(false)
    expect(await screen.findByText(/Office sales export is imported, in testing/)).toBeInTheDocument()
  })

  test('a bundle the server refuses is explained and cannot be imported', async () => {
    stub(() => json({ error_code: 'BAD_BUNDLE', message: 'It was made for vocabulary version 9; this machine has 10.' }, 422))
    render(<Formats />)
    fireEvent.click(await screen.findByRole('button', { name: 'Import a format' }))
    const dialog = await screen.findByRole('dialog', { name: 'Import a format' })
    const file = new File(['{}'], 'b.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', { value: async () => '{}' })
    await act(async () => {
      fireEvent.change(within(dialog).getByLabelText(/Bundle file/), { target: { files: [file] } })
    })
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('BAD_BUNDLE')
    expect(within(dialog).getByRole('button', { name: 'Import' })).toBeDisabled()
  })
})
