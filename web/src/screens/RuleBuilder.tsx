import { useEffect, useId, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { ApiError, approveRule, saveRule, switchRule } from '../api.ts'
import {
  blankFixture, blankForm, blankInput, builderSections, checkFields, checkTypeText, fieldLabel, findingSentence, forSave, parseList,
  requiredLeft, type BuilderSection,
} from '../ruleForm.ts'
import type {
  RuleCurrent, RuleForm, RuleFormCondition, RuleFormFixture, RuleFormInput, RuleFormLaw, RuleFormNamed, RuleFormPart,
  RuleFormFact, RuleFormRecord, RuleFormTableRow, RuleOptions, RuleSaved,
} from '../types.ts'
import { builderTaxYears } from '../filters.ts'
import { Modal } from './Modal.tsx'
import '../styles/rules.css'

// The Rule Builder page (SPEC-010 §2; Recon Screen Document v2 §10.3): a rule is filled in with
// plain fields, checked by the engine, and saved as a testing office rule. Nothing here decides
// whether a rule is right: the engine refuses a rule whose fields, fixtures or check do not hold,
// and names the field. The page replaces the rule list while it is open; the navigation on the
// left counts the required fields left in each section, and the bar at the bottom saves.

// The tests of a present_together condition (technical spec §B4.3).
const conditionNames = ['blank', 'not_blank', 'zero', 'nonzero', 'equals', 'matches', 'in', 'not_in', 'true', 'false', 'same_as']

interface Props {
  mode: 'create' | 'edit'
  current: RuleCurrent | null // edit: the rule as it is now
  options: RuleOptions
  taxYears: string[] // create: the tax years offered
  onSaved: (saved: RuleSaved) => void
  onApprove?: () => void // edit: a testing rule that may be approved; the Rules tab opens the approval
  onClose: () => void
}

export function RuleBuilder({ mode, current, options, taxYears, onSaved, onApprove, onClose }: Props) {
  const [form, setForm] = useState<RuleForm>(() =>
    mode === 'edit' && current?.form ? current.form : blankForm(options, taxYears[0] ?? ''),
  )
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const [saved, setSaved] = useState<RuleSaved | null>(null)
  // The guide is collapsed by default in both modes, so the form has the room. The header
  // scrolls away with the form (it is not sticky); the toggle brings the guide back.
  const [showHelp, setShowHelp] = useState(false)
  const page = useRef<HTMLDivElement>(null)
  const sectionId = useId()
  const readOnly = mode === 'edit' && current !== null && !current.editable
  const problems = error?.problems ?? []
  const problemFor = (field: string) => problems.find((p) => p.field === field)?.message

  const set = (patch: Partial<RuleForm>) => setForm((f) => ({ ...f, ...patch }))
  const setCheck = (patch: Partial<RuleForm['check']>) => setForm((f) => ({ ...f, check: { ...f.check, ...patch } }))

  // The page opens on its first field to fill in, and on the way back once saved.
  useEffect(() => {
    page.current?.querySelector<HTMLElement>('input:not([readonly]), select, textarea, .rb-saved button')?.focus({ preventScroll: true })
  }, [saved])

  const save = async () => {
    if (busy || readOnly) return
    setBusy(true)
    setError(null)
    try {
      const out = await saveRule(forSave({ ...form, mode }))
      setSaved(out)
      onSaved(out)
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError('ERROR', String(err)))
    } finally {
      setBusy(false)
    }
  }
  const submit = (e: FormEvent) => {
    e.preventDefault()
    save()
  }

  const title = mode === 'create' ? 'Add rule' : `Edit ${current?.rule_id ?? ''}`
  if (saved) {
    return (
      <div ref={page} className="rb-page">
        <header className="rb-head">
          <p className="eyebrow">Rules · Rule Builder</p>
          <h1>{title}</h1>
        </header>
        <div className="rb-saved" role="status">
          <p>
            Saved {saved.rule_id} version {saved.version} as <strong>{saved.status}</strong>, kept in {saved.file}.
          </p>
          <p className="muted">
            It is waiting for approval. Only a Partner's approval makes it active; until then it runs only when a run includes
            testing rules. The Rules tab labels it "testing".
          </p>
          <button type="button" className="primary" onClick={onClose}>
            Back to rules
          </button>
        </div>
      </div>
    )
  }

  const use = checkFields(form.check.type)
  const inputNames = form.inputs.map((i) => i.name).filter((n) => n !== '')
  const factInputs = form.inputs.filter((i) => i.kind === 'fact' && i.name !== '')
  const recordInputs = form.inputs.filter((i) => i.kind === 'record_type' && i.name !== '')
  const docText = (name: string) => options.document_types.find((d) => d.name === name)?.description ?? name
  const left = requiredLeft(form, mode, (current?.fixtures ?? []).map((f) => f.kind))
  const totalLeft = Object.values(left).reduce((a, b) => a + b, 0)
  const idOf = (s: BuilderSection) => `${sectionId}-${s}`
  // A section is reached from the navigation: it scrolls into view and its first field takes focus.
  const goTo = (s: BuilderSection) => {
    const el = document.getElementById(idOf(s))
    el?.scrollIntoView?.({ block: 'start', behavior: 'smooth' })
    el?.querySelector<HTMLElement>('input:not([readonly]):not(:disabled), select:not(:disabled), textarea:not(:disabled), button:not(:disabled)')?.focus({ preventScroll: true })
  }
  const leftText = (n: number) => (n === 0 ? 'done' : `${n} required left`)
  const taxYearChoices = mode === 'create' ? [...new Set([...builderTaxYears(), ...form.tax_years])].sort().reverse() : form.tax_years

  return (
    <div ref={page} className="rb-page">
      <form className="client-form rule-builder" aria-label={title} onSubmit={submit}>
        <header className="rb-head">
          <div>
            <p className="eyebrow">Rules · Rule Builder</p>
            <h1>{title}</h1>
          </div>
        </header>
        <section className="rule-head" aria-label={title}>
          <div className="rule-head-bar">
            <p className="muted">
              Fields marked <span className="req">*</span> are required.
            </p>
            <button
              type="button"
              className="ghost rule-head-toggle"
              aria-label="How to add a rule"
              aria-expanded={showHelp}
              aria-controls="rule-help"
              title={showHelp ? 'Hide the guide' : 'Show the guide'}
              onClick={() => setShowHelp((v) => !v)}
            >
              Guide <span aria-hidden="true">{showHelp ? '▴' : '▾'}</span>
            </button>
          </div>
          {showHelp && (
            <div id="rule-help" className="rule-help">
              <p>
                <strong>How to add a rule</strong>
              </p>
              <ol>
              <li>
                <strong>Who and what:</strong> a new ID such as RECON-DEP-902, a title and the rule text in plain English. The change is written in the audit log with your login name.
              </li>
              <li>
                <strong>Where it applies:</strong> choose the module, the category, the tax years and the valid-from date, and give at
                least one legal basis with its Act and section.
              </li>
              <li>
                <strong>What it reads:</strong> click "Add a figure or rows" for each figure the rule reads, and pick it from the list.
              </li>
              <li>
                <strong>The check:</strong> choose the check type, then the figures it compares. Type a tolerance in rupees only if a
                small difference is to pass. The sentence under the check states when a finding is raised. Choose the severity and the
                review document.
              </li>
              <li>
                <strong>Examples:</strong> add one pass fixture (figures that agree) and one fail fixture (figures that disagree). Fill in
                each figure's value, period, source file and locator.
              </li>
              <li>
                <strong>Save rule:</strong> Save as testing in the bar at the bottom. If a field is missing or wrong, the tool
                lists it at the top of this page. The rule is saved as testing.
              </li>
              </ol>
            </div>
          )}
        </section>
        {readOnly && (
          <p className="notice" role="note">
            {current?.reason} The page shows the rule but cannot save a change to it.
          </p>
        )}
        {problems.length > 0 && (
          <div className="notice" role="alert">
            <p>The rule was not saved. Fix these:</p>
            <ul>
              {problems.map((p, i) => (
                <li key={`${p.field}-${i}`}>
                  <strong>{fieldLabel(p.field)}:</strong> {p.message}
                </li>
              ))}
            </ul>
          </div>
        )}
        {error && problems.length === 0 && <p className="fail-text" role="alert">{error.message}</p>}
        <div className="rb-body">
        <nav className="rb-nav" aria-label="Sections of the rule">
          <ol>
            {builderSections.map((s) => (
              <li key={s.key}>
                <button type="button" className={left[s.key] === 0 ? 'rb-nav-item done' : 'rb-nav-item'} onClick={() => goTo(s.key)}>
                  <span>{s.title}</span>
                  <span className="rb-nav-left">{leftText(left[s.key])}</span>
                </button>
              </li>
            ))}
          </ol>
        </nav>
        <div className="rb-sections">
        <fieldset id={idOf('who')} className="rb-section" disabled={readOnly || busy}>
          <legend>Who and what</legend>
          <div className="form-section-premium">
            {mode === 'create' ? (
              <Field required label="ID" help="RECON, a short prefix of capitals, and three digits, such as RECON-DEP-002. It cannot be changed later." problem={problemFor('id')}>
                {(id) => <input id={id} type="text" value={form.id} maxLength={40} required onChange={(e) => set({ id: e.target.value.toUpperCase() })} />}
              </Field>
            ) : (
              <Field label="ID" help="A rule's ID does not change.">
                {(id) => <input id={id} type="text" value={form.id} readOnly />}
              </Field>
            )}
            <Field required label="Title" help="What the rule is called, in a few words." problem={problemFor('title')}>
              {(id) => <input id={id} type="text" value={form.title} maxLength={200} onChange={(e) => set({ title: e.target.value })} />}
            </Field>
            <Field required label="Rule text" help="Plain English: what is compared. Stored as text only; the engine never reads it." problem={problemFor('rule_text')}>
              {(id) => <textarea id={id} rows={3} value={form.rule_text} onChange={(e) => set({ rule_text: e.target.value })} />}
            </Field>
            <Field optional label="Notes" help="Your own remarks. Stored as text only.">
              {(id) => <textarea id={id} rows={2} value={form.notes} onChange={(e) => set({ notes: e.target.value })} />}
            </Field>
          </div>
        </fieldset>

        <fieldset id={idOf('where')} className="rb-section" disabled={readOnly || busy}>
          <legend>Where it applies</legend>
          <div className="form-section-premium">
            <Field required label="Module" help="The area of reconciliation the rule belongs to." problem={problemFor('module')}>
              {(id) => (
                <select id={id} value={form.module} onChange={(e) => set({ module: e.target.value })}>
                  {options.modules.map((m) => (
                    <option key={m} value={m}>{m}</option>
                  ))}
                </select>
              )}
            </Field>
            <Field required label="Category" help="The category from the list." problem={problemFor('category')}>
              {(id) => (
                <select id={id} value={form.category} onChange={(e) => set({ category: e.target.value })}>
                  {options.categories.map((c) => (
                    <option key={c.code} value={c.code}>{`${c.code}: ${c.name}`}</option>
                  ))}
                </select>
              )}
            </Field>
            <ChipField
              required={mode === 'create'}
              label="Tax years"
              help={mode === 'create' ? 'Press each tax year the rule applies to.' : 'Fixed once the rule exists.'}
              problem={problemFor('tax_years')}
              values={taxYearChoices}
              chosen={form.tax_years}
              fixed={mode === 'edit'}
              onToggle={(y) => set({ tax_years: form.tax_years.includes(y) ? form.tax_years.filter((x) => x !== y) : [...form.tax_years, y].sort() })}
            />
            <Field required label="Valid from" help="The first day the rule applies, such as 2025-04-01." problem={problemFor('valid_from')}>
              {(id) => <input id={id} type="text" value={form.valid_from} placeholder="2025-04-01" onChange={(e) => set({ valid_from: e.target.value })} />}
            </Field>
            <Field optional label="Valid to" help="Leave empty for no end date." problem={problemFor('valid_to')}>
              {(id) => <input id={id} type="text" value={form.valid_to} placeholder="2027-03-31" onChange={(e) => set({ valid_to: e.target.value })} />}
            </Field>
          </div>
          <h3 className="rb-sub">Legal basis <span className="req">*</span></h3>
          {form.law.map((l, i) => (
            <LawRow key={i} index={i} law={l} problem={(f) => problemFor(`law[${i}]${f}`)}
              onChange={(patch) => set({ law: form.law.map((x, j) => (j === i ? { ...x, ...patch } : x)) })}
              onRemove={() => set({ law: form.law.filter((_, j) => j !== i) })} />
          ))}
          <button type="button" onClick={() => set({ law: [...form.law, { act: '', section: '', reference: '' }] })}>
            Add a legal basis
          </button>
          {problemFor('law') && <p className="fail-text">{problemFor('law')}</p>}
        </fieldset>

        <fieldset id={idOf('reads')} className="rb-section" disabled={readOnly || busy}>
          <legend>What it reads <span className="req">*</span></legend>
          <p className="muted">Each figure comes from the fact vocabulary, and each row from a record type; pick from the lists.</p>
          {form.inputs.map((inp, i) => (
            <InputRow
              key={i}
              index={i}
              input={inp}
              options={options}
              problem={(sub: string) => problemFor(`inputs[${i}]${sub}`)}
              docText={docText}
              onChange={(patch) => set({ inputs: form.inputs.map((x, j) => (j === i ? { ...x, ...patch } : x)) })}
              onRemove={() => set({ inputs: form.inputs.filter((_, j) => j !== i) })}
            />
          ))}
          <button type="button" onClick={() => set({ inputs: [...form.inputs, blankInput()] })}>
            Add a figure or rows
          </button>
        </fieldset>

        <fieldset id={idOf('check')} className="rb-section" disabled={readOnly || busy}>
          <legend>The check</legend>
          <div className="form-section-premium">
            <Field required label="Check type" help={checkTypeText[form.check.type] ?? 'Choose what the rule does.'} problem={problemFor('check.type')}>
              {(id) => (
                <select id={id} value={form.check.type} onChange={(e) => setCheck({ type: e.target.value })}>
                  {options.check_types.map((t) => (
                    <option key={t} value={t}>{t}</option>
                  ))}
                </select>
              )}
            </Field>
            {use.tolerance && (
              <Field optional label="Tolerance in rupees" help="A difference up to this much still passes. Typed in rupees with at most two decimals; stored as paise. Empty for an exact match." problem={problemFor('check.tolerance')}>
                {(id) => <input id={id} type="text" value={form.check.tolerance} placeholder="1.00" onChange={(e) => setCheck({ tolerance: e.target.value })} />}
              </Field>
            )}
            {form.check.type === 'sum_equals' && (
              <>
                <Field label="Total compared with" help="The figure the figures added up must equal.">
                  {(id) => <Select id={id} value={form.check.total} names={inputNames} onChange={(v) => setCheck({ total: v })} />}
                </Field>
                <Field label="Column of the total" help="Only for rows: the amount column of each row compared. Empty for figures.">
                  {(id) => <input id={id} type="text" value={form.check.total_field} onChange={(e) => setCheck({ total_field: e.target.value })} />}
                </Field>
              </>
            )}
            {use.base && (
              <Field required label="Base" help="The figure the rate is applied to. Empty when parts are used instead." problem={problemFor('check.base')}>
                {(id) => <Select id={id} value={form.check.base} names={inputNames} onChange={(v) => setCheck({ base: v })} />}
              </Field>
            )}
            {use.rate && (
              <Field required label="Rate in per cent" help="A percentage with at most two decimals, such as 15.00." problem={problemFor('check.rate')}>
                {(id) => <input id={id} type="text" value={form.check.rate} placeholder="15.00" onChange={(e) => setCheck({ rate: e.target.value })} />}
              </Field>
            )}
            {use.value && (
              <Field required label={form.check.type === 'table_lookup' ? 'Figure looked up' : 'Figure compared'} help="The figure the check compares with the result.">
                {(id) => <Select id={id} value={form.check.value} names={inputNames} onChange={(v) => setCheck({ value: v })} />}
              </Field>
            )}
          </div>
          {use.parts && (
            <PartsEditor parts={form.check.parts} names={inputNames} onChange={(parts) => setCheck({ parts })}
              label={form.check.type === 'percent_of' ? 'Parts instead of the base' : 'Figures added or taken away'} />
          )}
          {use.when && <ConditionsEditor conditions={form.check.when} names={inputNames} onChange={(when) => setCheck({ when })} />}
          {use.keys && (
            <div className="filters">
              <Field label="Keys" help="The figures whose values pick the row of the table, separated by commas, such as a, b.">
                {(id) => <ListField id={id} value={form.check.keys} onChange={(keys) => setCheck({ keys })} />}
              </Field>
            </div>
          )}
          {use.table && <TableEditor rows={form.check.table} onChange={(table) => setCheck({ table })} />}
          {use.matching && (
            <div className="filters">
              <Field label="Amounts compared" help="Columns compared as amounts, separated by commas, such as taxable_value, igst.">
                {(id) => <ListField id={id} value={form.check.compare_amounts} onChange={(v) => setCheck({ compare_amounts: v })} />}
              </Field>
              <Field label="Other columns that must match" help="Columns that must be identical, separated by commas.">
                {(id) => <ListField id={id} value={form.check.compare_fields} onChange={(v) => setCheck({ compare_fields: v })} />}
              </Field>
              <Field label="Rows that are whole invoices" help="Inputs whose rows are invoices, such as b.">
                {(id) => <ListField id={id} value={form.check.duplicates_in} onChange={(v) => setCheck({ duplicates_in: v })} />}
              </Field>
              <Field label="Invoice amounts" help="Amounts stated once per invoice and repeated on each of its lines.">
                {(id) => <ListField id={id} value={form.check.invoice_amounts} onChange={(v) => setCheck({ invoice_amounts: v })} />}
              </Field>
              <Field label="Pair unmatched rows on" help="Columns that identify an invoice numbered differently on the two sides.">
                {(id) => <ListField id={id} value={form.check.pair_unmatched_on} onChange={(v) => setCheck({ pair_unmatched_on: v })} />}
              </Field>
            </div>
          )}
          <div className="rb-sentence" aria-live="polite">
            <p className="eyebrow">When a finding is raised</p>
            <p>{findingSentence(form)}</p>
          </div>
          <h3 className="rb-sub">Outcome</h3>
          <div className="form-section-premium">
            <Field required label="Severity" help="How serious a failure of the rule is." problem={problemFor('severity')}>
              {(id) => (
                <select id={id} value={form.severity} onChange={(e) => set({ severity: e.target.value })}>
                  {options.severities.map((s) => (
                    <option key={s} value={s}>{s}</option>
                  ))}
                </select>
              )}
            </Field>
            <Field required label="Review document" help="The document that needs fixing. It groups the findings; it never appears in finding text." problem={problemFor('review_document')}>
              {(id) => (
                <select id={id} value={form.review_document} onChange={(e) => set({ review_document: e.target.value })}>
                  {options.document_types.map((d) => (
                    <option key={d.name} value={d.name}>{d.name}</option>
                  ))}
                </select>
              )}
            </Field>
          </div>
        </fieldset>

        <fieldset id={idOf('examples')} className="rb-section" disabled={readOnly || busy}>
          <legend>Examples: a pass and a fail fixture <span className="req">*</span></legend>
          {mode === 'edit' && current && (current.fixtures ?? []).length > 0 && (
            <p className="muted">
              Kept from the rule as it is now: {(current.fixtures ?? []).map((f) => `${f.kind} "${f.name}"`).join(', ')}. A new fixture
              with the same name and kind replaces the kept one.
            </p>
          )}
          {mode === 'create' && <p className="muted">At least one pass fixture (figures that agree) and one fail fixture (figures that do not) are needed.</p>}
          {form.fixtures.map((fx, i) => (
            <FixtureBox
              key={i}
              index={i}
              fixture={fx}
              options={options}
              factNames={factInputs.map((x) => x.ref)}
              recordNames={recordInputs.map((x) => x.ref)}
              problem={(f) => problemFor(`fixtures[${i}]${f}`)}
              docText={docText}
              onChange={(patch) => set({ fixtures: (form.fixtures ?? []).map((x, j) => (j === i ? { ...x, ...patch } : x)) })}
              onRemove={() => set({ fixtures: (form.fixtures ?? []).filter((_, j) => j !== i) })}
            />
          ))}
          <div className="form-actions">
            <button type="button" onClick={() => set({ fixtures: [...(form.fixtures ?? []), blankFixture('pass')] })}>
              Add a pass fixture
            </button>
            <button type="button" onClick={() => set({ fixtures: [...(form.fixtures ?? []), blankFixture('fail')] })}>
              Add a fail fixture
            </button>
          </div>
          {problemFor('fixtures') && <p className="fail-text">{problemFor('fixtures')}</p>}
        </fieldset>
        </div>
        </div>

        <div className="rb-bar" role="group" aria-label="Save the rule">
          <span className="rb-bar-status" aria-live="polite">
            {mode === 'edit' && current?.status === 'testing' && !current.approvable && current.approval_note && (
              <span className="muted">Not ready for approval: {current.approval_note} </span>
            )}
            {totalLeft === 0 ? 'Every required field is filled in.' : `${totalLeft} required ${totalLeft === 1 ? 'field' : 'fields'} left.`}
          </span>
          <button type="button" className="ghost" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          {mode === 'edit' && onApprove && current?.status === 'testing' && current.approvable && (
            <button type="button" onClick={onApprove} disabled={readOnly || busy}>
              Approve
            </button>
          )}
          <button type="submit" disabled={readOnly || busy}>
            {busy ? 'Checking the rule…' : 'Save as testing'}
          </button>
        </div>
      </form>
    </div>
  )
}

