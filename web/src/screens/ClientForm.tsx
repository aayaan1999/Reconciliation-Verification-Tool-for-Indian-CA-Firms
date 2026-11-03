import { useState, type FormEvent, type ReactNode } from 'react'
import { ApiError } from '../api.ts'
import { constitutions, registrationTypes, states } from '../clientLists.ts'
import { allTaxYears } from '../filters.ts'
import type { ClientDetails, ClientProblem } from '../types.ts'
import { Field } from './RuleBuilder.tsx'
import { Modal } from './Modal.tsx'

interface Props {
  mode: 'add' | 'edit'
  initial: ClientDetails
  taxYears: string[] // add only: the tax year to open
  onSave: (details: ClientDetails, taxYear: string) => Promise<void>
  onCancel: () => void
}

// The shapes of the identity numbers, checked as they are typed so a slip shows at once. They
// are a help only: the server checks every value again (check characters included) and its
// answer is the one that counts, so a value these do not like can still be saved.
const formats: Record<string, { re: RegExp; max: number; message: string }> = {
  pan: { re: /^[A-Z]{5}[0-9]{4}[A-Z]$/, max: 10, message: 'A PAN is five letters, four digits and one letter, for example ABCDE1234F.' },
  tan: { re: /^[A-Z]{4}[0-9]{5}[A-Z]$/, max: 10, message: 'A TAN is four letters, five digits and one letter.' },
  gstin: { re: /^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][0-9A-Z]Z[0-9A-Z]$/, max: 15, message: 'A GSTIN is 15 characters: state code, PAN, entity number, Z and a check character.' },
  pin: { re: /^[1-9][0-9]{5}$/, max: 6, message: 'A PIN code is six digits and does not start with 0.' },
}
const stateName = (code: string) => states.find(([v]) => v === code)?.[1]

