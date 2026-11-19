package handler

import (
	"errors"
	"math"
	"net/http"

	"loyalty-engine-service/internal/domain"
	"loyalty-engine-service/internal/modules/catalog/model"
	"loyalty-engine-service/internal/modules/catalog/service"
	"loyalty-engine-service/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

type CatalogHandler struct {
	service service.CatalogService
}

func NewCatalogHandler(s service.CatalogService) *CatalogHandler {
	return &CatalogHandler{service: s}
}

// GetCatalog returns available voucher batches for redemption with optional filters
func (h *CatalogHandler) GetCatalog(c *gin.Context) {
	var filter model.CatalogFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_FILTER", "Invalid query parameters for catalog", err.Error())
		return
	}

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.Limit <= 0 {
		filter.Limit = 10
	}

	items, total, err := h.service.GetAvailableCatalog(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve voucher catalog", err.Error())
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(filter.Limit)))
	pagResp := &response.PaginationBody{
		Page:       filter.Page,
		Limit:      filter.Limit,
		TotalItems: total,
		TotalPages: totalPages,
	}

	response.SuccessWithMeta(c, http.StatusOK, items, pagResp)
}

// GetCatalogDetail returns specific voucher details by batch code
func (h *CatalogHandler) GetCatalogDetail(c *gin.Context) {
	batchCode := c.Param("batch_code")
	if batchCode == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "Batch code parameter is required", nil)
		return
	}

	item, err := h.service.GetCatalogDetail(c.Request.Context(), batchCode)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "Voucher catalog batch not found", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve voucher batch", err.Error())
		return
	}

	response.Success(c, http.StatusOK, item)
}

