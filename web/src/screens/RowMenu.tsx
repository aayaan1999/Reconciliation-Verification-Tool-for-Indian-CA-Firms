import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import { MoreIcon } from '../icons.tsx'

export interface RowAction {
  label: string
  onSelect: () => void
  danger?: boolean // shown in the failure colour (Disable)
}

// RowMenu is a row's actions behind one "⋯" button: a click opens the list of actions, and a
// click on one runs it. It is a menu for the keyboard too: Enter or Space opens it with the
// first action focused, ArrowDown and ArrowUp (Home, End) move between actions, Esc closes it
// and puts focus back on the button, and Tab closes it. A click outside closes it. Its keys and
// clicks stay inside it, so the row's own Enter (open the rule) and click do not fire.
// The list is placed against the window, under the button (above it near the bottom of the
// window), so a table's edge never cuts it off. A row with no actions shows no button.
// The list is drawn in <body> (a portal): a parent with a transform, such as the screen's and
// the table rows' entry animations, would otherwise take over its fixed position and move it
// away from the button. React still passes its keys and clicks up through this component.
export function RowMenu({ label, actions }: { label: string; actions: RowAction[] }) {
  const [open, setOpen] = useState(false)
  const [place, setPlace] = useState<CSSProperties>({})
  const button = useRef<HTMLButtonElement>(null)
  const menu = useRef<HTMLDivElement>(null)

  const close = (refocus: boolean) => {
    setOpen(false)
    if (refocus) button.current?.focus()
  }

  // Place the list next to the button before it is painted.
  useLayoutEffect(() => {
    if (!open || !button.current) return
    const r = button.current.getBoundingClientRect()
    const right = Math.max(8, window.innerWidth - r.right)
    const below = r.bottom + 4 + 44 * actions.length + 12 <= window.innerHeight
    setPlace(below ? { top: r.bottom + 4, right } : { bottom: window.innerHeight - r.top + 4, right })
  }, [open, actions.length])

  useEffect(() => {
    if (!open) return
    menu.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus()
    const onDown = (e: MouseEvent) => {
      if (!menu.current?.contains(e.target as Node) && !button.current?.contains(e.target as Node)) setOpen(false)
    }
    const onMove = () => setOpen(false) // the list is placed against the window, so it closes when the page moves
    document.addEventListener('mousedown', onDown)
    window.addEventListener('resize', onMove)
    window.addEventListener('scroll', onMove, true)
    return () => {
      document.removeEventListener('mousedown', onDown)
      window.removeEventListener('resize', onMove)
      window.removeEventListener('scroll', onMove, true)
    }
  }, [open])

  if (actions.length === 0) return null

  const onKeyDown = (e: KeyboardEvent) => {
    e.stopPropagation() // the row's keys (Enter opens the rule, arrows move the row) stay out
    if (!open) return
    const items = [...(menu.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])]
    const at = items.indexOf(document.activeElement as HTMLElement)
    const go = (i: number) => {
      e.preventDefault()
      items[(i + items.length) % items.length]?.focus()
    }
    switch (e.key) {
      case 'ArrowDown': return go(at + 1)
      case 'ArrowUp': return go(at - 1)
      case 'Home': return go(0)
      case 'End': return go(items.length - 1)
      case 'Escape':
        e.preventDefault()
        return close(true)
      case 'Tab':
        return close(false)
    }
  }

  return (
    <div className="row-menu" onKeyDown={onKeyDown} onClick={(e) => e.stopPropagation()}>
      <button
        ref={button}
        type="button"
        className="row-menu-btn"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={label}
        title="Actions"
        onClick={() => setOpen((o) => !o)}
      >
        <MoreIcon />
      </button>
      {open && createPortal(
        <div ref={menu} className="row-menu-list" role="menu" aria-label={label} style={place}>
          {actions.map((a) => (
            <button
              key={a.label}
              type="button"
              role="menuitem"
              className={a.danger ? 'danger-item' : undefined}
              onClick={() => {
                close(false)
                a.onSelect()
              }}
            >
              {a.label}
            </button>
          ))}
        </div>,
        document.body,
      )}
    </div>
  )
}
