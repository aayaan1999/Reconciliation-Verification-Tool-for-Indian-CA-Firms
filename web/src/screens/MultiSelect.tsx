import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import '../styles/multiselect.css'

export interface MultiOption {
  value: string
  label: string
  short?: string // used in the button when several are chosen, such as a code in place of "R1: Same figure…"
}

// MultiSelect is a filter that takes several values: a button showing the choice, and a list of
// check boxes in a popover. Nothing chosen means no filter. The caller decides what "any of" means.
export function MultiSelect({ label, allText = 'All', options, value, onChange, inline = false, disabled = false, showValues = false, captionClass = 'ms-caption' }: {
  label: string
  allText?: string // shown when nothing is chosen
  options: MultiOption[]
  value: string[]
  onChange: (next: string[]) => void
  inline?: boolean // the list opens in the page flow instead of floating
  disabled?: boolean // the choice is shown but cannot change
  showValues?: boolean // the button names the chosen values (up to three) instead of "2 selected"
  captionClass?: string // the look of the caption, so a form can use its own label style
}) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const button = useRef<HTMLButtonElement>(null)
  const popover = useRef<HTMLDivElement>(null)
  const [up, setUp] = useState(false)
  const [room, setRoom] = useState<number | null>(null)
  const id = useId()

  // Open below the button; flip above it when there is more room there; the list scrolls inside
  // whatever room is left, so the panel never leaves the screen.
  useLayoutEffect(() => {
    if (!open || inline || !button.current || !popover.current) return
    const b = button.current.getBoundingClientRect()
    // The visible area is the screen cut down by every scrolling container around the button (a dialog's body).
    let lo = 0
    let hi = window.innerHeight
    for (let el = button.current.parentElement; el && el !== document.body; el = el.parentElement) {
      const overflow = getComputedStyle(el).overflowY
      if (overflow === 'auto' || overflow === 'scroll' || overflow === 'hidden') {
        const r = el.getBoundingClientRect()
        lo = Math.max(lo, r.top)
        hi = Math.min(hi, r.bottom)
      }
    }
    const below = hi - b.bottom - 12
    const above = b.top - lo - 12
    const list = popover.current.querySelector('.ms-list')
    const chrome = popover.current.offsetHeight - (list?.clientHeight ?? 0)
    const want = (list?.scrollHeight ?? 0) + chrome
    const flip = want > below && above > below
    setUp(flip)
    setRoom(Math.max(48, (flip ? above : below) - chrome))
  }, [open, inline, options.length])

  useEffect(() => {
    if (!open) return
    const away = (e: MouseEvent) => {
      if (root.current && !root.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', away)
    return () => document.removeEventListener('mousedown', away)
  }, [open])

  const close = () => {
    setOpen(false)
    button.current?.focus()
  }
  const toggle = (v: string) => onChange(value.includes(v) ? value.filter((x) => x !== v) : [...value, v])
  const first = options.find((o) => o.value === value[0])
  const labelOf = (v: string) => {
    const o = options.find((x) => x.value === v)
    return o?.short ?? o?.label ?? v
  }
  const summary =
    value.length === 0 ? allText
    : value.length === 1 ? (first?.label ?? value[0])
    : showValues && value.length <= 3 ? value.map(labelOf).join(', ')
    : `${value.length} selected`

  return (
    <div
      className={inline ? 'ms ms-inline' : 'ms'}
      ref={root}
      onKeyDown={(e) => {
        if (e.key === 'Escape' && open) {
          e.stopPropagation()
          close()
        }
      }}
    >
      <span className={captionClass} id={`${id}-cap`}>{label}</span>
      <button
        ref={button}
        type="button"
        className="filter-select-premium ms-button"
        aria-haspopup="true"
        aria-expanded={open && !disabled}
        aria-controls={`${id}-list`}
        aria-label={`${label}: ${summary}`}
        disabled={disabled}
        onClick={() => setOpen(!open)}
      >
        <span className="ms-summary">{summary}</span>
        <span className="ms-caret" aria-hidden="true">▾</span>
      </button>
      {open && !disabled && (
        <div ref={popover} className={up ? 'ms-popover ms-up' : 'ms-popover'} style={room === null ? undefined : ({ '--ms-room': `${room}px` } as React.CSSProperties)} id={`${id}-list`} role="group" aria-label={label}>
          <div className="ms-actions">
            <button type="button" className="ghost" onClick={() => onChange(options.map((o) => o.value))}>Select all</button>
            <button type="button" className="ghost" onClick={() => onChange([])} disabled={value.length === 0}>Clear</button>
          </div>
          <ul className="ms-list">
            {options.map((o) => (
              <li key={o.value}>
                <label>
                  <input type="checkbox" checked={value.includes(o.value)} onChange={() => toggle(o.value)} />
                  <span>{o.label}</span>
                </label>
              </li>
            ))}
            {options.length === 0 && <li className="muted ms-none">Nothing to choose from.</li>}
          </ul>
        </div>
      )}
    </div>
  )
}
