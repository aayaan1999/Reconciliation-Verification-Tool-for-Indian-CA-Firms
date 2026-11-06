import '../styles/teach.css'
import '../styles/formats.css'
import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { ApiError, inspectFile, teachLayout } from '../api.ts'
import { CheckIcon, ChevronIcon } from '../icons.tsx'
import { MultiSelect } from './MultiSelect.tsx'
import { columnText, columnsOf, guessColumn, lastKey, norm, periodFormat, rowOptions, gstinPattern, singleFields, type FieldOption } from '../layout.ts'
import type { Inspection, LayoutMapping, TeachResult } from '../types.ts'
import { DryRunView } from './DryRunView.tsx'
import { Modal } from './Modal.tsx'

interface Props {
  file: File
  onSaved: (r: TeachResult) => void
  onCancel: () => void
  // Editing an office format (SPEC-016): the screen opens with this mapping, the name and the
  // document stay as they are, and saving makes a draft version that is published later.
  edit?: { formatId: string; mapping: LayoutMapping }
  // The Partner may keep the sample file, so that publishing a later change can read it again.
  canKeepSample?: boolean
}

type Step = 1 | 2 | 3

const steps: { n: Step; name: string }[] = [
  { n: 1, name: 'About the file' },
  { n: 2, name: 'Map the columns' },
  { n: 3, name: 'Preview and save' },
]

// The mapping fields chosen on the first step; a problem with any other is fixed on the columns step.
const aboutFields = ['name', 'document_type', 'rows', 'unit']

const asApiError = (err: unknown) => (err instanceof ApiError ? err : new ApiError('ERROR', String(err)))

