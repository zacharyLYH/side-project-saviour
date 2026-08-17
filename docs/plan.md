# Implementation Plan — Waterfall Work Packages

> Source of truth: `docs/prd.md`. Each phase below is a gated, medium-sized deliverable. Each ends with a demo and tests that pass on a laptop with only Docker installed. No phase starts until the previous phase's gate passes.

## How we keep everything locally testable

- The server runs on the host in dev (`make dev`) against the host Docker engine. Project containers are real containers on that same engine — siblings, not nested. The prod "server image ships project images" path (mounted socket) is exercised by `make e2e`, which runs the *same* flow through compose.
- Tests drive real Docker, real tmux, real git — wrapped in thin internal packages, never mocked at the boundary. Parsing/logic unit tests need no Docker.
- `test/fixtures/` ships tiny Dockerfiles (alpine+node), a sample "hello" repo, and fake CLIs so harness flows don't need real network installs.
- Email in dev prints the PIN to the server log (a `Mailer` interface; SMTP is a prod-only implementation). No SMTP needed on a laptop.
- Every phase adds a `make check` step next to `make dev` / `make test`.

## Phases at a glance

| # | Phase | Core deliverable | Demonstrable with |
|---|---|---|---|
| 1 | Foundations & control loop | repo layout, config, health, dev compose, Vite scaffold | `curl /health` |
| 2 | Storage & registry primitives | data dir, project.json CRUD, harness plugin loader, seeding | unit tests + `make dev` boot |
| 3 | Docker control plane | thin go-dockerclient wrapper (build/run/exec/resize/cp/logs/net) | integration test spins throwaway container |
| 4 | System authentication | email + PIN → JWT, middleware on all routes/WS | login prints PIN to logs |
| 5 | Project create pipeline | clone → Dockerfile check/scaffold → build → run → ready | fixture repo → "Ready" via curl |
| 6 | Terminal transport | WS ↔ tmux via exec hijack + resize + reconnect | typed into a browser terminal |
| 7 | Harnesses & sessions | session CRUD, CLI validation, builtins, launch/restart | fake CLI harness session |
| 8 | Preview & diff | port detect + reverse proxy + ports panel; git diff API+UI | http.server in container → URL |
| 9 | Env editor & persistence | masked env, login-shell injection, volume survival | `$VAR` after restart |
| 10 | Frontend completion & buttons | full UI, action buttons, creature-comfort keys, mobile | litmus test on a phone |
| 11 | Packaging & self-hosting | server image, compose, quickstart, minimal admin | fresh box via `docker compose up` |
| 12 | The secretary (v1) | chat, system guide, tools, confirm-diff, wire-key/switch-model | the 4 v1 abilities on a fixture |

---

## Phase 1 — Foundations & control loop

**Goal.** A bo-repo that builds, boots, and answers health. Nothing more.

**Scope.**
- Repo layout: `cmd/server`, `internal/{config,auth,docker,project,session,terminal,preview,diff,harness}`, `web/`, `test/fixtures`.
- Config via env/flags: `DATA_DIR`, `BIND`, `LOGIN_EMAIL`, `JWT_SECRET`, `SMTP_*`, `DOCKER_SOCK`.
- `Go server` skeleton: logging, graceful shutdown, `GET /health` + `GET /api/ping`.
- `internal/config` loads and validates; unknown config fails loudly.
- `Makefile`: `make dev`, `make test`, `make build`, `make check`, `make e2e` (stub).
- `docker-compose.dev.yml`: boots the server and a fake SMTP sink; Vite dev server proxying `/api` and `/ws`.
- `web/` Vite + React + TS scaffold rendering a placeholder page.

**Gate (test locally).** `docker compose -f docker-compose.dev.yml up`; `curl /health` returns 200; `make test` runs the config unit tests; server exits clean on SIGTERM.

---

## Phase 2 — Storage & registry primitives

**Goal.** The data dir is the entire database. Files, not tables.

**Scope.**
- Bootstrap `$DATA_DIR/{projects,harnesses,ssh}` with correct perms on first boot.
- `project.json` CRUD with atomic writes (temp + rename). Fields per PRD §8: `name/repo/branch/actions/env` — file mode `0600`.
- Harness plugin loader: scan `$DATA_DIR/harnesses/*.json`, validate schema (`name/command/install/auth`), parse `auth` block. One code path for builtins and user files.
- Seed builtins (terminal, opencode, freebuff, aider) on first boot as real editable files.
- Projects index (`$DATA_DIR/projects.json`) so the list renders without container scans.
- `weathervane` package for live container state — not needed until Phase 5; add stub.

**Gate.** Unit tests for CRUD atomicity, loader, schema rejection, seeding idempotence (`seeded twice = same files`). `make dev` shows the seeded tree in logs.

---

## Phase 3 — Docker control plane

**Goal.** A thin, boring wrapper over `go-dockerclient`. The server's whole interaction with Docker lives here.