// ChipField is a set of toggle chips under one label: each chip is pressed or not. Fixed chips
// show the choice without letting it change.
function ChipField({ label, help, problem, required, values, chosen, fixed, onToggle }: {
  label: string
  help?: string
  problem?: string
  required?: boolean
  values: string[]
  chosen: string[]
  fixed?: boolean
  onToggle: (value: string) => void
}) {
  const id = useId()
  return (
    <div className={`form-group-premium ${problem ? 'invalid' : ''}`} role="group" aria-labelledby={id}>
      <span id={id} className={required ? 'rb-label required' : 'rb-label'}>{label}</span>
      {values.length === 0 ? (
        <span className="muted">None to choose from.</span>
      ) : (
        <span className="rb-chips">
          {values.map((v) => (
            <button key={v} type="button" className="rb-chip" aria-pressed={chosen.includes(v)} disabled={fixed} onClick={() => onToggle(v)}>
              {v}
            </button>
          ))}
        </span>
      )}
      {help && <span className="help">{help}</span>}
      {problem && <span className="fail-text">{problem}</span>}
    </div>
  )
}

// Field is a labelled control. The label names the control for screen readers and tests; the
// help line says in plain words what to type.
export function Field({ label, help, problem, required, optional, children }: { label: string; help?: string; problem?: string; required?: boolean; optional?: boolean; children?: (id: string) => ReactNode }) {
  const id = useId()
  const labelEl = (
    <label htmlFor={children ? id : undefined} className={required ? 'required' : undefined}>
      {label}
    </label>
  )
  return (
    <div className={`form-group-premium ${problem ? 'invalid' : ''}`}>
      {optional ? (
        <div className="field-head">
          {labelEl}
          <span className="field-optional">Optional</span>
        </div>
      ) : labelEl}

      {children?.(id)}
      {help && <span className="help">{help}</span>}
      {problem && <span className="fail-text">{problem}</span>}
    </div>
  )
}

