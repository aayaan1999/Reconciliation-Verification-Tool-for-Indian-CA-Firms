import { useCallback, useEffect, useId, useMemo, useState, type FormEvent } from 'react'
import {
  ApiError, adminAddUser, adminResetPassword, adminSetActive, adminSetLockout, adminSetRole, adminUnlock,
  fetchAdminUsers, fetchAudit,
} from '../api.ts'
import { initials } from '../format.ts'
import { CheckIcon, CloseIcon, SearchIcon } from '../icons.tsx'
import { roleMatrix, roleNames, roleSummaries } from '../permissions.ts'
import type { Account, AdminList, AuditEntry, AuditList, Lockout } from '../types.ts'
import { Modal } from './Modal.tsx'
import { PageBar } from './PageBar.tsx'
import { usePager } from '../pager.ts'
import { Field } from './RuleBuilder.tsx'
import '../styles/admin.css'

// The Admin tab (SPEC-012 §4; Recon Screen Document v2 §11), for the Partner only, in three
// tabs: Users (add, change the role, reset the password, unlock, disable), Sign-in security
// (the lockout setting) and the Audit log. The server checks and logs every change; this screen
// only asks for it and shows the answer. It writes nothing to the audit log itself.

// The lockout limits the server accepts (internal/auth/service.go).
const failedRange = { min: 1, max: 20 }
const minutesRange = { min: 1, max: 1440 }

// The server's limits on a password, a login name, a display name and a reason.
const passwordLength = { min: 10, max: 200 }

const auditPageSize = 25

const tabs = [
  { id: 'users', name: 'Users' },
  { id: 'security', name: 'Sign-in security' },
  { id: 'audit', name: 'Audit log' },
] as const
type Tab = (typeof tabs)[number]['id']

const roleName = (role: string) => roleNames[role] ?? role

function when(at: string): string {
  return at.slice(0, 16).replace('T', ' ') + ' UTC'
}

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

// The one state a user's row shows, with the tone of its tag. Disabled outranks Locked, which
// outranks a pending password change, as the user meets them at sign-in in that order.
function status(u: Account): { text: string; tone: string } {
  if (!u.active) return { text: 'Disabled', tone: 'muted' }
  if (u.locked) return { text: 'Locked', tone: 'fail' }
  if (u.must_change_password) return { text: 'Must change password', tone: 'note' }
  return { text: 'Active', tone: 'pass' }
}

// The colour of an action's tag in the audit log: green for what was granted, red for what was
// refused or taken away, amber for passwords, violet for roles and settings. Others are grey.
const actionTones: Record<string, string> = {
  'user.setup': 'pass', 'user.add': 'pass', 'user.enable': 'pass', 'login.success': 'pass', 'user.unlock': 'pass',
  'login.failure': 'fail', 'setup.refused': 'fail', 'user.disable': 'fail',
  'password.change': 'note', 'user.password_reset': 'note',
  'user.role_change': 'violet', 'auth.lockout_set': 'violet',
}

// A problem shown on the screen: the server's own words, with its code.
function problem(err: unknown): string {
  if (err instanceof ApiError) return `${err.message} (${err.code})`
  return String(err)
}

// suggestPassword makes a random starting password for the Partner to pass on, in four groups
// of four from letters and digits that are hard to confuse (no 0, O, 1, l or I). Bytes at or
// above the largest multiple of the alphabet's length are drawn again, so every character is
// equally likely. It is a help on the screen only; the server checks the password it is sent.
const passwordAlphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789'
function suggestPassword(): string {
  const limit = 256 - (256 % passwordAlphabet.length)
  const chars: string[] = []
  while (chars.length < 16) {
    for (const b of crypto.getRandomValues(new Uint8Array(32))) {
      if (b < limit && chars.length < 16) chars.push(passwordAlphabet[b % passwordAlphabet.length])
    }
  }
  return [0, 4, 8, 12].map((i) => chars.slice(i, i + 4).join('')).join('-')
}

function Avatar({ u }: { u: Account }) {
  return <span className="client-avatar" aria-hidden="true">{initials(u.display_name) || u.login_name.slice(0, 1).toUpperCase()}</span>
}