**Scope.**
- Image build with streamed logs (create-project needs this).
- Container create/start/stop/remove/inspect with defaults: non-privileged, read-only rootfs where practical, resource limits, `sps-net` network, named volumes (`sps-<id>-repo`, `sps-<id>-home`).
- Exec: `docker exec` with `-i -t`, attach hijack, and `ResizeExecTTY`.
- Logs tail. File upload (the `docker cp` primitive) for env/harness config writes. Port detection (`ss -tlnp`). SSH-key mount into containers.
- All functions return typed errors; no raw docker API leaks past this package.

**Gate.** Integration test (run via `make docker-test`, tagged `integration`): builds a tiny fixture image, runs it, execs `echo`, uploads a file, inspects it, removes it. Fails fast (not silently) when Docker is unavailable, with a clear message.

---

## Phase 4 — System authentication

**Goal.** The only reason login exists is safe internet exposure. Keep it minimal: email + one-time PIN → JWT.

**Scope.**
- `Mailer` interface with two impls: `ConsoleMailer` (dev: print PIN to log) and `SmtpMailer` (prod, configured via env).
- Flow: request PIN for the configured `LOGIN_EMAIL` → rate-limit and expire PINs → verify → issue signed JWT (crypto/rand, expiry, `HttpOnly; SameSite=Strict` cookie).
- Middleware: every `/api/*` and `/ws/*` route requires a valid JWT (except login/PIN routes). WebSocket handshake validates the cookie too.
- Logged-out and logged-in UI states in the scaffolded frontend.

**Gate.** Unit tests: token signing/expiry, PIN expiry/rate-limit, middleware rejection. Manual: `make dev`, request PIN, see it in logs, complete login, hit authed endpoint. Document: this is not a SaaS identity system — one email, one person.

---

## Phase 5 — Project create pipeline

**Goal.** PRD's 80% path works end-to-end over the API: clone → build → ready.

**Scope.**
- `POST /api/projects` `{repoUrl, branch?}` → clone to `$DATA_DIR/projects/<id>/repo` → Dockerfile check (missing ⇒ message; scaffold a minimal default on request) → `docker build` with logs streamed → `docker run` with volumes/limits/net → container ready.
- `GET /api/projects`, `GET /api/projects/{id}` (state, live status), `POST /{id}/start|stop|restart`, `DELETE /{id}?scope=container|metadata|repo|all` with confirmation.
- Create progress surfaced as `SPS_STEP` events (clone ✓, build ✓, running ✓) consumable by the UI later.
- Pin branch; default to remote default. Untracked scaffold: record that setup/start never auto-ran (Phase 7/10 adds actions).

**Gate.** API smoke tests (`httptest` against a live server + host Docker): create a fixture repo, reach "Running", stop it (volumes survive), restart it, delete each scope. No UI required — proves it with curl.

---

## Phase 6 — Terminal transport

**Goal.** The heart of the product: a browser terminal that behaves like a real terminal.

**Scope.**
- `GET /ws/projects/{id}/sessions/{name}` bridging browser frames ↔ `docker exec -it tmux attach -t <name>`.
- Protocol: `input`, `resize` (→ `ResizeExecTTY`), `output`, `exit`. Reconnect = fresh `tmux attach`; session keeps running on disconnect.
- Minimal frontend terminal: xterm.js + `@xterm/addon-fit` in a temporary tab, resize on frame, desktop-width toggle.
- Creature-comfort keys as a first cut: ↑, Ctrl-C, Ctrl-L, Tab, Esc → send keystroke frame (full strip lands in Phase 10).

**Gate.** Open a project, open a terminal tab, run `echo hi` and a TUI (`top`), resize the window (browser and mobile), disconnect and reconnect → same scrollback. A Go test drives the WS client against a real session.

---

## Phase 7 — Harnesses & sessions

**Goal.** "+ New Session" works: sessions are real tmux sessions, discovered not stored.

**Scope.**
- Sessions API: list (from `tmux list-sessions`), create `{harnessId}`, kill, restart. Naming: `<harness>-<n>`.
- Harness launch: `tmux new-session -d 'cmd || echo "[cmd exited: $?]"'` in the repo dir; expose the exit line.
- CLI validation from PRD §5: after `install`, run `command --version|--help` in the container with a forced TTY + timeout; reject non-CLI with the PRD message.
- Install-on-demand: first launch runs the plugin's `install` in the container if the `command` is missing.
- Builtins documented with runtime requirements (node for opencode/freebuff); missing runtime ⇒ visible failure with a hint.
- Add-harness form writes a plugin file (form + folder are the same thing).

**Gate.** Launch a fake CLI harness (fixture shell script) in a real container, list/kill/restart it, verify the `|| echo` failure path surfaces. Validation probe rejects a "not-a-CLI" fixture with the exact PRD message.

---

## Phase 8 — Preview & diff

**Goal.** `http://localhost:3000` on the box becomes a clickable URL in the browser; changed files are a list.

