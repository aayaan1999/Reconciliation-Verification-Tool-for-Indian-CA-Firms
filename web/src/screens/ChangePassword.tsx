import { useId, useState, type FormEvent } from 'react'
import { ApiError, changeMyPassword, signOut } from '../api.ts'
import type { Session } from '../types.ts'
import { AuthLayout, PasswordField, minPasswordLength } from './Login.tsx'

// The change-password screen (SPEC-012 §2). A user who signed in with a starting password sees
// only this screen until the password is changed. The current password is required, so the
// starting password stops working once changed.
export function ChangePassword({ session, onChanged, onSignedOut }: {
  session: Session
  onChanged: (session: Session) => void
  onSignedOut: () => void
}) {
  const errorId = useId()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [repeat, setRepeat] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (busy) return
    setError(null)
    if ([...next].length < minPasswordLength) {
      setError(`The new password needs at least ${minPasswordLength} characters.`)
      return
    }
    if (next !== repeat) {
      setError('The two new passwords are not the same. Type the new password again.')
      return
    }
    setBusy(true)
    try {
      onChanged(await changeMyPassword(current, next))
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err))
    } finally {
      setBusy(false)
      setCurrent('')
      setNext('')
      setRepeat('')
    }
  }

  const leave = async () => {
    try {
      await signOut()
    } catch {
      // Already signed out: the screen goes back to the sign-in either way.
    }
    onSignedOut()
  }

  return (
    <AuthLayout>
      <form className="auth-form" onSubmit={submit} aria-labelledby="change-title" aria-busy={busy}>
        <h1 id="change-title">Choose your own password</h1>
        <p className="auth-sub">
          {session.user.display_name}, you signed in with a starting password. Choose a password of your own (at
          least {minPasswordLength} characters) to continue.
        </p>

        <PasswordField label="Current (starting) password" value={current} onChange={setCurrent} autoComplete="current-password" required autoFocus />
        <PasswordField label="New password" value={next} onChange={setNext} autoComplete="new-password" meter required
          invalid={error ? errorId : undefined} />
        <PasswordField label="New password again" value={repeat} onChange={setRepeat} autoComplete="new-password" required />

        {error && <p id={errorId} className="auth-alert error" role="alert">{error}</p>}

        <button type="submit" className="auth-submit" disabled={busy}>{busy ? 'Changing…' : 'Change password'}</button>
        <button type="button" className="ghost auth-secondary" onClick={leave}>Sign out</button>
      </form>
    </AuthLayout>
  )
}
