import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import App from './App.tsx'
import { sampleComparisons, sampleRules, sampleRuns } from './sample/index.ts'
import type { Session } from './types.ts'
import { openFirstClient, railTo } from './testFindings.ts'

// SPEC-013 in the browser (AC-6 rendered, AC-8): the Client file and the Rules screens show an
// action only when the signed-in role holds its permission, and the GSTIN shown is the one the
// server sent. The server is stubbed; the names and figures are illustrative.

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status, headers: { 'content-type': 'application/json' } })

// The permissions each role holds, as the server's matrix gives them (SPEC-012 §3).
const permissions = {
  partner: ['activate_rules', 'decide', 'delete_findings', 'disable_rules', 'draft_rules', 'export', 'manage_users', 'run', 'view'],
  manager: ['decide', 'delete_findings', 'draft_rules', 'export', 'run', 'view'],
  staff: ['decide', 'draft_rules', 'run', 'view'],
  read_only: ['view'],
}

function session(login: string, role: keyof typeof permissions): Session {
  return {
    user: {
      id: 1, login_name: login, display_name: login, role, active: true, must_change_password: false, failed_logins: 0,
      locked: false, locked_until: '', last_login_at: '', created_at: '2026-10-30T09:00:00Z', updated_at: '2026-10-30T09:00:00Z',
    },
    permissions: permissions[role],
    must_change_password: false,
  }
}

// The client's details as the server sends them: the GSTIN in full for a role that enters details,
// masked (last four characters) for a role that only reads.
const fullGSTIN = '27ZLVCD9565Z1ZF'
const details = (gstin: string) => ({
  id: 1,
  details: {
    legal_name: 'Deccan Byte Systems Pvt Ltd', trade_name: '', pan_last4: '565Z', constitution: 'company',
    date_of_incorporation: '', cin: '', tan: '', gstins: [{ gstin, registration_type: 'regular', primary: true }],
    address: { line1: '', line2: '', city: '', state: '', pin: '' }, contact_person: '', email: '', phone: '',
    accounting_software: '', notes: '',
  },
})

const client = { id: 1, name: 'Deccan Byte Systems Pvt Ltd', pan_last4: '565Z', last_run_on: '2026-10-16', documents: 2, counts: null }
const sales = sampleRuns['gst-sales']

// stubClientServer answers the calls the Client file and the Rules screen make for one session.
function stubClientServer(who: Session, gstin: string) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      if (init?.method === 'POST' || init?.method === 'PUT') throw new Error('no write is expected here: ' + url)
      if (url === '/api/v1/me') return json(who)
      if (url === '/api/v1/rules') return json(sampleRules)
      if (url === '/api/v1/comparisons') return json({ comparisons: sampleComparisons })
      if (url.startsWith('/api/v1/clients?')) return json({ tax_year: '2026-27', clients: [client] })
      if (url.startsWith('/api/v1/clients/1/result')) {
        return json({ client, tax_year: '2026-27', run_tax_years: ['2026-27'], report: sales.report,
          results: [{ comparison_id: 'gst_sales_vs_gstr1', last_run_on: '2026-10-16', report: sales.report }], decisions: {}, facts: sales.facts })
      }
      if (url === '/api/v1/clients/1') return json(details(gstin))
      throw new Error('unexpected ' + url)
    }),
  )
}

async function settle() {
  await act(async () => {})
  await act(async () => {})
}

// openClient renders the app and opens the first client's file, as a click on its row does.
async function openClient() {
  render(<App />)
  await settle()
  await act(async () => {
    openFirstClient()
  })
  await settle()
}

describe('the Client file shows only the actions the role holds (SPEC-013 AC-8)', () => {
  test('Partner and Manager see the Excel export and the delete control; Staff and Read-only see neither', async () => {
    for (const [login, role, has] of [['priya', 'partner', true], ['meera', 'manager', true], ['sunil', 'staff', false], ['tara', 'read_only', false]] as const) {
      stubClientServer(session(login, role), fullGSTIN)
      const { unmount } = render(<App />)
      await settle()
      await act(async () => {
        openFirstClient()
      })
      await settle()
      const exportButtons = screen.queryAllByRole('button', { name: 'Export Excel' })
      // Delete finding sits in the finding's … menu, which only a role that may delete has.
      const menus = screen.queryAllByRole('button', { name: 'More actions on this finding' })
      if (menus.length > 0) fireEvent.click(menus[0])
      const deleteButtons = screen.queryAllByRole('button', { name: 'Delete finding' })
      expect(exportButtons.length > 0, `${role}: Export Excel shown`).toBe(has)
      expect(deleteButtons.length > 0, `${role}: Delete finding shown`).toBe(has)
      unmount()
      cleanup()
      vi.unstubAllGlobals()
    }
  })
})

describe('the GSTIN the Client file shows is the one the server sent (SPEC-013 AC-6)', () => {
  test('Read-only sees the GSTIN masked, all but its last four characters', async () => {
    stubClientServer(session('tara', 'read_only'), '***********Z1ZF')
    await openClient()
    expect(document.body.textContent).toContain('***********Z1ZF')
    expect(document.body.textContent).not.toContain(fullGSTIN)
  })

  test('Staff sees the GSTIN in full', async () => {
    stubClientServer(session('sunil', 'staff'), fullGSTIN)
    await openClient()
    expect(document.body.textContent).toContain(fullGSTIN)
  })
})

describe('the Rules screen shows only the actions the role holds (SPEC-013 AC-8)', () => {
  // The Rules screen is opened from the rail.
  async function openRules(who: Session) {
    stubClientServer(who, fullGSTIN)
    render(<App />)
    await settle()
    await act(async () => {
      railTo('Rules')
    })
    await settle()
    // Only runnable rules have row actions. The sample's first runnable rules are the RECON-DEP
    // rules, so the search box shows them.
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search rules' }), { target: { value: 'RECON-DEP' } })
    await settle()
  }

  // A row's actions sit behind its "⋯" button (SPEC-010), so the menu is opened first. A role
  // that holds no action on the row has no "⋯" button at all.
  function rowActions(id: string): string[] {
    const more = screen.queryByRole('button', { name: `Actions for ${id}` })
    if (!more) return []
    fireEvent.click(more)
    return screen.queryAllByRole('menuitem').map((b) => b.textContent ?? '')
  }

  test('Staff can edit a rule but see no disable, enable or approve control', async () => {
    await openRules(session('sunil', 'staff'))
    const actions = rowActions('RECON-DEP-001')
    expect(actions).toContain('Edit')
    expect(actions).not.toContain('Disable')
    expect(actions).not.toContain('Enable again')
    expect(actions).not.toContain('Approve')
  })

  test('Read-only sees no edit, disable, enable or approve control', async () => {
    await openRules(session('tara', 'read_only'))
    expect(rowActions('RECON-DEP-001')).toEqual([])
    expect(screen.queryAllByRole('button', { name: /^Actions for / })).toHaveLength(0)
  })

  test('the Partner, who holds the permissions, sees the disable control (the control is not hidden for everyone)', async () => {
    await openRules(session('priya', 'partner'))
    expect(rowActions('RECON-DEP-001')).toContain('Disable')
  })
})
