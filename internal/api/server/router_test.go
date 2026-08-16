package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/supernurture/go-template/internal/config"
	"github.com/supernurture/go-template/pkg/logger"
)

func newTestRouter(t *testing.T, cfg *config.Config) *gin.Engine {
	t.Helper()
	log, err := logger.New(logger.Config{ServiceName: "test", Path: t.TempDir()})
	if err != nil {
		t.Fatalf("logger.New: %v", err)
	}
	t.Cleanup(func() { _ = log.Close() })
	router, err := NewRouter(cfg, log)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return router
}

func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Server.Mode = gin.TestMode
	cfg.Server.Timeout = time.Second
	return cfg
}

func TestNewRouterServesHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter(t, testConfig()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Body.String(), `{"condition":"Healthy"}`; got != want+"\n" && got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header missing, middleware chain did not run")
	}
}

func TestNewRouterRejectsBadTrustedProxy(t *testing.T) {
	cfg := testConfig()
	cfg.Server.TrustedProxies = []string{"not-an-ip"}
	log, err := logger.New(logger.Config{ServiceName: "test", Path: t.TempDir()})
	if err != nil {
		t.Fatalf("logger.New: %v", err)
	}
	t.Cleanup(func() { _ = log.Close() })
	if _, err := NewRouter(cfg, log); err == nil {
		t.Fatal("NewRouter accepted an invalid trusted proxy")
	}
}
