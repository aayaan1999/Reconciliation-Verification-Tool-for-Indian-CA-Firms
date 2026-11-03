import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import App from './App.tsx'
import { sampleRules } from './sample/index.ts'
import type { Session } from './types.ts'

// Recon Screen Document v2 §4.1 (first Partner), §4.2 (sign-in) and §3 (the rail by role). The
// server is stubbed; names and figures are illustrative.

afterEach(() => {
  vi.unstubAllGlobals()
  document.documentElement.removeAttribute('data-theme')
  localStorage.clear()
})

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })
}

type Route = (url: string, init?: RequestInit) => Response | undefined

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

async function settle() {
  await act(async () => {})
  await act(async () => {})
}

const signedOut = (url: string) => (url === '/api/v1/me' ? json({ error_code: 'NOT_SIGNED_IN', message: '' }, 401) : undefined)
const setupNeeded = (needed: boolean) => (url: string, init?: RequestInit) =>
  url === '/api/v1/setup' && (init?.method ?? 'GET') === 'GET' ? json({ needed }) : undefined

function session(role: string, permissions: string[], mustChange = false): Session {
  return {
    user: {
      id: 1, login_name: 'asha', display_name: 'A. Staff', role, active: true, must_change_password: mustChange,
      failed_logins: 0, locked: false, locked_until: '', last_login_at: '', created_at: '', updated_at: '',
    },
    permissions,
    must_change_password: mustChange,
  }
}

describe('first Partner setup (§4.1)', () => {
  async function openSetup() {
    const calls = stubServer(signedOut, setupNeeded(true),
      (url, init) => (url === '/api/v1/setup' && init?.method === 'POST' ? json({ login_name: 'partner' }, 201) : undefined))
    render(<App />)
    await settle()
    return { calls, form: screen.getByRole('form', { name: 'Set up the first Partner' }) }
  }

  test('login name and display name, a password with a strength meter, typed twice', async () => {
    const { form } = await openSetup()
    expect(within(form).getByText('First start · shown once')).toBeInTheDocument()
    expect(within(form).getByLabelText('Login name')).toBeInTheDocument()
    expect(within(form).getByLabelText('Display name')).toBeInTheDocument()
    const meter = within(form).getByRole('meter', { name: 'Password strength' })
    fireEvent.change(within(form).getByLabelText('Password'), { target: { value: 'short' } })
    expect(meter).toHaveAttribute('aria-valuetext', 'Too short: at least 10 characters')
    fireEvent.change(within(form).getByLabelText('Password'), { target: { value: 'Partner-Own-2026-Desk' } })
    expect(meter).toHaveAttribute('aria-valuetext', 'Very strong')
    expect(within(form).getByLabelText('Type the password again')).toHaveAttribute('type', 'password')
  })

  test('a password under 10 characters is refused before anything is sent', async () => {
    const { calls, form } = await openSetup()
    fireEvent.change(within(form).getByLabelText('Login name'), { target: { value: 'partner' } })
    fireEvent.change(within(form).getByLabelText('Display name'), { target: { value: 'A. Partner' } })
    fireEvent.change(within(form).getByLabelText('Password'), { target: { value: 'nine-char' } })
    fireEvent.change(within(form).getByLabelText('Type the password again'), { target: { value: 'nine-char' } })
    await act(async () => {
      fireEvent.click(within(form).getByRole('button', { name: 'Create Partner account' }))
    })
    expect(within(form).getByRole('alert')).toHaveTextContent('The password needs at least 10 characters.')
    expect(calls.some((c) => c.method === 'POST')).toBe(false)
  })

  test('two different passwords are refused before anything is sent', async () => {
    const { calls, form } = await openSetup()
    fireEvent.change(within(form).getByLabelText('Login name'), { target: { value: 'partner' } })
    fireEvent.change(within(form).getByLabelText('Display name'), { target: { value: 'A. Partner' } })
    fireEvent.change(within(form).getByLabelText('Password'), { target: { value: 'partner-own-2026' } })
    fireEvent.change(within(form).getByLabelText('Type the password again'), { target: { value: 'partner-own-2027' } })
    await act(async () => {
      fireEvent.click(within(form).getByRole('button', { name: 'Create Partner account' }))
    })
    expect(within(form).getByRole('alert')).toHaveTextContent('The two passwords are not the same.')
    expect(calls.some((c) => c.method === 'POST')).toBe(false)
  })

  test('after saving, a success note leads to the sign-in', async () => {
    const { form } = await openSetup()
    fireEvent.change(within(form).getByLabelText('Login name'), { target: { value: 'partner' } })
    fireEvent.change(within(form).getByLabelText('Display name'), { target: { value: 'A. Partner' } })
    fireEvent.change(within(form).getByLabelText('Password'), { target: { value: 'partner-own-2026' } })
    fireEvent.change(within(form).getByLabelText('Type the password again'), { target: { value: 'partner-own-2026' } })
    await act(async () => {
      fireEvent.click(within(form).getByRole('button', { name: 'Create Partner account' }))
    })
    const signIn = screen.getByRole('form', { name: 'Sign in' })
    expect(within(signIn).getByRole('status')).toHaveTextContent('The first Partner is set up. Sign in with the password you chose.')
  })
})

