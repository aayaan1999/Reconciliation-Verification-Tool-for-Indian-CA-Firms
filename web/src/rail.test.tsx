import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { Rail } from './screens/Rail.tsx'

const base = {
  current: 'season' as const, taxYear: '2025-26', findings: 28, toClassify: 12, showAdmin: false, live: true,
  user: { name: 'A. Staff', role: 'staff' }, theme: 'system' as const,
}

describe('navigation rail (Recon Screen Document v2 §3)', () => {
  it('shows the tax year, the counts waiting on each screen and the engine status', () => {
    render(<Rail {...base} onNav={() => {}} onTheme={() => {}} />)
    const nav = screen.getByRole('navigation', { name: 'Screens' })
    expect(within(nav).getByRole('button', { name: 'Season board, 28 findings' })).toHaveAttribute('aria-current', 'true')
    expect(within(nav).getByRole('button', { name: 'Worklist, 12 to classify' })).toBeInTheDocument()
    expect(within(nav).queryByRole('button', { name: /Admin/ })).not.toBeInTheDocument()
    expect(screen.getByText('2025-26')).toBeInTheDocument()
    expect(screen.getByText('Connected to the engine')).toBeInTheDocument()
  })

  it('leaves the Worklist out when it is switched off', () => {
    render(<Rail {...base} showWorklist={false} onNav={() => {}} onTheme={() => {}} />)
    const nav = screen.getByRole('navigation', { name: 'Screens' })
    expect(within(nav).queryByRole('button', { name: /Worklist/ })).not.toBeInTheDocument()
    expect(within(nav).getByRole('button', { name: 'Season board, 28 findings' })).toBeInTheDocument()
  })

  it('collapses with its round button and expands again', () => {
    render(<Rail {...base} onNav={() => {}} onTheme={() => {}} />)
    fireEvent.click(screen.getByRole('button', { name: 'Collapse the menu' }))
    expect(screen.getByRole('button', { name: 'Expand the menu' })).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(screen.getByRole('button', { name: 'Expand the menu' }))
    expect(screen.getByRole('button', { name: 'Collapse the menu' })).toHaveAttribute('aria-expanded', 'true')
  })

  it('holds theme and sign-out in the user menu; a click outside closes it', () => {
    const onTheme = vi.fn()
    const onSignOut = vi.fn()
    render(<Rail {...base} onNav={() => {}} onTheme={onTheme} onSignOut={onSignOut} />)
    fireEvent.click(screen.getByRole('button', { name: 'A. Staff, staff: menu' }))
    const menu = screen.getByRole('menu', { name: 'User menu' })
    expect(within(menu).getByRole('menuitemradio', { name: /Match the computer/ })).toHaveAttribute('aria-checked', 'true')
    expect(within(menu).queryByRole('menuitem', { name: /Keyboard shortcuts/ })).not.toBeInTheDocument()
    fireEvent.click(within(menu).getByRole('menuitemradio', { name: /Dark/ }))
    expect(onTheme).toHaveBeenCalledWith('dark')
    fireEvent.mouseDown(document.body)
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'A. Staff, staff: menu' }))
    fireEvent.click(screen.getByRole('menuitem', { name: /Sign out/ }))
    expect(onSignOut).toHaveBeenCalled()
  })
})
