import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import App from './App.tsx'
import { sampleRules } from './sample/index.ts'
import type { Session } from './types.ts'

// SPEC-012 in the browser: the sign-in screen, the change of a starting password, the Admin tab
// shown to the Partner only, buttons hidden from roles that may not use them, and the masked
// password fields of the Admin tab. The server is stubbed; the names are illustrative.

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })
}

const partnerPermissions = ['activate_rules', 'decide', 'delete_findings', 'disable_rules', 'draft_rules', 'export', 'manage_users', 'run', 'view']
const staffPermissions = ['decide', 'draft_rules', 'run', 'view']
const readOnlyPermissions = ['view']

function session(login: string, role: string, permissions: string[], mustChange = false): Session {
  return {
    user: {
      id: 1, login_name: login, display_name: login === 'priya' ? 'A. Partner' : login, role, active: true,
      must_change_password: mustChange, failed_logins: 0, locked: false, locked_until: '', last_login_at: '',
      created_at: '2026-10-30T09:00:00Z', updated_at: '2026-10-30T09:00:00Z',
    },
    permissions,
    must_change_password: mustChange,
  }
}

type Route = (url: string, init?: RequestInit) => Response | undefined

// stubServer answers the API calls the test names and refuses any other call, so a test fails
// on a request it did not expect.
function stubServer(...routes: Route[]) {
  const calls: { url: string; method: string; body: string }[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, method: init?.method ?? 'GET', body: String(init?.body ?? '') })
      for (const r of routes) {
        const answer = r(url, init)
        if (answer) return answer
      }
      throw new Error('unexpected ' + url)
    }),
  )
  return calls
}

const rules = (url: string) => (url === '/api/v1/rules' ? json(sampleRules) : undefined)
const noClients = (url: string) => (url.startsWith('/api/v1/clients?') ? json({ tax_year: '2026-27', clients: [] }) : undefined)
const comparisons = (url: string) => (url === '/api/v1/comparisons' ? json({ comparisons: [] }) : undefined)

async function settle() {
  await act(async () => {})
  await act(async () => {})
}

