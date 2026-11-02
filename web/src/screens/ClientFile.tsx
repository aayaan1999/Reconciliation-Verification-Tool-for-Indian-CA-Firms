import {
  useEffect, useImperativeHandle, useMemo, useRef, useState, type DragEvent, type FormEvent, type KeyboardEvent, type ReactNode, type Ref,
  type RefObject,
} from 'react'
import { ApiError, fetchClientDetails, fetchHistory, inspectFile, runChecks, updateClient } from '../api.ts'
import { decisionKeys, decisionState, decisionText, stateText, type DecideFn } from '../decisions.ts'
import { DecisionPanel } from './Decisions.tsx'
import { constitutions, states } from '../clientLists.ts'
import { filterResults, noResultFilter, resultRows, type ResultFilter, type ResultRow } from '../filters.ts'
import { formatPaise, formatValue, initials, locatorText } from '../format.ts'
import type { SampleRun } from '../sample/index.ts'
import type {
  CannotCheck, Client, ClientDetails, ComparisonEntry, DecisionKind, DocTotal, Finding, FindingDecision, Locator, Passed, RuleInfo, RunReport,
  StoredDocument, StoredRecord, Unread,
} from '../types.ts'
import { CheckIcon, ChevronIcon, CircleIcon, CloseIcon, DocIcon, UploadIcon } from '../icons.tsx'
import '../styles/clientfile.css'
import { useRowNav } from '../useRowNav.ts'
import { useCan } from '../permissions.ts'
import { ClientForm } from './ClientForm.tsx'
import { Modal } from './Modal.tsx'
import { TeachLayout } from './TeachLayout.tsx'

interface Props {
  client: Client
  runs: SampleRun[] // the client's latest results; a run of every rule replaces older typed ones
  rules: RuleInfo[] // every stored rule, to say how many a run checks
  live: boolean // a server is running, so files can be uploaded
  fromUpload: boolean // the run shown came from an upload, not the sample data
  taxYears: string[]
  clientId?: number // a client in the database: uploads add to its documents
  loading?: boolean
  viewYear?: string // the tax year whose results are shown (a client in the database)
  runYears?: string[] // the tax years in which the client has runs
  onYear?: (year: string) => void // show another tax year's results
  decisions?: Record<string, FindingDecision> // a client's decisions, by finding ID
  onDecide?: DecideFn // a client in the database: decide a finding
  onExport?: () => Promise<void> // a client in the database: the Excel export of the tax year (SPEC-013)
  onDeleteFinding?: DeleteFn // a client in the database: mark a finding deleted, with a reason (SPEC-013)
  onRun: (runs: SampleRun[], year: string, decisions?: Record<string, FindingDecision>) => void
  onEdited?: () => void // the client's details changed (its name may be on the Season board)
}

type Pane = 'findings' | 'documents' | 'history'

// DeleteFn marks a finding deleted with the reason given (SPEC-013 §4).
type DeleteFn = (f: Finding, reason: string) => Promise<void>

const typeId = (r: SampleRun) => r.report.comparison?.id ?? ''
const typeName = (r: SampleRun) => r.report.comparison?.name ?? 'All rules'
const yearsOf = (...lists: string[][]) => [...new Set(lists.flat().filter((y) => y !== ''))].sort()

