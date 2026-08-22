# side-project-saviour
Vibe coding is expensive and developers are busy people! What if you could vibe code use and rotate between free coding harnesses while on the move?

## Run locally

```
go -C server run ./cmd/server        # backend on :8080
npm -C web run dev                   # frontend on :5173 (proxies /api + /ws)
```

Project containers survive server exits by design (tmux sessions keep running).
When you're done hacking, tear down everything the server created:

```
docker rm -f $(docker ps -aq --filter name=sps-)
docker volume rm $(docker volume ls -q --filter name=sps-)
docker network rm sps-net
rm -rf server/data                   # optional: reset all local state
```

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
