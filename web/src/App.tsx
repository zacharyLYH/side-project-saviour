import { useEffect, useState, type FormEvent } from 'react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

// Phase 4: the only UI is auth — logged out, logging in, logged in. The
// logged-in view keeps the Phase 1 ping placeholder as the proxy proof.
type AuthState = 'loading' | 'out' | 'in'

export default function App() {
  const [state, setState] = useState<AuthState>('loading')
  const [email, setEmail] = useState('')

  useEffect(() => {
    fetch('/api/auth/me')
      .then((res) => (res.ok ? res.json() : Promise.reject(new Error(`HTTP ${res.status}`))))
      .then((data: { email: string }) => {
        setEmail(data.email)
        setState('in')
      })
      .catch(() => setState('out'))
  }, [])

  if (state === 'loading') {
    return (
      <main className="grid min-h-dvh place-items-center">
        <p className="text-muted-foreground text-sm">Loading…</p>
      </main>
    )
  }
  if (state === 'out') {
    return <LoginForm onLoggedIn={(e) => { setEmail(e); setState('in') }} />
  }
  return <LoggedIn email={email} onLogout={() => setState('out')} />
}

function LoginForm({ onLoggedIn }: { onLoggedIn: (email: string) => void }) {
  const [email, setEmail] = useState('')
  const [pin, setPin] = useState('')
  const [step, setStep] = useState<'email' | 'pin'>('email')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function requestPin(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const res = await fetch('/api/auth/request-pin', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email }),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setStep('pin')
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  async function verify(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const res = await fetch('/api/auth/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, pin }),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      onLoggedIn(email)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="bg-muted/40 grid min-h-dvh place-items-center p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Side Project Saviour</CardTitle>
          <CardDescription>Sign in with the PIN sent to your email.</CardDescription>
        </CardHeader>
        <CardContent>
          {step === 'email' ? (
            <form onSubmit={requestPin} className="flex flex-col gap-3">
              <Input
                type="email"
                required
                autoComplete="email"
                placeholder="you@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
              {error && <p className="text-destructive text-sm">{error}</p>}
              <Button type="submit" disabled={busy}>
                Send code
              </Button>
            </form>
          ) : (
            <form onSubmit={verify} className="flex flex-col gap-3">
              <Input
                type="text"
                inputMode="numeric"
                pattern="[0-9]*"
                maxLength={6}
                required
                autoFocus
                placeholder="6-digit PIN"
                value={pin}
                onChange={(e) => setPin(e.target.value)}
              />
              {error && <p className="text-destructive text-sm">{error}</p>}
              <Button type="submit" disabled={busy}>
                Log in
              </Button>
              <Button type="button" variant="ghost" disabled={busy} onClick={() => setStep('email')}>
                Back
              </Button>
            </form>
          )}
        </CardContent>
      </Card>
    </main>
  )
}

function LoggedIn({ email, onLogout }: { email: string; onLogout: () => void }) {
  const [ping, setPing] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetch('/api/ping')
      .then((res) => (res.ok ? res.json() : Promise.reject(new Error(`HTTP ${res.status}`))))
      .then((data: unknown) => setPing(JSON.stringify(data)))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)))
  }, [])

  async function logout() {
    try {
      await fetch('/api/auth/logout', { method: 'POST' })
    } catch {
      // still sign out client-side
    }
    onLogout()
  }

  return (
    <main className="mx-auto w-full max-w-md p-4">
      <header className="flex items-center justify-between py-4">
        <h1 className="text-lg font-semibold">Side Project Saviour</h1>
        <Button variant="ghost" size="sm" onClick={logout}>
          Log out
        </Button>
      </header>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Welcome, {email}</CardTitle>
          <CardDescription>Server ping: {ping ?? error ?? '…'}</CardDescription>
        </CardHeader>
      </Card>
    </main>
  )
}
