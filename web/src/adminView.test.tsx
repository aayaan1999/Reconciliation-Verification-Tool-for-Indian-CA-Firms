import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { Admin } from './screens/Admin.tsx'
import type { Account, AuditEntry } from './types.ts'

// The Admin screen as Recon Screen Document v2 §11 lays it out: four tabs (Currency was added by SPEC-021), the users with
// their state, the role matrix, Add a user and Manage in popups (never a side drawer), the lockout
// settings and the audit log with its search, filters and pages. The server is stubbed; every
// name is invented.

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })
}

function account(id: number, login: string, name: string, role: string, extra: Partial<Account> = {}): Account {
  return {
    id, login_name: login, display_name: name, role, active: true, must_change_password: false, failed_logins: 0,
    locked: false, locked_until: '', last_login_at: '', created_at: '2026-10-01T09:00:00Z', updated_at: '2026-10-01T09:00:00Z', ...extra,
  }
}

const me = account(1, 'priya', 'Priya Partner', 'partner', { last_login_at: '2026-10-30T09:15:00Z' })
const users = [
  me,
  account(2, 'sunil', 'Sunil Staff', 'staff', { must_change_password: true }),
  account(3, 'meena', 'Meena Manager', 'manager', { locked: true, failed_logins: 5, locked_until: '2026-10-30T10:00:00Z' }),
  account(4, 'ravi', 'Ravi Reader', 'read_only', { active: false }),
]
const roles = ['partner', 'manager', 'staff', 'read_only']

// thirty entries, seq 1 (oldest) to 30 (newest); every third one is by sunil.
const actions = ['login.success', 'user.add', 'login.failure', 'user.role_change', 'user.password_reset']
const entries: AuditEntry[] = Array.from({ length: 30 }, (_, i) => ({
  seq: i + 1,
  at: `2026-10-${String(i + 1).padStart(2, '0')}T08:00:00Z`,
  actor: i % 3 === 2 ? 'sunil' : 'priya',
  action: actions[i % actions.length],
  object_type: 'user',
  object_id: `user${i + 1}`,
  details: {},
})).reverse()

type Route = (url: string, init?: RequestInit) => Response | undefined

function stubServer(opts: { chainOk?: boolean } = {}, ...routes: Route[]) {
  const calls: { url: string; method: string; body: string }[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, method: init?.method ?? 'GET', body: String(init?.body ?? '') })
      for (const r of routes) {
        const answer = r(url, init)
        if (answer) return answer
      }
      if (url === '/api/v1/admin/users' && (init?.method ?? 'GET') === 'GET') {
        return json({ users, roles, lockout: { max_failed_logins: 5, lock_minutes: 15 } })
      }
      if (url === '/api/v1/admin/audit') {
        const ok = opts.chainOk ?? true
        return json({ entries, total: 30, chain: { ok, message: ok ? 'every entry links to the one before it' : 'entry 12 does not link to entry 11' } })
      }
      throw new Error('unexpected ' + url)
    }),
  )
  return calls
}

const posts = (calls: { url: string; method: string; body: string }[]) => calls.filter((c) => c.method !== 'GET')

async function settle() {
  await act(async () => {})
  await act(async () => {})
}

async function open() {
  render(<Admin me={me} />)
  await settle()
}

