import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { distinct, filterRules, noRuleFilter, ruleYearMatch, taxYearOptions, type RuleFilter } from '../filters.ts'
import { ApiError, fetchRuleCurrent, fetchRuleOptions, fetchRuleVersions, resetRule, pickRuleVersion } from '../api.ts'
import type { RuleCurrent, RuleInfo, RuleList, RuleOptions, RuleVersionRow } from '../types.ts'
import { ruleReadiness } from '../ruleForm.ts'
import { useRowNav } from '../useRowNav.ts'
import { usePager } from '../pager.ts'
import { PageBar } from './PageBar.tsx'
import { ApproveRule, RuleBuilder, SwitchRule } from './RuleBuilder.tsx'
import { Modal } from './Modal.tsx'
import { RuleDetail, RulebookEditor } from './RuleDetail.tsx'
import { useCan } from '../permissions.ts'
import { SearchIcon, SortIcon } from '../icons.tsx'
import '../styles/rules.css'

interface Props {
  list: RuleList
  live: boolean // true when the list came from the server, false for sample data
  onChanged?: () => void // after a rule is added, changed, switched or given another version: read the list again
}

const typeText: Record<string, string> = { statutory: 'Statutory', reconciliation: 'Reconciliation', 'tie-out': 'Tie-out' }
const statusText: Record<string, string> = { active: 'Active', testing: 'Testing', disabled: 'Switched off', draft: 'Draft' }
const statusTone: Record<string, string> = { active: 'pass', testing: 'note', disabled: 'muted', draft: 'muted' }
const sourceText: Record<string, string> = { rulebook: 'Rule book', builtin: 'Engine rule', office: 'Office edit' }
const domainText: Record<string, string> = {
  gst: 'GST', tds_tcs: 'TDS / TCS', income_tax: 'Income tax', payroll: 'Payroll', bank: 'Bank',
  accounting: 'Accounting', msme: 'MSME', company_law: 'Company law', audit: 'Audit', other: 'Other',
}
const label = (m: Record<string, string>, v: string) => m[v] ?? v

// What the Rule Builder page is open on: a new rule, or an existing one as it is now.
type Open = { mode: 'create' } | { mode: 'edit'; current: RuleCurrent } | null

// A switched-off rule is either described only (the rule book has it, but no check the engine
// can run) or a runnable rule someone disabled. The status filter alone cannot tell them apart.
type Runnable = '' | 'runnable' | 'described'

// The five tiles at the top of the page (Recon Screen Document v2 §10.1). Each one is a status,
// split as above for switched-off rules; pressing a tile filters the table.
type Tile = { key: string; label: string; status: string; runnable: Runnable; tone?: string; sub?: (rules: RuleInfo[]) => string }
const tiles: Tile[] = [
  { key: 'all', label: 'Rules in the book', status: '', runnable: '', sub: (rs) => `${rs.filter((r) => r.runnable).length} can run as checks` },
  { key: 'active', label: 'Approved', status: 'active', runnable: '', tone: 'pass' },
  { key: 'testing', label: 'Testing', status: 'testing', runnable: '', tone: 'warn', sub: () => 'waiting for approval' },
  { key: 'described', label: 'Switched off', status: 'disabled', runnable: 'described', sub: () => 'described only, not run yet' },
  { key: 'disabled', label: 'Disabled', status: 'disabled', runnable: 'runnable', tone: 'fail', sub: () => 'switched off with a reason' },
]
const runnableMatch = (r: RuleInfo, w: Runnable) => w === '' || (w === 'runnable') === r.runnable

type SortKey = 'id' | 'title' | 'type' | 'category' | 'years' | 'status' | 'source'
type Sort = { key: SortKey; dir: 'asc' | 'desc' } | null

const yearsText = (r: RuleInfo) =>
  r.tax_years.length > 0
    ? r.tax_years.join(', ')
    : r.valid_from || r.valid_to
      ? `${r.valid_from || '…'} to ${r.valid_to || 'no end'}`
      : 'not dated'

