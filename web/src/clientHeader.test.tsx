import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { ClientFile } from './screens/ClientFile.tsx'

// The Client file header and its start (Recon Screen Document v2 §6.1). Invented client only.
const client = { id: 'c9', name: 'Invented Analytics LLP', pan_last4: '901X', runs: [], last_run_on: '' }

function renderNew(live = true) {
  return render(
    <ClientFile client={client} runs={[]} rules={[]} live={live} fromUpload={false} taxYears={['2025-26']} onRun={() => {}} />,
  )
}

describe('Client file header (§6.1)', () => {
  afterEach(() => vi.unstubAllGlobals())

  test('shows the client with its PAN ending as a chip, Upload and run in the header, and the files to bring', () => {
    renderNew()
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Invented Analytics LLP')
    expect(screen.getByLabelText('Client details')).toHaveTextContent('PAN ending 901X')
    expect(screen.getByRole('button', { name: 'Upload and run' })).toBeInTheDocument()
    expect(screen.getByText(/No run yet for this client/)).toHaveTextContent('press Upload and run, or drop the files anywhere on this page')
    const bring = screen.getByRole('heading', { name: 'Files to bring' }).nextElementSibling as HTMLElement
    expect(within(bring).getAllByRole('listitem').map((li) => li.querySelector('strong')?.textContent)).toEqual([
      'The books', 'The tax suite', 'The portals',
    ])
  })

  test('files dropped anywhere on the page open Upload and run with them chosen', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('{}', { status: 500 })))
    renderNew()
    const page = screen.getByRole('heading', { level: 1 }).closest('section') as HTMLElement
    const file = new File(['{}'], 'ledger_2025-26.json', { type: 'application/json' })
    const dataTransfer = { files: [file], types: ['Files'] }
    fireEvent.dragOver(page, { dataTransfer })
    expect(page).toHaveClass('dragging')
    await act(async () => {
      fireEvent.drop(page, { dataTransfer })
    })
    expect(page).not.toHaveClass('dragging')
    const dialog = screen.getByRole('dialog', { name: 'Upload and run' })
    expect(within(dialog).getByLabelText('Files chosen')).toHaveTextContent('ledger_2025-26.json')
  })

  test('without a server, dropped files are not taken', () => {
    renderNew(false)
    const page = screen.getByRole('heading', { level: 1 }).closest('section') as HTMLElement
    fireEvent.dragOver(page, { dataTransfer: { files: [], types: ['Files'] } })
    expect(page).not.toHaveClass('dragging')
  })
})
