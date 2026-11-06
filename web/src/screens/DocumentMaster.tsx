import { useCallback, useEffect, useMemo, useState } from 'react'
import { ApiError, deleteDocument, fetchDocument, fetchDocuments, setDocumentActive } from '../api.ts'
import { docCategories, fileKindNames, fileKinds, formatsText, type DocDetail, type DocRow, type DocStatus } from '../docMasterTypes.ts'
import { SearchIcon } from '../icons.tsx'
import { usePager } from '../pager.ts'
import { DocumentForm } from './DocumentForm.tsx'
import { Modal } from './Modal.tsx'
import { PageBar } from './PageBar.tsx'
import '../styles/documents.css'

// The Admin tab's Document Master (SPEC-022): the kinds of document the tool can read, built-in
// and the Partner's own. The Partner adds a document in two steps that can be done on different
// days (the document, then its record structure), switches it on or off, and deletes one nothing
// uses. The engine checks every definition and writes the audit entry; this screen asks and shows
// the answer. A document that is not ready (a Draft) is offered nowhere until it is.

type SortKey = 'name' | 'code' | 'category' | 'status'
type Dialog =
  | { kind: 'add' }
  | { kind: 'edit'; code: string }
  | { kind: 'view'; code: string }
  | { kind: 'deactivate'; detail: DocDetail }
  | { kind: 'delete'; row: DocRow }

const statusTone: Record<DocStatus, string> = { active: 'pass', inactive: 'muted', draft: 'note' }
const statusText: Record<DocStatus, string> = { active: 'Active', inactive: 'Inactive', draft: 'Draft' }
const statusOrder: Record<DocStatus, number> = { active: 0, inactive: 1, draft: 2 }


function problem(err: unknown): string {
  if (err instanceof ApiError) return `${err.message} (${err.code})`
  return String(err)
}

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

