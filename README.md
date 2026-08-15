# side-project-saviour
Vibe coding is expensive and developers are busy people! What if you could vibe code use and rotate between free coding harnesses while on the move?

# Product Requirement Document (PRD): Mobile Cloud Coding Agent Control Plane

## 1. Executive Summary

The goal of this project is to build a self-hosted, cloud-based control plane optimized for mobile web browsers. It enables developers to execute, manage, and monitor AI coding agent CLI harnesses remotely, review code changes, manage Git workflows, and preview dynamic web application interfaces directly from a smartphone.

---

## 2. Core Constraints & Objectives

* **Mobile-First UX:** The interface must be tailored for vertical phone screens with touch-friendly navigation and no horizontal scroll clutter.
* **Harness Agility:** The backend must remain agnostic to the underlying coding CLI (e.g., Aider, OpenCode, Claude CLI), enabling seamless rotation between tools and credentials.
* **Persistent Sessions:** Unstable mobile connections or phone locks must not terminate running tasks or background build processes.
* **Git as Source of Truth:** Code synchronization between cloud workspace, local environment, and production must strictly adhere to GitHub repository states.

---

## 3. User Requirements & Technical Architecture

### 3.1 Authentication & Security (KISS Principle)

* **Email OTP Authentication:** Simple email-based login without passwords or social auth OAuth overhead.
* **Verification Workflow:**
* User inputs email address on the login screen.
* System sends a 6-digit numeric OTP with a short Time-To-Live (TTL) (e.g., 5 minutes).
* Successful validation yields a secure session token (JWT/Session Cookie) persistent across mobile browser refreshes.


* **Single-Tenant Guardrails:** Access restricted exclusively to authorized owner email addresses.

### 3.2 Workspace & CLI Harness Execution

* **Cloud Execution Environment:** Persistent containerized Linux workspace (Docker) hosting project code and CLI tooling.
* **Harness Rotation Engine:** A backend wrapper that accepts prompt inputs from the UI and pipes them to designated CLI harnesses, swapping API keys or tools dynamically based on active credit availability.
* **Real-time Streaming:** Input typed into the mobile interface acts as `stdin` for the active CLI tool; `stdout` and `stderr` stream back to the UI in real time.

### 3.3 Mobile UX & Multi-Tab Workspace

To maximize usability on mobile aspect ratios, the UI implements a tabbed/panel navigation paradigm:

#### Panel A: Agent Chat & Control

* **Prompt Bar:** Input area to submit instructions directly to the active CLI harness.
* **Harness Selector:** Interface element to select which underlying CLI tool or credit profile executes the prompt.
* **Lifecycle Buttons:** One-tap action triggers for quick operations:
* View current Git Status / Diffs.
* Stage & Commit changes.
* Create GitHub Pull Request (PR) with auto-generated descriptions.



#### Panel B: Terminal & Process Management

* **Multi-Terminal Support:** Interface to view and switch between all active terminal instances (e.g., CLI Agent stdout, dev web server, background tasks).
* **Persistence via Terminal Multiplexer:** Terminal instances backed by `tmux` headless sessions over WebSockets (`xterm.js` / `ttyd`), ensuring long-running builds or tasks continue uninterrupted when the phone screen turns off.

#### Panel C: Live UI Preview

* **Dynamic Reverse Proxy:** Built-in proxy router serving local development ports (e.g., Vite, Next.js on port 3000) inside an isolated preview pane.
* **Viewport Controls:** Mobile-optimized preview panel with refresh, device dimensions toggle, and error logs panel.

### 3.4 Git & Code Review Workflow

* **Bi-Directional Sync:** Automatic fetch/pull before executing new agent sessions.
* **Visual Diff Inspection:** Native mobile view to review modified, added, or deleted files before finalizing commits.
* **PR Automation:** Seamless integration with GitHub API/CLI to push topic branches and open PRs without dropping down into manual command typing.

---

## 4. Key Functional Scenarios

```
[ Mobile Login ] ──► [ Enter OTP ] ──► [ Select Active CLI Harness ]
                                                 │
   ┌─────────────────────────────────────────────┴─────────────────────────────────────────────┐
   ▼                                             ▼                                             ▼
[ Chat Tab ]                                  [ Terminal Tab ]                             [ Preview Tab ]
• Send prompts to CLI                          • Switch tmux sessions                        • Hot-reloading web UI
• Stream agent actions                        • Monitor dev server logs                     • Viewport size toggling
• Trigger "Review Diff" / "Create PR"         • Direct shell commands                       • Inspect console errors

```

---

## 5. Non-Functional Requirements

* **Latency:** Low-latency WebSocket streaming for real-time terminal input/output responsiveness.
* **Resource Isolation:** Dev container resource limits defined to ensure dev servers and AI CLI engines do not starve the control plane process.
* **Reconnection Resilience:** WebSockets automatically attempt reconnects upon network switches (e.g., Wi-Fi to Cellular) without resetting terminal history.
