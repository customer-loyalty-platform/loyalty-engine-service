package service

import (
	"context"
	"strings"

	"loyalty-engine-service/internal/domain"
	"loyalty-engine-service/internal/modules/catalog/model"
	"loyalty-engine-service/internal/modules/catalog/repository"

	"go.uber.org/zap"
)

type CatalogService interface {
	GetAvailableCatalog(ctx context.Context, filter model.CatalogFilter) ([]model.VoucherCatalogItem, int64, error)
	GetCatalogDetail(ctx context.Context, batchCode string) (*model.VoucherCatalogItem, error)
}

type catalogService struct {
	repo repository.CatalogRepository
	log  *zap.Logger
}

func NewCatalogService(repo repository.CatalogRepository, log *zap.Logger) CatalogService {
	return &catalogService{
		repo: repo,
		log:  log,
	}
}

func (s *catalogService) GetAvailableCatalog(ctx context.Context, filter model.CatalogFilter) ([]model.VoucherCatalogItem, int64, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.Limit <= 0 {
		filter.Limit = 10
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	items, total, err := s.repo.FindAvailableVouchers(ctx, filter)
	if err != nil {
		s.log.Error("failed to retrieve voucher catalog", zap.Error(err))
		return nil, 0, err
	}

	return items, total, nil
}

func (s *catalogService) GetCatalogDetail(ctx context.Context, batchCode string) (*model.VoucherCatalogItem, error) {
	batchCode = strings.TrimSpace(batchCode)
	if batchCode == "" {
		return nil, domain.ErrBadRequest
	}

	item, err := s.repo.FindByBatchCode(ctx, batchCode)
	if err != nil {
		s.log.Error("failed to retrieve voucher batch detail", zap.String("batch_code", batchCode), zap.Error(err))
		return nil, err
	}

	return item, nil
}

