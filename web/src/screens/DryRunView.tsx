import type { DryRun } from '../formatsTypes.ts'

// DryRunView says which rules can run on a layout and which cannot check, so the admin sees what
// a mapping gives before it is saved or published (SPEC-016). Only rules that read this layout's
// document are listed; a missing figure is named, and the rule is "cannot check", not failed.
export function DryRunView({ dry }: { dry: DryRun }) {
  const none = dry.can_run.length === 0 && dry.cannot_check.length === 0
  return (
    <section className="fmt-dry" aria-label="Which rules can run">
      <h3>Which rules can run</h3>
      {none ? (
        <p className="muted">No rule reads the figures of this document.</p>
      ) : (
        <div className="fmt-dry-cols">
          <div>
            <h4>{dry.can_run.length} can run</h4>
            {dry.can_run.length === 0 ? <p className="muted">None yet.</p> : (
              <ul>{dry.can_run.map((r) => <li key={r.id}><span className="mono">{r.id}</span> {r.title}</li>)}</ul>
            )}
          </div>
          <div>
            <h4>{dry.cannot_check.length} cannot check</h4>
            {dry.cannot_check.length === 0 ? <p className="muted">None.</p> : (
              <ul>
                {dry.cannot_check.map((r) => (
                  <li key={r.id}>
                    <span className="mono">{r.id}</span> {r.title}
                    <span className="muted"> Needs {(r.missing ?? []).join(', ')}.</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      )}
    </section>
  )
}
