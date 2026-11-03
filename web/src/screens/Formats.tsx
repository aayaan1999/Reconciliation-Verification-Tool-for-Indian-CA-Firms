import { useCallback, useEffect, useId, useMemo, useRef, useState, type FormEvent } from 'react'
import {
  ApiError, addParameter, changeAlias, exportFormat, fetchFormatMapping, fetchFormats, pickFormatVersion, publishFormat, removeKeptSample,
  resolveUnrecognised, switchFormat, switchParameter,
} from '../api.ts'
import type { DryRun, FormatInfo, FormatMapping, FormatsList, ParameterInfo, UnrecognisedInfo } from '../formatsTypes.ts'
import { FindFile, ImportDialog, UnrecognisedPanel } from './FormatsExtra.tsx'
import { DryRunView } from './DryRunView.tsx'
import { DocIcon, GapIcon, SearchIcon, SlidersIcon, UnknownFileIcon } from '../icons.tsx'
import { usePager } from '../pager.ts'
import type { LayoutMapping, TeachResult } from '../types.ts'
import { Modal } from './Modal.tsx'
import { PageBar } from './PageBar.tsx'
import { Field } from './RuleBuilder.tsx'
import { TeachLayout } from './TeachLayout.tsx'
import '../styles/formats.css'

// The Formats screen (SPEC-016), for the Partner. A format is a profile: how one kind of file is
// read. A parameter is a figure of the vocabulary that profiles write and rules read. The screen
// lists both, shows where each is used, switches office ones off and on with a reason, switches a
// format back to an earlier version, adds a parameter, and teaches a new format with the same
// mapping screen the Client file uses. The server checks and logs every change; this screen only
// asks for it and shows the answer.

const tabs = [
  { id: 'formats', name: 'Formats', icon: <DocIcon /> },
  { id: 'parameters', name: 'Parameters', icon: <SlidersIcon /> },
  { id: 'gaps', name: 'Gaps', icon: <GapIcon /> },
  { id: 'unrecognised', name: 'Unrecognised files', icon: <UnknownFileIcon /> },
] as const
type Tab = (typeof tabs)[number]['id']

