import { useMemo, useState } from 'react'
import { addClient } from '../api.ts'
import { emptyDetails } from '../clientLists.ts'
import { initials } from '../format.ts'
import { SearchIcon, SortIcon } from '../icons.tsx'
import { ClientForm } from './ClientForm.tsx'
import type { ServerClient } from '../types.ts'
import { usePager } from '../pager.ts'
import { PageBar } from './PageBar.tsx'
import { useCan } from '../permissions.ts'

// A BoardRow is one client as the Season board shows it.
export interface BoardRow {
  id: string
  name: string
  pan_last4: string
  last_run_on: string
  files: number | null // null: never run
  fail: number | null
  cannot: number | null
  disabled?: boolean // a disabled client (SPEC-004 part 10), listed only when asked
}

interface Props {
  taxYear: string
  rows: BoardRow[]
  live: boolean // a server is running, so clients can be added
  taxYears: string[]
  onOpen: (clientId: string) => void
  onAdded: (client: ServerClient) => void
  disabledRows?: BoardRow[] // disabled clients: left off the board and its counts unless shown (SPEC-004 part 10)
}

type SortKey = 'name' | 'last_run_on' | 'files' | 'fail' | 'cannot' | 'status'
type Sort = { key: SortKey; dir: 'asc' | 'desc' } | null

// statusRank orders clients by how much is left: findings first, then never run, then clear.
const statusRank = (c: BoardRow) => (c.fail === null ? 1 : c.fail > 0 ? 0 : 2)

// The KPI tiles double as filters of the table (Recon Screen Document v2 Â§5.3).
type Filter = 'all' | 'findings' | 'clear' | 'cannot' | 'none'
const filters: Record<Filter, (c: BoardRow) => boolean> = {
  all: () => true,
  findings: (c) => (c.fail ?? 0) > 0,
  clear: (c) => c.fail === 0,
  cannot: (c) => (c.cannot ?? 0) > 0,
  none: (c) => c.fail === null,
}

function compare(a: BoardRow, b: BoardRow, key: SortKey): number {
  switch (key) {
    case 'name':
      return a.name.localeCompare(b.name)
    case 'last_run_on':
      return (a.last_run_on || '').localeCompare(b.last_run_on || '')
    case 'status':
      return statusRank(a) - statusRank(b)
    default: {
      // A client never run sorts after every number, whichever the direction.
      const x = a[key] ?? -1
      const y = b[key] ?? -1
      return x - y
    }
  }
}

