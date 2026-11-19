package service

import (
	"context"
	"testing"
	"time"

	"loyalty-engine-service/internal/domain"
	"loyalty-engine-service/internal/modules/catalog/model"

	"go.uber.org/zap"
)

type mockCatalogRepo struct {
	findAvailableFn func(ctx context.Context, filter model.CatalogFilter) ([]model.VoucherCatalogItem, int64, error)
	findByCodeFn    func(ctx context.Context, batchCode string) (*model.VoucherCatalogItem, error)
}

func (m *mockCatalogRepo) FindAvailableVouchers(ctx context.Context, filter model.CatalogFilter) ([]model.VoucherCatalogItem, int64, error) {
	if m.findAvailableFn != nil {
		return m.findAvailableFn(ctx, filter)
	}
	return []model.VoucherCatalogItem{}, 0, nil
}

func (m *mockCatalogRepo) FindByBatchCode(ctx context.Context, batchCode string) (*model.VoucherCatalogItem, error) {
	if m.findByCodeFn != nil {
		return m.findByCodeFn(ctx, batchCode)
	}
	return nil, domain.ErrNotFound
}

func TestGetAvailableCatalog(t *testing.T) {
	log := zap.NewNop()
	mockRepo := &mockCatalogRepo{
		findAvailableFn: func(ctx context.Context, filter model.CatalogFilter) ([]model.VoucherCatalogItem, int64, error) {
			return []model.VoucherCatalogItem{
				{
					ID:             "vch-001",
					BatchCode:      "DISC-10K",
					Title:          "Diskon 10 Ribu",
					PointsCost:     100,
					TotalStock:     100,
					AvailableStock: 50,
					CreatedAt:      time.Now(),
				},
				{
					ID:             "vch-002",
					BatchCode:      "FREE-RIDE",
					Title:          "Gratis Naik Kereta",
					PointsCost:     250,
					TotalStock:     50,
					AvailableStock: 25,
					CreatedAt:      time.Now(),
				},
			}, 2, nil
		},
	}

	svc := NewCatalogService(mockRepo, log)

	items, total, err := svc.GetAvailableCatalog(context.Background(), model.CatalogFilter{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].BatchCode != "DISC-10K" {
		t.Errorf("expected first item DISC-10K, got %s", items[0].BatchCode)
	}
}

func TestGetCatalogDetail(t *testing.T) {
	log := zap.NewNop()
	mockRepo := &mockCatalogRepo{
		findByCodeFn: func(ctx context.Context, batchCode string) (*model.VoucherCatalogItem, error) {
			if batchCode == "DISC-10K" {
				return &model.VoucherCatalogItem{
					ID:             "vch-001",
					BatchCode:      "DISC-10K",
					Title:          "Diskon 10 Ribu",
					PointsCost:     100,
					TotalStock:     100,
					AvailableStock: 50,
					CreatedAt:      time.Now(),
				}, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	svc := NewCatalogService(mockRepo, log)

	// Test found
	item, err := svc.GetCatalogDetail(context.Background(), "DISC-10K")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.BatchCode != "DISC-10K" {
		t.Errorf("expected DISC-10K, got %s", item.BatchCode)
	}

	// Test empty code
	_, err = svc.GetCatalogDetail(context.Background(), "")
	if err != domain.ErrBadRequest {
		t.Errorf("expected ErrBadRequest for empty batchCode, got %v", err)
	}

	// Test not found
	_, err = svc.GetCatalogDetail(context.Background(), "NOT-EXIST")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

