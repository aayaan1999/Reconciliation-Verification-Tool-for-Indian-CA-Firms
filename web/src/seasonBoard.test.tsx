import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { SeasonBoard, type BoardRow } from './screens/SeasonBoard.tsx'

// Invented clients: one clear, one never run, one with findings and cannot-check results.
const rows: BoardRow[] = [
  { id: '1', name: 'Alpha Clear LLP', pan_last4: '111A', last_run_on: '2026-10-01', files: 3, fail: 0, cannot: 0 },
  { id: '2', name: 'Beta New Traders', pan_last4: '222B', last_run_on: '', files: null, fail: null, cannot: null },
  { id: '3', name: 'Gamma Works', pan_last4: '333C', last_run_on: '2026-10-02', files: 4, fail: 5, cannot: 2 },
]

const board = (r: BoardRow[], live = false) =>
  render(<SeasonBoard taxYear="2025-26" rows={r} live={live} taxYears={['2025-26']} onOpen={() => {}} onAdded={() => {}} />)

const names = () =>
  within(screen.getByRole('table', { name: 'Clients' }))
    .getAllByRole('row')
    .slice(1)
    .map((tr) => tr.querySelector('.client-name')?.textContent?.replace(/^[A-Z]{1,2}/, ''))

describe('Season board (Recon Screen Document v2 §5)', () => {
  it('lists clients needing work first: findings, then no run yet, then clear', () => {
    board(rows)
    expect(names()).toEqual(['Gamma Works', 'Beta New Traders', 'Alpha Clear LLP'])
  })

  it('filters the table from a KPI tile, and the same tile shows all again', () => {
    board(rows)
    const tiles = screen.getByRole('group', { name: /Season summary/ })
    const findings = within(tiles).getByRole('button', { name: /With findings to review/ })
    expect(findings).toHaveTextContent('5 findings')
    fireEvent.click(findings)
    expect(findings).toHaveAttribute('aria-pressed', 'true')
    expect(names()).toEqual(['Gamma Works'])
    fireEvent.click(within(tiles).getByRole('button', { name: /No run yet/ }))
    expect(names()).toEqual(['Beta New Traders'])
    fireEvent.click(within(tiles).getByRole('button', { name: /No run yet/ }))
    expect(names()).toHaveLength(3)
    expect(screen.getByText('2 of 3 clients checked')).toBeInTheDocument()
  })

  it('shows the three steps of a season on an empty board, with one Add client button', () => {
    board([], true)
    expect(screen.getByText(/No clients yet/)).toBeInTheDocument()
    expect(screen.getAllByRole('listitem')).toHaveLength(3)
    expect(screen.getAllByRole('button', { name: '+ Add client' })).toHaveLength(1)
    expect(screen.queryByRole('table', { name: 'Clients' })).not.toBeInTheDocument()
  })
})
