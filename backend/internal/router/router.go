package router

import (
	"log/slog"
	"net/http"

	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/config"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/handler"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/middleware"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/repository"
	"github.com/blueship581/carbon-capture-compliance-operations/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB, redisClient *redis.Client, logger *slog.Logger) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	engine.Use(middleware.RequestContext(logger))
	if len(cfg.TrustedProxies) > 0 {
		_ = engine.SetTrustedProxies(cfg.TrustedProxies)
	} else {
		_ = engine.SetTrustedProxies(nil)
	}

	securityRepository := repository.NewSecurityRepository(db)
	securityService := service.NewSecurityService(securityRepository, cfg)
	captureUnitRepository := repository.NewCaptureUnitRepository(db)
	permitRuleRepository := repository.NewPermitRuleRepository(db)
	emissionSampleRepository := repository.NewEmissionSampleRepository(db)
	complianceDecisionRepository := repository.NewComplianceDecisionRepository(db)
	captureUnitService := service.NewCaptureUnitService(captureUnitRepository, securityService)
	permitRuleService := service.NewPermitRuleService(permitRuleRepository, complianceDecisionRepository, emissionSampleRepository, captureUnitRepository, securityService)
	emissionSampleService := service.NewEmissionSampleService(emissionSampleRepository, securityService)
	complianceDecisionService := service.NewComplianceDecisionService(complianceDecisionRepository, securityService)
	captureUnitHandler := handler.NewCaptureUnitHandler(captureUnitService)
	permitRuleHandler := handler.NewPermitRuleHandler(permitRuleService)
	emissionSampleHandler := handler.NewEmissionSampleHandler(emissionSampleService)
	complianceDecisionHandler := handler.NewComplianceDecisionHandler(complianceDecisionService)
	systemHandler := handler.NewSystemHandler(securityService, captureUnitService, permitRuleService, emissionSampleService, complianceDecisionService, db, redisClient)

	engine.GET("/healthz", systemHandler.Health)
	engine.POST("/api/auth/login", systemHandler.Login)

	limiter := middleware.NewLimiter(redisClient, cfg.RequestLimit)
	api := engine.Group("/api")
	api.Use(limiter.Middleware(), middleware.Authenticate(cfg))
	api.GET("/overview", systemHandler.Overview)
	api.GET("/audits", middleware.RequireMinimumRole("reviewer"), systemHandler.Audits)
	api.GET("/session", systemHandler.Session)
	api.GET("/runtime", systemHandler.Runtime)
	api.GET("/audit-summary", middleware.RequireMinimumRole("reviewer"), systemHandler.AuditSummary)
	api.GET("/audits/:entityType/:id", middleware.RequireMinimumRole("reviewer"), systemHandler.EntityHistory)
	captureUnitHandler.Register(api)
	permitRuleHandler.Register(api)
	emissionSampleHandler.Register(api)
	complianceDecisionHandler.Register(api)

	engine.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "route_not_found"})
	})
	return engine
}
