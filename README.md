# side-project-saviour
Vibe coding is expensive and developers are busy people! What if you could vibe code use and rotate between free coding harnesses while on the move?

# Product Requirement Document (PRD): Mobile Cloud Agent Control Plane

## 1. Executive Summary

This document specifies the requirements for a self-hosted, cloud-based control plane optimized for mobile web browsers. The platform enables developers to execute, manage, and monitor AI coding agent CLI tools remotely, perform code reviews, manage Git workflows, and preview dynamic web application UIs straight from a smartphone screen.

The architecture offloads local API key/harness management by embedding **OmniRoute** as a background AI gateway and relies on **`tmux`** to ensure persistent headless terminal sessions.

---

## 2. System Architecture & Component Roles

```
┌────────────────────────────────────────────────────────────────────────┐
│                   Mobile Web UI (React / Next.js)                      │
│   [ Auth Pane ] ── [ Agent Chat Pane ] ── [ Terminals ] ── [ Preview ]  │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ (WebSockets & Dynamic Reverse Proxy)
┌───────────────────────────────────▼────────────────────────────────────┐
│                    Cloud Host Workspace (Netcup/VPS)                   │
│                                                                        │
│   ├── Control Plane Engine (Node.js/Go backend daemon)                  │
│   │   ├── Authentication & Token Validation (OTP)                      │
│   │   ├── Terminal Manager (xterm.js ↔ tmux PTY bridge)               │
│   │   └── Reverse Proxy (Routes dynamic dev ports to preview pane)     │
│   │                                                                    │
│   ├── Workspace Shell (tmux sessions)                                  │
│   │   ├── Session 1: Coding Agent CLI (Aider / OpenCode / Claude Code) │
│   │   └── Session 2+: Arbitrary Free Terminals & Dev Server           │
│   │                                                                    │
│   └── Local AI Gateway (OmniRoute service @ http://localhost:20128/v1) │
│       └── Pools & rotates free/paid AI provider tokens automatically  │
└────────────────────────────────────────────────────────────────────────┘

```

### Architectural Responsibilities:

* **OmniRoute (Background AI Router):** Handles token lifecycle, multi-provider quota fallbacks, and API account rotation. The agent CLI tools target OmniRoute’s OpenAI-compatible endpoint locally.
* **Control Plane Daemon:** Manages browser sessions, exposes dynamic terminal streams over WebSockets, proxies local development ports (e.g., Vite/Next.js) into web viewports, and executes Git lifecycle actions.
* **`tmux` Workspace Runtime:** Guarantees process persistence so long-running agent tasks, builds, or commands continue running even if the mobile device disconnects or locks its screen.

---

## 3. Detailed Requirements

### 3.1 Authentication & Access Control (KISS Principle)

* **Single-Tenant Guardrails:** Access is strictly limited to authorized owner email addresses.
* **Passwordless OTP Login:**
* User submits an email address on the initial login screen.
* System generates a 6-digit numeric OTP with a short Time-To-Live (5-minute TTL).
* Validating the code yields an HTTP-only session cookie persistent across browser refreshes.


* **Headless Out-of-Band OAuth Flow:**
* Initial browser-based CLI authentication (e.g., GitHub CLI, third-party services) is handled via SSH port forwarding (`ssh -L`) or a lightweight containerized headless browser (`noVNC + Chromium`) running on the host.



### 3.2 Workspace & Process Persistence

* **State Preservation:** All active shells and agent routines execute within background `tmux` sessions on the host machine.
* **Network Disconnection Resilience:** Closing the mobile browser, changing networks (Wi-Fi to Cellular), or phone lock states does not interrupt running tasks. Re-opening the UI automatically re-attaches to the active streams.

### 3.3 Mobile UI Panels (Optimized for Mobile Viewports)

#### Panel A: Agent Chat & Actions

* **Prompt Input:** Mobile-optimized prompt box sending user commands directly into `stdin` of the running CLI agent session.
* **Stream Output:** Live rendering of agent CLI `stdout`/`stderr` formatted with ANSI color decoding.
* **One-Tap Git Actions:** Dedicated visual buttons for rapid lifecycle commands:
* **Status & Diff:** Opens a visual modal detailing modified, added, and deleted files.
* **Commit & Push:** Automatically stages changes, prompts for a commit message, and pushes to GitHub.
* **Create PR:** Pushes current working branch to GitHub and triggers PR creation via GitHub API/CLI.



#### Panel B: Terminal Switcher (Multi-Terminal View)

* **Terminal Carousel/Tabs:** Ability to switch between the main CLI Agent execution stream and arbitrary headless terminal shells (0 or more).
* **Terminal Engine:** Integrated `xterm.js` over WebSocket connection providing full terminal interaction (interactive prompts, key bindings, scrolling).
* **Server Tailing:** Secondary terminals reserved for monitoring dev server logs (`npm run dev`, Docker build logs).

#### Panel C: Live UI Preview Pane

* **Dynamic Proxy Routing:** Reverse proxy exposing local app build ports (e.g., `localhost:3000`) securely to the mobile browser view.
* **Iframe Controls:** Top navigation bar inside the preview panel containing manual refresh, active dev port selector, and mobile/desktop viewport toggle buttons.

---

## 4. Hardware & Hosting Baseline

* **CPU & RAM Target:** 2 vCPUs / 4 GB RAM minimum (Sweet spot for running Node dev servers, OmniRoute, `tmux`, and fast TypeScript compile toolchains concurrently without OOM crashes).
* **Recommended Compute Hosts:** Netcup (VPS 1000/Lite series) or OVHcloud VPS for optimal price-to-performance; or a local home machine exposed via a Tailscale mesh network.

---

## 5. Summary of Key Scenarios

1. **Prompt & AI Routing:** User types a request in **Panel A** $\rightarrow$ Control plane passes text to CLI Agent $\rightarrow$ CLI Agent calls local **OmniRoute** endpoint $\rightarrow$ OmniRoute automatically picks an available free AI provider quota $\rightarrow$ Response streams back to user's phone in real time.
2. **Web App Testing:** CLI Agent edits frontend code $\rightarrow$ Dev server in **Panel B** triggers hot reload $\rightarrow$ User switches to **Panel C** to test UI changes directly in an embedded preview.
3. **Review & PR Creation:** User reviews changes in the Diff view $\rightarrow$ Taps **"Create PR"** $\rightarrow$ Code is pushed to GitHub and a PR link is generated, completing the workflow without typing manual git commands on a phone keyboard.
