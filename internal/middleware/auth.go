package middleware

import (
	"net/http"
	"strings"

	"loyalty-engine-service/internal/pkg/jwt"
	"loyalty-engine-service/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

func CustomerAuth(validator *jwt.TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Check direct gateway injected header (e.g., from Envoy / API Gateway)
		gatewayMemberID := c.GetHeader("X-Member-ID")
		if gatewayMemberID != "" {
			c.Set("member_id", gatewayMemberID)
			c.Next()
			return
		}

		// 2. Check Authorization Bearer header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Missing authorization header", nil)
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid authorization header format", nil)
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims, err := validator.ValidateToken(tokenString)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired token", err.Error())
			c.Abort()
			return
		}

		c.Set("member_id", claims.MemberID)
		c.Set("account_number", claims.AccountNumber)
		c.Set("tier_id", claims.TierID)

		c.Next()
	}
}
