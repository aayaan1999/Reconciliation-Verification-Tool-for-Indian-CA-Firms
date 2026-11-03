import { useEffect, useId, useRef, useState } from 'react'
import '../styles/multiselect.css'

export interface MultiOption {
  value: string
  label: string
}

// MultiSelect is a filter that takes several values: a button showing the choice, and a list of
// check boxes in a popover. Nothing chosen means no filter. The caller decides what "any of" means.
export function MultiSelect({ label, allText = 'All', options, value, onChange }: {
  label: string
  allText?: string
  options: MultiOption[]
  value: string[]
  onChange: (next: string[]) => void
}) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const button = useRef<HTMLButtonElement>(null)
  const id = useId()

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
  const summary = value.length === 0 ? allText : value.length === 1 ? (first?.label ?? value[0]) : `${value.length} selected`

  return (
    <div
      className="ms"
      ref={root}
      onKeyDown={(e) => {
        if (e.key === 'Escape' && open) {
          e.stopPropagation()
          close()
        }
      }}
    >
      <span className="ms-caption" id={`${id}-cap`}>{label}</span>
      <button
        ref={button}
        type="button"
        className="filter-select-premium ms-button"
        aria-haspopup="true"
        aria-expanded={open}
        aria-controls={`${id}-list`}
        aria-label={`${label}: ${summary}`}
        onClick={() => setOpen(!open)}
      >
        <span className="ms-summary">{summary}</span>
        <span className="ms-caret" aria-hidden="true">▾</span>
      </button>
      {open && (
        <div className="ms-popover" id={`${id}-list`} role="group" aria-label={label}>
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
