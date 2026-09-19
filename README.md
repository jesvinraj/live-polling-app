# Live Polling Tool

A full-stack, real-time polling application built with React, Go (Gin), MongoDB, and Redis.

Users can create polls with custom options and an optional expiration time, share a link or QR code with an audience, and watch live results update across all connected screens instantly as votes come in without needing to refresh the page.

---

## Live Links

- **Live Application (Frontend)**: `https://<your-frontend-subdomain>.vercel.app` *(update after deployment)*
- **Backend API Endpoint**: `https://<your-backend-subdomain>.railway.app/api/v1` *(update after deployment)*
- **Realtime WebSocket Endpoint**: `wss://<your-backend-subdomain>.railway.app/ws` *(update after deployment)*

---

## Tech Stack Overview

- **Frontend (`/frontend`)**: React 18, TypeScript, Vite, Tailwind CSS, Lucide Icons. Provides responsive UI, live animated vote result bars, SVG QR code generation, and auto-reconnecting WebSockets.
- **Backend (`/backend`)**: Go 1.22+ with Gin framework. Handles authenticated REST endpoints, strict input validation, rate limiting, and Gorilla WebSocket rooms.
- **Database**: MongoDB (Atlas). Provides durable persistence for user accounts, poll documents, and an append-only audit trail in the `votes` collection with compound unique indexes.
- **Real-Time Engine**: Redis (Upstash / Redis Cloud). Powers sub-millisecond atomic vote incrementing (`HINCRBY`), atomic deduplication (`SETNX` with Lua script), live Pub/Sub broadcasting (`PUBLISH`), and sorted-set ephemeral viewer presence tracking.

---

## Architecture and Key Design Decisions

### 1. Separation of Concerns
The project is strictly separated into two independent applications:
- `/backend`: A Go REST and WebSocket service using the Gin framework. It follows standard Go patterns, cleanly dividing routes, handlers, services, database drivers, middleware, models, and validation logic.
- `/frontend`: A modern React single-page application built with TypeScript, Vite, and Tailwind CSS.
- Communication between client and server occurs purely via authenticated JSON REST endpoints and bidirectional WebSockets.

### 2. MongoDB as the Durable Source of Truth
MongoDB stores persistent data across server restarts:
- `users`: Stores user credentials with bcrypt-hashed passwords. Email and username have unique indexes.
- `polls`: Stores questions, options, vote tallies, owner references, and expiration timestamps.
- `votes`: An audit log of all cast votes with a compound unique index on `(poll_id, voter_key)` to guarantee database-level deduplication.

### 3. Real-Time Engine with Redis
Redis is not just a cache; it performs three essential real-time operations:
- **Atomic Counters (`HINCRBY`)**: Vote counts are maintained in Redis hashes (`poll:{id}:votes`). Incrementing counters in Redis is atomic and executes in sub-millisecond time, avoiding database lock contention and race conditions during traffic spikes.
- **Atomic Deduplication (`SETNX` with Lua script)**: When a vote is cast, a Redis Lua script checks whether `voter:{poll_id}:{voter_token}` exists. If not, it marks the voter key with a 30-day TTL and increments the option count in a single atomic transaction.
- **Pub/Sub Live Broadcasting**: When a vote is recorded, the Go service publishes a `VOTE_UPDATE` payload to Redis channel `channel:poll:{poll_id}`. Every running backend instance has a single subscription per active poll room, broadcasting the update instantly over WebSockets to all connected browsers.
- **Cache Rebuild on Miss**: If Redis restarts or keys expire, the backend queries MongoDB to retrieve verified vote tallies and re-seeds the Redis hash. Active counts never reset to zero.
- **Ephemeral Viewer Presence**: Viewers are tracked in a Redis Sorted Set (`poll:{id}:viewers_zset`) with timestamp scores refreshed during WebSocket ping heartbeats. Any stale connections older than 120 seconds are automatically pruned, ensuring accurate live viewer counts even after browser or server crashes.

### 4. Anonymous Voter Identification and Shared Networks
To allow audience members to vote without forcing registration, each browser generates a persistent UUID `voterToken` saved in `localStorage`. 
- Using browser tokens rather than raw IP addresses ensures multiple colleagues or students on the same office or university Wi-Fi network (sharing one public IP) can each cast their own vote.
- An IP-based rate limiter (30 requests per minute) prevents automated bot scripts from flooding votes from a single machine.

