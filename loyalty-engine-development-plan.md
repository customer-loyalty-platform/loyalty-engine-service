# Development Plan: `loyalty-engine-service`

> **Versi Dokumen:** 1.0.0  
> **Dibuat:** 2026-11-18  
> **Scope:** Backend Go service untuk Earning & Burning point, digunakan oleh end-user / web-view client  
> **Status:** Draft — Siap untuk Review & Eksekusi

---

## 1. Gambaran Umum Service

### 1.1 Identitas Service

| Atribut | Nilai |
|---|---|
| **Nama Service** | `loyalty-engine-service` |
| **Go Module** | `loyalty-engine-service` |
| **Port Default** | `:8081` |
| **Consumer Utama** | End-user via `client-webview-service` (BFF) atau langsung dari Web View app |
| **Primary Schema DB** | `schema_earning`, `schema_burning` |
| **Read dari** | `schema_ledger`, `schema_backoffice` (rules & tiers — read only) |
| **Pattern** | Modular Monolith, Clean/Hexagonal Architecture |

### 1.2 Tanggung Jawab Service

Service ini bertanggung jawab **penuh** atas:

1. **Earning Engine** — Menerima inbound transaction event, mengevaluasi dynamic rules (dari `schema_backoffice.dynamic_rules`), menghitung poin yang diperoleh, menulis ke ledger
2. **Burning Engine** — Memproses redemption poin untuk voucher (atomic `FOR UPDATE SKIP LOCKED`)
3. **Member Balance & Ledger** — Membaca saldo real-time, riwayat transaksi poin
4. **Point Expiry** — Manajemen bucket expiry (FIFO), eksekusi batch expiry job
5. **Earning Reversal** — Void/reversal earning transaksi (refund, chargeback)
6. **Catalog Read** — Membaca katalog voucher yang tersedia untuk end-user

### 1.3 Yang BUKAN Tanggung Jawab Service Ini

- ❌ Manajemen admin / RBAC (→ `backoffice-service`)
- ❌ Konfigurasi earning rules, campaigns, tiers (→ `backoffice-service`)
- ❌ Settlement laporan keuangan partner (→ `backoffice-service`)
- ❌ Maker-checker workflow (→ `backoffice-service`)

---

## 2. Arsitektur & Pola Design

### 2.1 Layer Stack

```
┌─────────────────────────────────────────────────────────┐
│              HTTP / REST Delivery Layer                  │
│       (Gin Router, DTO Binding, Response Envelope)      │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│              Middleware Pipeline                         │
│  Customer JWT Auth │ Rate Limiter │ Idempotency Guard    │
│  Request ID Injector │ Recovery/Panic Handler           │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│              Application Service Layer                   │
│   Use Cases: Earn, Redeem, Balance, Expiry, Reversal    │
│   Rule Engine Evaluator │ Cap Enforcement               │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│              Domain Interface Layer                      │
│   Domain Entities │ Repository Interfaces │ Errors      │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│              Infrastructure / Data Layer                 │
│  pgxpool Master (Write) │ pgxpool Replica (Read)        │
│  Outbox Writer │ Background Job Workers                 │
└─────────────────────────────────────────────────────────┘
```

### 2.2 Interaksi Antar Service / Schema

```
[Web View App / BFF]
        │  HTTP REST (Bearer JWT - Customer Scope)
        ▼
[loyalty-engine-service :8081]
        │                    │
        │  READ ONLY         │  READ/WRITE
        ▼                    ▼
[schema_backoffice]    [schema_earning]
  dynamic_rules          earning_transactions
  tier_definitions       earning_rule_results
  campaigns              point_expiry_buckets
  partners               earning_cap_counters
                         earning_reversals
        │
        │  READ/WRITE
        ▼
[schema_ledger]
  members
  member_balances
  point_ledgers (partitioned)
        │
        │  READ/WRITE
        ▼
[schema_burning]
  voucher_batches
  voucher_inventories
        │
        │  ASYNC
        ▼
[schema_outbox]
  event_outbox
```

---

## 3. Struktur Direktori

