package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"loyalty-engine-service/internal/domain"
	balanceHandler "loyalty-engine-service/internal/modules/balance/handler"
	balanceModel "loyalty-engine-service/internal/modules/balance/model"
	balanceService "loyalty-engine-service/internal/modules/balance/service"
	catalogHandler "loyalty-engine-service/internal/modules/catalog/handler"
	catalogModel "loyalty-engine-service/internal/modules/catalog/model"
	catalogService "loyalty-engine-service/internal/modules/catalog/service"
	"loyalty-engine-service/internal/pkg/jwt"
	"loyalty-engine-service/internal/pkg/logger"
)

type mockRepo struct{}

func (m *mockRepo) GetBalanceByMemberID(ctx context.Context, memberID string) (*balanceModel.MemberBalance, error) {
	return &balanceModel.MemberBalance{
		MemberID:        memberID,
		TotalPoints:     500,
		LockedPoints:    0,
		ExpiringPoints:  50,
		AvailablePoints: 500,
		Version:         1,
		UpdatedAt:       time.Now(),
	}, nil
}

func (m *mockRepo) GetLedgerByMemberID(ctx context.Context, memberID string, pagination domain.PaginationQuery) ([]balanceModel.PointLedgerRow, int64, error) {
	return []balanceModel.PointLedgerRow{}, 0, nil
}

func (m *mockRepo) GetExpiringBucketsByMemberID(ctx context.Context, memberID string, limit int) ([]balanceModel.ExpiringBucketRow, error) {
	return []balanceModel.ExpiringBucketRow{}, nil
}

func (m *mockRepo) GetMemberProfile(ctx context.Context, memberID string) (*balanceModel.MemberProfile, error) {
	return &balanceModel.MemberProfile{
		MemberID:      memberID,
		AccountNumber: "ACC-001",
		Status:        "ACTIVE",
		CreatedAt:     time.Now(),
	}, nil
}

func (m *mockRepo) RegisterMember(ctx context.Context, accountNumber string, tierCode string) (*balanceModel.RegisterMemberResponse, error) {
	return &balanceModel.RegisterMemberResponse{
		MemberID:      "mem-100",
		AccountNumber: accountNumber,
		Status:        "ACTIVE",
		TierCode:      "BRONZE",
		CreatedAt:     time.Now(),
	}, nil
}

type mockCatalogRepo struct{}

func (m *mockCatalogRepo) FindAvailableVouchers(ctx context.Context, filter catalogModel.CatalogFilter) ([]catalogModel.VoucherCatalogItem, int64, error) {
	return []catalogModel.VoucherCatalogItem{
		{
			ID:             "vch-test",
			BatchCode:      "BATCH-01",
			Title:          "Voucher Test",
			PointsCost:     100,
			TotalStock:     50,
			AvailableStock: 25,
			CreatedAt:      time.Now(),
		},
	}, 1, nil
}

func (m *mockCatalogRepo) FindByBatchCode(ctx context.Context, batchCode string) (*catalogModel.VoucherCatalogItem, error) {
	if batchCode == "BATCH-01" {
		return &catalogModel.VoucherCatalogItem{
			ID:             "vch-test",
			BatchCode:      "BATCH-01",
			Title:          "Voucher Test",
			PointsCost:     100,
			TotalStock:     50,
			AvailableStock: 25,
			CreatedAt:      time.Now(),
		}, nil
	}
	return nil, domain.ErrNotFound
}

func TestHealthEndpoint(t *testing.T) {
	log := logger.NewLogger("development")
	tokenVal := jwt.NewTokenValidator("secret")

	r := NewRouter(RouterDependencies{
		DB:             nil, // health route does not query DB
		Logger:         log,
		TokenValidator: tokenVal,
		Env:            "development",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp StandardResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response body: %v", err)
	}

	if resp.Status != "SUCCESS" {
		t.Errorf("Expected status SUCCESS, got %s", resp.Status)
	}
}

func TestAuthMiddleware(t *testing.T) {
	log := logger.NewLogger("development")
	tokenVal := jwt.NewTokenValidator("secret-key")

	r := NewRouter(RouterDependencies{
		DB:             nil,
		Logger:         log,
		TokenValidator: tokenVal,
		Env:            "development",
	})

	// Test unauthenticated access to /api/v1/balance
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/balance", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected status 401, got %d", w.Code)
	}

	// Test valid token access
	token, _ := tokenVal.GenerateTestToken("mem-001", "ACC-001", 10000000000)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/balance", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("Expected status 200 with valid token, got %d", w2.Code)
	}
}

func TestIdempotencyMiddleware(t *testing.T) {
	log := logger.NewLogger("development")
	tokenVal := jwt.NewTokenValidator("secret-key")

	r := NewRouter(RouterDependencies{
		DB:             nil,
		Logger:         log,
		TokenValidator: tokenVal,
		Env:            "development",
	})

	token, _ := tokenVal.GenerateTestToken("mem-001", "ACC-001", 10000000000)

	// POST /api/v1/earn without X-Idempotency-Key
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/earn", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400 for missing idempotency key, got %d", w.Code)
	}

	// POST /api/v1/earn with X-Idempotency-Key
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/earn", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("X-Idempotency-Key", "idemp-test-12345")
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusAccepted {
		t.Fatalf("Expected status 202 Accepted, got %d", w2.Code)
	}
}

func TestBalanceRoutesWithHandler(t *testing.T) {
	log := logger.NewLogger("development")
	tokenVal := jwt.NewTokenValidator("secret-key")

	svc := balanceService.NewBalanceService(&mockRepo{}, log)
	bh := balanceHandler.NewBalanceHandler(svc)

	cSvc := catalogService.NewCatalogService(&mockCatalogRepo{}, log)
	ch := catalogHandler.NewCatalogHandler(cSvc)

	r := NewRouter(RouterDependencies{
		DB:             nil,
		Logger:         log,
		TokenValidator: tokenVal,
		Env:            "development",
		BalanceHandler: bh,
		CatalogHandler: ch,
	})

	token, _ := tokenVal.GenerateTestToken("mem-001", "ACC-001", 10000000000)

	// 1. GET /api/v1/balance
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/balance", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /balance, got %d", w.Code)
	}

	// 2. GET /api/v1/balance/transactions
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/balance/transactions?page=1&limit=5", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for /balance/transactions, got %d", w2.Code)
	}

	// 3. GET /api/v1/balance/expiring
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/api/v1/balance/expiring", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 for /balance/expiring, got %d", w3.Code)
	}

	// 4. GET /api/v1/member/profile
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("GET", "/api/v1/member/profile", nil)
	req4.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("expected 200 for /member/profile, got %d", w4.Code)
	}

	// 5. GET /api/v1/catalog/vouchers
	w5 := httptest.NewRecorder()
	req5, _ := http.NewRequest("GET", "/api/v1/catalog/vouchers?page=1&limit=10", nil)
	r.ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK {
		t.Fatalf("expected 200 for /catalog/vouchers, got %d", w5.Code)
	}

	// 6. GET /api/v1/catalog/vouchers/:batch_code
	w6 := httptest.NewRecorder()
	req6, _ := http.NewRequest("GET", "/api/v1/catalog/vouchers/BATCH-01", nil)
	r.ServeHTTP(w6, req6)
	if w6.Code != http.StatusOK {
		t.Fatalf("expected 200 for /catalog/vouchers/BATCH-01, got %d", w6.Code)
	}
}