const fileText: Record<string, string> = { json: 'JSON', xlsx: 'Excel', csv: 'CSV' }
const originText: Record<string, string> = { builtin: 'Standard', office: 'Office' }
const kindText: Record<string, string> = {
  money: 'Amount', int: 'Whole number', text: 'Text', date: 'Date', bool: 'Yes or no', rate: 'Rate', record: 'Rows',
}
const statusText: Record<string, string> = { draft: 'Draft', testing: 'Testing', active: 'Active', disabled: 'Disabled' }
const statusTone: Record<string, string> = { draft: 'muted', testing: 'note', active: 'pass', disabled: 'muted' }
const addableKinds = ['money', 'int', 'text', 'date', 'bool', 'rate']

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`
const when = (at: string) => at.slice(0, 16).replace('T', ' ')

function problem(err: unknown): string {
  if (err instanceof ApiError) return `${err.message} (${err.code})`
  return String(err)
}

const StatusTag = ({ status }: { status: string }) => (
  <span className={`tag ${statusTone[status] ?? 'muted'}`}>{statusText[status] ?? status}</span>
)

export function Formats({ canChange = true }: { canChange?: boolean }) {
  const [data, setData] = useState<FormatsList | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [tab, setTab] = useState<Tab>('formats')
  const [format, setFormat] = useState<string | null>(null)
  const [parameter, setParameter] = useState<string | null>(null)
  const [adding, setAdding] = useState(false)
  const [teaching, setTeaching] = useState<File | null>(null)
  // Editing an office format: its mapping is read, a sample file is chosen, then the mapping
  // screen opens on it (SPEC-016). Publishing asks for testing or active.
  const [edit, setEdit] = useState<{ mapping: FormatMapping; file: File | null } | null>(null)
  const [publishing, setPublishing] = useState<{ revision: number; version: string } | null>(null)
  // A file no format recognised: asked for again, then taught; or dismissed with a reason (SPEC-016).
  const [unrec, setUnrec] = useState<{ item: UnrecognisedInfo; file: File | null } | null>(null)
  const [dismissing, setDismissing] = useState<UnrecognisedInfo | null>(null)
  const [importing, setImporting] = useState(false)
  const picker = useRef<HTMLInputElement>(null)
  const base = useId()

  const load = useCallback(async () => {
    try {
      setData(await fetchFormats())
      setError(null)
    } catch (err) {
      setError(problem(err))
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    fetchFormats().then(
      (d) => { if (!cancelled) setData(d) },
      (err: unknown) => { if (!cancelled) setError(problem(err)) },
    )
    return () => { cancelled = true }
  }, [])

  // act runs one change; when the server accepts it the lists are read again. It returns the
  // server's problem for the dialog to show, or null when done.
  const act = useCallback(async (change: () => Promise<unknown>, done: string): Promise<string | null> => {
    try {
      await change()
      setNotice(done)
      await load()
      return null
    } catch (err) {
      return problem(err)
    }
  }, [load])

  const names = useMemo(() => new Map((data?.parameters ?? []).map((p) => [p.key, p.name])), [data])
  const open = data?.formats.find((f) => f.id === format) ?? null
  const openParam = data?.parameters.find((p) => p.key === parameter) ?? null
  const gapCount = data ? data.unsupplied.length + data.uncovered.length : 0
  const waiting = data?.unrecognised.length ?? 0

  return (
    <section aria-labelledby="formats-title" className="formats-screen">
      <div className="board-head">
        <div>
          <p className="eyebrow">Partner only</p>
          <h1 id="formats-title">Document formats</h1>
          <p className="muted fmt-lede">How the tool reads each kind of file, and the figures its rules read. Standard ones are read only; teach a new format from a sample file.</p>
        </div>
        {canChange && (
          <>
            <button type="button" onClick={() => setImporting(true)}>Import a format</button>
            <button type="button" className="action" onClick={() => picker.current?.click()}>+ Add a format</button>
            <input ref={picker} id={`${base}-file`} type="file" accept=".json,.xlsx,.csv" className="fmt-sr" tabIndex={-1}
              aria-label="Choose a sample file to teach a format"
              onChange={(e) => {
                const f = e.target.files?.[0]
                e.target.value = ''
                if (f) setTeaching(f)
              }} />
          </>
        )}
      </div>
      {error && <p className="error" role="alert">{error}</p>}
      {notice && <p className="notice" role="status">{notice}</p>}

      <div className="tabs fmt-tabs" role="tablist" aria-label="Formats sections">
        {tabs.map((t) => (
          <button key={t.id} id={`${base}-tab-${t.id}`} type="button" role="tab" aria-selected={tab === t.id}
            aria-controls={`${base}-panel`} onClick={() => setTab(t.id)}>
            {t.icon}
            {t.name}
            {t.id === 'gaps' && gapCount > 0 && <span className="fmt-badge" aria-label={`${gapCount} gaps`}>{gapCount}</span>}
            {t.id === 'unrecognised' && waiting > 0 && <span className="fmt-badge" aria-label={`${waiting} files waiting`}>{waiting}</span>}
          </button>
        ))}
      </div>

      <div id={`${base}-panel`} role="tabpanel" aria-labelledby={`${base}-tab-${tab}`}>
        {data === null && !error && <p className="loading">Loading the formats…</p>}
        {data !== null && tab === 'formats' && <FormatsPanel data={data} onOpen={setFormat} />}
        {data !== null && tab === 'parameters' && <ParametersPanel data={data} canChange={canChange} onOpen={setParameter} onAdd={() => setAdding(true)} />}
        {data !== null && tab === 'gaps' && <GapsPanel data={data} names={names} onTeach={() => picker.current?.click()} canChange={canChange} />}
        {data !== null && tab === 'unrecognised' && <UnrecognisedPanel data={data} canChange={canChange} onTeach={(item) => setUnrec({ item, file: null })} onDismiss={setDismissing} />}
      </div>

      {open && !edit && !publishing && (
        <FormatDetail f={open} names={names} canChange={canChange} onClose={() => setFormat(null)}
          onEdit={async () => {
            try {
              const m = await fetchFormatMapping(open.id)
              if (!m.editable) setError(m.reason)
              else setEdit({ mapping: m, file: null })
            } catch (err) {
              setError(problem(err))
            }
          }}
          onPublish={(revision, version) => setPublishing({ revision, version })}
          onExport={async () => {
            try {
              const file = await exportFormat(open.id)
              const url = URL.createObjectURL(file.blob)
              const a = document.createElement('a')
              a.href = url
              a.download = file.filename
              document.body.appendChild(a)
              a.click()
              a.remove()
              URL.revokeObjectURL(url)
              setNotice(`${open.name} is exported as ${file.filename}.`)
            } catch (err) {
              setError(problem(err))
            }
          }}
          onRemoveSample={(id, name) => act(() => removeKeptSample(id), `The kept sample ${name} is removed.`)}
          onSwitch={(action, reason) => act(() => switchFormat(open.id, action, reason), `${open.name} is ${action === 'disable' ? 'disabled' : 'back in testing'}.`)}
          onBack={(revision, version, reason) => act(() => pickFormatVersion(open.id, revision, reason), `${open.name} is back on ${version}, in testing.`)} />
      )}
      {openParam && (
        <ParameterDetail p={openParam} canChange={canChange} onClose={() => setParameter(null)}
          formatNames={new Map(data?.formats.map((f) => [f.id, f.name]))}
          onSwitch={(action, reason) => act(() => switchParameter(openParam.key, action, reason), `${openParam.name} is ${action === 'disable' ? 'disabled' : 'enabled again'}.`)}
          onAlias={(alias, remove) => act(() => changeAlias(openParam.key, alias, remove), remove ? `Alias "${alias}" removed.` : `Alias "${alias}" added.`)} />
      )}
      {adding && data && (
        <AddParameter documents={[...new Set(data.parameters.map((p) => p.document))].sort()} onClose={() => setAdding(false)}
          onAdd={async (p) => {
            const fault = await act(() => addParameter(p), `${p.name} is added.`)
            if (fault === null) setAdding(false)
            return fault
          }} />
      )}
      {open && edit && edit.file === null && (
        <EditSample name={open.name} onCancel={() => setEdit(null)} onFile={(file) => setEdit({ ...edit, file })} />
      )}
      {open && edit && edit.file && (
        <TeachLayout file={edit.file} canKeepSample={canChange} onCancel={() => setEdit(null)}
          edit={{ formatId: open.id, mapping: (edit.mapping.draft ?? edit.mapping.in_use)?.mapping ?? emptyMapping }}
          onSaved={(res: TeachResult) => {
            setEdit(null)
            setNotice(`Draft v${res.format_revision ?? ''} of ${open.name} is saved. It is not used until you publish it.`)
            void load()
          }} />
      )}
      {open && publishing && (
        <PublishDialog name={open.name} version={publishing.version} onClose={() => setPublishing(null)}
          run={async (status, confirm) => {
            try {
              const out = await publishFormat(open.id, publishing.revision, status, confirm)
              setNotice(`${open.name} ${publishing.version} is published as ${status}.`)
              await load()
              return { fault: null, changed: false, dry: out.dry_run ?? null }
            } catch (err) {
              return { fault: problem(err), changed: err instanceof ApiError && err.code === 'SAMPLES_CHANGED', dry: null }
            }
          }} />
      )}
      {importing && (
        <ImportDialog onClose={() => setImporting(false)}
          onImported={(plan) => {
            setImporting(false)
            setNotice(`${plan.format_name} is imported, in testing. Publish it as active when it reads your files as it should.`)
            void load()
          }} />
      )}
      {unrec && unrec.file === null && (
        <FindFile item={unrec.item} onCancel={() => setUnrec(null)} onFile={(file) => setUnrec({ ...unrec, file })} />
      )}
      {unrec && unrec.file && (
        <TeachLayout file={unrec.file} canKeepSample={canChange} onCancel={() => setUnrec(null)}
          onSaved={(res: TeachResult) => {
            const item = unrec.item
            setUnrec(null)
            setNotice(`Format saved as ${res.profile_id}. ${item.file_name} and files like it are now recognised.`)
            void resolveUnrecognised(item.id, 'taught', `Taught as ${res.profile_id}`).finally(() => load())
          }} />
      )}
      {dismissing && (
        <ReasonDialog title={`Dismiss ${dismissing.file_name}`} lede="The file is closed without a format. The reason is kept in the audit log." action="Dismiss" danger
          run={(reason) => act(() => resolveUnrecognised(dismissing.id, 'dismissed', reason), `${dismissing.file_name} is dismissed.`)}
          onClose={() => setDismissing(null)} />
      )}
      {teaching && (
        <TeachLayout file={teaching} canKeepSample={canChange} onCancel={() => setTeaching(null)}
          onSaved={(res: TeachResult) => {
            setTeaching(null)
            setNotice(`Format saved as ${res.profile_id}. Files in this layout are now recognised.`)
            void load()
          }} />
      )}
    </section>
  )
}

// ---------- Formats ----------

function FormatsPanel({ data, onOpen }: { data: FormatsList; onOpen: (id: string) => void }) {
  const [search, setSearch] = useState('')
  const [type, setType] = useState('')
  const [status, setStatus] = useState('')
  const [origin, setOrigin] = useState('')
  const q = search.trim().toLowerCase()
  const shown = data.formats.filter((f) =>
    (q === '' || f.name.toLowerCase().includes(q) || f.id.toLowerCase().includes(q) || f.document.toLowerCase().includes(q)) &&
    (type === '' || f.file_type === type) && (status === '' || f.status === status) && (origin === '' || f.source === origin))
  const pager = usePager(shown, 'formats', `${q}|${type}|${status}|${origin}`)
  const active = search !== '' || type !== '' || status !== '' || origin !== ''
  return (
    <>
      <div className="controls-premium fmt-filters" role="group" aria-label="Format filters">
        <label className="fmt-search">
          Search
          <span className="search-box">
            <SearchIcon />
            <input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Name, ID or document" aria-label="Search formats" />
          </span>
        </label>
        <label>File type
          <select value={type} onChange={(e) => setType(e.target.value)} aria-label="File type" className="filter-select-premium">
            <option value="">All types</option>
            {[...new Set(data.formats.map((f) => f.file_type))].sort().map((t) => <option key={t} value={t}>{fileText[t] ?? t}</option>)}
          </select>
        </label>
        <label>Status
          <select value={status} onChange={(e) => setStatus(e.target.value)} aria-label="Status" className="filter-select-premium">
            <option value="">All statuses</option>
            {['active', 'testing', 'disabled'].map((s) => <option key={s} value={s}>{statusText[s]}</option>)}
          </select>
        </label>
        <label>Origin
          <select value={origin} onChange={(e) => setOrigin(e.target.value)} aria-label="Origin" className="filter-select-premium">
            <option value="">All origins</option>
            <option value="builtin">Standard</option>
            <option value="office">Office</option>
          </select>
        </label>
        <button type="button" className="ghost" disabled={!active} onClick={() => { setSearch(''); setType(''); setStatus(''); setOrigin('') }}>Clear filters</button>
      </div>
      <p className="result-count" aria-live="polite">{shown.length} of {plural(data.formats.length, 'format', 'formats')}</p>
      {shown.length === 0 ? (
        <p className="empty">{data.formats.length === 0 ? 'No format is stored yet. Start the server once, and the standard formats are loaded.' : 'No format matches these filters. Choose Clear filters to see them all.'}</p>
      ) : (
        <div className="table-wrap">
          <table className="table-premium fmt-table" aria-label="Formats">
            <thead>
              <tr>
                <th scope="col">Format</th><th scope="col">File type</th><th scope="col">Document</th><th scope="col">Version</th>
                <th scope="col">Status</th><th scope="col">Figures the rules need</th><th scope="col">Last used</th><th scope="col"><span className="fmt-sr">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              {pager.rows.map((f) => (
                <tr key={f.id}>
                  <td><span className="fmt-name">{f.name}</span><span className="fmt-sub muted">{originText[f.source] ?? f.source}</span></td>
                  <td>{fileText[f.file_type] ?? f.file_type}</td>
                  <td>{f.document}</td>
                  <td>{f.version}</td>
                  <td><StatusTag status={f.status} /></td>
                  <td>{f.needed === 0 ? <span className="muted">No rule reads this document</span> : <span><span className={f.needed_read === f.needed ? "pass-text" : "fail-text"}>{f.needed_read} of {f.needed}</span> read</span>}</td>
                  <td>{f.last_used ? when(f.last_used) : <span className="muted">Never</span>}</td>
                  <td className="fmt-row-action"><button type="button" aria-label={`Manage ${f.name}`} onClick={() => onOpen(f.id)}>Manage</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <PageBar p={pager} noun="formats" />
    </>
  )
}

// ---------- Parameters ----------

function ParametersPanel({ data, canChange, onOpen, onAdd }: { data: FormatsList; canChange: boolean; onOpen: (key: string) => void; onAdd: () => void }) {
  const [search, setSearch] = useState('')
  const [document, setDocument] = useState('')
  const [origin, setOrigin] = useState('')
  const q = search.trim().toLowerCase()
  const shown = data.parameters.filter((p) =>
    (q === '' || p.key.toLowerCase().includes(q) || p.name.toLowerCase().includes(q) || p.aliases.some((a) => a.toLowerCase().includes(q))) &&
    (document === '' || p.document === document) && (origin === '' || p.source === origin))
  const pager = usePager(shown, 'parameters', `${q}|${document}|${origin}`)
  const active = search !== '' || document !== '' || origin !== ''
  return (
    <>
      <div className="fmt-toolbar">
        <p className="muted fmt-lede">Each one is a fact or record type of the vocabulary. Profiles write them and rules read them.</p>
        {canChange && <button type="button" onClick={onAdd}>+ Add a parameter</button>}
      </div>
      <div className="controls-premium fmt-filters" role="group" aria-label="Parameter filters">
        <label className="fmt-search">
          Search
          <span className="search-box">
            <SearchIcon />
            <input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Name, key or alias" aria-label="Search parameters" />
          </span>
        </label>
        <label>Document
          <select value={document} onChange={(e) => setDocument(e.target.value)} aria-label="Document" className="filter-select-premium">
            <option value="">All documents</option>
            {[...new Set(data.parameters.map((p) => p.document))].sort().map((d) => <option key={d}>{d}</option>)}
          </select>
        </label>
        <label>Origin
          <select value={origin} onChange={(e) => setOrigin(e.target.value)} aria-label="Origin of parameters" className="filter-select-premium">
            <option value="">All origins</option>
            <option value="builtin">Standard</option>
            <option value="office">Office</option>
          </select>
        </label>
        <button type="button" className="ghost" disabled={!active} onClick={() => { setSearch(''); setDocument(''); setOrigin('') }}>Clear filters</button>
      </div>
      <p className="result-count" aria-live="polite">{shown.length} of {plural(data.parameters.length, 'parameter', 'parameters')}</p>
      {shown.length === 0 ? (
        <p className="empty">No parameter matches these filters. Choose Clear filters to see them all.</p>
      ) : (
        <div className="table-wrap">
          <table className="table-premium fmt-table" aria-label="Parameters">
            <thead>
              <tr>
                <th scope="col">Parameter</th><th scope="col">Type</th><th scope="col">Document</th><th scope="col">Used by</th>
                <th scope="col">Status</th><th scope="col"><span className="fmt-sr">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              {pager.rows.map((p) => (
                <tr key={p.key}>
                  <td>
                    <span className="fmt-name">{p.name}</span>
                    <span className="fmt-sub mono muted">{p.key}</span>
                  </td>
                  <td>{kindText[p.kind] ?? p.kind}</td>
                  <td>{p.document}</td>
                  <td>{plural(p.used_by_formats.length, 'format', 'formats')} · {plural(p.used_by_rules.length, 'rule', 'rules')}</td>
                  <td>
                    {p.deprecated ? <span className="tag muted">Retired</span> : <span className={`tag ${p.status === 'active' ? 'pass' : 'muted'}`}>{p.status === 'active' ? 'Active' : 'Disabled'}</span>}
                    {p.source === 'office' && <span className="fmt-sub muted">Office</span>}
                  </td>
                  <td className="fmt-row-action"><button type="button" aria-label={`Open ${p.name}`} onClick={() => onOpen(p.key)}>{p.source === 'office' && canChange ? 'Manage' : 'View'}</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <PageBar p={pager} noun="parameters" />
    </>
  )
}

// ---------- Gaps ----------

function GapsPanel({ data, names, canChange, onTeach }: { data: FormatsList; names: Map<string, string>; canChange: boolean; onTeach: () => void }) {
  const pager = usePager(data.unsupplied, 'gaps')
  return (
    <>
      <section className="fmt-card" aria-labelledby="gaps-rules">
        <h2 id="gaps-rules">Rules with an input no active format reads ({data.uncovered.length})</h2>
        {data.uncovered.length === 0 ? (
          <p className="empty">Every active rule has its figures supplied by an active format.</p>
        ) : (
          <div className="table-wrap">
            <table className="table-premium fmt-table" aria-label="Rules that cannot run">
              <thead><tr><th scope="col">Rule</th><th scope="col">Name</th><th scope="col">Status</th><th scope="col">Figures not supplied</th></tr></thead>
              <tbody>
                {data.uncovered.map((r) => (
                  <tr key={r.id}>
                    <td className="mono">{r.id}</td>
                    <td>{r.title}</td>
                    <td><StatusTag status={r.status} /></td>
                    <td>{r.missing.map((k) => names.get(k) ?? k).join(', ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
      <section className="fmt-card" aria-labelledby="gaps-params">
        <h2 id="gaps-params">Parameters no active format reads ({data.unsupplied.length})</h2>
        {data.unsupplied.length === 0 ? (
          <p className="empty">Every parameter is read by an active format.</p>
        ) : (
          <>
            <div className="fmt-gap-head">
              <p className="muted">A rule that needs one of these cannot check until a format reads it.</p>
              {canChange && <button type="button" onClick={onTeach}>Teach a format</button>}
            </div>
            <ul className="fmt-gap-list" aria-label="Parameters not read">
              {pager.rows.map((k) => <li key={k}><span>{names.get(k) ?? k}</span><span className="mono muted">{k}</span></li>)}
            </ul>
            <PageBar p={pager} noun="gaps" />
          </>
        )}
      </section>
    </>
  )
}

// ---------- Popups ----------

// ReasonDialog asks for the reason a change needs, then runs it.
function ReasonDialog({ title, lede, action, danger, run, onClose }: {
  title: string
  lede: string
  action: string
  danger?: boolean
  run: (reason: string) => Promise<string | null>
  onClose: () => void
}) {
  const [reason, setReason] = useState('')
  const [fault, setFault] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (busy) return
    setBusy(true)
    const f = await run(reason)
    setBusy(false)
    if (f) setFault(f)
    else onClose()
  }
  return (
    <Modal title={title} onClose={onClose} closeDisabled={busy}>
      <form className="fmt-form" aria-label={title} onSubmit={submit}>
        <p className="muted">{lede}</p>
        <Field label="Reason" required>
          {(id) => <textarea id={id} rows={3} required maxLength={300} value={reason} onChange={(e) => setReason(e.target.value)} />}
        </Field>
        {fault && <p className="error" role="alert">{fault}</p>}
        <div className="form-actions">
          <button type="button" className="secondary-button" onClick={onClose} disabled={busy}>Cancel</button>
          <button type="submit" className={danger ? 'danger' : 'action'} disabled={busy}>{busy ? 'Working…' : action}</button>
        </div>
      </form>
    </Modal>
  )
}

type Ask = { kind: 'disable' | 'enable' } | { kind: 'back'; revision: number; version: string }

function FormatDetail({ f, names, canChange, onClose, onEdit, onPublish, onExport, onRemoveSample, onSwitch, onBack }: {
  f: FormatInfo
  names: Map<string, string>
  canChange: boolean
  onClose: () => void
  onEdit: () => void
  onPublish: (revision: number, version: string) => void
  onExport: () => void
  onRemoveSample: (id: number, name: string) => Promise<string | null>
  onSwitch: (action: 'disable' | 'enable', reason: string) => Promise<string | null>
  onBack: (revision: number, version: string, reason: string) => Promise<string | null>
}) {
  const [ask, setAsk] = useState<Ask | null>(null)
  const office = f.source === 'office'
  const disabled = f.status === 'disabled'
  const reads = [...f.reads_facts, ...f.reads_records]
  const versions = [...f.versions].sort((a, b) => b.revision - a.revision)

  if (ask) {
    const props = ask.kind === 'back'
      ? { title: `Switch back to ${ask.version}`, lede: 'This makes the earlier version the one in use, as testing. No version is deleted.', action: 'Switch back', run: (r: string) => onBack(ask.revision, ask.version, r) }
      : ask.kind === 'disable'
        ? { title: `Disable ${f.name}`, lede: 'Files in this layout are no longer recognised. Earlier comparisons still open as they were reported.', action: 'Disable', danger: true, run: (r: string) => onSwitch('disable', r) }
        : { title: `Enable ${f.name} again`, lede: 'It returns to testing.', action: 'Enable again', run: (r: string) => onSwitch('enable', r) }
    return <ReasonDialog {...props} onClose={() => setAsk(null)} />
  }
  return (
    <Modal title={f.name} onClose={onClose}>
      <div className="fmt-detail">
        <dl className="fmt-facts">
          <div><dt>File type</dt><dd>{fileText[f.file_type] ?? f.file_type}</dd></div>
          <div><dt>Document</dt><dd>{f.document}</dd></div>
          <div><dt>Origin</dt><dd>{originText[f.source] ?? f.source}{!office && <span className="muted"> (read only)</span>}</dd></div>
          <div><dt>Status</dt><dd><StatusTag status={f.status} /></dd></div>
          <div><dt>Version in use</dt><dd>{f.version}</dd></div>
          <div><dt>Figures read</dt><dd>{reads.length}</dd></div>
        </dl>
        {disabled && f.disabled_reason && <p className="notice">Disabled: {f.disabled_reason}</p>}
        {!office && <p className="muted">A standard format cannot be changed. To change how a layout is read, teach an office format from a sample file.</p>}

        <h3>Figures read</h3>
        {reads.length === 0 ? <p className="muted">This format's figures could not be listed.</p> : (
          <ul className="fmt-chips" aria-label="Figures read">
            {reads.map((k) => <li key={k} className="chip" title={k}>{names.get(k) ?? k}</li>)}
          </ul>
        )}

        <h3>Versions</h3>
        <div className="table-wrap">
          <table className="table-premium fmt-table" aria-label="Versions">
            <thead>
              <tr><th scope="col">Version</th><th scope="col">Status</th><th scope="col">By</th><th scope="col">When</th><th scope="col">What changed</th><th scope="col"><span className="fmt-sr">Actions</span></th></tr>
            </thead>
            <tbody>
              {versions.map((v) => (
                <tr key={v.revision}>
                  <td>{v.version}{v.revision === f.revision && <span className="muted"> (in use)</span>}</td>
                  <td>{!v.published ? <span className="tag note">Draft</span> : v.status === 'draft' ? <span className="tag muted">Published</span> : <StatusTag status={v.status} />}</td>
                  <td>{v.by}</td>
                  <td>{when(v.at)}</td>
                  <td>{v.note}</td>
                  <td className="fmt-row-action">
                    {canChange && office && !v.published && (
                      <button type="button" className="action" aria-label={`Publish ${v.version}`} onClick={() => onPublish(v.revision, v.version)}>Publish</button>
                    )}
                    {canChange && office && v.published && v.revision !== f.revision && v.status !== 'disabled' && (
                      <button type="button" aria-label={`Switch back to ${v.version}`} onClick={() => setAsk({ kind: 'back', revision: v.revision, version: v.version })}>Switch back</button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {office && (
          <>
            <h3>Kept samples</h3>
            {f.samples.length === 0 ? (
              <p className="muted">No sample is kept. Keep one when you teach or edit the format, and publishing a change reads it again to show what would differ.</p>
            ) : (
              <ul className="fmt-samples" aria-label="Kept samples">
                {f.samples.map((s) => (
                  <li key={s.id}>
                    <span><span className="mono">{s.file_name}</span><span className="muted"> · {Math.max(1, Math.round(s.size_bytes / 1024))} KB · kept by {s.kept_by}</span></span>
                    {canChange && <button type="button" className="ghost" aria-label={`Remove the kept sample ${s.file_name}`} onClick={() => void onRemoveSample(s.id, s.file_name)}>Remove</button>}
                  </li>
                ))}
              </ul>
            )}
          </>
        )}

        <div className="form-actions">
          <button type="button" className="secondary-button" onClick={onClose}>Close</button>
          {canChange && office && <button type="button" onClick={onExport}>Export</button>}
          {canChange && office && !disabled && <button type="button" onClick={onEdit}>Edit the mapping</button>}
          {canChange && office && !disabled && <button type="button" className="danger" onClick={() => setAsk({ kind: 'disable' })}>Disable</button>}
          {canChange && office && disabled && <button type="button" className="action" onClick={() => setAsk({ kind: 'enable' })}>Enable again</button>}
        </div>
      </div>
    </Modal>
  )
}

const emptyMapping: LayoutMapping = {
  name: '', document_type: '', rows: '', where: [], unit: 'rupees', columns: [], period: { path: '', format: '', const: '' },
  tax_year: '', gstin_path: '', identify: [],
}

// EditSample asks for a sample file in the format's layout: the mapping screen needs the file to
// list its fields. It is read by the server for this edit only and not kept.
function EditSample({ name, onFile, onCancel }: { name: string; onFile: (f: File) => void; onCancel: () => void }) {
  return (
    <Modal title={`Edit ${name}`} onClose={onCancel}>
      <div className="fmt-form">
        <p className="muted">Choose a sample file in this layout (JSON or Excel). It is used to list the fields for this edit, and is not kept. Use a file with invented or masked figures where you can.</p>
        <Field label="Sample file" required>
          {(id) => (
            <input id={id} type="file" accept=".json,.xlsx"
              onChange={(e) => {
                const f = e.target.files?.[0]
                if (f) onFile(f)
              }} />
          )}
        </Field>
        <div className="form-actions">
          <button type="button" className="secondary-button" onClick={onCancel}>Cancel</button>
        </div>
      </div>
    </Modal>
  )
}

// PublishDialog makes a draft the version in use, as testing or active, then shows which rules
// can run on it. Testing is read only by runs that include testing; active is read by every run.
function PublishDialog({ name, version, run, onClose }: {
  name: string
  version: string
  run: (status: 'testing' | 'active', confirm?: { reason: string }) => Promise<{ fault: string | null; changed: boolean; dry: DryRun | null }>
  onClose: () => void
}) {
  const [status, setStatus] = useState<'testing' | 'active'>('testing')
  const [fault, setFault] = useState<string | null>(null)
  const [done, setDone] = useState<{ dry: DryRun | null } | null>(null)
  // A kept sample that reads differently with this version needs a reason before it is published.
  const [changed, setChanged] = useState(false)
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (busy) return
    setBusy(true)
    const out = await run(status, changed ? { reason } : undefined)
    setBusy(false)
    if (out.fault) {
      setFault(out.fault)
      setChanged(out.changed)
    } else setDone({ dry: out.dry })
  }
  if (done) {
    return (
      <Modal title={`${name} ${version} is published`} onClose={onClose}>
        <div className="fmt-form">
          <p role="status">Published as {status}. {status === 'testing' ? 'Only runs that include testing read it, until it is published as active.' : 'Every run reads it.'}</p>
          {done.dry && <DryRunView dry={done.dry} />}
          <div className="form-actions"><button type="button" className="action" onClick={onClose}>Close</button></div>
        </div>
      </Modal>
    )
  }
  return (
    <Modal title={`Publish ${version} of ${name}`} onClose={onClose} closeDisabled={busy}>
      <form className="fmt-form" aria-label="Publish" onSubmit={submit}>
        <p className="muted">Publishing writes this version so the engine reads it, and makes it the version in use. The earlier version is kept and can be switched back to.</p>
        <fieldset className="fmt-status">
          <legend>Publish as</legend>
          <label>
            <input type="radio" name="fmt-publish-status" checked={status === 'testing'} onChange={() => setStatus('testing')} />
            <span><strong>Testing</strong><span className="muted"> Read only by runs that include testing.</span></span>
          </label>
          <label>
            <input type="radio" name="fmt-publish-status" checked={status === 'active'} onChange={() => setStatus('active')} />
            <span><strong>Active</strong><span className="muted"> Read by every run.</span></span>
          </label>
        </fieldset>
        {fault && <p className="error" role="alert">{fault}</p>}
        {changed && (
          <Field label="Reason for publishing it anyway" required help="A kept sample reads differently with this version. The reason is kept in the audit log.">
            {(id) => <textarea id={id} rows={2} required maxLength={300} value={reason} onChange={(e) => setReason(e.target.value)} />}
          </Field>
        )}
        <div className="form-actions">
          <button type="button" className="secondary-button" onClick={onClose} disabled={busy}>Cancel</button>
          <button type="submit" className="action" disabled={busy || (changed && reason.trim() === '')}>{busy ? 'Publishing…' : changed ? 'Publish anyway' : 'Publish'}</button>
        </div>
      </form>
    </Modal>
  )
}

function ParameterDetail({ p, formatNames, canChange, onClose, onSwitch, onAlias }: {
  p: ParameterInfo
  formatNames: Map<string, string>
  canChange: boolean
  onClose: () => void
  onSwitch: (action: 'disable' | 'enable', reason: string) => Promise<string | null>
  onAlias: (alias: string, remove: boolean) => Promise<string | null>
}) {
  const [alias, setAlias] = useState('')
  const [fault, setFault] = useState<string | null>(null)
  const [ask, setAsk] = useState<'disable' | 'enable' | null>(null)
  const office = p.source === 'office'
  const disabled = p.status === 'disabled'

  if (ask) {
    const disable = ask === 'disable'
    return (
      <ReasonDialog
        title={disable ? `Disable ${p.name}` : `Enable ${p.name} again`}
        lede={disable
          ? `${plural(p.used_by_formats.length, 'format', 'formats')} and ${plural(p.used_by_rules.length, 'rule', 'rules')} use it. A parameter that an active rule reads cannot be disabled.`
          : 'It is offered again.'}
        action={disable ? 'Disable' : 'Enable again'} danger={disable}
        run={(r) => onSwitch(disable ? 'disable' : 'enable', r)} onClose={() => setAsk(null)} />
    )
  }
  const change = async (a: string, remove: boolean) => {
    const f = await onAlias(a, remove)
    setFault(f)
    if (f === null && !remove) setAlias('')
  }
  return (
    <Modal title={p.name} onClose={onClose}>
      <div className="fmt-detail">
        <dl className="fmt-facts">
          <div><dt>Key</dt><dd className="mono">{p.key}</dd></div>
          <div><dt>Type</dt><dd>{kindText[p.kind] ?? p.kind}</dd></div>
          <div><dt>Document</dt><dd>{p.document}</dd></div>
          <div><dt>Origin</dt><dd>{originText[p.source] ?? p.source}{!office && <span className="muted"> (read only)</span>}</dd></div>
          <div><dt>Status</dt><dd>{p.deprecated ? <span className="tag muted">Retired</span> : <span className={`tag ${disabled ? 'muted' : 'pass'}`}>{disabled ? 'Disabled' : 'Active'}</span>}</dd></div>
          <div><dt>Version</dt><dd>{p.version}</dd></div>
        </dl>
        {p.description && <p>{p.description}</p>}
        {disabled && p.disabled_reason && <p className="notice">Disabled: {p.disabled_reason}</p>}

        {office && (
          <>
            <h3>Aliases</h3>
            <p className="muted fmt-help">Other labels that mean the same figure. They are used to suggest a match when a format is taught; nothing is mapped without a click.</p>
            {p.aliases.length === 0 ? <p className="muted">No aliases yet.</p> : (
              <ul className="fmt-chips" aria-label="Aliases">
                {p.aliases.map((a) => (
                  <li key={a} className="chip">
                    {a}
                    {canChange && <button type="button" className="ghost icon-btn" aria-label={`Remove alias ${a}`} onClick={() => change(a, true)}>×</button>}
                  </li>
                ))}
              </ul>
            )}
            {canChange && (
              <div className="fmt-alias-add">
                <Field label="Add an alias">{(id) => <input id={id} type="text" maxLength={80} value={alias} onChange={(e) => setAlias(e.target.value)} />}</Field>
                <button type="button" disabled={alias.trim() === ''} onClick={() => change(alias, false)}>Add alias</button>
              </div>
            )}
            {fault && <p className="error" role="alert">{fault}</p>}
          </>
        )}

        <h3>Where it is used</h3>
        <div className="fmt-used">
          <div>
            <h4>{plural(p.used_by_formats.length, 'format', 'formats')}</h4>
            {p.used_by_formats.length === 0 ? <p className="muted">No format reads it.</p> : <ul>{p.used_by_formats.map((id) => <li key={id}>{formatNames.get(id) ?? id}</li>)}</ul>}
          </div>
          <div>
            <h4>{plural(p.used_by_rules.length, 'rule', 'rules')}</h4>
            {p.used_by_rules.length === 0 ? <p className="muted">No rule reads it.</p> : (
              <ul>{p.used_by_rules.map((r) => <li key={r.id}><span className="mono">{r.id}</span> {r.title} <span className="muted">({statusText[r.status] ?? r.status})</span></li>)}</ul>
            )}
          </div>
        </div>

        <div className="form-actions">
          <button type="button" className="secondary-button" onClick={onClose}>Close</button>
          {canChange && office && !disabled && <button type="button" className="danger" onClick={() => setAsk('disable')}>Disable</button>}
          {canChange && office && disabled && <button type="button" className="action" onClick={() => setAsk('enable')}>Enable again</button>}
        </div>
      </div>
    </Modal>
  )
}

function AddParameter({ documents, onAdd, onClose }: {
  documents: string[]
  onAdd: (p: { key: string; name: string; kind: string; document: string; description: string; aliases: string[] }) => Promise<string | null>
  onClose: () => void
}) {
  const [v, setV] = useState({ name: '', key: '', kind: 'money', document: documents[0] ?? '', description: '', aliases: '' })
  const [fault, setFault] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const set = (k: keyof typeof v) => (e: { target: { value: string } }) => setV((x) => ({ ...x, [k]: e.target.value }))
  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (busy) return
    setBusy(true)
    const f = await onAdd({ ...v, aliases: v.aliases.split(',').map((a) => a.trim()).filter(Boolean) })
    setBusy(false)
    setFault(f)
  }
  return (
    <Modal title="Add a parameter" onClose={onClose} closeDisabled={busy}>
      <form className="fmt-form" aria-label="Add parameter" onSubmit={submit}>
        <p className="muted">A parameter is a named figure the rules can read. It is added now and listed here; a profile can write it once the vocabulary loader reads office parameters.</p>
        <div className="fmt-grid">
          <Field label="Name" required>{(id) => <input id={id} type="text" required maxLength={80} value={v.name} onChange={set('name')} />}</Field>
          <Field label="Key" required help='Like "gst.taxable_value": lower-case words, a dot between the group and the name.'>
            {(id) => <input id={id} type="text" className="mono" required maxLength={80} autoComplete="off" spellCheck={false} value={v.key} onChange={set('key')} />}
          </Field>
          <Field label="Type" required>
            {(id) => <select id={id} value={v.kind} onChange={set('kind')}>{addableKinds.map((k) => <option key={k} value={k}>{kindText[k]}</option>)}</select>}
          </Field>
          <Field label="Document" required>
            {(id) => <select id={id} value={v.document} onChange={set('document')}>{documents.map((d) => <option key={d}>{d}</option>)}</select>}
          </Field>
        </div>
        <Field label="Description" optional>{(id) => <textarea id={id} rows={2} maxLength={300} value={v.description} onChange={set('description')} />}</Field>
        <Field label="Other labels for it (aliases)" optional help="Separate with commas.">{(id) => <input id={id} type="text" maxLength={300} value={v.aliases} onChange={set('aliases')} />}</Field>
        {fault && <p className="error" role="alert">{fault}</p>}
        <div className="form-actions">
          <button type="button" className="secondary-button" onClick={onClose} disabled={busy}>Cancel</button>
          <button type="submit" className="action" disabled={busy}>{busy ? 'Adding…' : 'Add parameter'}</button>
        </div>
      </form>
    </Modal>
  )
}
