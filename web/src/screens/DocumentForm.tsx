import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { ApiError, addDocument, editDocument, fetchDocument, validateDocument } from '../api.ts'
import {
  docCategories, fileKindNames, fileKinds,
  type ColumnType, type DocColumn, type DocDetail, type DocInput, type DocProblem, type DocRow,
} from '../docMasterTypes.ts'
import { CheckIcon } from '../icons.tsx'
import { Modal } from './Modal.tsx'
import { Field } from './RuleBuilder.tsx'

// The Add and Edit popup of the Document Master (SPEC-022), in three steps like the layout
// wizard: (1) about the document, (2) its record structure, (3) the checks and the save. The
// engine decides what is valid; the checks shown here are its answer to /document-types/validate.
// A structure that is not valid yet saves the document as a Draft, which says what is missing and
// is offered nowhere until it is complete. Saving creates no layout, list detection, column
// mapping or rule.

type Mode = 'none' | 'copy' | 'new'

interface Form {
  code: string
  name: string
  category: string
  description: string
  formats: string[]
  status: 'active' | 'inactive'
  mode: Mode
  copyFrom: string
  rtName: string
  rtDescription: string
  columns: DocColumn[]
}

const blankColumn = (): DocColumn => ({ name: '', label: '', type: 'text', key: false, required: false })

const blank: Form = {
  code: '', name: '', category: '', description: '', formats: [], status: 'active',
  mode: 'new', copyFrom: '', rtName: '', rtDescription: '', columns: [blankColumn()],
}

function problemText(err: unknown): string {
  if (err instanceof ApiError) return `${err.message} (${err.code})`
  return String(err)
}

function inputOf(f: Form, editing: boolean): DocInput {
  const base: DocInput = {
    code: f.code.trim(), name: f.name.trim(), category: f.category, description: f.description.trim(),
    applicable_for: fileKinds.filter((k) => f.formats.includes(k)), status: f.status,
  }
  if (f.mode === 'none') return base
  return {
    ...base,
    record_type: {
      name: f.rtName.trim(), description: f.rtDescription.trim(),
      columns: f.columns.map((c) => ({ ...c, name: c.name.trim(), label: c.label.trim() })),
      ...(f.mode === 'copy' && f.copyFrom && !editing ? { copy_from: f.copyFrom } : {}),
    },
  }
}

function formOf(d: DocDetail): Form {
  return {
    code: d.code, name: d.name, category: d.category, description: d.description, formats: d.applicable_for,
    status: d.status === 'inactive' ? 'inactive' : 'active',
    mode: d.structure ? 'new' : 'none', copyFrom: '',
    rtName: d.structure?.name ?? '', rtDescription: d.structure?.description ?? '',
    columns: d.structure?.columns ?? [blankColumn()],
  }
}

// The checks of step 3, each with the fields of the engine's problems it covers.
const checks: { name: string; covers: (field: string) => boolean }[] = [
  { name: 'Document code', covers: (f) => f === 'code' },
  { name: 'Name', covers: (f) => f === 'name' },
  { name: 'Category', covers: (f) => f === 'category' },
  { name: 'Description', covers: (f) => f === 'description' },
  { name: 'File formats', covers: (f) => f === 'applicable_for' },
  { name: 'Record type', covers: (f) => f === 'record_type' || f.startsWith('record_type.') },
  { name: 'Columns', covers: (f) => f === 'columns' || f.startsWith('columns[') },
]
const documentFields = ['code', 'name', 'category', 'description', 'applicable_for']

const steps = [
  { n: 1, name: 'About the document' },
  { n: 2, name: 'Record structure' },
  { n: 3, name: 'Check and save' },
]

interface Props {
  code?: string // an edit of this office document
  rows: DocRow[] // every document, for "copy the columns of"
  onClose: () => void
  onSaved: (message: string) => void | Promise<void>
}