```text
loyalty-engine-service/
├── cmd/
│   └── server/
│       └── main.go                    # Bootstrap, DI, graceful shutdown
├── config/
│   ├── config.go                      # Viper config parser (ENV)
│   └── config.yaml                    # Template konfigurasi lokal
├── internal/
│   ├── delivery/
│   │   └── http/
│   │       ├── router.go              # Gin engine setup & route grouping
│   │       └── response.go            # StandardResponse envelope
│   ├── middleware/
│   │   ├── auth.go                    # Customer JWT parser & member_id extractor
│   │   ├── idempotency.go             # X-Idempotency-Key guard & caching
│   │   ├── rate_limiter.go            # Per-member rate limiting (sliding window)
│   │   └── recovery.go               # Global panic recovery & structured log
│   ├── domain/
│   │   ├── errors.go                  # Sentinel errors
│   │   └── pagination.go             # Pagination structs
│   ├── engine/
│   │   ├── rule_evaluator.go          # Dynamic JSON rule evaluation engine
│   │   ├── point_calculator.go        # Multiplier formula: base × tier × campaign × time
│   │   └── cap_enforcer.go            # Earning cap check & counter update
│   ├── modules/
│   │   ├── earning/                   # ★ CORE: Proses earning transaksi
│   │   │   ├── handler/handler.go
│   │   │   ├── service/service.go
│   │   │   ├── repository/repository.go
│   │   │   └── model/model.go
│   │   ├── burning/                   # ★ CORE: Proses redemption voucher
│   │   │   ├── handler/handler.go
│   │   │   ├── service/service.go
│   │   │   ├── repository/repository.go
│   │   │   └── model/model.go
│   │   ├── balance/                   # Read saldo & riwayat poin member
│   │   │   ├── handler/handler.go
│   │   │   ├── service/service.go
│   │   │   ├── repository/repository.go
│   │   │   └── model/model.go
│   │   ├── catalog/                   # Read voucher catalog untuk end-user
│   │   │   ├── handler/handler.go
│   │   │   ├── service/service.go
│   │   │   ├── repository/repository.go
│   │   │   └── model/model.go
│   │   ├── expiry/                    # Expiry bucket management & batch job
│   │   │   ├── handler/handler.go     # (GET expiring points info)
│   │   │   ├── service/service.go
│   │   │   ├── repository/repository.go
│   │   │   ├── model/model.go
│   │   │   └── worker/expiry_worker.go  # Background job
│   │   └── reversal/                  # Earning reversal (refund/chargeback)
│   │       ├── handler/handler.go
│   │       ├── service/service.go
│   │       ├── repository/repository.go
│   │       └── model/model.go
│   ├── workers/
│   │   ├── outbox_worker.go           # Transactional Outbox dispatcher
│   │   └── expiry_batch_worker.go     # Scheduled point expiry executor
│   └── pkg/
│       ├── database/
│       │   └── postgres.go            # pgxpool Master + Replica setup
│       ├── logger/
│       │   └── logger.go              # Uber Zap structured logger
│       └── jwt/
│           └── customer_jwt.go        # Customer JWT claims parser
├── migrations/
│   ├── 000001_init_earning_schema.up.sql
│   ├── 000001_init_earning_schema.down.sql
│   ├── 000002_init_partition_tables.up.sql
│   └── 000002_seed_data.up.sql
├── Makefile
├── Dockerfile
├── go.mod
└── go.sum
```

---

## 4. API Endpoint Design (End-User / Web View)

Semua endpoint menggunakan prefix `/api/v1`. Auth: `Bearer <Customer JWT>`.

### 4.1 Earning

| Method | Path | Deskripsi | Idempotent |
|--------|------|-----------|------------|
| `POST` | `/earn` | Submit earning transaction event | ✅ (`X-Idempotency-Key`) |
| `GET` | `/earn/history` | Riwayat earning transaksi member | — |
| `GET` | `/earn/transactions/:id` | Detail earning transaksi | — |

### 4.2 Burning (Redemption)

| Method | Path | Deskripsi | Idempotent |
|--------|------|-----------|------------|
| `POST` | `/redeem/voucher` | Redeem poin untuk voucher | ✅ (`X-Idempotency-Key`) |
| `GET` | `/redeem/history` | Riwayat redemption member | — |

### 4.3 Balance & Ledger

| Method | Path | Deskripsi |
|--------|------|-----------|
| `GET` | `/balance` | Saldo poin real-time member |
| `GET` | `/balance/transactions` | Ledger poin (paginated) |
| `GET` | `/balance/expiring` | Poin yang akan expire (bucket view) |

