package middleware

import (
	"net/http"
	"strings"

	"loyalty-engine-service/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

func RequireIdempotencyKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		idempotencyKey := strings.TrimSpace(c.GetHeader("X-Idempotency-Key"))
		if idempotencyKey == "" {
			response.Error(c, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "X-Idempotency-Key header is required for this operation", nil)
			c.Abort()
			return
		}

		if len(idempotencyKey) > 128 {
			response.Error(c, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "X-Idempotency-Key exceeds maximum length of 128 characters", nil)
			c.Abort()
			return
		}

		c.Set("idempotency_key", idempotencyKey)
		c.Next()
	}
}
