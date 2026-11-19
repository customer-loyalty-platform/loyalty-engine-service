package service

import (
	"context"
	"testing"
	"time"

	"loyalty-engine-service/internal/domain"
	"loyalty-engine-service/internal/modules/balance/model"

	"go.uber.org/zap"
)

type mockBalanceRepo struct {
	balanceFn         func(ctx context.Context, memberID string) (*model.MemberBalance, error)
	ledgerFn          func(ctx context.Context, memberID string, pagination domain.PaginationQuery) ([]model.PointLedgerRow, int64, error)
	expiringFn        func(ctx context.Context, memberID string, limit int) ([]model.ExpiringBucketRow, error)
	profileFn         func(ctx context.Context, memberID string) (*model.MemberProfile, error)
	registerFn        func(ctx context.Context, accountNumber string, tierCode string) (*model.RegisterMemberResponse, error)
}

func (m *mockBalanceRepo) GetBalanceByMemberID(ctx context.Context, memberID string) (*model.MemberBalance, error) {
	if m.balanceFn != nil {
		return m.balanceFn(ctx, memberID)
	}
	return nil, domain.ErrNotFound
}

func (m *mockBalanceRepo) GetLedgerByMemberID(ctx context.Context, memberID string, pagination domain.PaginationQuery) ([]model.PointLedgerRow, int64, error) {
	if m.ledgerFn != nil {
		return m.ledgerFn(ctx, memberID, pagination)
	}
	return []model.PointLedgerRow{}, 0, nil
}

func (m *mockBalanceRepo) GetExpiringBucketsByMemberID(ctx context.Context, memberID string, limit int) ([]model.ExpiringBucketRow, error) {
	if m.expiringFn != nil {
		return m.expiringFn(ctx, memberID, limit)
	}
	return []model.ExpiringBucketRow{}, nil
}

func (m *mockBalanceRepo) GetMemberProfile(ctx context.Context, memberID string) (*model.MemberProfile, error) {
	if m.profileFn != nil {
		return m.profileFn(ctx, memberID)
	}
	return nil, domain.ErrNotFound
}

func (m *mockBalanceRepo) RegisterMember(ctx context.Context, accountNumber string, tierCode string) (*model.RegisterMemberResponse, error) {
	if m.registerFn != nil {
		return m.registerFn(ctx, accountNumber, tierCode)
	}
	return nil, nil
}

func TestGetBalance(t *testing.T) {
	log := zap.NewNop()
	mockRepo := &mockBalanceRepo{
		balanceFn: func(ctx context.Context, memberID string) (*model.MemberBalance, error) {
			if memberID == "mem-001" {
				return &model.MemberBalance{
					MemberID:        memberID,
					TotalPoints:     1500.00,
					LockedPoints:    100.00,
					ExpiringPoints:  250.00,
					AvailablePoints: 1400.00,
					Version:         3,
					UpdatedAt:       time.Now(),
				}, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	svc := NewBalanceService(mockRepo, log)

	// Test found
	bal, err := svc.GetBalance(context.Background(), "mem-001")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if bal.TotalPoints != 1500.00 {
		t.Errorf("expected 1500.00 total points, got %f", bal.TotalPoints)
	}
	if bal.AvailablePoints != 1400.00 {
		t.Errorf("expected 1400.00 available points, got %f", bal.AvailablePoints)
	}

	// Test empty memberID
	_, err = svc.GetBalance(context.Background(), "")
	if err != domain.ErrBadRequest {
		t.Errorf("expected ErrBadRequest for empty memberID, got %v", err)
	}

	// Test not found
	_, err = svc.GetBalance(context.Background(), "mem-non-existent")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetLedgerHistory(t *testing.T) {
	log := zap.NewNop()
	mockRepo := &mockBalanceRepo{
		ledgerFn: func(ctx context.Context, memberID string, pagination domain.PaginationQuery) ([]model.PointLedgerRow, int64, error) {
			return []model.PointLedgerRow{
				{
					ID:           "led-01",
					MemberID:     memberID,
					Type:         "EARN",
					Amount:       500.00,
					BalanceAfter: 500.00,
					ReferenceID:  "tx-001",
					CreatedAt:    time.Now(),
				},
			}, 1, nil
		},
	}

	svc := NewBalanceService(mockRepo, log)

	rows, total, err := svc.GetLedgerHistory(context.Background(), "mem-001", domain.PaginationQuery{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(rows) != 1 || rows[0].Amount != 500.00 {
		t.Errorf("expected 1 row with amount 500, got %+v", rows)
	}
}

func TestRegisterMember(t *testing.T) {
	log := zap.NewNop()
	mockRepo := &mockBalanceRepo{
		registerFn: func(ctx context.Context, accountNumber string, tierCode string) (*model.RegisterMemberResponse, error) {
			return &model.RegisterMemberResponse{
				MemberID:      "mem-new",
				AccountNumber: accountNumber,
				Status:        "ACTIVE",
				TierCode:      "BRONZE",
				CreatedAt:     time.Now(),
			}, nil
		},
	}

	svc := NewBalanceService(mockRepo, log)

	resp, err := svc.RegisterMember(context.Background(), model.RegisterMemberRequest{
		AccountNumber: "ACC-TEST-999",
		TierCode:      "BRONZE",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.MemberID != "mem-new" {
		t.Errorf("expected MemberID mem-new, got %s", resp.MemberID)
	}
}