### 4.4 Catalog

| Method | Path | Deskripsi |
|--------|------|-----------|
| `GET` | `/catalog/vouchers` | List voucher tersedia (paginated, filter) |
| `GET` | `/catalog/vouchers/:batch_code` | Detail voucher catalog |

### 4.5 Member Utility

| Method | Path | Deskripsi |
|--------|------|-----------|
| `GET` | `/member/profile` | Info tier & status member |
| `POST` | `/member/register` | Registrasi member baru (earning trigger: REGISTRATION) |

### 4.6 Health

| Method | Path | Deskripsi |
|--------|------|-----------|
| `GET` | `/health` | Health check (no auth) |
| `GET` | `/health/ready` | Readiness probe (DB ping) |

---

## 5. Core Business Logic

### 5.1 Earning Flow (Paling Kompleks)

```
POST /api/v1/earn
       │
       ├─ [1] Validasi JWT → extract member_id
       ├─ [2] Idempotency Guard: cek idx_earning_txn_idempotency
       │       └─ Jika duplikat → return 200 OK dengan hasil lama
       │
       ├─ [3] Load member dari schema_ledger.members
       │       └─ Validasi status = 'ACTIVE'
       │
       ├─ [4] Load tier aktif dari schema_backoffice.tier_definitions
       │       └─ Extract tier_multiplier
       │
       ├─ [5] Load active dynamic rules (sorted by priority ASC)
       │       dari schema_backoffice.dynamic_rules WHERE status='ACTIVE'
       │
       ├─ [6] Rule Evaluator: evaluasi setiap rule terhadap payload
       │       Loop setiap rule:
       │       ├─ Cek conditions_json (AND/OR logic) vs request metadata
       │       ├─ Hitung: base_points = transaction_amount * base_rate
       │       ├─ Apply: tier_multiplier (dari tier)
       │       ├─ Apply: campaign_multiplier (jika ada aktif campaign)
       │       ├─ Apply: time_multiplier (jika off-peak window match)
       │       ├─ Apply: bonus_points (flat bonus dari action_json)
       │       ├─ calculated_points = (base × tier × campaign × time) + bonus
       │       └─ Apply cap: cek earning_caps & cap_counters
       │
       ├─ [7] BEGIN TRANSACTION:
       │       a. INSERT earning_transactions (status=PROCESSING)
       │       b. INSERT earning_rule_results (satu row per rule yang match)
       │       c. INSERT point_expiry_buckets (FIFO bucket, expires_at dihitung)
       │       d. UPDATE earning_cap_counters (upsert, atomic counter++)
       │       e. CALL ledger write:
       │           UPDATE schema_ledger.member_balances
       │               SET total_points = total_points + final_points,
       │                   version = version + 1
       │               WHERE member_id = $1 AND version = $current_version
       │           → Jika 0 rows affected: Optimistic Lock conflict → retry/abort
       │           INSERT schema_ledger.point_ledgers
       │               (member_id, type='EARN', amount, balance_after, reference_id, idempotency_key)
       │       f. UPDATE earning_transactions SET status='COMPLETED'
       │       g. UPSERT earning_daily_summary
       │       h. INSERT schema_outbox.event_outbox (POINTS_EARNED event)
       │   COMMIT
       │
       └─ [8] Return 201 Created
              { earned_points, final_balance, rule_applied, expires_at }
```

### 5.2 Burning Flow (Voucher Redemption)

```
POST /api/v1/redeem/voucher
       │
       ├─ [1] Validasi JWT → member_id
       ├─ [2] Idempotency guard
       ├─ [3] Load & validasi member (ACTIVE)
       │
       ├─ [4] Load voucher_batch → validasi:
       │       - available_stock > 0
       │       - member punya cukup poin (total_points - locked_points >= points_cost)
       │
       ├─ [5] BEGIN TRANSACTION:
       │       a. SELECT id FROM schema_burning.voucher_inventories
       │              WHERE batch_id = $1 AND status = 'AVAILABLE'
       │              LIMIT 1 FOR UPDATE SKIP LOCKED
       │          → Jika 0 rows: ROLLBACK → 409 "Out of Stock"
       │
       │       b. DEDUCT POINTS di schema_ledger:
       │           UPDATE member_balances
       │               SET total_points = total_points - points_cost,
       │                   version = version + 1
       │               WHERE member_id = $1 AND version = $v AND total_points >= points_cost
       │           → Jika 0 rows: ROLLBACK → 422 "Insufficient Points"
       │           INSERT point_ledgers (type='REDEEM', amount=-points_cost)
       │
       │       c. CLAIM voucher:
       │           UPDATE voucher_inventories
       │               SET status='CLAIMED', claimed_by=$member, claimed_at=NOW()
       │               WHERE id = $selected_voucher_id
       │           UPDATE voucher_batches
       │               SET available_stock = available_stock - 1
       │               WHERE id = $1
       │
       │       d. UPDATE point_expiry_buckets (FIFO consumption)
       │       e. INSERT schema_outbox.event_outbox (VOUCHER_REDEEMED event)
       │   COMMIT
       │
       └─ [6] Return 200 OK { voucher_code, redeemed_at, remaining_balance }
```

