import { useState, type ReactNode } from 'react'
import { ApiError, saveRulebookRule } from '../api.ts'
import { checkTypeText } from '../ruleForm.ts'
import type { RuleCurrent, RuleForm, RulebookPair, RulebookRule } from '../types.ts'

// The whole rule in the Rules tab's summary (SPEC-010 scope 8): every section its file holds,
// in plain words, read from the version in use. An engine rule shows its form (what it reads,
// its check, tolerance, severity, legal basis and fixtures); a rule-book rule shows its purpose,
// governance, validity, legal basis, inputs, matching, calculation, parameters, outcomes, tests,
// pseudocode, sources and notes. Nothing here is a finding: it is the rule as written.

// Section is one titled block of the summary. It is left out when it has nothing to show.
function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="rule-detail-section" aria-label={title}>
      <h4>{title}</h4>
      {children}
    </section>
  )
}

// Facts is a two-column list of named values; a row with an empty value is left out.
function Facts({ rows }: { rows: [string, ReactNode][] }) {
  const shown = rows.filter(([, v]) => v !== '' && v !== null && v !== undefined)
  if (shown.length === 0) return <p className="muted">Nothing written.</p>
  return (
    <dl className="rule-facts">
      {shown.map(([k, v]) => (
        <Pair key={k} k={k} v={v} />
      ))}
    </dl>
  )
}

function Pair({ k, v }: { k: string; v: ReactNode }) {
  return (
    <>
      <dt>{k}</dt>
      <dd className="pre-line">{v}</dd>
    </>
  )
}

const pairText = (p: RulebookPair) => (p.expression ? p.expression : `${p.left ?? ''} ↔ ${p.right ?? ''}`)

export function RuleDetail({ current }: { current: RuleCurrent }) {
  return (
    <div className="rule-detail">
      {current.status === 'disabled' && current.disabled_reason && (
        <p className="rule-readiness muted" role="status">Switched off because: {current.disabled_reason}</p>
      )}
      {current.rulebook ? <RulebookDetail r={current.rulebook} /> : current.form ? <EngineDetail f={current.form} current={current} /> : (
        <p className="muted">{current.reason || 'The rule\'s details cannot be shown.'}</p>
      )}
    </div>
  )
}

