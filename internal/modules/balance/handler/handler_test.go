package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"loyalty-engine-service/internal/domain"
	"loyalty-engine-service/internal/modules/balance/model"
	"loyalty-engine-service/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

type mockService struct {
	balanceFn  func(ctx context.Context, memberID string) (*model.MemberBalance, error)
	ledgerFn   func(ctx context.Context, memberID string, pagination domain.PaginationQuery) ([]model.PointLedgerRow, int64, error)
	expiringFn func(ctx context.Context, memberID string, limit int) ([]model.ExpiringBucketRow, error)
	profileFn  func(ctx context.Context, memberID string) (*model.MemberProfile, error)
	registerFn func(ctx context.Context, req model.RegisterMemberRequest) (*model.RegisterMemberResponse, error)
}

func (m *mockService) GetBalance(ctx context.Context, memberID string) (*model.MemberBalance, error) {
	if m.balanceFn != nil {
		return m.balanceFn(ctx, memberID)
	}
	return nil, nil
}

func (m *mockService) GetLedgerHistory(ctx context.Context, memberID string, pagination domain.PaginationQuery) ([]model.PointLedgerRow, int64, error) {
	if m.ledgerFn != nil {
		return m.ledgerFn(ctx, memberID, pagination)
	}
	return nil, 0, nil
}

func (m *mockService) GetExpiringPoints(ctx context.Context, memberID string, limit int) ([]model.ExpiringBucketRow, error) {
	if m.expiringFn != nil {
		return m.expiringFn(ctx, memberID, limit)
	}
	return nil, nil
}

func (m *mockService) GetProfile(ctx context.Context, memberID string) (*model.MemberProfile, error) {
	if m.profileFn != nil {
		return m.profileFn(ctx, memberID)
	}
	return nil, nil
}

func (m *mockService) RegisterMember(ctx context.Context, req model.RegisterMemberRequest) (*model.RegisterMemberResponse, error) {
	if m.registerFn != nil {
		return m.registerFn(ctx, req)
	}
	return nil, nil
}

func setupTestRouter(h *BalanceHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		// Mock auth context
		c.Set("member_id", "mem-test-01")
		c.Next()
	})

	r.GET("/api/v1/balance", h.GetBalance)
	r.GET("/api/v1/balance/transactions", h.GetLedgerHistory)
	r.GET("/api/v1/balance/expiring", h.GetExpiringPoints)
	r.GET("/api/v1/member/profile", h.GetProfile)
	r.POST("/api/v1/member/register", h.RegisterMember)

	return r
}

func TestHandlerGetBalance(t *testing.T) {
	mockSvc := &mockService{
		balanceFn: func(ctx context.Context, memberID string) (*model.MemberBalance, error) {
			return &model.MemberBalance{
				MemberID:        memberID,
				TotalPoints:     2000,
				LockedPoints:    200,
				AvailablePoints: 1800,
				ExpiringPoints:  50,
				Version:         1,
				UpdatedAt:       time.Now(),
			}, nil
		},
	}

	h := NewBalanceHandler(mockSvc)
	router := setupTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/balance", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var resp response.StandardResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}

	if resp.Status != "SUCCESS" {
		t.Errorf("expected SUCCESS status, got %s", resp.Status)
	}
}

func TestHandlerRegisterMember(t *testing.T) {
	mockSvc := &mockService{
		registerFn: func(ctx context.Context, req model.RegisterMemberRequest) (*model.RegisterMemberResponse, error) {
			return &model.RegisterMemberResponse{
				MemberID:      "mem-123",
				AccountNumber: req.AccountNumber,
				Status:        "ACTIVE",
				TierCode:      "SILVER",
				CreatedAt:     time.Now(),
			}, nil
		},
	}

	h := NewBalanceHandler(mockSvc)
	router := setupTestRouter(h)

	bodyBytes, _ := json.Marshal(model.RegisterMemberRequest{
		AccountNumber: "ACC-5555",
		TierCode:      "SILVER",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/member/register", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", w.Code)
	}
}