// Select picks one of the names of the rule's figures and rows (or none).
function Select({ id, value, names, onChange }: { id: string; value: string; names: string[]; onChange: (v: string) => void }) {
  return (
    <select id={id} value={value} onChange={(e) => onChange(e.target.value)}>
      <option value="">(none)</option>
      {names.map((n) => (
        <option key={n} value={n}>{n}</option>
      ))}
    </select>
  )
}

// ListField is a list typed with commas. The text is kept as typed; the list is read from it.
function ListField({ id, value, onChange }: { id: string; value: string[]; onChange: (v: string[]) => void }) {
  return <input id={id} type="text" defaultValue={value.join(', ')} onChange={(e) => onChange(parseList(e.target.value))} />
}

function InputRow({
  index, input, options, problem, docText, onChange, onRemove,
}: {
  index: number
  input: RuleFormInput
  options: RuleOptions
  problem: (sub: string) => string | undefined
  docText: (name: string) => string
  onChange: (patch: Partial<RuleFormInput>) => void
  onRemove: () => void
}) {
  const facts = groupBy(options.facts.map((f) => ({ value: f.name, label: f.name, doc: f.document_type })))
  const recordType = options.record_types.find((r) => r.name === input.ref)
  return (
    <fieldset className="row-box card-premium-enhanced">
      <legend>Figure or row {index + 1}</legend>
      <div className="form-section-premium">
        <Field required label={`Name in the rule ${index + 1}`} help="A letter or a word, such as a or b. The check refers to it." problem={problem('.name')}>
          {(id) => <input id={id} type="text" value={input.name} onChange={(e) => onChange({ name: e.target.value })} />}
        </Field>
        <Field required label={`Kind ${index + 1}`} help="A single figure, or the rows of a record type." problem={problem('.kind')}>
          {(id) => (
            <select id={id} value={input.kind} onChange={(e) => onChange({ kind: e.target.value as RuleFormInput['kind'], ref: '', default: '' })}>
              <option value="fact">A figure</option>
              <option value="record_type">Rows of a record type</option>
            </select>
          )}
        </Field>
        {input.kind === 'record_type' ? (
          <ChipField
            required
            label={`Which rows ${index + 1}`}
            help={recordType ? docText(recordType.document_type) : 'Press the record type whose rows the rule reads.'}
            problem={problem('.ref')}
            values={options.record_types.map((r) => r.name)}
            chosen={[input.ref]}
            onToggle={(v) => onChange({ ref: input.ref === v ? '' : v })}
          />
        ) : (
          <Field required label={`Which figure ${index + 1}`} help="Pick from the list; the list is the vocabulary." problem={problem('.ref')}>
            {(id) => (
              <select id={id} value={input.ref} onChange={(e) => onChange({ ref: e.target.value })}>
                <option value="">(choose)</option>
                {[...facts.entries()].map(([doc, list]) => (
                  <optgroup key={doc} label={docText(doc)}>
                    {list.map((o) => (
                      <option key={o.value} value={o.value}>{o.label}</option>
                    ))}
                  </optgroup>
                ))}
              </select>
            )}
          </Field>
        )}
        <Field label={`Label in finding text ${index + 1}`} help="Shown next to its value in a finding." problem={problem('.label')}>
          {(id) => <input id={id} type="text" value={input.label} onChange={(e) => onChange({ label: e.target.value })} />}
        </Field>
        {input.kind === 'fact' && (
          <label className="check">
            <input type="checkbox" aria-label={`Optional figure ${index + 1}`} checked={input.optional} onChange={(e) => onChange({ optional: e.target.checked, default: e.target.checked ? input.default : '' })} />
            Optional: the rule still runs when it is missing (only where the law makes absence meaningful)
          </label>
        )}
        {input.kind === 'fact' && input.optional && (
          <Field label={`Value when missing ${index + 1}`} help="The value used when the figure is not in the file." problem={problem('.default')}>
            {(id) => <input id={id} type="text" value={input.default} onChange={(e) => onChange({ default: e.target.value })} />}
          </Field>
        )}
      </div>
      <button type="button" className="ghost" onClick={onRemove}>
        Remove this figure or row
      </button>
    </fieldset>
  )
}