// The Client file: an "Upload and run" panel (no type of check is chosen: every rule in the
// system runs on what was uploaded, and each file is recognised by its contents); then what
// was read from each document and the results (findings grouped by the document to review,
// each with both figures, the difference and both sources; Cannot check; rules not run;
// passed checks) with filters for status, rule, category, document and a search. Left and
// right arrows move between the tabs; up and down between documents; Enter shows the
// figures read from one.
export function ClientFile(props: Props) {
  const { client, rules, live, fromUpload, taxYears, clientId, loading, viewYear, runYears, onYear, onRun, onEdited } = props
  // A run of every rule is the client's result; results of a type of check from before types
  // were removed show only while the client has no run of every rule.
  const runs = props.runs.some((r) => !r.report.comparison) ? props.runs.filter((r) => !r.report.comparison) : props.runs
  const decisions = props.decisions ?? {}
  const onDecide = props.onDecide
  const canRun = useCan()('run') // uploading files, running checks and editing the client (SPEC-012)
  // The Excel export and deleting a finding are shown only to the roles that hold them (SPEC-013).
  const can = useCan()
  const onExport = can('export') ? props.onExport : undefined
  const onDeleteFinding = can('delete_findings') ? props.onDeleteFinding : undefined
  const [exportError, setExportError] = useState('')
  const [tab, setTab] = useState('')
  const shown = Math.max(0, runs.findIndex((r) => typeId(r) === tab))
  const run = runs.length > 0 ? runs[shown] : undefined
  const [old, setOld] = useState<OldComparison | null>(null) // an earlier comparison being explored
  // The views of a type of check: its findings, the documents it read, and its earlier runs.
  const [pane, setPane] = useState<Pane>('findings')
  const view = old?.run ?? run
  const files = view?.report.files ?? []
  // Each view names how much it holds: the findings of the run, the documents it read.
  const panes: [Pane, string, number | undefined][] = [
    ['findings', 'Findings', view?.report.counts.fail],
    ['documents', 'Documents', files.length],
    ...(clientId !== undefined ? [['history', 'Run history', undefined] as [Pane, string, undefined]] : []),
  ]
  const [details, setDetails] = useState<ClientDetails | null>(null)
  const [editing, setEditing] = useState(false)
  useEffect(() => {
    if (clientId === undefined) return
    let cancelled = false
    fetchClientDetails(clientId)
      .then((d) => {
        if (!cancelled) setDetails(d)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [clientId])
  const upload = useRef<UploadHandle>(null)
  const [dragging, setDragging] = useState(false) // files are dragged over the page
  // Keys: e edits the client, u opens Upload and run (not while typing in a field).
  useEffect(() => {
    const onKey = (e: globalThis.KeyboardEvent) => {
      const target = e.target as HTMLElement | null
      if (target && ['INPUT', 'SELECT', 'TEXTAREA'].includes(target.tagName)) return
      if (e.ctrlKey || e.metaKey || e.altKey) return
      if (e.key === 'e' && clientId !== undefined) {
        e.preventDefault()
        setEditing(true)
      } else if (e.key === 'u' && canRun) {
        e.preventDefault()
        upload.current?.open()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [clientId, canRun])
  const [open, setOpen] = useState<number | null>(null)
  const nav = useRowNav(files.length, (i) => setOpen(open === i ? null : i))
  const table = useRef<HTMLTableElement>(null)
  const heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => (table.current ?? heading.current)?.focus({ preventScroll: true }), [view])
  // Choosing the Documents view puts the keys on its table: up and down choose a file.
  useEffect(() => {
    if (pane === 'documents') table.current?.focus({ preventScroll: true })
  }, [pane])
  const choose = (i: number) => {
    setTab(typeId(runs[i]))
    setOld(null)
    setOpen(null)
    nav.setSelected(0)
  }
  const tabKeys = (e: KeyboardEvent) => {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    e.preventDefault()
    choose((shown + (e.key === 'ArrowRight' ? 1 : runs.length - 1)) % runs.length)
  }

  // Files can be dropped anywhere on the page: they go to the Upload and run panel.
  const canDrop = canRun && live
  const onPageDrag = (e: DragEvent) => {
    if (!canDrop || !e.dataTransfer.types.includes('Files')) return
    e.preventDefault()
    setDragging(true)
  }

  return (
    <section
      aria-labelledby="client-title"
      className={dragging ? 'client-file dragging' : 'client-file'}
      onDragOver={onPageDrag}
      onDragLeave={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setDragging(false)
      }}
      onDrop={(e) => {
        if (!canDrop) return
        e.preventDefault()
        setDragging(false)
        upload.current?.receive(e.dataTransfer.files)
      }}
    >
      {dragging && (
        <div className="page-drop" aria-hidden="true">
          <UploadIcon /> Drop the files to upload and run
        </div>
      )}
      <div className="file-head">
        <div className="file-id">
          <span className="file-avatar" aria-hidden="true">{initials(client.name)}</span>
          <div className="file-id-text">
            <p className="eyebrow">Client file</p>
            <h1 id="client-title" ref={heading} tabIndex={-1}>{client.name}</h1>
            {!editing && (
              <ClientChips panLast4={client.pan_last4} d={clientId !== undefined ? details : null}
                onEdit={canRun && clientId !== undefined && details ? () => setEditing(true) : undefined} />
            )}
          </div>
        </div>
        <div className="file-actions">
          {clientId !== undefined && viewYear && onYear && (
            <label className="year-pick">
              <span>Results for</span>
              <select aria-label="Results for tax year" value={viewYear} onChange={(e) => {
                setOld(null)
                onYear(e.target.value)
              }}>
                {yearsOf(taxYears, runYears ?? [], [viewYear]).map((y) => (
                  <option key={y} value={y}>
                    {y}
                    {(runYears ?? []).includes(y) ? '' : ' (no run yet)'}
                  </option>
                ))}
              </select>
            </label>
          )}
          {onExport && (
            <button
              type="button"
              onClick={() => {
                setExportError('')
                onExport().catch((e: unknown) => setExportError(e instanceof Error ? e.message : 'The export failed.'))
              }}
            >
              Export Excel
            </button>
          )}
          {canRun && (
            <UploadPanel
              ref={upload}
              key={viewYear}
              live={live}
              taxYears={yearsOf(taxYears, runYears ?? [], viewYear ? [viewYear] : [])}
              initialYear={viewYear}
              clientId={clientId}
              rules={rules}
              hasResults={run !== undefined}
              waiting={loading === true}
              onRun={(next, first, year, decided) => {
                setTab(first)
                setOld(null)
                setPane('findings')
                setOpen(null)
                onRun(next, year, decided)
              }}
            />
          )}
        </div>
      </div>
      {exportError && <p className="error" role="alert">{exportError}</p>}
      {!canRun && <p className="muted">Your role can view this client, but not upload files or run checks.</p>}
      {canRun && clientId !== undefined && details && editing && (
        <ClientForm
          mode="edit"
          initial={details}
          taxYears={[]}
          onCancel={() => setEditing(false)}
          onSave={async (d) => {
            setDetails(await updateClient(clientId, d))
            setEditing(false)
            onEdited?.()
          }}
        />
      )}
      {loading ? (
        <p className="loading">Loading the latest result…</p>
      ) : !run ? (
        <div className="file-start">
          <p className="empty">
            No run yet for this client{viewYear ? ` in tax year ${viewYear}` : ''}.
            {canRun && ' Next: press Upload and run (u), or drop the files anywhere on this page.'} Each file is recognised
            by its contents, and every rule in the system runs on what was uploaded.
          </p>
          <h2>Files to bring</h2>
          <ol className="bring-list">
            <li><strong>The books</strong><span>From the accounting package: ledgers, sales register, expense ledger (this year and last year's, for the Worklist).</span></li>
            <li><strong>The tax suite</strong><span>The filing pack: computation, financial statements, tax audit report, Form 26AS, Form 67.</span></li>
            <li><strong>The portals</strong><span>GST returns such as GSTR-1, and other portal downloads.</span></li>
          </ol>
          <p className="muted">JSON, Excel (.xlsx) or CSV. Save an old .xls file as .xlsx or .csv first.</p>
        </div>
      ) : (
        <>
          {runs.length > 1 && (
          <div className="tabs premium-tabs" role="tablist" aria-label="Types of check" onKeyDown={tabKeys}>
            {runs.map((r, i) => (
              <button
                key={typeId(r)}
                type="button"
                role="tab"
                aria-selected={i === shown}
                tabIndex={i === shown ? 0 : -1}
                onClick={() => choose(i)}
                className="premium-tab"
              >
                <span className="tab-name" title={typeName(r)}>{typeName(r)}</span>{' '}
                <span className="muted">
                  <span className="sep">· </span>{r.report.tax_year} · {r.report.counts.fail} fail
                  {onDecide ? ` · ${openCount(r, decisions)} open` : ''}
                  {r.last_run_on ? ` · ${r.last_run_on}` : ''}
                </span>
              </button>
            ))}
          </div>
          )}
          <div role="tabpanel" aria-label={typeName(run)}>
            {old && (
              <div className="cf-readonly" role="note">
                <p>
                  Run {old.entry.no} of {old.of}, run on {old.entry.finished_at}. Read only: this is what the tool reported
                  then, from the drafts it read. Decisions cannot be made here.
                </p>
                <button type="button" onClick={() => setOld(null)}>Back to latest</button>
              </div>
            )}
            <RunSummary run={old?.run ?? run} fromUpload={fromUpload} old={old}
              decisions={old || !onDecide ? undefined : decisions} />
            <div className="views cf-views" role="group" aria-label="Views of this check">
              {panes.map(([id, text, count]) => (
                <button key={id} type="button" aria-pressed={pane === id} onClick={() => setPane(id)}>
                  {text}
                  {count !== undefined && <>{' '}<span className="cf-view-count">{count}</span></>}
                </button>
              ))}
            </div>
            {pane === 'history' && clientId !== undefined && viewYear ? (
              <ComparisonList key={`${viewYear}|${typeId(run)}`} clientId={clientId} taxYear={viewYear} latest={run} opened={old?.entry.no}
                onOpen={(o) => {
                  setOld(o)
                  setPane('findings')
                  setOpen(null)
                  nav.setSelected(0)
                }} />
            ) : (
              <RunView key={`${typeId(run)}|${old?.entry.no ?? ''}`} run={old?.run ?? run} nav={nav} open={open}
                setOpen={setOpen} table={table} decisions={old ? {} : decisions} onDecide={old ? undefined : onDecide}
                onDeleteFinding={old ? undefined : onDeleteFinding}
                part={pane === 'documents' ? 'documents' : 'findings'} />
            )}
          </div>
        </>
      )}
    </section>
  )
}

// DocumentRail lists the documents the shown type of check reads: whether each was received
// and the format it was recognised as. A run with no type of check lists the files read.
function DocumentRail({ run }: { run: SampleRun }) {
  const r = run.report
  const profileOf = (type: string) => r.files.find((f) => f.document_type === type)
  if (r.comparison) {
    return (
      <div className="panel doc-rail">
        <h3>Documents this check reads</h3>
        <ul className="doc-list" aria-label="Documents of the comparison">
          {r.comparison.documents.map((d) => {
            const f = profileOf(d.document_type)
            return (
              <li key={d.document_type}>
                <span className={d.present ? 'got' : 'missing'}>{d.present ? <CheckIcon /> : <CircleIcon />}</span>
                <span>
                  <small>{d.present ? 'Read' : 'Not uploaded yet'}</small>{' '}
                  {d.name}
                  {d.present && f && <small>Format: {f.profile_id} v{f.profile_version}</small>}
                </span>
              </li>
            )
          })}
        </ul>
      </div>
    )
  }
  return (
    <div className="panel doc-rail">
      <h3>Files read</h3>
      <ul className="doc-list">
        {r.files.map((f) => (
          <li key={f.file + '|' + f.document_type}>
            <span className={f.errors.length > 0 ? 'missing' : 'got'}>{f.errors.length > 0 ? <CircleIcon /> : <CheckIcon />}</span>
            <span>
              {f.document_type ? f.document_type.replace(/_/g, ' ') : f.file}
              <small>
                {f.file} · {f.profile_id} v{f.profile_version}
              </small>
            </span>
          </li>
        ))}
      </ul>
    </div>
  )
}

// filesText names the files a comparison read: one file holding several documents (a
// filing pack) once, with how many documents it gave.
function filesText(files: ComparisonEntry['files']): string {
  const byFile = new Map<string, string[]>()
  for (const f of files) {
    const k = `${f.file_name}, draft ${f.draft_no}`
    byFile.set(k, [...(byFile.get(k) ?? []), f.document_type.replace(/_/g, ' ')])
  }
  return [...byFile].map(([file, docs]) => (docs.length === 1 ? `${docs[0]}: ${file}` : `${file} (${docs.length} documents)`)).join('; ')
}

interface OldComparison {
  entry: ComparisonEntry
  of: number // the number of the latest comparison
  run: SampleRun
}

// ComparisonList lists every run of the shown type of check as Comparison 1, 2, 3…, newest
// first, with the drafts each read. Choosing an earlier one opens it to explore, read only,
// exactly as it was reported (SPEC-009).
function ComparisonList({ clientId, taxYear, latest, opened, onOpen }: {
  clientId: number
  taxYear: string
  latest: SampleRun // the shown type's latest result; a new run lists again
  opened?: number
  onOpen: (o: OldComparison | null) => void
}) {
  const [list, setList] = useState<ComparisonEntry[] | null>(null) // null until the server answers
  const [error, setError] = useState('')
  const comparison = typeId(latest)
  useEffect(() => {
    let cancelled = false
    fetchHistory(clientId, taxYear, comparison)
      .then((h) => {
        if (!cancelled) setList(Array.isArray(h.comparisons) ? h.comparisons : [])
      })
      .catch(() => {
        if (!cancelled) setList([]) // an older server has no history: the latest result is shown as before
      })
    return () => {
      cancelled = true
    }
  }, [clientId, taxYear, comparison, latest])
  if (list === null) return <p className="loading">Loading the runs…</p>
  if (list.length === 0) return <p className="empty">No earlier runs are kept for this client in tax year {taxYear}.</p>
  const last = list.length
  const choose = async (c: ComparisonEntry) => {
    if (c.no === last) return onOpen(null)
    try {
      const h = await fetchHistory(clientId, taxYear, comparison, c.no)
      if (h.report && h.facts) onOpen({ entry: c, of: last, run: { report: h.report, facts: h.facts, last_run_on: c.finished_at } })
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }
  return (
    <div className="cf-history">
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <ol className="cf-timeline" aria-label="Runs of this client">
        {[...list].reverse().map((c) => {
          const shownNow = (opened ?? last) === c.no
          return (
            <li key={c.no} className={shownNow ? 'cf-run-card shown' : 'cf-run-card'}>
              <div className="cf-run-card-head">
                <h4>Run {c.no}</h4>
                {c.no === last && <span className="tag pass">Latest</span>}
                <span className="muted">{c.finished_at}</span>
              </div>
              {c.counts ? (
                <>
                  <ResultBar counts={c.counts} />
                  <p className="cf-run-counts">
                    <span className="pass-text">{c.counts.pass} pass</span> ·{' '}
                    <span className="fail-text">{c.counts.fail} {c.counts.fail === 1 ? 'finding' : 'findings'}</span> ·{' '}
                    <span className="cf-warn-text">{c.counts.cannot_check} cannot check</span>
                    <FindingsChange now={c} before={list.find((p) => p.no === c.no - 1)} />
                  </p>
                </>
              ) : (
                <p className="muted">Result not kept for this run</p>
              )}
              <p className="cf-run-files">Read: {filesText(c.files)}</p>
              {c.includes_testing && <p className="cf-warn-text">Included rules not yet approved by the CA</p>}
              <button type="button" aria-current={shownNow} disabled={!c.kept} onClick={() => choose(c)}
                aria-label={c.no === last ? `Show Run ${c.no}, the latest` : `Open Run ${c.no} read only`}>
                {c.no === last ? 'Show the latest' : 'Open read only'}
              </button>
            </li>
          )
        })}
      </ol>
    </div>
  )
}

// FindingsChange says how the number of findings moved from the run before: a count of
// findings, never a judgment of which run was right.
function FindingsChange({ now, before }: { now: ComparisonEntry; before?: ComparisonEntry }) {
  if (!now.counts || !before?.counts) return null
  const d = now.counts.fail - before.counts.fail
  const n = Math.abs(d)
  const word = n === 1 ? 'finding' : 'findings'
  return (
    <span className="cf-change">
      {' · '}
      {d === 0 ? `same number of findings as Run ${before.no}` : `${n} ${d > 0 ? 'more' : 'fewer'} ${word} than Run ${before.no}`}
    </span>
  )
}

// ResultBar shows pass, findings and cannot check in proportion to the checks of a run. The
// widths come from whole counts; no amount of money is involved.
function ResultBar({ counts }: { counts: { checks: number; pass: number; fail: number; cannot_check: number } }) {
  if (counts.checks === 0) return null
  const share = (n: number) => `${(n * 100) / counts.checks}%`
  return (
    <div className="stack-bar cf-result-bar" aria-hidden="true">
      <span className="pass" style={{ width: share(counts.pass) }} />
      <span className="fail" style={{ width: share(counts.fail) }} />
      <span className="cf-cannot" style={{ width: share(counts.cannot_check) }} />
    </div>
  )
}

// RunSummary is the card over a run's result (Recon Screen Document v2 §7.1): which run, its
// date and tax year, the counts, how many findings are still open, a bar of the results and
// the note when rules not yet approved by the CA ran.
function RunSummary({ run, fromUpload, old, decisions }: {
  run: SampleRun
  fromUpload: boolean
  old: OldComparison | null
  decisions?: Record<string, FindingDecision> // a client in the database, on its latest run
}) {
  const r = run.report
  const c = r.counts
  const findings = r.findings.flatMap((g) => g.findings)
  const open = decisions ? findings.filter((f) => decisionState(decisions, f) === 'open').length : 0
  const counts: [string, number, string][] = [
    ['Checks', c.checks, ''],
    ['Pass', c.pass, 'pass'],
    ['Findings', c.fail, 'fail'],
    ['Cannot check', c.cannot_check, 'warn'],
  ]
  return (
    <section className="cf-summary" aria-label="Result summary">
      <div className="cf-summary-top">
        <div className="cf-run">
          <p className="cf-run-label">
            {old ? `Run ${old.entry.no} · read only` : 'Latest run'} · <span>{fromUpload ? 'Result of the files you uploaded' : 'Sample data'}</span>
          </p>
          <p className="cf-run-when">
            {run.last_run_on ? `${run.last_run_on} · ` : ''}tax year {r.tax_year}
          </p>
        </div>
        <ul className="cf-counts" aria-label="Checks of this run">
          {counts.map(([label, n, tone]) => (
            <li key={label} className={tone}>
              <b>{n}</b> <span>{label}</span>
            </li>
          ))}
        </ul>
        {decisions && (
          <p className="cf-open">
            <b>{open}</b> <span>open, no decision yet</span> <span className="muted">{findings.length - open} decided</span>
          </p>
        )}
      </div>
      <ResultBar counts={c} />
      {r.includes_testing && (
        <p className="cf-testing" role="note">
          This run includes rules not yet approved by the CA (status testing).
        </p>
      )}
    </section>
  )
}

// ClientChips shows who the client is as chips under its name (Recon Screen Document v2 §6.1):
// PAN ending, trade name, constitution, city, GST registrations and accounting software. The
// full PAN is never shown. Without the details (sample data) only the PAN ending shows.
function ClientChips({ panLast4, d, onEdit }: { panLast4: string; d: ClientDetails | null; onEdit?: () => void }) {
  const label = (list: [string, string][], v: string) => list.find(([k]) => k === v)?.[1] ?? v
  const place = d ? [d.address.city, d.address.state ? label(states, d.address.state) : ''].filter(Boolean).join(', ') : ''
  const gstins = d?.gstins ?? []
  const chips: { text: string; title?: string; mono?: boolean }[] = [
    { text: `PAN ending ${d?.pan_last4 || panLast4}`, mono: true },
    ...(d?.trade_name ? [{ text: d.trade_name, title: 'Trade name' }] : []),
    ...(d?.constitution ? [{ text: label(constitutions, d.constitution) }] : []),
    ...(place ? [{ text: place }] : []),
    // Each GSTIN as the server sent it (masked for roles that may not see it in full, SPEC-013
    // AC-6): the first two, then a count of the rest with their numbers on hover.
    ...gstins.slice(0, 2).map((g) => ({
      text: `GSTIN ${g.gstin}${g.primary && gstins.length > 1 ? ' (primary)' : ''}`, mono: true,
    })),
    ...(gstins.length > 2
      ? [{ text: `+${gstins.length - 2} more GSTINs`, title: gstins.slice(2).map((g) => g.gstin).join(', ') }]
      : []),
    ...(d?.accounting_software ? [{ text: d.accounting_software, title: 'Accounting software' }] : []),
  ]
  return (
    <ul className="client-chips" aria-label="Client details">
      {chips.map((c) => (
        <li key={c.text} className={c.mono ? 'chip mono' : 'chip'} title={c.title}>{c.text}</li>
      ))}
      {onEdit && (
        <li>
          <button type="button" className="ghost chip-action" aria-keyshortcuts="e" onClick={onEdit} title="Edit client (e)">
            Edit client
          </button>
        </li>
      )}
    </ul>
  )
}

// UploadHandle lets the Client file open the panel (key u) and hand it files dropped anywhere
// on the page.
interface UploadHandle {
  open: () => void
  receive: (list: FileList) => void
}

// What the server said about a chosen file before the run (Recon Screen Document v2 §6.2):
// still being checked, recognised (as which formats, for which tax year), matching no known
// format, or not checked (the server could not inspect it; the run reports on it).
type Seen =
  | { state: 'checking' }
  | { state: 'known'; as: string; year: string }
  | { state: 'unknown' }
  | { state: 'unchecked' }

// An old Excel file cannot be read; it is listed so the user sees what to do with it.
const isXls = (f: File) => /\.xls$/i.test(f.name)

function sizeText(bytes: number): string {
  return bytes < 1024 ? `${bytes} B` : `${Math.round(bytes / 1024)} KB`
}

// The four things a run does on the server. The run is one request, so the screen cannot
// tell which step is under way: the list shows the order, not progress.
const runSteps = ['Reading the files', 'Recognising each document', 'Running every rule', 'Saving the result']

// UploadPanel uploads files and runs every rule in the system on them: there is no type of
// check to choose, and each file is recognised by its contents, never by its name.
function UploadPanel({ ref, ...props }: {
  ref?: Ref<UploadHandle>
  live: boolean
  taxYears: string[]
  clientId?: number
  initialYear?: string
  rules: RuleInfo[]
  hasResults: boolean
  waiting: boolean // the client's latest result is still loading: the form does not open yet
  onRun: (runs: SampleRun[], first: string, year: string, decisions?: Record<string, FindingDecision>) => void
}) {
  const { live, taxYears, initialYear, clientId, rules, hasResults, waiting, onRun } = props
  const [files, setFiles] = useState<File[]>([])
  const [seen, setSeen] = useState<Record<string, Seen>>({}) // by file name
  const [taxYear, setTaxYear] = useState(initialYear ?? taxYears[0] ?? '2025-26')
  const [yearNote, setYearNote] = useState('') // what the files say about their tax year
  const [testing, setTesting] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const [teaching, setTeaching] = useState<File | null>(null)
  const [taught, setTaught] = useState('')
  const [shut, setShut] = useState(hasResults) // the form closed; it opens by itself only for a client with no results
  // Results that arrive after the form opened (a client's result still loading) close it too,
  // so the findings are what the screen shows first.
  // A form the user opened stays open.
  const [hadResults, setHadResults] = useState(hasResults)
  const [asked, setAsked] = useState(false)
  if (hasResults !== hadResults) {
    setHadResults(hasResults)
    if (hasResults && !asked) setShut(true)
  }
  const [over, setOver] = useState(false) // files are being dragged over the drop area
  const [removing, setRemoving] = useState('') // the file whose removal is being confirmed
  const years = yearsOf(taxYears.length > 0 ? taxYears : ['2025-26'], [taxYear])
  const year = taxYear
  const readable = files.filter((f) => !isXls(f))
  // A file no known format matches can be taught on the mapping screen: the server said so
  // when the file was added, or when the checks ran.
  const unknown = (f: File) =>
    seen[f.name]?.state === 'unknown' || (error?.code === 'PROFILE_NO_MATCH' && error.message.includes(f.name))

  // chooseFiles asks the server what each file is and which tax year it is for and, when
  // they agree, selects that year: a run for another year would refuse every file
  // (CONFLICTING_VALUES).
  const chooseFiles = async (list: File[]) => {
    setFiles(list)
    setYearNote('')
    const toCheck = list.filter((f) => !isXls(f))
    if (!live || toCheck.length === 0) return
    setSeen(Object.fromEntries(toCheck.map((f) => [f.name, { state: 'checking' } as Seen])))
    const found = new Set<string>()
    for (const f of toCheck) {
      let s: Seen = { state: 'unchecked' }
      try {
        const i = await inspectFile(f)
        if (i.tax_year) found.add(i.tax_year)
        if (typeof i.recognised_as === 'string') {
          s = i.recognised_as === '' ? { state: 'unknown' } : { state: 'known', as: i.recognised_as, year: i.tax_year ?? '' }
        }
      } catch {
        // A file the server cannot inspect is reported when the checks run.
      }
      setSeen((prev) => ({ ...prev, [f.name]: s }))
    }
    const ys = [...found].sort()
    if (ys.length === 1) {
      if (ys[0] !== taxYear) setYearNote(`The files are for tax year ${ys[0]}, so the tax year was set to ${ys[0]}.`)
      setTaxYear(ys[0])
    } else if (ys.length === 2 && priorYear(ys[1]) === ys[0]) {
      // This year's files with last year's ledger (SPEC-006 part 3): the run is this year's.
      setYearNote(
        `The files are for tax years ${ys[0]} and ${ys[1]}, so the tax year was set to ${ys[1]}. ` +
          `A ledger for ${ys[0]} is kept as last year's, for the Worklist; other ${ys[0]} files are not read.`,
      )
      setTaxYear(ys[1])
    } else if (ys.length > 1) {
      setYearNote(`The files are for different tax years (${ys.join(', ')}); a run reads only the files of the tax year chosen.`)
    }
  }

  // What a run checks: every rule written to run (the rest of the rule book is text only for
  // now) that is active, or testing with the switch below on.
  const runnable = rules.filter((r) => r.runnable && r.tax_years.includes(year))
  const willRun = runnable.filter((r) => r.status === 'active' || (testing && r.status === 'testing')).length

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (busy) return
    setError(null)
    setBusy(true)
    try {
      const res = await runChecks(readable, year, testing, clientId)
      // A reply without results (an older server) still shows its single report.
      const results = res.results ?? [{ comparison_id: '', last_run_on: '', report: res.report }]
      onRun(
        results.map((r) => ({ report: r.report, facts: res.facts, last_run_on: r.last_run_on })),
        '',
        year,
        res.decisions,
      )
      setShut(true) // the form closes on the new results
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError('ERROR', String(err)))
    } finally {
      setBusy(false)
    }
  }

  const dropped = (list: FileList) => {
    const ok = [...list].filter((f) => /\.(json|csv|xlsx|xls)$/i.test(f.name))
    if (ok.length > 0) chooseFiles([...files.filter((f) => !ok.some((o) => o.name === f.name)), ...ok])
  }
  const openPanel = () => {
    setAsked(true)
    setShut(false)
  }
  const close = () => {
    setAsked(false)
    setShut(true)
  }
  useImperativeHandle(ref, () => ({
    open: openPanel,
    receive: (list) => {
      openPanel()
      if (live && !busy) dropped(list)
    },
  }))

  // fileState is one file's line under its name and its tag on the right.
  const fileState = (f: File): { line: ReactNode; tag?: ReactNode } => {
    if (isXls(f)) {
      return {
        line: <span className="fail-text">An old Excel file (.xls) is not read. Save it as .xlsx or .csv first, then add it again.</span>,
        tag: <span className="tag fail">Not read</span>,
      }
    }
    if (unknown(f)) {
      return { line: <span className="cf-warn-text">The file matches no known format.</span>, tag: <span className="tag note">Not recognised</span> }
    }
    const s = seen[f.name]
    if (s?.state === 'checking') return { line: <span className="muted">Checking what this file is…</span> }
    if (s?.state === 'known') {
      return { line: <span className="muted">{s.as}{s.year ? ` · ${s.year}` : ''}</span>, tag: <span className="tag pass">Recognised</span> }
    }
    return { line: <span className="muted">Recognised when the checks run.</span> }
  }

  const count = readable.length
  return (
    <>
    <button type="button" className="primary btn-golden" aria-haspopup="dialog" aria-keyshortcuts="u"
      title={hasResults ? 'Upload newer or more files and run every rule again (u)' : "Upload the client's files and run the checks (u)"}
      onClick={openPanel}>
      <UploadIcon /> Upload and run
    </button>
    {!shut && !waiting && (
    <Modal title="Upload and run" onClose={close} closeDisabled={busy}>
    <form className="upload cf-upload" onSubmit={submit} aria-label="Upload and run">
      {busy ? (
        <div className="cf-running" aria-live="polite">
          <div className="cf-spinner" role="progressbar" aria-label="Running the checks" />
          <p className="cf-running-title">
            Running {willRun} {willRun === 1 ? 'rule' : 'rules'} on {count} {count === 1 ? 'file' : 'files'}
          </p>
          <ol className="cf-run-steps">
            {runSteps.map((s) => (
              <li key={s}>{s}</li>
            ))}
          </ol>
          <p className="muted">The server does these in one go; the result opens as soon as it is saved.</p>
        </div>
      ) : (
      <>
      <div className="upload-body">
      {clientId !== undefined && (
        <p className="muted">Files are added to this client's documents; a newer file of the same document replaces the older one.</p>
      )}
      {!live && (
        <p className="notice" role="note">
          Sample data: start recon-server to upload files and run the checks.
        </p>
      )}
      <p className="cf-what-runs" role="note" aria-label="What runs">
        <b>Every rule in the system runs on the files uploaded; there is nothing to choose.</b> Each file is recognised by
        what it contains, not by its name. {live && `${willRun} of the ${rules.length} rules in the system can run now; the
        others are not written as checks yet.`}
      </p>
      <div
        className={over ? 'cf-drop over' : 'cf-drop'}
        onDragOver={(e) => {
          if (!live) return
          e.preventDefault()
          setOver(true)
        }}
        onDragLeave={() => setOver(false)}
        onDrop={(e) => {
          e.preventDefault()
          e.stopPropagation() // the page's own drop area is behind this one
          setOver(false)
          if (live) dropped(e.dataTransfer.files)
        }}
      >
        <span className="cf-drop-icon" aria-hidden="true"><UploadIcon /></span>
        <label>
          <span className="cf-drop-title">Drop files here or <span className="cf-link">choose files</span></span>
          <span className="cf-drop-help">JSON, Excel (.xlsx) or CSV, one or more. Save an old .xls file as .xlsx or .csv first.</span>
          <input
            type="file"
            multiple
            className="cf-sr-only"
            accept=".json,application/json,.xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,.csv,text/csv"
            aria-label="Files to check"
            disabled={!live}
            onChange={(e) => chooseFiles([...(e.target.files ?? [])])}
          />
        </label>
      </div>
      {files.length > 0 && (
        <ul className="cf-files" aria-label="Files chosen">
          {files.map((f) => {
            const s = fileState(f)
            return (
              <li key={f.name} className={isXls(f) ? 'refused' : unknown(f) ? 'unknown' : undefined}>
                <span className="cf-file-icon" aria-hidden="true"><DocIcon /></span>
                <span className="cf-file-text">
                  <span className="cf-file-name">{f.name} <small className="muted">{sizeText(f.size)}</small></span>
                  {s.line}
                </span>
                {unknown(f) && !teaching && (
                  <button type="button" aria-label={`Teach this layout (${f.name})`} onClick={() => setTeaching(f)}>
                    Teach this layout
                  </button>
                )}
                {s.tag}
                {removing === f.name ? (
                  <span className="confirm" role="group" aria-label={`Remove ${f.name}?`}>
                    Remove?
                    <button type="button" className="danger" onClick={() => {
                      setRemoving('')
                      chooseFiles(files.filter((x) => x !== f))
                    }}>
                      Remove
                    </button>
                    <button type="button" onClick={() => setRemoving('')}>Keep</button>
                  </span>
                ) : (
                  <button type="button" className="ghost icon-btn" aria-label={`Remove ${f.name}`} title="Remove this file" onClick={() => setRemoving(f.name)}>
                    <CloseIcon />
                  </button>
                )}
              </li>
            )
          })}
        </ul>
      )}
      <div className="cf-year">
        <label>
          <span className="cf-year-label">Tax year</span>
          <select aria-label="Tax year of the run" value={year} onChange={(e) => setTaxYear(e.target.value)} disabled={!live}>
            {years.map((y) => (
              <option key={y}>{y}</option>
            ))}
          </select>
          <span className="muted">Worked out from the files. Change it only if the files are dated wrongly.</span>
        </label>
        {yearNote && (
          <p className="notice" role="status">
            {yearNote}
          </p>
        )}
      </div>
      {error && (
        <p className="error" role="alert">
          <strong>{error.code}</strong>: {error.message}
        </p>
      )}
      {taught && <p className="pass-text">{taught}</p>}
      {teaching && (
        <TeachLayout
          file={teaching}
          onCancel={() => setTeaching(null)}
          onSaved={(res) => {
            // The file now has a layout: the run recognises it.
            setSeen((prev) => ({ ...prev, [teaching.name]: { state: 'unchecked' } }))
            setTeaching(null)
            setError(null)
            setTaught(`Layout saved as ${res.profile_id}. Press Run checks again.`)
          }}
        />
      )}
      </div>
      <div className="cf-upload-foot">
        <label className="cf-switch">
          <input type="checkbox" checked={testing} onChange={(e) => setTesting(e.target.checked)} disabled={!live} />
          Include rules not yet approved by the CA
        </label>
        <button type="button" className="ghost" onClick={close}>Cancel</button>
        <button type="submit" className="btn-golden" disabled={!live || count === 0}>
          Run checks
          {count > 0 && <span aria-hidden="true"> on {count} {count === 1 ? 'file' : 'files'}</span>}
        </button>
      </div>
      </>
      )}
    </form>
    </Modal>
    )}
    </>
  )
}

interface RunViewProps {
  run: SampleRun
  nav: ReturnType<typeof useRowNav>
  open: number | null
  setOpen: (i: number | null) => void
  table: RefObject<HTMLTableElement | null>
  decisions: Record<string, FindingDecision>
  onDecide?: DecideFn
  onDeleteFinding?: DeleteFn

  part: 'findings' | 'documents' // the view of the type of check shown
}

// openCount is the number of findings of a run with no decision, or open again.
function openCount(r: SampleRun, decisions: Record<string, FindingDecision>): number {
  return r.report.findings.flatMap((g) => g.findings).filter((f) => decisionState(decisions, f) === 'open').length
}

const statusText: Record<string, string> = { fail: 'Fail', cannot_check: 'Cannot check', pass: 'Pass' }

// The result filter: one kind of result at a time, or all of them.
const segments: [string, string][] = [['fail', 'Findings'], ['cannot_check', 'Cannot check'], ['pass', 'Passed'], ['', 'All']]

// The one result shown in full beside the list has this id, so choosing a result can focus it.
const detailId = 'cf-detail'

const typing = (t: EventTarget | null) => t instanceof HTMLElement && ['INPUT', 'SELECT', 'TEXTAREA'].includes(t.tagName)

// A group of results under the document they belong to: the document to fix for a finding,
// the document a missing figure comes from for Cannot check.
interface ResultGroup {
  heading: string
  rows: ResultRow[]
}

function RunView({ run, nav, open, setOpen, table, decisions, onDecide, onDeleteFinding, part }: RunViewProps) {
  const [decisionFilter, setDecisionFilter] = useState('')
  const [at, setAt] = useState('') // the key of the result chosen in the list
  const [autoNext, setAutoNext] = useState(true)
  const [said, setSaid] = useState('') // what the last decision recorded, for a moment
  const [deleted, setDeleted] = useState<string[]>([]) // findings deleted on this screen
  const r = run.report
  const files = r.files

  const [f, setF] = useState<ResultFilter>({ ...noResultFilter, status: 'fail' })
  const rows = useMemo(() => resultRows(r), [r])
  // A decision filter keeps only findings with that decision: other results have none.
  const byDecision = (x: ResultRow) => decisionFilter === '' || (x.finding !== undefined && decisionState(decisions, x.finding) === decisionFilter)
  const others = filterResults(rows, f, 'status').filter(byDecision)
  const shown = others.filter((x) => f.status === '' || x.status === f.status)
  const options = (field: keyof ResultFilter, pick: (row: ResultRow) => string[]) =>
    [...new Set(filterResults(rows, f, field).flatMap(pick))].filter((v) => v !== '').sort()
  const active = f.search !== '' || f.document !== '' || f.category !== '' || f.ruleId !== '' || decisionFilter !== ''
  const clear = () => {
    setF({ ...noResultFilter, status: f.status })
    setDecisionFilter('')
  }
  const clearOnEscape = (e: KeyboardEvent) => {
    if (e.key === 'Escape' && active) {
      e.stopPropagation()
      clear()
    }
  }
  const set = (field: keyof ResultFilter) => (e: { target: { value: string } }) => setF({ ...f, [field]: e.target.value })

  const groups: ResultGroup[] = []
  for (const x of shown) {
    const heading = x.documents[0] ?? ''
    const g = groups.find((y) => y.heading === heading)
    if (g) g.rows.push(x)
    else groups.push({ heading, rows: [x] })
  }
  // The list holds every result shown, by document; beside it the one chosen is shown in
  // full (the first, until another is chosen), so the screen holds one result, not all.
  const flat = groups.flatMap((g) => g.rows)
  const chosen = flat.find((x) => x.key === at) ?? flat[0]
  const chosenAt = chosen ? flat.indexOf(chosen) : -1
  const chosenGroup = groups.find((g) => chosen && g.rows.includes(chosen))
  const pending = useRef(false) // focus the result once the chosen one is shown
  useEffect(() => {
    if (!pending.current || !chosen) return
    pending.current = false
    const detail = document.getElementById(detailId)
    detail?.focus({ preventScroll: true })
    detail?.scrollIntoView?.({ block: 'nearest' })
  }, [chosen])
  useEffect(() => {
    if (!said) return
    const t = setTimeout(() => setSaid(''), 4000)
    return () => clearTimeout(t)
  }, [said])
  const pick = (x: ResultRow) => {
    pending.current = true
    setAt(x.key)
  }
  // j / k move to the next or previous result anywhere in the list or the result shown (not
  // while typing); the arrows do too with the result itself selected.
  const moveKeys = (e: KeyboardEvent<HTMLDivElement>) => {
    if (typing(e.target) || e.ctrlKey || e.metaKey || e.altKey || flat.length === 0) return
    const onDetail = (e.target as HTMLElement).id === detailId
    const down = e.key === 'j' || (onDetail && e.key === 'ArrowDown')
    const up = e.key === 'k' || (onDetail && e.key === 'ArrowUp')
    if (!down && !up) return
    e.preventDefault()
    e.stopPropagation()
    pick(flat[down ? Math.min(chosenAt + 1, flat.length - 1) : Math.max(chosenAt - 1, 0)])
  }
  // After a decision the screen moves on to the next open finding, unless the user turned
  // that off; a short note says what was recorded.
  async function decideAndMove(decide: DecideFn, id: string, kind: DecisionKind, note: string) {
    await decide(id, kind, note)
    const decided = flat.find((x) => x.finding?.finding_id === id)
    setSaid(`${decisionText[kind]} is recorded for ${decided?.ruleId ?? 'the finding'}.`)
    if (!autoNext) return
    const after = [...flat.slice(chosenAt + 1), ...flat.slice(0, chosenAt)]
    const next = after.find((x) => x.finding && x.finding.finding_id !== id && decisionState(decisions, x.finding) === 'open')
    if (next) pick(next)
  }
  const counts = (s: string) => others.filter((x) => x.status === s).length
  // A run that only shows totals (a sales and a purchase register, with no return to match
  // them against) runs no check and has no results.
  const noChecks = r.totals.length > 0 && r.counts.checks === 0 && r.not_selected.length === 0

  if (part === 'documents') {
    return <RunDocuments run={run} nav={nav} open={open} setOpen={setOpen} table={table} />
  }
  const noun = f.status === 'fail' ? 'Finding' : 'Result'
  const chosenFinding = chosen?.finding
  const canDelete = onDeleteFinding && chosenFinding && !chosenFinding.deleted && !deleted.includes(chosenFinding.finding_id)
  return (
    <>
      {r.totals.length > 0 && <TotalsTable totals={r.totals} />}
      {noChecks ? (
        <p className="notice" role="note">
          No checks ran on these files: the totals above are shown side by side and give no findings.
        </p>
      ) : (
        <>
          <div className="cf-filterbar" role="group" aria-label="Result filters" onKeyDown={clearOnEscape}>
            <div className="cf-segments" role="group" aria-label="Result filter">
              {segments.map(([s, text]) => (
                <button key={text} type="button" aria-pressed={f.status === s} onClick={() => setF({ ...f, status: s })}>
                  {text}
                  {s !== '' && <>{' '}<span className={s === 'fail' ? 'cf-seg-count fail' : 'cf-seg-count'}>{counts(s)}</span></>}
                </button>
              ))}
            </div>
            <Meaning />
            <input type="search" className="cf-search" aria-label="Search results" placeholder="Rule ID or name" value={f.search}
              onChange={set('search')} />
            {onDecide && (
              <select aria-label="Decision filter" value={decisionFilter} onChange={(e) => setDecisionFilter(e.target.value)}>
                <option value="">Any decision</option>
                {Object.entries(stateText).map(([k, t]) => (
                  <option key={k} value={k}>{t}</option>
                ))}
              </select>
            )}
            <select aria-label="Document filter" value={f.document} onChange={set('document')}>
              <option value="">All documents</option>
              {options('document', (x) => x.documents).map((v) => (
                <option key={v} value={v}>{v}</option>
              ))}
            </select>
            <select aria-label="Category filter" value={f.category} onChange={set('category')}>
              <option value="">All categories</option>
              {options('category', (x) => [x.category]).map((v) => (
                <option key={v} value={v}>{v}</option>
              ))}
            </select>
            {active && (
              <button type="button" className="ghost" onClick={clear} title="Clear the filters (Esc)">
                Clear
              </button>
            )}
          </div>
          <p className="cf-result-count" aria-live="polite">
            {shown.length} of {rows.length} results
          </p>
          {said && <p className="cf-toast" role="status">{said}</p>}

          {flat.length === 0 &&
            (active || f.status !== 'fail' ? (
              <p className="empty">Nothing here{active ? ' matches the filters' : ''}.</p>
            ) : (
              <p className="empty">
                <CheckIcon /> No findings. No disagreements found across {files.length} {files.length === 1 ? 'document' : 'documents'}.
              </p>
            ))}
          {chosen && chosenGroup && (
            <div className="desk cf-desk" onKeyDown={moveKeys}>
              <ResultIndex groups={groups} decisions={onDecide ? decisions : undefined} at={chosen.key} onPick={pick} />
              <div className="cf-detail-panel">
                <div className="cf-detail-head">
                  <span className="cf-detail-doc"><DocIcon /> {chosenGroup.heading}</span>
                  <span className="cf-steps" role="group" aria-label="Move between findings">
                    <span className="muted">
                      {noun} {chosenAt + 1} of {flat.length}
                    </span>
                    <button type="button" className="ghost icon-btn" aria-label="Previous" title="Previous (k)"
                      onClick={() => pick(flat[chosenAt - 1])} disabled={chosenAt === 0}>
                      <ChevronIcon dir="left" />
                    </button>
                    <button type="button" className="ghost icon-btn" aria-label="Next" title="Next (j)"
                      onClick={() => pick(flat[chosenAt + 1])} disabled={chosenAt === flat.length - 1}>
                      <ChevronIcon dir="right" />
                    </button>
                  </span>
                  {canDelete && (
                    <FindingMenu f={chosenFinding} onDelete={async (reason) => {
                      await onDeleteFinding(chosenFinding, reason)
                      setDeleted([...deleted, chosenFinding.finding_id])
                    }} />
                  )}
                </div>
                {chosenFinding ? (
                  <FindingCard key={chosenFinding.finding_id} f={chosenFinding} deletedNow={deleted.includes(chosenFinding.finding_id)}
                    decision={onDecide ? {
                      d: decisions[chosenFinding.finding_id],
                      onDecide: (id, kind, note) => decideAndMove(onDecide, id, kind, note),
                      autoNext: { on: autoNext, set: setAutoNext },
                    } : undefined} />
                ) : chosen.cannot ? (
                  <CannotCard c={chosen.cannot} taxYear={r.tax_year} />
                ) : chosen.passed ? (
                  <PassCard p={chosen.passed} />
                ) : null}
              </div>
            </div>
          )}

          {r.not_selected.length > 0 && (
            <Section title={`Not run: not selected (${r.not_selected.length})`}>
              <ul className="not-run" aria-label="Rules not run">
                {r.not_selected.map((n) => (
                  <li key={n.rule_id}>
                    <span className="tag muted">Not run</span> <strong>[{n.rule_id}]</strong> {n.title}
                  </li>
                ))}
              </ul>
            </Section>
          )}
        </>
      )}
    </>
  )
}

// RunDocuments is the Documents view of a type of check (Recon Screen Document v2 §8.1): four
// totals, the documents it reads, and every document read with how much of it was read.
// Up and down choose a document; Enter opens what was read from it in a drawer.
function RunDocuments({ run, nav, open, setOpen, table }: Pick<RunViewProps, 'run' | 'nav' | 'open' | 'setOpen' | 'table'>) {
  const files = run.report.files
  const docs = run.facts.engagements.flatMap((e) => e.documents)
  const openFile = open === null ? undefined : files[open]
  // A file that holds several documents (a bundle, SPEC-005) has one row per document.
  const openDoc = openFile
    ? docs.find((d) => d.file_name === openFile.file && (!openFile.document_type || d.document_type === openFile.document_type))
    : undefined
  const shared = (fr: { file: string }) => files.filter((x) => x.file === fr.file).length > 1
  const fileLabel = (fr: { file: string; document_type: string }) =>
    shared(fr) && fr.document_type ? `${fr.file}, ${fr.document_type.replace(/_/g, ' ')}` : fr.file
  const sum = (pick: (fr: (typeof files)[number]) => number) => files.reduce((n, fr) => n + pick(fr), 0)
  const totals: [number, string][] = [
    [new Set(files.map((fr) => fr.file)).size, 'Files uploaded'],
    [files.filter((fr) => fr.profile_id !== '').length, 'Documents recognised'],
    [sum((fr) => fr.fact_count + fr.record_count), 'Figures and rows read by the checks'],
    [sum((fr) => fr.unread.length), 'Values no check uses yet'],
  ]
  return (
    <>
      <ul className="cf-doc-totals" aria-label="What was read">
        {totals.map(([n, label]) => (
          <li key={label}>
            <b>{n}</b> <span>{label}</span>
          </li>
        ))}
      </ul>
      <DocumentRail run={run} />
      <div className="table-wrap">
      <table ref={table} className="grid table-premium cf-docs" tabIndex={0} onKeyDown={nav.onKeyDown} aria-label="Documents">
        <thead>
          <tr>
            <th>File</th>
            <th>Recognised as</th>
            <th>Tax year</th>
            <th className="num">Figures / rows read</th>
            <th className="num">Not read</th>
            <th>Coverage</th>
            <th>Problems</th>
          </tr>
        </thead>
        <tbody>
          {files.map((fr, i) => {
            const read = fr.fact_count + fr.record_count
            const all = read + fr.unread.length
            return (
              <tr
                key={fr.file + '|' + fr.document_type}
                className={i === nav.selected ? 'selected' : undefined}
                aria-selected={i === nav.selected}
                onClick={() => {
                  nav.setSelected(i)
                  setOpen(open === i ? null : i)
                }}
              >
                <td className="client-name">{fileLabel(fr)}</td>
                <td>
                  {fr.source_system} <span className="muted mono">({fr.profile_id} v{fr.profile_version})</span>
                </td>
                <td className="nowrap">{fr.tax_year || '-'}</td>
                <td className="num">{readText(fr.fact_count, fr.record_count)}</td>
                <td className="num">{fr.unread.length}</td>
                <td>
                  <span className="cf-coverage" title={`${read} read, ${fr.unread.length} not read`}>
                    <span style={{ width: `${all === 0 ? 0 : (read * 100) / all}%` }} />
                  </span>
                </td>
                <td>
                  {fr.errors.length === 0 && fr.warnings.length === 0 && '-'}
                  {fr.errors.map((e) => (
                    <div key={e.code + e.path + e.message} className="fail-text">
                      {e.code} at {e.path || '-'}: {e.message}
                    </div>
                  ))}
                  {fr.warnings.map((w) => (
                    <div key={w.code + w.path + w.message}>
                      {w.code} at {w.path || '-'}: {w.message}
                    </div>
                  ))}
                  {fr.errors.length > 0 && <div className="muted">The figures of this file are not used, because it has errors.</div>}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
      </div>
      <p className="hint">
        "Not read" counts single values in the file that no check uses; the bar shows what was read against what was not.
        Choose a document and press Enter (or click it) to see what was read from it, what was not, and why.
      </p>

      {openFile && (
        <Modal drawer title={`Read from ${fileLabel(openFile)}`} onClose={() => setOpen(null)}>
          <ReadDrawer file={openFile} doc={openDoc} />
        </Modal>
      )}
    </>
  )
}

// ReadDrawer lists what was read from one document and what was not, and why, with a filter
// (everything, read, not read) and a search over names, places and reasons.
function ReadDrawer({ file, doc }: { file: RunReport['files'][number]; doc?: StoredDocument }) {
  const [which, setWhich] = useState<'all' | 'read' | 'unread'>('all')
  const [q, setQ] = useState('')
  const has = (...texts: string[]) => q.trim() === '' || texts.some((t) => t.toLowerCase().includes(q.trim().toLowerCase()))
  const facts = (doc?.facts ?? []).filter((x) => has(x.name, x.period, x.locator.path))
  const records = (doc?.records ?? []).filter((x) => has(x.match_key, x.locator.path, ...Object.values(x.fields)))
  const unread = file.unread.filter((u) => has(u.path, u.reason))
  const nothing = !doc || (doc.facts.length === 0 && doc.records.length === 0)
  const choices: [typeof which, string][] = [['all', 'Everything'], ['read', 'Read'], ['unread', 'Not read']]
  return (
    <div className="cf-drawer-body">
      <p className="muted">
        {file.source_system} · {file.profile_id} v{file.profile_version}
        {file.tax_year ? ` · tax year ${file.tax_year}` : ''}
      </p>
      <div className="cf-drawer-tools">
        <input type="search" aria-label="Search what was read" placeholder="Name, place in the file or reason" value={q}
          onChange={(e) => setQ(e.target.value)} />
        <div className="cf-segments" role="group" aria-label="Show">
          {choices.map(([k, t]) => (
            <button key={k} type="button" aria-pressed={which === k} onClick={() => setWhich(k)}>{t}</button>
          ))}
        </div>
      </div>
      {which !== 'unread' && (
        <>
          {nothing && (
            <p>
              Nothing was stored from this file
              {file.errors.length > 0 ? ': it has errors, listed in the Documents table.' : '.'}
            </p>
          )}
          {records.length > 0 && <RowsTable records={records} />}
          {facts.length > 0 && (
            <table className="grid compact table-premium" aria-label="Figures read">
              <thead>
                <tr>
                  <th>Figure</th>
                  <th>Period</th>
                  <th className="num">Value</th>
                  <th>Source</th>
                </tr>
              </thead>
              <tbody>
                {facts.map((fact) => (
                  <tr key={fact.name + fact.period + fact.scope}>
                    <td>{fact.name}</td>
                    <td>{fact.period}</td>
                    <td className="num">{formatValue(fact.value)}</td>
                    <td>{locatorText(fact.locator)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          {!nothing && facts.length === 0 && records.length === 0 && <p className="muted">Nothing read matches the search.</p>}
        </>
      )}
      {which !== 'read' &&
        (unread.length === 0 && file.unread.length > 0 ? (
          <p className="muted">Nothing that was not read matches the search.</p>
        ) : (
          <NotRead unread={unread} />
        ))}
    </div>
  )
}

// ResultIndex lists the results shown, by document, beside the one shown in full: each with
// its rule, the engine's difference and its decision (or Open). A group folds away under its
// heading, which says how many of its findings are open and how many it holds.
function ResultIndex({ groups, decisions, at, onPick }: {
  groups: ResultGroup[]
  decisions?: Record<string, FindingDecision> // a client in the database: decisions are shown
  at: string
  onPick: (x: ResultRow) => void
}) {
  return (
    <nav className="cf-index" aria-label="Findings by document to fix">
      {groups.map((g) => {
        const findings = g.rows.filter((x) => x.finding)
        const open = decisions ? findings.filter((x) => x.finding && decisionState(decisions, x.finding) === 'open').length : 0
        return (
          <details key={g.heading} open className="cf-group">
            <summary>
              <DocIcon />
              <span className="cf-group-name" title={g.heading}>{g.heading}</span>
              <span className="cf-group-count">
                {decisions && findings.length > 0 ? <><span className="fail-text">{open} open</span> · {g.rows.length}</> : g.rows.length}
              </span>
            </summary>
            <ul>
              {g.rows.map((x) => (
                <li key={x.key}>
                  <button type="button" onClick={() => onPick(x)} aria-current={at === x.key}>
                    <IndexLine x={x} decisions={decisions} />
                  </button>
                </li>
              ))}
            </ul>
          </details>
        )
      })}
      <p className="cf-index-keys" aria-hidden="true">
        <kbd>j</kbd> <kbd>k</kbd> next / previous
      </p>
    </nav>
  )
}

// IndexLine is one result in the list: rule (and invoice), the difference when it is an
// amount, the title, and the decision, Open, or the kind of result.
function IndexLine({ x, decisions }: { x: ResultRow; decisions?: Record<string, FindingDecision> }) {
  const f = x.finding
  const money = f && f.values.length > 0 && f.values[0].value.kind === 'money' && f.show_difference
  const state = f && decisions ? decisionState(decisions, f) : ''
  return (
    <>
      <span className="cf-line-top">
        <span className="rid">{[x.ruleId, f?.subject.match_key].filter(Boolean).join(' · ')}</span>
        {money && <span className="cf-line-amount">{formatPaise(f.difference_paise)}</span>}
      </span>
      <span className="t">{x.title}</span>
      <span className="cf-line-state">
        {x.status === 'cannot_check' ? (
          <span className="tag note">Cannot check</span>
        ) : x.status === 'pass' ? (
          <span className="tag pass">Pass</span>
        ) : state && state !== 'open' ? (
          <span className={state === 'not_an_issue' ? 'tag pass' : state === 'ask_client' ? 'tag note' : 'tag fail'}>{stateText[state]}</span>
        ) : state === 'open' ? (
          <span className="tag fail">Open</span>
        ) : f?.code_text ? (
          <span className="tag fail">{f.code_text}</span>
        ) : money ? null : (
          <span className="tag fail">{statusText.fail}</span>
        )}
      </span>
    </>
  )
}

// Section is a part of the results that folds away under its heading, which carries the
// count. It starts closed; a click, Enter or Space on the heading opens it.
function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <details className="results-section">
      <summary>
        <h4>{title}</h4>
      </summary>
      <div className="results-section-body">{children}</div>
    </details>
  )
}

// readText says what was read from a file: single figures, rows (GST invoices), or both.
function readText(facts: number, rows: number): string {
  const rowText = `${rows} ${rows === 1 ? 'row' : 'rows'}`
  if (rows === 0) return String(facts)
  return facts === 0 ? rowText : `${facts} + ${rowText}`
}

// Meaning explains the words of the results, in plain language. Its "?" opens the words
// under the filter line.
function Meaning() {
  return (
    <details className="meaning cf-meaning">
      <summary title='What do "result", "finding" and "cannot check" mean?'>
        <span aria-hidden="true">?</span>
        <span className="cf-sr-only">What do "result", "finding" and "cannot check" mean?</span>
      </summary>
      <dl>
        <dt>Result</dt>
        <dd>
          What one check gave on the documents uploaded. Each check compares a figure or an invoice in one document with
          the same figure or invoice in another, and its result is one of: Pass, Fail or Cannot check.
        </dd>
        <dt>Pass</dt>
        <dd>The two documents agree, within the tolerance the rule allows (for example Rs 1).</dd>
        <dt>Fail, a finding</dt>
        <dd>
          The two documents disagree. A finding shows both values, the difference between them and where each value came
          from (file and place in the file). The tool says only that the documents differ; which document to change is
          for the reviewer to decide.
        </dd>
        <dt>Cannot check</dt>
        <dd>
          A figure or row the check needs is not in the documents uploaded (for example GSTR-2B was not uploaded), so the
          check could not be made. It is never counted as a pass.
        </dd>
        <dt>Not run</dt>
        <dd>A rule of the type of check that was left unticked for this run.</dd>
      </dl>
    </details>
  )
}

// NotRead lists what was not read from a file and why, grouped: rows the format leaves
// out (with the condition each failed), columns of rows that were read which no check uses
// yet, and other parts of the file.
function NotRead({ unread }: { unread: Unread[] }) {
  if (unread.length === 0) return <p className="muted">Everything in this file was read.</p>
  const general = (p: string) => p.replace(/\[\d+\]/g, '[*]')
  // Rows left out, grouped by reason.
  const left = new Map<string, Set<string>>()
  // Columns not used, by column (path inside the row), with how many values.
  const columns = new Map<string, number>()
  // Other parts of the file, by top-level part.
  const parts = new Map<string, number>()
  for (const u of unread) {
    if (u.kind === 'row_left_out') {
      const why = u.reason.replace(/^row left out: /, '')
      left.set(why, (left.get(why) ?? new Set()).add(u.row))
    } else if (u.kind === 'column_not_used' && u.row) {
      const col = general(u.path.slice(u.row.length)).replace(/^\./, '')
      columns.set(col, (columns.get(col) ?? 0) + 1)
    } else {
      const m = /^\$(\.[^.[]+|\['[^']*'\])?/.exec(u.path)
      const part = m?.[0] ?? u.path
      parts.set(part, (parts.get(part) ?? 0) + 1)
    }
  }
  const rowsLeft = [...left.values()].reduce((n, s) => n + s.size, 0)
  return (
    <div className="not-read" aria-label="Not read, and why">
      <h4>Not read, and why ({unread.length} values)</h4>
      <p className="muted">
        Nothing here is used by any check, and nothing is guessed from it. The format reads only the figures and rows its
        checks compare.
      </p>
      {left.size > 0 && (
        <>
          <h5>Rows left out ({rowsLeft})</h5>
          <ul>
            {[...left.entries()].map(([why, rows]) => (
              <li key={why}>
                {rows.size} {rows.size === 1 ? 'row' : 'rows'}: {why}{' '}
                <span className="muted">({[...rows].sort().join(', ')})</span>
              </li>
            ))}
          </ul>
        </>
      )}
      {columns.size > 0 && (
        <>
          <h5>Columns of the rows read that no check uses yet ({columns.size})</h5>
          <p>
            {[...columns.entries()]
              .sort(([a], [b]) => a.localeCompare(b))
              .map(([col, n]) => `${col} (${n})`)
              .join(' · ')}
          </p>
        </>
      )}
      {parts.size > 0 && (
        <>
          <h5>Other parts of the file this format does not read ({parts.size})</h5>
          <ul>
            {[...parts.entries()]
              .sort(([a], [b]) => a.localeCompare(b))
              .map(([part, n]) => (
                <li key={part}>
                  {part} <span className="muted">({n} {n === 1 ? 'value' : 'values'})</span>
                </li>
              ))}
          </ul>
        </>
      )}
    </div>
  )
}

// RowsTable lists the rows read from a document (GST invoices), each with its columns,
// amounts to the paisa and the place in the file it came from.
const rowAmountOrder = ['taxable_value', 'igst', 'cgst', 'sgst', 'cess', 'invoice_value']
function RowsTable({ records }: { records: StoredRecord[] }) {
  const amounts = [...new Set(records.flatMap((r) => Object.keys(r.amounts)))].sort((a, b) => {
    const ia = rowAmountOrder.indexOf(a)
    const ib = rowAmountOrder.indexOf(b)
    return (ia < 0 ? 99 : ia) - (ib < 0 ? 99 : ib) || a.localeCompare(b)
  })
  const fields = [...new Set(records.flatMap((r) => Object.keys(r.fields)))].sort()
  const label = (k: string) => columnText[k] ?? k.replace(/_/g, ' ')
  return (
    <>
      <p className="muted">
        {records.length} {records.length === 1 ? 'row' : 'rows'} read
      </p>
      <div className="scroll">
        <table className="grid compact table-premium" aria-label="Rows read">
          <thead>
            <tr>
              {fields.map((k) => (
                <th key={k}>{label(k)}</th>
              ))}
              {amounts.map((k) => (
                <th key={k} className="num">
                  {label(k)}
                </th>
              ))}
              <th>Source</th>
            </tr>
          </thead>
          <tbody>
            {records.map((r) => (
              <tr key={r.locator.path + r.match_key}>
                {fields.map((k) => (
                  <td key={k}>{r.fields[k] ?? ''}</td>
                ))}
                {amounts.map((k) => (
                  <td key={k} className="num">
                    {k in r.amounts ? formatPaise(r.amounts[k]) : ''}
                  </td>
                ))}
                <td>{locatorText(r.locator)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  )
}

// TotalsTable shows each document's rows added up, side by side. They are shown, not
// compared: a totals comparison (sales vs purchases) gives no finding.
const totalColumns = ['taxable_value', 'igst', 'cgst', 'sgst', 'cess']
const columnText: Record<string, string> = {
  taxable_value: 'Taxable value', igst: 'IGST', cgst: 'CGST', sgst: 'SGST/UTGST', cess: 'Cess',
}

function TotalsTable({ totals }: { totals: DocTotal[] }) {
  return (
    <div className="panel" aria-label="Totals">
      <h4>Totals (shown side by side, not compared)</h4>
      <div className="scroll">
      <table className="grid compact table-premium">
        <thead>
          <tr>
            <th />
            {totals.map((t) => (
              <th key={t.document_type} className="num">{t.document}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>Rows (lines)</td>
            {totals.map((t) => (
              <td key={t.document_type} className="num">{t.rows}</td>
            ))}
          </tr>
          {totalColumns.map((c) => (
            <tr key={c}>
              <td>{columnText[c]}</td>
              {totals.map((t) => (
                <td key={t.document_type} className="num">{formatPaise(t.amounts[c] ?? 0)}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      </div>
      <p className="muted">The invoice value is left out: books repeat it on every line of an invoice.</p>
    </div>
  )
}

// RowTable shows a row-matching result: for each column, the value in each document and,
// for amounts, the difference.
function RowTable({ f }: { f: Finding }) {
  const order: string[] = []
  const cells: Record<string, { a?: Finding['values'][number]; b?: Finding['values'][number] }> = {}
  for (const v of f.values) {
    const [side, ...rest] = v.input.split('.')
    const col = rest.join('.')
    const label = rest.length > 1 ? v.label : v.label.split(', ').slice(1).join(', ') || col
    if (!cells[label]) {
      cells[label] = {}
      order.push(label)
    }
    cells[label][side === 'a' ? 'a' : 'b'] = v
  }
  const sides = [f.values.find((v) => v.input.startsWith('a.')), f.values.find((v) => v.input.startsWith('b.'))]
  const heading = (v: Finding['values'][number] | undefined, fallback: string) => (v ? v.label.split(', ')[0] : fallback)
  return (
    <div className="scroll">
    <table className="grid compact rows table-premium">
      <thead>
        <tr>
          <th />
          <th className="num">{heading(sides[0], 'First document')}</th>
          <th className="num">{heading(sides[1], 'Second document')}</th>
          <th className="num">Difference</th>
        </tr>
      </thead>
      <tbody>
        {order.map((l) => {
          const { a, b } = cells[l]
          const money = a?.value.kind === 'money' && b?.value.kind === 'money'
          const diff = money && a && b && a.value.kind === 'money' && b.value.kind === 'money' ? Math.abs(a.value.paise - b.value.paise) : null
          return (
            <tr key={l}>
              <td>{l}</td>
              <td className="num">{a ? formatValue(a.value) : '-'}</td>
              <td className="num">{b ? formatValue(b.value) : '-'}</td>
              <td className={diff ? 'num diff' : 'num'}>{diff === null ? '' : formatPaise(diff)}</td>
            </tr>
          )
        })}
      </tbody>
    </table>
    </div>
  )
}

// lawText writes a rule's legal basis as the engine report does.
function lawText(f: Finding): string {
  return f.law_refs.map((l) => `${l.act} – ${/^[0-9]/.test(l.section) ? 'Sec ' : ''}${l.section}${l.reference ? ` (${l.reference})` : ''}`).join('; ')
}

// CopyIcon is two sheets, for the button that copies a figure's place in its file.
const CopyIcon = () => (
  <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"
    strokeLinejoin="round" aria-hidden="true">
    <rect x="5.5" y="5.5" width="8" height="8" rx="1.5" />
    <path d="M10.5 5.5V3.75c0-.7-.55-1.25-1.25-1.25h-5.5c-.7 0-1.25.55-1.25 1.25v5.5c0 .7.55 1.25 1.25 1.25H5.5" />
  </svg>
)

// CopyPlace copies "file place" so it can be pasted into a note or a search. The browser
// gives the clipboard only to a secure page (localhost); elsewhere the button is left out
// and the text can still be selected.
function CopyPlace({ l, what }: { l: Locator; what: string }) {
  const [done, setDone] = useState(false)
  if (!window.isSecureContext || !navigator.clipboard) return null
  return (
    <button type="button" className="ghost icon-btn cf-copy" aria-label={done ? `Copied the place of ${what}` : `Copy the place of ${what}`}
      title="Copy the file and place" onClick={() => {
        navigator.clipboard.writeText(locatorText(l)).then(() => setDone(true), () => setDone(false))
      }}>
      {done ? <CheckIcon /> : <CopyIcon />}
    </button>
  )
}

// diffLabel names the difference row by the subtraction it is, when it is exactly A − B or
// B − A of the two figures shown; otherwise it is the engine's difference, named plainly.
function diffLabel(f: Finding): string {
  if (f.values.length !== 2) return 'Difference'
  const [a, b] = f.values.map((v) => (v.value.kind === 'money' ? v.value.paise : null))
  if (a === null || b === null) return 'Difference'
  if (a - b === f.difference_paise) return 'Difference (A − B)'
  if (b - a === f.difference_paise) return 'Difference (B − A)'
  return 'Difference'
}

// FigureTable shows a finding's figures as rows of one table, read top to bottom like a
// working paper (Recon Screen Document v2 §7.3): A, B… each with its label, the file and place
// it comes from, and its amount; the last row is the engine's difference. A value the check
// computed (a sum, a law table's value) has no file. It never says which figure is right.
function FigureTable({ f }: { f: Finding }) {
  const money = f.values.length > 0 && f.values[0].value.kind === 'money' && f.show_difference
  const allMoney = f.values.every((v) => v.value.kind === 'money')
  return (
    <table className="cf-figures" aria-label="Figures of this finding">
      <thead>
        <tr>
          <th><span className="cf-sr-only">Row</span></th>
          <th>Figure</th>
          <th>Where it comes from</th>
          <th className="num">{allMoney ? 'Amount (₹)' : 'Value'}</th>
        </tr>
      </thead>
      <tbody>
        {f.values.map((v, i) => {
          const letter = String.fromCharCode(65 + i)
          return (
            <tr key={v.input}>
              <td><span className="cf-letter">{letter}</span></td>
              <td className="cf-figure">{v.label}</td>
              <td className="cf-where">
                {v.locator.file_name !== '' ? (
                  <span className="src-line">
                    <span className="cf-src-file"><DocIcon /> {v.locator.file_name}</span>{' '}
                    <code>{v.locator.path}</code>
                    <CopyPlace l={v.locator} what={letter} />
                  </span>
                ) : (
                  <span className="muted">Not from a file</span>
                )}
              </td>
              <td className="num cf-amount">{formatValue(v.value)}</td>
            </tr>
          )
        })}
        {money && (
          <tr className="cf-diff">
            <td><span className="cf-letter" aria-hidden="true">≠</span></td>
            <td className="cf-figure" colSpan={2}>{diffLabel(f)}</td>
            <td className="num cf-amount">{formatPaise(f.difference_paise)}</td>
          </tr>
        )}
      </tbody>
    </table>
  )
}

// FindingCard shows one finding as the engine report does: every figure with its label and
// source, the difference, and the rule basis, then the decision. With the finding selected
// (not while typing a note), f / x / a apply Fix / Not an issue / Ask client.
function FindingCard({ f, decision, deletedNow }: {
  f: Finding
  decision?: { d?: FindingDecision; onDecide: DecideFn; autoNext: { on: boolean; set: (on: boolean) => void } }
  deletedNow: boolean
}) {
  const act = useRef<((k: DecisionKind) => void) | null>(null)
  const decided = decision?.d && decision.d.state !== 'open'
  const keys = (e: KeyboardEvent<HTMLElement>) => {
    if (!decision || typing(e.target) || e.ctrlKey || e.metaKey || e.altKey) return
    const kind = (Object.keys(decisionKeys) as DecisionKind[]).find((k) => decisionKeys[k] === e.key)
    if (!kind) return
    e.preventDefault()
    e.stopPropagation()
    act.current?.(kind)
  }
  // A value the check computed (a sum, a percentage, a law table's value) has no source.
  const sources = [...new Set(f.values.filter((v) => v.locator.file_name !== '').map((v) => locatorText(v.locator)))]
  const label = f.code ? `${f.rule_id} ${f.title} ${f.subject.match_key}` : `${f.rule_id} ${f.title}`
  return (
    <article id={detailId} tabIndex={0} className={decided ? 'finding cf-finding decided' : 'finding cf-finding'} aria-label={label}
      onKeyDown={keys}>
      <header className="cf-finding-head">
        <span className="cf-rule-id">{f.rule_id}</span>
        <h3>{f.title}</h3>
        <ul className="cf-finding-chips">
          {f.subject.period && <li className="chip">{f.subject.period}</li>}
          {f.subject.match_key && <li className="chip mono">{f.subject.match_key}</li>}
          {f.category && <li className="chip">{f.category}</li>}
          {f.code && <li className="tag fail">{f.code_text}</li>}
        </ul>
      </header>
      {f.code ? (
        <>
          <RowTable f={f} />
          <p className="basis">Source: {sources.join(' | ')}</p>
        </>
      ) : (
        <FigureTable f={f} />
      )}
      <p className="basis cf-basis">Rule basis: {lawText(f)}</p>
      {f.deleted && <p className="basis deleted">Deleted by {f.deleted.by} on {f.deleted.at}: {f.deleted.reason}</p>}
      {deletedNow && <p className="basis deleted">Deleted. The finding is kept; the other roles no longer see it.</p>}
      {decision && (
        <DecisionPanel f={f} d={decision.d} onDecide={decision.onDecide} actRef={act} autoNext={decision.autoNext} />
      )}
    </article>
  )
}

// CannotCard shows a Cannot check result: what the check needed and could not find, and the
// documents to upload so it can run. It is never counted as a pass.
function CannotCard({ c, taxYear }: { c: CannotCheck; taxYear: string }) {
  const docs = [...new Set(c.missing.map((m) => m.document))]
  return (
    <section id={detailId} tabIndex={0} className="cf-finding cf-cannot-card" aria-label={`${c.rule_id} ${c.title}`}>
      <header className="cf-finding-head">
        <span className="cf-rule-id">{c.rule_id}</span>
        <h3>{c.title}</h3>
        <ul className="cf-finding-chips">
          {c.subject.period && <li className="chip">{c.subject.period}</li>}
          <li className="tag note">Cannot check</li>
        </ul>
      </header>
      <h4>Why</h4>
      <ul className="cf-why">
        {c.missing.map((m) => (
          <li key={m.fact}>
            Cannot check: {m.rows ? 'rows' : 'figure'} not found in {m.document}: {m.label}{' '}
            <span className="muted">({m.fact})</span>
          </li>
        ))}
      </ul>
      <h4>What to upload</h4>
      <p>
        {docs.join(', ')} for tax year {taxYear}, holding the {c.missing.length === 1 ? 'value' : 'values'} above. Then run
        the checks again.
      </p>
    </section>
  )
}

// PassCard shows a passed check: the rule and that the documents agree. The engine keeps no
// figures for a pass, so none are shown.
function PassCard({ p }: { p: Passed }) {
  return (
    <section id={detailId} tabIndex={0} className="cf-finding" aria-label={`${p.rule_id} ${p.title}`}>
      <header className="cf-finding-head">
        <span className="cf-rule-id">{p.rule_id}</span>
        <h3>{p.title}</h3>
        <ul className="cf-finding-chips">
          {p.subject.period && <li className="chip">{p.subject.period}</li>}
          {p.category && <li className="chip">{p.category}</li>}
          <li className="tag pass">Pass</li>
        </ul>
      </header>
      <p>The documents agree for this check, within the tolerance the rule allows.</p>
    </section>
  )
}

// FindingMenu is the … menu of the finding shown (SPEC-013 §4). Its one action, Delete
// finding, asks for a reason in a dialog; the reason is kept in the audit trail. Esc closes
// the menu, then the dialog, one layer at a time.
function FindingMenu({ f, onDelete }: { f: Finding; onDelete: (reason: string) => Promise<void> }) {
  const [menu, setMenu] = useState(false)
  const [asking, setAsking] = useState(false)
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (busy || reason.trim() === '') return
    setError('')
    setBusy(true)
    onDelete(reason.trim())
      .then(() => setAsking(false))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'The finding was not deleted.'))
      .finally(() => setBusy(false))
  }
  return (
    <span className="cf-menu" onKeyDown={(e) => {
      if (e.key === 'Escape' && menu) {
        e.stopPropagation()
        setMenu(false)
      }
    }} onBlur={(e) => {
      if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setMenu(false)
    }}>
      <button type="button" className="ghost icon-btn" aria-label="More actions on this finding" aria-expanded={menu}
        title="More actions" onClick={() => setMenu(!menu)}>
        …
      </button>
      {menu && (
        <span className="cf-menu-list">
          <button type="button" className="danger" onClick={() => {
            setMenu(false)
            setReason('')
            setError('')
            setAsking(true)
          }}>
            Delete finding
          </button>
        </span>
      )}
      {asking && (
        <Modal title={`Delete ${f.rule_id}?`} onClose={() => setAsking(false)} closeDisabled={busy}>
          <form className="cf-delete" aria-label="Delete this finding" onSubmit={submit}>
            <p>The finding is removed from this client's list. The reason, your name and the time are kept in the audit trail.</p>
            <label>
              Reason for deleting <span className="fail-text" aria-hidden="true">*</span>
              <textarea value={reason} maxLength={300} rows={3} required placeholder={`For example: duplicate of another ${f.rule_id.split('-').slice(0, 2).join('-')} finding`}
                onChange={(e) => setReason(e.target.value)} />
            </label>
            {error && <p className="error" role="alert">{error}</p>}
            <div className="cf-delete-actions">
              <button type="button" className="ghost" onClick={() => setAsking(false)} disabled={busy}>Cancel</button>
              <button type="submit" className="danger" disabled={busy || reason.trim() === ''}>Delete finding</button>
            </div>
          </form>
        </Modal>
      )}
    </span>
  )
}

// priorYear is the tax year before ty ("2025-26" gives "2024-25").
function priorYear(ty: string): string {
  const start = Number(ty.slice(0, 4)) - 1
  return `${start}-${String((start + 1) % 100).padStart(2, '0')}`
}