### 5.3 Point Expiry Batch Worker

```
Scheduler: Runs every 1 hour (configurable via ENV)
       │
       ├─ Query: SELECT * FROM schema_earning.point_expiry_buckets
       │             WHERE expires_at <= NOW()
       │               AND status IN ('ACTIVE', 'PARTIALLY_CONSUMED')
       │             LIMIT 500
       │             FOR UPDATE SKIP LOCKED
       │
       ├─ Per bucket:
       │   BEGIN TRANSACTION:
       │   a. INSERT point_ledgers (type='EXPIRATION', amount=-remaining_points)
       │   b. UPDATE member_balances (deduct remaining_points)
       │   c. UPDATE point_expiry_buckets SET status='EXPIRED', expired_at=NOW()
       │   d. INSERT outbox (POINTS_EXPIRED event)
       │   COMMIT
       │
       └─ Sleep → Next iteration
```

### 5.4 Dynamic Rule Evaluator Logic

```go
// Pseudo-code evaluasi rule
for _, rule := range activeRules {
    if !isWithinTimeWindow(rule) { continue }
    if !evaluateConditions(rule.ConditionsJSON, txPayload) { continue }

    base := txPayload.Amount * baseRate
    pts  := base * tierMultiplier * campaignMultiplier * timeMultiplier
    pts += rule.ActionJSON.BonusPoints

    if rule.ActionJSON.MaxPointsCap != nil {
        pts = min(pts, *rule.ActionJSON.MaxPointsCap)
    }

    results = append(results, RuleResult{...})
}

totalPoints = sum(results.FinalPoints)
```

---

## 6. Response Envelope

Sama dengan `backoffice-service` untuk konsistensi:

```json
{
  "status": "SUCCESS",
  "timestamp": "2026-11-18T15:00:00Z",
  "data": { ... },
  "pagination": null
}
```

Error response:

```json
{
  "status": "ERROR",
  "timestamp": "2026-11-18T15:00:00Z",
  "error": {
    "code": "INSUFFICIENT_POINTS",
    "message": "Poin tidak mencukupi untuk melakukan redeem",
    "details": {}
  }
}
```

---

## 7. Error Code Catalog

| HTTP Status | Error Code | Trigger |
|-------------|-----------|---------|
| 400 | `INVALID_REQUEST` | Validasi body gagal |
| 400 | `INVALID_IDEMPOTENCY_KEY` | Header X-Idempotency-Key tidak ada / format salah |
| 401 | `UNAUTHORIZED` | JWT tidak valid / expired |
| 404 | `MEMBER_NOT_FOUND` | Member tidak ditemukan |
| 409 | `DUPLICATE_TRANSACTION` | Idempotency key sudah diproses |
| 409 | `OUT_OF_STOCK` | Voucher habis |
| 409 | `OPTIMISTIC_LOCK_CONFLICT` | Race condition pada balance update |
| 422 | `MEMBER_SUSPENDED` | Status member bukan ACTIVE |
| 422 | `INSUFFICIENT_POINTS` | Saldo tidak cukup untuk redeem |
| 422 | `EARNING_CAP_EXCEEDED` | Melebihi batas earning harian/bulanan |
| 429 | `RATE_LIMIT_EXCEEDED` | Terlalu banyak request dari member |
| 500 | `INTERNAL_ERROR` | Unexpected server error |

---

## 8. Middleware Design

### 8.1 Customer JWT Auth Middleware