// groupBy keeps the order of the list and groups it by document type, for the optgroups.
function groupBy<T extends { doc: string }>(list: T[]): Map<string, T[]> {
  const out = new Map<string, T[]>()
  for (const x of list) {
    out.set(x.doc, [...(out.get(x.doc) ?? []), x])
  }
  return out
}

function PartsEditor({ parts, names, label, onChange }: { parts: RuleFormPart[]; names: string[]; label: string; onChange: (p: RuleFormPart[]) => void }) {
  return (
    <fieldset className="row-box card-premium-enhanced">
      <legend>{label}</legend>
      {parts.map((p, i) => (
        <div className="form-section-premium" key={i}>
          <Field label={`Figure ${i + 1} of the parts`}>
            {(id) => <Select id={id} value={p.input} names={names} onChange={(v) => onChange(parts.map((x, j) => (j === i ? { ...x, input: v } : x)))} />}
          </Field>
          <Field label={`Column ${i + 1} of the parts`} help="For rows only: the amount column; empty for figures.">
            {(id) => <input id={id} type="text" value={p.field} onChange={(e) => onChange(parts.map((x, j) => (j === i ? { ...x, field: e.target.value } : x)))} />}
          </Field>
          <Field label={`Sign ${i + 1} of the parts`} help="+ is added, - is taken away.">
            {(id) => (
              <select id={id} value={p.sign} onChange={(e) => onChange(parts.map((x, j) => (j === i ? { ...x, sign: e.target.value } : x)))}>
                <option value="+">+ added</option>
                <option value="-">- taken away</option>
              </select>
            )}
          </Field>
          <button type="button" className="ghost" onClick={() => onChange(parts.filter((_, j) => j !== i))}>Remove</button>
        </div>
      ))}
      <button type="button" onClick={() => onChange([...parts, { input: '', field: '', sign: '+' }])}>Add a figure to the parts</button>
    </fieldset>
  )
}

