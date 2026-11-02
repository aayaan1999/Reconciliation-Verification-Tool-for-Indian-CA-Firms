import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  classifyItem, decideFinding, deleteFinding, exportClient, fetchClientResult, fetchClients, fetchRules, fetchSession,
  fetchWorklist, onSessionEnded, signOut, type SessionState,
} from './api.ts'

// downloadFile saves a file the server sent, under the name the server gave it (SPEC-013 §4).
function downloadFile(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}
import { PermissionContext, allowAll, canFrom, type Can } from './permissions.ts'
import { Login } from './screens/Login.tsx'
import { ChangePassword } from './screens/ChangePassword.tsx'
import { Admin } from './screens/Admin.tsx'
import {
  sampleClients, sampleRules, sampleRuns, sampleTaxYear, sampleWorklist, type SampleRun,
} from './sample/index.ts'
import { ClientFile } from './screens/ClientFile.tsx'
import { Rules } from './screens/Rules.tsx'
import { SeasonBoard, type BoardRow } from './screens/SeasonBoard.tsx'
import { Worklist } from './screens/Worklist.tsx'
import { features } from './features.ts'
import { Shortcuts } from './screens/Shortcuts.tsx'
import { CommandPalette, type PaletteAction } from './screens/CommandPalette.tsx'
import { Rail, type Theme } from './screens/Rail.tsx'
import type { Client, FactList, FindingDecision, RuleList, ServerClientList, TypeResult, WorklistItem } from './types.ts'

// The colour theme: follow the computer's setting, or a fixed light or dark one, kept in
// this browser only.
const themeKey = 'recon.theme'
function storedTheme(): Theme {
  try {
    const t = localStorage.getItem(themeKey)
    return t === 'light' || t === 'dark' ? t : 'system'
  } catch {
    return 'system'
  }
}

type Route = { screen: 'season' } | { screen: 'client'; id: string } | { screen: 'worklist' } | { screen: 'rules' } | { screen: 'admin' }

