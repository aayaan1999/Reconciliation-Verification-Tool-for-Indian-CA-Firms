import '../styles/auth.css'
import { useEffect, useId, useState, type FormEvent, type KeyboardEvent, type ReactNode } from 'react'
import { ApiError, fetchSetupNeeded, setUpFirstPartner, signIn } from '../api.ts'
import type { Session } from '../types.ts'
import { formatPaise } from '../format.ts'
import { BookIcon, DocIcon, LedgerIcon, ListIcon } from '../icons.tsx'

// The server refuses a password shorter than this (internal/auth MinPasswordLength), so the
// form says so before sending it.
export const minPasswordLength = 10

// The example on the brand panel (Recon Screen Document v2 §4.2): invented figures, shown as
// the arithmetic a finding carries. Amounts are whole paise.
const example = { computation: 13831000, schedule: 13586200 }

const OfflineIcon = () => (
  <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5"
    strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false">
    <path d="M2 2l12 12M1.5 6a9 9 0 0 1 3.4-2M7 3.1A9 9 0 0 1 14.5 6M4 8.6a5 5 0 0 1 2.4-1.5M9.6 7.2A5 5 0 0 1 12 8.6M6.4 11.1a2 2 0 0 1 3.2 0" />
  </svg>
)

const promises: { icon: ReactNode; strong: string; rest: string }[] = [
  { icon: <OfflineIcon />, strong: 'Works fully offline', rest: 'on the office machine' },
  { icon: <BookIcon />, strong: 'Rule-driven checks', rest: 'the CA can read and approve' },
  { icon: <DocIcon />, strong: 'GST, income and deduction', rest: 'tie-outs in one run' },
  { icon: <ListIcon />, strong: 'Every decision kept', rest: 'in a chained audit trail' },
]

// AuthLayout is the frame of the screens shown before the tool opens (sign-in, first Partner,
// change of a starting password): the navy brand panel on the left, the form on the right.
// Below 900px the panel shortens to the brand and the headline.
export function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="auth">
      <aside className="auth-hero" aria-label="About the tool">
        <div className="auth-brand">
          <span className="auth-logo" aria-hidden="true">
            <LedgerIcon size={20} />
          </span>
          <div>
            <p className="auth-name">Recon</p>
            <p className="auth-for">Reconciliation &amp; Verification Tool for CA firms</p>
          </div>
        </div>
        <div className="auth-pitch">
          <p className="auth-headline">Every place two documents disagree, to the rupee.</p>
          <p className="auth-lead">
            Reads what Tally, the tax suite, the GST portal and TRACES produce, compares the same figure wherever
            it appears, and shows you both sides.
          </p>
          <figure className="auth-example" aria-label="Example finding">
            <figcaption>
              <span className="auth-example-id">RECON-DEP-001</span> Depreciation in the computation and its schedule
              <span className="auth-example-tag">Example</span>
            </figcaption>
            <div className="auth-equation">
              <div>
                <span>Computation</span>
                <b>₹{formatPaise(example.computation)}</b>
              </div>
              <span className="auth-op">−</span>
              <div>
                <span>Fixed-asset schedule</span>
                <b>₹{formatPaise(example.schedule)}</b>
              </div>
              <span className="auth-op">=</span>
              <div className="auth-difference">
                <span>Difference</span>
                <b>₹{formatPaise(example.computation - example.schedule)}</b>
              </div>
            </div>
          </figure>
          <ul className="auth-promises">
            {promises.map((p) => (
              <li key={p.strong}>
                <span className="auth-promise-icon" aria-hidden="true">{p.icon}</span>
                <span><strong>{p.strong}</strong> {p.rest}</span>
              </li>
            ))}
          </ul>
        </div>
        <p className="auth-foot">Nothing is sent to the internet. Findings come only from rules, never from AI.</p>
      </aside>
      <main className="auth-main">{children}</main>
    </div>
  )
}

// passwordStrength rates a new password by its length and the kinds of character in it. The
// server asks only for the length; the meter helps the user choose a longer one.
function passwordStrength(password: string): { score: 0 | 1 | 2 | 3 | 4; text: string } {
  const length = [...password].length
  if (length < minPasswordLength) return { score: 0, text: `Too short: at least ${minPasswordLength} characters` }
  const kinds = [/[a-z]/, /[A-Z]/, /[0-9]/, /[^A-Za-z0-9]/].filter((k) => k.test(password)).length
  const score = Math.min(4, 1 + (length >= 14 ? 1 : 0) + (length >= 18 ? 1 : 0) + (kinds >= 3 ? 1 : 0)) as 1 | 2 | 3 | 4
  return { score, text: ['', 'Fair', 'Good', 'Strong', 'Very strong'][score] }
}

function StrengthMeter({ password }: { password: string }) {
  const s = passwordStrength(password)
  return (
    <div className={`auth-meter s${s.score}`} role="meter" aria-label="Password strength"
      aria-valuemin={0} aria-valuemax={4} aria-valuenow={s.score} aria-valuetext={s.text}>
      <span className="auth-meter-bars" aria-hidden="true">
        {[1, 2, 3, 4].map((n) => <i key={n} className={n <= s.score ? 'on' : undefined} />)}
      </span>
      <span className="auth-meter-text">{s.text}</span>
    </div>
  )
}

