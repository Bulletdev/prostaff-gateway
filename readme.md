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
[![License](https://img.shields.io/badge/License-CC%20BY--NC--SA%204.0-lightgrey.svg)](http://creativecommons.org/licenses/by-nc-sa/4.0/)

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
│  [■] Global Rate Limiting     — Token bucket per region (x/time/rate)       │
│  [■] Two-tier Cache           — L1 in-process (sync.Map) + L2 Redis         │
│  [■] Circuit Breaker          — 3-state per region: closed/open/half-open   │
│  [■] Internal JWT Auth        — Only ProStaff services can call the gateway │
│  [■] Regional Routing         — Auto-resolves Match-V5 routing region       │
│  [■] Graceful Degradation     — Redis down? L1 cache keeps serving          │
│  [■] Structured JSON Logging  — slog JSON, compatible with log aggregators  │
│  [■] Health Endpoint          — Circuit breaker states + Redis connectivity │
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
║  Cache L1            ║  sync.Map in-process (TTL + GC goroutine)          ║
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
prostaff-api (Rails) ──────────┐
ProStaff-Scraper (Python) ─────┤
                               ▼
              [prostaff-riot-gateway :4444]
                   ┌──────────────────────┐
                   │  JWT InternalAuth    │  validates service identity
                   │  RegionLimiter       │  token bucket per region
                   │  RegionBreakers      │  circuit breaker per region
                   │  MemoryCache (L1)    │  ~0ms hits, GC every 60s
                   │  RedisCache  (L2)    │  shared across instances
                   └──────────────────────┘
                               │
                               ▼
                        Riot Games API
                   (br1, na1, euw1, kr, ...)
```

### Request Pipeline

```
1. Validate JWT          → 401 if missing or invalid
2. Validate region       → 400 if not in allowed list
3. Check L1 cache        → return in < 2ms if hit
4. Check L2 Redis        → populate L1, return if hit
5. Check circuit breaker → 503 if region circuit open
6. Acquire rate limiter  → blocks until token available
7. Call Riot API         → 5s timeout
8. On success            → populate L1 + L2, return 200
9. On failure            → increment breaker, return mapped error
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
GET  /riot/summoner/{region}/by-name/{name}
GET  /riot/account/{region}/{riotId}/{tagline}

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
  "redis": "ok",
  "circuit_breakers": {
    "br1": "closed",
    "euw1": "open"
  }
}
```

---

## 05 · Configuration

All configuration via environment variables (`.env`):

```bash
# Gateway
PORT=4444
INTERNAL_JWT_SECRET=<same as prostaff-api JWT_SECRET_KEY>

# Riot API
RIOT_API_KEY=RGAPI-...
RIOT_API_TIMEOUT=5s

# Rate Limiting (Riot dev key: 20/s, 100/2min)
RIOT_RATE_LIMIT_PER_SECOND=20
RIOT_RATE_LIMIT_BURST=20

# Cache L2
REDIS_URL=redis://redis:6379/1   # db 1, separate from prostaff-api db 0
CACHE_ENABLED=true

# Circuit Breaker
CIRCUIT_BREAKER_THRESHOLD=5      # failures to open circuit
CIRCUIT_BREAKER_TIMEOUT=60       # failure counting window (seconds)
CIRCUIT_BREAKER_COOLDOWN=30      # seconds before half-open probe

# Logging
LOG_LEVEL=info                   # debug | info | warn | error
```

---

## 06 · Cache TTLs

```
╔═══════════════════════╦══════════╦══════════╗
║  Resource             ║  L1 TTL  ║  L2 TTL  ║
╠═══════════════════════╬══════════╬══════════╣
║  summoner by PUUID    ║  10 min  ║  10 min  ║
║  summoner by name     ║   5 min  ║   5 min  ║
║  account (riot ID)    ║   1 h    ║   1 h    ║
║  league entries       ║   5 min  ║   5 min  ║
║  match IDs list       ║   5 min  ║   5 min  ║
║  match detail         ║   1 h    ║  24 h    ║
║  champion mastery     ║  30 min  ║   1 h    ║
╚═══════════════════════╩══════════╩══════════╝
```

Match detail has a 24h L2 TTL because match data is immutable once the game ends.

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
│   │   ├── memory.go           — L1: sync.Map + TTL + GC goroutine
│   │   ├── redis.go            — L2: go-redis/v9, graceful fallback
│   │   └── ttl.go              — TTL constants per resource type
│   ├── circuit/
│   │   └── breaker.go          — 3-state machine, RegionBreakers
│   ├── config/
│   │   └── config.go           — typed config, godotenv + os.Getenv
│   ├── handlers/
│   │   ├── base.go             — shared fetch pipeline (cache→riot→cache)
│   │   ├── health.go           — GET /health
│   │   ├── summoner.go         — summoner + account endpoints
│   │   ├── league.go           — ranked/league endpoints
│   │   ├── matches.go          — Match-V5 IDs + detail
│   │   └── mastery.go          — champion mastery
│   ├── ratelimit/
│   │   └── limiter.go          — token bucket per region, region validation
│   ├── riot/
│   │   └── client.go           — HTTP client, rate limit + circuit integration
│   └── webutils/
│       └── json_helpers.go     — WriteJSON, RawJSON, ErrorJSON
├── dockerfile                  — multi-stage, Alpine final image
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
    dockerfile: dockerfile
  ports:
    - "4444:4444"
  environment:
    - INTERNAL_JWT_SECRET=${JWT_SECRET_KEY}
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
  payload = { service: "prostaff-api", exp: 1.hour.from_now.to_i }
  JWT.encode(payload, ENV.fetch("JWT_SECRET_KEY"), "HS256")
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
```

---

**Last Updated**: 2026-04-17
**Go Version**: 1.23
**Cache Strategy**: L1 in-process + L2 Redis
**Rate Limit**: 20 req/s per region (configurable)
