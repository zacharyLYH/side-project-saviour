import { useEffect, useState, type FormEvent } from 'react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { TerminalView } from '@/components/Terminal'
import { useProjects } from '@/hooks/useProjects'

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
  const { projects, loading, error: projectsError, refresh } = useProjects()
  const [terminal, setTerminal] = useState<string | null>(null)
  const [repoUrl, setRepoUrl] = useState('')
  const [branch, setBranch] = useState('')
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)

  async function createProject(e: FormEvent) {
    e.preventDefault()
    setCreating(true)
    setCreateError(null)
    try {
      const body = repoUrl.trim() ? { repoUrl: repoUrl.trim(), branch: branch.trim() } : {}
      const res = await fetch('/api/projects', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (!res.ok) throw new Error(await res.text())
      setRepoUrl('')
      setBranch('')
      refresh()
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : String(err))
    } finally {
      setCreating(false)
    }
  }

  async function deleteProject(id: string) {
    if (!window.confirm('Delete this project — container and all volumes?')) return
    try {
      await fetch(`/api/projects/${id}?scope=all`, { method: 'DELETE' })
      refresh()
    } catch {
      // leave the row in place; the next refresh shows the truth
    }
  }

  async function logout() {
    try {
      await fetch('/api/auth/logout', { method: 'POST' })
    } catch {
      // still sign out client-side
    }
    onLogout()
  }

  if (terminal) {
    return <TerminalView projectId={terminal} sessionName="main" onBack={() => { setTerminal(null); refresh() }} />
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
        </CardHeader>
      </Card>
      <Card className="mt-4">
        <CardHeader>
          <CardTitle className="text-base">Projects</CardTitle>
          <CardDescription>One terminal per project, session "main".</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          {loading && <p className="text-muted-foreground text-sm">Loading projects…</p>}
          {!loading && projectsError && (
            <p className="text-destructive text-sm">Failed to load projects: {projectsError}</p>
          )}
          {!loading && !projectsError && projects.length === 0 && (
            <p className="text-muted-foreground text-sm">No projects yet.</p>
          )}
          {projects.map((p) => (
            <div key={p.id} className="flex items-center justify-between text-sm">
              <span>{p.name}</span>
              <div className="flex gap-2">
                <Button size="sm" variant="outline" onClick={() => setTerminal(p.id)}>
                  Terminal
                </Button>
                <Button size="sm" variant="ghost" onClick={() => deleteProject(p.id)}>
                  Delete
                </Button>
              </div>
            </div>
          ))}

          <form onSubmit={createProject} className="mt-2 flex flex-col gap-2 border-t pt-3">
            <Input
              type="text"
              placeholder="Repo URL (optional — blank = plain sandbox)"
              value={repoUrl}
              onChange={(e) => setRepoUrl(e.target.value)}
            />
            <Input
              type="text"
              placeholder="Branch (optional)"
              value={branch}
              onChange={(e) => setBranch(e.target.value)}
            />
            {createError && <p className="text-destructive max-h-24 overflow-auto break-all text-xs">{createError}</p>}
            <Button type="submit" disabled={creating}>
              {creating ? 'Creating…' : 'Create project'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </main>
  )
}