function ConditionsEditor({ conditions, names, onChange }: { conditions: RuleFormCondition[]; names: string[]; onChange: (c: RuleFormCondition[]) => void }) {
  const patch = (i: number, p: Partial<RuleFormCondition>) => onChange(conditions.map((x, j) => (j === i ? { ...x, ...p } : x)))
  return (
    <fieldset className="row-box card-premium-enhanced">
      <legend>Conditions that must all hold</legend>
      {conditions.map((c, i) => (
        <div className="form-section-premium" key={i}>
          <Field label={`Figure of condition ${i + 1}`}>
            {(id) => <Select id={id} value={c.input} names={names} onChange={(v) => patch(i, { input: v })} />}
          </Field>
          <Field label={`Column of condition ${i + 1}`} help="For rows only; empty for figures.">
            {(id) => <input id={id} type="text" value={c.field} onChange={(e) => patch(i, { field: e.target.value })} />}
          </Field>
          <Field label={`Test of condition ${i + 1}`}>
            {(id) => (
              <select id={id} value={c.is} onChange={(e) => patch(i, { is: e.target.value })}>
                <option value="">(choose)</option>
                {conditionNames.map((n) => (
                  <option key={n} value={n}>{n}</option>
                ))}
              </select>
            )}
          </Field>
          <Field label={`Value of condition ${i + 1}`} help="For the tests equals, matches and same_as.">
            {(id) => <input id={id} type="text" value={c.value} onChange={(e) => patch(i, { value: e.target.value })} />}
          </Field>
          <Field label={`Values of condition ${i + 1}`} help="For the tests in and not_in, separated by commas.">
            {(id) => <ListField id={id} value={c.values} onChange={(v) => patch(i, { values: v })} />}
          </Field>
          <button type="button" className="ghost" onClick={() => onChange(conditions.filter((_, j) => j !== i))}>Remove</button>
        </div>
      ))}
      <button type="button" onClick={() => onChange([...conditions, { input: '', field: '', is: '', value: '', values: [] }])}>Add a condition</button>
    </fieldset>
  )
}