```
Request → Extract Bearer token → Parse JWT
       → Validate signature (RSA public key / shared secret)
       → Extract: member_id, account_number
       → Inject ke Gin context: c.Set("member_id", ...)
       → Jika invalid: 401 UNAUTHORIZED
```

### 8.2 Idempotency Middleware

```
Request (POST) → Cek header X-Idempotency-Key
       → Validasi format (UUID atau string ≤ 128 char)
       → Inject ke context
       → Repository akan cek DB sebelum proses
```

### 8.3 Rate Limiter (Per Member)

```
Sliding Window Counter: max 10 req/menit per member_id
       → Implementasi via PostgreSQL counter (Phase 1, tanpa Redis)
       → Jika exceeded: 429 RATE_LIMIT_EXCEEDED
```

---

## 9. Dependencies (Go Modules)

```go
// go.mod
module loyalty-engine-service

go 1.22

require (
    github.com/gin-gonic/gin         v1.10.x    // HTTP Framework
    github.com/jackc/pgx/v5          v5.7.x     // PostgreSQL driver & pool
    github.com/golang-jwt/jwt/v5     v5.2.x     // JWT parsing
    go.uber.org/zap                  v1.27.x    // Structured logger
    github.com/spf13/viper           v1.19.x    // Config management
    github.com/google/uuid           v1.6.x     // UUID generation
    github.com/robfig/cron/v3        v3.0.x     // Background scheduler (expiry worker)
    github.com/shopspring/decimal     v1.4.x     // Presisi desimal keuangan
)
```

---

## 10. Development Phases

### Phase 0: Project Scaffolding & Infrastructure
**Estimasi: 1–2 hari**

- [ ] Init Go module `loyalty-engine-service`
- [ ] Setup direktori struktur lengkap
- [ ] Setup `config/config.go` (Viper: DB DSN, port, JWT secret, timeouts)
- [ ] Setup `internal/pkg/database/postgres.go` (pgxpool Master + Replica)
- [ ] Setup `internal/pkg/logger/logger.go` (Uber Zap, structured JSON)
- [ ] Setup `internal/pkg/jwt/customer_jwt.go` (parser claims)
- [ ] Setup `internal/delivery/http/response.go` (StandardResponse envelope)
- [ ] Setup `internal/domain/errors.go` & `pagination.go`
- [ ] Setup `cmd/server/main.go` (bootstrap + graceful shutdown)
- [ ] Buat `Makefile` (run, build, migrate, test)
- [ ] Buat `Dockerfile` (multi-stage build)
- [ ] Setup middleware: `auth.go`, `recovery.go`, `idempotency.go`
- [ ] Buat migration SQL `000001_init_earning_schema.up.sql` (copy dari databases-design.md)
- [ ] Buat migration SQL `000002_init_partitions.up.sql` (monthly partitions earning_transactions)

**Deliverable:** Service bisa start, respond `GET /health`, connect ke DB.

---

### Phase 1: Balance & Member Profile Module
**Estimasi: 1–2 hari**

- [ ] `internal/modules/balance/model/model.go`
  - `MemberBalance`, `PointLedgerEntry`, `ExpiringPointsBucket`
- [ ] `internal/modules/balance/repository/repository.go`
  - `GetMemberBalance(member_id)` → query `schema_ledger.member_balances`
  - `GetPointLedger(member_id, pagination)` → query `schema_ledger.point_ledgers` (partitioned)
  - `GetExpiringBuckets(member_id)` → query `schema_earning.point_expiry_buckets`
- [ ] `internal/modules/balance/service/service.go`
  - `GetBalance()`, `GetLedgerHistory()`, `GetExpiringPoints()`
- [ ] `internal/modules/balance/handler/handler.go`
  - `GET /api/v1/balance`
  - `GET /api/v1/balance/transactions`
  - `GET /api/v1/balance/expiring`
- [ ] Unit test: service layer

**Deliverable:** Member bisa lihat saldo, history, dan expiring points.

---

### Phase 2: Catalog Module (Read Only)
**Estimasi: 1 hari**

- [ ] `internal/modules/catalog/model/model.go`
  - `VoucherCatalogItem` (batch + available stock)
- [ ] `internal/modules/catalog/repository/repository.go`
  - `FindAllAvailable(pagination, filters)` → `schema_burning.voucher_batches WHERE available_stock > 0`
  - `FindByBatchCode(code)`
