```
██████╗ ██╗ ██████╗ ████████╗     ██████╗  █████╗ ████████╗███████╗██╗    ██╗ █████╗ ██╗   ██╗
██╔══██╗██║██╔═══██╗╚══██╔══╝    ██╔════╝ ██╔══██╗╚══██╔══╝██╔════╝██║    ██║██╔══██╗╚██╗ ██╔╝
██████╔╝██║██║   ██║   ██║       ██║  ███╗███████║   ██║   █████╗  ██║ █╗ ██║███████║ ╚████╔╝
██╔══██╗██║██║   ██║   ██║       ██║   ██║██╔══██║   ██║   ██╔══╝  ██║███╗██║██╔══██║  ╚██╔╝
██║  ██║██║╚██████╔╝   ██║       ╚██████╔╝██║  ██║   ██║   ███████╗╚███╔███╔╝██║  ██║   ██║
╚═╝  ╚═╝╚═╝ ╚═════╝    ╚═╝        ╚═════╝ ╚═╝  ╚═╝   ╚═╝   ╚══════╝ ╚══╝╚══╝ ╚═╝  ╚═╝   ╚═╝
                  Riot API Gateway — ProStaff Ecosystem
```

<div align="center">

[![Go Version](https://img.shields.io/badge/go-1.23-00ADD8?logo=go)](https://golang.org/)
[![Redis](https://img.shields.io/badge/Redis-7-DC382D?logo=redis)](https://redis.io/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker)](https://www.docker.com/)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

</div>

---

```
╔══════════════════════════════════════════════════════════════════════════════╗
║  PROSTAFF RIOT GATEWAY — Go 1.23                                             ║
╠══════════════════════════════════════════════════════════════════════════════╣
║  Centralized Riot Games API gateway for the ProStaff ecosystem.              ║
║  Token bucket rate limiting · Two-tier cache · Circuit breaker               ║
╚══════════════════════════════════════════════════════════════════════════════╝
```

---

<details>
<summary><kbd>▶ Features (click to expand)</kbd></summary>

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  [■] Global App Rate Limiting — Single token bucket for the API key         │
│  [■] Two-tier Cache           — L1 LRU in-process + L2 Redis                │
│  [■] Negative Cache           — 404s cached in L1 (short TTL per resource)  │
│  [■] Circuit Breaker          — Per (region, endpoint): match ≠ summoner    │
│  [■] Retry with Backoff       — 5xx retried 3× (0/100ms/500ms) before open  │ 
│  [■] Internal JWT Auth        — aud-validated; user tokens rejected         │
│  [■] Regional Routing         — Auto-resolves Match-V5 routing region       │
│  [■] Graceful Degradation     — Redis down? L1 cache keeps serving          │
│  [■] Request ID Propagation   — X-Request-ID for cross-service log correl.  │
│  [■] Build Info in /health    — version, commit, built_at via ldflags       │
│  [■] Structured JSON Logging  — slog JSON, compatible with log aggregators  │
│  [■] Graceful Shutdown        — 5s drain on SIGTERM                         │
│  [■] Docker Ready             — Multi-stage build, image < 20MB             │
└─────────────────────────────────────────────────────────────────────────────┘
```

</details>

---

## Table of Contents

```
┌──────────────────────────────────────────────────────┐
│  01 · Quick Start                                    │
│  02 · Technology Stack                               │
│  03 · Architecture                                   │
│  04 · API Endpoints                                  │
│  05 · Configuration                                  │
│  06 · Cache & Rate Limiting                          │
│  07 · Development                                    │
│  08 · Integration with ProStaff                      │
└──────────────────────────────────────────────────────┘
```

---

## 01 · Quick Start

```bash
# Copy env and fill in values
cp .env.example .env

# Start gateway + Redis
docker compose up -d

# Check health
curl http://localhost:4444/health
```

**Response:**
```json
{
  "status": "ok",
  "redis": "ok",
  "circuit_breakers": {}
}
```

---

## 02 · Technology Stack

```
╔══════════════════════╦════════════════════════════════════════════════════╗
║  LAYER               ║  TECHNOLOGY                                        ║
╠══════════════════════╬════════════════════════════════════════════════════╣
║  Language            ║  Go 1.23                                           ║
║  HTTP Router         ║  Gorilla Mux v1.8                                  ║
║  Authentication      ║  JWT HS256 (golang-jwt/jwt v5)                     ║
║  Rate Limiting       ║  golang.org/x/time/rate (token bucket)             ║
║  Cache L1            ║  hashicorp/golang-lru v2 (LRU + TTL + GC)          ║
║  Cache L2            ║  Redis 7 (go-redis/v9)                             ║
║  Circuit Breaker     ║  Custom 3-state state machine                      ║
║  Config              ║  godotenv + os.Getenv                              ║
║  Logging             ║  log/slog (JSON handler, stdlib Go 1.21+)          ║
║  Container           ║  Docker multi-stage (Alpine, image < 20MB)         ║
╚══════════════════════╩════════════════════════════════════════════════════╝
```

---

## 03 · Architecture

```
prostaff-front (vinext) ───────────────────────────┐
prostaff-mobile(vue.js\quasar+capacitor) ──────────┤
ArenaBR (NextJS)  ─────────────────────────────────┤
Scrims  (NextJS)  ─────────────────────────────────┤
                                  ┌────────────────┘  
                                  ▼
prostaff-api (Rails) ──────────────────────────────┐
ProStaff-Scraper (Python) ─────────────────────────┤
                                        ┌──────────┘
                                        ▼
                          [prostaff-riot-gateway :4444]
                         ┌──────────────────────┐
                         │  JWT InternalAuth    │  aud-validated service identity
                         │  AppLimiter (global) │  single token bucket per API key
                         │  RegionBreakers      │  circuit breaker per (region,endpoint)
                         │  MemoryCache (L1)    │  LRU, max 10k entries, negative cache
                         │  RedisCache  (L2)    │  shared across instances
                         └──────────────────────┘
                                     │
                                     ▼
                                Riot Games API
                           (br1, na1, euw1, kr, ...)
```

### Request Pipeline

```
1. Validate JWT              → 401 if missing, invalid, or wrong aud
2. Validate region           → 400 if not in allowed list
3. Check L1 negative cache   → 404 if resource was confirmed absent recently
4. Check L1 cache            → return in < 2ms if hit
5. Check L2 Redis            → populate L1, return if hit
6. Check circuit breaker     → 503 if (region, endpoint) circuit open
7. Acquire rate limiter      → blocks until token available (global)
8. Call Riot API             → 5s timeout, retry 5xx up to 3× with backoff
9. On success                → populate L1 + L2, return 200
10. On 404                   → cache negative in L1, return 404
11. On persistent failure    → trip circuit breaker, return 502
```

### Regional Routing (Match-V5)

Riot Match-V5 uses regional routing instead of server routing.
The gateway resolves automatically:

```
br1, na1, la1, la2  →  americas
euw1, eun1, tr1, ru →  europe
kr, jp1             →  asia
oc1                 →  sea
```

---

## 04 · API Endpoints

**Base URL:** `http://riot-gateway:4444`
**Auth header:** `Authorization: Bearer <internal-jwt>` (all `/riot/*` endpoints)

```
# Public
GET  /health

# Summoner / Account
GET  /riot/summoner/{region}/by-puuid/{puuid}
GET  /riot/summoner/{region}/by-riot-id/{gameName}/{tagLine}  ← preferred
GET  /riot/summoner/{region}/by-name/{name}                   ← 410 Gone (Riot deprecated 2024)
GET  /riot/account/{region}/{riotId}/{tagline}
GET  /riot/account/{region}/by-puuid/{puuid}

# Ranked
GET  /riot/league/{region}/by-summoner/{summonerId}
GET  /riot/league/{region}/by-puuid/{puuid}

# Matches (auto-resolves routing region)
GET  /riot/matches/{region}/{puuid}/ids?count=20&queue=420&start=0
GET  /riot/match/{region}/{matchId}

# Champion Mastery
GET  /riot/mastery/{region}/{puuid}/top?count=10
```

### Health Response

```json
{
  "status": "ok",
  "version": "v1.0.0",
  "commit": "abc1234",
  "built_at": "2026-04-20T01:00:00Z",
  "redis": "ok",
  "circuit_breakers": {
    "br1:summoner": "closed",
    "americas:match": "open"
  }
}
```

---

## 05 · Configuration

All configuration via environment variables (`.env`):

```bash
# Gateway
PORT=4444
# Must be different from prostaff-api user JWT secret — generate with: openssl rand -hex 32
# Tokens must include aud: "prostaff-riot-gateway"
INTERNAL_JWT_SECRET=<dedicated-gateway-secret>

# Riot API
RIOT_API_KEY=RGAPI-...
RIOT_API_TIMEOUT=5s

# Rate Limiting (Riot dev key: 20/s, 100/2min — single global bucket)
RIOT_RATE_LIMIT_PER_SECOND=20
RIOT_RATE_LIMIT_BURST=20
RIOT_RATE_LIMIT_PER_2MIN=100

# Cache L1 (in-process LRU)
CACHE_L1_MAX_SIZE=10000          # max entries before LRU eviction

# Cache L2 (Redis)
REDIS_URL=redis://redis:6379/1   # db 1, separate from prostaff-api db 0
CACHE_ENABLED=true

# Circuit Breaker
CIRCUIT_BREAKER_THRESHOLD=5      # consecutive failures to open circuit
CIRCUIT_BREAKER_TIMEOUT=60       # failure counting window (seconds)
CIRCUIT_BREAKER_COOLDOWN=30      # seconds before half-open probe

# Logging
LOG_LEVEL=info                   # debug | info | warn | error
```

---

## 06 · Cache TTLs

```
╔═══════════════════════╦══════════╦══════════╦══════════════╗
║  Resource             ║  L1 TTL  ║  L2 TTL  ║  404 (L1)    ║
╠═══════════════════════╬══════════╬══════════╬══════════════╣
║  summoner by riot-id  ║  10 min  ║  10 min  ║  30s         ║
║  summoner by PUUID    ║  10 min  ║  10 min  ║  2 min       ║
║  account (riot ID)    ║   1 h    ║   1 h    ║  2 min       ║
║  league entries       ║   5 min  ║   5 min  ║  30s         ║
║  match IDs list       ║   5 min  ║   5 min  ║  30s         ║
║  match detail         ║   1 h    ║   24 h   ║  5 min       ║
║  champion mastery     ║  30 min  ║   1 h    ║  30s         ║
╚═══════════════════════╩══════════╩══════════╩══════════════╝
```

Match detail has a 24h L2 TTL because match data is immutable once the game ends.
404s are cached only in L1 — they're short-lived and should not persist across instances.

---

## 07 · Development

```bash
# Build (Go not required locally, uses Docker)
docker run --rm -v $(pwd):/app -w /app golang:1.23-alpine go build ./cmd/server/

# Lint (staticcheck)
docker run --rm -v $(pwd):/app -w /app golang:1.23-alpine sh -c \
  "go install honnef.co/go/tools/cmd/staticcheck@latest 2>/dev/null && staticcheck ./..."

# go vet
docker run --rm -v $(pwd):/app -w /app golang:1.23-alpine go vet ./...

# Run tests
docker run --rm -v $(pwd):/app -w /app golang:1.23-alpine go test ./...

# Start with docker compose
docker compose up -d

# View logs
docker compose logs -f gateway

# Check health
curl http://localhost:4444/health
```

### Project Structure

```
prostaff-riot-gateway/
├── cmd/server/main.go          — entry point, router wiring, graceful shutdown
├── internal/
│   ├── auth/
│   │   ├── jwt.go              — ServiceClaims, ValidateServiceToken
│   │   └── middleware.go       — InternalAuth JWT middleware
│   ├── cache/
│   │   ├── memory.go           — L1: hashicorp/golang-lru + TTL + negative cache
│   │   ├── redis.go            — L2: go-redis/v9, graceful fallback
│   │   └── ttl.go              — TTL + negative TTL constants per resource type
│   ├── circuit/
│   │   └── breaker.go          — 3-state machine, RegionBreakers per (region,endpoint)
│   ├── config/
│   │   └── config.go           — typed config, godotenv + os.Getenv
│   ├── handlers/
│   │   ├── base.go             — shared fetch pipeline (neg cache→L1→L2→riot)
│   │   ├── health.go           — GET /health with version + circuit breaker states
│   │   ├── summoner.go         — summoner (by-riot-id, by-puuid) + account endpoints
│   │   ├── league.go           — ranked/league endpoints
│   │   ├── matches.go          — Match-V5 IDs + detail
│   │   └── mastery.go          — champion mastery
│   ├── middleware/
│   │   └── requestid.go        — X-Request-ID propagation (UUID v4 fallback)
│   ├── ratelimit/
│   │   └── limiter.go          — global AppLimiter (1s + 2min), region validation
│   ├── riot/
│   │   └── client.go           — HTTP client, retry backoff, circuit integration
│   └── webutils/
│       └── json_helpers.go     — WriteJSON, RawJSON, ErrorJSON
├── Dockerfile                  — multi-stage, Alpine final image
├── docker-compose.yml          — gateway + Redis
├── .env.example
└── go.mod
```

---

## 08 · Integration with ProStaff

### Adding the gateway to prostaff-api docker-compose

```yaml
# docker-compose.yml (prostaff-api)
riot-gateway:
  image: prostaff-riot-gateway:latest
  build:
    context: ../prostaff-riot-gateway
    dockerfile: Dockerfile
  ports:
    - "4444:4444"
  environment:
    - INTERNAL_JWT_SECRET=${RIOT_GATEWAY_JWT_SECRET}   # dedicated secret, not the user JWT
    - RIOT_API_KEY=${RIOT_API_KEY}
    - REDIS_URL=redis://redis:6379/1
    - PORT=4444
  depends_on:
    redis:
      condition: service_healthy
  networks:
    - prostaff_network
  restart: unless-stopped
```

### Migrating riot_api_service.rb

```ruby
# Before
BASE_URL = "https://#{region}.api.riotgames.com"
headers = { "X-Riot-Token" => ENV["RIOT_API_KEY"] }

# After
GATEWAY_URL = ENV.fetch("RIOT_GATEWAY_URL", "http://riot-gateway:4444")
headers = { "Authorization" => "Bearer #{internal_jwt}" }

def internal_jwt
  payload = {
    iss: "prostaff-api",
    aud: "prostaff-riot-gateway",
    sub: "service-account",
    service: "prostaff-api",
    exp: 1.hour.from_now.to_i
  }
  JWT.encode(payload, ENV.fetch("RIOT_GATEWAY_JWT_SECRET"), "HS256")
end
```

### Migrating ProStaff-Scraper (Python)

```python
GATEWAY_URL = os.getenv("RIOT_GATEWAY_URL", "http://riot-gateway:4444")
headers = {"Authorization": f"Bearer {generate_internal_jwt()}"}

response = requests.get(
    f"{GATEWAY_URL}/riot/match/americas/{match_id}",
    headers=headers,
    timeout=10
)

# generate_internal_jwt must include aud: "prostaff-riot-gateway"
```

---

**Last Updated**: 2026-04-20
**Go Version**: 1.23
**Cache Strategy**: L1 LRU (hashicorp/golang-lru) + L2 Redis + negative cache
**Rate Limit**: Global token bucket per API key (20 req/s / 100 req/2min, configurable)
