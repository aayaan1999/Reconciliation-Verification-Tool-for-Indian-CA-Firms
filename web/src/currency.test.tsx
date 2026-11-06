import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { Admin } from './screens/Admin.tsx'
import type { Account, CurrencyState } from './types.ts'

// The Admin tab's Currency page (SPEC-021): the rate in force, the form that sets a new one, the
// change from the rate in force, the history and the explanations. The server is stubbed; every
// name and figure is invented.

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })

const me: Account = {
  id: 1, login_name: 'priya', display_name: 'Priya Partner', role: 'partner', active: true, must_change_password: false, failed_logins: 0,
  locked: false, locked_until: '', last_login_at: '', created_at: '2026-10-01T09:00:00Z', updated_at: '2026-10-01T09:00:00Z',
}

const empty: CurrencyState = { set: false, usd_rate: '', effective_from: '', note: '', problem: 'dollar rate not set', detail: 'usd-rate.yaml is missing', history: [] }
const inForce: CurrencyState = {
  set: true, usd_rate: '83.2500', effective_from: '2026-11-04', note: 'morning card rate', problem: '', detail: '',
  history: [{ at: '2026-11-04T08:30:00Z', by: 'priya', usd_rate: '83.2500', previous: '', effective_from: '2026-11-04', note: 'morning card rate' }],
}

function stub(initial: CurrencyState, onPut?: (body: Record<string, string>) => Response) {
  const calls: { url: string; method: string; body: string }[] = []
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    calls.push({ url, method, body: String(init?.body ?? '') })
    if (url === '/api/v1/admin/users') return json({ users: [me], roles: ['partner'], lockout: { max_failed_logins: 5, lock_minutes: 15 } })
    if (url === '/api/v1/admin/audit') return json({ entries: [], total: 0, chain: { ok: true, message: 'ok' } })
    if (url === '/api/v1/currency' && method === 'GET') return json(initial)
    if (url === '/api/v1/currency' && method === 'PUT') return onPut ? onPut(JSON.parse(String(init?.body))) : json(initial)
    throw new Error('unexpected ' + method + ' ' + url)
  }))
  return calls
}

async function settle() {
  await act(async () => {})
  await act(async () => {})
}

async function openCurrency() {
  render(<Admin me={me} />)
  await settle()
  fireEvent.click(screen.getByRole('tab', { name: 'Currency' }))
  await settle()
}

const rateBox = () => screen.getByLabelText(/Rupees for one US dollar/)
const dateBox = () => screen.getByLabelText(/Effective from/)
const save = () => screen.getByRole('button', { name: 'Save rate' })