function TableEditor({ rows, onChange }: { rows: RuleFormTableRow[]; onChange: (r: RuleFormTableRow[]) => void }) {
  const patch = (i: number, p: Partial<RuleFormTableRow>) => onChange(rows.map((x, j) => (j === i ? { ...x, ...p } : x)))
  return (
    <fieldset className="row-box card-premium-enhanced">
      <legend>Table of values the figure is compared with</legend>
      <p className="muted">Each row gives the values of the keys, the value expected, and where the value comes from (such as a treaty article).</p>
      {rows.map((r, i) => (
        <div className="form-section-premium" key={i}>
          <Field label={`Keys of table row ${i + 1}`} help="Separated by commas, in the order of the keys.">
            {(id) => <ListField id={id} value={r.keys ?? []} onChange={(v) => patch(i, { keys: v })} />}
          </Field>
          <Field label={`Value of table row ${i + 1}`}>
            {(id) => <input id={id} type="text" value={r.value} onChange={(e) => patch(i, { value: e.target.value })} />}
          </Field>
          <Field label={`Source of table row ${i + 1}`} help="Where the value comes from.">
            {(id) => <input id={id} type="text" value={r.source} onChange={(e) => patch(i, { source: e.target.value })} />}
          </Field>
          <button type="button" className="ghost" onClick={() => onChange(rows.filter((_, j) => j !== i))}>Remove</button>
        </div>
      ))}
      <button type="button" onClick={() => onChange([...rows, { keys: [], value: '', source: '' }])}>Add a table row</button>
    </fieldset>
  )
}

function LawRow({ index, law, problem, onChange, onRemove }: {
  index: number
  law: RuleFormLaw
  problem: (f: string) => string | undefined
  onChange: (patch: Partial<RuleFormLaw>) => void
  onRemove: () => void
}) {
  return (
    <div className="form-section-premium">
      <Field required label={`Act ${index + 1}`} help="The Act, such as Income-tax Act, 1961." problem={problem('')}>
        {(id) => <input id={id} type="text" value={law.act} onChange={(e) => onChange({ act: e.target.value })} />}
      </Field>
      <Field required label={`Section ${index + 1}`} help="The section, such as 32." problem={problem('')}>
        {(id) => <input id={id} type="text" value={law.section} onChange={(e) => onChange({ section: e.target.value })} />}
      </Field>
      <Field label={`Reference ${index + 1}`} help="Optional: a note of where it is in the rule book.">
        {(id) => <input id={id} type="text" value={law.reference} onChange={(e) => onChange({ reference: e.target.value })} />}
      </Field>
      <button type="button" className="ghost" onClick={onRemove}>Remove</button>
    </div>
  )
}

