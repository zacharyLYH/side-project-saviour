# side-project-saviour
Vibe coding is expensive and developers are busy people! What if you could vibe code use and rotate between free coding harnesses while on the move?

# 1. Product Summary

### Working concept

A **self-hosted, container-based cloud development environment** that users access primarily through a web browser, including mobile.

Its core differentiator is that users can run **multiple AI coding harnesses inside the same project environment**, switching between them freely.

Examples of harnesses:

* Freebuff
* OpenCode
* Aider
* Future open-source/free coding harnesses
* User-created/custom harnesses

The product does **not** attempt to orchestrate these harnesses intelligently in v1.

Instead:

> **The user chooses a harness → the server launches it in a persistent tmux session → the browser displays that session.**

This deliberately keeps the architecture simple.

---

# 2. Product Thesis

Existing cloud coding agents tend to combine:

> cloud environment + proprietary/selected coding agent + proprietary UX.

This product separates them:

> **cloud development environment + arbitrary coding harnesses + lightweight browser UI**

The user owns the infrastructure and can host it on:

* Mac Mini
* home server
* VPS
* cloud VM
* dedicated server
* potentially Kubernetes later

The product should feel like:

> **tmux + Docker + a lightweight web UI + AI coding agents**

rather than:

> “another fully-fledged browser IDE.”

---

# 3. Goals

## Primary goals

1. **Self-hosted**

   * No mandatory SaaS backend.
   * No mandatory account with the product creator.
   * User owns compute, projects, credentials and data.

2. **Environment-agnostic**

   * Node is not a requirement.
   * Python, Go, Rust, Java, C++, etc. should work.
   * Arbitrary Linux development environments should be possible.

3. **Containerized**

   * Each project gets an isolated, reproducible environment.
   * AI harnesses operate inside that environment.

4. **Multiple AI harnesses**

   * Ship several useful harness integrations/configurations.
   * Users choose which harness to launch.
   * Users can add their own harnesses.

5. **Persistent sessions**

   * Harnesses run in tmux sessions.
   * Disconnecting the browser does not kill the work.
   * Users can reconnect later.

6. **Excellent mobile accessibility**

   * The primary interface should work well from a phone.
   * No requirement to use a desktop IDE.

7. **KISS**

   * Avoid building an orchestration platform.
   * Avoid reinventing coding-agent internals.
   * Prefer exposing existing tools directly.

---

# 4. Explicit Non-Goals

These are important because the project can easily balloon.

## Not building

### ❌ A SaaS

No:

* billing
* subscriptions
* hosted multi-tenancy
* central account infrastructure
* usage-based billing
* mandatory cloud infrastructure

### ❌ A new AI coding harness

We are not building our own:

* planner
* coding agent
* tool-use loop
* context manager
* reasoning engine

Harnesses such as OpenCode/Freebuff/Aider do that.

### ❌ An AI harness orchestrator — initially

We are **not** automatically:

* selecting the best harness
* detecting rate limits
* migrating conversations between harnesses
* transferring context between harnesses
* automatically switching models
* merging multiple agents' work

The user is the orchestrator.

### ❌ A browser IDE

We are not initially building:

* Monaco-based IDE
* sophisticated file explorer
* integrated debugger
* language servers
* autocomplete
* full Git client
* VS Code clone

### ❌ Automatic model routing

The product may expose multiple harnesses, but it does not need to understand the models inside them.

If one harness is rate-limited:

> User opens another harness.

Simple.

### ❌ Universal abstraction over harness APIs

We do not need a sophisticated normalized event protocol.

A harness is fundamentally:

> **a process the platform knows how to launch in a session.**

---

# 5. Core User Experience

The fundamental workflow should be:

```text
Add repository
      ↓
Create/prepare environment
      ↓
Open project
      ↓
Choose AI harness
      ↓
Launch harness in tmux
      ↓
Interact through browser
      ↓
Disconnect / close phone
      ↓
Harness keeps running
      ↓
Return later
      ↓
Reconnect to same session
```

The user should be able to have:

```text
Project: my-saas

┌──────────────────────────────────────────────┐
│ OpenCode │ Freebuff │ Terminal │ Preview │ + │
├──────────────────────────────────────────────┤
│                                              │
│             Current session                  │
│                                              │
│        live terminal / harness UI            │
│                                              │
└──────────────────────────────────────────────┘
```

The UI is fundamentally **tabs over persistent sessions**.

---

# 6. Core Mental Model

The product has four primary concepts.

## Project

A Git repository/workspace.

Examples:

* `my-saas`
* `personal-api`
* `ml-experiment`

