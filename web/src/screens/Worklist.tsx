import '../styles/worklist.css'
import { useEffect, useRef, useState } from 'react'
import { useCan } from '../permissions.ts'
import { formatPaise } from '../format.ts'
import { SearchIcon } from '../icons.tsx'
import type { WorklistItem, WorklistKind } from '../types.ts'
import { usePager } from '../pager.ts'
import { PageBar } from './PageBar.tsx'

export type ClassifyFn = (item: WorklistItem, classification: string) => Promise<void>

interface Props {
  items: WorklistItem[]
  taxYear: string
  live: boolean
  loading?: boolean
  onClassify?: ClassifyFn // live only: keeps the classification in the database
}

const months = (n: number) => (n === 1 ? '1 month' : `${n} months`)

// What changed for an item, beyond the two totals and their months.
function detail(it: WorklistItem): string {
  switch (it.kind) {
    case 'new_head':
      return 'no entry last year'
    case 'vanished':
      return 'no entry this year'
    case 'broken_recurrence':
      return `every month last year; none for ${it.missing_months.join(', ')}`
    case 'step_change':
      return `${it.times} times last year`
  }
}

// The months of a financial year, April first, as "YYYY-MM" for the year ending in March.
function yearMonths(missing: string[]): string[] | null {
  const first = [...missing].sort()[0]
  if (!first) return null
  const [y, m] = first.split('-').map(Number)
  const start = m >= 4 ? y : y - 1
  return Array.from({ length: 12 }, (_, i) => {
    const mm = ((3 + i) % 12) + 1
    return `${mm >= 4 ? start : start + 1}-${String(mm).padStart(2, '0')}`
  })
}

const monthNames = ['April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December', 'January', 'February', 'March']

type MonthState = 'both' | 'this' | 'last' | 'none'
const stateWords: Record<MonthState, string> = {
  both: 'entry both years',
  this: 'entry this year only',
  last: 'entry last year only',
  none: 'no entry',
}

// monthStrip gives each month, April to March, which years had an entry. The comparison
// counts months but does not say which ones, so a year's months are known only when it has
// an entry in all twelve or in none, or for a broken recurrence, which names the missing
// months. Elsewhere it returns null and no strip is drawn.
function monthStrip(it: WorklistItem): MonthState[] | null {
  const whole = (n: number) => (n === 12 ? true : n === 0 ? false : null)
  const last = whole(it.prior_months)
  if (last === null) return null
  let thisYear: boolean[]
  if (it.missing_months.length > 0) {
    const ym = yearMonths(it.missing_months)
    if (!ym || !it.missing_months.every((m) => ym.includes(m)) || 12 - it.missing_months.length !== it.current_months) return null
    thisYear = ym.map((m) => !it.missing_months.includes(m))
  } else {
    const t = whole(it.current_months)
    if (t === null) return null
    thisYear = Array.from({ length: 12 }, () => t)
  }
  return thisYear.map((t) => (t && last ? 'both' : t ? 'this' : last ? 'last' : 'none'))
}

function stripLabel(strip: MonthState[]): string {
  const parts = (['both', 'this', 'last', 'none'] as const).flatMap((s) => {
    const names = monthNames.filter((_, i) => strip[i] === s)
    if (names.length === 0) return []
    const which = names.length === 12 ? 'every month' : names.length > 3 ? months(names.length) : names.join(', ')
    return [`${stateWords[s]}: ${which}`]
  })
  return `April to March, ${parts.join('; ')}`
}

function MonthStrip({ it }: { it: WorklistItem }) {
  const strip = monthStrip(it)
  if (!strip) {
    return (
      <span className="wl-strip-none" title="The ledgers give how many months had an entry, not which ones.">
        Months not listed
      </span>
    )
  }
  return (
    <span className="wl-strip" role="img" aria-label={stripLabel(strip)}>
      {strip.map((s, i) => (
        <i key={i} className={s} title={`${monthNames[i]}: ${stateWords[s]}`} />
      ))}
    </span>
  )
}

// The change types in the order the comparison reports them.
const kinds: WorklistKind[] = ['new_head', 'vanished', 'broken_recurrence', 'step_change']

const onlyOpenKey = 'recon.worklist.onlyOpen'
function storedOnlyOpen(): boolean {
  try {
    return localStorage.getItem(onlyOpenKey) === 'true'
  } catch {
    return false
  }
}

const rowKey = (it: WorklistItem) => `${it.client_id}|${it.key}`