// A change sent to the server: it resolves to the server's problem, or null when it was done.
type Act = (change: () => Promise<unknown>, done: string) => Promise<string | null>

export function Admin({ me }: { me: Account }) {
  const [list, setList] = useState<AdminList | null>(null)
  const [audit, setAudit] = useState<AuditList | null>(null)
  const [tab, setTab] = useState<Tab>('users')
  const [selected, setSelected] = useState<number | null>(null)
  const [dialog, setDialog] = useState<'add' | 'roles' | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const base = useId()

  const reload = useCallback(async () => {
    try {
      const [users, log] = await Promise.all([fetchAdminUsers(), fetchAudit()])
      setList(users)
      setAudit(log)
    } catch (err) {
      setError(problem(err))
    }
  }, [])

  // The first read: the state is set only when the answer arrives, never inside the effect.
  useEffect(() => {
    let cancelled = false
    Promise.all([fetchAdminUsers(), fetchAudit()])
      .then(([users, log]) => {
        if (!cancelled) {
          setList(users)
          setAudit(log)
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(problem(err))
      })
    return () => {
      cancelled = true
    }
  }, [])

  // act runs one change and, when the server accepts it, shows the outcome and reads the users
  // and the audit log again (the server has written the change to the log).
  const act: Act = async (change, done) => {
    setError(null)
    setNotice(null)
    setBusy(true)
    try {
      await change()
      setNotice(done)
      await reload()
      return null
    } catch (err) {
      return problem(err)
    } finally {
      setBusy(false)
    }
  }


  const users = list?.users ?? []
  const user = users.find((u) => u.id === selected) ?? null
  const layerOpen = dialog !== null || user !== null

  return (
    <section aria-labelledby="admin-title" className="admin-screen">
      <div className="board-head">
        <div>
          <p className="eyebrow">Partner only</p>
          <h1 id="admin-title">Admin</h1>
          <p className="muted admin-lede">
            Signed in as {me.display_name} ({me.login_name}). The server writes every change made here to the audit log with your login name.
          </p>
        </div>
        {tab === 'users' && list !== null && (
          <button type="button" className="action" onClick={() => setDialog('add')}>
            + Add a user
          </button>
        )}
      </div>
      {error && <p className="error" role="alert">{error}</p>}
      {notice && !layerOpen && <p className="notice" role="status">{notice}</p>}

      <div className="tabs admin-tabs" role="tablist" aria-label="Admin sections">
        {tabs.map((t) => (
          <button
            key={t.id}
            id={`${base}-tab-${t.id}`}
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            aria-controls={`${base}-panel`}
            onClick={() => setTab(t.id)}
          >
            {t.name}
          </button>
        ))}
      </div>

      <div id={`${base}-panel`} role="tabpanel" aria-labelledby={`${base}-tab-${tab}`} className="admin-panel">
        {tab === 'users' && (
          <UsersPanel list={list} me={me} onManage={setSelected} onRoles={() => setDialog('roles')} />
        )}
        {tab === 'security' && (
          list === null ? (
            <p className="loading">Loading the sign-in settings…</p>
          ) : (
            <SecurityPanel
              key={`${list.lockout.max_failed_logins}-${list.lockout.lock_minutes}`}
              lockout={list.lockout}
              busy={busy}
              onSave={(l) => act(() => adminSetLockout(l), `Lock after ${l.max_failed_logins} failed sign-ins, for ${l.lock_minutes} minutes.`)}
            />
          )
        )}
        {tab === 'audit' && (audit === null ? <p className="loading">Loading the audit log…</p> : <AuditPanel audit={audit} />)}
      </div>

      {dialog === 'roles' && <RoleMatrix onClose={() => setDialog(null)} />}
      {dialog === 'add' && list !== null && (
        <AddUser
          roles={list.roles}
          busy={busy}
          onClose={() => setDialog(null)}
          onAdd={async (u) => {
            const p = await act(() => adminAddUser(u), `${u.login_name} is added.`)
            if (p === null) setDialog(null)
            return p
          }}
        />
      )}
      {user && list && (
        <ManageUser key={user.id} user={user} me={me} roles={list.roles} busy={busy} notice={notice} act={act}
          onClose={() => {
            setSelected(null)
            setNotice(null)
          }} />
      )}
    </section>
  )
}

function UsersPanel({ list, me, onManage, onRoles }: { list: AdminList | null; me: Account; onManage: (id: number) => void; onRoles: () => void }) {
  // The users are shown a page at a time, with the page bar under the list, as on the other screens.
  const pager = usePager(list?.users ?? [], 'users')
  if (list === null) return <p className="loading">Loading the users…</p>
  return (
    <>
      <div className="admin-toolbar">
        <span className="result-count">{plural(list.users.length, 'user', 'users')}</span>
        <button type="button" onClick={onRoles}>What each role can do</button>
      </div>
      <div className="table-wrap">
        <table className="table-premium admin-users" aria-label="Users">
          <thead>
            <tr>
              <th scope="col">User</th>
              <th scope="col">Role</th>
              <th scope="col">Status</th>
              <th scope="col">Last sign-in</th>
              <th scope="col"><span className="sr-only">Actions</span></th>
            </tr>
          </thead>
          <tbody>
            {pager.rows.map((u) => {
              const s = status(u)
              return (
                <tr key={u.id}>
                  <td>
                    <span className="client-cell">
                      <Avatar u={u} />
                      <span className="admin-who">
                        <span className="admin-name">
                          {u.display_name}
                          {u.id === me.id && <span className="muted"> (you)</span>}
                        </span>
                        <span className="mono admin-login">{u.login_name}</span>
                      </span>
                    </span>
                  </td>
                  <td>{roleName(u.role)}</td>
                  <td><span className={`tag ${s.tone}`}>{s.text}</span></td>
                  <td>{u.last_login_at ? when(u.last_login_at) : <span className="muted">Never</span>}</td>
                  <td className="admin-row-action">
                    <button type="button" aria-label={`Manage ${u.login_name}`} onClick={() => onManage(u.id)}>Manage</button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
      <PageBar p={pager} noun="users" />
    </>
  )
}

// RoleMatrix shows the four roles against the nine things a role may be allowed to do.
function RoleMatrix({ onClose }: { onClose: () => void }) {
  const roles = Object.keys(roleNames)
  return (
    <Modal title="What each role can do" onClose={onClose}>
      <div className="table-wrap">
        <table className="table-premium admin-matrix" aria-label="What each role can do">
          <thead>
            <tr>
              <th scope="col">Action</th>
              {roles.map((r) => (
                <th scope="col" key={r}>{roleName(r)}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {roleMatrix.map((row) => (
              <tr key={row.permission}>
                <th scope="row">{row.action}</th>
                {roles.map((r) => (
                  <td key={r}>
                    {row.roles.includes(r) ? (
                      <span className="admin-yes"><CheckIcon /> Yes</span>
                    ) : (
                      <span className="admin-no"><CloseIcon /> No</span>
                    )}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="muted admin-matrix-note">The screens hide what a role may not do, and the server refuses it whatever the screen shows.</p>
    </Modal>
  )
}

// RolePicker chooses a role, each with the sentence that says what it may do.
function RolePicker({ roles, value, onChange, disabled }: { roles: string[]; value: string; onChange: (role: string) => void; disabled?: boolean }) {
  const name = useId()
  return (
    <fieldset className="admin-roles" disabled={disabled}>
      <legend>Role</legend>
      {roles.map((r) => (
        <label key={r} className={value === r ? 'admin-role chosen' : 'admin-role'}>
          <input type="radio" name={name} value={r} checked={value === r} onChange={() => onChange(r)}
            aria-labelledby={`${name}-${r}-name`} aria-describedby={`${name}-${r}`} />
          <span>
            <strong id={`${name}-${r}-name`}>{roleName(r)}</strong>
            {roleSummaries[r] && <span id={`${name}-${r}`} className="muted">{roleSummaries[r]}</span>}
          </span>
        </label>
      ))}
    </fieldset>
  )
}

// PasswordField is a masked password box with a "show" toggle (SPEC-012 §4) and, when asked,
// a Suggest button that fills in a random password and shows it so it can be passed on.
function PasswordField({ label, value, onChange, help, suggest }: { label: string; value: string; onChange: (v: string) => void; help?: string; suggest?: boolean }) {
  const [show, setShow] = useState(false)
  return (
    <Field label={label} help={help} required>
      {(id) => (
        <div className="admin-password">
          <input id={id} type={show ? 'text' : 'password'} autoComplete="new-password" spellCheck={false} required
            minLength={passwordLength.min} maxLength={passwordLength.max} value={value} onChange={(e) => onChange(e.target.value)} />
          <button type="button" aria-pressed={show} onClick={() => setShow((s) => !s)}>{show ? 'Hide' : 'Show'}</button>
          {suggest && (
            <button type="button" onClick={() => {
              onChange(suggestPassword())
              setShow(true)
            }}>Suggest</button>
          )}
        </div>
      )}
    </Field>
  )
}

function AddUser({ roles, busy, onAdd, onClose }: {
  roles: string[]
  busy: boolean
  onAdd: (u: { login_name: string; display_name: string; role: string; password: string }) => Promise<string | null>
  onClose: () => void
}) {
  const [loginName, setLoginName] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [role, setRole] = useState(roles.includes('staff') ? 'staff' : roles[0] ?? '')
  const [password, setPassword] = useState('')
  const [fault, setFault] = useState<string | null>(null)
  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (busy) return
    setFault(await onAdd({ login_name: loginName, display_name: displayName, role, password }))
  }
  return (
    <Modal title="Add a user" onClose={onClose} closeDisabled={busy}>
      <form className="admin-form" aria-label="Add user" onSubmit={submit}>
        <p className="muted">The user signs in with this starting password and must choose their own at the first sign-in.</p>
        <div className="admin-grid">
          <Field label="Display name" required>
            {(id) => <input id={id} type="text" maxLength={100} required value={displayName} onChange={(e) => setDisplayName(e.target.value)} />}
          </Field>
          <Field label="Login name" help="Letters and digits, with dots, underscores or hyphens; 2 to 40 characters." required>
            {(id) => <input id={id} type="text" className="mono" maxLength={40} required autoComplete="off" spellCheck={false} value={loginName} onChange={(e) => setLoginName(e.target.value)} />}
          </Field>
        </div>
        <RolePicker roles={roles} value={role} onChange={setRole} />
        <PasswordField label="Starting password (at least 10 characters)" value={password} onChange={setPassword} suggest />
        {fault && <p className="error" role="alert">{fault}</p>}
        <div className="form-actions admin-actions">
          <button type="button" className="secondary-button" onClick={onClose} disabled={busy}>Cancel</button>
          <button type="submit" className="action" disabled={busy}>{busy ? 'Adding…' : 'Add user'}</button>
        </div>
      </form>
    </Modal>
  )
}

// ManageUser is the popup for one user: change the role, reset the password, unlock, and
// disable (after a confirmation) or enable. A Partner's own account offers neither a role
// change nor disabling: another Partner must do those (the server refuses them too).
function ManageUser({ user, me, roles, busy, notice, act, onClose }: {
  user: Account
  me: Account
  roles: string[]
  busy: boolean
  notice: string | null
  act: Act
  onClose: () => void
}) {
  const self = user.id === me.id
  const [role, setRole] = useState(user.role)
  const [roleReason, setRoleReason] = useState('')
  const [password, setPassword] = useState('')
  const [activeReason, setActiveReason] = useState('')
  const [confirming, setConfirming] = useState(false)
  const [fault, setFault] = useState<string | null>(null)
  const s = status(user)

  const run = async (change: () => Promise<unknown>, done: string, after: () => void) => {
    setFault(null)
    const p = await act(change, done)
    setFault(p)
    if (p === null) after()
  }

  return (
    <Modal title={`Manage ${user.login_name}`} onClose={onClose} closeDisabled={busy}>
      <div className="admin-drawer admin-manage">
        <div className="admin-profile">
          <Avatar u={user} />
          <div>
            <p className="admin-name">{user.display_name}</p>
            <p className="mono admin-login">{user.login_name}</p>
          </div>
          <span className="chip">{roleName(user.role)}</span>
          <span className={`tag ${s.tone}`}>{s.text}</span>
        </div>
        <p className="muted admin-meta">Last sign-in: {user.last_login_at ? when(user.last_login_at) : 'never'}</p>
        {notice && <p className="notice" role="status">{notice}</p>}
        {fault && <p className="error" role="alert">{fault}</p>}
        {self && (
          <p className="notice admin-self" role="note">
            This is your own account. You cannot change your own role or disable yourself; another Partner must do it.
          </p>
        )}

        {!self && (
          <form className="admin-form admin-block" aria-label="Change role" onSubmit={(e) => {
            e.preventDefault()
            run(() => adminSetRole(user.id, role, roleReason), `Role of ${user.login_name} changed to ${roleName(role)}.`, () => setRoleReason(''))
          }}>
            <h3>Role</h3>
            <RolePicker roles={roles} value={role} onChange={setRole} disabled={busy} />
            <Field label="Reason for the change" required>
              {(id) => <input id={id} type="text" maxLength={300} required value={roleReason} onChange={(e) => setRoleReason(e.target.value)} />}
            </Field>
            <button type="submit" disabled={busy || role === user.role || roleReason.trim() === ''}>Change role</button>
          </form>
        )}

        <form className="admin-form admin-block" aria-label="Reset password" onSubmit={(e) => {
          e.preventDefault()
          run(() => adminResetPassword(user.id, password),
            `A new starting password was set for ${user.login_name}. They must change it at their next sign-in.`, () => setPassword(''))
        }}>
          <h3>Password</h3>
          <PasswordField label="New starting password (at least 10 characters)" value={password} onChange={setPassword}
            help="The user must choose their own password at the next sign-in." />
          <button type="submit" disabled={busy || password === ''}>Reset password</button>
        </form>

        {user.locked && (
          <div className="admin-block">
            <h3>Locked</h3>
            <p className="muted">
              Locked after {plural(user.failed_logins, 'failed sign-in', 'failed sign-ins')}{user.locked_until ? `, until ${when(user.locked_until)}` : ''}.
            </p>
            <button type="button" disabled={busy} onClick={() => run(() => adminUnlock(user.id), `${user.login_name} is unlocked.`, () => undefined)}>
              Unlock {user.login_name}
            </button>
          </div>
        )}

        {!self && (
          <form className="admin-form admin-block" aria-label={user.active ? 'Disable user' : 'Enable user'} onSubmit={(e) => {
            e.preventDefault()
            if (user.active && !confirming) {
              setConfirming(true)
              return
            }
            run(() => adminSetActive(user.id, !user.active, activeReason),
              user.active ? `${user.login_name} is disabled and signed out.` : `${user.login_name} is enabled again.`,
              () => {
                setActiveReason('')
                setConfirming(false)
              })
          }}>
            <h3>{user.active ? 'Disable' : 'Enable'}</h3>
            <Field label={user.active ? 'Reason for disabling' : 'Reason for enabling'} required>
              {(id) => <input id={id} type="text" maxLength={300} required value={activeReason}
                onChange={(e) => setActiveReason(e.target.value)} readOnly={confirming} />}
            </Field>
            {confirming ? (
              <div className="admin-confirm" role="group" aria-label="Confirm disabling">
                <p>Disable {user.login_name}? They are signed out at once and cannot sign in until a Partner enables the account again.</p>
                <button type="submit" className="danger" disabled={busy}>Yes, disable {user.login_name}</button>
                <button type="button" className="secondary-button" disabled={busy} onClick={() => setConfirming(false)}>Keep active</button>
              </div>
            ) : (
              <button type="submit" className={user.active ? 'danger' : undefined} disabled={busy || activeReason.trim() === ''}>
                {user.active ? 'Disable user' : 'Enable user'}
              </button>
            )}
          </form>
        )}
      </div>
    </Modal>
  )
}

// A whole number typed into a box, or null when it is not one within the range.
function wholeIn(text: string, r: { min: number; max: number }): number | null {
  if (!/^\d{1,5}$/.test(text.trim())) return null
  const n = Number(text.trim())
  return n >= r.min && n <= r.max ? n : null
}

function SecurityPanel({ lockout, busy, onSave }: { lockout: Lockout; busy: boolean; onSave: (l: Lockout) => Promise<string | null> }) {
  const [failed, setFailed] = useState(String(lockout.max_failed_logins))
  const [minutes, setMinutes] = useState(String(lockout.lock_minutes))
  const [fault, setFault] = useState<string | null>(null)
  const f = wholeIn(failed, failedRange)
  const m = wholeIn(minutes, minutesRange)
  const changed = f !== lockout.max_failed_logins || m !== lockout.lock_minutes
  const valid = f !== null && m !== null
  return (
    <div className="admin-security">
      <form className="admin-card admin-form" aria-label="Lockout settings" onSubmit={async (e) => {
        e.preventDefault()
        if (f === null || m === null || !changed || busy) return
        setFault(null)
        setFault(await onSave({ max_failed_logins: f, lock_minutes: m }))
      }}>
        <h2>Lockout</h2>
        <div className="admin-grid">
          <Field label={`Failed sign-ins before the lock (${failedRange.min} to ${failedRange.max})`} required
            problem={f === null ? `Enter a whole number from ${failedRange.min} to ${failedRange.max}.` : undefined}>
            {(id) => <input id={id} type="number" inputMode="numeric" min={failedRange.min} max={failedRange.max} required
              aria-invalid={f === null} value={failed} onChange={(e) => setFailed(e.target.value)} />}
          </Field>
          <Field label={`Minutes locked (${minutesRange.min} to ${minutesRange.max})`} required
            problem={m === null ? `Enter a whole number from ${minutesRange.min} to ${minutesRange.max}.` : undefined}>
            {(id) => <input id={id} type="number" inputMode="numeric" min={minutesRange.min} max={minutesRange.max} required
              aria-invalid={m === null} value={minutes} onChange={(e) => setMinutes(e.target.value)} />}
          </Field>
        </div>
        <p className="admin-readback" aria-live="polite">
          {valid
            ? `An account locks after ${plural(f, 'failed sign-in', 'failed sign-ins')} and stays locked for ${plural(m, 'minute', 'minutes')}, or until a Partner unlocks it.`
            : 'Enter numbers within the ranges shown.'}
          {valid && changed && <span className="muted"> Not saved yet.</span>}
        </p>
        {fault && <p className="error" role="alert">{fault}</p>}
        <div className="admin-actions">
          <button type="submit" className="action" disabled={busy || !valid || !changed}>{busy ? 'Saving…' : 'Save lockout'}</button>
        </div>
      </form>
      <section className="admin-card" aria-labelledby="protect-title">
        <h2 id="protect-title">How sign-in is protected</h2>
        <ul className="admin-protect">
          <li>Passwords are kept only as argon2id hashes; nobody, a Partner included, can read one.</li>
          <li>A password has at least 10 characters.</li>
          <li>A new user, and a user whose password a Partner resets, must choose their own password at the next sign-in.</li>
          <li>A session ends after 30 minutes without use, and after 10 hours in any case.</li>
          <li>Disabling a user signs them out at once.</li>
          <li>The server writes every sign-in and every change on this tab to the audit log, each entry linked to the one before it.</li>
        </ul>
      </section>
    </div>
  )
}

function matches(e: AuditEntry, q: string): boolean {
  if (q === '') return true
  const text = [e.actor, e.action, e.object_type, e.object_id, when(e.at), ...Object.entries(e.details).map(([k, v]) => `${k}: ${v}`)]
  return text.some((t) => t.toLowerCase().includes(q))
}

function AuditPanel({ audit }: { audit: AuditList }) {
  const [query, setQuery] = useState('')
  const [who, setWho] = useState('')
  const [action, setAction] = useState('')
  const [page, setPage] = useState(0)
  const newestFirst = useMemo(() => [...audit.entries].sort((a, b) => b.seq - a.seq), [audit.entries])
  const actors = useMemo(() => [...new Set(audit.entries.map((e) => e.actor))].sort(), [audit.entries])
  const actions = useMemo(() => [...new Set(audit.entries.map((e) => e.action))].sort(), [audit.entries])
  const q = query.trim().toLowerCase()
  const shown = newestFirst.filter((e) => (who === '' || e.actor === who) && (action === '' || e.action === action) && matches(e, q))
  const pages = Math.max(1, Math.ceil(shown.length / auditPageSize))
  const current = Math.min(page, pages - 1)
  const rows = shown.slice(current * auditPageSize, (current + 1) * auditPageSize)
  const filtering = q !== '' || who !== '' || action !== ''
  // Any change of what is searched for starts again at the first page.
  const refine = (set: (v: string) => void) => (v: string) => {
    set(v)
    setPage(0)
  }

  return (
    <>
      <div className={audit.chain.ok ? 'audit-banner pass' : 'audit-banner fail'} role="status">
        {audit.chain.ok ? <CheckIcon /> : <CloseIcon />}
        <p>
          <strong>{audit.chain.ok ? 'The audit chain is intact.' : 'The audit chain is broken.'}</strong>{' '}
          <span>{audit.chain.message}. {plural(audit.total, 'entry', 'entries')} in the log.</span>
          {audit.entries.length < audit.total && <span> The newest {audit.entries.length} are listed here.</span>}
        </p>
      </div>

      <div className="board-controls admin-audit-controls">
        <div className="search-box">
          <SearchIcon />
          <input type="search" aria-label="Search the audit log" placeholder="Who, action, object or detail" value={query}
            onChange={(e) => refine(setQuery)(e.target.value)} />
        </div>
        <label className="admin-filter">
          Who
          <select value={who} onChange={(e) => refine(setWho)(e.target.value)}>
            <option value="">Everyone</option>
            {actors.map((a) => (
              <option key={a} value={a}>{a}</option>
            ))}
          </select>
        </label>
        <label className="admin-filter">
          Action
          <select value={action} onChange={(e) => refine(setAction)(e.target.value)}>
            <option value="">All actions</option>
            {actions.map((a) => (
              <option key={a} value={a}>{a}</option>
            ))}
          </select>
        </label>
        <span className="result-count" aria-live="polite">
          {filtering ? `${shown.length} of ${plural(newestFirst.length, 'entry', 'entries')}` : plural(newestFirst.length, 'entry', 'entries')}, newest first
        </span>
      </div>

      {shown.length === 0 ? (
        <p className="empty">{filtering ? 'No entry matches. Clear the search or the filters to see every entry.' : 'The audit log has no entries yet.'}</p>
      ) : (
        <>
          <div className="table-wrap">
            <table className="table-premium admin-audit" aria-label="Audit log">
              <thead>
                <tr>
                  <th scope="col">When</th>
                  <th scope="col">Who</th>
                  <th scope="col">Action</th>
                  <th scope="col">Object</th>
                  <th scope="col">Details</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((e) => (
                  <tr key={e.seq}>
                    <td className="admin-when">{when(e.at)}</td>
                    <td className="mono">{e.actor}</td>
                    <td><span className={`tag ${actionTones[e.action] ?? 'muted'}`}>{e.action}</span></td>
                    <td>{e.object_type ? `${e.object_type} ${e.object_id}` : e.object_id}</td>
                    <td>{Object.entries(e.details).map(([k, v]) => `${k}: ${v}`).join('; ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {pages > 1 && (
            <div className="pager admin-pager" role="group" aria-label="Pages of the audit log">
              <span className="pager-range" aria-live="polite">
                {current * auditPageSize + 1}–{Math.min((current + 1) * auditPageSize, shown.length)} of {shown.length}
              </span>
              <span className="pager-steps">
                <button type="button" onClick={() => setPage(current - 1)} disabled={current === 0}>Previous</button>
                <span>Page {current + 1} of {pages}</span>
                <button type="button" onClick={() => setPage(current + 1)} disabled={current >= pages - 1}>Next</button>
              </span>
            </div>
          )}
        </>
      )}
    </>
  )
}