## Environment

The isolated runtime in which the project operates.

Typically a Docker container or collection of containers.

Contains:

* source code
* dependencies
* development tools
* environment variables
* running processes

## Session

A persistent interactive process inside the environment.

Examples:

* OpenCode
* Freebuff
* Aider
* bash
* zsh
* custom agent

Sessions are backed by tmux initially.

## View

A browser representation of something associated with the project/session.

Examples:

* terminal
* AI harness
* web preview
* diff
* logs

---

# 7. Architecture

```text
                         USER
                          │
                          ▼
                  ┌───────────────┐
                  │   Web Client  │
                  │ Desktop/Mobile│
                  └───────┬───────┘
                          │
                       WebSocket
                          │
                          ▼
                  ┌───────────────┐
                  │ Project Server│
                  └───────┬───────┘
                          │
             ┌────────────┼────────────┐
             │            │            │
             ▼            ▼            ▼
        Project Mgmt   Session Mgmt  Preview
             │            │
             │            ▼
             │          tmux
             │            │
             │       ┌────┼────┬─────┐
             │       ▼    ▼    ▼     ▼
             │     OpenCode Freebuff Aider bash
             │
             ▼
        Docker Environment
             │
      ┌──────┼──────────────┐
      ▼      ▼              ▼
    Files  Processes     Dependencies
```

The server is the **control plane**.

The container is the **execution plane**.

The browser is the **interaction plane**.

---

# 8. Container Architecture

## Principle

The container is the boundary between the host and project.

AI harnesses should generally execute **inside the project's environment**, not directly on the host.

Conceptually:

```text
Host
│
├── Product Server
│
└── Project Container
    │
    ├── /workspace
    │   └── repository
    │
    ├── dependencies
    ├── environment variables
    ├── tmux
    │
    ├── OpenCode session
    ├── Freebuff session
    ├── terminal session
    └── application processes
```

This allows different projects to have completely different environments.

Example:

```text
Project A
Ubuntu + Go + PostgreSQL

Project B
Ubuntu + Python + PyTorch

Project C
Ubuntu + Rust

Project D
Ubuntu + Node + Redis
```

---

# 9. Environment Initialization

A major product feature is making a repository usable by an AI agent without extensive manual setup.

The system should inspect the project and determine how to prepare it.

Prefer existing project conventions over AI inference.

Possible signals include:

* Dockerfile
* Docker Compose
* devcontainer configuration
* language manifests
* package manifests
* lockfiles
* README
* setup scripts

Examples:

```text
go.mod
    → Go environment

package.json
    → Node environment

pyproject.toml
    → Python environment

Cargo.toml
    → Rust environment

Dockerfile
    → use project-defined environment
```

AI inference can eventually be used when the repository gives insufficient information.

### Desired initialization UX

```text
Initialize Project

✓ Repository cloned
✓ Environment detected
✓ Dependencies installed
✓ Services started
✓ Tests executed

Environment ready.
```

The goal is:

> **An incoming AI agent should be able to inspect the environment and become productive with minimal human explanation.**

---

# 10. Project Portability

Projects should ideally be portable between machines.

The system should avoid creating proprietary project metadata that is necessary to understand the codebase.

The repository remains the primary source of truth.

Useful project conventions should preferably be represented through standard mechanisms such as:

* Docker
* devcontainers
* Compose
* language manifests
* scripts
* documentation

The product should enhance these rather than replace them.

---

# 11. Harness Model

A harness is simply an available executable/program that can be launched inside a project environment.

Examples:

```text
AI Harnesses
────────────
OpenCode
Freebuff
Aider
...
```

And non-AI sessions:

```text
Utilities
─────────
Terminal
Logs
...
```

The initial harness registry can be extremely simple:

```text
Harness
├── display name
├── command
├── icon/metadata
└── optional setup requirements
```

There is deliberately **no requirement for a universal harness API**.

---

# 12. Harness Launching

When the user selects:

> OpenCode

the platform:

1. ensures the project environment exists
2. ensures required dependencies are available
3. creates a tmux session
4. launches the harness inside it
5. connects the browser to that session

Conceptually:

```text
User
 ↓
"Open OpenCode"
 ↓
Server
 ↓
Project container
 ↓
Create tmux session
 ↓
Launch OpenCode
 ↓
Browser attaches
```

The same mechanism applies to:

* Freebuff
* Aider
* terminal
* custom user commands

---

# 13. Why tmux?

tmux gives us persistent sessions almost for free.

It provides:

