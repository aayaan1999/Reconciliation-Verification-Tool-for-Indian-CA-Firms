import '../styles/palette.css'
import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { SearchIcon } from '../icons.tsx'

export interface PaletteClient {
  id: string
  name: string
  pan_last4: string
}

export interface PaletteRule {
  id: string
  title: string
}

// A PaletteAction is something the shell can already do (go to a screen, switch the theme,
// open the shortcuts sheet, sign out). current marks the theme in use.
export interface PaletteAction {
  id: string
  text: string
  hint?: string
  current?: boolean
  run: () => void
}

interface Option {
  key: string
  text: string
  meta: string
  mono?: boolean
  run: () => void
}

// Each group shows at most this many matches, so a large season stays one short list.
const perGroup = 8

// CommandPalette is the Ctrl K search (Recon Screen Document v2 §3, §12.1): a client by name or
// PAN ending, a rule by ID or name, or an action. Focus stays in the search box; the arrow keys
// choose, Enter runs the choice, Esc closes. Focus goes back where it was when it closes.
export function CommandPalette({ clients, rules, actions, onOpenClient, onOpenRule, onClose }: {
  clients: PaletteClient[]
  rules: PaletteRule[]
  actions: PaletteAction[]
  onOpenClient: (id: string) => void
  onOpenRule: (id: string) => void
  onClose: () => void
}) {
  const listId = useId()
  const input = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)

  useEffect(() => {
    const before = document.activeElement as HTMLElement | null
    input.current?.focus()
    return () => before?.focus?.()
  }, [])

  const groups = useMemo(() => {
    const q = query.trim().toLowerCase()
    const has = (text: string) => text.toLowerCase().includes(q)
    const choose = (run: () => void) => () => {
      onClose()
      run()
    }
    const clientOptions: Option[] = q === '' ? [] : clients
      .filter((c) => has(c.name) || has(c.pan_last4))
      .slice(0, perGroup)
      .map((c) => ({ key: `client-${c.id}`, text: c.name, meta: `PAN ending ${c.pan_last4}`, run: choose(() => onOpenClient(c.id)) }))
    const ruleOptions: Option[] = q === '' ? [] : rules
      .filter((r) => has(r.id) || has(r.title))
      .slice(0, perGroup)
      .map((r) => ({ key: `rule-${r.id}`, text: r.id, meta: r.title, mono: true, run: choose(() => onOpenRule(r.id)) }))
    const actionOptions: Option[] = actions
      .filter((a) => q === '' || has(a.text))
      .map((a) => ({ key: `action-${a.id}`, text: a.text, meta: a.current ? 'In use' : (a.hint ?? ''), run: choose(a.run) }))
    return [
      { title: 'Clients', options: clientOptions },
      { title: 'Rules', options: ruleOptions },
      { title: 'Actions', options: actionOptions },
    ].filter((g) => g.options.length > 0)
  }, [query, clients, rules, actions, onOpenClient, onOpenRule, onClose])

  const flat = groups.flatMap((g) => g.options)
  const at = flat.length === 0 ? -1 : Math.min(active, flat.length - 1)
  const optionId = (i: number) => `${listId}-${i}`

  useEffect(() => {
    if (at >= 0) document.getElementById(optionId(at))?.scrollIntoView?.({ block: 'nearest' })
  })

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      onClose()
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      if (flat.length > 0) setActive((at + (e.key === 'ArrowDown' ? 1 : flat.length - 1)) % flat.length)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      flat[at]?.run()
    } else if (e.key === 'Tab') {
      // The search box is the only control: focus stays in the dialog.
      e.preventDefault()
    }
  }

  let index = 0
  return (
    <div className="palette-backdrop" onClick={onClose}>
      <div className="palette" role="dialog" aria-modal="true" aria-label="Search or jump" onClick={(e) => e.stopPropagation()}>
        <div className="palette-input">
          <SearchIcon />
          <input
            ref={input}
            type="text"
            role="combobox"
            aria-label="Search clients, rules and actions"
            aria-expanded={flat.length > 0}
            aria-controls={listId}
            aria-autocomplete="list"
            aria-activedescendant={at >= 0 ? optionId(at) : undefined}
            placeholder="Client name or PAN ending, rule ID or name, or an action"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              setActive(0)
            }}
            onKeyDown={onKeyDown}
            autoComplete="off"
            spellCheck={false}
          />
        </div>
        {flat.length === 0 ? (
          <p className="palette-empty" role="status">Nothing matches “{query.trim()}”.</p>
        ) : (
          <div id={listId} className="palette-list" role="listbox" aria-label="Results">
            {groups.map((g) => (
              <ul key={g.title} className="palette-group" role="group" aria-label={g.title}>
                <li className="palette-group-head" role="presentation">{g.title}</li>
                {g.options.map((o) => {
                  const i = index++
                  return (
                    <li
                      key={o.key}
                      id={optionId(i)}
                      role="option"
                      aria-selected={i === at}
                      className="palette-option"
                      onMouseMove={() => setActive(i)}
                      onClick={o.run}
                    >
                      <span className={o.mono ? 'palette-text mono' : 'palette-text'}>{o.text}</span>
                      {o.meta && <span className="palette-meta">{o.meta}</span>}
                    </li>
                  )
                })}
              </ul>
            ))}
          </div>
        )}
        <p className="palette-foot" aria-hidden="true">
          <span><kbd>↑</kbd> <kbd>↓</kbd> choose</span>
          <span><kbd>Enter</kbd> open</span>
          <span><kbd>Esc</kbd> close</span>
        </p>
      </div>
    </div>
  )
}
