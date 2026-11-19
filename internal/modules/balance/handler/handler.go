package handler

import (
	"errors"
	"math"
	"net/http"
	"strconv"

	"loyalty-engine-service/internal/domain"
	"loyalty-engine-service/internal/modules/balance/model"
	"loyalty-engine-service/internal/modules/balance/service"
	"loyalty-engine-service/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

type BalanceHandler struct {
	service service.BalanceService
}

func NewBalanceHandler(s service.BalanceService) *BalanceHandler {
	return &BalanceHandler{service: s}
}

// GetBalance returns real-time balance for the authenticated member
func (h *BalanceHandler) GetBalance(c *gin.Context) {
	memberID := c.GetString("member_id")
	if memberID == "" {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Member ID context is missing", nil)
		return
	}

	bal, err := h.service.GetBalance(c.Request.Context(), memberID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "Member balance record not found", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve member balance", err.Error())
		return
	}

	response.Success(c, http.StatusOK, bal)
}

// GetLedgerHistory returns paginated ledger transaction history for the authenticated member
func (h *BalanceHandler) GetLedgerHistory(c *gin.Context) {
	memberID := c.GetString("member_id")
	if memberID == "" {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Member ID context is missing", nil)
		return
	}

	var pagination domain.PaginationQuery
	if err := c.ShouldBindQuery(&pagination); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_PAGINATION", "Invalid pagination parameters", err.Error())
		return
	}

	if pagination.Page <= 0 {
		pagination.Page = 1
	}
	if pagination.Limit <= 0 {
		pagination.Limit = 10
	}

	rows, total, err := h.service.GetLedgerHistory(c.Request.Context(), memberID, pagination)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve point ledgers", err.Error())
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pagination.Limit)))
	pagResp := &response.PaginationBody{
		Page:       pagination.Page,
		Limit:      pagination.Limit,
		TotalItems: total,
		TotalPages: totalPages,
	}

	response.SuccessWithMeta(c, http.StatusOK, rows, pagResp)
}

// GetExpiringPoints returns point buckets that are about to expire
func (h *BalanceHandler) GetExpiringPoints(c *gin.Context) {
	memberID := c.GetString("member_id")
	if memberID == "" {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Member ID context is missing", nil)
		return
	}

	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	buckets, err := h.service.GetExpiringPoints(c.Request.Context(), memberID, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve expiring points", err.Error())
		return
	}

	response.Success(c, http.StatusOK, buckets)
}

// GetProfile returns member status, tier details, and balance summary
func (h *BalanceHandler) GetProfile(c *gin.Context) {
	memberID := c.GetString("member_id")
	if memberID == "" {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Member ID context is missing", nil)
		return
	}

	profile, err := h.service.GetProfile(c.Request.Context(), memberID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "Member profile not found", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve member profile", err.Error())
		return
	}

	response.Success(c, http.StatusOK, profile)
}

// RegisterMember registers a new member and initializes their balance in PostgreSQL
func (h *BalanceHandler) RegisterMember(c *gin.Context) {
	var req model.RegisterMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid registration request payload", err.Error())
		return
	}

	resp, err := h.service.RegisterMember(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			response.Error(c, http.StatusConflict, "CONFLICT", "Account number already registered", err.Error())
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			response.Error(c, http.StatusBadRequest, "INVALID_TIER", "Specified tier code does not exist", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to register member", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, resp)
}