// FixtureBox is one fixture: its kind, name and description, and the figures or rows it holds,
// each with the source it comes from (invariant 1).
function FixtureBox({
  index, fixture, options, factNames, recordNames, problem, docText, onChange, onRemove,
}: {
  index: number
  fixture: RuleFormFixture
  options: RuleOptions
  factNames: string[]
  recordNames: string[]
  problem: (f: string) => string | undefined
  docText: (name: string) => string
  onChange: (patch: Partial<RuleFormFixture>) => void
  onRemove: () => void
}) {
  const facts = fixture.facts ?? []
  const records = fixture.records ?? []
  const setFact = (i: number, p: Partial<RuleFormFact>) =>
    onChange({ facts: facts.map((x, j) => (j === i ? { ...x, ...p } : x)) })
  const setRecord = (i: number, p: Partial<RuleFormRecord>) => onChange({ records: records.map((x, j) => (j === i ? { ...x, ...p } : x)) })
  const kindText = fixture.kind === 'pass' ? 'Pass' : 'Fail'
  return (
    <fieldset className="row-box card-premium-enhanced">
      <legend>{kindText} fixture {index + 1}</legend>
      <div className="form-section-premium">
        <Field required label={`Kind of fixture ${index + 1}`} help="A pass fixture must pass; a fail fixture must fail." problem={problem('.kind')}>
          {(id) => (
            <select id={id} value={fixture.kind} onChange={(e) => onChange({ kind: e.target.value as 'pass' | 'fail' })}>
              <option value="pass">Pass</option>
              <option value="fail">Fail</option>
            </select>
          )}
        </Field>
        <Field required label={`Name of fixture ${index + 1}`} help="Letters, digits and _, such as values_agree." problem={problem('.name')}>
          {(id) => <input id={id} type="text" value={fixture.name} onChange={(e) => onChange({ name: e.target.value })} />}
        </Field>
        <Field label={`Description of fixture ${index + 1}`} help="One sentence: what the fixture shows." problem={problem('.description')}>
          {(id) => <input id={id} type="text" value={fixture.description} onChange={(e) => onChange({ description: e.target.value })} />}
        </Field>
      </div>
      {facts.map((f, i) => (
        <div className="filters" key={`f${i}`}>
          <Field required label={`Figure ${i + 1} of fixture ${index + 1}`} help="One of the rule's figures." problem={problem(`.facts[${i}].fact`)}>
            {(id) => (
              <select id={id} value={f.fact} onChange={(e) => setFact(i, { fact: e.target.value })}>
                <option value="">(choose)</option>
                {factNames.map((n) => (
                  <option key={n} value={n}>{n}</option>
                ))}
              </select>
            )}
          </Field>
          <Field required label={`Value of figure ${i + 1}, fixture ${index + 1}`} help="Rupees with at most two decimals, such as 112450.50." problem={problem(`.facts[${i}].value`)}>
            {(id) => <input id={id} type="text" value={f.value} onChange={(e) => setFact(i, { value: e.target.value })} />}
          </Field>
          <Field required label={`Period of figure ${i + 1}, fixture ${index + 1}`} help="Such as FY2025-26." problem={problem(`.facts[${i}].period`)}>
            {(id) => <input id={id} type="text" value={f.period} onChange={(e) => setFact(i, { period: e.target.value })} />}
          </Field>
          <Field required label={`Source file of figure ${i + 1}, fixture ${index + 1}`} help="The file the figure is read from, for the finding text." problem={problem(`.facts[${i}].file`)}>
            {(id) => <input id={id} type="text" value={f.file} onChange={(e) => setFact(i, { file: e.target.value })} />}
          </Field>
          <Field required label={`Locator of figure ${i + 1}, fixture ${index + 1}`} help="Where the figure is in the file, such as a JSON path." problem={problem(`.facts[${i}].locator`)}>
            {(id) => <input id={id} type="text" value={f.locator} onChange={(e) => setFact(i, { locator: e.target.value })} />}
          </Field>
          <button type="button" className="ghost" onClick={() => onChange({ facts: facts.filter((_, j) => j !== i) })}>Remove figure</button>
        </div>
      ))}
      {records.map((r, i) => {
        const rt = options.record_types.find((x) => x.name === r.record_type)
        return (
          <fieldset className="row-box card-premium-enhanced" key={`r${i}`}>
            <legend>Row {i + 1} of fixture {index + 1}</legend>
            <div className="form-section-premium">
              <Field label={`Record type of row ${i + 1}, fixture ${index + 1}`} help={rt ? docText(rt.document_type) : 'The rows of one of the rule inputs.'} problem={problem(`.records[${i}].record_type`)}>
                {(id) => (
                  <select id={id} value={r.record_type} onChange={(e) => setRecord(i, { record_type: e.target.value, fields: [], amounts: [] })}>
                    <option value="">(choose)</option>
                    {recordNames.map((n) => (
                      <option key={n} value={n}>{n}</option>
                    ))}
                  </select>
                )}
              </Field>
              <Field label={`Period of row ${i + 1}, fixture ${index + 1}`} help="Such as FY2025-26 or 2025-26 Q1." problem={problem(`.records[${i}].period`)}>
                {(id) => <input id={id} type="text" value={r.period} onChange={(e) => setRecord(i, { period: e.target.value })} />}
              </Field>
              <Field label={`Source file of row ${i + 1}, fixture ${index + 1}`} problem={problem(`.records[${i}].file`)}>
                {(id) => <input id={id} type="text" value={r.file} onChange={(e) => setRecord(i, { file: e.target.value })} />}
              </Field>
              <Field label={`Locator of row ${i + 1}, fixture ${index + 1}`} help="Where the row is in the file." problem={problem(`.records[${i}].locator`)}>
                {(id) => <input id={id} type="text" value={r.locator} onChange={(e) => setRecord(i, { locator: e.target.value })} />}
              </Field>
            </div>
            <NamedRows title="Amounts of the row" names={rt?.amounts ?? []} rows={r.amounts ?? []} onChange={(amounts) => setRecord(i, { amounts })} />
            <NamedRows title="Text columns of the row" names={rt?.fields ?? []} rows={r.fields ?? []} onChange={(fields) => setRecord(i, { fields })} />
            <button type="button" className="ghost" onClick={() => onChange({ records: records.filter((_, j) => j !== i) })}>Remove row</button>
          </fieldset>
        )
      })}
      <div className="form-actions">
        <button type="button" onClick={() => onChange({ facts: [...facts, { fact: '', value: '', period: '', scope: '', file: '', locator: '' }] })}>
          Add a figure to this fixture
        </button>
        <button type="button" onClick={() => onChange({ records: [...records, { record_type: '', period: '', scope: '', file: '', locator: '', fields: [], amounts: [] }] })}>
          Add a row to this fixture
        </button>
        <button type="button" className="ghost" onClick={onRemove}>Remove this fixture</button>
      </div>
    </fieldset>
  )
}