* process persistence
* reconnectability
* multiple sessions
* terminal semantics
* familiar Linux tooling
* easy debugging
* low conceptual complexity

If the browser disconnects:

```text
Browser ✕
    │
    │
    ▼
tmux session continues
    │
    ▼
harness continues working
```

When the user reconnects:

```text
Browser
   ↓
Attach to existing tmux session
```

This is exactly the behavior we want.

---

# 14. Browser ↔ Terminal Interaction

The browser should essentially act as a remote terminal client.

The server needs to provide:

* terminal input
* terminal output
* resize events
* session lifecycle
* reconnect support

WebSockets are the natural transport.

The browser should not need to understand the underlying harness.

It receives a terminal/session stream.

This allows the product to support arbitrary TUIs without understanding their internals.

---

# 15. Mobile UX

Mobile is a first-class target, but **mobile should not dictate the underlying architecture**.

The UI should prioritize:

### Project list

```text
My Projects

● my-saas
● personal-api
● ml-experiment
```

### Project sessions

```text
my-saas

[ OpenCode ]
[ Freebuff ]
[ Terminal ]
[ Preview ]
[ + New Session ]
```

### Session view

The session itself should occupy most of the screen.

Controls should remain minimal:

```text
← my-saas       ⋮

OpenCode
────────────────────

<interactive session>

────────────────────
```

Potential mobile improvements:

* large touch targets
* virtual keyboard support
* copy/paste
* scrollback
* session reconnect
* portrait/landscape support
* quick session switching

Do not initially attempt to redesign every TUI into a native mobile UI.

---

# 16. Tabs

Tabs are the central UX abstraction.

Possible tabs:

```text
🤖 OpenCode
🤖 Freebuff
💻 Terminal
🌐 Preview
📄 Diff
```

The important conceptual point:

> A tab does not necessarily represent a UI page. It represents a persistent project interaction/session.

For v1:

* AI tabs → tmux sessions
* terminal tabs → tmux sessions
* preview → proxied application
* diff → web view
* settings → normal web UI

This is intentionally heterogeneous behind one simple UI.

---

# 17. Web Preview

For projects exposing HTTP services, the platform should make them accessible through the browser.

Conceptually:

```text
Project container
       │
       │ localhost:3000
       ▼
Project server
       │
       │ authenticated proxy
       ▼
Browser
```

The user can therefore:

1. ask an AI harness to modify the application
2. switch to Preview
3. see the result immediately

This is especially useful for web projects.

Preview should **not** initially become a full deployment system.

---

# 18. Diff View

A simple diff viewer is highly valuable because AI-generated changes need inspection.

Minimum functionality:

* changed files
* additions/deletions
* syntax-aware or plain-text diff
* refresh
* optionally link to file

Example:

```text
Changes

M src/auth.go       +14 -3
M src/server.go     +8 -1
A tests/auth_test.go +42

[ View ]
```

Do not initially build:

* full IDE editing
* inline commenting
* advanced merge tooling
* visual Git client

Git remains available through the terminal/harness.

---

# 19. Environment Variables

Provide a simple project-level environment variable editor.

Example:

```text
Environment Variables

DATABASE_URL       •••••••••
OPENAI_API_KEY     •••••••••
STRIPE_SECRET_KEY  •••••••••

[ Add variable ]
```

Requirements:

* masked secrets by default
* editable values
* add/remove
* inject into project environment
* restart/recreate environment when necessary

Secrets should not casually appear in browser output or logs.

Advanced secret-management systems are out of scope for v1.

---

# 20. Multiple Harnesses Are the Primary Differentiator

The product should make this behavior normal:

```text
User starts Freebuff
        ↓
works for a while
        ↓
Freebuff rate limit
        ↓
User opens OpenCode
        ↓
same project
same files
same environment
        ↓
continue working
```

There is no need to transfer conversation history.

The shared state is the project itself:

```text
Filesystem
Git state
Processes
Logs
Database
Environment
```

The human can tell the new harness what they want, or let it inspect the project.

This is intentionally simpler than automated agent handoff.

---

# 21. User-Provided Harnesses

Users should eventually be able to add arbitrary harnesses.

Example conceptual configuration:

```text
Name: My Agent
Command: my-agent
```

The product should then expose it under:

```text
+ New Session
    ↓
My Agent
```

This makes the system extensible without requiring the core project to know every AI harness.

Potential future extension:

* custom setup command
* required environment variables
* required packages
* default working directory
* icon
* container image

But v1 should keep this extremely lightweight.

---

# 22. Harness Installation

There are two reasonable approaches.

## Option A — Preinstalled in project image