export function DocumentForm({ code, rows, onClose, onSaved }: Props) {
  const editing = code !== undefined
  const [detail, setDetail] = useState<DocDetail | null>(null)
  const [form, setForm] = useState<Form>(blank)
  const [step, setStep] = useState(1)
  const [problems, setProblems] = useState<DocProblem[]>([])
  const [checked, setChecked] = useState(false)
  const [busy, setBusy] = useState(false)
  const [fault, setFault] = useState<string | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)

  // An edit reads the document first; the form is set only when the answer arrives.
  useEffect(() => {
    if (code === undefined) return
    let cancelled = false
    fetchDocument(code)
      .then((d) => {
        if (cancelled) return
        setDetail(d)
        setForm(formOf(d))
      })
      .catch((err: unknown) => {
        if (!cancelled) setLoadError(problemText(err))
      })
    return () => {
      cancelled = true
    }
  }, [code])

  const locked = detail?.structure?.locked ?? false
  const original = useMemo(() => new Set((detail?.structure?.columns ?? []).map((c) => c.name)), [detail])
  const copySources = useMemo(() => rows.filter((r) => r.record_type !== '' && r.code !== code), [rows, code])
  const set = <K extends keyof Form>(key: K, value: Form[K]) => {
    setForm((f) => ({ ...f, [key]: value }))
    setChecked(false)
  }
  const setColumn = (i: number, patch: Partial<DocColumn>) => set('columns', form.columns.map((c, j) => (j === i ? { ...c, ...patch } : c)))
  const hasStructure = detail?.structure != null

  const fieldProblem = (field: string) => problems.find((p) => p.field === field)?.message
  const columnProblems = (i: number) => problems.filter((p) => p.field.startsWith(`columns[${i}]`)).map((p) => p.message)
  const generalColumnProblems = problems.filter((p) => p.field === 'columns' || p.field === 'record_type' || (p.field.startsWith('record_type.') && p.field !== 'record_type.name'))

  // check asks the engine what is wrong with the form as it stands.
  const check = async (): Promise<DocProblem[] | null> => {
    setBusy(true)
    setFault(null)
    try {
      const v = await validateDocument(inputOf(form, editing), editing)
      setProblems(v.problems)
      setChecked(true)
      return v.problems
    } catch (err) {
      setFault(problemText(err))
      return null
    } finally {
      setBusy(false)
    }
  }

  // Choosing a document to copy asks the engine for its columns, which then fill the table.
  const copyColumns = async (source: string) => {
    setForm((f) => ({ ...f, copyFrom: source, columns: [], rtName: f.rtName || `${f.code.trim() || 'document'}_row` }))
    setChecked(false)
    if (!source) return
    setBusy(true)
    setFault(null)
    try {
      const v = await validateDocument({
        ...inputOf({ ...form, copyFrom: source, mode: 'copy', columns: [], rtName: form.rtName || `${form.code.trim() || 'document'}_row` }, false),
      }, false)
      setForm((f) => ({ ...f, copyFrom: source, columns: v.record_type?.columns ?? [] }))
    } catch (err) {
      setFault(problemText(err))
    } finally {
      setBusy(false)
    }
  }

  const go = async (n: number) => {
    if (n > step) {
      const found = await check()
      if (found === null) return
      // Step 1 is left only with its own fields in order; the structure is filled in at step 2.
      if (step === 1 && found.some((p) => documentFields.includes(p.field))) return
    }
    setStep(n)
  }

  const documentProblems = problems.filter((p) => documentFields.includes(p.field))
  const structureProblems = problems.filter((p) => !documentFields.includes(p.field))
  const draft = form.mode === 'none' || structureProblems.length > 0
  // A document that is ready already cannot be saved with a structure that is not valid: the engine refuses it.
  const blocked = documentProblems.length > 0 || (hasStructure && structureProblems.length > 0)

  const save = async () => {
    setBusy(true)
    setFault(null)
    try {
      const input = inputOf(form, editing)
      const out = editing ? await editDocument(code, input) : await addDocument(input)
      const d = out.document
      await onSaved(
        d.status === 'draft'
          ? `${d.name} is saved as a Draft: ${out.problems.length > 0 ? out.problems[0].message : 'its record structure is not added yet'}. It is not offered anywhere until the structure is complete.`
          : `${d.name} is saved and ${d.status === 'active' ? 'offered in the layout wizard' : 'switched off'}.`,
      )
    } catch (err) {
      if (err instanceof ApiError && err.problems.length > 0) {
        setProblems(err.problems)
        setChecked(true)
        setStep(err.problems.some((p) => documentFields.includes(p.field)) ? 1 : 3)
      }
      setFault(problemText(err))
    } finally {
      setBusy(false)
    }
  }

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (busy) return
    if (step < 3) void go(step + 1)
    else if (checked && !blocked) void save()
  }

  const title = editing ? `Edit ${detail?.name ?? 'document'}` : 'Add document'
  if (loadError) {
    return (
      <Modal title={title} onClose={onClose}>
        <p className="error" role="alert">{loadError}</p>
      </Modal>
    )
  }
  if (editing && detail === null) {
    return (
      <Modal title={title} onClose={onClose}>
        <p className="loading">Loading the document…</p>
      </Modal>
    )
  }

  return (
    <Modal title={title} onClose={onClose}>
      <form className="teach docs-form" aria-label={title} onSubmit={submit} noValidate>
        <div className="teach-head">
          <ol className="teach-steps" aria-label="Steps">
            {steps.map((s) => (
              <li key={s.n} className={s.n < step ? 'done' : s.n === step ? 'current' : undefined} aria-current={s.n === step ? 'step' : undefined}>
                <span className="teach-step-mark" aria-hidden="true">{s.n < step ? <CheckIcon /> : s.n}</span>
                {s.name}
              </li>
            ))}
          </ol>
        </div>

        {step === 1 && (
          <fieldset className="teach-card docs-about">
            <legend>About the document</legend>
            <div className="admin-grid">
              <Field label="Document name" required problem={fieldProblem('name')} help="What staff will see, for example Sales return register.">
                {(id) => <input id={id} type="text" autoFocus value={form.name} maxLength={80} aria-invalid={fieldProblem('name') !== undefined}
                  onChange={(e) => set('name', e.target.value)} />}
              </Field>
              <Field label="Unique document code" required problem={fieldProblem('code')}
                help={editing ? 'A code is never changed.' : 'Lower case letters, digits and _, 3 to 40 characters, for example sales_return.'}>
                {(id) => <input id={id} type="text" className="mono" value={form.code} maxLength={60} readOnly={editing} autoComplete="off"
                  aria-invalid={fieldProblem('code') !== undefined} onChange={(e) => set('code', e.target.value)} />}
              </Field>
              <Field label="Category" required problem={fieldProblem('category')}>
                {(id) => (
                  <select id={id} value={form.category} aria-invalid={fieldProblem('category') !== undefined} onChange={(e) => set('category', e.target.value)}>
                    <option value="">Choose…</option>
                    {docCategories.map((c) => <option key={c} value={c}>{c}</option>)}
                  </select>
                )}
              </Field>
              <Field label="Status" help="A document is offered only when it is Active and its structure is complete.">
                {(id) => (
                  <select id={id} value={form.status} onChange={(e) => set('status', e.target.value === 'inactive' ? 'inactive' : 'active')}>
                    <option value="active">Active</option>
                    <option value="inactive">Inactive</option>
                  </select>
                )}
              </Field>
            </div>
            <fieldset className="docs-formats" aria-describedby="docs-formats-help">
              <legend className="required">Applicable for</legend>
              {fileKinds.map((k) => (
                <label key={k} className="check">
                  <input type="checkbox" checked={form.formats.includes(k)}
                    onChange={(e) => set('formats', e.target.checked ? [...form.formats, k] : form.formats.filter((x) => x !== k))} />
                  {fileKindNames[k]}
                </label>
              ))}
              <span id="docs-formats-help" className="help">The layout wizard offers this document only for files in these formats.</span>
              {fieldProblem('applicable_for') && <span className="fail-text">{fieldProblem('applicable_for')}</span>}
            </fieldset>
            <Field label="Description" required problem={fieldProblem('description')} help="10 to 300 characters, on one line.">
              {(id) => <textarea id={id} rows={2} value={form.description} maxLength={400} aria-invalid={fieldProblem('description') !== undefined}
                onChange={(e) => set('description', e.target.value)} />}
            </Field>
          </fieldset>
        )}

        {step === 2 && (
          <fieldset className="teach-card docs-structure">
            <legend>Record structure</legend>
            {hasStructure ? (
              <p className="muted">
                The columns of one row of this document.
                {detail?.structure?.copied_from && <> Copied from <span className="mono">{detail.structure.copied_from}</span>; it is independent of it now.</>}
              </p>
            ) : (
              <div className="docs-modes" role="radiogroup" aria-label="Where the columns come from">
                <label className="check">
                  <input type="radio" name="docs-mode" checked={form.mode === 'new'} onChange={() => {
                    set('mode', 'new')
                    if (form.columns.length === 0) set('columns', [blankColumn()])
                  }} />
                  Define new columns
                </label>
                <label className="check">
                  <input type="radio" name="docs-mode" checked={form.mode === 'copy'} onChange={() => set('mode', 'copy')} />
                  Copy the columns of an existing document
                </label>
                <label className="check">
                  <input type="radio" name="docs-mode" checked={form.mode === 'none'} onChange={() => set('mode', 'none')} />
                  Add the structure later (saves a Draft)
                </label>
              </div>
            )}

            {form.mode === 'copy' && !hasStructure && (
              <Field label="Copy the columns of" help="The copy is independent: changing it never changes the document it came from.">
                {(id) => (
                  <select id={id} value={form.copyFrom} onChange={(e) => void copyColumns(e.target.value)}>
                    <option value="">Choose…</option>
                    {copySources.map((r) => <option key={r.code} value={r.record_type}>{`${r.name} (${r.record_type})`}</option>)}
                  </select>
                )}
              </Field>
            )}

            {form.mode === 'none' && (
              <p className="muted">The document is saved as a Draft. It is offered nowhere until you come back and add its structure.</p>
            )}

            {form.mode !== 'none' && (
              <>
                {locked && (
                  <p className="notice" role="note">
                    A layout, stored rows or a rule already read this structure, so its columns, types and key and required flags cannot change. You can change labels
                    and the description, and add columns that are neither a key nor required.
                  </p>
                )}
                <div className="admin-grid">
                  <Field label="Record type name" required problem={fieldProblem('record_type.name')}
                    help={hasStructure ? 'A record type is never renamed.' : 'Lower case letters, digits and _, for example sales_return_row.'}>
                    {(id) => <input id={id} type="text" className="mono" value={form.rtName} readOnly={hasStructure} maxLength={60} autoComplete="off"
                      aria-invalid={fieldProblem('record_type.name') !== undefined} onChange={(e) => set('rtName', e.target.value)} />}
                  </Field>
                  <Field label="Record type description" required>
                    {(id) => <input id={id} type="text" value={form.rtDescription} maxLength={300} onChange={(e) => set('rtDescription', e.target.value)} />}
                  </Field>
                </div>

                <div className="table-wrap">
                  <table className="table-premium docs-columns" aria-label="Columns">
                    <thead>
                      <tr>
                        <th scope="col">Name</th>
                        <th scope="col">Label</th>
                        <th scope="col">Type</th>
                        <th scope="col">Key</th>
                        <th scope="col">Required</th>
                        <th scope="col"><span className="sr-only">Order and remove</span></th>
                      </tr>
                    </thead>
                    <tbody>
                      {form.columns.map((c, i) => {
                        const fixed = locked && original.has(c.name)
                        const fresh = locked && !original.has(c.name)
                        const messages = columnProblems(i)
                        return (
                          <tr key={i}>
                            <td>
                              <input type="text" className="mono" aria-label={`Column ${i + 1} name`} value={c.name} disabled={fixed} autoComplete="off"
                                aria-invalid={messages.length > 0} onChange={(e) => setColumn(i, { name: e.target.value })} />
                              {messages.map((m) => <span key={m} className="fail-text docs-cell-problem">{m}</span>)}
                            </td>
                            <td>
                              <input type="text" aria-label={`Column ${i + 1} label`} value={c.label} onChange={(e) => setColumn(i, { label: e.target.value })} />
                            </td>
                            <td>
                              <select aria-label={`Column ${i + 1} type`} value={c.type} disabled={fixed}
                                onChange={(e) => setColumn(i, { type: e.target.value as ColumnType, key: e.target.value === 'amount' ? false : c.key })}>
                                <option value="text">Text</option>
                                <option value="date">Date</option>
                                <option value="amount">Amount</option>
                              </select>
                            </td>
                            <td>
                              <input type="checkbox" aria-label={`Column ${i + 1} is part of the key`} checked={c.key} disabled={fixed || fresh || c.type === 'amount'}
                                onChange={(e) => setColumn(i, { key: e.target.checked })} />
                            </td>
                            <td>
                              <input type="checkbox" aria-label={`Column ${i + 1} is required`} checked={c.required} disabled={fixed || fresh}
                                onChange={(e) => setColumn(i, { required: e.target.checked })} />
                            </td>
                            <td className="docs-col-actions">
                              <button type="button" aria-label={`Move column ${i + 1} up`} disabled={i === 0}
                                onClick={() => set('columns', swap(form.columns, i, i - 1))}>↑</button>
                              <button type="button" aria-label={`Move column ${i + 1} down`} disabled={i === form.columns.length - 1}
                                onClick={() => set('columns', swap(form.columns, i, i + 1))}>↓</button>
                              <button type="button" className="ghost" aria-label={`Remove column ${i + 1}`} disabled={fixed || form.columns.length === 1}
                                onClick={() => set('columns', form.columns.filter((_, j) => j !== i))}>Remove</button>
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
                <div className="docs-col-add">
                  <button type="button" onClick={() => set('columns', [...form.columns, blankColumn()])}>Add column</button>
                  <span className="help">Key columns identify a row and are text or dates, never amounts. At least one column must not be a key.</span>
                </div>
                {generalColumnProblems.map((p) => <p key={`${p.field}|${p.message}`} className="fail-text">{p.message}</p>)}
              </>
            )}
          </fieldset>
        )}

        {step === 3 && (
          <fieldset className="teach-card docs-check">
            <legend>Check and save</legend>
            {!checked ? (
              <p className="loading">Checking…</p>
            ) : (
              <ul className="docs-checks" aria-label="Checks">
                {checks.map((c) => {
                  const own = problems.filter((p) => c.covers(p.field))
                  const skipped = form.mode === 'none' && (c.name === 'Record type' || c.name === 'Columns')
                  return (
                    <li key={c.name} className={own.length > 0 ? 'bad' : skipped ? 'skipped' : 'good'}>
                      <span className="docs-check-mark" aria-hidden="true">{own.length > 0 ? '✕' : skipped ? '–' : '✓'}</span>
                      <span className="docs-check-name">{c.name}</span>
                      <span className="docs-check-text">
                        {own.length > 0 ? own.map((p) => p.message).join(' ') : skipped ? 'Not added yet' : 'Passes'}
                      </span>
                    </li>
                  )
                })}
              </ul>
            )}
            {checked && (
              <p className="admin-readback" aria-live="polite">
                {blocked
                  ? 'Fix what is marked before saving.'
                  : draft
                    ? 'This saves a Draft: it is offered nowhere until its record structure is complete.'
                    : 'Ready. This document will be offered in the layout wizard for files in its formats.'}
              </p>
            )}
            <p className="muted">Saving creates no layout, list detection, column mapping or rule. Each is a separate step.</p>
          </fieldset>
        )}

        {fault && <p className="error" role="alert">{fault}</p>}

        <div className="admin-actions docs-buttons">
          <button type="button" onClick={onClose}>Cancel</button>
          {step > 1 && <button type="button" disabled={busy} onClick={() => setStep(step - 1)}>Back</button>}
          {step < 3 && <button type="submit" className="action" disabled={busy}>{busy ? 'Checking…' : 'Next'}</button>}
          {step === 3 && (
            <button type="submit" className="action" disabled={busy || !checked || blocked}>
              {busy ? 'Saving…' : draft ? 'Save as Draft' : 'Save document'}
            </button>
          )}
        </div>
      </form>
    </Modal>
  )
}

function swap<T>(list: T[], a: number, b: number): T[] {
  const out = [...list]
  ;[out[a], out[b]] = [out[b], out[a]]
  return out
}
