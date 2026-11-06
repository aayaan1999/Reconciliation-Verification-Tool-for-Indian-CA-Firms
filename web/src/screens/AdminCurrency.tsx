import { useEffect, useState } from 'react'
import { ApiError, fetchCurrency, setCurrency } from '../api.ts'
import type { CurrencyState } from '../types.ts'
import { Field } from './RuleBuilder.tsx'

// The Admin tab's Currency page (SPEC-021): the fixed rate, in rupees per US dollar, that a run
// uses for an amount written in dollars. The Partner types the rate (the tool is offline and never
// fetches one), the date it takes effect and an optional note; the server validates it, writes it
// and writes the audit entry with the signed-in user. This screen shows the answer. It checks the
// same limits as the server only to explain a slip beside the field; the server decides.

// The limits the server accepts (cmd/recon-engine/currencyadmin.go).
const noteMax = 200

// A rate is digits with up to four decimal places. It is turned into whole 1/10,000 rupees (an
// integer) only to compare two rates and to show how far they differ; it is never money.
function rateUnits(text: string): number | null {
  const m = /^(\d{1,4})(?:\.(\d{1,4}))?$/.exec(text.trim())
  if (!m) return null
  return Number(m[1]) * 10000 + Number((m[2] ?? '').padEnd(4, '0'))
}

const minUnits = 1 * 10000
const maxUnits = 1000 * 10000 // exclusive

// The change from the rate in force to the rate typed, in percent with two decimals, written with
// integers only: units are 1/10,000 rupee, so the change in 1/100 percent is rounded from them.
function percentChange(from: number, to: number): string {
  const hundredths = Math.round(((to - from) * 10000) / from)
  const sign = hundredths > 0 ? '+' : hundredths < 0 ? '−' : ''
  const abs = Math.abs(hundredths)
  return `${sign}${Math.trunc(abs / 100)}.${String(abs % 100).padStart(2, '0')}%`
}

function isDate(text: string): boolean {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(text)
  if (!m) return false
  const [y, mo, d] = [Number(m[1]), Number(m[2]), Number(m[3])]
  const t = new Date(Date.UTC(y, mo - 1, d))
  return t.getUTCFullYear() === y && t.getUTCMonth() === mo - 1 && t.getUTCDate() === d
}

function today(): string {
  const n = new Date()
  const p = (v: number) => String(v).padStart(2, '0')
  return `${n.getFullYear()}-${p(n.getMonth() + 1)}-${p(n.getDate())}`
}

function when(at: string): string {
  return at.slice(0, 16).replace('T', ' ') + ' UTC'
}

function problem(err: unknown): string {
  if (err instanceof ApiError) return `${err.message} (${err.code})`
  return String(err)
}

export function CurrencyPanel({ me, onChanged }: { me: string; onChanged: () => void }) {
  const [state, setState] = useState<CurrencyState | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [saved, setSaved] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    fetchCurrency()
      .then((s) => {
        if (!cancelled) setState(s)
      })
      .catch((err: unknown) => {
        if (!cancelled) setLoadError(problem(err))
      })
    return () => {
      cancelled = true
    }
  }, [])

  if (loadError) return <p className="error" role="alert">{loadError}</p>
  if (state === null) return <p className="loading">Loading the dollar rate…</p>
  return (
    <RatePage
      // A new form each time the rate in force changes, so it never shows a stale draft.
      key={`${state.usd_rate}|${state.effective_from}|${state.history.length}`}
      state={state}
      me={me}
      saved={saved}
      onSaved={(s) => {
        setSaved(`The rate is ₹${s.usd_rate} for each US dollar, effective ${s.effective_from}. It applies from the next run.`)
        setState(s)
        onChanged()
      }}
    />
  )
}