### 5. Authentication and Security
- **Password Security**: Passwords are validated for length and complexity (minimum 8 characters with letters and numbers) and hashed with `bcrypt`.
- **JWT Authentication**: Protected endpoints require a valid `Bearer <token>` signed with HMAC-SHA256.
- **Origin Validation**: Strict CORS and WebSocket `CheckOrigin` rules restrict access strictly to designated frontend domains.
- **Request Size Limiting**: Middleware restricts request body sizes to 32 KB to protect against memory exhaustion.
- **Rate Limiting**: Redis-backed sliding window limiters protect login, registration, and voting endpoints against brute-force attacks.

---

## Local Development Setup

### Prerequisites
- Go 1.22+
- Node.js 18+ and npm
- Docker and Docker Compose

### 1. Clone the repository
```bash
git clone <your-repo-url>
cd <repo-folder>
```

### 2. Start local MongoDB and Redis containers
```bash
docker compose up -d
```
This starts MongoDB on port `27017` and Redis on port `6379`.

### 3. Configure and start the Go backend
```bash
cd backend
cp .env.example .env
go run ./cmd/server
```
The Go server will start on `http://localhost:8080`.

### 4. Configure and start the React frontend
```bash
cd ../frontend
cp .env.example .env
npm install
npm run dev
```
Open `http://localhost:5173` in your browser.

---

## How to Test the Live Flow Locally

1. Open `http://localhost:5173` in **Browser Window 1**.
2. Register a new account (`Sign Up`) and log in.
3. Click **Create Poll**, enter a question and options, and submit.
4. Copy the public poll URL using the **Copy Link** button.
5. Open an incognito window (**Browser Window 2**) and paste the link.
6. Notice that both windows indicate **2 people viewing** on the live status badge.
7. Cast a vote in Window 2:
   - Window 2 immediately confirms your vote and transitions to the live results view.
   - Window 1 automatically updates its vote counts and animates its percentage bars in real time with zero reload.
8. Click **Close Poll** in Window 1:
   - Both windows immediately display the closed poll banner and disable further voting.

---

## Deployment Guide