// sortText is what a column sorts by: the words the table shows, so the order matches the screen.
function sortText(r: RuleInfo, key: SortKey): string {
  switch (key) {
    case 'id': return r.id
    case 'title': return r.title
    case 'type': return `${label(typeText, r.type)} ${r.domain ? label(domainText, r.domain) : ''}`
    case 'category': return r.category
    case 'years': return yearsText(r)
    case 'status': return label(statusText, r.status)
    case 'source': return label(sourceText, r.source)
  }
}
const byText = (a: string, b: string) => a.localeCompare(b, 'en', { numeric: true, sensitivity: 'base' })

// The Rules view (SPEC-004 part 6, SPEC-010; Recon Screen Document v2 §10). Five tiles count the
// rules by status and filter the table; the filter row has tax year, type, subject, category,
// status, source and a search. "/" moves to the search box; Esc clears the filters; arrow keys
// move between rules and Enter opens one in a drawer. Column headings sort; a third click
// returns to the book's order. With recon-server running, the rules can be added and changed in
// the Rule Builder page, switched off and on again with a reason, and given another version
// (SPEC-010); without it the list is read only.
export function Rules({ list, live, onChanged }: Props) {
  const can = useCan() // what the signed-in role may do to a rule (SPEC-012)
  const [f, setF] = useState<RuleFilter>(noRuleFilter)
  const [runnable, setRunnable] = useState<Runnable>('')
  const [sort, setSort] = useState<Sort>(null)
  const [open, setOpen] = useState<string | null>(null)
  const [rbEdit, setRbEdit] = useState<RuleCurrent | null>(null) // a Partner's edit of a rule-book rule (SPEC-010 scope 8)
  const [builder, setBuilder] = useState<Open>(null)
  const [options, setOptions] = useState<RuleOptions | null>(null)
  const [switching, setSwitching] = useState<{ id: string; action: 'disable' | 'enable'; rulebook: boolean } | null>(null)
  const [approving, setApproving] = useState<string | null>(null)
  const [failure, setFailure] = useState<string | null>(null)
  const search = useRef<HTMLInputElement>(null)
  const table = useRef<HTMLTableElement>(null)
  const rules = list.rules
  const shown = useMemo(() => {
    const found = filterRules(rules, f).filter((r) => runnableMatch(r, runnable))
    if (!sort) return found
    const sorted = [...found].sort((a, b) => byText(sortText(a, sort.key), sortText(b, sort.key)) || byText(a.id, b.id))
    return sort.dir === 'asc' ? sorted : sorted.reverse()
  }, [rules, f, runnable, sort])
  const pager = usePager(shown, 'rules', `${JSON.stringify(f)}|${runnable}|${sort?.key ?? ''}|${sort?.dir ?? ''}`)
  const nav = useRowNav(pager.rows.length, (i) => setOpen(open === pager.rows[i].id ? null : pager.rows[i].id))
  // A new page starts with its first rule selected. The page is compared during render, not in
  // an effect, so the selection moves in the same render as the page.
  const [navPage, setNavPage] = useState(pager.page)
  if (navPage !== pager.page) {
    setNavPage(pager.page)
    nav.setSelected(0)
  }
  const active = Object.values(f).some((v) => v !== '') || runnable !== ''

  const valuesOf = (field: keyof RuleFilter, pick: (r: RuleInfo) => string) =>
    distinct(filterRules(rules, f, field), pick)
  const years = useMemo(() => taxYearOptions(rules, list.tax_years), [rules, list.tax_years])

  useEffect(() => {
    const onKey = (e: globalThis.KeyboardEvent) => {
      const target = e.target as HTMLElement | null
      if (e.key === '/' && !(target && ['INPUT', 'SELECT', 'TEXTAREA'].includes(target.tagName))) {
        e.preventDefault()
        search.current?.focus()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
  // The list takes focus when the screen opens and again when the Rule Builder popup closes.
  useEffect(() => {
    if (!builder) table.current?.focus({ preventScroll: true })
  }, [builder])

  const clearFilters = () => {
    setF(noRuleFilter)
    setRunnable('')
  }
  // Esc closes the rule drawer first, then clears the filters; with neither, it goes back.
  const clearOnEscape = (e: KeyboardEvent) => {
    if (e.key !== 'Escape') return
    if (open !== null) {
      e.stopPropagation()
      setOpen(null)
      table.current?.focus()
    } else if (active) {
      e.stopPropagation()
      clearFilters()
      table.current?.focus()
    }
  }
  const openRule = rules.find((r) => r.id === open)
  const set = (field: keyof RuleFilter) => (e: { target: { value: string } }) => {
    setF({ ...f, [field]: e.target.value })
    if (field === 'status') setRunnable('')
  }

  const select = (field: keyof RuleFilter, name: string, values: string[], text: (v: string) => string = (v) => v) => (
    <label>
      {name}
      <select value={f[field]} onChange={set(field)} onKeyDown={clearOnEscape} aria-label={name} className="filter-select-premium">
        <option value="">All</option>
        {values.map((v) => (
          <option key={v} value={v}>
            {text(v)}
          </option>
        ))}
      </select>
    </label>
  )

  const sortBy = (key: SortKey) =>
    setSort((s) => (s?.key !== key ? { key, dir: 'asc' } : s.dir === 'asc' ? { key, dir: 'desc' } : null))
  const head = (key: SortKey, text: string) => {
    const dir = sort?.key === key ? sort.dir : null
    return (
      <th className="sortable" aria-sort={dir === 'asc' ? 'ascending' : dir === 'desc' ? 'descending' : 'none'}>
        <button type="button" onClick={() => sortBy(key)} title={`Sort by ${text.toLowerCase()}`}>
          {text} <SortIcon dir={dir} />
        </button>
      </th>
    )
  }

  // The lists the Rule Builder offers are read once, when it is first opened.
  const ensureOptions = async (): Promise<RuleOptions> => {
    if (options) return options
    const o = await fetchRuleOptions()
    setOptions(o)
    return o
  }
  const failed = (e: unknown) => setFailure(e instanceof ApiError ? e.message : String(e))
  const openCreate = async () => {
    setFailure(null)
    try {
      await ensureOptions()
      setBuilder({ mode: 'create' })
    } catch (e) {
      failed(e)
    }
  }
  const openEdit = async (id: string) => {
    setFailure(null)
    try {
      const current = await fetchRuleCurrent(id)
      if (current.rulebook) {
        setRbEdit(current)
        return
      }
      await ensureOptions()
      setBuilder({ mode: 'edit', current })
    } catch (e) {
      failed(e)
    }
  }
  const changed = () => onChanged?.()
  const switchOf = (r: RuleInfo) => setSwitching({ id: r.id, action: r.status === 'disabled' ? 'enable' : 'disable', rulebook: !r.runnable })

  const dialogs = (
    <>
      {rbEdit && (
        <Modal title={`Edit ${rbEdit.rule_id}`} onClose={() => setRbEdit(null)}>
          <RulebookEditor current={rbEdit} onSaved={changed} onClose={() => setRbEdit(null)} />
        </Modal>
      )}
      {approving && (
        <ApproveRule
          id={approving}
          onDone={() => {
            setApproving(null)
            changed()
          }}
          onClose={() => setApproving(null)}
        />
      )}
      {switching && (
        <SwitchRule
          id={switching.id}
          action={switching.action}
          rulebook={switching.rulebook}
          onDone={() => {
            setSwitching(null)
            changed()
          }}
          onClose={() => setSwitching(null)}
        />
      )}
    </>
  )

  // Add rule and Edit rule open as a popup over the rule book, so the rail and the list stay
  // where they were and nothing typed is lost by a stray click outside it.
  const builderPopup = builder && options ? (
    <Modal title={builder.mode === 'create' ? 'Add rule' : `Edit ${builder.current.rule_id}`} onClose={() => setBuilder(null)}>
      {builder.mode === 'create' ? (
        <RuleBuilder mode="create" current={null} options={options} taxYears={list.tax_years} onSaved={changed} onClose={() => setBuilder(null)} />
      ) : (
        <RuleBuilder
          mode="edit"
          current={builder.current}
          options={options}
          taxYears={list.tax_years}
          onSaved={changed}
          onApprove={can('activate_rules') ? () => {
            setApproving(builder.current.rule_id)
            setBuilder(null)
          } : undefined}
          onClose={() => setBuilder(null)}
        />
      )}
    </Modal>
  ) : null

  return (
    <section aria-labelledby="rules-title" className="rules-page" onKeyDown={clearOnEscape}>
      <div className="board-head">
        <div>
          <p className="eyebrow">The firm's rule book · {live ? 'the rules in the database' : 'read only'}</p>
          <h1 id="rules-title">Rules</h1>
        </div>
        <button
          type="button"
          className="action"
          onClick={openCreate}
          disabled={!live}
          title={live ? 'Add a rule in the Rule Builder' : 'Start recon-server to add a rule'}
        >
          Add rule
        </button>
      </div>
      {!live && (
        <p className="notice" role="note">
          Sample data: start recon-server to see the rules in the database.
        </p>
      )}
      {failure && (
        <p className="notice" role="alert">
          {failure}
        </p>
      )}
      <div className="kpi-strip rules-tiles" role="group" aria-label="Rules by status: choose a tile to filter the table">
        {tiles.map((t) => {
          const pressed = f.status === t.status && runnable === t.runnable
          return (
            <button
              key={t.key}
              type="button"
              className={t.tone ? `kpi ${t.tone}` : 'kpi'}
              aria-pressed={pressed}
              onClick={() => {
                const off = pressed && t.key !== 'all'
                setF({ ...f, status: off ? '' : t.status })
                setRunnable(off ? '' : t.runnable)
              }}
            >
              <span className="kpi-label">{t.label}</span>
              <span className="kpi-value">{rules.filter((r) => (t.status === '' || r.status === t.status) && runnableMatch(r, t.runnable)).length}</span>
              {t.sub && <span className="kpi-sub">{t.sub(rules)}</span>}
            </button>
          )
        })}
      </div>
      <div className="controls-premium rules-filters" role="group" aria-label="Rule filters">
        <label className="rules-search">
          Search
          <span className="search-box">
            <SearchIcon />
            <input
              ref={search}
              type="search"
              value={f.search}
              onChange={set('search')}
              onKeyDown={clearOnEscape}
              placeholder="ID, name or category"
              aria-label="Search rules"
              aria-keyshortcuts="/"
            />
            <kbd aria-hidden="true">/</kbd>
          </span>
        </label>
        <label>
          Tax year
          <select value={f.taxYear} onChange={set('taxYear')} onKeyDown={clearOnEscape} aria-label="Tax year" className="filter-select-premium">
            <option value="">All years</option>
            {years.map((y) => (
              <option key={y}>{y}</option>
            ))}
          </select>
        </label>
        {select('type', 'Type', valuesOf('type', (r) => r.type), (v) => label(typeText, v))}
        {select('domain', 'Subject', valuesOf('domain', (r) => r.domain), (v) => label(domainText, v))}
        {select('category', 'Category', valuesOf('category', (r) => r.category))}
        {select('status', 'Status', valuesOf('status', (r) => r.status), (v) => label(statusText, v))}
        {select('source', 'Source', valuesOf('source', (r) => r.source), (v) => label(sourceText, v))}
        <button type="button" onClick={clearFilters} disabled={!active} className="ghost">
          Clear filters
        </button>
      </div>
      <p className="result-count" aria-live="polite">
        {shown.length} of {rules.length} rules
        {f.taxYear && ' (rules with no tax year or dates count for every year and are marked "not dated")'}
      </p>
      {sort && (
        <p className="rules-sorted">
          <button type="button" className="ghost" onClick={() => setSort(null)}>
            Back to the rule book's order
          </button>
        </p>
      )}
      {shown.length === 0 && <p className="empty">No rule matches these filters. Press Esc to clear them.</p>}
      <div className="rules-layout">
        <div className="table-wrap">
          <table ref={table} className="grid stack table-premium rules-table" tabIndex={0} onKeyDown={(e) => pager.onKeyDown(e) || nav.onKeyDown(e)} aria-label="Rules list">
            <thead>
              <tr>
                {head('id', 'Rule')}
                {head('title', 'Name')}
                {head('type', 'Type and subject')}
                {head('category', 'Category')}
                {head('years', 'Tax years')}
                {head('status', 'Status')}
                {head('source', 'Source')}
                {live && <th>Actions</th>}
              </tr>
            </thead>
            <tbody>
              {pager.rows.map((r, i) => (
                <RuleRow
                  key={r.id}
                  r={r}
                  taxYear={f.taxYear}
                  selected={i === nav.selected}
                  open={open === r.id}
                  live={live}
                  onClick={() => {
                    nav.setSelected(i)
                    setOpen(open === r.id ? null : r.id)
                  }}
                  onEdit={() => openEdit(r.id)}
                  onApprove={() => setApproving(r.id)}
                  onSwitch={() => switchOf(r)}
                />
              ))}
            </tbody>
          </table>
        </div>
      </div>
      <PageBar p={pager} noun="rules" />
      {openRule && (
        <Modal
          title={openRule.id}
          onClose={() => {
            setOpen(null)
            table.current?.focus()
          }}
        >
          <RuleDrawer
            key={openRule.id}
            r={openRule}
            live={live}
            onEdit={() => {
              setOpen(null)
              openEdit(openRule.id)
            }}
            onApprove={() => {
              setOpen(null)
              setApproving(openRule.id)
            }}
            onSwitch={() => {
              setOpen(null)
              switchOf(openRule)
            }}
            onChanged={changed}
          />
        </Modal>
      )}
      {builderPopup}
      {dialogs}
    </section>
  )
}

const versionSource: Record<string, string> = { builtin: 'Engine rule (standard)', office: 'Office edit', rulebook: 'Rule book' }

// RuleDrawer shows one rule in the drawer: whether it can be approved, what it is, where it is
// kept, when it applies and, with the server running, its versions (SPEC-010 AC-5, AC-6) and its
// actions in the footer. Each change is recorded in the audit log with the signed-in user's name.
function RuleDrawer({ r, live, onEdit, onApprove, onSwitch, onChanged }: {
  r: RuleInfo
  live: boolean
  onEdit: () => void
  onApprove: () => void
  onSwitch: () => void
  onChanged: () => void
}) {
  const can = useCan()
  const withVersions = live
  const [versions, setVersions] = useState<RuleVersionRow[] | null>(null)
  const [detail, setDetail] = useState<RuleCurrent | null>(null) // the whole rule (SPEC-010 scope 8)
  const [detailError, setDetailError] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [reads, setReads] = useState(0) // bumped after a change, to read the versions again
  const message = (e: unknown) => (e instanceof ApiError ? e.message : String(e))
  useEffect(() => {
    if (!withVersions) return
    let current = true
    fetchRuleVersions(r.id).then(
      (v) => current && setVersions(v.versions ?? []),
      (e) => current && setError(message(e)),
    )
    fetchRuleCurrent(r.id).then(
      (c) => current && setDetail(c),
      (e) => current && setDetailError(message(e)),
    )
    return () => {
      current = false
    }
  }, [r.id, withVersions, reads])
  const change = async (action: () => Promise<unknown>) => {
    if (busy) return
    setBusy(true)
    try {
      await action()
      setError(null)
      setReads((n) => n + 1)
      onChanged()
    } catch (e) {
      setError(message(e))
    } finally {
      setBusy(false)
    }
  }
  const inUse = versions?.find((v) => v.active)
  // The standard version is the engine's built-in one, or, for a rule-book rule, the rule book's.
  const standardSource = r.runnable ? 'builtin' : 'rulebook'
  const standard = versions?.findLast((v) => v.source === standardSource)
  const note = ruleReadiness(r, live)
  const mayReset = r.source === 'office' && standard !== undefined && inUse !== undefined && inUse.source !== standardSource && can('activate_rules')
  const maySwitch = r.status === 'disabled' ? can('activate_rules') : can('disable_rules')
  // An engine rule is drafted by anyone who may draft rules; the rule book is CA-approved
  // content, so only a Partner edits a rule-book rule (SPEC-010 scope 8).
  const mayEdit = r.runnable ? can('draft_rules') : can('activate_rules')
  const actions = live && (maySwitch || mayEdit || mayReset || (r.runnable && r.status === 'testing' && can('activate_rules')))

  return (
    <section className="rule-drawer" aria-label={`Rule ${r.id}`}>
      <h3 className="rule-drawer-title">{r.title}</h3>
      <p className="rule-drawer-tags">
        <span className={`tag ${statusTone[r.status] ?? 'muted'}`}>{label(statusText, r.status)}</span>
        <span className="chip">{label(sourceText, r.source)}</span>
        {!r.runnable && <span className="muted">Described only, not run yet</span>}
      </p>
      <p className={`rule-readiness ${note.tone}`} role="note">{note.text}</p>
      <dl className="rule-facts">
        <dt>Type</dt>
        <dd>{label(typeText, r.type)}</dd>
        <dt>Subject</dt>
        <dd>{r.domain ? label(domainText, r.domain) : '-'}</dd>
        <dt>Category</dt>
        <dd>{r.category}</dd>
        <dt>Tax years</dt>
        <dd>{r.tax_years.length > 0 ? r.tax_years.join(', ') : 'not named'}</dd>
        <dt>Valid</dt>
        <dd>{r.valid_from || r.valid_to ? `${r.valid_from || '…'} to ${r.valid_to || 'no end'}` : 'not dated'}</dd>
        <dt>Version</dt>
        <dd>
          Version {r.version} (stored revision {r.revision})
        </dd>
        <dt>File</dt>
        <dd className="mono">{r.file_path}</dd>
      </dl>
      {live ? (
        detail ? <RuleDetail current={detail} /> : detailError ? (
          <p className="fail-text" role="alert">The rule's details could not be read: {detailError}</p>
        ) : <p className="muted">Reading the whole rule…</p>
      ) : (
        <p className="muted">The whole rule shows here when recon-server is running.</p>
      )}
      {withVersions && (
        <section aria-label={`Versions of ${r.id}`} className="versions">
          <h4>Versions</h4>
          {error && <p className="fail-text" role="alert">{error}</p>}
          {versions === null ? (
            !error && <p className="muted">Reading the versions…</p>
          ) : (
            <table className="grid table-premium" aria-label="Versions">
              <thead>
                <tr>
                  <th>Version</th>
                  <th>Source</th>
                  <th>Made by</th>
                  <th>In use</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                {versions.map((v) => (
                  <tr key={v.id}>
                    <td className="mono">{v.version}</td>
                    <td>{versionSource[v.source] ?? v.source}</td>
                    <td>{v.created_by}</td>
                    <td>{v.active ? <span className="tag pass">In use</span> : ''}</td>
                    <td>
                      {!v.active && can('activate_rules') && (
                        <button type="button" className="btn-sm-premium" disabled={busy} onClick={() => change(() => pickRuleVersion(r.id, v.id))} aria-label={`Use version ${v.version}`}>
                          Use this version
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      )}
      {actions && (
        <div className="rule-drawer-foot" role="group" aria-label="Actions on this rule">
          {maySwitch && (
            <button type="button" className={r.status === 'disabled' ? '' : 'danger'} onClick={onSwitch}>
              {r.status === 'disabled' ? 'Enable again' : 'Disable'}
            </button>
          )}
          {mayReset && (
            <button type="button" disabled={busy} onClick={() => change(() => resetRule(r.id))}>
              {r.runnable ? 'Reset to standard rule' : 'Reset to rule-book version'}
            </button>
          )}
          <span className="rule-drawer-gap" />
          {mayEdit && <button type="button" onClick={onEdit}>Edit this rule</button>}
          {r.runnable && r.status === 'testing' && can('activate_rules') && (
            <button type="button" className="primary" disabled={!r.approvable} onClick={onApprove} title={r.approvable ? undefined : note.text}>
              Approve
            </button>
          )}
        </div>
      )}
    </section>
  )
}

function RuleRow(props: {
  r: RuleInfo
  taxYear: string
  selected: boolean
  open: boolean
  live: boolean
  onClick: () => void
  onEdit: () => void
  onApprove: () => void
  onSwitch: () => void
}) {
  const { r, taxYear, selected, open, live, onClick, onEdit, onApprove, onSwitch } = props
  const can = useCan() // what the signed-in role may do to a rule (SPEC-012)
  const notDated = taxYear !== '' && ruleYearMatch(r, taxYear) === 'not_dated'
  return (
    <tr className={selected ? 'selected' : undefined} aria-selected={selected} aria-expanded={open} onClick={onClick}>
      <td className="mono"><span className="rule-id">{r.id}</span></td>
      <td data-label="Name" className="wide">
        {r.title}
        {!r.runnable && <div className="muted">Described only, not run yet</div>}
      </td>
      <td data-label="Type and subject">
        {label(typeText, r.type)}
        {r.domain && <div className="muted">{label(domainText, r.domain)}</div>}
      </td>
      <td data-label="Category">{r.category}</td>
      <td data-label="Tax years" className={notDated ? 'muted' : undefined}>{yearsText(r)}</td>
      <td data-label="Status">
        <span className={`tag ${statusTone[r.status] ?? 'muted'}`}>{label(statusText, r.status)}</span>
      </td>
      <td data-label="Source">{label(sourceText, r.source)}</td>
      {live && (
        <td data-label="Actions" className="row-actions">
          {/* A rule-book rule is edited by a Partner only (SPEC-010 scope 8). */}
          <>
              {(r.runnable ? can('draft_rules') : can('activate_rules')) && (
                <button type="button" className="btn-sm-premium" aria-label={`Edit ${r.id}`} onClick={(e) => { e.stopPropagation(); onEdit() }}>
                  Edit
                </button>
              )}
              {r.runnable && r.status === 'testing' && r.approvable && can('activate_rules') && (
                <button type="button" className="btn-sm-premium" aria-label={`Approve ${r.id}`} onClick={(e) => { e.stopPropagation(); onApprove() }}>
                  Approve
                </button>
              )}
              {(r.status === 'disabled' ? can('activate_rules') : can('disable_rules')) && (
                <button
                  type="button"
                  className={`btn-sm-premium ${r.status === 'disabled' ? '' : 'danger'}`}
                  aria-label={r.status === 'disabled' ? `Enable again ${r.id}` : `Disable ${r.id}`}
                  onClick={(e) => { e.stopPropagation(); onSwitch() }}
                >
                  {r.status === 'disabled' ? 'Enable again' : 'Disable'}
                </button>
              )}
          </>
        </td>
      )}
    </tr>
  )
}
