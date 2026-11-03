import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'
import { ClientFile } from './screens/ClientFile.tsx'
import { SeasonBoard, type BoardRow } from './screens/SeasonBoard.tsx'
import { PermissionContext, canFrom } from './permissions.ts'

// SPEC-004 part 10: disabling a client. Invented clients only.
const client = { id: 'c9', name: 'Invented Analytics LLP', pan_last4: '901X', runs: [], last_run_on: '' }

function renderFile(props: Partial<Parameters<typeof ClientFile>[0]>, permissions = ['view', 'run', 'decide', 'disable_clients']) {
  return render(
    <PermissionContext.Provider value={canFrom(permissions)}>
      <ClientFile client={client} runs={[]} rules={[]} live fromUpload={false} taxYears={['2025-26']} onRun={() => {}} {...props} />
    </PermissionContext.Provider>,
  )
}

describe('Disabling a client (SPEC-004 part 10)', () => {
  test('a Partner disables the client with a reason; the reason is required', async () => {
    const onSet = vi.fn(async () => {})
    renderFile({ onSetDisabled: onSet })
    fireEvent.click(screen.getByRole('button', { name: 'Disable client' }))
    const dialog = screen.getByRole('dialog', { name: 'Disable this client?' })
    const submit = within(dialog).getByRole('button', { name: 'Disable client' })
    expect(submit).toBeDisabled()
    fireEvent.change(within(dialog).getByRole('textbox'), { target: { value: ' no longer a client ' } })
    await act(async () => {
      fireEvent.click(submit)
    })
    expect(onSet).toHaveBeenCalledWith(true, 'no longer a client')
    expect(screen.queryByRole('dialog', { name: 'Disable this client?' })).not.toBeInTheDocument()
  })

  test('a disabled client opens read only, with the banner and Enable again', async () => {
    const onSet = vi.fn(async () => {})
    renderFile({ disabled: { on: '2026-11-02', by: 'A. Partner', reason: 'no longer a client' }, onSetDisabled: onSet })
    expect(screen.getByRole('note')).toHaveTextContent('Disabled on 2026-11-02 by A. Partner: no longer a client')
    expect(screen.queryByRole('button', { name: 'Upload and run' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Disable client' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Enable again' }))
    const dialog = screen.getByRole('dialog', { name: 'Enable this client again?' })
    fireEvent.change(within(dialog).getByRole('textbox'), { target: { value: 'engaged again' } })
    await act(async () => {
      fireEvent.click(within(dialog).getByRole('button', { name: 'Enable again' }))
    })
    expect(onSet).toHaveBeenCalledWith(false, 'engaged again')
  })

  test('without the permission there is no Disable or Enable action', () => {
    renderFile({ disabled: { on: '2026-11-02', by: 'A. Partner', reason: 'gone' } }, ['view', 'run'])
    expect(screen.queryByRole('button', { name: 'Enable again' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Upload and run' })).not.toBeInTheDocument()
  })

  test('the Season board leaves disabled clients off until asked, and marks them', () => {
    const rows: BoardRow[] = [{ id: '1', name: 'Alpha Clear LLP', pan_last4: '111A', last_run_on: '2026-10-01', files: 3, fail: 0, cannot: 0 }]
    const gone: BoardRow[] = [{ id: '2', name: 'Omega Closed Traders', pan_last4: '999Z', last_run_on: '', files: null, fail: null, cannot: null }]
    render(<SeasonBoard taxYear="2025-26" rows={rows} disabledRows={gone} live taxYears={['2025-26']} onOpen={() => {}} onAdded={() => {}} />)
    const table = screen.getByRole('table', { name: 'Clients' })
    expect(within(table).queryByText('Omega Closed Traders')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('checkbox', { name: 'Show disabled clients (1)' }))
    const row = within(table).getByText('Omega Closed Traders').closest('tr') as HTMLElement
    expect(row).toHaveClass('row-disabled')
    expect(within(row).getByText('Disabled')).toBeInTheDocument()
    // The tiles count only the clients in use.
    expect(within(screen.getByRole('group', { name: /Season summary/ })).getByRole('button', { name: /^Clients/ })).toHaveTextContent('1')
  })
})
