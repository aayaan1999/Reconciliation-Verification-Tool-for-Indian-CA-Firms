import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import App from './App.tsx'
import { sampleRules } from './sample/index.ts'
import type { Session } from './types.ts'

// Recon Screen Document v2 §4.1 (first Partner), §4.2 (sign-in), §3 and §12.1 (the shortcuts
// sheet and the Ctrl K search). The server is stubbed; names and figures are illustrative.

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
  test('no Worklist on the rail, key 2 does nothing, and the search does not offer it', () => {
    render(<App />)
    const nav = screen.getByRole('navigation', { name: 'Screens' })
    expect(within(nav).queryByRole('button', { name: /Worklist/ })).not.toBeInTheDocument()
    fireEvent.keyDown(document.body, { key: '2' })
    expect(screen.queryByRole('table', { name: 'Worklist items' })).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Season board')
    fireEvent.keyDown(document.body, { key: 'k', ctrlKey: true })
    fireEvent.change(within(screen.getByRole('dialog')).getByRole('combobox'), { target: { value: 'Worklist' } })
    expect(screen.queryByRole('option', { name: /Go to Worklist/ })).not.toBeInTheDocument()
  })
})

describe('shortcuts sheet (§12.1)', () => {
  test('? opens the sheet grouped into Anywhere, Lists and tables, and Client file', () => {
    render(<App />)
    fireEvent.keyDown(document.body, { key: '?' })
    const sheet = screen.getByRole('dialog', { name: 'Keyboard shortcuts' })
    const groups = within(sheet).getAllByRole('heading', { level: 3 }).map((h) => h.textContent)
    expect(groups).toEqual(['Anywhere', 'Lists and tables', 'Client file'])
    const anywhere = within(sheet).getByRole('region', { name: 'Anywhere' })
    for (const k of ['Ctrl', 'K', '1', '3', '4', '[', '?', 'Esc']) expect(within(anywhere).getByText(k, { selector: 'kbd' })).toBeInTheDocument()
    // The Worklist is hidden for now (features.ts), so its key 2 is not listed.
    expect(within(anywhere).queryByText('2', { selector: 'kbd' })).not.toBeInTheDocument()
    const lists = within(sheet).getByRole('region', { name: 'Lists and tables' })
    for (const k of ['/', 'n', 'Enter']) expect(within(lists).getAllByText(k, { selector: 'kbd' }).length).toBeGreaterThan(0)
    const file = within(sheet).getByRole('region', { name: 'Client file' })
    for (const k of ['u', 'e']) expect(within(file).getByText(k, { selector: 'kbd' })).toBeInTheDocument()
    fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})

describe('Ctrl K search (§3, §12.1)', () => {
  function ctrlK(target: Element = document.activeElement ?? document.body) {
    fireEvent.keyDown(target, { key: 'k', ctrlKey: true })
  }

  test('opens even while typing in a field, and finds a client by PAN ending', () => {
    render(<App />)
    const search = screen.getByRole('searchbox', { name: 'Search clients' })
    search.focus()
    ctrlK(search)
    const dialog = screen.getByRole('dialog', { name: 'Search or jump' })
    const input = within(dialog).getByRole('combobox', { name: 'Search clients, rules and actions' })
    expect(document.activeElement).toBe(input)
    fireEvent.change(input, { target: { value: '234F' } })
    expect(within(dialog).getByRole('option', { name: /Illustrative Traders Private Limited/ })).toHaveAttribute('aria-selected', 'true')
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Illustrative Traders Private Limited')
  })

  test('finds a client by name and a rule by ID; Enter on the rule opens Rules', () => {
    render(<App />)
    ctrlK()
    const input = screen.getByRole('combobox', { name: 'Search clients, rules and actions' })
    fireEvent.change(input, { target: { value: 'sample exports' } })
    expect(screen.getByRole('option', { name: /Sample Exports LLP/ })).toBeInTheDocument()
    const rule = sampleRules.rules[0]
    fireEvent.change(input, { target: { value: rule.id.toLowerCase() } })
    const group = screen.getByRole('group', { name: 'Rules' })
    expect(within(group).getAllByRole('option')[0]).toHaveTextContent(rule.id)
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Rules')
  })

  test('arrow keys choose an action, Enter runs it; Tab keeps focus inside; Esc closes only the search', () => {
    render(<App />)
    fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Enter' })
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Illustrative Traders Private Limited')
    ctrlK()
    const input = screen.getByRole('combobox', { name: 'Search clients, rules and actions' })
    fireEvent.keyDown(input, { key: 'Tab' })
    expect(document.activeElement).toBe(input)
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Illustrative Traders Private Limited')

    ctrlK()
    const again = screen.getByRole('combobox', { name: 'Search clients, rules and actions' })
    fireEvent.change(again, { target: { value: 'theme' } })
    const dialog = screen.getByRole('dialog', { name: 'Search or jump' })
    const options = within(dialog).getAllByRole('option').map((o) => o.textContent)
    expect(options).toEqual(['Theme: Light', 'Theme: Dark', 'Theme: Match the computerIn use'])
    fireEvent.keyDown(again, { key: 'ArrowDown' })
    fireEvent.keyDown(again, { key: 'Enter' })
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
  })

  test('the rail button opens it; it offers the shortcuts sheet', () => {
    render(<App />)
    fireEvent.click(screen.getByRole('button', { name: 'Search or jump' }))
    const input = screen.getByRole('combobox', { name: 'Search clients, rules and actions' })
    fireEvent.change(input, { target: { value: 'shortcuts' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(screen.getByRole('dialog', { name: 'Keyboard shortcuts' })).toBeInTheDocument()
  })

  const rules = (url: string) => (url === '/api/v1/rules' ? json(sampleRules) : undefined)
  const noClients = (url: string) => (url.startsWith('/api/v1/clients?') ? json({ tax_year: '2026-27', clients: [] }) : undefined)

  test('offers Admin and Sign out to the Partner', async () => {
    stubServer((url) => (url === '/api/v1/me' ? json(session('partner', ['manage_users', 'view'])) : undefined), rules, noClients)
    render(<App />)
    await settle()
    ctrlK()
    const actions = within(screen.getByRole('group', { name: 'Actions' })).getAllByRole('option').map((o) => o.textContent)
    expect(actions).toContain('Go to Admin4')
    expect(actions).toContain('Sign out')
  })

  test('does not offer Admin to a user who may not manage users', async () => {
    stubServer((url) => (url === '/api/v1/me' ? json(session('staff', ['view'])) : undefined), rules, noClients)
    render(<App />)
    await settle()
    ctrlK()
    const actions = within(screen.getByRole('group', { name: 'Actions' })).getAllByRole('option').map((o) => o.textContent)
    expect(actions).not.toContain('Go to Admin4')
    expect(actions).toContain('Go to Rules3')
  })

  test('does nothing in front of the sign-in', async () => {
    stubServer(signedOut, setupNeeded(false))
    render(<App />)
    await settle()
    ctrlK()
    expect(screen.queryByRole('dialog', { name: 'Search or jump' })).toBeNull()
  })
})
