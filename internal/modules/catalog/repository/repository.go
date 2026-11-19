package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"loyalty-engine-service/internal/domain"
	"loyalty-engine-service/internal/modules/catalog/model"
	"loyalty-engine-service/internal/pkg/database"

	"github.com/jackc/pgx/v5"
)

type CatalogRepository interface {
	FindAvailableVouchers(ctx context.Context, filter model.CatalogFilter) ([]model.VoucherCatalogItem, int64, error)
	FindByBatchCode(ctx context.Context, batchCode string) (*model.VoucherCatalogItem, error)
}

type catalogRepository struct {
	db *database.PostgresDB
}

func NewCatalogRepository(db *database.PostgresDB) CatalogRepository {
	return &catalogRepository{db: db}
}

func (r *catalogRepository) FindAvailableVouchers(ctx context.Context, filter model.CatalogFilter) ([]model.VoucherCatalogItem, int64, error) {
	whereClauses := "WHERE available_stock > 0"
	var args []interface{}
	argIdx := 1

	if strings.TrimSpace(filter.Search) != "" {
		searchTerm := "%" + strings.TrimSpace(filter.Search) + "%"
		whereClauses += fmt.Sprintf(" AND (batch_code ILIKE $%d OR title ILIKE $%d)", argIdx, argIdx)
		args = append(args, searchTerm)
		argIdx++
	}

	if filter.MaxPoints != nil && *filter.MaxPoints >= 0 {
		whereClauses += fmt.Sprintf(" AND points_cost <= $%d", argIdx)
		args = append(args, *filter.MaxPoints)
		argIdx++
	}

	if filter.MinPoints != nil && *filter.MinPoints >= 0 {
		whereClauses += fmt.Sprintf(" AND points_cost >= $%d", argIdx)
		args = append(args, *filter.MinPoints)
		argIdx++
	}

	// 1. Count total matching items
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM schema_burning.voucher_batches %s", whereClauses)
	var total int64
	err := r.db.Replica.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count catalog items: %w", err)
	}

	if total == 0 {
		return []model.VoucherCatalogItem{}, 0, nil
	}

	// 2. Sorting clause
	orderBy := "ORDER BY created_at DESC"
	switch filter.SortBy {
	case "points_asc":
		orderBy = "ORDER BY points_cost ASC, created_at DESC"
	case "points_desc":
		orderBy = "ORDER BY points_cost DESC, created_at DESC"
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 10
	}
	offset := filter.Offset()

	dataQuery := fmt.Sprintf(`
		SELECT id, batch_code, title, points_cost, total_stock, available_stock, COALESCE(created_at, NOW())
		FROM schema_burning.voucher_batches
		%s
		%s
		LIMIT $%d OFFSET $%d`, whereClauses, orderBy, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Replica.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query catalog items: %w", err)
	}
	defer rows.Close()

	var items []model.VoucherCatalogItem
	for rows.Next() {
		var item model.VoucherCatalogItem
		if err := rows.Scan(
			&item.ID,
			&item.BatchCode,
			&item.Title,
			&item.PointsCost,
			&item.TotalStock,
			&item.AvailableStock,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan catalog item: %w", err)
		}
		items = append(items, item)
	}

	return items, total, nil
}

func (r *catalogRepository) FindByBatchCode(ctx context.Context, batchCode string) (*model.VoucherCatalogItem, error) {
	query := `
		SELECT id, batch_code, title, points_cost, total_stock, available_stock, COALESCE(created_at, NOW())
		FROM schema_burning.voucher_batches
		WHERE batch_code = $1`

	var item model.VoucherCatalogItem
	err := r.db.Replica.QueryRow(ctx, query, batchCode).Scan(
		&item.ID,
		&item.BatchCode,
		&item.Title,
		&item.PointsCost,
		&item.TotalStock,
		&item.AvailableStock,
		&item.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query voucher batch by code: %w", err)
	}

	return &item, nil
}

