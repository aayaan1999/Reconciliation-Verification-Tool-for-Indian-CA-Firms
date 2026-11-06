import { useEffect, useRef, useState, type ReactNode } from 'react'
import {
  AutoIcon, BoardIcon, BookIcon, ChevronIcon, FormatsIcon, KeyboardIcon, LedgerIcon, ListIcon, MoonIcon, SignOutIcon, SunIcon, UsersIcon,
} from '../icons.tsx'
import { initials } from '../format.ts'

export type Theme = 'system' | 'light' | 'dark'
export type RailScreen = 'season' | 'worklist' | 'rules' | 'formats' | 'admin'

const collapsedKey = 'recon.rail.collapsed'
function storedCollapsed(): boolean {
  try {
    return localStorage.getItem(collapsedKey) === '1'
  } catch {
    return false
  }
}

const themes: { value: Theme; text: string; icon: ReactNode }[] = [
  { value: 'light', text: 'Light', icon: <SunIcon /> },
  { value: 'dark', text: 'Dark', icon: <MoonIcon /> },
  { value: 'system', text: 'Match the computer', icon: <AutoIcon /> },
]

interface Props {
  current: RailScreen
  onNav: (screen: RailScreen) => void
  taxYear: string
  taxYears?: string[] // the years the Season board can show; with onTaxYear, the year becomes a choice
  onTaxYear?: (year: string) => void
  findings: number | null // findings across the season's clients; null when no client has a run
  toClassify: number | null // Worklist items not yet classified; null until the list is known
  showWorklist?: boolean // false hides the Worklist (features.ts); shown when not given
  showFormats?: boolean // the Formats screen, for a user who may manage formats (SPEC-016)
  showAdmin: boolean
  live: boolean
  user: { name: string; role: string } | null
  theme: Theme
  onTheme: (t: Theme) => void
  onSignOut?: () => void
}

// The navigation rail (Recon Screen Document v2 §3): the same on every screen, with the tax
// year at the top, a count of what waits on each screen, the engine status and a user menu
// holding theme and sign-out. It collapses to icons with the round button, and the choice is
// kept in this browser only.
export function Rail(p: Props) {
  const [collapsed, setCollapsed] = useState(storedCollapsed)
  const [menu, setMenu] = useState(false)
  const menuButton = useRef<HTMLButtonElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    try {
      localStorage.setItem(collapsedKey, collapsed ? '1' : '0')
    } catch {
      // Not kept in this browser: the rail opens expanded next time.
    }
  }, [collapsed])

  // The menu takes focus when it opens and closes on a click outside it or on the button.
  useEffect(() => {
    if (!menu) return
    menuRef.current?.querySelector<HTMLElement>('[role^="menuitem"]')?.focus()
    const onDown = (e: MouseEvent) => {
      if (!menuRef.current?.contains(e.target as Node) && !menuButton.current?.contains(e.target as Node)) setMenu(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [menu])

  const item = (screen: RailScreen, text: string, icon: ReactNode, count?: number | null, countText?: string) => (
    <button
      type="button"
      aria-current={p.current === screen}
      aria-label={count != null && count > 0 ? `${text}, ${count} ${countText}` : text}
      onClick={() => p.onNav(screen)}
      title={collapsed ? text : undefined}
    >
      <span className="rail-icon">{icon}</span>
      <span className="rail-text">{text}</span>
      {count != null && count > 0 && <span className="rail-count" title={`${count} ${countText}`}>{count}</span>}
    </button>
  )

  return (
    <header className={collapsed ? 'rail collapsed' : 'rail'}>
      <div className="rail-brand">
        <LedgerIcon />
        <p className="rail-name">Recon</p>
      </div>

      <div className="rail-year" title="The tax year of the Season board, the Worklist and new clients">
        <span className="rail-year-label">Tax year</span>
        {p.taxYears && p.taxYears.length > 1 && p.onTaxYear ? (
          <select className="rail-year-value rail-year-select" aria-label="Tax year of the Season board" value={p.taxYear}
            onChange={(e) => p.onTaxYear?.(e.target.value)}>
            {p.taxYears.map((y) => (
              <option key={y} value={y}>{y}</option>
            ))}
          </select>
        ) : (
          <span className="rail-year-value">{p.taxYear}</span>
        )}
      </div>

      <nav aria-label="Screens">
        {item('season', 'Season board', <BoardIcon />, p.findings, 'findings')}
        {p.showWorklist !== false && item('worklist', 'Worklist', <ListIcon />, p.toClassify, 'to classify')}
        {item('rules', 'Rules', <BookIcon />)}
        {p.showFormats && item('formats', 'Formats', <FormatsIcon />)}
        {p.showAdmin && item('admin', 'Admin', <UsersIcon />)}
      </nav>

      <div className="rail-end">
        <span className={p.live ? 'rail-status live' : 'rail-status'} role="note">
          <span className="rail-dot" aria-hidden="true" />
          <span className="rail-text">{p.live ? 'Connected to the engine' : 'Sample data'}</span>
        </span>

        <div className="rail-user">
          <button
            ref={menuButton}
            type="button"
            className="rail-user-btn"
            aria-haspopup="menu"
            aria-expanded={menu}
            aria-label={p.user ? `${p.user.name}, ${p.user.role}: menu` : 'Settings menu'}
            onClick={() => setMenu((m) => !m)}
          >
            <span className="rail-avatar" aria-hidden="true">{p.user ? initials(p.user.name) : <KeyboardIcon />}</span>
            <span className="rail-text rail-who">
              {p.user ? p.user.name : 'Settings'}
              {p.user && <span>{p.user.role}</span>}
            </span>
            <span className="rail-text"><ChevronIcon dir="up" /></span>
          </button>
          {menu && (
            <div ref={menuRef} className="rail-menu" role="menu" aria-label="User menu">
              <p className="rail-menu-head" aria-hidden="true">Theme</p>
              {themes.map((t) => (
                <button
                  key={t.value}
                  type="button"
                  role="menuitemradio"
                  aria-checked={p.theme === t.value}
                  onClick={() => p.onTheme(t.value)}
                >
                  {t.icon} {t.text}
                </button>
              ))}
              {p.onSignOut && (
                <>
                  <hr />
                  <button type="button" role="menuitem" className="danger-item" onClick={() => { setMenu(false); p.onSignOut?.() }}>
                    <SignOutIcon /> Sign out
                  </button>
                </>
              )}
            </div>
          )}
        </div>

        <button
          type="button"
          className="rail-collapse"
          onClick={() => setCollapsed((c) => !c)}
          aria-label={collapsed ? 'Expand the menu' : 'Collapse the menu'}
          aria-expanded={!collapsed}
          title={collapsed ? 'Expand' : 'Collapse'}
        >
          <ChevronIcon dir={collapsed ? 'right' : 'left'} />
        </button>
      </div>
    </header>
  )
}