- [ ] `internal/modules/catalog/service/service.go`
- [ ] `internal/modules/catalog/handler/handler.go`
  - `GET /api/v1/catalog/vouchers`
  - `GET /api/v1/catalog/vouchers/:batch_code`

**Deliverable:** End-user bisa browse voucher yang tersedia.

---

### Phase 3: Rule Engine Core (Critical Path)
**Estimasi: 2–3 hari**

- [ ] `internal/engine/rule_evaluator.go`
  - Load active rules dari `schema_backoffice.dynamic_rules`
  - Fungsi `EvaluateConditions(rule, payload)` → AND/OR logic
  - Support operator: `EQUALS`, `NOT_EQUALS`, `GREATER_THAN`, `LESS_THAN`, `BETWEEN`, `IN`
  - Support field: `transaction_type`, `transaction_amount`, `source_channel`, `hour_of_day`, `day_of_week`, `partner_id`, `category`
- [ ] `internal/engine/point_calculator.go`
  - Fungsi `CalculatePoints(baseAmount, tierMultiplier, campaignMultiplier, timeMultiplier, bonusPoints, cap)`
  - Formula: `final = (amount * baseRate * tierMultiplier * campaignMultiplier * timeMultiplier) + bonusPoints`
  - Presisi: gunakan `shopspring/decimal`
- [ ] `internal/engine/cap_enforcer.go`
  - Load caps aktif dari `schema_earning.earning_caps`
  - Query counter dari `schema_earning.earning_cap_counters`
  - Fungsi `CheckAndApplyCap(memberID, ruleID, points, tx)` → bool (blocked), decimal (adjusted)
- [ ] Unit test komprehensif untuk setiap evaluator

**Deliverable:** Rule engine bisa evaluasi rules dan hitung poin secara akurat.

---

### Phase 4: Earning Module (Core Feature)
**Estimasi: 3–4 hari**

- [ ] `internal/modules/earning/model/model.go`
  - `EarnRequest`, `EarnResponse`, `EarningTransaction`, `EarningRuleResult`
- [ ] `internal/modules/earning/repository/repository.go`
  - `InsertEarningTransaction(tx, ...)`
  - `InsertRuleResults(tx, []result)`
  - `InsertExpiryBucket(tx, ...)`
  - `UpsertCapCounter(tx, ...)`
  - `UpsertDailySummary(tx, ...)`
  - `FindByIdempotencyKey(key)` → cek duplikat
  - `WriteLedgerEntry(tx, member_id, amount, ...)` → `schema_ledger.point_ledgers`
  - `UpdateMemberBalance(tx, member_id, points, version)` → optimistic lock
  - `WriteOutboxEvent(tx, aggregate_type, aggregate_id, payload)`
- [ ] `internal/modules/earning/service/service.go`
  - Orkestrasi lengkap flow [1]–[8] dari Section 5.1
  - Handle idempotency (return cached result jika duplicate)
  - Handle optimistic lock retry (max 3x)
- [ ] `internal/modules/earning/handler/handler.go`
  - `POST /api/v1/earn`
  - `GET /api/v1/earn/history`
  - `GET /api/v1/earn/transactions/:id`
- [ ] Integration test dengan DB (testcontainers atau test DB)

**Deliverable:** Earning flow end-to-end bekerja, idempotent, poin tercatat di ledger.

---

### Phase 5: Burning Module (Core Feature)
**Estimasi: 2–3 hari**

- [ ] `internal/modules/burning/model/model.go`
  - `RedeemRequest`, `RedeemResponse`, `RedemptionRecord`
- [ ] `internal/modules/burning/repository/repository.go`
  - `LockAvailableVoucher(tx, batch_id)` → `FOR UPDATE SKIP LOCKED`
  - `ClaimVoucher(tx, voucher_id, member_id)`
  - `DecrementBatchStock(tx, batch_id)`
  - `DeductMemberPoints(tx, member_id, points, version)` → optimistic lock
  - `WriteLedgerRedeem(tx, ...)`
  - `ConsumeExpiryBuckets(tx, member_id, points)` → FIFO consumption
  - `WriteOutboxEvent(tx, ...)`
- [ ] `internal/modules/burning/service/service.go`
  - Orkestrasi lengkap flow dari Section 5.2
  - Validasi sufficient balance sebelum begin transaction
  - Handle `FOR UPDATE SKIP LOCKED` → out-of-stock gracefully
