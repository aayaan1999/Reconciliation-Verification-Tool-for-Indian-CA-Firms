import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterAll, afterEach, beforeAll, describe, expect, test, vi } from 'vitest'
import App from './App.tsx'
import { features } from './features.ts'
import { sampleRules } from './sample/index.ts'
import { openFirstClient, railTo } from './testFindings.ts'

// The app has no keyboard shortcuts and no key handling of its own (product owner, 2 November
// 2026): a key does only what the browser does with it. The Worklist is switched on here so
// that its old key 2 would have had a screen to open.
beforeAll(() => {
  features.worklist = true
})
afterAll(() => {
  features.worklist = false
})
afterEach(() => vi.unstubAllGlobals())

// The keys that used to be shortcuts, pressed where a global shortcut listened.
const oldKeys: { key: string; ctrlKey?: boolean }[] = [
  { key: '1' }, { key: '2' }, { key: '3' }, { key: '4' }, { key: '?' }, { key: 'n' }, { key: '/' },
  { key: 'u' }, { key: 'e' }, { key: '[' }, { key: 'k', ctrlKey: true },
]

function pressOldKeys(target: Element = document.body) {
  for (const k of oldKeys) fireEvent.keyDown(target, k)
}

// noKeyHints checks that the screen shows no key hint: no kbd element, and no control that
// announces a shortcut.
function noKeyHints() {
  expect(document.querySelectorAll('kbd')).toHaveLength(0)
  expect(document.querySelectorAll('[aria-keyshortcuts]')).toHaveLength(0)
}

const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status, headers: { 'content-type': 'application/json' } })

describe('no keyboard shortcuts', () => {
  test('the old shortcut keys change nothing on the Season board', () => {
    render(<App />)
    pressOldKeys()
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Season board')
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(screen.getByRole('button', { name: 'Collapse the menu' })).toHaveAttribute('aria-expanded', 'true')
  })

  test('arrows, Enter and Esc on the Season board table change nothing', () => {
    render(<App />)
    const table = screen.getByRole('table', { name: 'Clients' })
    const before = table.innerHTML
    for (const key of ['ArrowDown', 'ArrowUp', 'End', 'Home', 'PageDown', 'Enter', 'Escape']) fireEvent.keyDown(table, { key })
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Season board')
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(table.innerHTML).toBe(before)
  })

  test('the old Client file keys open nothing, and Esc does not leave the Client file', () => {
    render(<App />)
    openFirstClient()
    const name = screen.getByRole('heading', { level: 1 }).textContent
    pressOldKeys()
    for (const key of ['j', 'k', 'f', 'x', 'a', 'Escape']) fireEvent.keyDown(document.body, { key })
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(name ?? '')
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  test('no key hints on the Season board, a Client file, Rules or the Worklist', () => {
    render(<App />)
    noKeyHints()
    openFirstClient()
    noKeyHints()
    railTo('Rules')
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Rules')
    noKeyHints()
    railTo('Worklist')
    noKeyHints()
    fireEvent.click(screen.getByRole('button', { name: /: menu$|^Settings menu$/ }))
    expect(within(screen.getByRole('menu', { name: 'User menu' })).queryByText(/Keyboard shortcuts/)).toBeNull()
    expect(screen.queryByRole('button', { name: 'Search or jump' })).toBeNull()
  })

  test('the client form: no key hints, and Esc does not close it', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) => {
        if (url === '/api/v1/rules') return json(sampleRules)
        if (url.startsWith('/api/v1/clients?')) return json({ tax_year: '2025-26', clients: [] })
        throw new Error('unexpected ' + url)
      }),
    )
    render(<App />)
    await act(async () => {})
    await act(async () => {})
    fireEvent.click(screen.getByRole('button', { name: '+ Add client' }))
    const dialog = screen.getByRole('dialog', { name: 'Add client' })
    noKeyHints()
    expect(dialog).not.toHaveTextContent('Ctrl')
    // The footer holds Cancel and Save only.
    const footer = dialog.querySelector('.client-form-actions') as HTMLElement
    expect(within(footer).getAllByRole('button').map((b) => b.textContent)).toEqual(['Cancel', 'Save client'])
    fireEvent.keyDown(screen.getByLabelText('Legal name'), { key: 'Escape' })
    fireEvent.keyDown(dialog, { key: 'Escape' })
    pressOldKeys(screen.getByLabelText('Legal name'))
    expect(screen.getByRole('dialog', { name: 'Add client' })).toBeInTheDocument()
    fireEvent.click(within(footer).getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
