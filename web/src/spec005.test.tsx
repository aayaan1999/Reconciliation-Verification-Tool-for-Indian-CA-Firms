import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, test } from 'vitest'
import App from './App.tsx'
import { bannedIn } from './wording.ts'
import { showFinding, showView, openBoardClient } from './testFindings.ts'

// SPEC-005: the tax suite's filing pack (Test Pack A, fictional Nimbus Logic Systems Private
// Limited) is one JSON file holding several documents; each is its own row and opens its
// own figures.

function openPack() {
  render(<App />)
  openBoardClient('Nimbus Logic Systems Private Limited')
  expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Nimbus Logic Systems Private Limited (Test Pack A)')
}

describe('A file that holds several documents (SPEC-005)', () => {
  test('AC-7: each document of the pack is its own row, and opens its own figures', () => {
    openPack()
    showView('Documents')
    const docs = screen.getByRole('table', { name: 'Documents' })
    const rows = within(docs).getAllByRole('row').slice(1)
    expect(rows.map((r) => (r as HTMLTableRowElement).cells[0].textContent)).toEqual([
      '09_filing_pack_structured.json, computation',
      '09_filing_pack_structured.json, financial statements',
      '09_filing_pack_structured.json, form 26as',
      '09_filing_pack_structured.json, form 67',
      '09_filing_pack_structured.json, mat report',
      '09_filing_pack_structured.json, tax audit report',
    ])

    // The tax audit report: its figures and rows, each with its place in the file.
    fireEvent.click(rows[5])
    const panel = screen.getByLabelText('Read from 09_filing_pack_structured.json, tax audit report')
    expect(panel).toHaveTextContent('tar.depreciation_schedule.total')
    expect(panel).toHaveTextContent('1,35,862.00')
    expect(panel).toHaveTextContent('$.form_3cd.clause_18_depreciation.total.depreciation_allowable')
  })

  test('the planted defects F01 and F03 show both values, the difference and both sources', () => {
    openPack()
    const dep = showFinding(/RECON-DEP-002/)
    for (const text of ['1,35,862.00', '1,38,310.00', '2,448.00', '$.form_3cd.clause_18_depreciation.total.depreciation_allowable', '$.computation.less.allowed_depreciation']) {
      expect(dep).toHaveTextContent(text)
    }
    const mat = showFinding(/RECON-MAT-002/)
    for (const text of ['61,96,838.00', '63,58,482.00', '1,61,644.00', '$.form_29B.sl_20_tax_payable_115JB']) {
      expect(mat).toHaveTextContent(text)
    }
    expect(showFinding(/RECON-MAT-001/)).toHaveTextContent('9,68,390.00')
    expect(bannedIn(document.body.textContent ?? '')).toEqual([])
  })
})

describe('Findings of the SPEC-006 check types', () => {
  test('a sum shows its parts, the computed sum (no source) and the difference', () => {
    openPack()
    const sum = showFinding(/RECON-PGBP-001/)
    for (const text of ['less: Tax audit report, clause 18 depreciation', 'Sum of the figures above', '3,50,02,268.00', '3,49,99,820.00', 'Difference', '2,448.00']) {
      expect(sum).toHaveTextContent(text)
    }
    // A figure read from a file names its source under its label; the computed sum has none.
    const sumRow = within(sum).getByText('Sum of the figures above').closest('tr') as HTMLTableRowElement
    expect(sumRow.querySelector('.src-line')).toBeNull()
    const partRow = within(sum).getByText(/less: Tax audit report, clause 18 depreciation/).closest('tr') as HTMLTableRowElement
    expect(partRow.querySelector('.src-line')?.textContent).not.toBe('')
  })

  test('values found together show no difference; a blank value says so', () => {
    openPack()
    const together = showFinding(/RECON-TAR-008/)
    expect(together).toHaveTextContent('Gross receipts of profession')
    expect(together).toHaveTextContent('tested: contains "profession"')
    expect(together).not.toHaveTextContent('Difference')
    const blank = showFinding(/RECON-TAR-050/)
    expect(blank).toHaveTextContent('(blank)')
    const table = showFinding(/RECON-FTC-002/)
    expect(table).toHaveTextContent('Rule table: India–Portugal DTAA, Article 12(2)')
    expect(table).toHaveTextContent('16.69')
    expect(table).toHaveTextContent('India–Portugal DTAA – Article 12 (paragraph 2)')
    expect(bannedIn(document.body.textContent ?? '')).toEqual([])
  })

  test('one finding is shown at a time; the index lists them all and shows the one chosen', () => {
    openPack()
    const steps = screen.getByRole('group', { name: 'Move between findings' })
    // 27 answer-key findings, and the Rs 10 the pack's own MAT credit table is apart (SPEC-007).
    expect(steps).toHaveTextContent('Finding 1 of 28')
    expect(screen.getAllByRole('article')).toHaveLength(1)
    expect(screen.queryByRole('article', { name: /RECON-TAR-050/ })).toBeNull()
    const index = screen.getByRole('navigation', { name: 'Findings by document to fix' })
    expect(within(index).getAllByRole('button')).toHaveLength(28)
    const card = showFinding(/RECON-TAR-050/)
    expect(document.activeElement).toBe(card)
    expect(screen.getAllByRole('article')).toHaveLength(1)
    // Previous chooses the finding before it; Next goes back.
    const at = Number(/Finding (\d+) of/.exec(steps.textContent ?? '')?.[1])
    fireEvent.click(within(steps).getByRole('button', { name: 'Previous' }))
    expect(steps).toHaveTextContent(`Finding ${at - 1} of 28`)
    fireEvent.click(within(steps).getByRole('button', { name: 'Next' }))
    expect(screen.getByRole('article', { name: /RECON-TAR-050/ })).toBeInTheDocument()
  })
})