- [ ] `internal/modules/burning/handler/handler.go`
  - `POST /api/v1/redeem/voucher`
  - `GET /api/v1/redeem/history`
- [ ] Integration test: concurrent redemption (race condition test)

**Deliverable:** Redemption voucher bekerja atomik, tidak ada double-claim.

---

### Phase 6: Reversal Module
**Estimasi: 1–2 hari**

- [ ] `internal/modules/reversal/model/model.go`
- [ ] `internal/modules/reversal/repository/repository.go`
  - `InsertReversal(tx, ...)`
  - `FindOriginalEarningTransaction(id)`
  - `ReversePointsInLedger(tx, ...)`
  - `UpdateExpiryBucketsOnReversal(tx, ...)`
- [ ] `internal/modules/reversal/service/service.go`
  - Validasi: earning transaction harus status COMPLETED
  - Validasi: belum pernah di-reverse (idempotency)
  - Deduct points dari ledger (type=ADJUSTMENT_DEBIT)
- [ ] `internal/modules/reversal/handler/handler.go`
  - `POST /api/v1/earn/transactions/:id/reverse`

> ⚠️ Reversal bisa dipanggil oleh service internal (dari backoffice) dengan service-to-service auth header, bukan customer JWT.

---

### Phase 7: Background Workers
**Estimasi: 2 hari**

#### 7.1 Outbox Worker

- [ ] `internal/workers/outbox_worker.go`
  - Poll `schema_outbox.event_outbox WHERE status='PENDING'` setiap 5 detik
  - `FOR UPDATE SKIP LOCKED`
  - Dispatch event ke webhook / internal service (HTTP POST atau log untuk Phase 1)
  - Update status ke `PROCESSED` atau `FAILED` (dengan retry_count++)
  - Max retry: 5x, lalu status = `FAILED`

#### 7.2 Expiry Batch Worker

- [ ] `internal/workers/expiry_batch_worker.go`
  - Dijadwalkan via `robfig/cron/v3`: `0 * * * *` (setiap jam)
  - Proses batch 500 buckets per run
  - Flow: query expired buckets → BEGIN TX → deduct ledger → update bucket → INSERT outbox → COMMIT
  - Logging per batch: total processed, total points expired

**Deliverable:** Poin expired otomatis, event dikirim async.

---

### Phase 8: Member Registration & Profile
**Estimasi: 1 hari**

- [ ] `internal/modules/balance/handler` — tambah endpoint:
  - `POST /api/v1/member/register` → INSERT `schema_ledger.members` + `member_balances` + trigger earning REGISTRATION
  - `GET /api/v1/member/profile` → load member + tier info + current balance summary

---

### Phase 9: Testing & Hardening
**Estimasi: 2–3 hari**

- [ ] Unit test coverage ≥ 80% untuk service & engine layer
- [ ] Integration test: earning + burning full flow dengan real PostgreSQL
- [ ] Concurrency test: 100 goroutine concurrent redeem → validasi 0 double-claim
- [ ] Idempotency test: kirim request sama 10x → hanya 1 yang diproses
- [ ] Load test sederhana: `POST /earn` dengan 500 req/s selama 60 detik
- [ ] Error scenario test: DB down, insufficient points, optimistic lock conflict

---

### Phase 10: Observability & Ops
**Estimasi: 1 hari**

- [ ] Structured log format (JSON ke stdout): `timestamp`, `service`, `member_id`, `event`, `execution_time_ms`
- [ ] `GET /health` → 200 OK `{ status: "UP" }`
- [ ] `GET /health/ready` → ping DB → 200/503
- [ ] Log setiap transaksi earn/redeem dengan duration
- [ ] Dockerfile finalisasi (multi-stage, scratch image)
- [ ] Environment variable documentation (`.env.example`)

---

## 11. Environment Variables