// TeachLayout is the mapping screen (SPEC-004 part 8.4): for a file no known format
// matches, the user says once which field is which; the server checks the choices against
// the file, shows the rows it would read, and saves the layout for every later file. It is a
// wizard of three steps (Recon Screen Document v2 §6.3): about the file, the columns, and a
// preview that must read cleanly before the layout can be saved.
export function TeachLayout({ file, onSaved, onCancel, edit, canKeepSample }: Props) {
  const [inspection, setInspection] = useState<Inspection | null>(null)
  const [error, setError] = useState<ApiError | null>(null)
  const [busy, setBusy] = useState(false)
  const [preview, setPreview] = useState<TeachResult | null>(null)
  const [step, setStep] = useState<Step>(1)
  const [discarding, setDiscarding] = useState(false) // asking before the mapping is thrown away

  const [name, setName] = useState('')
  const [docType, setDocType] = useState('')
  const [rows, setRows] = useState('')
  const [rowsSuggested, setRowsSuggested] = useState(false) // the list was picked by the system, not yet changed or confirmed
  const [unit, setUnit] = useState<LayoutMapping['unit']>('rupees')
  const [negative, setNegative] = useState<'' | 'drcr'>('')
  const [keepSample, setKeepSample] = useState(false)
  const [columns, setColumns] = useState<Record<string, string>>({})
  const [suggested, setSuggested] = useState<Record<string, boolean>>({}) // guessed, not yet changed or confirmed
  const [dateFormats, setDateFormats] = useState<Record<string, boolean>>({})
  const [where, setWhere] = useState<{ path: string; values: string[] }>({ path: '', values: [] }) // the rows kept: the field and the value, or values, it may hold
  const [periodMode, setPeriodMode] = useState<'field' | 'fixed'>('field')
  const [period, setPeriod] = useState({ path: '', format: '', const: '' })
  const [taxYear, setTaxYear] = useState('')
  const [gstinPath, setGstinPath] = useState('')
  const [identify, setIdentify] = useState({ path: '', equals: '' })
  const editMapping = edit?.mapping
  const excel = /\.xlsx$/i.test(file.name) // a workbook: its values are columns of its sheets

  useEffect(() => {
    let cancelled = false
    inspectFile(file)
      .then((ins) => {
        if (cancelled) return
        setInspection(ins)
        // First guesses: the longest list of objects, and the file's period and GSTIN.
        const lists = ins.lists.filter((l) => rowOptions(ins.fields, l.path).length >= 2)
        const best = [...lists].sort((a, b) => b.count - a.count)[0]
        if (best) {
          setRows(best.path)
          setRowsSuggested(true)
        }
        const top = singleFields(ins.fields, excel)
        const p = top.find((f) => ['period', 'fp', 'month', 'returnperiod', 'taxperiod', 'retperiod'].includes(norm(lastKey(f.path))))
        if (p) setPeriod({ path: p.path, format: periodFormat(p.example), const: '' })
        else setPeriodMode('fixed')
        const g = top.find((f) => gstinPattern.test(f.example))
        if (g) setGstinPath(g.path)
        setName(file.name.replace(/\.(json|xlsx|csv)$/i, '').replace(/[_-]?(19|20)\d{2}.*$/, '').replace(/[_-]+/g, ' ').trim())
        if (editMapping) {
          // The format's own mapping replaces the guesses: nothing is "suggested" any more.
          const m = editMapping
          setName(m.name)
          setDocType(m.document_type)
          setRows(m.rows)
          setRowsSuggested(false)
          setUnit(m.unit)
          setNegative(m.negative === 'drcr' ? 'drcr' : '')
          setColumns(Object.fromEntries(m.columns.map((c) => [c.name, c.path])))
          setSuggested({})
          setDateFormats(Object.fromEntries(m.columns.map((c) => [c.name, c.date_format !== ''])))
          setWhere(m.where[0] ? { path: m.where[0].path, values: m.where[0].in?.length ? m.where[0].in : [m.where[0].equals] } : { path: '', values: [] })
          setPeriodMode(m.period.path ? 'field' : 'fixed')
          setPeriod({ path: m.period.path, format: m.period.format, const: m.period.const })
          setTaxYear(m.tax_year)
          setGstinPath(m.gstin_path)
          setIdentify(m.identify[0] ?? { path: '', equals: '' })
        }
      })
      .catch((err) => !cancelled && setError(asApiError(err)))
    return () => {
      cancelled = true
    }
  }, [file, excel, editMapping])

  const type = inspection?.document_types.find((d) => d.name === docType)
  const options = useMemo(() => (inspection ? rowOptions(inspection.fields, rows) : []), [inspection, rows])
  const topFields = useMemo(() => singleFields(inspection?.fields ?? [], excel), [inspection, excel])
  const list = inspection?.lists.find((l) => l.path === rows)
  // The lists worth choosing from: those with at least two fields to read, and the one already chosen.
  const listChoices = useMemo(
    () => (inspection ? inspection.lists.filter((l) => l.path === rows || rowOptions(inspection.fields, l.path).length >= 2) : []),
    [inspection, rows],
  )
  const onlyList = listChoices.length === 1 ? listChoices[0] : null

  // Changing the list guesses the columns again from its fields and drops a row filter that
  // named a field of the old list; step 3 reads the file again, so the preview shows the new rows.
  const chooseList = (r: string) => {
    setRows(r)
    setRowsSuggested(false)
    setWhere({ path: '', values: [] })
    guess(docType, r)
  }

  // A new document type or list of rows guesses the columns again, each marked Suggested.
  const guess = (dt: string, r: string) => {
    const t = inspection?.document_types.find((d) => d.name === dt)
    const opts = inspection ? rowOptions(inspection.fields, r) : []
    const cols: Record<string, string> = {}
    const marks: Record<string, boolean> = {}
    const dates: Record<string, boolean> = {}
    for (const c of t?.columns ?? []) {
      cols[c.name] = guessColumn(c.name, opts)
      marks[c.name] = cols[c.name] !== ''
      const ex = opts.find((o) => o.path === cols[c.name])?.example ?? ''
      dates[c.name] = /^\d{2}-\d{2}-\d{4}$/.test(ex)
    }
    setColumns(cols)
    setSuggested(marks)
    setDateFormats(dates)
  }

  const mapping = (): LayoutMapping => ({
    name,
    document_type: docType,
    rows,
    where: where.path && where.values.length > 0 ? [where.values.length === 1 ? { path: where.path, equals: where.values[0] } : { path: where.path, equals: '', in: where.values }] : [],
    unit,
    negative,
    columns: (type?.columns ?? [])
      .filter((c) => columns[c.name])
      .map((c) => ({ name: c.name, path: columns[c.name], date_format: dateFormats[c.name] ? 'dd-mm-yyyy' : '' })),
    period: periodMode === 'field' ? { path: period.path, format: period.format, const: '' } : { path: '', format: '', const: period.const },
    tax_year: taxYear,
    gstin_path: gstinPath,
    identify: identify.path ? [identify] : [],
  })

  const send = async (checkOnly: boolean) => {
    setBusy(true)
    setError(null)
    if (checkOnly) setPreview(null)
    try {
      const res = await teachLayout(file, mapping(), checkOnly, edit?.formatId, canKeepSample && keepSample)
      setPreview(res)
      if (!checkOnly) onSaved(res)
    } catch (err) {
      setError(asApiError(err))
    } finally {
      setBusy(false)
    }
  }

  const missingKey = (type?.columns ?? []).some((c) => c.key && !columns[c.name])
  const canLeave: Record<Step, boolean> = { 1: Boolean(docType && rows), 2: !missingKey, 3: false }

  // Opening the preview always reads the file again with the choices as they are now, so the
  // rows shown are the rows the saved layout reads.
  const goTo = (s: Step) => {
    if (busy) return
    setDiscarding(false)
    if (s !== 3) {
      setPreview(null)
      setError(null)
    }
    setStep(s)
    if (s === 3) void send(true)
  }

  // Enter (in a text field, or on Next) moves on; the last step saves only by its button.
  const submit = (e: FormEvent) => {
    e.preventDefault()
    // The wizard is drawn inside the upload form; its submit must not run the checks there.
    e.stopPropagation()
    if (step < 3 && canLeave[step]) goTo((step + 1) as Step)
  }

  // Cancel or the close button asks before choices made here are thrown away; the close button
  // again (or Discard) discards them.
  const close = () => {
    if (discarding || !docType) onCancel()
    else setDiscarding(true)
  }

  const filterValues = options.find((o) => o.path === where.path)?.values ?? []
  // A built-in column keeps its wording from columnText; an office column shows the label the Partner gave it in the Document Master.
  const label = (column: string) => columnText[column] ?? type?.columns.find((c) => c.name === column)?.label ?? column
  const fieldSelect = (aria: string, value: string, opts: FieldOption[], onChange: (v: string) => void, none = 'Not in the file') => (
    <select aria-label={aria} value={value} onChange={(e) => onChange(e.target.value)}>
      <option value="">{none}</option>
      {opts.map((o) => (
        <option key={o.path} value={o.path}>
          {o.path}
          {o.example ? ` (e.g. ${o.example})` : ''}
        </option>
      ))}
    </select>
  )
  const topOptions: FieldOption[] = topFields.map((f) => ({ path: f.path, example: f.example }))
  const mappedTo = columnsOf(columns)
  const mappedColumns = (type?.columns ?? []).filter((c) => columns[c.name])
  const clean = Boolean(preview && !error && !busy && preview.rows > 0)
  const fixStep: Step = error?.problems.length && error.problems.every((p) => aboutFields.includes(p.field)) ? 1 : 2

  const head = (
    <div className="teach-head">
      <p className="muted">
        {edit
          ? `Change how files in this layout are read. ${file.name} is the sample. Saving makes a draft; it is used only after it is published.`
          : `${file.name} matches no known format. Say once which field is which; from then on every file in this layout is read by itself.`}
        {inspection?.recognised_as && ` (This file is already read as ${inspection.recognised_as}.)`}
      </p>
      <ol className="teach-steps" aria-label="Steps">
        {steps.map((s) => (
          <li key={s.n} className={s.n < step ? 'done' : s.n === step ? 'current' : undefined} aria-current={s.n === step ? 'step' : undefined}>
            <span className="teach-step-mark" aria-hidden="true">
              {s.n < step ? <CheckIcon /> : s.n}
            </span>
            {s.name}
          </li>
        ))}
      </ol>
    </div>
  )

  if (!inspection) {
    return (
      <Modal title={edit ? 'Edit this layout' : 'Teach this layout'} onClose={onCancel} drawer>
        <div className="teach">
          {head}
          {error ? (
            <p className="error" role="alert">
              <strong>{error.code}</strong>: {error.message}
            </p>
          ) : (
            <p className="muted" role="status">Reading the fields of the file…</p>
          )}
          <div className="form-actions teach-actions">
            <button type="button" className="ghost" onClick={onCancel}>Cancel</button>
          </div>
        </div>
      </Modal>
    )
  }

  // The values of the file beside the mapping, each field labelled with the columns mapped to it.
  const firstValues = (
    <aside className="teach-aside" aria-label="First values in the file">
      <p className="eyebrow">First values in the file</p>
      <p className="muted teach-aside-sub">
        <span className="mono">{rows}</span>
        {list && ` · ${list.count} ${list.count === 1 ? 'row' : 'rows'}`}
      </p>
      <ul>
        {options.map((o) => (
          <li key={o.path} className={mappedTo[o.path] ? 'mapped' : undefined}>
            <span className="mono teach-path">{o.path}</span>
            <span className="mono teach-example">{o.example}</span>
            {mappedTo[o.path] && <span className="teach-mapped">{mappedTo[o.path].map(label).join(', ')}</span>}
          </li>
        ))}
      </ul>
    </aside>
  )

  return (
    <Modal title={edit ? 'Edit this layout' : 'Teach this layout'} onClose={close} drawer>
      <form className="teach" aria-label={edit ? 'Edit this layout' : 'Teach this layout'} onSubmit={submit}>
        {head}

        {step === 1 && (
          <fieldset className="teach-card teach-about">
            <legend>About the file</legend>
            <label>
              Name of the layout
              <input type="text" aria-label="Name of the layout" value={name} maxLength={60} autoFocus={!edit} readOnly={Boolean(edit)} onChange={(e) => setName(e.target.value)} />
            </label>
            <label>
              Document
              <select aria-label="Document" value={docType} disabled={Boolean(edit)} onChange={(e) => { setDocType(e.target.value); guess(e.target.value, rows) }}>
                <option value="">Choose…</option>
                {inspection.document_types.map((d) => (
                  <option key={d.name} value={d.name}>{d.description}</option>
                ))}
              </select>
              {inspection.document_types.length === 0 && (
                <span className="help" role="status">No active document accepts a file of this format; the Partner can add one in Document Master.</span>
              )}
            </label>
            {onlyList ? (
              <label>
                List of rows (one per invoice)
                <input type="text" readOnly aria-label="List of rows" className="mono" value={onlyList.path} />
                <span className="help">The only list of rows in the file · {onlyList.count} {onlyList.count === 1 ? 'item' : 'items'}</span>
              </label>
            ) : (
              <div className="teach-listpick">
                <label>
                  List of rows (one per invoice)
                  <select aria-label="List of rows" value={rows} onChange={(e) => chooseList(e.target.value)}>
                    <option value="">Choose…</option>
                    {listChoices.map((l) => (
                      <option key={l.path} value={l.path}>{`${l.path} (${l.count} items)`}</option>
                    ))}
                  </select>
                </label>
                {rowsSuggested && rows && (
                  <button type="button" className="teach-suggested" aria-label={`Confirm list of rows: ${rows}`}
                    title="Suggested: the longest list in the file. Choose another list, or press to confirm this one."
                    onClick={() => setRowsSuggested(false)}>
                    Suggested
                  </button>
                )}
              </div>
            )}
            <label>
              Amounts are in
              <select aria-label="Amounts are in" value={unit} onChange={(e) => setUnit(e.target.value as typeof unit)}>
                <option value="rupees">Rupees (1,12,450.50 is Rs 1,12,450.50)</option>
                <option value="paise">Paise (11245050 is Rs 1,12,450.50)</option>
                <option value="lakhs">Lakhs (12.5 is Rs 12,50,000)</option>
                <option value="crores">Crores (1.25 is Rs 1,25,00,000)</option>
              </select>
              <span className="help">One unit for every amount column of this layout. An amount written with $ or USD is converted at the dollar rate set on the Admin tab, whatever you choose here.</span>
            </label>
            <fieldset className="teach-unit">
              <legend>Negative amounts are written</legend>
              <label className="check">
                <input type="radio" name="teach-negative" checked={negative === ''} onChange={() => setNegative('')} />
                With a minus sign or in brackets
              </label>
              <label className="check">
                <input type="radio" name="teach-negative" checked={negative === 'drcr'} onChange={() => setNegative('drcr')} />
                With Dr or Cr after the amount (Cr is negative)
              </label>
            </fieldset>
            {!canLeave[1] && <p className="muted">Choose the document and the list of rows to go on.</p>}
          </fieldset>
        )}

        {step === 2 && type && (
          <div className="teach-body">
            <div className="teach-main">
              <div className="teach-card">
                <table className="grid compact teach-columns" aria-label="Which field is each column">
                  <thead>
                    <tr>
                      <th>Column</th>
                      <th>Field in the file</th>
                      <th>Example value</th>
                    </tr>
                  </thead>
                  <tbody>
                    {type.columns.map((c, i) => {
                      const path = columns[c.name] ?? ''
                      const example = options.find((o) => o.path === path)?.example
                      return (
                        <tr key={c.name}>
                          <td>
                            {label(c.name)}
                            {c.key && (
                              <>
                                <span className="teach-required" aria-hidden="true"> *</span>
                                <span className="teach-help">Needed: rows are matched on it</span>
                              </>
                            )}
                            {c.required && !c.key && (
                              <>
                                <span className="teach-required" aria-hidden="true"> *</span>
                                <span className="teach-help">Marked required in the Document Master</span>
                              </>
                            )}
                          </td>
                          <td>
                            <div className="teach-field">
                              <select aria-label={label(c.name)} value={path} autoFocus={i === 0}
                                onChange={(e) => {
                                  setColumns({ ...columns, [c.name]: e.target.value })
                                  setSuggested({ ...suggested, [c.name]: false })
                                }}>
                                <option value="">Not in the file</option>
                                {options.map((o) => (
                                  <option key={o.path} value={o.path}>{o.path}</option>
                                ))}
                              </select>
                              {suggested[c.name] && (
                                <button type="button" className="teach-suggested" aria-label={`Confirm ${label(c.name)}: ${path}`}
                                  title="Suggested from the field name. Choose another field, or press to confirm this one."
                                  onClick={() => setSuggested({ ...suggested, [c.name]: false })}>
                                  Suggested
                                </button>
                              )}
                            </div>
                            {(c.type ? c.type === 'date' : c.name.endsWith('_date')) && (
                              <label className="check">
                                <input type="checkbox" aria-label={`${label(c.name)} is written dd-mm-yyyy`} checked={dateFormats[c.name] ?? false}
                                  onChange={(e) => setDateFormats({ ...dateFormats, [c.name]: e.target.checked })} />
                                Written dd-mm-yyyy
                              </label>
                            )}
                          </td>
                          <td className="mono teach-example">{example || '–'}</td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>

              <fieldset className="teach-card">
                <legend>Which rows are read</legend>
                <div className="teach-row">
                  <label>
                    Read only rows where
                    {fieldSelect('Row filter field', where.path, options.filter((o) => !o.path.startsWith('^')), (v) => setWhere({ path: v, values: [] }), 'Every row')}
                  </label>
                  {where.path && (filterValues.length > 0 ? (
                    <MultiSelect
                      label="is any of"
                      allText="Choose values"
                      captionClass="teach-filter-caption"
                      options={filterValues.map((v) => ({ value: v, label: v }))}
                      value={where.values}
                      onChange={(values) => setWhere({ ...where, values })}
                      showValues
                    />
                  ) : (
                    <label>
                      equals
                      <input type="text" aria-label="Row filter value" value={where.values[0] ?? ''} onChange={(e) => setWhere({ ...where, values: e.target.value ? [e.target.value] : [] })} />
                    </label>
                  ))}
                </div>
              </fieldset>

              <fieldset className="teach-card">
                <legend>Period and client</legend>
                <div className="teach-row">
                  <label className="check teach-option">
                    <input type="radio" name="period-mode" checked={periodMode === 'fixed'} onChange={() => setPeriodMode('fixed')} />
                    Same period for every file
                  </label>
                  <label className="check teach-option">
                    <input type="radio" name="period-mode" checked={periodMode === 'field'} onChange={() => setPeriodMode('field')} />
                    Period from a field
                  </label>
                </div>
                <div className="teach-row">
                  {periodMode === 'field' ? (
                    <>
                      <label>
                        Period field
                        {fieldSelect('Period field', period.path, topOptions, (v) => {
                          const ex = topOptions.find((o) => o.path === v)?.example ?? ''
                          setPeriod({ ...period, path: v, format: periodFormat(ex) })
                        }, 'Choose…')}
                      </label>
                      <label>
                        Written as
                        <select aria-label="Period written as" value={period.format} onChange={(e) => setPeriod({ ...period, format: e.target.value })}>
                          <option value="">2026-08</option>
                          <option value="mmyyyy">082026 (mmyyyy)</option>
                          <option value="yyyymm">202608 (yyyymm)</option>
                        </select>
                      </label>
                    </>
                  ) : (
                    <label>
                      Period
                      <input type="text" aria-label="Period" placeholder="2026-08, or FY for the whole year" value={period.const}
                        onChange={(e) => setPeriod({ ...period, const: e.target.value })} />
                    </label>
                  )}
                  {periodMode === 'fixed' && period.const === 'FY' && (
                    <label>
                      Tax year
                      <input type="text" aria-label="Tax year of the layout" placeholder="2026-27" value={taxYear} onChange={(e) => setTaxYear(e.target.value)} />
                    </label>
                  )}
                </div>
                <div className="teach-row">
                  <label>
                    Client&apos;s own GSTIN
                    {fieldSelect("Client's GSTIN field", gstinPath, topOptions, setGstinPath)}
                  </label>
                  <label>
                    Only this layout has (optional)
                    {fieldSelect('Identifying field', identify.path, topOptions, (v) => setIdentify({ path: v, equals: topOptions.find((o) => o.path === v)?.example ?? '' }), 'Nothing more')}
                  </label>
                  {identify.path && (
                    <label>
                      equal to
                      <input type="text" aria-label="Identifying value" value={identify.equals} onChange={(e) => setIdentify({ ...identify, equals: e.target.value })} />
                    </label>
                  )}
                </div>
              </fieldset>
              {missingKey && <p className="muted">Choose the field for each column marked *; rows are matched on it.</p>}
            </div>
            {firstValues}
          </div>
        )}

        {step === 3 && (
          <div className="teach-body">
            <div className="teach-main">
              <div className="teach-intro">
                <p className="muted">
                  The rows exactly as they will be read, with the place each came from. A wrong choice gives an error here instead of a wrong figure.
                </p>
                <button type="button" className="secondary-button" disabled={busy} onClick={() => send(true)}>
                  Preview again
                </button>
              </div>
              {busy && (
                <>
                  <p className="muted" role="status">Reading the rows…</p>
                  <div className="progress" role="progressbar" aria-label="Checking the layout" />
                </>
              )}

              {error && (
                <div className="error" role="alert">
                  <strong>{error.code}</strong>: {error.message}
                  {error.problems.length > 1 && (
                    <ul>
                      {error.problems.map((p, i) => (
                        <li key={i}>{p.field}: {p.message}</li>
                      ))}
                    </ul>
                  )}
                  <button type="button" className="link teach-fix" onClick={() => goTo(fixStep)}>
                    {fixStep === 1 ? 'Back to About the file' : 'Back to the columns'}
                  </button>
                </div>
              )}

              {preview && preview.rows === 0 && (
                <div className="error" role="alert">
                  No rows were read from {rows}.
                  <button type="button" className="link teach-fix" onClick={() => goTo(2)}>Back to the columns</button>
                </div>
              )}

              {preview && preview.rows > 0 && (
                <div aria-label="Rows read">
                  <p className="teach-read" role="status">
                    <CheckIcon />
                    <span>
                      <strong>{preview.rows} rows read</strong> · period {preview.period}
                      {preview.tax_year && ` · tax year ${preview.tax_year}`}
                      {preview.pan_last4 && ` · PAN ending ${preview.pan_last4}`}
                      {preview.warnings > 0 && ` · ${preview.warnings} ${preview.warnings === 1 ? 'warning' : 'warnings'} while reading`}
                      . {edit ? 'The edit can be saved as a draft.' : 'The layout can be saved.'}
                    </span>
                  </p>
                  {preview.dry_run && <DryRunView dry={preview.dry_run} />}
                  {preview.preview.length < preview.rows && <p className="muted">The first {preview.preview.length} rows:</p>}
                  <div className="teach-card teach-scroll">
                    <table className="grid compact">
                      <thead>
                        <tr>
                          <th>Read from</th>
                          {mappedColumns.map((c) => (
                            <th key={c.name} className={c.amount ? 'num' : undefined}>{label(c.name)}</th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {preview.preview.map((r) => (
                          <tr key={r.locator}>
                            <td className="mono">{r.locator}</td>
                            {mappedColumns.map((c) => (
                              <td key={c.name} className={c.amount ? 'num' : undefined}>{c.amount ? r.amounts[c.name] : r.fields[c.name]}</td>
                            ))}
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}
            </div>
            {firstValues}
          </div>
        )}

        {step === 3 && canKeepSample && (
          <label className="check teach-keep">
            <input type="checkbox" checked={keepSample} onChange={(e) => setKeepSample(e.target.checked)} />
            <span>
              Keep this sample file, so that publishing a later change can read it again.
              <span className="muted"> The file is stored in the database as it is. Keep it only if its figures are invented or masked of names, PANs and GSTINs.</span>
            </span>
          </label>
        )}

        <div className="form-actions teach-actions">
          {step > 1 && (
            <button type="button" className="ghost" disabled={busy} onClick={() => goTo((step - 1) as Step)}>
              <ChevronIcon dir="left" /> Back
            </button>
          )}
          {discarding ? (
            <span className="confirm" role="group" aria-label="Discard this mapping?">
              Discard this mapping? The choices made here are not kept.
              <button type="button" className="danger" onClick={onCancel}>Discard</button>
              <button type="button" autoFocus onClick={() => setDiscarding(false)}>Keep editing</button>
            </span>
          ) : (
            <button type="button" className="ghost" onClick={close}>Cancel</button>
          )}
          {step === 1 && (
            <button type="submit" disabled={!canLeave[1]}>
              Next <ChevronIcon dir="right" />
            </button>
          )}
          {step === 2 && (
            <button type="submit" disabled={!canLeave[2]}>
              Preview rows <ChevronIcon dir="right" />
            </button>
          )}
          {step === 3 && (
            <button type="button" className="primary" disabled={!clean} onClick={() => send(false)}>
              <CheckIcon /> {busy && preview ? 'Saving…' : edit ? 'Save as draft' : 'Save layout'}
            </button>
          )}
        </div>
      </form>
    </Modal>
  )
}
