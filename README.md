# Live Polling Tool

A real-time polling app. A user signs up, creates a poll and shares the link. Anyone with the link can vote without an account, and everyone looking at the poll sees the results change as votes arrive, with no page refresh.

Built with React, Go (Gin), MongoDB and Redis.

## Live links

- App: https://live-polling-app-amber.vercel.app
- Backend API: https://live-polling-app-i53z.onrender.com/api/v1
- Health check: https://live-polling-app-i53z.onrender.com/healthz
- WebSocket: `wss://live-polling-app-i53z.onrender.com/ws` (used by the app for live updates, not opened in a browser)

The backend runs on Render's free plan, so the first visit after a quiet period can take up to a minute while it wakes up.

## Features

- Sign up and log in; only logged-in users can create or manage polls
- Polls with 2 to 10 options and an optional closing time
- Share a poll with a copy-link button or a QR code
- One vote per person per poll, no account needed to vote
- Live results with percentage bars, a live viewer count and a connection status badge
- Owners can close or delete a poll, and every open page updates immediately

## Project structure

```
/frontend            React app (Vite, TypeScript, Tailwind CSS)
/backend             Go service (Gin)
docker-compose.yml   Local MongoDB and Redis
README.md
```

Inside `/backend`: `handlers` (HTTP and WebSocket), `services` (poll, vote, WebSocket hub), `middleware`, `models`, `database`, `utils` (validation), `config`.

## How each part of the stack is used

| Layer | What it does here |
|---|---|
| React | UI, live result bars, WebSocket hook that reconnects automatically |
| Go (Gin) | REST API, auth, server-side validation, rate limiting, WebSocket rooms |
| MongoDB | Users, polls, and a record of every vote |
| Redis | Vote counters, duplicate-vote check, Pub/Sub for live updates, viewer count, rate limiting |

## How a vote reaches every browser

1. The browser sends the vote to the Go API.
2. The API validates it: the poll is open, the option belongs to the poll, the voter token is a valid UUID.
3. A Redis Lua script checks whether this voter already voted and, if not, increments the option's counter. Both happen in one atomic step.
4. The vote is saved to MongoDB. If that fails, the Redis change is rolled back.
5. The API publishes the new counts to a Redis channel for that poll.
6. Each backend instance listens to the channel and pushes the update over WebSocket to every browser viewing the poll.

## Key decisions

- **MongoDB is the source of truth, Redis is the fast path.** If Redis restarts or a key expires, counts are rebuilt from MongoDB, never reset to zero. A background job repairs any difference left by a crash, and only touches polls with no vote in the last minute so it cannot overwrite a vote still being saved.
- **Duplicate votes.** The Lua script prevents them in Redis, and a unique index on `(poll_id, voter_key)` in MongoDB is a second safety net.
- **Voters are identified by a browser token, not an IP address.** People on the same office or campus network share one IP, so an IP check would block them.
- **Authentication.** Passwords need at least 8 characters with letters and numbers and are hashed with bcrypt. Login returns a JWT that protected routes check.
- **Validation and security.** All input is validated on the server. CORS and the WebSocket origin check allow only my frontend domain. Request bodies are limited to 32 KB. Login, signup and voting are rate limited per IP, and WebSocket connections are capped per IP and per poll. `TRUSTED_PROXIES` controls which proxies are allowed to set the client IP.
- **Health checks.** `/healthz` does no database work, so the host can call it often without using Redis commands. `/api/v1/health` is the detailed check and caches its result for 60 seconds.

## Redis usage on the free plan

Upstash's free plan currently allows 500,000 commands per month. These figures are my estimates from the heartbeats and background jobs, not measured numbers, and I check the usage counter on the Upstash dashboard.

- One poll open in one tab: about 6 commands per minute, roughly 250,000 a month if the tab stays open around the clock.
- Three tabs open: about 10 commands per minute, roughly 445,000 a month if left open around the clock.
- Each vote adds a few commands.

## Run locally

Requirements: Go (version in `backend/go.mod`), Node.js 18+, Docker.

```
git clone https://github.com/jesvinraj/live-polling-app.git
cd live-polling-app

docker compose up -d

cd backend
cp .env.example .env
go run ./cmd/server

cd ../frontend
cp .env.example .env
npm install
npm run dev
```

The API runs on http://localhost:8080 and the app on http://localhost:5173. To try it, open the app in two browser windows (one private), create a poll in the first, open its link in the second and vote. The first window should update without a refresh.

## Environment variables

Backend (`backend/.env.example`):

| Variable | Purpose |
|---|---|
| `GIN_MODE` | `debug` or `release` |
| `MONGO_URI` | MongoDB connection string |
| `MONGO_DB_NAME` | Database name |
| `REDIS_ADDR` | Redis URL (`rediss://` for hosted Redis) |
| `JWT_SECRET` | Signing secret; in release mode the server will not start if it is missing or shorter than 32 characters |
| `JWT_EXPIRATION_HOURS` | Login token lifetime |
| `ALLOWED_ORIGINS` | Frontend address(es) allowed by CORS and the WebSocket origin check |
| `TRUSTED_PROXIES` | Proxies whose forwarded IP headers are trusted |

Frontend (`frontend/.env.example`): `VITE_API_URL` (REST base URL) and `VITE_WS_URL` (WebSocket base URL). These are read at build time.

## Deployment

- **Database:** MongoDB Atlas free cluster.
- **Redis:** Upstash free database, using the `rediss://` URL.
- **Backend:** Render web service (free plan), root directory `backend`.
  - Build: `go build -tags netgo -ldflags '-s -w' -o server ./cmd/server`
  - Start: `./server`
  - Health check path: `/healthz`
- **Frontend:** Vercel, root directory `frontend`, Vite preset. The backend is hosted separately, so only the frontend is built here. `vercel.json` rewrites make direct links like `/polls/<id>` work.

After the frontend was deployed, I set its address in the backend's `ALLOWED_ORIGINS`.

## Known limitations

- A browser token stops accidental repeat votes, but someone who clears their storage or uses a private window can vote again. A "require login to vote" option would close this.
- Render's free plan puts the backend to sleep when idle, and Upstash's free plan caps commands per month.
