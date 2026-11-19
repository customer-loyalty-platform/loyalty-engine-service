# ⚡ Loyalty Engine Service (Go)

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://golang.org/)
[![Gin](https://img.shields.io/badge/Gin-v1.9.1-008ECF?style=flat-square&logo=gin&logoColor=white)](https://gin-gonic.com/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-4169E1?style=flat-square&logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![pgx](https://img.shields.io/badge/pgx-v5-336791?style=flat-square&logo=postgresql&logoColor=white)](https://github.com/jackc/pgx)
[![Zap](https://img.shields.io/badge/Zap-Logger-yellow?style=flat-square)](https://github.com/uber-go/zap)

High-performance, fault-tolerant transaction engine for the **Enterprise General Loyalty & Rewards Platform**. Written in Go using Hexagonal / Clean Architecture, this service powers customer-facing web-views, mobile applications, and partner integrations for real-time points accrual (**Earning**), voucher redemption (**Burning**), wallet balances, and member profile management.

---

> ⚠️ **Architecture Notice: Decoupled Service Architecture**  
> `loyalty-engine-service` is an autonomous microservice that operates independently from the administrative backend (`backoffice-service`) and web portal (`backoffice`):
>
> - **`loyalty-engine-service` (Port 8081 - This Service):** Executes customer-facing transaction workloads, points ledger double-entry bookkeeping, rule evaluation, balance queries, and voucher claim locks.
> - **`backoffice-service` (Port 8080 - Separate Backend):** Governs back-office administrative operations, dynamic rule creation, tier configurations, maker-checker financial controls, and audit trails.
> - **Direct PostgreSQL Integration (Zero Redis Dependency):** All transaction atomicity, point deduplication, and catalog queries execute directly against partitioned PostgreSQL databases utilizing Master (Read/Write) and Read-Replica connection pools.

---

## 📑 Table of Contents

- [Business Process & High-Level Flows](#-business-process--high-level-flows)
  - [1. End-to-End System Context](#1-end-to-end-system-context)
  - [2. Points Earning Flow (Accrual Engine)](#2-points-earning-flow-accrual-engine)
  - [3. Points Burning Flow (Voucher Redemption)](#3-points-burning-flow-voucher-redemption)
  - [4. Balance, Ledger & Point Expiration (FIFO)](#4-balance-ledger--point-expiration-fifo)
  - [5. Public Member Onboarding & Catalog Discovery](#5-public-member-onboarding--catalog-discovery)
- [Technical Architecture](#-technical-architecture)
  - [Database Schemas & Partitioning](#database-schemas--partitioning)
  - [High-Throughput Concurrency & Idempotency](#high-throughput-concurrency--idempotency)
- [Project Structure](#-project-structure)
- [API Reference](#-api-reference)
- [Configuration & Environment](#-configuration--environment)
- [Getting Started](#-getting-started)
  - [Build and Run](#build-and-run)
  - [Run Migrations](#run-migrations)
  - [Run Tests](#run-tests)
- [Docker Deployment](#-docker-deployment)

---

## 🔄 Business Process & High-Level Flows

### 1. End-to-End System Context

```text
┌────────────────────────────────────────────────────────────────────────┐
│                   Customer Channels (Web-View / App / POS)            │
└────────────────────────────────────┬───────────────────────────────────┘
                                     │
           HTTPS REST API (:8081)    │ [Customer Bearer JWT + Idempotency-Key]
                                     ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        loyalty-engine-service                          │
│                                                                        │
│   ┌────────────────────┐   ┌────────────────────┐   ┌──────────────┐   │
│   │   Balance Module   │   │   Catalog Module   │   │ Earning/Burn │   │
│   │ (Profile & Ledger) │   │ (Voucher Discovery)│   │ Engine Core  │   │
│   └─────────┬──────────┘   └─────────┬──────────┘   └───────┬──────┘   │
└─────────────┼────────────────────────┼──────────────────────┼──────────┘
              │                        │                      │
              │ Master / Replica       │ Read-Replica         │ ACID Trans.
              ▼                        ▼                      ▼
┌────────────────────────────────────────────────────────────────────────┐
│                           PostgreSQL Database                          │
│                                                                        │
│  ├─ schema_ledger   : members, point_ledgers (Monthly Range Partition) │
│  ├─ schema_earning  : earning_transactions, daily_caps, campaign_state │
│  ├─ schema_burning  : voucher_batches, vouchers (Lock pool)            │
│  └─ schema_backoffice: dynamic_rules, tier_definitions (Config sync)   │
└────────────────────────────────────────────────────────────────────────┘
```

---

### 2. Points Earning Flow (Accrual Engine)

When a customer completes a qualifying action (purchase, review, referral, or mission):

```text
[ Client / POS / Webview ]
           │
           │ POST /api/v1/earn
           │ Headers: Authorization: Bearer <Customer_JWT>
           │          X-Idempotency-Key: <UUID>
           ▼
[ Idempotency Middleware ]
           │ Checks if request already executed; returns cached response if duplicate
           ▼
[ Rule Evaluator & Multiplier ]
           │ 1. Fetches active rules from `schema_backoffice.dynamic_rules`
           │ 2. Evaluates conditional JSON expressions (Merchant, Category, Min Spend)
           │ 3. Applies Tier Multiplier (e.g. Platinum 1.5x, Gold 1.2x)
           │ 4. Verifies daily/monthly cap thresholds (`schema_earning.earning_daily_caps`)
           ▼
[ PostgreSQL Atomic Transaction (Master) ]
           │
           ├─ 1. INSERT INTO schema_earning.earning_transactions (Partitioned)
           ├─ 2. INSERT INTO schema_ledger.point_ledgers (Type: 'EARN', +Points, Partitioned)
           ├─ 3. UPDATE schema_ledger.members (point_balance += EarnedPoints, lifetime_points += EarnedPoints)
           └─ 4. Check for Tier Upgrade thresholds (SPEND / POINTS)
           ▼
[ Response 200 OK ] ──▶ Returns transaction ID, points earned, and updated balance
```

---

### 3. Points Burning Flow (Voucher Redemption)

When a customer redeems accumulated points for a merchant or partner voucher:

```text
[ Member Web-View ] ──▶ GET /api/v1/catalog/vouchers (Browses available voucher batches)
           │
           ▼
[ Click Redeem ] ──▶ POST /api/v1/redeem/voucher
                     Headers: X-Idempotency-Key: <UUID>
                     Body: { "batch_code": "VOUCH-DISC-50K" }
           │
           ▼
[ Atomic Double-Entry & Inventory Lock Transaction ]
           │
           ├─ 1. SELECT point_balance FROM schema_ledger.members WHERE id = $1 FOR UPDATE;
           │     * Validates: point_balance >= voucher_points_cost
           │
           ├─ 2. SELECT id, voucher_code FROM schema_burning.vouchers
           │     WHERE batch_id = $2 AND status = 'AVAILABLE'
           │     LIMIT 1 FOR UPDATE SKIP LOCKED;
           │     * Ensures zero race conditions under high concurrent demand
           │
           ├─ 3. UPDATE schema_burning.vouchers
           │     SET status = 'CLAIMED', claimed_by_member_id = $1, claimed_at = NOW();
           │
           ├─ 4. INSERT INTO schema_ledger.point_ledgers (Type: 'REDEEM', -Points);
           │
           └─ 5. UPDATE schema_ledger.members SET point_balance -= Cost;
           ▼
[ Response 200 OK ] ──▶ Returns decrypted voucher code, expiration date, and remaining balance
```

---

### 4. Balance, Ledger & Point Expiration (FIFO)

- **Total vs Usable Balance:** The engine maintains an instant aggregate of active points while enforcing a FIFO (First-In, First-Out) ledger model.
- **Expiring Points Telemetry:** The `GET /api/v1/balance/expiring` endpoint forecasts points slated to expire within the next 30 to 90 days, enabling timely gamified push reminders.
- **Audit-Proof Double Entry:** Point mutations are never directly overwritten; every credit, debit, or reversal creates an immutable row in the partitioned `point_ledgers` table.

---

### 5. Public Member Onboarding & Catalog Discovery

- **Registration (`POST /api/v1/member/register`):** Creates member identity in `schema_ledger.members`, assigns default baseline tier (e.g., Bronze/Classic), and credits welcome rewards if configured.
- **Catalog Browsing (`GET /api/v1/catalog/vouchers`):** Read-only, highly optimized catalog queries executed against PostgreSQL read-replicas, supporting pagination, category filtering, and point affordability indicators.

---

## 🛠️ Technical Architecture

### Database Schemas & Partitioning

| Schema | Tables Handled | Description & Strategy |
|---|---|---|
| `schema_earning` | `earning_transactions`, `earning_rules_cache`, `earning_daily_caps` | Core accrual tracking; `earning_transactions` is range-partitioned by month. |
| `schema_ledger` | `members`, `point_ledgers`, `point_expiration_buckets` | Double-entry accounting; `point_ledgers` is range-partitioned by transaction date. |
| `schema_burning` | `voucher_batches`, `vouchers` | Catalog definitions and serialized voucher code pool with row-level reservation locks. |
| `schema_backoffice` | `dynamic_rules`, `tier_definitions` | Synchronized configuration data authored via the BackOffice Command Center. |

### High-Throughput Concurrency & Idempotency

- **Pessimistic Concurrency (`FOR UPDATE SKIP LOCKED`):** Prevents inventory double-spending on limited-quantity voucher flash drops without distributed locks.
- **Strict Idempotency:** State-changing endpoints (`POST /earn`, `POST /redeem/voucher`) enforce client-supplied `X-Idempotency-Key` headers to guarantee zero duplicate point grants during network retries.
- **Master/Replica Segregation:** Write mutations flow through `DB_MASTER_DSN`, while telemetry, catalog browsing, and ledger history query `DB_REPLICA_DSN`.

---

## 📁 Project Structure

```text
loyalty-engine-service/
├── cmd/
│   └── server/
│       └── main.go                  # Bootstrap, DI container, and graceful shutdown
├── config/
│   ├── config.go                    # Viper loader & environment binding
│   └── config.yaml                  # Default application settings
├── internal/
│   ├── delivery/
│   │   └── http/
│   │       ├── response.go          # Standardized JSON response envelope
│   │       ├── router.go            # Route registration & middleware chaining
│   │       └── router_test.go       # Route integration tests
│   ├── domain/
│   │   ├── errors.go                # Centralized business domain errors
│   │   └── pagination.go            # Pagination models & cursor helpers
│   ├── middleware/
│   │   ├── auth.go                  # Customer JWT token validator
│   │   ├── idempotency.go           # X-Idempotency-Key enforcement
│   │   └── recovery.go              # Panic recovery with Zap structured logging
│   ├── modules/
│   │   ├── balance/                 # Member profile, points balance & ledger history
│   │   │   ├── handler/
│   │   │   ├── model/
│   │   │   ├── repository/
│   │   │   └── service/
│   │   └── catalog/                 # Read-only voucher catalog discovery
│   │       ├── handler/
│   │       ├── model/
│   │       ├── repository/
│   │       └── service/
│   └── pkg/
│       ├── database/                # pgxpool PostgreSQL Master / Replica manager
│       ├── jwt/                     # Customer JWT verification (HS256)
│       ├── logger/                  # Uber Zap structured logger
│       └── response/                # Shared response models (prevents import cycles)
├── migrations/                      # Partitioned schema migrations
│   ├── 000001_init_earning_schema.up.sql
│   └── 000002_init_partition_tables.up.sql
├── Dockerfile                       # Minimal scratch multi-stage build (<25MB)
├── Makefile                         # Automation scripts (build, run, test)
├── go.mod
└── go.sum
```

---

## 📡 API Reference

Base Path: `/api/v1`

### Health & Liveness
- `GET /health` — Liveness probe (Returns `200 UP`)
- `GET /health/ready` — Readiness probe (Pings PostgreSQL connection pool)

### Public Endpoints
- `POST /api/v1/member/register` — Onboard new loyalty member
- `GET /api/v1/catalog/vouchers` — Discover active voucher batches (supports `?page=`, `?page_size=`, `?category=`)
- `GET /api/v1/catalog/vouchers/:batch_code` — Get voucher batch details & availability

### Authenticated Member Endpoints (`Authorization: Bearer <Customer_JWT>`)
- `GET /api/v1/balance` — Get real-time point balance, tier rank, and status
- `GET /api/v1/balance/transactions` — Paginated member point ledger history
- `GET /api/v1/balance/expiring` — Point expiration forecasting within 30-90 days
- `GET /api/v1/member/profile` — Full member profile with tier benefits

### State-Changing Endpoints (`Requires X-Idempotency-Key`)
- `POST /api/v1/earn` — Process earning event (Purchase, Review, Mission)
- `POST /api/v1/redeem/voucher` — Redeem points for a voucher code

---

## ⚙️ Configuration & Environment

Configuration is managed via Viper and can be overridden via environment variables or `.env`:

| Variable | Default Value | Description |
|---|---|---|
| `APP_PORT` | `8081` | HTTP server port |
| `APP_ENV` | `development` | Environment mode (`development` / `production`) |
| `DATABASE_MASTER_DSN` | `postgres://postgres:postgres@localhost:5432/loyalty_databases?sslmode=disable` | Master PostgreSQL DSN (RW) |
| `DATABASE_REPLICA_DSN` | `postgres://postgres:postgres@localhost:5432/loyalty_databases?sslmode=disable` | Read-Replica PostgreSQL DSN (RO) |
| `DATABASE_MAX_CONNS` | `50` | Maximum connections per pool |
| `DATABASE_MIN_CONNS` | `5` | Minimum idle connections |
| `JWT_SECRET` | `loyalty-engine-super-secret-key-change-in-production` | Secret key for customer JWT validation |
| `JWT_SIGNING_METHOD` | `HS256` | JWT signing algorithm |

---

## 🚀 Getting Started

### Prerequisites
- **Go**: Version `1.21+`
- **PostgreSQL**: Version `15+`

### Build and Run

```bash
# Navigate to service directory
cd loyalty-engine-service

# Run server locally
go run ./cmd/server/main.go

# Or build binary
go build -o bin/loyalty-engine-service.exe ./cmd/server/main.go
./bin/loyalty-engine-service.exe
```

The server will initialize on port `8081`.

### Run Tests

```bash
go test ./... -v
```

---

## 🐳 Docker Deployment

A lightweight, multi-stage Dockerfile is provided:

```bash
# Build Docker image
docker build -t loyalty-engine-service:latest .

# Run container
docker run -p 8081:8081 --env-file .env loyalty-engine-service:latest
```

