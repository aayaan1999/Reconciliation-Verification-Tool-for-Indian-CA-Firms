import { fireEvent, screen, within } from '@testing-library/react'

// The client file shows one finding at a time: showFinding chooses the finding named in the
// findings index, as a user would, and returns its card.
export function showFinding(name: RegExp): HTMLElement {
  const index = screen.getByRole('navigation', { name: 'Findings by document to fix' })
  fireEvent.click(within(index).getByRole('button', { name }))
  return screen.getByRole('article', { name })
}

// openBoardClient opens a client from the Season board with the keyboard, as a user would:
// arrows down to its row, then Enter. The board lists clients needing work first, so the
// row is found by name, not by its place in the sample list.
export function openBoardClient(name: string): void {
  const rows = within(screen.getByRole('table', { name: 'Clients' })).getAllByRole('row').slice(1)
  const at = rows.findIndex((r) => r.textContent?.includes(name))
  if (at < 0) throw new Error(`no client ${name} on the board`)
  const key = (k: string) => fireEvent.keyDown(document.activeElement ?? document.body, { key: k })
  for (let i = 0; i < at; i++) key('ArrowDown')
  key('Enter')
}

// showView opens one of the views of the type of check shown: "Findings", "Documents" or
// "Run history". A view's button may carry its count after the name ("Findings 28").
export function showView(name: string): void {
  const view = within(screen.getByRole('group', { name: 'Views of this check' }))
    .getByRole('button', { name: (n) => n === name || n.startsWith(`${name} `) })
  fireEvent.click(view)
}

// showResults chooses one kind of result in the result filter: "Findings", "Cannot check",
// "Passed" or "All". Each but All carries its count after the name.
export function showResults(name: string): void {
  const segment = within(screen.getByRole('group', { name: 'Result filter' }))
    .getByRole('button', { name: (n) => n === name || n.startsWith(`${name} `) })
  fireEvent.click(segment)
}
