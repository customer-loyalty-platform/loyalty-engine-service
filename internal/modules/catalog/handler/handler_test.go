package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"loyalty-engine-service/internal/domain"
	"loyalty-engine-service/internal/modules/catalog/model"
	"loyalty-engine-service/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

type mockCatalogService struct {
	getCatalogFn func(ctx context.Context, filter model.CatalogFilter) ([]model.VoucherCatalogItem, int64, error)
	getDetailFn  func(ctx context.Context, batchCode string) (*model.VoucherCatalogItem, error)
}

func (m *mockCatalogService) GetAvailableCatalog(ctx context.Context, filter model.CatalogFilter) ([]model.VoucherCatalogItem, int64, error) {
	if m.getCatalogFn != nil {
		return m.getCatalogFn(ctx, filter)
	}
	return []model.VoucherCatalogItem{}, 0, nil
}

func (m *mockCatalogService) GetCatalogDetail(ctx context.Context, batchCode string) (*model.VoucherCatalogItem, error) {
	if m.getDetailFn != nil {
		return m.getDetailFn(ctx, batchCode)
	}
	return nil, domain.ErrNotFound
}

func setupCatalogTestRouter(h *CatalogHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/api/v1/catalog/vouchers", h.GetCatalog)
	r.GET("/api/v1/catalog/vouchers/:batch_code", h.GetCatalogDetail)

	return r
}

func TestHandlerGetCatalog(t *testing.T) {
	mockSvc := &mockCatalogService{
		getCatalogFn: func(ctx context.Context, filter model.CatalogFilter) ([]model.VoucherCatalogItem, int64, error) {
			return []model.VoucherCatalogItem{
				{
					ID:             "vch-1",
					BatchCode:      "BATCH-01",
					Title:          "Voucher Makan",
					PointsCost:     50,
					TotalStock:     100,
					AvailableStock: 80,
					CreatedAt:      time.Now(),
				},
			}, 1, nil
		},
	}

	h := NewCatalogHandler(mockSvc)
	router := setupCatalogTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/catalog/vouchers?page=1&limit=10&max_points=100", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var resp response.StandardResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %s", resp.Status)
	}

	if resp.Pagination == nil || resp.Pagination.TotalItems != 1 {
		t.Errorf("expected pagination with 1 total item, got %+v", resp.Pagination)
	}
}

func TestHandlerGetCatalogDetail(t *testing.T) {
	mockSvc := &mockCatalogService{
		getDetailFn: func(ctx context.Context, batchCode string) (*model.VoucherCatalogItem, error) {
			if batchCode == "BATCH-01" {
				return &model.VoucherCatalogItem{
					ID:             "vch-1",
					BatchCode:      "BATCH-01",
					Title:          "Voucher Makan",
					PointsCost:     50,
					TotalStock:     100,
					AvailableStock: 80,
					CreatedAt:      time.Now(),
				}, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	h := NewCatalogHandler(mockSvc)
	router := setupCatalogTestRouter(h)

	// Test found
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/catalog/vouchers/BATCH-01", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	// Test not found
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/catalog/vouchers/NON-EXISTENT", nil)
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", w2.Code)
	}
}

