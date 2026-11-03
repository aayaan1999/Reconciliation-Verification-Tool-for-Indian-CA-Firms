import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { canFrom, PermissionContext } from './permissions.ts'
import { Rules } from './screens/Rules.tsx'
import type { RuleInfo, RuleList } from './types.ts'

// SPEC-010 scope 9: several rules switched off or on with one reason. Rules are illustrative.

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function rule(over: Partial<RuleInfo>): RuleInfo {
  return {
    id: 'RECON-XYZ-001', version: '1', revision: 1, source: 'builtin', status: 'testing', runnable: true, type: 'tie-out', domain: '',
    category: 'R1', title: 'Illustrative rule', tax_years: ['2025-26'], valid_from: '2025-04-01', valid_to: '',
    file_path: 'x', approvable: false, approval_note: '', ...over,
  }
}

const book: RuleList = {
  tax_years: ['2025-26'],
  rules: [
    rule({ id: 'RECON-DEP-001', title: 'Alpha rule', status: 'active' }),
    rule({ id: 'RECON-DEP-002', title: 'Bravo rule', status: 'testing' }),
    rule({ id: 'RC-GST-001', title: 'Delta rule', status: 'disabled', runnable: false, source: 'rulebook', type: 'statutory', domain: 'gst' }),
    rule({ id: 'RC-GST-002', title: 'Echo rule', status: 'disabled', runnable: false, source: 'rulebook', type: 'statutory', domain: 'gst' }),
  ],
}

const partner = canFrom(['activate_rules', 'disable_rules', 'draft_rules', 'view'])

function show(can = partner, onChanged = () => {}) {
  render(
    <PermissionContext.Provider value={can}>
      <Rules list={book} live onChanged={onChanged} />
    </PermissionContext.Provider>,
  )
}
const bar = () => screen.queryByRole('region', { name: 'Selected rules' })

describe('Disable or enable several rules (SPEC-010 scope 9)', () => {
  test('each row has a tick box; the counts say what each action can change', () => {
    show()
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select RECON-DEP-001' }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select RC-GST-001' }))
    // Ticking does not open the rule.
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(bar()).toHaveTextContent('2 rules selected')
    expect(within(bar()!).getByRole('button', { name: 'Disable (1)' })).toBeInTheDocument()
    expect(within(bar()!).getByRole('button', { name: 'Enable again (1)' })).toBeInTheDocument()
    fireEvent.click(within(bar()!).getByRole('button', { name: 'Clear selection' }))
    expect(bar()).not.toBeInTheDocument()
  })

  test('the heading selects every rule the filters show, and keeps hidden ticks', () => {
    show()
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search rules' }), { target: { value: 'RC-GST' } })
    const all = screen.getByRole('checkbox', { name: 'Select all 2 rules shown' })
    fireEvent.click(all)
    expect(bar()).toHaveTextContent('2 rules selected')
    expect(within(bar()!).queryByRole('button', { name: /^Disable/ })).not.toBeInTheDocument()
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search rules' }), { target: { value: 'RECON' } })
    expect(bar()).toHaveTextContent('2 rules selected (2 hidden by the filters)')
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select all 2 rules shown' }))
    expect(bar()).toHaveTextContent('4 rules selected')
    expect(within(bar()!).getByRole('button', { name: 'Disable (2)' })).toBeInTheDocument()
    expect(within(bar()!).getByRole('button', { name: 'Enable again (2)' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select all 2 rules shown' }))
    expect(bar()).toHaveTextContent('2 rules selected (2 hidden by the filters)')
  })

  test('one reason switches them all in one request, then the list is read again', async () => {
    const calls: { url: string; body: unknown }[] = []
    vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, body: JSON.parse(String(init?.body ?? '{}')) })
      return new Response('{"rules":[]}', { status: 200, headers: { 'content-type': 'application/json' } })
    }))
    const onChanged = vi.fn()
    show(partner, onChanged)
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select RECON-DEP-001' }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select RECON-DEP-002' }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select RC-GST-001' }))
    fireEvent.click(within(bar()!).getByRole('button', { name: 'Disable (2)' }))
    const dialog = screen.getByRole('dialog', { name: 'Switch off 2 rules' })
    expect(within(dialog).getByRole('list', { name: 'Rules to change' })).toHaveTextContent('RECON-DEP-001RECON-DEP-002')
    const submit = within(dialog).getByRole('button', { name: 'Switch off 2 rules' })
    expect(submit).toBeDisabled()
    fireEvent.change(within(dialog).getByRole('textbox'), { target: { value: 'not used this season' } })
    await act(async () => {
      fireEvent.click(submit)
    })
    expect(calls).toEqual([{ url: '/api/v1/rules/bulk/disable', body: { rule_ids: ['RECON-DEP-001', 'RECON-DEP-002'], reason: 'not used this season' } }])
    expect(onChanged).toHaveBeenCalled()
    expect(bar()).not.toBeInTheDocument()
  })

  test('a refusal is shown and nothing is cleared', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ error_code: 'NOT_DISABLED', message: 'RC-GST-001 is not switched off' }),
      { status: 409, headers: { 'content-type': 'application/json' } })))
    show()
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select RC-GST-001' }))
    fireEvent.click(within(bar()!).getByRole('button', { name: 'Enable again (1)' }))
    const dialog = screen.getByRole('dialog', { name: 'Enable 1 rule again' })
    fireEvent.change(within(dialog).getByRole('textbox'), { target: { value: 'listed as in use' } })
    await act(async () => {
      fireEvent.click(within(dialog).getByRole('button', { name: 'Enable 1 rule again' }))
    })
    expect(within(dialog).getByRole('alert')).toHaveTextContent('RC-GST-001 is not switched off')
    expect(bar()).toHaveTextContent('1 rule selected')
  })

  test('a role that may not disable or enable rules gets no tick boxes', () => {
    show(canFrom(['draft_rules', 'view']))
    expect(screen.queryAllByRole('checkbox')).toHaveLength(0)
  })

  test('a role that may disable but not enable sees only Disable', () => {
    show(canFrom(['disable_rules', 'view']))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select RECON-DEP-001' }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select RC-GST-001' }))
    expect(within(bar()!).getByRole('button', { name: 'Disable (1)' })).toBeInTheDocument()
    expect(within(bar()!).queryByRole('button', { name: /Enable again/ })).not.toBeInTheDocument()
  })
})
