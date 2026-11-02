import { useEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type ReactNode } from 'react'
import {
  AutoIcon, BoardIcon, BookIcon, ChevronIcon, KeyboardIcon, LedgerIcon, ListIcon, MoonIcon, SearchIcon, SignOutIcon, SunIcon, UsersIcon,
} from '../icons.tsx'
import '../styles/palette.css'
import { initials } from '../format.ts'

export type Theme = 'system' | 'light' | 'dark'
export type RailScreen = 'season' | 'worklist' | 'rules' | 'admin'

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
  findings: number | null // findings across the season's clients; null when no client has a run
  toClassify: number | null // Worklist items not yet classified; null until the list is known
  showWorklist?: boolean // false hides the Worklist (features.ts); shown when not given
  showAdmin: boolean
  live: boolean
  user: { name: string; role: string } | null
  theme: Theme
  onTheme: (t: Theme) => void
  onShortcuts: () => void
  onSearch?: () => void // opens the Ctrl K search
  onSignOut?: () => void
}

// The navigation rail (Recon Screen Document v2 §3): the same on every screen, with the tax
// year at the top, a count of what waits on each screen, the engine status and a user menu
// holding theme, shortcuts and sign-out. It collapses to icons with the round button or [,
// and the choice is kept in this browser only.
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

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement | null
      if (t && ['INPUT', 'SELECT', 'TEXTAREA'].includes(t.tagName)) return
      if (e.key === '[' && !e.ctrlKey && !e.metaKey && !e.altKey) setCollapsed((c) => !c)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  // The menu takes focus when it opens, closes on a click outside it, and Esc closes only
  // the menu (not the screen behind it), returning focus to the button.
  useEffect(() => {
    if (!menu) return
    menuRef.current?.querySelector<HTMLElement>('[role^="menuitem"]')?.focus()
    const onDown = (e: MouseEvent) => {
      if (!menuRef.current?.contains(e.target as Node) && !menuButton.current?.contains(e.target as Node)) setMenu(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [menu])

  const closeMenu = () => {
    setMenu(false)
    menuButton.current?.focus()
  }

  const onMenuKey = (e: ReactKeyboardEvent) => {
    const items = [...(menuRef.current?.querySelectorAll<HTMLElement>('[role^="menuitem"]') ?? [])]
    const i = items.indexOf(document.activeElement as HTMLElement)
    if (e.key === 'Escape') {
      e.stopPropagation()
      closeMenu()
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      const n = items.length
      items[(i + (e.key === 'ArrowDown' ? 1 : n - 1)) % n]?.focus()
    }
  }

  const item = (screen: RailScreen, key: string, text: string, icon: ReactNode, count?: number | null, countText?: string) => (
    <button
      type="button"
      aria-current={p.current === screen}
      aria-label={count != null && count > 0 ? `${text}, ${count} ${countText}` : text}
      onClick={() => p.onNav(screen)}
      title={collapsed ? `${text} (${key})` : undefined}
    >
      <span className="rail-icon">{icon}</span>
      <span className="rail-text">{text}</span>
      {count != null && count > 0 && <span className="rail-count" title={`${count} ${countText}`}>{count}</span>}
      <kbd>{key}</kbd>
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
        <span className="rail-year-value">{p.taxYear}</span>
      </div>

      {p.onSearch && (
        <button type="button" className="rail-search" onClick={p.onSearch} aria-keyshortcuts="Control+K"
          aria-label="Search or jump" title={collapsed ? 'Search or jump (Ctrl K)' : undefined}>
          <span className="rail-icon"><SearchIcon /></span>
          <span className="rail-text">Search or jump</span>
          <kbd>Ctrl K</kbd>
        </button>
      )}

      <nav aria-label="Screens">
        {item('season', '1', 'Season board', <BoardIcon />, p.findings, 'findings')}
        {p.showWorklist !== false && item('worklist', '2', 'Worklist', <ListIcon />, p.toClassify, 'to classify')}
        {item('rules', '3', 'Rules', <BookIcon />)}
        {p.showAdmin && item('admin', '4', 'Admin', <UsersIcon />)}
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
            <div ref={menuRef} className="rail-menu" role="menu" aria-label="User menu" onKeyDown={onMenuKey}>
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
              <hr />
              <button type="button" role="menuitem" onClick={() => { setMenu(false); p.onShortcuts() }}>
                <KeyboardIcon /> Keyboard shortcuts <kbd>?</kbd>
              </button>
              {p.onSignOut && (
                <button type="button" role="menuitem" className="danger-item" onClick={() => { setMenu(false); p.onSignOut?.() }}>
                  <SignOutIcon /> Sign out
                </button>
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
          title={`${collapsed ? 'Expand' : 'Collapse'} ([)`}
        >
          <ChevronIcon dir={collapsed ? 'right' : 'left'} />
        </button>
      </div>
    </header>
  )
}