// NamedRows is a list of named columns of a row: an amount or a text column, picked by name.
function NamedRows({ title, names, rows, onChange }: { title: string; names: string[]; rows: RuleFormNamed[]; onChange: (r: RuleFormNamed[]) => void }) {
  return (
    <fieldset className="row-box card-premium-enhanced">
      <legend>{title}</legend>
      {rows.map((r, i) => (
        <div className="form-section-premium" key={i}>
          <Field label={`Column ${i + 1} of ${title.toLowerCase()}`}>
            {(id) => (
              <select id={id} value={r.name} onChange={(e) => onChange(rows.map((x, j) => (j === i ? { ...x, name: e.target.value } : x)))}>
                <option value="">(choose)</option>
                {names.map((n) => (
                  <option key={n} value={n}>{n}</option>
                ))}
              </select>
            )}
          </Field>
          <Field label={`Value ${i + 1} of ${title.toLowerCase()}`}>
            {(id) => <input id={id} type="text" value={r.value} onChange={(e) => onChange(rows.map((x, j) => (j === i ? { ...x, value: e.target.value } : x)))} />}
          </Field>
          <button type="button" className="ghost" onClick={() => onChange(rows.filter((_, j) => j !== i))}>Remove</button>
        </div>
      ))}
      <button type="button" onClick={() => onChange([...rows, { name: '', value: '' }])}>Add a column</button>
    </fieldset>
  )
}

// SwitchRule switches a rule off or back on (SPEC-010 §4). The reason is required and is kept in
// the audit log with the signed-in user's login name; the rule's files and versions stay.
export function SwitchRule({ id, action, rulebook = false, onDone, onClose }: { id: string; action: 'disable' | 'enable'; rulebook?: boolean; onDone: () => void; onClose: () => void }) {
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const title = action === 'disable' ? `Switch off ${id}` : `Enable ${id} again`
  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (busy) return
    setBusy(true)
    setError(null)
    try {
      await switchRule(id, action, reason.trim())
      onDone()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Modal title={title} onClose={onClose} closeDisabled={busy}>
      <form className="rule-reason-form" aria-label={title} onSubmit={submit}>
        <p className="muted">
          {action === 'disable'
            ? 'A switched-off rule does not run. Its files and versions are kept; nothing is deleted.'
            : rulebook
              ? 'The rule-book rule is switched on again. It is described only, so it is still never run. Loading the rule book again keeps what you set.'
              : 'The rule comes back as testing, and runs only when a run includes testing rules.'}
        </p>
        <Field label="Reason" required help={action === 'disable' ? 'Why the rule is switched off. Required.' : 'Why the rule is switched on again. Required.'}>
          {(fid) => <textarea id={fid} rows={4} value={reason} required onChange={(e) => setReason(e.target.value)} />}
        </Field>
        {error && <p className="fail-text" role="alert">{error}</p>}
        <div className="rule-reason-actions">
          <button type="button" className="ghost" onClick={onClose} disabled={busy}>Cancel</button>
          <button type="submit" className={action === 'disable' ? 'danger' : 'primary'} disabled={busy || reason.trim() === ''}>
            {action === 'disable' ? 'Switch off' : 'Enable again'}
          </button>
        </div>
      </form>
    </Modal>
  )
}

// ApproveRule makes a testing rule active (SPEC-010 §4). The approver types a reason; the engine
// checks the gate (the fixtures pass, and a client's run included the rule) and writes one audit
// entry with the signed-in Partner's login name (SPEC-012). The server refuses anyone else.
export function ApproveRule({ id, onDone, onClose }: { id: string; onDone: () => void; onClose: () => void }) {
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const title = `Approve ${id}`
  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (busy) return
    setBusy(true)
    setError(null)
    try {
      await approveRule(id, reason.trim())
      onDone()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Modal title={title} onClose={onClose} closeDisabled={busy}>
      <form className="rule-reason-form" aria-label={title} onSubmit={submit}>
        <p className="muted">
          An active rule runs in every run, without including testing rules. The approval is written in the audit log with your login name.
        </p>
        <Field label="Reason" required help="Why the rule is approved. Required.">
          {(fid) => <textarea id={fid} rows={4} value={reason} required onChange={(e) => setReason(e.target.value)} />}
        </Field>
        {error && <p className="fail-text" role="alert">{error}</p>}
        <div className="rule-reason-actions">
          <button type="button" className="ghost" onClick={onClose} disabled={busy}>Cancel</button>
          <button type="submit" className="primary" disabled={busy || reason.trim() === ''}>
            Approve
          </button>
        </div>
      </form>
    </Modal>
  )
}