// PasswordField is a masked password box with a Show button, a Caps Lock warning and, for a new
// password, the strength meter. Its label names the input, so a screen reader reads it.
export function PasswordField({ label, value, onChange, autoComplete, meter = false, required = false, hint, invalid, autoFocus }: {
  label: string
  value: string
  onChange: (v: string) => void
  autoComplete: 'current-password' | 'new-password'
  meter?: boolean
  required?: boolean
  hint?: string
  invalid?: string // the id of the message that explains why the field is refused
  autoFocus?: boolean
}) {
  const id = useId()
  const [shown, setShown] = useState(false)
  const [caps, setCaps] = useState(false)
  const readCaps = (e: KeyboardEvent<HTMLInputElement>) => setCaps(e.getModifierState('CapsLock'))
  const described = [hint && `${id}-hint`, caps && `${id}-caps`, invalid].filter(Boolean).join(' ')
  return (
    <div className="auth-field">
      <label htmlFor={id} className={required ? 'auth-required' : undefined}>{label}</label>
      <div className="auth-password">
        <input
          id={id}
          type={shown ? 'text' : 'password'}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={readCaps}
          onKeyUp={readCaps}
          onBlur={() => setCaps(false)}
          autoComplete={autoComplete}
          spellCheck={false}
          required
          autoFocus={autoFocus}
          aria-invalid={invalid ? true : undefined}
          aria-describedby={described || undefined}
        />
        <button type="button" className="auth-reveal" aria-pressed={shown} onClick={() => setShown((s) => !s)}
          aria-label={`${shown ? 'Hide' : 'Show'} ${label.toLowerCase()}`}>
          {shown ? 'Hide' : 'Show'}
        </button>
      </div>
      {caps && <p id={`${id}-caps`} className="auth-caps" role="status">Caps Lock is on.</p>}
      {meter && <StrengthMeter password={value} />}
      {hint && <p id={`${id}-hint`} className="auth-hint">{hint}</p>}
    </div>
  )
}

// Sign-in screen (SPEC-012, SPEC-014; Recon Screen Document v2 §4.1, §4.2). On an empty
// database the first Partner is set up here once; afterwards it signs users in. A refusal
// shows the server's message as written.
export function Login({ onSignedIn }: { onSignedIn: (session: Session) => void }) {
  const ids = { login: useId(), name: useId(), error: useId() }
  const [setupNeeded, setSetupNeeded] = useState<boolean | null>(null)
  const [loginName, setLoginName] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [repeat, setRepeat] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    fetchSetupNeeded().then((needed) => {
      if (!cancelled) setSetupNeeded(needed === true)
    })
    return () => {
      cancelled = true
    }
  }, [])

  const setup = setupNeeded === true

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (busy) return
    setError(null)
    setNotice(null)
    if (setup && [...password].length < minPasswordLength) {
      setError(`The password needs at least ${minPasswordLength} characters.`)
      return
    }
    if (setup && password !== repeat) {
      setError('The two passwords are not the same. Type the password again.')
      return
    }
    setBusy(true)
    try {
      if (setup) {
        await setUpFirstPartner(loginName, displayName, password)
        setSetupNeeded(false)
        setNotice('The first Partner is set up. Sign in with the password you chose.')
      } else {
        onSignedIn(await signIn(loginName, password))
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err))
    } finally {
      setPassword('')
      setRepeat('')
      setBusy(false)
    }
  }

  // A server refusal marks the fields as invalid and points them at the message.
  const invalid = error ? { 'aria-invalid': true as const, 'aria-describedby': ids.error } : {}
  const titleId = setup ? 'setup-title' : 'signin-title'

  return (
    <AuthLayout>
      <form className="auth-form" onSubmit={submit} aria-labelledby={titleId} aria-busy={busy}>
        {setup ? (
          <>
            <p className="auth-badge">First start · shown once</p>
            <h1 id={titleId}>Set up the first Partner</h1>
            <p className="auth-sub">
              No one has used Recon on this machine yet. The first Partner can add every other user from the Admin tab.
            </p>
          </>
        ) : (
          <>
            <h1 id={titleId}>Sign in</h1>
            <p className="auth-sub">Enter your login name and password to continue.</p>
          </>
        )}

        {!setup && notice && (
          <p className="auth-alert success" role="status">{notice}</p>
        )}

        <div className={setup ? 'auth-pair' : undefined}>
          <div className="auth-field">
            <label htmlFor={ids.login} className={setup ? 'auth-required' : undefined}>Login name</label>
            <input
              id={ids.login}
              type="text"
              value={loginName}
              onChange={(e) => setLoginName(e.target.value)}
              autoComplete="username"
              autoCapitalize="none"
              spellCheck={false}
              required
              autoFocus
              {...invalid}
            />
          </div>
          {setup && (
            <div className="auth-field">
              <label htmlFor={ids.name} className="auth-required">Display name</label>
              <input
                id={ids.name}
                type="text"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                maxLength={100}
                required
              />
            </div>
          )}
        </div>

        <PasswordField
          label="Password"
          value={password}
          onChange={setPassword}
          autoComplete={setup ? 'new-password' : 'current-password'}
          meter={setup}
          required={setup}
          hint={setup ? undefined : 'New users sign in with the starting password from the Partner and choose their own.'}
          invalid={error ? ids.error : undefined}
        />
        {setup && (
          <PasswordField label="Type the password again" value={repeat} onChange={setRepeat} autoComplete="new-password" required />
        )}

        {error && (
          <p id={ids.error} className="auth-alert error" role="alert">{error}</p>
        )}

        <button type="submit" className="auth-submit" disabled={busy || setupNeeded === null}>
          {setup ? (busy ? 'Creating…' : 'Create Partner account') : busy ? 'Signing in…' : 'Sign in'}
        </button>
      </form>
    </AuthLayout>
  )
}
