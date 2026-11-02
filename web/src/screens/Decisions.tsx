import { useEffect, useId, useRef, useState, type RefObject } from 'react'
import { ApiError } from '../api.ts'
import { decisionKeys, decisionMeaning, decisionText, stateText, type DecideFn } from '../decisions.ts'
import type { DecisionKind, Finding, FindingDecision } from '../types.ts'

// Decisions on findings (SPEC-006 part 3): Fix, Not an issue, Ask client. A decision never
// changes a finding: the figures, the difference and the sources stay as the tool found them.

function when(at: string): string {
  return at.slice(0, 16).replace('T', ' ') + ' UTC'
}

// A problem with a decision, in words: what to do, where, and (from the server) the code.
interface Problem {
  text: string
  code?: string
  field?: 'note'
}

const kinds: DecisionKind[] = ['fix', 'not_an_issue', 'ask_client']

// The tone of a state's tag: open is a finding still to act on; Ask client waits on the client.
const stateTone: Record<string, string> = { open: 'fail', fix: 'fail', not_an_issue: 'pass', ask_client: 'note' }

// actRef, when given, is set to the decide function so the finding card can apply a
// decision from the keyboard (f / x / a) with the same checks as the cards. autoNext, when
// given, is the switch that moves the screen to the next open finding after a decision.
export function DecisionPanel({ f, d, onDecide, actRef, autoNext }: {
  f: Finding
  d?: FindingDecision
  onDecide: DecideFn
  actRef?: RefObject<((k: DecisionKind) => void) | null>
  autoNext?: { on: boolean; set: (on: boolean) => void }
}) {
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<Problem | null>(null)
  const noteRef = useRef<HTMLTextAreaElement>(null)
  const ids = useId()
  const state = d?.state ?? 'open'
  const name = [f.rule_id, f.subject.match_key].filter(Boolean).join(' ')

  const decide = async (kind: DecisionKind) => {
    setError(null)
    if (kind === 'not_an_issue' && note.trim() === '') {
      setError({ text: 'Write a note saying why this is not an issue, then choose Not an issue again.', field: 'note' })
      return
    }
    setBusy(true)
    try {
      await onDecide(f.finding_id, kind, note)
      setNote('')
    } catch (err) {
      const e = err instanceof ApiError ? err : new ApiError('ERROR', String(err))
      setError({ text: e.message.charAt(0).toUpperCase() + e.message.slice(1), code: e.code })
    } finally {
      setBusy(false)
    }
  }

  useEffect(() => {
    if (actRef) actRef.current = busy ? null : decide
  })

  // The history comes oldest first; the newest is shown on top.
  const history = d ? [...d.history].reverse() : []
  return (
    <div className="cf-decision" role="group" aria-label={`Decision on ${name}`}>
      <div className="cf-decision-head">
        <h4>
          Decision <span className={`tag ${stateTone[state] ?? 'muted'}`}>{stateText[state]}</span>
        </h4>
        {autoNext && (
          <label className="cf-check">
            <input type="checkbox" checked={autoNext.on} onChange={(e) => autoNext.set(e.target.checked)} />
            Go to the next open finding after deciding
          </label>
        )}
      </div>
      {d?.current.on_other_values && (
        <p className="notice" role="note">
          The figures changed after this decision, so the finding is open again.
        </p>
      )}
      <textarea ref={noteRef} className="cf-note" aria-label={`Note on ${name}`} value={note} maxLength={2000} rows={2}
        placeholder="Note for the file (needed for Not an issue): for example, which document will be corrected and by whom"
        onChange={(e) => setNote(e.target.value)} />
      <div className="cf-decide-cards">
        {kinds.map((k) => (
          <button key={k} type="button" className={`cf-decide cf-${k}`} disabled={busy} aria-pressed={d?.state === k}
            aria-label={decisionText[k]} aria-describedby={`${ids}-${k}`} aria-keyshortcuts={decisionKeys[k]}
            title={`${decisionText[k]} (key ${decisionKeys[k]})`} onClick={() => decide(k)}>
            <span className="cf-decide-name">{decisionText[k]}</span>
            <span className="cf-decide-meaning" id={`${ids}-${k}`}>{decisionMeaning[k]}</span>
            <kbd aria-hidden="true">{decisionKeys[k].toUpperCase()}</kbd>
          </button>
        ))}
      </div>
      {error && (
        <p className="error" role="alert">
          {error.text}
          {error.code && <span className="muted"> ({error.code})</span>}
          {error.field && (
            <>
              {' '}
              <button type="button" className="ghost" onClick={() => noteRef.current?.focus()}>
                Go to the note
              </button>
            </>
          )}
        </p>
      )}
      {history.length > 0 && (
        <>
          <h5 className="cf-history-title">
            History ({history.length} {history.length === 1 ? 'decision' : 'decisions'})
          </h5>
          <ol className="cf-decision-history">
            {history.map((h, i) => (
              <li key={history.length - i}>
                <span>
                  <b>{decisionText[h.decision]}</b> by {h.decided_by} · {when(h.decided_at)}
                  {i === 0 && !h.on_other_values && <span className="tag muted">Current</span>}
                  {h.on_other_values ? ' (on figures that have changed since)' : ''}
                </span>
                {h.note && <q>{h.note}</q>}
              </li>
            ))}
          </ol>
        </>
      )}
      <p className="cf-decision-rule muted">
        Decisions are added on top and never overwritten. They follow the finding into later runs.
      </p>
    </div>
  )
}