describe('sign-in (§4.2)', () => {
  test('the left panel shows an example finding as A − B = difference and the four promises', async () => {
    stubServer(signedOut, setupNeeded(false))
    render(<App />)
    await settle()
    const about = screen.getByRole('complementary', { name: 'About the tool' })
    const example = within(about).getByRole('figure', { name: 'Example finding' })
    expect(example).toHaveTextContent('Computation₹1,38,310.00−Fixed-asset schedule₹1,35,862.00=Difference₹2,448.00')
    for (const promise of ['Works fully offline', 'Rule-driven checks', 'GST, income and deduction', 'Every decision kept']) {
      expect(within(about).getByText(promise)).toBeInTheDocument()
    }
    expect(screen.getByRole('form', { name: 'Sign in' })).toBeInTheDocument()
  })

  test('a locked account shows the server message as written', async () => {
    const locked = 'This account is locked after too many failed sign-ins. A Partner can unlock it on the Admin tab.'
    stubServer(signedOut, setupNeeded(false),
      (url) => (url === '/api/v1/login' ? json({ error_code: 'ACCOUNT_LOCKED', message: locked }, 401) : undefined))
    render(<App />)
    await settle()
    const form = screen.getByRole('form', { name: 'Sign in' })
    fireEvent.change(within(form).getByLabelText('Login name'), { target: { value: 'asha' } })
    fireEvent.change(within(form).getByLabelText('Password'), { target: { value: 'any-password-1' } })
    await act(async () => {
      fireEvent.click(within(form).getByRole('button', { name: 'Sign in' }))
    })
    expect(within(form).getByRole('alert')).toHaveTextContent(locked)
  })

  test('a Caps Lock warning shows while Caps Lock is on, and the password can be shown', async () => {
    stubServer(signedOut, setupNeeded(false))
    render(<App />)
    await settle()
    const form = screen.getByRole('form', { name: 'Sign in' })
    const password = within(form).getByLabelText('Password')
    fireEvent.keyDown(password, { key: 'A', modifierCapsLock: true })
    expect(within(form).getByText('Caps Lock is on.')).toBeInTheDocument()
    fireEvent.keyUp(password, { key: 'CapsLock', modifierCapsLock: false })
    expect(within(form).queryByText('Caps Lock is on.')).not.toBeInTheDocument()
    fireEvent.click(within(form).getByRole('button', { name: 'Show password' }))
    expect(password).toHaveAttribute('type', 'text')
  })
})

describe('change of a starting password', () => {
  test('a new password under 10 characters is refused before anything is sent', async () => {
    const calls = stubServer((url) => (url === '/api/v1/me' ? json(session('staff', ['view'], true)) : undefined))
    render(<App />)
    await settle()
    const form = screen.getByRole('form', { name: 'Choose your own password' })
    fireEvent.change(within(form).getByLabelText('Current (starting) password'), { target: { value: 'starting-2026-pass' } })
    fireEvent.change(within(form).getByLabelText('New password'), { target: { value: 'short-one' } })
    fireEvent.change(within(form).getByLabelText('New password again'), { target: { value: 'short-one' } })
    await act(async () => {
      fireEvent.click(within(form).getByRole('button', { name: 'Change password' }))
    })
    expect(within(form).getByRole('alert')).toHaveTextContent('The new password needs at least 10 characters.')
    expect(calls.some((c) => c.url === '/api/v1/me/password')).toBe(false)
  })
})

describe('the Worklist is hidden for now (features.ts)', () => {
  test('no Worklist on the rail, and the Season board stays', () => {
    render(<App />)
    const nav = screen.getByRole('navigation', { name: 'Screens' })
    expect(within(nav).queryByRole('button', { name: /Worklist/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('table', { name: 'Worklist items' })).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Season board')
  })
})

describe('the rail by role', () => {
  const rules = (url: string) => (url === '/api/v1/rules' ? json(sampleRules) : undefined)
  const noClients = (url: string) => (url.startsWith('/api/v1/clients?') ? json({ tax_year: '2026-27', clients: [] }) : undefined)

  test('offers Admin and Sign out to the Partner', async () => {
    stubServer((url) => (url === '/api/v1/me' ? json(session('partner', ['manage_users', 'view'])) : undefined), rules, noClients)
    render(<App />)
    await settle()
    const nav = screen.getByRole('navigation', { name: 'Screens' })
    expect(within(nav).getByRole('button', { name: 'Admin' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /: menu$/ }))
    expect(within(screen.getByRole('menu', { name: 'User menu' })).getByRole('menuitem', { name: 'Sign out' })).toBeInTheDocument()
  })

  test('does not offer Admin to a user who may not manage users', async () => {
    stubServer((url) => (url === '/api/v1/me' ? json(session('staff', ['view'])) : undefined), rules, noClients)
    render(<App />)
    await settle()
    const nav = screen.getByRole('navigation', { name: 'Screens' })
    expect(within(nav).queryByRole('button', { name: 'Admin' })).not.toBeInTheDocument()
    expect(within(nav).getByRole('button', { name: 'Rules' })).toBeInTheDocument()
  })
})
