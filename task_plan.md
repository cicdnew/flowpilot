# FlowPilot Build/Deploy Task Plan

## Phase 1 — Build & Unit Tests ✅ COMPLETE

- [x] Fix build errors (shadowed imports, missing types)
- [x] Fix DB helpers (stale tx, mutex, timeout, log limits)
- [x] Fix batch loop bounds & copy bugs
- [x] Fix all unit tests
- [x] Verify build + all tests pass

## Phase 2 — Config & Docker Setup

- [x] Create `.flowpilot.yml` (mode, port, DB path, Chrome flags)
- [x] Create `Dockerfile.prod` (multi-stage build)
- [x] Create `Dockerfile.server` (headless, --yolo mode)
- [x] Create `Dockerfile.full` (BrowserBase proxy for headful Chrome)
- [x] Create `docker-compose.yml` (app + server + BrowserBase services)
- [x] Create `entrypoint.sh` (auto-detect mode: serve vs full)
- [x] Create `.env.example` with all required env vars
- [x] Add WebSocket proxying to `server.py` (SOCKS5 ↔ BrowserBase)
- [x] Update GitHub Actions CI (build only, no deploy)
- [x] Update README with deploy instructions

## Phase 3 — BrowserBase Setup (Required for deploy)

- [ ] Sign up at https://www.browserbase.com
- [ ] Create API key + Project ID
- [ ] Add secrets to GitHub repo:
  - `BROWSERBASE_API_KEY`
  - `BROWSERBASE_PROJECT_ID`
- [ ] (Optional) Enable stealth mode add-on in BrowserBase dashboard

## Phase 4 — Railway Deploy

### 4a — Headless Server
- [ ] Go to Railway dashboard → New Project
- [ ] Connect GitHub repo `alihasan/flowpilot`
- [ ] Select **server** (Dockerfile.server)
- [ ] Set env vars:
  - `DATABASE_URL = postgresql://...`  (from Railway Postgres addon)
  - `SESSION_TTL_HOURS = 24`
  - `RATE_LIMIT_RPM = 30`
  - `BIND_HOST = 0.0.0.0`
  - `BIND_PORT = 8080`
  - `ROVODEV_MODE = server`
  - `ROVODEV_YOLO = true`
- [ ] Set port to 8080
- [ ] Get deployed URL (e.g. `flowpilot-server.up.railway.app`)

### 4b — BrowserBase Integration (Full Mode)
- [ ] Add to same project, select **full** (Dockerfile.full)
- [ ] Set env vars:
  - `BROWSERBASE_API_KEY = <your key>`
  - `BROWSERBASE_PROJECT_ID = <your project>`
  - `FLOWPILOT_SERVER_URL = https://flowpilot-server.up.railway.app`
  - `BIND_HOST = 0.0.0.0`
  - `BIND_PORT = 8081`
- [ ] Set port to 8081
- [ ] Get deployed URL (e.g. `flowpilot-full.up.railway.app`)

### 4c — Client Config
- [ ] Update client `.flowpilot.yml`:
  ```yaml
  mode: web
  server:
    url: https://flowpilot-server.up.railway.app
    auth_token: <generated_token>
  timeout_seconds: 600
  ```

## Phase 5 — Telegram Bot Deploy (Optional)

### 5a — BotFather Setup
- [ ] Create bot via @BotFather
- [ ] Get bot token
- [ ] Disable privacy mode (`/setprivacy` → Disable)
- [ ] Set bot commands:
  ```
  run - Run a Rovo Dev task
  plan - Generate a build plan
  chat - Send a message
  status - Check status
  ```
- [ ] Copy bot token

### 5b — Telegram Worker Deploy
- [ ] In Railway, deploy **telegram-worker** service
- [ ] Set env vars:
  - `TELEGRAM_BOT_TOKEN = <your token>`
  - `DATABASE_URL = postgresql://...`
  - `SESSION_TTL_HOURS = 24`
  - `RATE_LIMIT_RPM = 10`
  - `BIND_HOST = 0.0.0.0`
  - `BIND_PORT = 8082`
- [ ] Set port to 8082
- [ ] Verify bot responds to `/run` and `/plan`

## Phase 6 — Testing & Validation

### 6a — Server Tests
- [ ] Health check: `curl https://<server>/health`
- [ ] Create session: `curl -X POST https://<server>/session`
- [ ] Run task via REST API
- [ ] Check WebSocket proxy works

### 6b — Telegram Tests
- [ ] Send `/run test task` to bot
- [ ] Verify streaming response
- [ ] Verify logs show in database

### 6c — E2E Flow
- [ ] `flowpilot run "hello world"` via server
- [ ] `flowpilot run "create a React app"` via full mode (with BrowserBase)
- [ ] Telegram `/run` and `/plan` commands
- [ ] Check database has correct session records

## Phase 7 — Monitoring & Ops

- [ ] Set up Railway metrics dashboard
- [ ] Configure alerts for high CPU/memory
- [ ] Set up log drain to external service (optional)
- [ ] Configure autoscaling (if needed)
- [ ] Set up backup for database (Railway Postgres backup)

## Phase 8 — Cost Optimization

- [ ] Monitor BrowserBase session usage
- [ ] Tune BrowserBase add-ons (stealth, IP geolocation)
- [ ] Optimize container resource limits
- [ ] Consider using Railway's free tier for dev environment
- [ ] Set up cost alerts

## Rollback Plan

If deployment fails:
1. Check Railway logs for errors
2. Verify environment variables are set correctly
3. Check database connectivity
4. Roll back to previous commit if needed
5. Use `railway down` to tear down if necessary