// ClientForm adds or edits a client with all its details: identity numbers, GST
// registrations, address and contact. It opens as a popup in the middle of the screen, over
// the board or the client file, and closes with Cancel or its close button. The
// server checks every value and reports the field and the reason of each problem.
// Both modes share one layout: the same sections in the same order. Add and edit differ only
// in the PAN (required when adding, optional when editing) and the tax year (add only).
export function ClientForm({ mode, initial, taxYears, onSave, onCancel }: Props) {
  const [d, setD] = useState<ClientDetails>(() => ({ ...initial, pan: '', gstins: initial.gstins.map((g) => ({ ...g })) }))
  const [taxYear, setTaxYear] = useState(taxYears[0] ?? '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const [touched, setTouched] = useState<Set<string>>(() => new Set())
  const [focusGstin, setFocusGstin] = useState<number | null>(null)
  const problems: ClientProblem[] = error?.problems ?? []
  const problemFor = (field: string) => problems.find((p) => p.field === field || p.field.startsWith(field + '.'))
  const set = <K extends keyof ClientDetails>(k: K, v: ClientDetails[K]) => setD({ ...d, [k]: v })
  const setAddr = (k: keyof ClientDetails['address'], v: string) => setD({ ...d, address: { ...d.address, [k]: v } })
  const setGstin = (i: number, patch: Partial<ClientDetails['gstins'][number]>) => {
    const gstins = d.gstins.map((g, j) => (j === i ? { ...g, ...patch } : patch.primary ? { ...g, primary: false } : g))
    // The state is read from the primary GSTIN's first two digits while the address has none.
    const primary = gstins.find((g) => g.primary) ?? gstins[0]
    const code = primary?.gstin.slice(0, 2) ?? ''
    const state = d.address.state === '' && stateName(code) ? code : d.address.state
    setD({ ...d, gstins, address: { ...d.address, state } })
  }
  const addGstin = () => {
    setD({ ...d, gstins: [...d.gstins, { gstin: '', registration_type: 'regular', primary: d.gstins.length === 0 }] })
    setFocusGstin(d.gstins.length)
  }

  // A format message shows once the field is full or has been left, never while the first
  // characters are typed.
  const formatProblem = (kind: keyof typeof formats, name: string, value: string) => {
    const f = formats[kind]
    if (value === '' || f.re.test(value)) return undefined
    return value.length >= f.max || touched.has(name) ? f.message : undefined
  }
  const leave = (name: string) => () => setTouched((t) => (t.has(name) ? t : new Set(t).add(name)))

  // The GSTIN's characters 3 to 12 are the PAN. When editing, only the PAN's last four are known.
  const panWarning = (gstin: string): string | undefined => {
    if (!formats.gstin.re.test(gstin)) return undefined
    const pan = (d.pan ?? '').toUpperCase()
    if (formats.pan.re.test(pan)) return gstin.slice(2, 12) === pan ? undefined : `This GSTIN does not contain the PAN ${pan}.`
    if (pan === '' && initial.pan_last4) {
      return gstin.slice(8, 12) === initial.pan_last4 ? undefined : `This GSTIN does not contain the PAN ending ${initial.pan_last4}.`
    }
    return undefined
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await onSave({ ...d, pan: (d.pan ?? '').toUpperCase() }, taxYear)
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError('ERROR', String(err)))
    } finally {
      setBusy(false)
    }
  }
  const saveDisabled = busy || d.legal_name.trim() === '' || (mode === 'add' && (d.pan ?? '').length !== 10)

  // field is one labelled control (the shared Field from RuleBuilder: the label sits above the
  // control, a required label gets its star from CSS, an optional one says so, and a problem,
  // the server's first, shows under it). The aria-label keeps each control's accessible name.
  const field = (name: string, label: string, input: (id: string) => ReactNode,
    opts: { required?: boolean; wide?: boolean; local?: string; hint?: string } = {}) => {
    const f = (
      <Field label={label} required={opts.required} optional={!opts.required} help={opts.hint}
        problem={problemFor(name)?.message ?? opts.local}>
        {input}
      </Field>
    )
    return opts.wide ? <div className="field-wide">{f}</div> : f
  }
  const title = mode === 'add' ? 'Add client' : 'Edit client'
  return (
    <Modal title={title} onClose={onCancel}>
      <form className="client-form" aria-label={title} onSubmit={submit}>
        <fieldset className="client-section">
          <legend>Identity</legend>
          <div className="client-grid">
            {field('legal_name', 'Legal name', (id) => (
              <input id={id} type="text" aria-label="Legal name" value={d.legal_name} maxLength={200} required
                onChange={(e) => set('legal_name', e.target.value)} />
            ), { required: true, wide: true })}
            {field('trade_name', 'Trade name', (id) => (
              <input id={id} type="text" aria-label="Trade name" value={d.trade_name ?? ''} maxLength={200}
                onChange={(e) => set('trade_name', e.target.value)} />
            ))}
            {field('pan', mode === 'add' ? 'PAN' : `PAN (leave empty to keep the one ending ${initial.pan_last4})`, (id) => (
              <input id={id} type="text" aria-label="PAN" className="mono" value={d.pan ?? ''} maxLength={10} required={mode === 'add'} autoComplete="off"
                spellCheck={false} placeholder="ABCDE1234F" onBlur={leave('pan')} onChange={(e) => set('pan', e.target.value.toUpperCase())} />
            ), { required: mode === 'add', local: formatProblem('pan', 'pan', d.pan ?? ''),
              hint: 'Only the last four characters are shown after saving.' })}
            {field('constitution', 'Constitution', (id) => (
              <select id={id} aria-label="Constitution" value={d.constitution} onChange={(e) => set('constitution', e.target.value)}>
                <option value="">Not given</option>
                {constitutions.map(([v, t]) => (
                  <option key={v} value={v}>{t}</option>
                ))}
              </select>
            ))}
            {field('date_of_incorporation', 'Date of incorporation', (id) => (
              <input id={id} type="date" aria-label="Date of incorporation" value={d.date_of_incorporation}
                onChange={(e) => set('date_of_incorporation', e.target.value)} />
            ))}
            {field('cin', 'CIN or LLPIN', (id) => (
              <input id={id} type="text" aria-label="CIN or LLPIN" className="mono" value={d.cin ?? ''} maxLength={21} spellCheck={false}
                onChange={(e) => set('cin', e.target.value)} />
            ))}
            {field('tan', 'TAN', (id) => (
              <input id={id} type="text" aria-label="TAN" className="mono" value={d.tan ?? ''} maxLength={10} spellCheck={false}
                onBlur={leave('tan')} onChange={(e) => set('tan', e.target.value.toUpperCase())} />
            ), { local: formatProblem('tan', 'tan', d.tan ?? '') })}
            {mode === 'add' &&
              field('tax_year', 'Tax year', (id) => (
                <select id={id} aria-label="Tax year of the client" value={taxYear} onChange={(e) => setTaxYear(e.target.value)}>
                  {/* The rules' years and every year from 1999-00 to 2046-47; the first rule year stays the default. */}
                  {[...new Set([...taxYears, ...allTaxYears()])].sort().map((y) => (
                    <option key={y}>{y}</option>
                  ))}
                </select>
              ), { required: true })}
          </div>
        </fieldset>

        <fieldset className="client-section">
          <legend>GST registrations</legend>
          {d.gstins.length === 0 && <p className="muted">No GSTIN yet. Add one with + Add GSTIN.</p>}
          {d.gstins.map((g, i) => {
            const state = g.gstin.length >= 2 ? stateName(g.gstin.slice(0, 2)) : undefined
            const warn = panWarning(g.gstin)
            return (
              <div className="gstin-block" key={i}>
                <div className="client-grid">
                  <Field label={`GSTIN ${i + 1}`} problem={problemFor(`gstins[${i}]`)?.message ?? formatProblem('gstin', `gstins[${i}]`, g.gstin)}>
                    {(id) => (
                      <input id={id} type="text" aria-label={`GSTIN ${i + 1}`} className="mono" value={g.gstin} maxLength={15} spellCheck={false}
                        autoFocus={focusGstin === i} onBlur={leave(`gstins[${i}]`)}
                        onChange={(e) => setGstin(i, { gstin: e.target.value.toUpperCase() })} />
                    )}
                  </Field>
                  <Field label="Registration type">
                    {(id) => (
                      <select id={id} aria-label={`Registration type ${i + 1}`} value={g.registration_type}
                        onChange={(e) => setGstin(i, { registration_type: e.target.value })}>
                        {registrationTypes.map(([v, t]) => (
                          <option key={v} value={v}>{t}</option>
                        ))}
                      </select>
                    )}
                  </Field>
                </div>
                {(state || warn) && (
                  <p className="gstin-info">
                    {state && <span className="chip">State {g.gstin.slice(0, 2)}: {state}</span>}
                    {warn && <span className="warn-text" role="status">{warn}</span>}
                  </p>
                )}
                <div className="gstin-foot">
                  <label className="check-option">
                    <input type="radio" name="primary-gstin" aria-label={`Primary GSTIN ${i + 1}`} checked={g.primary}
                      onChange={() => setGstin(i, { primary: true })} />
                    Primary
                  </label>
                  <button type="button" className="ghost" onClick={() => setD({ ...d, gstins: d.gstins.filter((_, j) => j !== i) })}>
                    Remove
                  </button>
                </div>
              </div>
            )
          })}
          {problemFor('gstins') && !problemFor('gstins[') && <p className="fail-text">{problemFor('gstins')?.message}</p>}
          <div className="gstin-add">
            <button type="button" onClick={addGstin}>
              + Add GSTIN
            </button>
          </div>
        </fieldset>

        <fieldset className="client-section">
          <legend>Address, contact and software</legend>
          <div className="client-grid">
            {field('address.line1', 'Address line 1', (id) => (
              <input id={id} type="text" aria-label="Address line 1" value={d.address.line1} onChange={(e) => setAddr('line1', e.target.value)} />
            ), { wide: true })}
            {field('address.line2', 'Address line 2', (id) => (
              <input id={id} type="text" aria-label="Address line 2" value={d.address.line2} onChange={(e) => setAddr('line2', e.target.value)} />
            ), { wide: true })}
            {field('address.city', 'City', (id) => (
              <input id={id} type="text" aria-label="City" value={d.address.city} onChange={(e) => setAddr('city', e.target.value)} />
            ))}
            {field('address.state', 'State', (id) => (
              <select id={id} aria-label="State" value={d.address.state} onChange={(e) => setAddr('state', e.target.value)}>
                <option value="">Not given</option>
                {states.map(([v, t]) => (
                  <option key={v} value={v}>{`${v} ${t}`}</option>
                ))}
              </select>
            ))}
            {field('address.pin', 'PIN code', (id) => (
              <input id={id} type="text" aria-label="PIN code" inputMode="numeric" maxLength={6} value={d.address.pin}
                onBlur={leave('address.pin')} onChange={(e) => setAddr('pin', e.target.value)} />
            ), { local: formatProblem('pin', 'address.pin', d.address.pin) })}
            {field('contact_person', 'Contact person', (id) => (
              <input id={id} type="text" aria-label="Contact person" value={d.contact_person ?? ''} onChange={(e) => set('contact_person', e.target.value)} />
            ))}
            {field('email', 'Email', (id) => (
              <input id={id} type="email" aria-label="Email" value={d.email ?? ''} onChange={(e) => set('email', e.target.value)} />
            ))}
            {field('phone', 'Phone', (id) => (
              <input id={id} type="tel" aria-label="Phone" value={d.phone ?? ''} onChange={(e) => set('phone', e.target.value)} />
            ))}
            {field('accounting_software', 'Accounting software', (id) => (
              <input id={id} type="text" aria-label="Accounting software" value={d.accounting_software ?? ''} onChange={(e) => set('accounting_software', e.target.value)} />
            ))}
            {field('notes', 'Notes', (id) => (
              <textarea id={id} aria-label="Notes" value={d.notes} maxLength={2000} rows={3} onChange={(e) => set('notes', e.target.value)} />
            ), { wide: true })}
          </div>
        </fieldset>

        {error && (
          <div className="error" role="alert">
            <strong>{error.code}</strong>: {error.message}
            {error.problems.length > 1 && (
              <ul>
                {error.problems.map((p) => (
                  <li key={p.field + p.code}>{p.field}: {p.message}</li>
                ))}
              </ul>
            )}
          </div>
        )}

        {/* One footer row kept in view while the form scrolls: Cancel at the left, Save at the right. */}
        <div className="form-actions client-form-actions">
          <button type="button" className="secondary-button" onClick={onCancel}>Cancel</button>
          <button type="submit" className="premium-golden" disabled={saveDisabled}>
            {busy ? 'Saving…' : mode === 'add' ? 'Save client' : 'Save changes'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
