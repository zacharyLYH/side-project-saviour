# side-project-saviour
Vibe coding is expensive and developers are busy people! What if you could vibe code use and rotate between free coding harnesses while on the move?

## Required environment variables

Create a `.env` file (it's gitignored) and fill it in. The server loads it from its working directory when you run it on the host — so `server/.env` when you `go run` inside `server/`, or the repo root. In `docker compose`, pass them via `env_file` instead. Real environment variables always win over the file.

| Variable | Description |
|---|---|
| `SPS_LOGIN_EMAIL` | Email address that receives login PINs |
| `SMTP_HOST` | SMTP server (Google: `smtp.gmail.com`) |
| `SMTP_PORT` | SMTP port (Google: `587`) |
| `SMTP_USER` | Gmail address |
| `SMTP_PASSWORD` | Gmail app password |
| `SMTP_FROM` | Sender address |
