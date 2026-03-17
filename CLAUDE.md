# Erebus API

REST API server exposing Nyx Command Center functionality over HTTP for PWA/mobile consumption.

## Stack
- Go + Fiber v2
- MongoDB Atlas
- Google OAuth2 + JWT auth
- SSE streaming for chat proxy

## Build & Run
```bash
go build -o erebus-api .
./erebus-api
```

## Environment
Copy `.env.example` to `.env` and fill in values. Required: `MONGODB_URI`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `JWT_SECRET`.

## Project Layout
- `main.go` — server bootstrap and route registration
- `config.go` — environment variable loading
- `middleware.go` — JWT auth, CORS, request logging
- `handlers/` — route handlers by domain
- `models/` — MongoDB document structs
- `db/` — database connection