The image contains common harnesses.

Advantages:

* fastest launch
* predictable
* easy to understand

Disadvantages:

* larger images
* harness versions need updating

## Option B — Install on demand

When a harness is selected, install it into the environment.

Advantages:

* smaller base environment
* easy to add harnesses

Disadvantages:

* slower first launch
* dependency/network failures

### Recommended

Support both eventually.

For v1, prefer **preconfigured harness installation where practical**, with user-defined commands as an escape hatch.

---

# 23. Security Model

This is a self-hosted developer tool with AI processes capable of executing arbitrary commands.

Security must therefore be explicit.

## Principle

Treat project containers and AI harnesses as **untrusted code execution environments**.

At minimum:

* isolate project environments from the host
* avoid unnecessary host filesystem mounts
* avoid privileged containers
* constrain resources where practical
* control network access where practical
* never expose the Docker socket directly to project processes
* protect environment variables/secrets
* authenticate access to the web UI
* use HTTPS when exposed beyond localhost/trusted LAN

The product should make the dangerous capabilities obvious to the administrator.

---

# 24. Self-Hosting

The ideal initial installation experience:

```text
Install
   ↓
Configure
   ↓
Start server
   ↓
Open browser
   ↓
Create project
```

Target deployment environments:

### First-class

* Linux server
* macOS
* VPS
* home server

### Eventually

* Kubernetes
* NAS
* ARM machines
* cloud VMs

The system should not require a proprietary cloud service.

---

# 25. Data Ownership

All important user state should live on the user's machine/server.

Data includes:

* repositories
* project configuration
* containers
* environment variables
* session state
* tmux sessions
* logs

No application data should need to pass through a central vendor backend.

AI provider communication naturally leaves the machine according to whichever harness/provider the user chooses.

---

# 26. Authentication

Because this is self-hosted, authentication can initially be simple.

Minimum requirements:

* local authentication
* configurable password/access mechanism
* session authentication
* ability to bind to localhost/LAN
* secure remote deployment guidance

Do not initially build:

* organizations
* RBAC
* SSO
* teams
* enterprise identity

Those can be added if actual users need them.

---

# 27. Persistence

A project should survive:

* browser refresh
* browser disconnect
* server restart where practical
* container restart where practical

tmux sessions should persist while their environment is running.

Project state should not depend on the browser being open.

---

# 28. Failure Handling

The system should favor transparency over clever recovery.

Examples:

### Harness crashes

Show:

> OpenCode exited unexpectedly.

Allow:

> Restart session.

### Harness rate-limited

Do **not** automatically migrate.

Show:

> Freebuff appears to be rate-limited.

User can:

> Open another harness.

### Container dies

Show:

> Project environment stopped.

Allow:

> Restart environment.

### Preview unavailable

Show:

> No application currently listening on the configured port.

This is preferable to building complicated automatic recovery prematurely.

---

# 29. Versioning

Projects should ideally pin important environment versions.

Examples:

* base image
* language runtime
* package manager
* harness versions

The product should avoid silently changing a working environment underneath the user.

---

# 30. Observability

The administrator should be able to determine:

* which projects exist
* which containers are running
* which sessions are running
* resource usage
* recent errors
* container logs
* server logs

A simple admin/debug screen is sufficient.

No need for:

* Datadog integration
* distributed tracing
* complex metrics pipelines

in v1.

---

# 31. Resource Management

AI agents can consume substantial CPU/RAM.

The system should eventually support per-project limits:

```text
CPU:    4 cores
Memory: 8 GB
Disk:   50 GB
```

But reasonable defaults are sufficient for the first implementation.

The server should prevent one runaway project from trivially consuming the entire host.

---

# 32. Git

Git is fundamental but should not become a separate product.

The platform needs to understand enough Git to:

* clone repositories
* identify repository state
* display basic status
* display diffs
* optionally create commits/branches later

All advanced Git operations can remain accessible through the terminal or harness.

---

# 33. Project Lifecycle

A project lifecycle might be:

```text
Create
  ↓
Clone
  ↓
Initialize
  ↓
Running
  ↓
Stopped
  ↓
Restart
  ↓
Delete
```

Deletion should distinguish:

* delete container
* delete project metadata
* delete repository data

Destructive operations should require confirmation.

---

# 34. Proposed MVP

The smallest version that demonstrates the thesis:

### Infrastructure

* self-hosted server
* Docker-based project environments
* persistent project storage
* tmux

### Web UI

* project list
* project creation
* project page
* session tabs
* terminal/session streaming

### Harnesses

