import '../styles/palette.css'
import { useEffect, useRef, type KeyboardEvent } from 'react'
import { CloseIcon } from '../icons.tsx'

// The keys the screens answer to today, grouped as the Recon Screen Document v2 §12.1 groups
// them. A key is listed only where a handler for it exists.
const groups: { title: string; keys: [string[], string][] }[] = [
  {
    title: 'Anywhere',
    keys: [
      [['Ctrl', 'K'], 'Search clients, rules and actions'],
      [['1'], 'Season board'],
      [['2'], 'Worklist'],
      [['3'], 'Rules'],
      [['4'], 'Admin (the Partner only)'],
      [['['], 'Collapse or expand the menu'],
      [['?'], 'Show or hide this sheet'],
      [['Esc'], 'Close the top panel, or go back to the Season board'],
    ],
  },
  {
    title: 'Lists and tables',
    keys: [
      [['/'], 'Search the list (Season board and Rules)'],
      [['↑', '↓'], 'Move between rows (j and k also work)'],
      [['Home', 'End'], 'First or last row'],
      [['PgUp', 'PgDn'], 'Previous or next page'],
      [['Enter'], 'Open the row'],
      [['n'], 'Add client (Season board, with the server running)'],
      [['Space'], 'Tick a Worklist item, to give several items one classification'],
      [['Ctrl', 'Enter'], 'Save the client form'],
    ],
  },
  {
    title: 'Client file',
    keys: [
      [['u'], 'Upload and run'],
      [['e'], 'Edit the client details'],
      [['←', '→'], 'Move between the types of check'],
      [['j', 'k'], 'Move between findings when a finding is selected'],
      [['f'], 'Fix the selected finding'],
      [['x'], 'Mark the selected finding Not an issue (a note is needed)'],
      [['a'], 'Ask client about the selected finding'],
    ],
  },
]

// The keys of the Worklist, left off the sheet while the Worklist is hidden (features.ts).
const worklistKeys = new Set(['Worklist', 'Tick a Worklist item, to give several items one classification'])

// Shortcuts is the "?" sheet. Esc or the close button shuts it, and focus goes back where it was.
export function Shortcuts({ onClose, showWorklist = true }: { onClose: () => void; showWorklist?: boolean }) {
  const shown = groups.map((g) => ({ ...g, keys: g.keys.filter(([, text]) => showWorklist || !worklistKeys.has(text)) }))
  const close = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    const before = document.activeElement as HTMLElement | null
    close.current?.focus()
    return () => before?.focus?.()
  }, [])
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Tab') {
      // One focusable control: keep focus inside the sheet.
      e.preventDefault()
      close.current?.focus()
    }
  }
  return (
    <div className="sheet-backdrop" onClick={onClose}>
      <div className="sheet shortcuts" role="dialog" aria-modal="true" aria-labelledby="shortcuts-title"
        onClick={(e) => e.stopPropagation()} onKeyDown={onKeyDown}>
        <div className="shortcuts-head">
          <h2 id="shortcuts-title">Keyboard shortcuts</h2>
          <button ref={close} type="button" className="ghost icon-btn" onClick={onClose} aria-label="Close keyboard shortcuts">
            <CloseIcon />
          </button>
        </div>
        <p className="shortcuts-note">Single keys do nothing while you are typing in a field.</p>
        <div className="shortcuts-groups">
          {shown.map((g, i) => (
            <section key={g.title} aria-labelledby={`shortcuts-group-${i}`}>
              <h3 id={`shortcuts-group-${i}`}>{g.title}</h3>
              <dl>
                {g.keys.map(([ks, text]) => (
                  <div key={text} className="shortcuts-row">
                    <dt>
                      {ks.map((k) => (
                        <kbd key={k}>{k}</kbd>
                      ))}
                    </dt>
                    <dd>{text}</dd>
                  </div>
                ))}
              </dl>
            </section>
          ))}
        </div>
      </div>
    </div>
  )
}
