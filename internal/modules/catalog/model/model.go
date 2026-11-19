package model

import "time"

type VoucherCatalogItem struct {
	ID             string    `json:"id"`
	BatchCode      string    `json:"batch_code"`
	Title          string    `json:"title"`
	PointsCost     float64   `json:"points_cost"`
	TotalStock     int64     `json:"total_stock"`
	AvailableStock int64     `json:"available_stock"`
	CreatedAt      time.Time `json:"created_at"`
}

type CatalogFilter struct {
	Page      int      `form:"page,default=1"`
	Limit     int      `form:"limit,default=10"`
	Search    string   `form:"search"`
	MaxPoints *float64 `form:"max_points"`
	MinPoints *float64 `form:"min_points"`
	SortBy    string   `form:"sort_by"` // "points_asc", "points_desc", "newest"
}

func (f CatalogFilter) Offset() int {
	if f.Page <= 0 {
		return 0
	}
	return (f.Page - 1) * f.Limit
}