```env
# Service
APP_PORT=8081
APP_ENV=production

# Database
DB_MASTER_DSN=postgres://user:pass@localhost:5432/loyalty_databases?sslmode=disable
DB_REPLICA_DSN=postgres://user:pass@replica:5432/loyalty_databases?sslmode=require
DB_MAX_CONNS=50
DB_MIN_CONNS=5
DB_MAX_CONN_IDLE_SECS=300
DB_MAX_CONN_LIFE_SECS=3600

# JWT
JWT_SECRET=your-rsa-public-key-or-hmac-secret
JWT_ALGORITHM=HS256

# Workers
OUTBOX_POLL_INTERVAL_SECS=5
EXPIRY_BATCH_CRON=0 * * * *
EXPIRY_BATCH_SIZE=500

# Rate Limiting
RATE_LIMIT_MAX_REQ=10
RATE_LIMIT_WINDOW_SECS=60
```

---

## 12. Database Migration Strategy

Migration dijalankan saat startup (`--migrate` flag) atau via Makefile.

```
migrations/
├── 000001_init_earning_schema.up.sql     # Semua tabel schema_earning (dari databases-design.md)
├── 000001_init_earning_schema.down.sql   # DROP semua tabel schema_earning
├── 000002_create_partitions.up.sql       # Buat monthly partitions earning_transactions
│                                          # (auto-create untuk 3 bulan ke depan)
├── 000002_create_partitions.down.sql
└── 000003_seed_earning_caps.up.sql       # Seed default earning caps (global daily cap, dll)
```

---

## 13. Timeline Summary

| Phase | Konten | Estimasi |
|-------|--------|----------|
| **P0** | Scaffolding, Infra, Middleware | 1–2 hari |
| **P1** | Balance & Member Profile | 1–2 hari |
| **P2** | Catalog (Read) | 1 hari |
| **P3** | Rule Engine Core ⚡ | 2–3 hari |
| **P4** | Earning Module ⚡ | 3–4 hari |
| **P5** | Burning Module ⚡ | 2–3 hari |
| **P6** | Reversal Module | 1–2 hari |
| **P7** | Background Workers | 2 hari |
| **P8** | Member Registration | 1 hari |
| **P9** | Testing & Hardening | 2–3 hari |
| **P10** | Observability & Ops | 1 hari |
| **Total** | | **~17–24 hari kerja** |

> ⚡ = Critical Path — prioritas tertinggi.  
> P3 harus selesai sebelum P4; P4 dan P5 bisa paralel setelah P3 selesai.

---

## 14. Risiko & Mitigasi

| Risiko | Dampak | Mitigasi |
|--------|--------|---------|
| Optimistic lock conflict tinggi saat peak | Degradasi performa earning | Implement retry max 3x dengan exponential backoff |
| Rule evaluator lambat jika rules banyak | Latency earn > 200ms | Cache rules di memory (refresh setiap 30 detik), sort by priority |
| Partition earning_transactions tidak ada untuk bulan baru | Insert gagal | Auto-create partition worker setiap awal bulan |
| Concurrency voucher redemption (FOR UPDATE SKIP LOCKED) | Deadlock | Test konkuren, pastikan hanya satu row di-lock per transaksi |
| Idempotency key collision | Double earning | Unique index di DB, middleware wajib untuk semua POST |
| Expiry batch job terlalu lambat | Poin expired tidak tepat waktu | Batch size configurable, horizontal scaling via flag `--worker-only` |

---

## 15. Catatan Keputusan Arsitektur

1. **Tanpa Redis (Phase 1):** Semua rate limiting, idempotency, dan cap enforcement menggunakan PostgreSQL native (consistent dengan SDD §3 decision). Redis bisa ditambah di Phase 2 sebagai optimization layer.

2. **Schema isolation:** Service hanya boleh WRITE ke `schema_earning` dan `schema_ledger`. READ dari `schema_backoffice` (rules, tiers) selalu read-only. Cross-schema FK hanya untuk referential integrity.

3. **Shared database, separate service:** Berbeda dari microservices pure (tiap service punya DB sendiri), tapi konsisten dengan SDD §3.1 yang menggunakan logical schema isolation di satu PostgreSQL cluster.

4. **Outbox Pattern untuk events:** Semua side-effects (notifikasi, tier upgrade check, audit) dikirim via `schema_outbox.event_outbox` agar tidak memblok main transaction.

5. **Decimal precision:** Gunakan `shopspring/decimal` atau `numeric(15,2)` di DB. Jangan pernah gunakan `float64` untuk kalkulasi poin.

6. **Partitioned table `earning_transactions`:** Harus ada partition untuk bulan aktif sebelum insert bisa dilakukan. Worker auto-create partition wajib dijalankan.