* 2–3 popular AI harnesses
* terminal
* configurable custom harness

### Project features

* Git repository cloning
* environment initialization
* environment variables
* basic diff
* basic web preview

### Persistence

* persistent containers
* persistent tmux sessions
* reconnect after browser disconnect

That is enough.

---

# 35. MVP User Journey

```text
1. User installs the application.

2. User opens it from their phone.

3. User creates:
      my-project

4. User enters:
      GitHub repository URL

5. System initializes the environment.

6. User sees:
      ✓ Project ready

7. User taps:
      + New Session

8. User chooses:
      Freebuff

9. Freebuff launches in a tmux session.

10. User gives it a coding task.

11. User closes their phone.

12. Agent continues running.

13. User returns later.

14. Session is still there.

15. Freebuff hits its limit.

16. User opens another tab:
      OpenCode

17. OpenCode sees the same project state.

18. User continues.

19. User opens:
      Preview

20. User inspects:
      Diff

21. User commits/pushes through the harness or terminal.
```

If this experience works well, the product has proven its core thesis.

---

# 36. Future Possibilities

These are deliberately **not MVP requirements**.

## Automatic harness failover

```text
Freebuff
   ↓ rate limit
OpenCode
   ↓ rate limit
Aider
```

Potentially invisible to the user.

## Harness capability detection

Understand:

* model
* context length
* tool support
* rate limits
* authentication

## Automatic task routing

Choose the most appropriate harness.

## Agent handoff

Generate state summaries when switching harnesses.

## Multiple simultaneous agents

```text
Agent A → backend
Agent B → frontend
Agent C → tests
```

## Remote environments

Allow environments to live on separate machines.

## Kubernetes

Run many isolated project environments.

## GPU environments

Useful for ML projects.

## Browser automation

Expose Playwright/browser sessions.

## Scheduled agents

Run projects periodically.

## Shared projects

Multiple people connecting to the same environment.

## Rich IDE features

Only if demand emerges.

---

# 37. Architecture Evolution

The architecture should permit future sophistication without requiring it now.

### V1

```text
Browser
  ↓
Server
  ↓
Docker
  ↓
tmux
  ↓
Harness
```

### Possible V2

```text
Browser
  ↓
Server
  ↓
Session Manager
  ↓
Harness Manager
  ↓
Harness
```

### Possible V3

```text
Browser
  ↓
Agent Orchestrator
  ↓
Task Router
  ├── Harness A
  ├── Harness B
  └── Harness C
        ↓
   Shared Environment
```

The important principle:

> **Do not build V3 infrastructure to solve V1 problems.**

---

# 38. Product Principles

### 1. Existing tools first

If Docker, tmux, Git, OpenCode, Freebuff or another tool already solves something, use it.

### 2. The project filesystem is the shared state

Don't invent complicated synchronization mechanisms.

### 3. The user can orchestrate

Users can manually switch harnesses.

Automation is optional future functionality.

### 4. Terminal is a feature, not a failure

A raw terminal is an extremely powerful escape hatch.

### 5. Don't build an IDE

The browser UI should expose the environment, not replace every developer tool.

### 6. Self-hosted means hackable

Users should be able to understand and modify how the system works.

### 7. Prefer boring primitives

Docker.

tmux.

Git.

WebSockets.

Filesystem.

HTTP.

Simple configuration.

### 8. Optimize for the first 80%

A user should get from:

> “I have a repo”

to:

> “AI is working on it”

as quickly as possible.

---

# 39. Core Differentiation

The product is **not** primarily differentiated by having a better AI.

It is differentiated by combining:

```text
Self-hosted
     +
Containerized
     +
Environment-agnostic
     +
Multiple coding harnesses
     +
Persistent sessions
     +
Mobile-accessible
     +
Simple/hackable
```

The resulting proposition:

> **Your own AI coding cloud, running on your hardware, where you can use whichever coding agent you want.**

---

# 40. The Litmus Test

The project is succeeding if this feels natural:

> “I have a Go project running on my home server. I open it from my phone, start Freebuff, tell it to fix something, close my phone, come back two hours later, inspect the diff, notice Freebuff is rate-limited, open OpenCode in another tab, tell it to continue, check the web preview, and push the result.”

If that workflow is smooth, **we have a product**.

If we find ourselves building an elaborate abstraction merely to make the underlying harnesses invisible, we should stop and ask:

> **Does the user actually care?**

For v1, the answer should probably be **no**.

The magic is not invisible orchestration.

The magic is:

> **“There is a computer in my pocket that I own, my project is always there, and I can throw whichever AI agent I want at it.”**