describe('the Currency tab', () => {
  test('with no rate it says so, what that means, and offers to set the first rate', async () => {
    stub(empty)
    await openCurrency()
    expect(screen.getByRole('tab', { name: 'Currency' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByText('No rate set.')).toBeInTheDocument()
    expect(screen.getByText(/the checks that need it say Cannot check, and nothing is guessed/)).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Set the rate' })).toBeInTheDocument()
    expect(screen.getByText('No rate has been set on this page yet.')).toBeInTheDocument()
    expect(save()).toBeDisabled()
  })

  test('the rate in force, the next run, and the history are shown', async () => {
    stub(inForce)
    await openCurrency()
    expect(screen.getByText('₹83.2500')).toBeInTheDocument()
    expect(screen.getByText(/A change applies to the next run/)).toBeInTheDocument()
    const rows = within(screen.getByRole('table', { name: 'Rate history' })).getAllByRole('row').slice(1)
    expect(rows).toHaveLength(1)
    expect(rows[0]).toHaveTextContent('priya')
    expect(rows[0]).toHaveTextContent('83.2500')
    expect(rows[0]).toHaveTextContent('No rate before')
    expect(rows[0]).toHaveTextContent('2026-11-04 08:30 UTC')
  })

  test('a slip is explained beside the field and cannot be saved', async () => {
    stub(inForce)
    await openCurrency()
    for (const bad of ['abc', '8325', '0.8325', '83.12345', '$83', '-83']) {
      fireEvent.change(rateBox(), { target: { value: bad } })
      expect(save(), bad).toBeDisabled()
      expect(rateBox(), bad).toHaveAttribute('aria-invalid', 'true')
    }
    fireEvent.change(rateBox(), { target: { value: '8325' } })
    expect(screen.getByText(/at least 1 and below 1,000 rupees for one dollar/)).toBeInTheDocument()
    fireEvent.change(rateBox(), { target: { value: '84.5' } })
    expect(rateBox()).toHaveAttribute('aria-invalid', 'false')
    expect(save()).toBeEnabled()
    fireEvent.change(dateBox(), { target: { value: '' } })
    expect(save()).toBeDisabled()
  })

  test('the change from the rate in force is shown with integers, up and down', async () => {
    stub(inForce)
    await openCurrency()
    fireEvent.change(rateBox(), { target: { value: '84.1' } })
    expect(screen.getByText('+1.02%')).toBeInTheDocument() // (84.1000 - 83.2500) / 83.2500 = 1.0210%
    fireEvent.change(rateBox(), { target: { value: '80' } })
    expect(screen.getByText('−3.90%')).toBeInTheDocument()
    fireEvent.change(rateBox(), { target: { value: '83.2500' } })
    expect(screen.queryByText(/Change from the rate in force/)).not.toBeInTheDocument()
  })

  test('saving sends the rate, the date and the note, refreshes the history and says what happens next', async () => {
    const calls = stub(empty, (body) => json({
      set: true, usd_rate: '84.0000', effective_from: body.effective_from, note: body.note, problem: '', detail: '',
      history: [{ at: '2026-11-05T09:00:00Z', by: 'priya', usd_rate: '84.0000', previous: '', effective_from: body.effective_from, note: body.note }],
    }))
    await openCurrency()
    fireEvent.change(rateBox(), { target: { value: '84' } })
    fireEvent.change(dateBox(), { target: { value: '2026-11-05' } })
    fireEvent.change(screen.getByLabelText(/Note/), { target: { value: '  bank card rate  ' } })
    await act(async () => {
      fireEvent.click(save())
    })
    await settle()
    const put = calls.find((c) => c.method === 'PUT')
    expect(put).toBeDefined()
    expect(JSON.parse(put!.body)).toEqual({ usd_rate: '84', effective_from: '2026-11-05', note: 'bank card rate' })
    expect(screen.getByRole('status')).toHaveTextContent('The rate is ₹84.0000 for each US dollar, effective 2026-11-05. It applies from the next run.')
    expect(within(screen.getByRole('table', { name: 'Rate history' })).getAllByRole('row')).toHaveLength(2)
    expect(screen.getByRole('heading', { name: 'Change the rate' })).toBeInTheDocument()
    // The audit log is read again, since the server wrote an entry.
    expect(calls.filter((c) => c.url === '/api/v1/admin/audit').length).toBeGreaterThan(1)
  })

  test('a refusal by the server is shown and nothing is claimed saved', async () => {
    stub(inForce, () => json({ error_code: 'BAD_RATE', message: 'usd_rate must be at least 1 and below 1,000 rupees per dollar; check the figure' }, 422))
    await openCurrency()
    fireEvent.change(rateBox(), { target: { value: '90' } })
    await act(async () => {
      fireEvent.click(save())
    })
    await settle()
    expect(screen.getByRole('alert')).toHaveTextContent('(BAD_RATE)')
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  test('the tab is a real tab and the page asks only the local server', async () => {
    const calls = stub(inForce)
    await openCurrency()
    for (const c of calls) expect(c.url.startsWith('/'), c.url).toBe(true)
    expect(screen.getByRole('tab', { name: 'Currency' })).toHaveAttribute('aria-controls')
    expect(document.querySelector('img[src^="http"], link[href^="http"], script[src^="http"]')).toBeNull()
  })
})
