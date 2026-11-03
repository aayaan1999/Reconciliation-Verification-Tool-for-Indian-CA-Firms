import { useRef, useState } from 'react'
import { ApiError, importBundle } from '../api.ts'
import type { FormatsList, ImportPlan, UnrecognisedInfo } from '../formatsTypes.ts'
import { usePager } from '../pager.ts'
import { Modal } from './Modal.tsx'
import { PageBar } from './PageBar.tsx'
import { Field } from './RuleBuilder.tsx'

// The parts of the Formats screen for files no format recognised, and for importing a format
// from a bundle (SPEC-016).

const fileText: Record<string, string> = { json: 'JSON', xlsx: 'Excel', csv: 'CSV' }
const when = (at: string) => at.slice(0, 16).replace('T', ' ')

function problem(err: unknown): string {
  if (err instanceof ApiError) return `${err.message} (${err.code})`
  return String(err)
}

// sha256Hex is the hash of a file's bytes, as the server keeps it for an unrecognised file.
async function sha256Hex(file: File): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', await file.arrayBuffer())
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, '0')).join('')
}

export function UnrecognisedPanel({ data, canChange, onTeach, onDismiss }: {
  data: FormatsList
  canChange: boolean
  onTeach: (u: UnrecognisedInfo) => void
  onDismiss: (u: UnrecognisedInfo) => void
}) {
  const pager = usePager(data.unrecognised, 'unrecognised')
  if (data.unrecognised.length === 0) {
    return <p className="empty">No file is waiting for a format. A file no format recognises is listed here after a run.</p>
  }
  return (
    <>
      <p className="muted fmt-lede fmt-toolbar">Files that no format recognised. Teach a format so the next file in that layout is read, or dismiss a file with a reason. Only the file's name, size and hash are kept here.</p>
      <div className="table-wrap">
        <table className="table-premium fmt-table" aria-label="Unrecognised files">
          <thead>
            <tr>
              <th scope="col">File</th><th scope="col">Type</th><th scope="col">Tax year</th><th scope="col">Noted</th>
              <th scope="col"><span className="fmt-sr">Actions</span></th>
            </tr>
          </thead>
          <tbody>
            {pager.rows.map((u) => (
              <tr key={u.id}>
                <td><span className="mono">{u.file_name}</span><span className="fmt-sub muted">{Math.max(1, Math.round(u.size_bytes / 1024))} KB</span></td>
                <td>{fileText[u.file_type] ?? '—'}</td>
                <td>{u.tax_year || '—'}</td>
                <td>{when(u.noted_at)}</td>
                <td className="fmt-row-action">
                  {canChange && (
                    <>
                      <button type="button" className="action" aria-label={`Teach a format for ${u.file_name}`} onClick={() => onTeach(u)}>Teach this format</button>
                      <button type="button" className="ghost" aria-label={`Dismiss ${u.file_name}`} onClick={() => onDismiss(u)}>Dismiss</button>
                    </>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <PageBar p={pager} noun="files" />
    </>
  )
}

// FindFile asks for the file that was noted, again: the server kept only its name and hash, so the
// mapping screen needs the file itself. A different file can still be used, after a warning.
export function FindFile({ item, onFile, onCancel }: { item: UnrecognisedInfo; onFile: (f: File) => void; onCancel: () => void }) {
  const [other, setOther] = useState<File | null>(null)
  return (
    <Modal title={`Teach a format for ${item.file_name}`} onClose={onCancel}>
      <div className="fmt-form">
        <p className="muted">Choose this file again. The server kept only its name and hash, so the mapping screen needs the file itself to list its fields. It is not kept.</p>
        <Field label="The file" required>
          {(id) => (
            <input id={id} type="file" accept=".json,.xlsx,.csv"
              onChange={async (e) => {
                const f = e.target.files?.[0]
                if (!f) return
                if ((await sha256Hex(f)) === item.sha256) onFile(f)
                else setOther(f)
              }} />
          )}
        </Field>
        {other && (
          <p className="notice" role="alert">
            {other.name} is not the file that was noted ({item.file_name}). You can still teach a format from it.
            <button type="button" onClick={() => onFile(other)}>Use it anyway</button>
          </p>
        )}
        <div className="form-actions">
          <button type="button" className="secondary-button" onClick={onCancel}>Cancel</button>
        </div>
      </div>
    </Modal>
  )
}

// ImportDialog reads a bundle file, shows what it would add, and imports it on the Partner's click.
// The server checks the bundle with the same strict loaders as a taught format.
export function ImportDialog({ onClose, onImported }: { onClose: () => void; onImported: (plan: ImportPlan) => void }) {
  const [bundle, setBundle] = useState<unknown>(null)
  const [fileName, setFileName] = useState('')
  const [plan, setPlan] = useState<ImportPlan | null>(null)
  const [fault, setFault] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const input = useRef<HTMLInputElement>(null)

  const choose = async (f: File) => {
    setPlan(null)
    setFault(null)
    setFileName(f.name)
    try {
      const parsed: unknown = JSON.parse(await f.text())
      setBundle(parsed)
      setBusy(true)
      setPlan(await importBundle(parsed, true))
    } catch (err) {
      setBundle(null)
      setFault(err instanceof SyntaxError ? `${f.name} is not a JSON file.` : problem(err))
    } finally {
      setBusy(false)
    }
  }
  const apply = async () => {
    setBusy(true)
    try {
      onImported(await importBundle(bundle, false))
    } catch (err) {
      setFault(problem(err))
      setBusy(false)
    }
  }
  return (
    <Modal title="Import a format" onClose={onClose} closeDisabled={busy}>
      <div className="fmt-form">
        <p className="muted">A bundle is one office format with its parameters, exported from another office machine. It is checked first, and nothing is changed until you import. An imported format starts in testing.</p>
        <Field label="Bundle file" required help="A .json file made by Export on another machine.">
          {(id) => <input ref={input} id={id} type="file" accept=".json" onChange={(e) => { const f = e.target.files?.[0]; if (f) void choose(f) }} />}
        </Field>
        {busy && !plan && <p className="loading">Checking {fileName}…</p>}
        {fault && <p className="error" role="alert">{fault}</p>}
        {plan && (
          <section className="fmt-dry" aria-label="What the bundle adds">
            <h3>What it adds</h3>
            <ul>
              <li>Format <strong>{plan.format_name}</strong> <span className="mono muted">{plan.format}</span>, in testing</li>
              <li>{plan.parameters_added.length === 0 ? 'No new parameters' : `${plan.parameters_added.length} new parameters: ${plan.parameters_added.join(', ')}`}</li>
              {plan.parameters_unchanged.length > 0 && <li>{plan.parameters_unchanged.length} parameters already here and the same</li>}
            </ul>
          </section>
        )}
        <div className="form-actions">
          <button type="button" className="secondary-button" onClick={onClose} disabled={busy}>Cancel</button>
          <button type="button" className="action" disabled={!plan || busy} onClick={apply}>{busy && plan ? 'Importing…' : 'Import'}</button>
        </div>
      </div>
    </Modal>
  )
}