// The working screens (§11.2): Season board, Client file and Worklist, plus Rules, the
// read-only start of the Rule Builder. With recon-server running, the clients, their
// results and the rules come from the database, and clients can be added and their files
// checked; without it, the screens show sample data and say so.
// Keys: 1 Season board, 2 Worklist, 3 Rules, Esc back.
export default function App() {
  const [route, setRoute] = useState<Route>({ screen: 'season' })
  // Who is signed in (SPEC-012). null while the server is asked; the sign-in screen when no
  // one is; the sample data when there is no server at all.
  const [session, setSession] = useState<SessionState | null>(null)
  const me = session?.kind === 'signed_in' ? session.session : null
  const can = useMemo<Can>(() => (me ? canFrom(me.permissions) : allowAll), [me])
  // The Ctrl K search; it works only while the rail and the screens show, not in front of the
  // sign-in or the change of a starting password.
  const [palette, setPalette] = useState(false)
  const shellShown = session?.kind !== 'signed_out' && !me?.must_change_password
  const [liveRules, setLiveRules] = useState<RuleList | null>(null)
  const [liveClients, setLiveClients] = useState<ServerClientList | null>(null)
  const [liveRuns, setLiveRuns] = useState<Record<string, SampleRun[] | null>>({}) // per client, one per type of check
  const [loadingRun, setLoadingRun] = useState(false)
  const [uploaded, setUploaded] = useState<Record<string, SampleRun[]>>({}) // sample clients, this session only
  // Each client's decisions on its findings, by finding ID (SPEC-006 part 3).
  const [liveDecisions, setLiveDecisions] = useState<Record<string, Record<string, FindingDecision>>>({})
  // The Worklist of the tax year, from the database (SPEC-006 part 3); null until loaded.
  const [liveWorklist, setLiveWorklist] = useState<WorklistItem[] | null>(null)
  // The tax year each client's file shows, and the years in which it has runs.
  const [clientYear, setClientYear] = useState<Record<string, { year: string; years: string[] }>>({})

  const live = liveRules !== null
  const rules = liveRules ?? sampleRules
  const taxYear = rules.tax_years[0] ?? sampleTaxYear

  const refreshClients = useCallback(async (ty: string) => {
    const list = await fetchClients(ty)
    if (list) setLiveClients(list)
  }, [])

  // The rules are read again after one is added, changed, switched or given another version.
  const refreshRules = useCallback(async () => {
    const list = await fetchRules()
    if (list) setLiveRules(list)
  }, [])

  // The app reads the server only once someone may use it: a signed-in user who has chosen
  // their password, or no server at all (the sample data).
  const loadable = session !== null && session.kind !== 'signed_out' && !me?.must_change_password

  // The signed-out state drops everything read from the server, so the next sign-in reads it
  // again and the previous user's figures are not left on screen.
  const endSession = useCallback(() => {
    setSession({ kind: 'signed_out' })
    setLiveRules(null)
    setLiveClients(null)
    setLiveRuns({})
    setLiveDecisions({})
    setLiveWorklist(null)
    setClientYear({})
    setUploaded({})
    setPalette(false)
    setRoute({ screen: 'season' })
  }, [])

  useEffect(() => {
    let cancelled = false
    fetchSession().then((s) => {
      if (!cancelled) setSession(s)
    })
    return () => {
      cancelled = true
    }
  }, [])

  // A request refused with 401 (the session ended) brings the sign-in screen back.
  useEffect(() => {
    onSessionEnded(endSession)
    return () => onSessionEnded(null)
  }, [endSession])

  useEffect(() => {
    if (!loadable) return
    let cancelled = false
    fetchRules().then((list) => {
      if (cancelled || !list) return
      setLiveRules(list)
      refreshClients(list.tax_years[0] ?? sampleTaxYear)
    })
    return () => {
      cancelled = true
    }
  }, [refreshClients, loadable])

  // The Worklist is read again each time it is opened, so runs since are in it (the last
  // list read stays on screen until the new one comes).
  useEffect(() => {
    if (!live || route.screen !== 'worklist') return
    let cancelled = false
    fetchWorklist(taxYear).then((wl) => {
      if (!cancelled) setLiveWorklist(wl?.items ?? [])
    })
    return () => {
      cancelled = true
    }
  }, [live, route.screen, taxYear])

  const [theme, setTheme] = useState<Theme>(storedTheme)
  useEffect(() => {
    const root = document.documentElement
    if (theme === 'system') root.removeAttribute('data-theme')
    else root.setAttribute('data-theme', theme)
    try {
      localStorage.setItem(themeKey, theme)
    } catch {
      // Not kept in this browser: the OS setting is followed next time.
    }
  }, [theme])

  const [shortcuts, setShortcuts] = useState(false)
  // Each screen opens at its top, not at the scroll position of the screen before it.
  // (A block body: browsers may return a Promise from scrollTo, which React would take as cleanup.)
  useEffect(() => {
    window.scrollTo?.(0, 0)
  }, [route])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      // Ctrl K opens the search from anywhere, a field included (it types no character).
      if (shellShown && (e.ctrlKey || e.metaKey) && !e.altKey && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setShortcuts(false)
        setPalette(true)
        return
      }
      // With the search open, the keys belong to it: Esc closes it and nothing else.
      if (palette) {
        if (e.key === 'Escape') setPalette(false)
        return
      }
      const target = e.target as HTMLElement | null
      if (target && ['INPUT', 'SELECT', 'TEXTAREA'].includes(target.tagName)) return
      if (shortcuts) {
        if (e.key === 'Escape' || e.key === '?') setShortcuts(false)
        return
      }
      if (e.key === '?') setShortcuts(true)
      else if (e.key === '1') setRoute({ screen: 'season' })
      else if (e.key === '2' && features.worklist) setRoute({ screen: 'worklist' })
      else if (e.key === '3') setRoute({ screen: 'rules' })
      else if (e.key === '4' && me && can('manage_users')) setRoute({ screen: 'admin' })
      else if (e.key === 'Escape') setRoute({ screen: 'season' })
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [shortcuts, palette, shellShown, me, can])

  // Opening a client in the database loads its latest result of each type of check for a
  // tax year: the one asked for, else the one last shown, else the latest year with a run
  // (so a client checked for 2026-27 does not open on 2025-26).
  const openClient = async (id: string, year?: string) => {
    setRoute({ screen: 'client', id })
    if (!live) return
    setLoadingRun(true)
    try {
      let y = year ?? clientYear[id]?.year ?? taxYear
      let res = await fetchClientResult(Number(id), y)
      const years = res.run_tax_years ?? []
      const latest = years[years.length - 1]
      if (year === undefined && clientYear[id] === undefined && latest && latest !== y) {
        y = latest
        res = await fetchClientResult(Number(id), y)
      }
      const runs = typeRuns(res.results ?? (res.report ? [{ comparison_id: '', last_run_on: '', report: res.report }] : []), res.facts)
      setLiveRuns((r) => ({ ...r, [id]: runs.length > 0 ? runs : null }))
      setLiveDecisions((d) => ({ ...d, [id]: res.decisions ?? {} }))
      setClientYear((c) => ({ ...c, [id]: { year: y, years } }))
    } catch {
      setLiveRuns((r) => ({ ...r, [id]: null }))
    } finally {
      setLoadingRun(false)
    }
  }

  // The rows of the Season board.
  const sampleList: Client[] = sampleClients.map((c) => (uploaded[c.id] ? { ...c, last_run_on: 'uploaded now' } : c))
  const sampleRunsFor = (c: Client): SampleRun[] => uploaded[c.id] ?? c.runs.map((r) => sampleRuns[r])
  const rows: BoardRow[] = live
    ? (liveClients?.clients ?? []).map((c) => ({
        id: String(c.id), name: c.name, pan_last4: c.pan_last4, last_run_on: c.last_run_on,
        files: c.counts ? c.documents : null, fail: c.counts?.fail ?? null, cannot: c.counts?.cannot_check ?? null,
      }))
    : sampleList.map((c) => {
        // As on the live board: the sum over the client's types of check.
        const rs = sampleRunsFor(c).map((x) => x.report)
        const sum = (pick: (r: (typeof rs)[number]) => number) => (rs.length > 0 ? rs.reduce((n, r) => n + pick(r), 0) : null)
        return {
          id: c.id, name: c.name, pan_last4: c.pan_last4, last_run_on: c.last_run_on,
          files: rs.length > 0 ? new Set(rs.flatMap((r) => r.files.map((f) => f.file))).size : null,
          fail: sum((r) => r.counts.fail), cannot: sum((r) => r.counts.cannot_check),
        }
      })

  // What waits on each screen, for the rail: findings across the board's clients, and Worklist
  // items not yet classified (known once the Worklist has been read).
  const withRun = rows.filter((r) => r.fail !== null)
  const seasonFindings = withRun.length > 0 ? withRun.reduce((n, r) => n + (r.fail ?? 0), 0) : null
  const worklistItems = live ? liveWorklist : sampleWorklist
  const toClassify = worklistItems ? worklistItems.filter((i) => i.classification === null).length : null

  const renderClient = (id: string) => {
    if (live) {
      const c = liveClients?.clients.find((x) => String(x.id) === id)
      if (!c) return null
      const client: Client = { id, name: c.name, pan_last4: c.pan_last4, runs: [], last_run_on: c.last_run_on }
      return (
        <ClientFile
          key={id}
          client={client}
          runs={liveRuns[id] ?? []}
          rules={rules.rules}
          viewYear={clientYear[id]?.year ?? taxYear}
          runYears={clientYear[id]?.years ?? []}
          onYear={(y) => openClient(id, y)}
          decisions={liveDecisions[id] ?? {}}
          onDecide={can('decide') ? async (findingId, decision, note) => {
            const d = await decideFinding(c.id, clientYear[id]?.year ?? taxYear, findingId, decision, note)
            setLiveDecisions((all) => ({ ...all, [id]: { ...(all[id] ?? {}), [findingId]: d } }))
          } : undefined}
          onExport={can('export') ? async () => {
            const file = await exportClient(c.id, clientYear[id]?.year ?? taxYear)
            downloadFile(file.blob, file.filename)
          } : undefined}
          onDeleteFinding={can('delete_findings') ? async (f, reason) => {
            await deleteFinding(c.id, clientYear[id]?.year ?? taxYear, f.finding_id, reason)
          } : undefined}
          live
          fromUpload
          loading={loadingRun && liveRuns[id] === undefined}
          taxYears={rules.tax_years}
          clientId={c.id}
          onEdited={() => refreshClients(taxYear)}
          onRun={(runs, year, decisions) => {
            setLiveRuns((r) => ({ ...r, [id]: runs }))
            setLiveDecisions((d) => ({ ...d, [id]: decisions ?? {} }))
            setClientYear((c) => {
              const years = [...new Set([...(c[id]?.years ?? []), year])].sort()
              return { ...c, [id]: { year, years } }
            })
            refreshClients(taxYear)
          }}
        />
      )
    }
    const c = sampleList.find((x) => x.id === id)
    if (!c) return null
    return (
      <ClientFile
        key={id}
        client={c}
        runs={sampleRunsFor(c)}
        rules={rules.rules}
        live={false}
        fromUpload={uploaded[id] !== undefined}
        taxYears={rules.tax_years}
        onRun={(runs) => setUploaded((u) => ({ ...u, [id]: runs }))}
      />
    )
  }

  const leave = me ? async () => {
    try {
      await signOut()
    } catch {
      // Already signed out: the sign-in screen comes back either way.
    }
    endSession()
  } : undefined

  // What the Ctrl K search offers besides clients and rules: only what the rail and its user
  // menu already do, and Admin only for a user who may manage users.
  const showAdmin = me !== null && can('manage_users')
  const paletteActions: PaletteAction[] = [
    { id: 'season', text: 'Go to Season board', hint: '1', run: () => setRoute({ screen: 'season' }) },
    ...(features.worklist ? [{ id: 'worklist', text: 'Go to Worklist', hint: '2', run: () => setRoute({ screen: 'worklist' }) }] : []),
    { id: 'rules', text: 'Go to Rules', hint: '3', run: () => setRoute({ screen: 'rules' }) },
    ...(showAdmin ? [{ id: 'admin', text: 'Go to Admin', hint: '4', run: () => setRoute({ screen: 'admin' }) }] : []),
    { id: 'light', text: 'Theme: Light', current: theme === 'light', run: () => setTheme('light') },
    { id: 'dark', text: 'Theme: Dark', current: theme === 'dark', run: () => setTheme('dark') },
    { id: 'system', text: 'Theme: Match the computer', current: theme === 'system', run: () => setTheme('system') },
    { id: 'shortcuts', text: 'Keyboard shortcuts', hint: '?', run: () => setShortcuts(true) },
    ...(leave ? [{ id: 'sign-out', text: 'Sign out', run: leave }] : []),
  ]
  const closePalette = useCallback(() => setPalette(false), [])

  // The sign-in comes first: nothing of the tool shows until someone signs in, and a user with a
  // starting password sees only the change of password (SPEC-012 §2).
  // While the session is being asked, the screens show with no server data (nothing is read
  // until the answer comes: see loadable).
  if (session?.kind === 'signed_out') return <Login onSignedIn={(s) => setSession({ kind: 'signed_in', session: s })} />
  if (me?.must_change_password) {
    return (
      <ChangePassword
        session={me}
        onChanged={(s) => setSession({ kind: 'signed_in', session: s })}
        onSignedOut={endSession}
      />
    )
  }

  return (
    <PermissionContext.Provider value={can}>
      <Rail
        current={route.screen === 'client' ? 'season' : route.screen}
        onNav={(screen) => setRoute({ screen })}
        taxYear={taxYear}
        findings={seasonFindings}
        toClassify={toClassify}
        showWorklist={features.worklist}
        showAdmin={showAdmin}
        live={live}
        user={me ? { name: me.user.display_name, role: me.user.role.replace('_', '-') } : null}
        theme={theme}
        onTheme={setTheme}
        onShortcuts={() => setShortcuts(true)}
        onSearch={() => setPalette(true)}
        onSignOut={leave}
      />
      {shortcuts && <Shortcuts showWorklist={features.worklist} onClose={() => setShortcuts(false)} />}
      {palette && (
        <CommandPalette
          clients={rows}
          rules={rules.rules}
          actions={paletteActions}
          onOpenClient={openClient}
          onOpenRule={() => setRoute({ screen: 'rules' })}
          onClose={closePalette}
        />
      )}
      <main key={route.screen === 'client' ? `client-${route.id}` : route.screen} className="screen">
        {route.screen === 'season' && (
          <SeasonBoard
            taxYear={taxYear}
            rows={rows}
            live={live}
            taxYears={rules.tax_years}
            onOpen={openClient}
            onAdded={async (c) => {
              await refreshClients(taxYear)
              openClient(String(c.id))
            }}
          />
        )}
        {route.screen === 'client' && renderClient(route.id)}
        {route.screen === 'worklist' && features.worklist &&
          (live ? (
            <Worklist
              items={liveWorklist ?? []}
              taxYear={taxYear}
              live
              loading={liveWorklist === null}
              onClassify={async (item, classification) => {
                const items = await classifyItem(item.client_id, taxYear, item.kind, item.ledger, classification)
                setLiveWorklist((all) => [
                  ...(all ?? []).filter((x) => x.client_id !== item.client_id),
                  ...items,
                ].sort((a, b) => a.client_name.localeCompare(b.client_name) || a.client_id - b.client_id))
              }}
            />
          ) : (
            <Worklist items={sampleWorklist} taxYear={sampleTaxYear} live={false} />
          ))}
        {route.screen === 'rules' && <Rules list={rules} live={live} onChanged={refreshRules} />}
        {route.screen === 'admin' && me && can('manage_users') && <Admin me={me.user} />}
      </main>
    </PermissionContext.Provider>
  )
}

// typeRuns turns a client's latest result of each type into the runs its Client file shows.
function typeRuns(results: TypeResult[], facts: FactList): SampleRun[] {
  return results.map((r) => ({ report: r.report, facts, last_run_on: r.last_run_on }))
}
