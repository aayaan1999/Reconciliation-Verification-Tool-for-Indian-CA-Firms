import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, test } from 'vitest'
import { MultiSelect } from './screens/MultiSelect.tsx'
import { builderTaxYears } from './filters.ts'

afterEach(cleanup)

function Host() {
  const [v, setV] = useState<string[]>([])
  return <MultiSelect label="Status" options={[{ value: 'a', label: 'Alpha' }, { value: 'b', label: 'Beta' }]} value={v} onChange={setV} />
}

describe('MultiSelect', () => {
  test('opens, toggles, clears and closes on Escape with focus back on the button', () => {
    render(<Host />)
    const button = screen.getByRole('button', { name: 'Status: All' })
    expect(button).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(button)
    expect(button).toHaveAttribute('aria-expanded', 'true')
    fireEvent.click(screen.getByRole('checkbox', { name: 'Alpha' }))
    expect(screen.getByRole('button', { name: 'Status: Alpha' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('checkbox', { name: 'Beta' }))
    expect(screen.getByRole('button', { name: 'Status: 2 selected' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }))
    expect(screen.getByRole('button', { name: 'Status: All' })).toBeInTheDocument()
    fireEvent.keyDown(screen.getByRole('checkbox', { name: 'Alpha' }), { key: 'Escape' })
    expect(screen.queryByRole('checkbox')).toBeNull()
    expect(document.activeElement).toBe(button)
  })

  test('Rule Builder offers 2017-18 to 2030-31', () => {
    const y = builderTaxYears()
    expect(y[0]).toBe('2017-18')
    expect(y[y.length - 1]).toBe('2030-31')
    expect(y).toHaveLength(14)
  })
})
