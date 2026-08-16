package server

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/supernurture/go-template/internal/api/server/modules/health"
	healthcontract "github.com/supernurture/go-template/internal/api/server/oapicodegen/health"
	"github.com/supernurture/go-template/internal/config"
	"github.com/supernurture/go-template/internal/middleware"
	"github.com/supernurture/go-template/pkg/logger"
)

// NewRouter builds the gin engine: mode, trusted proxies, the default middleware chain, and every module's generated routes.
func NewRouter(cfg *config.Config, log *logger.Logger) (*gin.Engine, error) {
	gin.SetMode(cfg.Server.Mode)

	router := gin.New()
	if err := router.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}
	router.Use(middleware.Default(cfg, log)...)

	register(router)
	return router, nil
}

func register(router gin.IRouter) {
	healthcontract.RegisterHandlers(router, healthcontract.NewStrictHandler(health.NewHandler(), nil))
}