// The Season board: every client for the tax year, with the counts from its latest run.
// Clicking a client opens it; the search box and the tiles filter the list.
// Column headings sort; a third click returns to the original order.
export function SeasonBoard({ taxYear, rows, live, taxYears, onOpen, onAdded, disabledRows = [] }: Props) {
  const canRun = useCan()('run') // adding a client (SPEC-012)
  const [query, setQuery] = useState('')
  const [sort, setSort] = useState<Sort>(null)
  const [filter, setFilter] = useState<Filter>('all')
  const [showDisabled, setShowDisabled] = useState(false)
  // The table lists the clients in use, and the disabled ones too when asked; the tiles count only those in use.
  const listed = useMemo(() => (showDisabled ? [...rows, ...disabledRows.map((r) => ({ ...r, disabled: true }))] : rows), [rows, disabledRows, showDisabled])
  // With no column chosen, clients needing work come first; within a status the list keeps
  // its own order (Array.prototype.sort is stable).
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase()
    const found = listed.filter(
      (r) => filters[filter](r) && (q === '' || r.name.toLowerCase().includes(q) || r.pan_last4.toLowerCase().includes(q)),
    )
    if (!sort) return [...found].sort((a, b) => statusRank(a) - statusRank(b))
    const sorted = [...found].sort((a, b) => compare(a, b, sort.key) || a.name.localeCompare(b.name))
    return sort.dir === 'asc' ? sorted : sorted.reverse()
  }, [listed, query, sort, filter])

  const pager = usePager(visible, 'season', `${query}|${sort?.key ?? ''}|${sort?.dir ?? ''}|${filter}|${showDisabled}`)
  const [adding, setAdding] = useState(false)

  const count = (f: Filter) => rows.filter(filters[f]).length
  const ran = rows.length - count('none')
  const totalFindings = rows.reduce((n, r) => n + (r.fail ?? 0), 0)
  const totalCannot = rows.reduce((n, r) => n + (r.cannot ?? 0), 0)
  const tiles: { f: Filter; label: string; value: number; sub?: string; tone?: string }[] = [
    { f: 'all', label: 'Clients', value: rows.length, sub: `${ran} checked` },
    { f: 'findings', label: 'With findings to review', value: count('findings'), sub: `${totalFindings} findings`, tone: 'fail' },
    { f: 'clear', label: 'No findings', value: count('clear'), tone: 'pass' },
    { f: 'cannot', label: 'Cannot check', value: count('cannot'), sub: `${totalCannot} checks`, tone: 'warn' },
    { f: 'none', label: 'No run yet', value: count('none') },
  ]
  // The share of each status in the season, as widths of one bar.
  const share = (n: number) => `${rows.length === 0 ? 0 : (n * 100) / rows.length}%`

  const sortBy = (key: SortKey) =>
    setSort((s) => (s?.key !== key ? { key, dir: 'asc' } : s.dir === 'asc' ? { key, dir: 'desc' } : null))
  const head = (key: SortKey, text: string, num = false) => {
    const dir = sort?.key === key ? sort.dir : null
    return (
      <th className={num ? 'sortable num' : 'sortable'} aria-sort={dir === 'asc' ? 'ascending' : dir === 'desc' ? 'descending' : 'none'}>
        <button type="button" onClick={() => sortBy(key)} title={`Sort by ${text.toLowerCase()}`}>
          {text} <SortIcon dir={dir} />
        </button>
      </th>
    )
  }

  const addButton = (
    <button type="button" className="action" onClick={() => setAdding(true)}>
      + Add client
    </button>
  )

  return (
    <section aria-labelledby="season-title">
      <div className="board-head">
        <div>
          <p className="eyebrow">Filing season</p>
          <h1 id="season-title">Season board, tax year {taxYear}</h1>
        </div>
        {live && !adding && canRun && rows.length > 0 && addButton}
      </div>

      {rows.length > 0 && (
        <div className="board-summary">
          <div className="kpi-strip" role="group" aria-label="Season summary: choose a tile to filter the clients">
            {tiles.map((t) => (
              <button
                key={t.f}
                type="button"
                className={t.tone ? `kpi ${t.tone}` : 'kpi'}
                aria-pressed={filter === t.f}
                onClick={() => setFilter((cur) => (cur === t.f && t.f !== 'all' ? 'all' : t.f))}
              >
                <span className="kpi-label">{t.label}</span>
                <span className="kpi-value">{t.value}</span>
                {t.sub && <span className="kpi-sub">{t.sub}</span>}
              </button>
            ))}
          </div>
          <div className="season-progress">
            <div className="season-progress-text">
              <span>Season progress</span>
              <span>{ran} of {rows.length} clients checked</span>
            </div>
            <div className="stack-bar" aria-hidden="true">
              <span className="fail" style={{ width: share(count('findings')) }} />
              <span className="pass" style={{ width: share(count('clear')) }} />
            </div>
            <ul className="stack-legend" aria-label="Clients by status">
              <li><span className="dot fail" />With findings {count('findings')}</li>
              <li><span className="dot pass" />No findings {count('clear')}</li>
              <li><span className="dot" />No run yet {count('none')}</li>
            </ul>
          </div>
        </div>
      )}

      {!live && (
        <p className="notice" role="note">
          Sample clients: start recon-server to add your own clients.
        </p>
      )}
      {adding && (
        <ClientForm
          mode="add"
          initial={emptyDetails()}
          taxYears={taxYears.length > 0 ? taxYears : [taxYear]}
          onCancel={() => setAdding(false)}
          onSave={async (details, ty) => {
            const c = await addClient(details, ty)
            setAdding(false)
            onAdded(c)
          }}
        />
      )}

      {rows.length === 0 && disabledRows.length === 0 && (
        <div className="onboard">
          <h2>No clients yet. A season takes three steps.</h2>
          <ol>
            <li><strong>Add the client</strong><span>Name, PAN and GST registrations.</span></li>
            <li><strong>Upload and run</strong><span>Drop the books, tax suite and portal files; every rule runs.</span></li>
            <li><strong>Decide each finding</strong><span>Fix, Not an issue or Ask client, with both figures in front of you.</span></li>
          </ol>
          {live && !adding && canRun && addButton}
        </div>
      )}

      {(rows.length > 0 || disabledRows.length > 0) && (
        <div className="board-list">
          <div className="board-controls">
            <div className="search-box">
              <SearchIcon />
              <input
                type="search"
                aria-label="Search clients"
                placeholder="Client name or PAN ending"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
            </div>
            <span className="result-count" aria-live="polite">
              {visible.length !== listed.length ? `${visible.length} of ${listed.length} clients` : `${listed.length} clients`}
              {filter !== 'all' && (
                <>
                  {' '}Â·{' '}
                  <button type="button" className="ghost" onClick={() => setFilter('all')}>
                    Show all
                  </button>
                </>
              )}
              {sort && (
                <>
                  {' '}Â·{' '}
                  <button type="button" className="ghost" onClick={() => setSort(null)}>
                    Clients needing work first
                  </button>
                </>
              )}
            </span>
            {disabledRows.length > 0 && (
              <label className="board-toggle">
                <input type="checkbox" checked={showDisabled} onChange={(e) => setShowDisabled(e.target.checked)} />
                Show disabled clients ({disabledRows.length})
              </label>
            )}
          </div>

          {visible.length === 0 && (
            <p className="empty">
              {query.trim() !== '' ? `No client matches "${query}". Clear the search box to see every client.` : 'No client has this status.'}
            </p>
          )}
          <div className="table-wrap">
            <table className="grid stack table-premium" aria-label="Clients">
              <thead>
                <tr>
                  {head('name', 'Client')}
                  <th>PAN ending</th>
                  {head('last_run_on', 'Last run')}
                  {head('files', 'Files read', true)}
                  {head('fail', 'Findings', true)}
                  {head('cannot', 'Cannot check', true)}
                  {head('status', 'Status')}
                </tr>
              </thead>
              <tbody>
                {pager.rows.map((c) => (
                  <tr key={c.id} className={c.disabled ? 'row-disabled' : undefined} onClick={() => onOpen(c.id)}>
                    <td className="client-name">
                      <span className="client-cell">
                        <span className="client-avatar" aria-hidden="true">{initials(c.name)}</span>
                        {c.name}
                      </span>
                    </td>
                    <td data-label="PAN ending" className="mono">{c.pan_last4}</td>
                    <td data-label="Last run">{c.last_run_on || '-'}</td>
                    <td data-label="Files read" className="num">{c.files ?? '-'}</td>
                    <td data-label="Findings" className="num">
                      {c.fail === null ? '-' : <span className={c.fail > 0 ? 'count fail' : 'count'}>{c.fail}</span>}
                    </td>
                    <td data-label="Cannot check" className="num">
                      {c.cannot === null ? '-' : <span className={c.cannot > 0 ? 'count warn' : 'count'}>{c.cannot}</span>}
                    </td>
                    <td data-label="Status">
                      {c.disabled ? (
                        <span className="tag muted">Disabled</span>
                      ) : c.fail === null ? (
                        <span className="tag muted">No run yet</span>
                      ) : c.fail > 0 ? (
                        <span className="tag fail">Findings to review</span>
                      ) : (
                        <span className="tag pass">No findings</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <PageBar p={pager} noun="clients" />
        </div>
      )}
    </section>
  )
}