### 1. Hosted Database: MongoDB Atlas
1. Create a free cluster on [MongoDB Atlas](https://www.mongodb.com/cloud/atlas).
2. Under **Network Access**, add IP `0.0.0.0/0` (Allow access from anywhere).
3. Under **Database Access**, create a database user with read/write permissions.
4. Copy the connection string:
   `mongodb+srv://<username>:<password>@cluster0.xxxxx.mongodb.net/?retryWrites=true&w=majority`

### 2. Hosted Realtime Engine: Redis Cloud or Upstash
1. Create a free database on [Upstash](https://upstash.com/) or [Redis Cloud](https://redis.com/).
2. Copy the TLS connection URL starting with `rediss://`:
   `rediss://default:<password>@<endpoint>:6379`
   *(Upstash free tier provides 10,000 commands/day; idle consumption for this app is under 2,000 commands/day).*

### 3. Backend Deployment: Railway or Render
1. Push your repository to GitHub.
2. In Railway or Render, create a new Web Service pointing to the `/backend` directory.
3. Set the build command to: `go build -o server ./cmd/server` and start command to `./server`.
4. Set the **Health Check Path** on Render to: `/healthz` *(this lightweight probe returns 200 OK without touching MongoDB or Redis, consuming 0 commands from your free database quotas)*.
5. Add the following environment variables:
   - `PORT`: `8080` (or host assigned port)
   - `GIN_MODE`: `release`
   - `MONGO_URI`: `<your_mongodb_atlas_connection_string>`
   - `MONGO_DB_NAME`: `polling_app`
   - `REDIS_ADDR`: `<your_upstash_or_redis_cloud_url>`
   - `JWT_SECRET`: `<generated_random_64_character_string>` (must be at least 32 characters in release mode)
   - `JWT_EXPIRATION_HOURS`: `72`
   - `ALLOWED_ORIGINS`: `https://your-frontend.vercel.app`
   - `TRUSTED_PROXIES`: `127.0.0.1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16`
6. *Note for Render Free Tier*: If using Render free web services, set up a free monitor on [UptimeRobot](https://uptimerobot.com/) to ping `https://your-backend.onrender.com/healthz` every 5 minutes so the free container never spins down.

### 4. Frontend Deployment: Vercel or Netlify
1. Connect your repository to [Vercel](https://vercel.com/) or [Netlify](https://www.netlify.com/).
2. Set Root Directory to `frontend`.
3. Set Build Command to `npm run build` and Output Directory to `dist`.
4. Add the environment variables:
   - `VITE_API_URL`: `https://your-backend.railway.app/api/v1` (or your Render backend URL)
   - `VITE_WS_URL`: `wss://your-backend.railway.app/ws` (or your Render backend URL)
5. Deploy. The included `vercel.json` and `_redirects` files ensure direct links like `/polls/:id` route correctly.

---

## Redis Command Budget Analysis

- **Idle (0 viewers connected, 0 votes)**:
  - WebSocket hub background auto-expiration check: 0 Redis commands (queries MongoDB directly).
  - Background viewer pruning and reconciliation: 0 Redis commands (skips immediately when no active rooms exist).
  - **Total at idle: 0 Redis commands/minute (0 commands/day)**.
- **Single Connected Viewer (1 poll with 1 open browser tab, no voting)**:
  - WebSocket ping heartbeats (every 54s): ~2.2 commands/min (`ZADD` + `EXPIRE`).
  - Slow background viewer presence cleanup (every 60s): 3.0 commands/min (`ZREMRANGEBYSCORE`, `ZCARD`, `PUBLISH`).
  - Safe reconciliation (every 5 minutes = 0.2 runs/min): ~0.6 commands/min (`GET` timestamp, `SETNX` lock, `HGETALL`).
  - **Total with 1 tab open: ~5.8 Redis commands/minute (~349 commands/hour)**.
  - **Total across a full 24-hour continuous session: ~8,380 commands/day** (comfortably under the 10,000 commands/day free limit on Upstash).
- **Three Connected Viewers (1 poll with 3 open browser tabs, no voting)**:
  - WebSocket ping heartbeats (every 54s across 3 clients): ~6.7 commands/min.
  - Slow presence cleanup (every 60s): 3.0 commands/min.
  - Safe reconciliation (every 5 minutes): ~0.6 commands/min.
  - **Total with 3 tabs open: ~10.3 Redis commands/minute (~618 commands/hour)**.
  - On **Redis Cloud Free Tier** (unlimited commands, 30 MB), it runs 24/7 with zero command caps.

---

## Live Deployment Verification Checklist

After deploying, verify the live system end to end:

- [ ] **Health Endpoint & Secret Safety**: Open `https://your-backend.domain/api/v1/health`. Confirm `database: "connected"`, `redis: "connected"`, and that `clientIp` matches your public IP without exposing passwords, URIs, or JWT secrets.
- [ ] **IP Spoofing Protection Test**: Send a request with a spoofed header:
  ```bash
  curl -H "X-Forwarded-For: 9.9.9.9" https://your-backend.domain/api/v1/health
  ```
  Confirm `clientIp` ignores the fake `9.9.9.9` and continues showing your actual IP address.
- [ ] **Authentication & Validation**: Create an account with a password under 8 characters or without numbers to verify backend rejection. Create a valid account, log out, and log back in.
- [ ] **Poll Creation**: Create a new poll with an optional closing timestamp and confirm it appears on your dashboard.
- [ ] **Live Real-Time Voting**: Open the public poll URL on two separate devices (or scan the QR code on a mobile phone). Cast a vote on one device and verify that the other device updates smoothly in real time without refreshing.
- [ ] **Duplicate Prevention**: Try voting again from the same device and verify it is rejected with a clear message.
- [ ] **Owner Controls**: Close or delete the poll and verify that all open viewer tabs receive the update immediately.

---

## What I Would Add with More Time
- Support for multiple-choice and ranked-choice voting options.
- Exporting live poll results to CSV or summary PDF reports.
- Custom color themes and logo embedding for branded presentation polls.