describe('the sign-in screen (SPEC-012 AC-3)', () => {
  test('with no session, the sign-in screen shows and no tool screen does', async () => {
    stubServer((url) => (url === '/api/v1/me' ? json({ error_code: 'NOT_SIGNED_IN', message: 'Sign in to continue.' }, 401) : undefined),
      (url) => (url === '/api/v1/setup' ? json({ needed: false }) : undefined))
    render(<App />)
    await settle()
    expect(screen.getByRole('form', { name: 'Sign in' })).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Screens' })).not.toBeInTheDocument()
    expect(screen.queryByText('Season board')).not.toBeInTheDocument()
  })

  test('a wrong password shows the server message, and the password box is cleared', async () => {
    stubServer(
      (url) => (url === '/api/v1/me' ? json({ error_code: 'NOT_SIGNED_IN', message: '' }, 401) : undefined),
      (url) => (url === '/api/v1/setup' ? json({ needed: false }) : undefined),
      (url, init) => {
        if (url !== '/api/v1/login') return undefined
        expect(JSON.parse(String(init?.body))).toEqual({ login_name: 'nobody', password: 'wrong-password-1' })
        return json({ error_code: 'BAD_CREDENTIALS', message: 'The login name or password is not right.' }, 401)
      },
    )
    render(<App />)
    await settle()
    const form = screen.getByRole('form', { name: 'Sign in' })
    fireEvent.change(within(form).getByLabelText('Login name'), { target: { value: 'nobody' } })
    fireEvent.change(within(form).getByLabelText('Password'), { target: { value: 'wrong-password-1' } })
    await act(async () => {
      fireEvent.click(within(form).getByRole('button', { name: 'Sign in' }))
    })
    expect(within(form).getByRole('alert')).toHaveTextContent('The login name or password is not right.')
    expect(within(form).getByLabelText('Password')).toHaveValue('')
  })

  test('the sign-in form has no link or script from outside the program (AC-13)', async () => {
    stubServer((url) => (url === '/api/v1/me' ? json({ error_code: 'NOT_SIGNED_IN', message: '' }, 401) : undefined),
      (url) => (url === '/api/v1/setup' ? json({ needed: false }) : undefined))
    render(<App />)
    await settle()
    const html = document.documentElement.outerHTML
    expect(html).not.toMatch(/<link[^>]+href="https?:/)
    expect(html).not.toMatch(/<script[^>]+src="https?:/)
    expect(html).not.toMatch(/src="https?:/)
  })
})

describe('the first Partner (SPEC-012 AC-9)', () => {
  test('on a first start the setup form is offered and sends the Partner\'s own password', async () => {
    const calls = stubServer(
      (url) => (url === '/api/v1/me' ? json({ error_code: 'NOT_SIGNED_IN', message: '' }, 401) : undefined),
      (url) => (url === '/api/v1/setup' ? json({ needed: true }) : undefined),
      (url, init) => (url === '/api/v1/setup' && init?.method === 'POST' ? json({ login_name: 'priya', display_name: 'A. Partner', role: 'partner' }, 201) : undefined),
    )
    render(<App />)
    await settle()
    const form = screen.getByRole('form', { name: 'Set up the first Partner' })
    fireEvent.change(within(form).getByLabelText('Login name'), { target: { value: 'priya' } })
    fireEvent.change(within(form).getByLabelText('Display name'), { target: { value: 'A. Partner' } })
    fireEvent.change(within(form).getByLabelText('Password'), { target: { value: 'partner-own-2026' } })
    fireEvent.change(within(form).getByLabelText('Type the password again'), { target: { value: 'partner-own-2026' } })
    await act(async () => {
      fireEvent.click(within(form).getByRole('button', { name: 'Create Partner account' }))
    })
    const sent = calls.find((c) => c.url === '/api/v1/setup' && c.method === 'POST')
    expect(JSON.parse(sent?.body ?? '{}')).toEqual({ login_name: 'priya', display_name: 'A. Partner', password: 'partner-own-2026' })
    expect(await screen.findByText('The first Partner is set up. Sign in with the password you chose.')).toBeInTheDocument()
  })
})

describe('the starting password (SPEC-012 AC-2)', () => {
  test('a user with a starting password sees only the change of password', async () => {
    stubServer((url) => (url === '/api/v1/me' ? json(session('asha', 'staff', staffPermissions, true)) : undefined))
    render(<App />)
    await settle()
    expect(screen.getByRole('heading', { name: 'Choose your own password' })).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Screens' })).not.toBeInTheDocument()
    expect(screen.queryByText('Season board')).not.toBeInTheDocument()
  })

  test('after the change, the tool opens and the new password is sent in full', async () => {
    const calls = stubServer(
      (url) => (url === '/api/v1/me' ? json(session('asha', 'staff', staffPermissions, true)) : undefined),
      rules,
      noClients,
      comparisons,
      (url, init) => {
        if (url !== '/api/v1/me/password') return undefined
        expect(JSON.parse(String(init?.body))).toEqual({ current_password: 'starting-2026-pass', new_password: 'chosen-2026-pass' })
        return json(session('asha', 'staff', staffPermissions, false))
      },
    )
    render(<App />)
    await settle()
    const form = screen.getByRole('form', { name: 'Choose your own password' })
    fireEvent.change(within(form).getByLabelText('Current (starting) password'), { target: { value: 'starting-2026-pass' } })
    fireEvent.change(within(form).getByLabelText('New password'), { target: { value: 'chosen-2026-pass' } })
    fireEvent.change(within(form).getByLabelText('New password again'), { target: { value: 'chosen-2026-pass' } })
    await act(async () => {
      fireEvent.click(within(form).getByRole('button', { name: 'Change password' }))
    })
    expect(calls.some((c) => c.url === '/api/v1/me/password')).toBe(true)
    expect(await screen.findByRole('navigation', { name: 'Screens' })).toBeInTheDocument()
  })
})

describe('roles on the screens (SPEC-012 AC-7)', () => {
  test('the Partner sees the Admin tab', async () => {
    stubServer((url) => (url === '/api/v1/me' ? json(session('priya', 'partner', partnerPermissions)) : undefined), rules, noClients, comparisons)
    render(<App />)
    await settle()
    expect(screen.getByRole('button', { name: /Admin/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'A. Partner, partner: menu' })).toBeInTheDocument()
  })

  test('a Staff member does not see the Admin tab and cannot add a client', async () => {
    stubServer((url) => (url === '/api/v1/me' ? json(session('sunil', 'staff', staffPermissions)) : undefined), rules, noClients, comparisons)
    render(<App />)
    await settle()
    expect(screen.queryByRole('button', { name: /Admin/ })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '+ Add client' })).toBeInTheDocument()
  })

  test('a Read-only user sees no add button and no way to change a rule', async () => {
    stubServer((url) => (url === '/api/v1/me' ? json(session('tara', 'read_only', readOnlyPermissions)) : undefined), rules, noClients, comparisons)
    render(<App />)
    await settle()
    expect(screen.queryByRole('button', { name: /Admin/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '+ Add client' })).not.toBeInTheDocument()
  })

  test('the Admin tab is not shown when the server is not running (sample data)', async () => {
    stubServer()
    render(<App />)
    await settle()
    expect(screen.queryByRole('button', { name: /Admin/ })).not.toBeInTheDocument()
    expect(screen.getByText('Sample data')).toBeInTheDocument()
  })
})

describe('the Admin tab (SPEC-012 §4)', () => {
  const users = [
    { id: 1, login_name: 'priya', display_name: 'A. Partner', role: 'partner', active: true, must_change_password: false, failed_logins: 0, locked: false, locked_until: '', last_login_at: '2026-10-30T09:00:00Z', created_at: '', updated_at: '' },
    { id: 2, login_name: 'sunil', display_name: 'Sunil', role: 'staff', active: true, must_change_password: true, failed_logins: 0, locked: false, locked_until: '', last_login_at: '', created_at: '', updated_at: '' },
  ]
  const adminAnswers = (url: string) => {
    if (url === '/api/v1/admin/users') return json({ users, roles: ['partner', 'manager', 'staff', 'read_only'], lockout: { max_failed_logins: 5, lock_minutes: 15 } })
    if (url === '/api/v1/admin/audit') return json({ entries: [{ seq: 1, at: '2026-10-30T09:00:00Z', actor: 'priya', action: 'user.add', object_type: 'user', object_id: 'sunil', details: { role: 'staff' } }], total: 1, chain: { ok: true, message: 'every entry links to the one before it' } })
    return undefined
  }

  test('lists the users, and adding a user sends a masked starting password the Partner typed', async () => {
    const calls = stubServer(
      (url) => (url === '/api/v1/me' ? json(session('priya', 'partner', partnerPermissions)) : undefined),
      rules, noClients, comparisons, adminAnswers,
      (url, init) => {
        if (url !== '/api/v1/admin/users' || init?.method !== 'POST') return undefined
        return json({ ...users[1], id: 3, login_name: 'meera', display_name: 'Meera', role: 'manager' }, 201)
      },
    )
    render(<App />)
    await settle()
    fireEvent.click(screen.getByRole('button', { name: /Admin/ }))
    await settle()
    const table = screen.getByRole('table', { name: 'Users' })
    expect(within(table).getByText('sunil')).toBeInTheDocument()
    expect(within(table).getByText('Must change password')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '+ Add a user' }))
    const add = screen.getByRole('form', { name: 'Add user' })
    const pw = within(add).getByLabelText('Starting password (at least 10 characters)')
    expect(pw).toHaveAttribute('type', 'password')
    fireEvent.click(within(add).getByRole('button', { name: 'Show' }))
    expect(within(add).getByLabelText('Starting password (at least 10 characters)')).toHaveAttribute('type', 'text')
    fireEvent.change(within(add).getByLabelText('Login name'), { target: { value: 'meera' } })
    fireEvent.change(within(add).getByLabelText('Display name'), { target: { value: 'Meera' } })
    fireEvent.click(within(add).getByRole('radio', { name: 'Manager' }))
    fireEvent.change(within(add).getByLabelText('Starting password (at least 10 characters)'), { target: { value: 'starting-meera-2026' } })
    await act(async () => {
      fireEvent.click(within(add).getByRole('button', { name: 'Add user' }))
    })
    const sent = calls.find((c) => c.url === '/api/v1/admin/users' && c.method === 'POST')
    expect(JSON.parse(sent?.body ?? '{}')).toEqual({ login_name: 'meera', display_name: 'Meera', role: 'manager', password: 'starting-meera-2026' })
    expect(await screen.findByText('meera is added.')).toBeInTheDocument()
  })

  test('the audit log says whether the chain is intact and names the actor', async () => {
    stubServer(
      (url) => (url === '/api/v1/me' ? json(session('priya', 'partner', partnerPermissions)) : undefined),
      rules, noClients, comparisons, adminAnswers,
    )
    render(<App />)
    await settle()
    fireEvent.click(screen.getByRole('button', { name: /Admin/ }))
    await settle()
    fireEvent.click(screen.getByRole('tab', { name: 'Audit log' }))
    expect(screen.getByText(/The audit chain is intact/)).toBeInTheDocument()
    const log = screen.getByRole('table', { name: 'Audit log' })
    expect(within(log).getByText('priya')).toBeInTheDocument()
    expect(within(log).getByText('user.add')).toBeInTheDocument()
  })
})