function EngineDetail({ f, current }: { f: RuleForm; current: RuleCurrent }) {
  const c = f.check
  const list = (xs: string[]) => xs.filter(Boolean).join(', ')
  return (
    <>
      <Section title="What the rule says">
        <Facts rows={[['Rule text', f.rule_text], ['Notes', f.notes], ['Module', f.module], ['Review document', f.review_document]]} />
      </Section>
      <Section title="What it reads">
        {f.inputs.length === 0 ? <p className="muted">No figures named.</p> : (
          <table className="grid table-premium rule-detail-table" aria-label="Figures it reads">
            <thead><tr><th>Name</th><th>Kind</th><th>Figure or rows</th><th>Label</th></tr></thead>
            <tbody>
              {f.inputs.map((i) => (
                <tr key={i.name}>
                  <td className="mono">{i.name}</td>
                  <td>{i.kind === 'fact' ? 'A figure' : 'Rows'}{i.optional ? ' (optional)' : ''}</td>
                  <td className="mono">{i.ref}</td>
                  <td>{i.label}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Section>
      <Section title="Check">
        <Facts rows={[
          ['Check type', `${c.type}${checkTypeText[c.type] ? ` — ${checkTypeText[c.type]}` : ''}`],
          ['Tolerance', c.tolerance ? `₹${c.tolerance}` : 'exact'],
          ['Parts', c.parts.map((p) => `${p.sign} ${p.input}${p.field ? `.${p.field}` : ''}`).join(' ')],
          ['Total', [c.total, c.total_field].filter(Boolean).join('.')],
          ['Base', c.base],
          ['Rate', c.rate ? `${c.rate}%` : ''],
          ['Figure compared', c.value],
          ['Keys', list(c.keys)],
          ['Amounts compared', list(c.compare_amounts)],
          ['Fields compared', list(c.compare_fields)],
          ['Conditions', c.when.map((w) => `${w.input}${w.field ? `.${w.field}` : ''} ${w.is} ${w.value || w.values.join(', ')}`.trim()).join('; ')],
          ['Table rows', c.table.length > 0 ? String(c.table.length) : ''],
          ['Severity', f.severity],
        ]} />
      </Section>
      <Section title="Where it applies">
        <Facts rows={[['Tax years', f.tax_years.join(', ')], ['Valid', `${f.valid_from || '…'} to ${f.valid_to || 'no end'}`], ['Category', f.category]]} />
      </Section>
      <Section title="Legal basis">
        {f.law.length === 0 ? <p className="muted">None written.</p> : (
          <ul className="rule-detail-list">
            {f.law.map((l, i) => <li key={i}>{[l.act, l.section && `section ${l.section}`, l.reference].filter(Boolean).join(', ')}</li>)}
          </ul>
        )}
      </Section>
      <Section title="Examples it is tested on">
        {current.fixtures.length === 0 ? <p className="muted">None kept.</p> : (
          <ul className="rule-detail-list">
            {current.fixtures.map((x) => (
              <li key={`${x.kind}/${x.name}`}>
                <span className={`tag ${x.kind === 'pass' ? 'pass' : 'note'}`}>{x.kind === 'pass' ? 'Pass' : 'Fail'}</span> <span className="mono">{x.name}</span>
                {x.description && <> — {x.description}</>}
              </li>
            ))}
          </ul>
        )}
      </Section>
    </>
  )
}

function RulebookDetail({ r }: { r: RulebookRule }) {
  const g = r.governance
  return (
    <>
      <Section title="Purpose">
        <p className="pre-line">{r.rule.purpose || '—'}</p>
      </Section>
      <Section title="Governance">
        <Facts rows={[
          ['Verification', g.verification_label || g.verification_status],
          ['Reviewed by', g.reviewed_by], ['Reviewed on', g.reviewed_on], ['Review decision', g.review_decision],
          ['Last changed by', g.last_modified_by], ['Last changed on', g.last_modified_on], ['Change note', g.change_note],
        ]} />
      </Section>
      <Section title="Validity">
        <Facts rows={[['Valid', `${r.validity.valid_from || '…'} to ${r.validity.valid_to || 'no end'}`], ['Effective', r.validity.effective_text], ['Period basis', r.validity.period_basis]]} />
      </Section>
      <Section title="Legal basis">
        <Facts rows={[
          ['Provisions', r.legal.provisions], ['Applicability', r.legal.applicability], ['Exemptions and exceptions', r.legal.exemptions_exceptions],
          ['Thresholds and rates', r.legal.thresholds_rates], ['Rounding', r.legal.rounding], ['Due dates', r.legal.due_dates],
        ]} />
      </Section>
      <Section title="Inputs">
        {r.inputs.documents.length > 0 && (
          <table className="grid table-premium rule-detail-table" aria-label="Documents it reads">
            <thead><tr><th>Role</th><th>Document</th><th>Required</th></tr></thead>
            <tbody>
              {r.inputs.documents.map((d, i) => (
                <tr key={i}><td>{d.role}</td><td className="mono">{d.doc_code}</td><td>{d.required ? 'Yes' : 'No'}</td></tr>
              ))}
            </tbody>
          </table>
        )}
        <Facts rows={[['Fields', r.inputs.fields.join(', ')], ['Prerequisites', r.inputs.prerequisites], ['Input checks', r.inputs.input_validations]]} />
      </Section>
      <Section title="Matching">
        <Facts rows={[
          ['Keys', r.matching.keys.map(pairText).join('\n')], ['Key normalisation', r.matching.key_normalisation],
          ['Aggregation', r.matching.aggregation], ['Comparisons', r.matching.comparisons.map(pairText).join('\n')], ['Tolerance', r.matching.tolerance],
        ]} />
      </Section>
      <Section title="Calculation">
        <Facts rows={[['Formula', r.calculation.formula]]} />
        {r.calculation.steps.length > 0 && (
          <ol className="rule-detail-list">{r.calculation.steps.map((s, i) => <li key={i}>{s}</li>)}</ol>
        )}
      </Section>
      {r.parameters.length > 0 && (
        <Section title="Parameters">
          <table className="grid table-premium rule-detail-table" aria-label="Parameters">
            <thead><tr><th>Parameter</th><th>Value</th><th>Type</th><th>Can be changed</th></tr></thead>
            <tbody>
              {r.parameters.map((p) => (
                <tr key={p.key}>
                  <td>{p.label || p.key}</td><td className="mono">{p.value}{p.unit ? ` ${p.unit}` : ''}</td><td>{p.type}</td><td>{p.editable ? 'Yes' : 'No'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Section>
      )}
      <Section title="Outcomes">
        {r.outcomes.categories.length > 0 && (
          <table className="grid table-premium rule-detail-table" aria-label="Outcome categories">
            <thead><tr><th>Code</th><th>Meaning</th><th>Severity</th></tr></thead>
            <tbody>
              {r.outcomes.categories.map((o) => (
                <tr key={o.code}><td className="mono">{o.code}</td><td>{o.description}{o.action ? ` (${o.action})` : ''}</td><td>{o.severity}</td></tr>
              ))}
            </tbody>
          </table>
        )}
        <Facts rows={[
          ['Default severity', r.outcomes.default_severity], ['Expected output', r.outcomes.expected_output],
          ['Recommended action', r.outcomes.recommended_action], ['Common discrepancies', r.outcomes.common_discrepancies],
        ]} />
      </Section>
      <Section title="Tests">
        <Facts rows={[['Worked example', r.tests.worked_example], ['Positive', r.tests.positive], ['Negative', r.tests.negative]]} />
      </Section>
      {r.implementation.pseudocode && (
        <Section title="Pseudocode">
          <pre className="rule-detail-code">{r.implementation.pseudocode}</pre>
        </Section>
      )}
      {r.sources.length > 0 && (
        <Section title="Sources">
          {/* Shown as text, never as links: the tool works offline (invariant 9). */}
          <ul className="rule-detail-list">
            {r.sources.map((s, i) => (
              <li key={i}>
                {s.title}{s.official ? ' (official)' : ''}{s.ref ? ` — ${s.ref}` : ''}
                {s.url && <div className="mono muted rule-detail-url">{s.url}</div>}
              </li>
            ))}
          </ul>
        </Section>
      )}
      {r.notes && (
        <Section title="Notes">
          <p className="pre-line">{r.notes}</p>
        </Section>
      )}
    </>
  )
}

// LinesField edits a list as one entry per line. The text is kept as typed, so a new empty
// line can be started; the list it reports leaves empty lines out.
function LinesField({ value, name, onChange }: { value: string[]; name: string; onChange: (v: string[]) => void }) {
  const [raw, setRaw] = useState(value.join('\n'))
  return (
    <label className="rbe-field">
      {name} <span className="muted">(one per line)</span>
      <textarea rows={Math.max(3, value.length + 1)} value={raw}
        onChange={(e) => {
          setRaw(e.target.value)
          onChange(e.target.value.split('\n').map((s) => s.trim()).filter((s) => s !== ''))
        }} />
    </label>
  )
}

// RulebookEditor is the Partner's window for a rule-book rule (SPEC-010 scope 8): plain text
// boxes for every section, lists one entry per line, no YAML. The ID, type and subject are
// shown, not edited. Saving needs a reason; the engine keeps the edit as a new office version
// and the rule-book file stays as it is.
export function RulebookEditor({ current, onSaved, onClose }: { current: RuleCurrent; onSaved: () => void; onClose: () => void }) {
  const [r, setR] = useState<RulebookRule>(() => structuredClone(current.rulebook as RulebookRule))
  const [reason, setReason] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  // set changes one field of one section by its path in the rule.
  const set = <S extends keyof RulebookRule>(section: S, field: string, value: unknown) =>
    setR((old) => ({ ...old, [section]: { ...(old[section] as object), [field]: value } }))
  const text = <S extends keyof RulebookRule>(section: S, field: string, name: string, long = false) => {
    const value = String((r[section] as Record<string, unknown>)[field] ?? '')
    const onChange = (e: { target: { value: string } }) => set(section, field, e.target.value)
    return (
      <label className="rbe-field">
        {name}
        {long ? <textarea rows={3} value={value} onChange={onChange} /> : <input value={value} onChange={onChange} />}
      </label>
    )
  }
  const lines = (value: string[], name: string, onChange: (v: string[]) => void) => (
    <LinesField value={value} name={name} onChange={onChange} />
  )
  const pairs = (value: RulebookPair[]) => value.map(pairText)
  const toPairs = (xs: string[]): RulebookPair[] => xs.map((x) => {
    const [left, right] = x.split('↔').map((s) => s.trim())
    return right === undefined ? { expression: left } : { left, right }
  })

  const save = async () => {
    if (busy) return
    if (reason.trim() === '') {
      setError('Say why the rule is changed: the reason is kept in the audit log.')
      return
    }
    setBusy(true)
    try {
      await saveRulebookRule(current.rule_id, reason, r)
      onSaved()
      onClose()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form className="rulebook-editor" aria-label={`Edit rule-book rule ${current.rule_id}`} onSubmit={(e) => { e.preventDefault(); void save() }}>
      <p className="rule-readiness muted" role="note">
        The rule book's file is not changed: your edit is kept as a new office version, with your name and the reason, and
        <strong> Reset to rule-book version</strong> puts the original back. ID {r.rule.id}, type {r.rule.type} and subject {r.rule.domain} stay as they are.
      </p>
      <fieldset><legend>Rule</legend>
        {text('rule', 'name', 'Name *')}
        {text('rule', 'category', 'Category')}
        {text('rule', 'purpose', 'Purpose *', true)}
      </fieldset>
      <fieldset><legend>Governance</legend>
        {text('governance', 'verification_status', 'Verification status')}
        {text('governance', 'verification_label', 'Verification label')}
        {text('governance', 'reviewed_by', 'Reviewed by')}
        {text('governance', 'reviewed_on', 'Reviewed on')}
        {text('governance', 'review_decision', 'Review decision')}
      </fieldset>
      <fieldset><legend>Validity</legend>
        {text('validity', 'valid_from', 'Valid from')}
        {text('validity', 'valid_to', 'Valid to')}
        {text('validity', 'effective_text', 'Effective')}
        {text('validity', 'period_basis', 'Period basis')}
      </fieldset>
      <fieldset><legend>Legal basis</legend>
        {text('legal', 'provisions', 'Provisions', true)}
        {text('legal', 'applicability', 'Applicability', true)}
        {text('legal', 'exemptions_exceptions', 'Exemptions and exceptions', true)}
        {text('legal', 'thresholds_rates', 'Thresholds and rates', true)}
        {text('legal', 'rounding', 'Rounding')}
        {text('legal', 'due_dates', 'Due dates')}
      </fieldset>
      <fieldset><legend>Inputs</legend>
        {lines(r.inputs.fields, 'Fields', (v) => set('inputs', 'fields', v))}
        {text('inputs', 'prerequisites', 'Prerequisites', true)}
        {text('inputs', 'input_validations', 'Input checks', true)}
      </fieldset>
      <fieldset><legend>Matching</legend>
        {lines(pairs(r.matching.keys), 'Keys (left ↔ right)', (v) => set('matching', 'keys', toPairs(v)))}
        {text('matching', 'key_normalisation', 'Key normalisation', true)}
        {text('matching', 'aggregation', 'Aggregation')}
        {lines(pairs(r.matching.comparisons), 'Comparisons (left ↔ right)', (v) => set('matching', 'comparisons', toPairs(v)))}
        {text('matching', 'tolerance', 'Tolerance')}
      </fieldset>
      <fieldset><legend>Calculation</legend>
        {text('calculation', 'formula', 'Formula', true)}
        {lines(r.calculation.steps, 'Steps', (v) => set('calculation', 'steps', v))}
      </fieldset>
      {r.parameters.length > 0 && (
        <fieldset><legend>Parameters</legend>
          {r.parameters.map((p, i) => (
            <label key={p.key} className="rbe-field">
              {p.label || p.key}{p.unit ? ` (${p.unit})` : ''}
              <input value={p.value} onChange={(e) => setR((old) => ({
                ...old, parameters: old.parameters.map((q, j) => (j === i ? { ...q, value: e.target.value } : q)),
              }))} />
            </label>
          ))}
        </fieldset>
      )}
      <fieldset><legend>Outcomes</legend>
        {r.outcomes.categories.map((o, i) => (
          <div key={i} className="rbe-row">
            <span className="mono">{o.code}</span>
            <label className="rbe-field">Meaning
              <input value={o.description} onChange={(e) => set('outcomes', 'categories', r.outcomes.categories.map((q, j) => (j === i ? { ...q, description: e.target.value } : q)))} />
            </label>
            <label className="rbe-field">Severity
              <input value={o.severity} onChange={(e) => set('outcomes', 'categories', r.outcomes.categories.map((q, j) => (j === i ? { ...q, severity: e.target.value } : q)))} />
            </label>
          </div>
        ))}
        {text('outcomes', 'default_severity', 'Default severity')}
        {text('outcomes', 'expected_output', 'Expected output', true)}
        {text('outcomes', 'recommended_action', 'Recommended action', true)}
        {text('outcomes', 'common_discrepancies', 'Common discrepancies', true)}
      </fieldset>
      <fieldset><legend>Tests</legend>
        {text('tests', 'worked_example', 'Worked example', true)}
        {text('tests', 'positive', 'Positive', true)}
        {text('tests', 'negative', 'Negative', true)}
      </fieldset>
      <fieldset><legend>Pseudocode</legend>
        {text('implementation', 'pseudocode', 'Pseudocode', true)}
      </fieldset>
      <fieldset><legend>Notes</legend>
        <label className="rbe-field">Notes
          <textarea rows={3} value={r.notes} onChange={(e) => setR((old) => ({ ...old, notes: e.target.value }))} />
        </label>
      </fieldset>
      <div className="form-actions">
        <label className="rbe-field rbe-reason">Reason for the change *
          <input value={reason} onChange={(e) => setReason(e.target.value)} aria-required="true" />
        </label>
        {error && <p className="fail-text" role="alert">{error}</p>}
        <button type="button" onClick={onClose} disabled={busy}>Cancel</button>
        <button type="submit" className="primary" disabled={busy}>Save as office version</button>
      </div>
    </form>
  )
}