export function DocumentMasterPanel({ onChanged }: { onChanged?: () => void }) {
  const [rows, setRows] = useState<DocRow[] | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [q, setQ] = useState('')
  const [category, setCategory] = useState('')
  const [origin, setOrigin] = useState('')
  const [format, setFormat] = useState('')
  const [status, setStatus] = useState('')
  const [sort, setSort] = useState<{ key: SortKey; desc: boolean } | null>(null)
  const [dialog, setDialog] = useState<Dialog | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [fault, setFault] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const reload = useCallback(async () => {
    try {
      setRows(await fetchDocuments())
      setLoadError(null)
    } catch (err) {
      setLoadError(problem(err))
    }
  }, [])

  // The first read: the state is set only when the answer arrives.
  useEffect(() => {
    let cancelled = false
    fetchDocuments()
      .then((r) => {
        if (!cancelled) setRows(r)
      })
      .catch((err: unknown) => {
        if (!cancelled) setLoadError(problem(err))
      })
    return () => {
      cancelled = true
    }
  }, [])

  const shown = useMemo(() => {
    const needle = q.trim().toLowerCase()
    const list = (rows ?? []).filter(
      (d) =>
        (!needle || [d.name, d.code, d.category, d.description].some((s) => s.toLowerCase().includes(needle))) &&
        (!category || d.category === category) &&
        (!origin || d.origin === origin) &&
        (!format || d.applicable_for.includes(format)) &&
        (!status || d.status === status),
    )
    if (!sort) return list
    const by = (d: DocRow): string | number => (sort.key === 'status' ? statusOrder[d.status] : d[sort.key].toLowerCase())
    return [...list].sort((a, b) => {
      const x = by(a)
      const y = by(b)
      const c = x < y ? -1 : x > y ? 1 : a.code.localeCompare(b.code)
      return sort.desc ? -c : c
    })
  }, [rows, q, category, origin, format, status, sort])

  const pager = usePager(shown, 'documents', `${q}|${category}|${origin}|${format}|${status}`)

  // act runs one change, then reads the list again; a refusal is shown in words.
  const act = async (change: () => Promise<unknown>, done: string): Promise<boolean> => {
    setBusy(true)
    setFault(null)
    setNotice(null)
    try {
      await change()
      setNotice(done)
      onChanged?.()
      await reload()
      return true
    } catch (err) {
      setFault(problem(err))
      return false
    } finally {
      setBusy(false)
    }
  }

  // Deactivating is always allowed. When formats, stored rows or rules read the document, the
  // Partner first sees what keeps working; with nothing reading it, it is switched off at once.
  const deactivate = async (row: DocRow) => {
    setFault(null)
    setNotice(null)
    try {
      const detail = await fetchDocument(row.code)
      const u = detail.usage
      if (u.formats.length + u.rules.length + u.records > 0) setDialog({ kind: 'deactivate', detail })
      else await act(() => setDocumentActive(row.code, false), `${row.name} is switched off.`)
    } catch (err) {
      setFault(problem(err))
    }
  }

  const sortBy = (key: SortKey) => setSort((s) => (s?.key === key ? { key, desc: !s.desc } : { key, desc: false }))
  const sortLabel = (key: SortKey): 'ascending' | 'descending' | 'none' => (sort?.key === key ? (sort.desc ? 'descending' : 'ascending') : 'none')
  const header = (key: SortKey, text: string) => (
    <th scope="col" aria-sort={sortLabel(key)}>
      <button type="button" className="docs-sort" onClick={() => sortBy(key)} aria-label={`Sort by ${text.toLowerCase()}`}>
        {text}
        <span aria-hidden="true">{sort?.key === key ? (sort.desc ? ' ▼' : ' ▲') : ''}</span>
      </button>
    </th>
  )

  if (loadError) return <p className="error" role="alert">{loadError}</p>
  if (rows === null) return <p className="loading">Loading the documents…</p>

  const filtering = Boolean(q || category || origin || format || status)
  const clear = () => {
    setQ('')
    setCategory('')
    setOrigin('')
    setFormat('')
    setStatus('')
  }

  return (
    <div className="docs">
      <p className="muted docs-lede">
        The kinds of document the tool can read. Adding a document here creates no layout, list detection, column mapping or rule: teach a layout on the
        mapping screen once the document is ready, then write rules over its rows in the Rule Builder.
      </p>

      <div className="admin-toolbar docs-toolbar">
        <span className="result-count" aria-live="polite">{plural(shown.length, 'document', 'documents')}{filtering ? ` of ${rows.length}` : ''}</span>
        <button type="button" className="action" onClick={() => setDialog({ kind: 'add' })}>Add document</button>
      </div>

      <div className="docs-filters" role="search" aria-label="Filter documents">
        <label className="docs-search">
          <span className="sr-only">Search documents</span>
          <SearchIcon />
          <input type="search" placeholder="Search name, code or description" aria-label="Search documents" value={q} onChange={(e) => setQ(e.target.value)} />
        </label>
        <label>
          Category
          <select aria-label="Category" value={category} onChange={(e) => setCategory(e.target.value)}>
            <option value="">All</option>
            {docCategories.map((c) => <option key={c} value={c}>{c}</option>)}
          </select>
        </label>
        <label>
          Origin
          <select aria-label="Origin" value={origin} onChange={(e) => setOrigin(e.target.value)}>
            <option value="">All</option>
            <option value="builtin">Built-in</option>
            <option value="office">Office</option>
          </select>
        </label>
        <label>
          File format
          <select aria-label="File format" value={format} onChange={(e) => setFormat(e.target.value)}>
            <option value="">All</option>
            {fileKinds.map((k) => <option key={k} value={k}>{fileKindNames[k]}</option>)}
          </select>
        </label>
        <label>
          Status
          <select aria-label="Status" value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="">All</option>
            <option value="active">Active</option>
            <option value="inactive">Inactive</option>
            <option value="draft">Draft</option>
          </select>
        </label>
        {filtering && <button type="button" onClick={clear}>Clear filters</button>}
      </div>

      {fault && <p className="error" role="alert">{fault}</p>}
      {notice && <p className="notice" role="status">{notice}</p>}

      {shown.length === 0 ? (
        <p className="muted docs-empty">
          {filtering ? 'No document matches these filters.' : 'No document yet. Add one.'}
        </p>
      ) : (
        <div className="table-wrap">
          <table className="table-premium docs-table" aria-label="Documents">
            <thead>
              <tr>
                {header('name', 'Name')}
                {header('code', 'Code')}
                {header('category', 'Category')}
                <th scope="col">Description</th>
                <th scope="col">Formats</th>
                <th scope="col">Structure</th>
                {header('status', 'Status')}
                <th scope="col"><span className="sr-only">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              {pager.rows.map((d) => (
                <tr key={d.code}>
                  <td>
                    <span className="docs-name">{d.name}</span>
                    {d.origin === 'builtin' && <span className="tag muted docs-chip">Built-in</span>}
                  </td>
                  <td className="mono">{d.code}</td>
                  <td>{d.category}</td>
                  <td className="docs-desc">{d.description}</td>
                  <td>{formatsText(d.applicable_for)}</td>
                  <td>{d.record_type ? <span className="mono">{d.record_type}</span> : <span className="muted">No structure yet</span>}</td>
                  <td><span className={`tag ${statusTone[d.status]}`}>{statusText[d.status]}</span></td>
                  <td className="docs-actions">
                    <button type="button" aria-label={`View ${d.name}`} onClick={() => setDialog({ kind: 'view', code: d.code })}>View</button>
                    {d.origin === 'office' && (
                      <button type="button" aria-label={`Edit ${d.name}`} onClick={() => setDialog({ kind: 'edit', code: d.code })}>Edit</button>
                    )}
                    {d.status === 'inactive' && (
                      <button type="button" aria-label={`Activate ${d.name}`} disabled={busy}
                        onClick={() => act(() => setDocumentActive(d.code, true), `${d.name} is switched on.`)}>Activate</button>
                    )}
                    {d.status === 'active' && (
                      <button type="button" aria-label={`Deactivate ${d.name}`} disabled={busy} onClick={() => deactivate(d)}>Deactivate</button>
                    )}
                    {d.origin === 'office' && d.referenced_by.formats + d.referenced_by.rules === 0 && (
                      <button type="button" className="ghost" aria-label={`Delete ${d.name}`} onClick={() => setDialog({ kind: 'delete', row: d })}>Delete</button>
                    )}
                    {d.origin === 'office' && d.referenced_by.formats + d.referenced_by.rules > 0 && (
                      <span className="muted docs-used" title="Read by layouts or rules: deactivate it instead of deleting it">
                        Used by {plural(d.referenced_by.formats, 'layout', 'layouts')}, {plural(d.referenced_by.rules, 'rule', 'rules')}
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <PageBar p={pager} noun="documents" />

      {(dialog?.kind === 'add' || dialog?.kind === 'edit') && (
        <DocumentForm
          code={dialog.kind === 'edit' ? dialog.code : undefined}
          rows={rows}
          onClose={() => setDialog(null)}
          onSaved={async (message) => {
            setDialog(null)
            setFault(null)
            setNotice(message)
            onChanged?.()
            await reload()
          }}
        />
      )}
      {dialog?.kind === 'view' && <DocumentView code={dialog.code} onClose={() => setDialog(null)} />}
      {dialog?.kind === 'deactivate' && (
        <DeactivateWarning
          detail={dialog.detail}
          busy={busy}
          onClose={() => setDialog(null)}
          onConfirm={async () => {
            const ok = await act(() => setDocumentActive(dialog.detail.code, false), `${dialog.detail.name} is switched off. What reads it keeps working.`)
            if (ok) setDialog(null)
          }}
        />
      )}
      {dialog?.kind === 'delete' && (
        <Modal title={`Delete ${dialog.row.name}`} onClose={() => setDialog(null)}>
          <p>
            Delete <strong>{dialog.row.name}</strong> (<span className="mono">{dialog.row.code}</span>) and its record structure? Nothing reads it. This cannot be undone;
            the audit log keeps the record that it was deleted.
          </p>
          <div className="admin-actions">
            <button type="button" onClick={() => setDialog(null)}>Cancel</button>
            <button type="button" className="action" disabled={busy}
              onClick={async () => {
                const ok = await act(() => deleteDocument(dialog.row.code), `${dialog.row.name} is deleted.`)
                if (ok) setDialog(null)
              }}>
              {busy ? 'Deleting…' : 'Delete document'}
            </button>
          </div>
        </Modal>
      )}
    </div>
  )
}

// DeactivateWarning says what keeps working before a document that something reads is switched off.
function DeactivateWarning({ detail, busy, onClose, onConfirm }: { detail: DocDetail; busy: boolean; onClose: () => void; onConfirm: () => void }) {
  const u = detail.usage
  return (
    <Modal title={`Deactivate ${detail.name}`} onClose={onClose}>
      <p role="alert">
        <strong>{detail.name}</strong> will no longer be offered for new layouts or new rules. What already reads it keeps working; nothing is changed or switched off.
      </p>
      <ul className="docs-uses">
        <li>{plural(u.formats.length, 'layout', 'layouts')}{u.formats.length > 0 && <>: <span className="mono">{u.formats.join(', ')}</span></>}</li>
        <li>{plural(u.records, 'stored row', 'stored rows')}</li>
        <li>
          {plural(u.active_rules.length, 'active rule', 'active rules')}{u.active_rules.length > 0 && <>: <span className="mono">{u.active_rules.join(', ')}</span></>}
          {u.rules.length > u.active_rules.length && <span className="muted"> ({plural(u.rules.length, 'rule', 'rules')} read it in all)</span>}
        </li>
      </ul>
      <div className="admin-actions">
        <button type="button" onClick={onClose}>Cancel</button>
        <button type="button" className="action" disabled={busy} onClick={onConfirm}>{busy ? 'Working…' : 'Deactivate'}</button>
      </div>
    </Modal>
  )
}

// DocumentView shows one document read only: its columns, what reads it and its audit history.
function DocumentView({ code, onClose }: { code: string; onClose: () => void }) {
  const [detail, setDetail] = useState<DocDetail | null>(null)
  const [error, setError] = useState<string | null>(null)
  useEffect(() => {
    let cancelled = false
    fetchDocument(code)
      .then((d) => {
        if (!cancelled) setDetail(d)
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(problem(err))
      })
    return () => {
      cancelled = true
    }
  }, [code])

  return (
    <Modal title={detail ? detail.name : 'Document'} onClose={onClose}>
      <div className="docs-view">
        {error && <p className="error" role="alert">{error}</p>}
        {!detail && !error && <p className="loading">Loading the document…</p>}
        {detail && (
          <>
            <dl className="docs-facts">
              <dt>Code</dt><dd className="mono">{detail.code}</dd>
              <dt>Category</dt><dd>{detail.category}</dd>
              <dt>Origin</dt><dd>{detail.origin === 'builtin' ? 'Built-in (its definition is in the program files)' : 'Office'}</dd>
              <dt>Status</dt><dd><span className={`tag ${statusTone[detail.status]}`}>{statusText[detail.status]}</span></dd>
              <dt>Applicable for</dt><dd>{formatsText(detail.applicable_for)}</dd>
              <dt>Description</dt><dd>{detail.description}</dd>
            </dl>

            <h3>Record structure</h3>
            {detail.structure ? (
              <>
                <p className="muted">
                  <span className="mono">{detail.structure.name}</span>
                  {detail.structure.copied_from && <> · copied from <span className="mono">{detail.structure.copied_from}</span> (independent since)</>}
                  {detail.structure.locked && <> · read by a layout, stored rows or a rule, so it is locked</>}
                </p>
                <div className="table-wrap">
                  <table className="table-premium" aria-label="Columns">
                    <thead>
                      <tr>
                        <th scope="col">Column</th>
                        <th scope="col">Label</th>
                        <th scope="col">Type</th>
                        <th scope="col">Key</th>
                        <th scope="col">Required</th>
                      </tr>
                    </thead>
                    <tbody>
                      {detail.structure.columns.map((c) => (
                        <tr key={c.name}>
                          <td className="mono">{c.name}</td>
                          <td>{c.label}</td>
                          <td>{c.type}</td>
                          <td>{c.key ? 'Yes' : 'No'}</td>
                          <td>{c.required ? 'Yes' : 'No'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </>
            ) : (
              <p className="muted">No record structure yet. The document is a Draft until one is added.</p>
            )}

            <h3>Read by</h3>
            <ul className="docs-uses">
              <li>{plural(detail.usage.formats.length, 'layout', 'layouts')}{detail.usage.formats.length > 0 && <>: <span className="mono">{detail.usage.formats.join(', ')}</span></>}</li>
              <li>{plural(detail.usage.records, 'stored row', 'stored rows')}</li>
              <li>{plural(detail.usage.rules.length, 'rule', 'rules')}{detail.usage.rules.length > 0 && <>: <span className="mono">{detail.usage.rules.join(', ')}</span></>}</li>
            </ul>

            <h3>History</h3>
            {detail.history.length === 0 ? (
              <p className="muted">Nothing has been changed on this document.</p>
            ) : (
              <div className="table-wrap">
                <table className="table-premium" aria-label="History">
                  <thead>
                    <tr>
                      <th scope="col">When</th>
                      <th scope="col">By</th>
                      <th scope="col">What</th>
                    </tr>
                  </thead>
                  <tbody>
                    {detail.history.map((h) => (
                      <tr key={`${h.at}|${h.action}`}>
                        <td>{h.at.slice(0, 16).replace('T', ' ')} UTC</td>
                        <td>{h.by}</td>
                        <td className="mono">{h.action}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )}
        <div className="admin-actions">
          <button type="button" onClick={onClose}>Close</button>
        </div>
      </div>
    </Modal>
  )
}