describe('the Admin tabs (§11)', () => {
  test('four tabs, each chosen with a click', async () => {
    stubServer()
    await open()
    const list = screen.getByRole('tablist', { name: 'Admin sections' })
    expect(within(list).getAllByRole('tab').map((t) => t.textContent)).toEqual(['Users', 'Sign-in security', 'Currency', 'Document Master', 'Audit log'])
    expect(screen.getByRole('tab', { name: 'Users' })).toHaveAttribute('aria-selected', 'true')
    fireEvent.click(screen.getByRole('tab', { name: 'Sign-in security' }))
    expect(screen.getByRole('tab', { name: 'Sign-in security' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('form', { name: 'Lockout settings' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab', { name: 'Audit log' }))
    expect(screen.getByRole('tab', { name: 'Audit log' })).toHaveAttribute('aria-selected', 'true')
    fireEvent.click(screen.getByRole('tab', { name: 'Users' }))
    expect(screen.getByRole('tab', { name: 'Users' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('table', { name: 'Users' })).toBeInTheDocument()
  })
})

describe('Users (§11.1)', () => {
  test('each user shows avatar initials, name and login, role, status and last sign-in', async () => {
    stubServer()
    await open()
    const rows = within(screen.getByRole('table', { name: 'Users' })).getAllByRole('row').slice(1)
    expect(rows).toHaveLength(4)
    expect(within(rows[0]).getByText('PP')).toBeInTheDocument()
    expect(within(rows[0]).getByText('priya')).toBeInTheDocument()
    expect(within(rows[0]).getByText('Partner')).toBeInTheDocument()
    expect(within(rows[0]).getByText('Active')).toHaveClass('tag', 'pass')
    expect(within(rows[0]).getByText('2026-10-30 09:15 UTC')).toBeInTheDocument()
    expect(within(rows[1]).getByText('Must change password')).toBeInTheDocument()
    expect(within(rows[1]).getByText('Never')).toBeInTheDocument()
    expect(within(rows[2]).getByText('Locked')).toHaveClass('tag', 'fail')
    expect(within(rows[2]).getByText('Manager')).toBeInTheDocument()
    expect(within(rows[3]).getByText('Disabled')).toBeInTheDocument()
    expect(within(rows[3]).getByText('Read-only')).toBeInTheDocument()
    expect(within(rows[3]).getByRole('button', { name: 'Manage ravi' })).toBeInTheDocument()
  })

  test('"What each role can do" shows the four roles against twelve actions', async () => {
    stubServer()
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'What each role can do' }))
    const matrix = within(screen.getByRole('dialog', { name: 'What each role can do' })).getByRole('table')
    const head = within(matrix).getAllByRole('columnheader').map((c) => c.textContent)
    expect(head).toEqual(['Action', 'Partner', 'Manager', 'Staff', 'Read-only'])
    const body = within(matrix).getAllByRole('row').slice(1)
    expect(body).toHaveLength(12)
    const cells = (action: string) => within(within(matrix).getByRole('rowheader', { name: action }).closest('tr') as HTMLElement)
      .getAllByRole('cell').map((c) => c.textContent?.trim())
    expect(cells('View clients, findings and the Worklist')).toEqual(['Yes', 'Yes', 'Yes', 'Yes'])
    expect(cells('Decide findings and classify the Worklist')).toEqual(['Yes', 'Yes', 'Yes', 'No'])
    expect(cells('Delete findings')).toEqual(['Yes', 'Yes', 'No', 'No'])
    expect(cells('Admin tab: users, sign-in security, the dollar rate, audit log')).toEqual(['Yes', 'No', 'No', 'No'])
    expect(cells('Disable a client or enable it again')).toEqual(['Yes', 'No', 'No', 'No'])
    fireEvent.click(screen.getByRole('button', { name: 'Close what each role can do' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  test('Add a user: the role picker explains each role, Suggest fills a starting password, and the user is sent', async () => {
    // A fixed run of random bytes, so the suggested password is known.
    vi.spyOn(crypto, 'getRandomValues').mockImplementation(<T extends ArrayBufferView | null>(a: T): T => {
      const bytes = a as unknown as Uint8Array
      bytes.forEach((_, i) => {
        bytes[i] = i
      })
      return a
    })
    const calls = stubServer({}, (url, init) =>
      url === '/api/v1/admin/users' && init?.method === 'POST' ? json(account(5, 'asha', 'Asha New', 'read_only', { must_change_password: true }), 201) : undefined)
    await open()
    fireEvent.click(screen.getByRole('button', { name: '+ Add a user' }))
    const dialog = screen.getByRole('dialog', { name: 'Add a user' })
    const readOnly = within(dialog).getByRole('radio', { name: 'Read-only' })
    expect(readOnly).toHaveAccessibleDescription('Views clients, findings and the Worklist. Cannot change anything.')
    expect(within(dialog).getByRole('radio', { name: 'Staff' })).toBeChecked()
    fireEvent.click(readOnly)
    fireEvent.change(within(dialog).getByLabelText('Display name'), { target: { value: 'Asha New' } })
    fireEvent.change(within(dialog).getByLabelText('Login name'), { target: { value: 'asha' } })
    const pw = within(dialog).getByLabelText('Starting password (at least 10 characters)')
    expect(pw).toHaveAttribute('type', 'password')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Suggest' }))
    expect(pw).toHaveValue('ABCD-EFGH-JKLM-NPQR')
    expect(pw).toHaveAttribute('type', 'text')
    await act(async () => {
      fireEvent.click(within(dialog).getByRole('button', { name: 'Add user' }))
    })
    await settle()
    const sent = posts(calls)
    expect(sent).toHaveLength(1)
    expect(JSON.parse(sent[0].body)).toEqual({ login_name: 'asha', display_name: 'Asha New', role: 'read_only', password: 'ABCD-EFGH-JKLM-NPQR' })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.getByText('asha is added.')).toBeInTheDocument()
  })

  test('a refused user stays in the dialog with the server\'s reason', async () => {
    stubServer({}, (url, init) =>
      url === '/api/v1/admin/users' && init?.method === 'POST' ? json({ error_code: 'LOGIN_TAKEN', message: 'That login name is in use.' }, 409) : undefined)
    await open()
    fireEvent.click(screen.getByRole('button', { name: '+ Add a user' }))
    const dialog = screen.getByRole('dialog', { name: 'Add a user' })
    fireEvent.change(within(dialog).getByLabelText('Display name'), { target: { value: 'Second Sunil' } })
    fireEvent.change(within(dialog).getByLabelText('Login name'), { target: { value: 'sunil' } })
    fireEvent.change(within(dialog).getByLabelText('Starting password (at least 10 characters)'), { target: { value: 'invented-start-2026' } })
    await act(async () => {
      fireEvent.click(within(dialog).getByRole('button', { name: 'Add user' }))
    })
    expect(within(dialog).getByRole('alert')).toHaveTextContent('That login name is in use. (LOGIN_TAKEN)')
  })

  test('the Partner\'s own account offers no role change and no disabling', async () => {
    stubServer()
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Manage priya' }))
    const drawer = screen.getByRole('dialog', { name: 'Manage priya' })
    expect(drawer).not.toHaveClass('drawer') // a popup in the middle of the screen
    expect(within(drawer).getByText(/You cannot change your own role or disable yourself/)).toBeInTheDocument()
    expect(within(drawer).queryByRole('form', { name: 'Change role' })).not.toBeInTheDocument()
    expect(within(drawer).queryByRole('button', { name: 'Disable user' })).not.toBeInTheDocument()
    expect(within(drawer).getByRole('form', { name: 'Reset password' })).toBeInTheDocument()
  })

  test('Manage changes the role with a reason, and disabling asks for confirmation first', async () => {
    const calls = stubServer({}, (url, init) =>
      init?.method === 'POST' && url.startsWith('/api/v1/admin/users/2/') ? json(users[1]) : undefined)
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Manage sunil' }))
    const drawer = screen.getByRole('dialog', { name: 'Manage sunil' })
    const change = within(drawer).getByRole('button', { name: 'Change role' })
    expect(change).toBeDisabled()
    fireEvent.click(within(drawer).getByRole('radio', { name: 'Manager' }))
    fireEvent.change(within(drawer).getByLabelText('Reason for the change'), { target: { value: 'Leads the audit team' } })
    await act(async () => {
      fireEvent.click(change)
    })
    await settle()
    expect(posts(calls).map((c) => [c.url, JSON.parse(c.body)])).toEqual([
      ['/api/v1/admin/users/2/role', { role: 'manager', reason: 'Leads the audit team' }],
    ])
    expect(within(drawer).getByRole('status')).toHaveTextContent('Role of sunil changed to Manager.')

    fireEvent.change(within(drawer).getByLabelText('Reason for disabling'), { target: { value: 'Left the firm' } })
    fireEvent.click(within(drawer).getByRole('button', { name: 'Disable user' }))
    expect(posts(calls)).toHaveLength(1)
    const confirm = within(drawer).getByRole('group', { name: 'Confirm disabling' })
    expect(confirm).toHaveTextContent('Disable sunil?')
    await act(async () => {
      fireEvent.click(within(confirm).getByRole('button', { name: 'Yes, disable sunil' }))
    })
    await settle()
    const last = posts(calls)[1]
    expect(last.url).toBe('/api/v1/admin/users/2/active')
    expect(JSON.parse(last.body)).toEqual({ active: false, reason: 'Left the firm' })
  })

  test('Keep active cancels the disabling without sending anything', async () => {
    const calls = stubServer()
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Manage sunil' }))
    const drawer = screen.getByRole('dialog', { name: 'Manage sunil' })
    fireEvent.change(within(drawer).getByLabelText('Reason for disabling'), { target: { value: 'Left the firm' } })
    fireEvent.click(within(drawer).getByRole('button', { name: 'Disable user' }))
    fireEvent.click(within(drawer).getByRole('button', { name: 'Keep active' }))
    expect(within(drawer).queryByRole('group', { name: 'Confirm disabling' })).not.toBeInTheDocument()
    expect(posts(calls)).toHaveLength(0)
  })

  test('a locked user can be unlocked, and a password reset sends the new starting password', async () => {
    const calls = stubServer({}, (url, init) =>
      init?.method === 'POST' && url.startsWith('/api/v1/admin/users/3/') ? json(users[2]) : undefined)
    await open()
    fireEvent.click(screen.getByRole('button', { name: 'Manage sunil' }))
    expect(within(screen.getByRole('dialog')).queryByRole('button', { name: /Unlock/ })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Close manage sunil' }))
    fireEvent.click(screen.getByRole('button', { name: 'Manage meena' }))
    const drawer = screen.getByRole('dialog', { name: 'Manage meena' })
    expect(within(drawer).getByText('Locked after 5 failed sign-ins, until 2026-10-30 10:00 UTC.')).toBeInTheDocument()
    await act(async () => {
      fireEvent.click(within(drawer).getByRole('button', { name: 'Unlock meena' }))
    })
    await settle()
    fireEvent.change(within(drawer).getByLabelText('New starting password (at least 10 characters)'), { target: { value: 'invented-reset-2026' } })
    await act(async () => {
      fireEvent.click(within(drawer).getByRole('button', { name: 'Reset password' }))
    })
    await settle()
    expect(posts(calls).map((c) => [c.url, c.body])).toEqual([
      ['/api/v1/admin/users/3/unlock', '{}'],
      ['/api/v1/admin/users/3/password', JSON.stringify({ password: 'invented-reset-2026' })],
    ])
  })
})

describe('Sign-in security (§11.2)', () => {
  test('the settings are read back, checked against 1 to 20 and 1 to 1440, and Save works only on a change', async () => {
    const calls = stubServer({}, (url, init) =>
      url === '/api/v1/admin/lockout' && init?.method === 'POST' ? json({ max_failed_logins: 3, lock_minutes: 60 }) : undefined)
    await open()
    fireEvent.click(screen.getByRole('tab', { name: 'Sign-in security' }))
    const form = screen.getByRole('form', { name: 'Lockout settings' })
    const save = within(form).getByRole('button', { name: 'Save lockout' })
    expect(within(form).getByText(/An account locks after 5 failed sign-ins and stays locked for 15 minutes/)).toBeInTheDocument()
    expect(save).toBeDisabled()

    const failed = within(form).getByLabelText('Failed sign-ins before the lock (1 to 20)')
    const minutes = within(form).getByLabelText('Minutes locked (1 to 1440)')
    fireEvent.change(failed, { target: { value: '21' } })
    expect(within(form).getByText('Enter a whole number from 1 to 20.')).toBeInTheDocument()
    expect(save).toBeDisabled()
    fireEvent.change(failed, { target: { value: '3' } })
    fireEvent.change(minutes, { target: { value: '1441' } })
    expect(within(form).getByText('Enter a whole number from 1 to 1440.')).toBeInTheDocument()
    expect(save).toBeDisabled()
    fireEvent.change(minutes, { target: { value: '15' } })
    expect(within(form).getByText(/An account locks after 3 failed sign-ins and stays locked for 15 minutes/)).toBeInTheDocument()
    expect(save).toBeEnabled()
    fireEvent.change(minutes, { target: { value: '60' } })
    await act(async () => {
      fireEvent.click(save)
    })
    await settle()
    expect(posts(calls).map((c) => [c.url, JSON.parse(c.body)])).toEqual([['/api/v1/admin/lockout', { max_failed_logins: 3, lock_minutes: 60 }]])
    expect(screen.getByRole('list')).toHaveTextContent('argon2id')
  })

  test('returning to the saved values disables Save again', async () => {
    stubServer()
    await open()
    fireEvent.click(screen.getByRole('tab', { name: 'Sign-in security' }))
    const form = screen.getByRole('form', { name: 'Lockout settings' })
    const failed = within(form).getByLabelText('Failed sign-ins before the lock (1 to 20)')
    fireEvent.change(failed, { target: { value: '6' } })
    expect(within(form).getByRole('button', { name: 'Save lockout' })).toBeEnabled()
    fireEvent.change(failed, { target: { value: '5' } })
    expect(within(form).getByRole('button', { name: 'Save lockout' })).toBeDisabled()
  })
})

describe('Audit log (§11.3)', () => {
  async function openAudit(chainOk = true) {
    const calls = stubServer({ chainOk })
    await open()
    fireEvent.click(screen.getByRole('tab', { name: 'Audit log' }))
    return calls
  }
  const seqColumn = () =>
    within(screen.getByRole('table', { name: 'Audit log' })).getAllByRole('row').slice(1).map((r) => within(r).getAllByRole('cell')[3].textContent)

  test('a green banner says the chain is intact with the entry count', async () => {
    await openAudit()
    const banner = screen.getByRole('status')
    expect(banner).toHaveClass('audit-banner', 'pass')
    expect(banner).toHaveTextContent('The audit chain is intact.')
    expect(banner).toHaveTextContent('30 entries in the log.')
  })

  test('a red banner says the chain is broken, with the server\'s reason', async () => {
    await openAudit(false)
    const banner = screen.getByRole('status')
    expect(banner).toHaveClass('audit-banner', 'fail')
    expect(banner).toHaveTextContent('The audit chain is broken.')
    expect(banner).toHaveTextContent('entry 12 does not link to entry 11')
  })

  test('newest first, 25 a page, with each action as a coloured tag', async () => {
    await openAudit()
    const objects = seqColumn()
    expect(objects).toHaveLength(25)
    expect(objects[0]).toBe('user user30')
    expect(objects[24]).toBe('user user6')
    const pages = screen.getByRole('group', { name: 'Pages of the audit log' })
    expect(pages).toHaveTextContent('1–25 of 30')
    fireEvent.click(within(pages).getByRole('button', { name: 'Next' }))
    expect(seqColumn()).toEqual(['user user5', 'user user4', 'user user3', 'user user2', 'user user1'])
    const log = screen.getByRole('table', { name: 'Audit log' })
    expect(within(log).getAllByText('login.success')[0]).toHaveClass('tag', 'pass')
    expect(within(log).getAllByText('user.add')[0]).toHaveClass('tag', 'pass')
    expect(within(log).getAllByText('login.failure')[0]).toHaveClass('tag', 'fail')
  })

  test('filters by who and by action, and searches; a filter starts again at the first page', async () => {
    await openAudit()
    fireEvent.click(within(screen.getByRole('group', { name: 'Pages of the audit log' })).getByRole('button', { name: 'Next' }))
    fireEvent.change(screen.getByLabelText('Who'), { target: { value: 'sunil' } })
    expect(seqColumn()).toEqual(['user user30', 'user user27', 'user user24', 'user user21', 'user user18', 'user user15', 'user user12', 'user user9', 'user user6', 'user user3'])
    expect(screen.queryByRole('group', { name: 'Pages of the audit log' })).not.toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Action'), { target: { value: 'login.failure' } })
    expect(seqColumn()).toEqual(['user user18', 'user user3'])
    fireEvent.change(screen.getByLabelText('Who'), { target: { value: '' } })
    fireEvent.change(screen.getByLabelText('Action'), { target: { value: '' } })
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search the audit log' }), { target: { value: 'user17' } })
    expect(seqColumn()).toEqual(['user user17'])
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search the audit log' }), { target: { value: 'nobody-here' } })
    expect(screen.getByText(/No entry matches/)).toBeInTheDocument()
  })

  test('the screen only reads the log: opening it sends no change', async () => {
    const calls = await openAudit()
    expect(posts(calls)).toHaveLength(0)
  })
})

describe('Users in pages', () => {
  test('twelve users show ten at a time, with the page bar under the list', async () => {
    const many = [me, ...Array.from({ length: 11 }, (_, i) => account(i + 10, `user${i + 10}`, `User ${i + 10}`, 'staff'))]
    stubServer({}, (url, init) => (url === '/api/v1/admin/users' && (init?.method ?? 'GET') === 'GET'
      ? json({ users: many, roles, lockout: { max_failed_logins: 5, lock_minutes: 15 } }) : undefined))
    await open()
    const table = screen.getByRole('table', { name: 'Users' })
    expect(within(table).getAllByRole('row')).toHaveLength(11) // the heading and ten users
    const pages = screen.getByRole('group', { name: 'Pages of users' })
    expect(pages).toHaveTextContent('1–10 of 12')
    // The page bar comes after the table in the page.
    expect(table.compareDocumentPosition(pages) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    fireEvent.click(within(pages).getByRole('button', { name: 'Next' }))
    expect(within(screen.getByRole('table', { name: 'Users' })).getAllByRole('row')).toHaveLength(3)
    expect(screen.getByRole('group', { name: 'Pages of users' })).toHaveTextContent('11–12 of 12')
  })
})
