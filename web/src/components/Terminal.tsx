import { useEffect, useRef, useState } from 'react'
import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'

// A dark theme that feels like a real terminal, not a default palette.
const TERM_THEME = {
  background: '#0a0e14',
  foreground: '#b3b1ad',
  cursor: '#e6b450',
  cursorAccent: '#0a0e14',
  selectionBackground: '#273747',
  selectionForeground: '#e6e4d9',
  black: '#01060e',
  red: '#ea6c73',
  green: '#91b362',
  yellow: '#f9af4f',
  blue: '#53bdfa',
  magenta: '#fae994',
  cyan: '#90e1c6',
  white: '#c7c7c7',
  brightBlack: '#686868',
  brightRed: '#f07178',
  brightGreen: '#c2d94c',
  brightYellow: '#ffb454',
  brightBlue: '#59c2ff',
  brightMagenta: '#ffee99',
  brightCyan: '#95e6cb',
  brightWhite: '#ffffff',
}

// TerminalView bridges one xterm.js instance to
// /ws/projects/{id}/sessions/{name}. Frames: input/resize out, output/exit
// in — the server owns the protocol (internal/httpapi/terminal.go).
export function TerminalView({ projectId, sessionName, onBack }: {
  projectId: string
  sessionName: string
  onBack: () => void
}) {
  const hostRef = useRef<HTMLDivElement>(null)
  const [status, setStatus] = useState<'connecting' | 'live' | 'ended'>('connecting')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let disposed = false
    let opened = false
    let ws: WebSocket | null = null

    const term = new Terminal({
      cursorBlink: true,
      cursorStyle: 'bar',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, "Cascadia Code", "Fira Code", monospace',
      fontSize: 14,
      lineHeight: 1.3,
      letterSpacing: 0.3,
      theme: TERM_THEME,
      allowProposedApi: true,
    })
    const fit = new FitAddon()
    term.loadAddon(fit)

    const observer = new ResizeObserver(() => {
      if (!opened || !ws) return
      fit.fit()
      sendResize(ws, fit)
    })

    // The server's session create doubles as an ensure call: 201 = created,
    // 200 = already existed — both mean we may attach.
    async function ensureSessionThenDial() {
      const res = await fetch(`/api/projects/${projectId}/sessions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: sessionName }),
      })
      if (disposed) return
      if (!res.ok) {
        const detail = await res.text()
        setStatus('ended')
        setError(`session create failed (${res.status}): ${detail}`)
        return
      }

      const proto = location.protocol === 'https:' ? 'wss' : 'ws'
      ws = new WebSocket(`${proto}://${location.host}/ws/projects/${projectId}/sessions/${sessionName}`)

      ws.onopen = () => {
        if (disposed || !ws) return
        opened = true
        term.open(hostRef.current!)
        fit.fit()
        term.focus()
        sendResize(ws, fit)
        setStatus('live')
      }
      ws.onmessage = (ev) => {
        if (disposed) return
        const frame = JSON.parse(ev.data as string) as { type: string; data?: string; code?: number }
        if (frame.type === 'output') term.write(frame.data ?? '')
        if (frame.type === 'exit') {
          setStatus('ended')
          term.write(`\r\n\x1b[38;2;234;108;115m[session ended${frame.code ? ` (code ${frame.code})` : ''}]\x1b[0m\r\n`)
          ws!.close()
        }
      }
      ws.onclose = () => { if (!disposed) setStatus('ended') }
      ws.onerror = () => { if (!disposed) setStatus('ended') }

      term.onData((data) => {
        if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'input', data }))
      })
    }
    void ensureSessionThenDial()

    observer.observe(hostRef.current!)

    return () => {
      disposed = true
      observer.disconnect()
      ws?.close()
      term.dispose()
    }
  }, [projectId, sessionName])

  return (
    <div className="flex min-h-dvh flex-col bg-[#0a0e14]">
      <header className="flex items-center gap-3 px-4 py-2">
        <Button
          variant="ghost"
          size="sm"
          className="text-white/50 hover:text-white/90 hover:bg-white/[0.06]"
          onClick={onBack}
        >
          ← Projects
        </Button>
        <Separator orientation="vertical" className="h-4 bg-white/10" />
        <span className="font-mono text-xs tracking-wide text-white/40">
          {projectId}<span className="mx-1.5 text-white/20">·</span>{sessionName}
        </span>
        <div className="ml-auto">
          <StatusBadge status={status} />
        </div>
      </header>
      {error && (
        <Badge variant="destructive" className="mx-4 mb-2 w-fit text-xs font-normal">
          {error}
        </Badge>
      )}
      <div ref={hostRef} className="min-h-0 flex-1 p-1" />
    </div>
  )
}

function StatusBadge({ status }: { status: 'connecting' | 'live' | 'ended' }) {
  const variant = status === 'ended' ? 'secondary' : status === 'live' ? 'default' : 'outline'
  const label = status === 'live' ? 'Connected' : status === 'connecting' ? 'Connecting…' : 'Disconnected'
  return (
    <Badge variant={variant} className="gap-1.5 font-mono text-[10px] uppercase tracking-wider">
      <StatusDot status={status} />
      {label}
    </Badge>
  )
}

function StatusDot({ status }: { status: 'connecting' | 'live' | 'ended' }) {
  const color =
    status === 'live'
      ? 'bg-emerald-400 shadow-[0_0_6px_rgba(52,211,153,0.5)]'
      : status === 'connecting'
        ? 'bg-amber-400 animate-pulse'
        : 'bg-white/20'
  return <span className={`inline-block size-1.5 rounded-full ${color}`} />
}

function sendResize(ws: WebSocket, fit: FitAddon) {
  if (ws.readyState !== WebSocket.OPEN) return
  const dims = fit.proposeDimensions()
  if (dims && dims.rows > 0 && dims.cols > 0) {
    ws.send(JSON.stringify({ type: 'resize', rows: dims.rows, cols: dims.cols }))
  }
}
