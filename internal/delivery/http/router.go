package http

import (
	"context"
	"net/http"
	"time"

	"loyalty-engine-service/internal/middleware"
	balanceHandler "loyalty-engine-service/internal/modules/balance/handler"
	balanceRepo "loyalty-engine-service/internal/modules/balance/repository"
	balanceService "loyalty-engine-service/internal/modules/balance/service"
	catalogHandler "loyalty-engine-service/internal/modules/catalog/handler"
	catalogRepo "loyalty-engine-service/internal/modules/catalog/repository"
	catalogService "loyalty-engine-service/internal/modules/catalog/service"
	"loyalty-engine-service/internal/pkg/database"
	"loyalty-engine-service/internal/pkg/jwt"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type RouterDependencies struct {
	DB             *database.PostgresDB
	Logger         *zap.Logger
	TokenValidator *jwt.TokenValidator
	Env            string
	BalanceHandler *balanceHandler.BalanceHandler
	CatalogHandler *catalogHandler.CatalogHandler
}

func NewRouter(deps RouterDependencies) *gin.Engine {
	if deps.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()

	// Global Middlewares
	r.Use(middleware.Recovery(deps.Logger))
	r.Use(middleware.RequestLogger(deps.Logger))

	// CORS Setup
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Idempotency-Key", "X-Member-ID"},
		ExposeHeaders:    []string{"Content-Length", "X-Idempotency-Key"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Health check endpoints (Public)
	r.GET("/health", func(c *gin.Context) {
		Success(c, http.StatusOK, gin.H{
			"status":  "UP",
			"service": "loyalty-engine-service",
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	})

	r.GET("/health/ready", func(c *gin.Context) {
		if deps.DB == nil || deps.DB.Master == nil {
			Error(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Database not configured", nil)
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := deps.DB.Master.Ping(ctx); err != nil {
			Error(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Database connection unhealthy", err.Error())
			return
		}

		Success(c, http.StatusOK, gin.H{
			"status":   "READY",
			"database": "CONNECTED",
		})
	})

	// Initialize handlers if not injected
	bHandler := deps.BalanceHandler
	if bHandler == nil && deps.DB != nil {
		bRepo := balanceRepo.NewBalanceRepository(deps.DB)
		bSvc := balanceService.NewBalanceService(bRepo, deps.Logger)
		bHandler = balanceHandler.NewBalanceHandler(bSvc)
	}

	cHandler := deps.CatalogHandler
	if cHandler == nil && deps.DB != nil {
		cRepo := catalogRepo.NewCatalogRepository(deps.DB)
		cSvc := catalogService.NewCatalogService(cRepo, deps.Logger)
		cHandler = catalogHandler.NewCatalogHandler(cSvc)
	}

	// API v1 route groups
	v1 := r.Group("/api/v1")
	{
		// Public member registration onboarding endpoint
		if bHandler != nil {
			v1.POST("/member/register", bHandler.RegisterMember)
		}

		// Catalog endpoints (Public or Member accessible)
		if cHandler != nil {
			v1.GET("/catalog/vouchers", cHandler.GetCatalog)
			v1.GET("/catalog/vouchers/:batch_code", cHandler.GetCatalogDetail)
		} else {
			v1.GET("/catalog/vouchers", func(c *gin.Context) {
				Success(c, http.StatusOK, gin.H{
					"message": "Catalog module placeholder",
				})
			})
		}

		// Authenticated member routes (End-User / Web View)
		memberAuth := v1.Group("")
		memberAuth.Use(middleware.CustomerAuth(deps.TokenValidator))
		{
			// Balance and Profile endpoints
			if bHandler != nil {
				memberAuth.GET("/balance", bHandler.GetBalance)
				memberAuth.GET("/balance/transactions", bHandler.GetLedgerHistory)
				memberAuth.GET("/balance/expiring", bHandler.GetExpiringPoints)
				memberAuth.GET("/member/profile", bHandler.GetProfile)
			} else {
				memberAuth.GET("/balance", func(c *gin.Context) {
					memberID := c.GetString("member_id")
					Success(c, http.StatusOK, gin.H{
						"member_id": memberID,
						"message":   "Balance module placeholder",
					})
				})
			}

			// State-changing routes with Idempotency Key
			withIdempotency := memberAuth.Group("")
			withIdempotency.Use(middleware.RequireIdempotencyKey())
			{
				withIdempotency.POST("/earn", func(c *gin.Context) {
					idempotencyKey := c.GetString("idempotency_key")
					Success(c, http.StatusAccepted, gin.H{
						"idempotency_key": idempotencyKey,
						"message":         "Earning engine initialized (Phase 4)",
					})
				})

				withIdempotency.POST("/redeem/voucher", func(c *gin.Context) {
					idempotencyKey := c.GetString("idempotency_key")
					Success(c, http.StatusAccepted, gin.H{
						"idempotency_key": idempotencyKey,
						"message":         "Burning engine initialized (Phase 5)",
					})
				})
			}
		}
	}

	return r
}