function RatePage({ state, me, saved, onSaved }: { state: CurrencyState; me: string; saved: string | null; onSaved: (s: CurrencyState) => void }) {
  const [rate, setRate] = useState(state.usd_rate)
  const [date, setDate] = useState(today())
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [fault, setFault] = useState<string | null>(null)

  const units = rateUnits(rate)
  const rateOk = units !== null && units >= minUnits && units < maxUnits
  const current = state.set ? rateUnits(state.usd_rate) : null
  const dateOk = isDate(date)
  const noteOk = [...note].length <= noteMax && !/[\r\n\t]/.test(note)
  const unchanged = state.set && units === current && date === state.effective_from && note.trim() === state.note
  const canSave = rateOk && dateOk && noteOk && !unchanged && !busy

  const rateProblem =
    rate.trim() === '' ? undefined
    : units === null ? 'Write the rate with digits and up to four decimal places, such as 83.2500.'
    : !rateOk ? 'A rate is at least 1 and below 1,000 rupees for one dollar. Check the figure.'
    : undefined

  return (
    <div className="admin-security">
      <section className="admin-card" aria-labelledby="rate-now-title">
        <h2 id="rate-now-title">Rate in force</h2>
        {state.set ? (
          <p className="admin-readback" aria-live="polite">
            <strong>₹{state.usd_rate}</strong> for each US dollar, effective {state.effective_from || 'no date written'}.
            {state.note && <span className="muted"> {state.note}</span>}
          </p>
        ) : (
          <p className="admin-readback" aria-live="polite">
            <strong>No rate set.</strong> An amount written in dollars ($1,250.50 or USD 1,250.50) is not read until a rate is set; the checks that need it say Cannot check, and nothing is guessed. Amounts in rupees are not affected.
            {state.problem === 'dollar rate not valid' && <span className="fail-text"> The rate file is not valid: {state.detail}.</span>}
          </p>
        )}
        <p className="muted">
          One rate is used for every document in a run. A change applies to the next run: a file that holds dollar amounts is read again at the new rate as a new draft, and earlier runs keep the rate they used.
          The tool is offline and never looks up a rate; type the one you use.
        </p>
      </section>

      <form
        className="admin-card admin-form"
        aria-label="Set the dollar rate"
        onSubmit={async (e) => {
          e.preventDefault()
          if (!canSave) return
          setBusy(true)
          setFault(null)
          try {
            const next = await setCurrency({ usd_rate: rate.trim(), effective_from: date, note: note.trim() })
            onSaved(next)
          } catch (err) {
            setFault(problem(err))
          } finally {
            setBusy(false)
          }
        }}
      >
        <h2>{state.set ? 'Change the rate' : 'Set the rate'}</h2>
        <div className="admin-grid">
          <Field label="Rupees for one US dollar" required help="For example 83.2500. Up to four decimal places." problem={rateProblem}>
            {(id) => (
              <input id={id} type="text" inputMode="decimal" autoComplete="off" required aria-invalid={rateProblem !== undefined}
                value={rate} onChange={(e) => setRate(e.target.value)} />
            )}
          </Field>
          <Field label="Effective from" required help="The date this rate was fixed. For your record; it is not compared with the date of a document."
            problem={date !== '' && !dateOk ? 'Enter a real date.' : undefined}>
            {(id) => (
              <input id={id} type="date" required aria-invalid={date !== '' && !dateOk} value={date} onChange={(e) => setDate(e.target.value)} />
            )}
          </Field>
        </div>
        <Field label="Note" optional help={`Where the rate came from, for example the bank's card rate. Up to ${noteMax} characters.`}
          problem={!noteOk ? `One line of up to ${noteMax} characters.` : undefined}>
          {(id) => <input id={id} type="text" autoComplete="off" maxLength={noteMax + 20} aria-invalid={!noteOk} value={note} onChange={(e) => setNote(e.target.value)} />}
        </Field>
        <p className="admin-readback" aria-live="polite">
          {rateOk && current !== null && units !== current && (
            <>Change from the rate in force: <strong>{percentChange(current, units)}</strong>. </>
          )}
          {rateOk && units !== null && current === null && <>The first rate will be ₹{rate.trim()} for each US dollar. </>}
          {canSave && <span className="muted">Not saved yet. Saving is recorded in the audit log under {me}.</span>}
          {!canSave && !unchanged && !busy && <span className="muted">Enter a rate and a date to save.</span>}
          {unchanged && <span className="muted">This is the rate in force.</span>}
        </p>
        {fault && <p className="error" role="alert">{fault}</p>}
        {saved && <p className="notice" role="status">{saved}</p>}
        <div className="admin-actions">
          <button type="submit" className="action" disabled={!canSave}>{busy ? 'Saving…' : 'Save rate'}</button>
        </div>
      </form>

      <section className="admin-card" aria-labelledby="rate-history-title">
        <h2 id="rate-history-title">Earlier rates</h2>
        {state.history.length === 0 ? (
          <p className="muted">No rate has been set on this page yet.</p>
        ) : (
          <div className="table-wrap">
            <table className="table-premium" aria-label="Rate history">
              <thead>
                <tr>
                  <th scope="col">When</th>
                  <th scope="col">By</th>
                  <th scope="col">Rate (₹ per US dollar)</th>
                  <th scope="col">Replaced</th>
                  <th scope="col">Effective from</th>
                  <th scope="col">Note</th>
                </tr>
              </thead>
              <tbody>
                {state.history.map((h) => (
                  <tr key={`${h.at}|${h.usd_rate}|${h.effective_from}`}>
                    <td>{when(h.at)}</td>
                    <td>{h.by}</td>
                    <td>{h.usd_rate}</td>
                    <td>{h.previous || 'No rate before'}</td>
                    <td>{h.effective_from}</td>
                    <td>{h.note}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  )
}