// The Worklist (SPEC-006 part 3): ledger heads that are new, vanished, missing months or
// much changed compared with last year, for staff to classify (clause mapping, GST status,
// TDS section, 43B category). They are not findings, and the tool never classifies them:
// the suggestions offered while typing are only classifications staff already wrote.
// Each classification is a small form, so Enter in its field saves it as the Save button does.
// Ticked items share one classification.
export function Worklist({ items, taxYear, live, loading, onClassify }: Props) {
  const [drafts, setDrafts] = useState<Record<string, string>>({})
  const [saved, setSaved] = useState<Record<string, string>>({}) // sample data: this screen only
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState('')
  const [editing, setEditing] = useState<string | null>(null)
  const canDecide = useCan()('decide') // classifying Worklist items (SPEC-012)
  const [onlyOpen, setOnlyOpenState] = useState(storedOnlyOpen)
  const [kind, setKind] = useState<WorklistKind | 'all'>('all')
  const [query, setQuery] = useState('')
  const [ticked, setTicked] = useState<string[]>([])
  const [bulk, setBulk] = useState('')
  const [bulkError, setBulkError] = useState('')
  // The last classification saved, with what the items had before, for Undo.
  const [last, setLast] = useState<{ items: WorklistItem[]; before: Record<string, string>; text: string } | null>(null)
  const inputs = useRef<Record<string, HTMLInputElement | null>>({})

  const current = (it: WorklistItem) => (live ? (it.classification?.text ?? '') : (saved[rowKey(it)] ?? it.classification?.text ?? ''))
  const q = query.trim().toLowerCase()
  const shown = items.filter(
    (it) =>
      (!onlyOpen || current(it) === '') &&
      (kind === 'all' || it.kind === kind) &&
      (q === '' || [it.ledger, it.client_name, it.group, current(it)].join(' ').toLowerCase().includes(q)),
  )
  const pager = usePager(shown, 'worklist', `${onlyOpen}|${kind}|${q}`)
  const page = pager.rows
  // A classified row shows its text; Edit turns it back into the field.
  useEffect(() => {
    if (editing) inputs.current[editing]?.focus()
  }, [editing])

  const setOnlyOpen = (on: boolean) => {
    setOnlyOpenState(on)
    try {
      localStorage.setItem(onlyOpenKey, String(on))
    } catch {
      // Not kept in this browser: every item is shown next time.
    }
  }

  const open = items.filter((it) => current(it) === '').length
  const done = items.length - open
  // Suggestions are the classifications already written on this list, never a list of the tool's own.
  const suggestions = [...new Set(items.map(current).filter((t) => t !== ''))].sort()

  // keep saves one classification on the given items, in order; it returns the first error.
  const dropDraft = (k: string) =>
    setDrafts((d) => {
      const { [k]: _, ...rest } = d
      return rest
    })
  const keep = async (list: WorklistItem[], text: string): Promise<string> => {
    if (!live || !onClassify) {
      setSaved((s) => ({ ...s, ...Object.fromEntries(list.map((it) => [rowKey(it), text])) }))
      for (const it of list) dropDraft(rowKey(it))
      return ''
    }
    for (const it of list) {
      const k = rowKey(it)
      setBusy(k)
      try {
        await onClassify(it, text)
        dropDraft(k)
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err)
        setErrors((e) => ({ ...e, [k]: msg }))
        setBusy('')
        return msg
      }
    }
    setBusy('')
    return ''
  }

  const save = async (it: WorklistItem) => {
    if (busy !== '') return
    const k = rowKey(it)
    const text = (drafts[k] ?? current(it)).trim()
    if (text === '') {
      setErrors((e) => ({ ...e, [k]: 'Write the classification first.' }))
      return
    }
    setErrors((e) => ({ ...e, [k]: '' }))
    const before = { [k]: current(it) }
    if ((await keep([it], text)) === '') {
      setLast({ items: [it], before, text })
      setEditing(null)
    }
  }

  // Cancel on a classified row being edited drops what was typed and shows the saved text again.
  const leave = (it: WorklistItem) => {
    dropDraft(rowKey(it))
    setEditing(null)
  }

  const tickedItems = items.filter((it) => ticked.includes(rowKey(it)))
  const saveBulk = async () => {
    if (busy !== '') return
    if (bulk.trim() === '') return setBulkError('Write the classification first.')
    setBulkError('')
    const before = Object.fromEntries(tickedItems.map((it) => [rowKey(it), current(it)]))
    const err = await keep(tickedItems, bulk.trim())
    if (err !== '') return setBulkError(err)
    setLast({ items: tickedItems, before, text: bulk.trim() })
    setTicked([])
    setBulk('')
  }

  // Undo puts back what the items had before the last save. The server refuses an empty
  // classification, so there a classification can be replaced but not removed: Undo is
  // offered only when every item had one before.
  const canUndo = last !== null && (!live || last.items.every((it) => last.before[rowKey(it)] !== ''))
  const undo = async () => {
    if (!last) return
    if (!live || !onClassify) {
      setSaved((s) => {
        const next = { ...s }
        for (const it of last.items) {
          const k = rowKey(it)
          if (last.before[k] === '' && !it.classification) delete next[k]
          else next[k] = last.before[k]
        }
        return next
      })
      setLast(null)
      return
    }
    for (const it of last.items) {
      if ((await keep([it], last.before[rowKey(it)])) !== '') return
    }
    setLast(null)
  }

  const tick = (k: string) => setTicked((t) => (t.includes(k) ? t.filter((x) => x !== k) : [...t, k]))
  const kindText = (k: WorklistKind) => items.find((it) => it.kind === k)?.kind_text ?? k
  const share = (n: number) => `${items.length === 0 ? 0 : (n * 100) / items.length}%`

  return (
    <section aria-labelledby="worklist-title" className="worklist">
      <div className="header-premium">
        <div>
          <p className="eyebrow">Tax year {taxYear} · new and changed ledger heads</p>
          <h1 id="worklist-title">Worklist</h1>
        </div>
      </div>
      <p className="muted wl-intro">
        Ledger heads that are new, vanished, missing months or much changed from last year. These are not
        findings: staff write the classification (clause, GST status, TDS section, 43B category); the tool never fills it in.
      </p>
      {!live && (
        <p className="sample" role="note">
          Sample data: the engine's Worklist on Test Pack A's two ledgers. Classifications are not kept until the server is running.
        </p>
      )}
      {loading ? (
        <p className="loading">Loading the Worklist…</p>
      ) : items.length === 0 ? (
        <p className="empty">
          No items for {taxYear}. Upload this year's and last year's expense ledger for a client (type of check "Expense ledgers:
          this year against last year").
        </p>
      ) : (
        <>
          <div className="wl-summary" aria-label="Worklist summary" role="group">
            <div className="wl-progress-head">
              <p className="wl-left">
                <b>{open}</b> left to classify
              </p>
              <p className="result-count">
                {items.length} items, {open} not classified yet.
              </p>
            </div>
            <div
              className="stack-bar"
              role="progressbar"
              aria-label="Items classified"
              aria-valuemin={0}
              aria-valuemax={items.length}
              aria-valuenow={done}
              aria-valuetext={`${done} of ${items.length} classified`}
            >
              <span className="pass" style={{ width: share(done) }} />
            </div>
            <div className="wl-chips" role="group" aria-label="Show items by what changed">
              <button type="button" className="chip wl-chip" aria-pressed={kind === 'all'} onClick={() => setKind('all')}>
                All <span className="wl-chip-count">{items.length}</span>
              </button>
              {kinds
                .filter((k) => items.some((it) => it.kind === k))
                .map((k) => (
                  <button
                    key={k}
                    type="button"
                    className={`chip wl-chip wl-kind-${k}`}
                    aria-pressed={kind === k}
                    onClick={() => setKind((cur) => (cur === k ? 'all' : k))}
                  >
                    {kindText(k)} <span className="wl-chip-count">{items.filter((it) => it.kind === k).length}</span>
                  </button>
                ))}
            </div>
          </div>
          {open === 0 && <p className="empty">Every item is classified. The Worklist is clear for {taxYear}.</p>}
          <div className="wl-controls">
            <div className="search-box">
              <SearchIcon />
              <input
                type="search"
                aria-label="Search the Worklist"
                placeholder="Ledger head, client, group or classification"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
            </div>
            <label className="check">
              <input type="checkbox" checked={onlyOpen} onChange={(e) => setOnlyOpen(e.target.checked)} />
              Only items not classified
            </label>
          </div>
          <datalist id="wl-suggestions">
            {suggestions.map((s) => (
              <option key={s} value={s} />
            ))}
          </datalist>
          {shown.length === 0 && (
            <p className="empty">
              {q !== '' ? `No item matches "${query}". Clear the search box to see every item.` : 'No item of this kind is left to show.'}
            </p>
          )}
          <div className="table-wrap">
            <table className="grid table-premium wl-table" aria-label="Worklist items">
              <thead>
                <tr>
                  <th aria-label="Tick" />
                  <th>Ledger head</th>
                  <th>What changed</th>
                  <th className="num">Last year</th>
                  <th className="num">This year</th>
                  <th>
                    <span aria-hidden="true" className="wl-strip-head">
                      {monthNames.map((m, i) => (
                        <span key={i}>{m[0]}</span>
                      ))}
                    </span>
                    <span className="wl-sr">Months, April to March</span>
                  </th>
                  <th>Classification</th>
                </tr>
              </thead>
              <tbody>
                {page.map((it) => {
                  const k = rowKey(it)
                  const text = current(it)
                  const isDone = text !== ''
                  const showField = !isDone || editing === k
                  return (
                    <tr key={k} className={isDone ? 'done' : undefined} title={it.text}>
                      <td data-label="Tick">
                        <input
                          type="checkbox"
                          aria-label={`Tick ${it.ledger}`}
                          checked={ticked.includes(k)}
                          disabled={!canDecide}
                          onChange={() => tick(k)}
                        />
                      </td>
                      <td data-label="Ledger head">
                        <strong className="wl-ledger">{it.ledger}</strong>
                        <span className="wl-sub">
                          {it.client_name}
                          {it.group && ` · ${it.group}`}
                        </span>
                      </td>
                      <td data-label="What changed">
                        <span className={`tag wl-kind wl-kind-${it.kind}`}>{it.kind_text}</span>
                        <span className="wl-sub">{detail(it)}</span>
                      </td>
                      <td data-label="Last year" className="num">
                        {it.kind === 'new_head' ? '-' : formatPaise(it.prior_total)}
                        <span className="wl-sub">{months(it.prior_months)}</span>
                      </td>
                      <td data-label="This year" className="num">
                        {it.kind === 'vanished' ? '-' : formatPaise(it.current_total)}
                        <span className="wl-sub">{months(it.current_months)}</span>
                      </td>
                      <td data-label="Months, April to March">
                        <MonthStrip it={it} />
                      </td>
                      <td className="wide wl-class" data-label="Classification">
                        {showField ? (
                          <form
                            className="wl-field"
                            aria-label={`Classify ${it.ledger}`}
                            onSubmit={(e) => {
                              e.preventDefault()
                              save(it)
                            }}
                          >
                            <input
                              aria-label={`Classification of ${it.ledger}`}
                              ref={(el) => {
                                inputs.current[k] = el
                              }}
                              list="wl-suggestions"
                              value={drafts[k] ?? text}
                              maxLength={500}
                              disabled={busy === k || !canDecide}
                              placeholder="Clause, GST status, TDS section…"
                              onChange={(e) => setDrafts((d) => ({ ...d, [k]: e.target.value }))}
                            />
                            <button type="submit" className="btn-sm-premium" disabled={busy !== '' || !canDecide}>
                              Save
                            </button>
                            {isDone && (
                              <button type="button" className="ghost" onClick={() => leave(it)} disabled={busy !== ''}>
                                Cancel
                              </button>
                            )}
                          </form>
                        ) : (
                          <div className="wl-saved">
                            <span className="tag pass">Classified</span>
                            <span className="wl-saved-text">{text}</span>
                            <button
                              type="button"
                              className="ghost"
                              aria-label={`Edit the classification of ${it.ledger}`}
                              onClick={() => setEditing(k)}
                              disabled={!canDecide}
                            >
                              Edit
                            </button>
                          </div>
                        )}
                        {isDone && live && it.classification && (
                          <span className="wl-sub">
                            by {it.classification.by}, {it.classification.at.slice(0, 10)}
                          </span>
                        )}
                        {isDone && !live && saved[k] !== undefined && <span className="wl-sub">this screen only</span>}
                        {errors[k] && (
                          <span className="error" role="alert">
                            {errors[k]}
                          </span>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
          <PageBar p={pager} noun="items" />
          {ticked.length > 0 && (
            <form
              className="wl-bulk"
              role="group"
              aria-label="Classify the ticked items"
              onSubmit={(e) => {
                e.preventDefault()
                saveBulk()
              }}
            >
              <strong>{ticked.length} ticked</strong>
              <input
                aria-label="Classification for the ticked items"
                placeholder="Clause, GST status, TDS section…"
                list="wl-suggestions"
                value={bulk}
                maxLength={500}
                readOnly={!canDecide}
                onChange={(e) => setBulk(e.target.value)}
              />
              <button type="submit" className="action" disabled={busy !== '' || !canDecide}>
                Save for {ticked.length} {ticked.length === 1 ? 'item' : 'items'}
              </button>
              <button type="button" className="ghost" onClick={() => setTicked([])}>
                Untick all
              </button>
              {bulkError && (
                <p className="error" role="alert">
                  {bulkError}
                </p>
              )}
            </form>
          )}
        </>
      )}
      {last && (
        <div className={ticked.length > 0 ? 'wl-toast raised' : 'wl-toast'} role="status">
          <span>
            Saved "{last.text}" for {last.items.length === 1 ? last.items[0].ledger : `${last.items.length} items`}.
            {!canUndo && ' Use Edit on the row to change it.'}
          </span>
          {canUndo && (
            <button type="button" className="ghost" onClick={undo} disabled={busy !== '' || !canDecide}>
              Undo
            </button>
          )}
          <button type="button" className="ghost" aria-label="Close the message" onClick={() => setLast(null)}>
            ×
          </button>
        </div>
      )}
    </section>
  )
}
