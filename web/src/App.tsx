import { useEffect, useState } from 'react'

// Phase 1 placeholder: proves the Vite dev server reaches the Go server
// through the /api proxy. Replaced by the real UI from Phase 4 on.
export default function App() {
  const [ping, setPing] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetch('/api/ping')
      .then((res) => (res.ok ? res.json() : Promise.reject(new Error(`HTTP ${res.status}`))))
      .then((data: unknown) => setPing(JSON.stringify(data)))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)))
  }, [])

  return (
    <main style={{ fontFamily: 'system-ui, sans-serif', padding: '2rem' }}>
      <h1>Side Project Saviour</h1>
      <p>Phase 1 scaffold — server ping: {ping ?? error ?? '…'}</p>
    </main>
  )
}