**Scope.**
- Port detection inside the container (`ss -tlnp`), probe (`GET /` on the detected port), per-project auto port from a pool with manual override.
- Authenticated reverse proxy (cookie in front), stream flush for SSE/WS (`FlushInterval`), no path rewriting — app served at origin root.
- Preview UI: ports panel (port, URL, state), one-tap open.
- Diff: `git status --porcelain -uno`, `git diff --numstat` → `{path, added, deleted}`, raw diff per file; "untracked: N files" note. Diff UI list + plain view.

**Gate.** Start `python3 -m http.server 8000` inside a project (via a button or terminal), see it in the ports panel, open the proxied URL from a phone over LAN. Make a change to the fixture repo and see it in Diff.

---

## Phase 9 — Env editor & persistence

**Goal.** Secrets are a file, masked, and survive restarts.

**Scope.**
- Env endpoints: `GET` (masked, reveal-one), `PUT` (upsert/remove) → writes `project.json` `env` (0600) and uploads `/root/.sps-env` into the container; `profile.d` snippet sources it in every login shell; new sessions see new env without a recreate.
- Secrets never in `docker inspect`, logs, or the UI; project file 0600.
- Persistence audit: repo volume, home volume, data dir — stop/start, server restart, compose down/up survival. Confirm delete scopes don't leak.
- Restart behavior: sessions terminate on container restart; reopen relaunches them (state survives).

**Gate.** Set `FOO=bar`, start a session, `echo $FOO` prints bar. `docker compose down && up`, project still lists, sessions relaunch, `FOO` still set. Reveal-one masked values in UI; assert logs contain no secret.

---

## Phase 10 — Frontend completion & buttons

**Goal.** The litmus test is smooth on a phone.

**Scope.**
- Full React UI: login, projects list, project page with tabs (Sessions, Terminal, Preview, Diff, Env), "＋ New Session", empty-state "Clone a repo →".
- Action buttons: detected defaults (`package.json` scripts, Makefile, `go.mod`), custom `actions` (project file), "＋ Add" two-field form writing the file. Run in a session, stream output to terminal.
- Full creature-comfort key strip (↑, Ctrl-C, Ctrl-L, Tab, Esc, ←/→), copy/paste, desktop-width mode, resize on keyboard open/rotation.
- Mobile pass: portrait/landscape, safe-areas, touch targets.

**Gate.** The PRD litmus on a real phone over LAN: open Go project → tap Run tests → tap Freebuff/conversation → close phone → come back → Diff → handed to OpenCode → Preview → push. List what broke on mobile; fix.

---

## Phase 11 — Packaging & self-hosting

**Goal.** A fresh box becomes a working instance with one command.

**Scope.**
- Server container image (multi-stage: distroless + certs; the control plane only — socket mount does the rest).
- `docker-compose.yml` (self-host): socket mount, data volume, ports `8080` + preview pool, `SMTP_*`, `LOGIN_EMAIL`.
- Quickstart doc: one command to first project. Minimal admin page (projects/containers/sessions/errors) from PRD §12.
- `make e2e`: runs the Phase 5–10 flow against the compose stack, verifying the "image ships images" path exactly as prod does.

**Gate.** Fresh box (or `docker run` on a laptop), `docker compose up`, create + open a project from a phone using real SMTP. `make e2e` green.

---

## Phase 12 — The secretary (v1 subset)

**Goal.** PRD §11, bounded: rides authed CLI credentials, confirm-diff only, no repo writes.

**Scope.**
- System guide (the "taught bounds" doc) shipped as markdown.
- Chat bubble UI (projects screen + per project) that streams an LLM conversation using the authed CLI's provider/key (or `SPS_HELPER_API_KEY` override).
- Tools (server-side, bounded): read project.json / harness plugins / harness config in home volume / container state; probe auth & ports; apply confirm-diff to the three config categories via the Phase 3 file primitive.
- v1 abilities: wire-up-my-key (masked field per harness), switch-model (one-line config diff + confirm), suggest-buttons, where-do-i-stand (portfolio briefing).
- Bounds enforced in code, not prose: no repo/tmux write paths, secrets never in chat, dumb-bot (no polling/autonomy).

**Gate.** With a real API key + a fixture container, run all four v1 abilities; each edit lands only after an explicit confirm; assert the toolset cannot reach a repo path (negative test). Confirm-diff unit tests.

---

## Later, not planned now

Deferred per PRD §12: import/export (repeatable-experience substrate), harness version pins + admin update, headless-browser policy, image-tag versioning. Keep the "copy `$DATA_DIR`" backup story as the v1 story.

## Ordering rationale (waterfall gates)

1→2→3 stack the primitives before any user-facing path exists. 4 (login) gates everything exposed. 5 gives the 80% path bare; 6 makes it interactive; 7 adds agents; 8 adds seeing+diffing; 9 hardens state; 10 is the mobile polish pass on the now-complete loop; 11 ships; 12 is the deliberately last, highest-scope feature so it cannot block the core loop.