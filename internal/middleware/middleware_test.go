package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/supernurture/go-template/internal/config"
	"github.com/supernurture/go-template/pkg/logger"
)

// newRouter mounts the default chain on a router with one echo route and one panicking route.
func newRouter(t *testing.T, cfg *config.Config) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	log, err := logger.New(logger.Config{ServiceName: "test", Path: t.TempDir()})
	if err != nil {
		t.Fatalf("logger.New: %v", err)
	}
	t.Cleanup(func() { _ = log.Close() })

	router := gin.New()
	router.Use(Default(cfg, log)...)
	router.POST("/echo", func(c *gin.Context) {
		body, err := c.GetRawData()
		if err != nil {
			c.String(http.StatusRequestEntityTooLarge, "too large")
			return
		}
		c.String(http.StatusOK, "%s|%s", RequestIDFrom(c.Request.Context()), body)
	})
	router.POST("/bind", func(c *gin.Context) {
		var req struct {
			Name string `json:"name"`
		}
		if Bind(c, &req) {
			c.String(http.StatusOK, req.Name)
		}
	})
	router.GET("/boom", func(*gin.Context) { panic("boom") })
	router.GET("/abort", func(*gin.Context) { panic(http.ErrAbortHandler) })
	router.GET("/slow", func(c *gin.Context) { <-c.Request.Context().Done() }) // waits on ctx, writes nothing

	return router
}

func do(router *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestDefaultChain(t *testing.T) {
	cfg := &config.Config{}
	cfg.App.Env = "development"
	cfg.Server.CORSOrigins = []string{"https://app.example.com"}
	cfg.Server.MaxBodyBytes = 16
	cfg.Server.Timeout = 50 * time.Millisecond
	router := newRouter(t, cfg)

	t.Run("generates and echoes a request id", func(t *testing.T) {
		rec := do(router, httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("hi")))

		id := rec.Header().Get(requestIDHeader)
		if id == "" {
			t.Fatal("want a generated request id header")
		}
		if got, want := rec.Body.String(), id+"|hi"; got != want {
			t.Errorf("body = %q, want %q (id must reach the handler context)", got, want)
		}
	})

	t.Run("rejects a hostile inbound request id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("hi"))
		req.Header.Set(requestIDHeader, "ok-1")
		if got := do(router, req).Header().Get(requestIDHeader); got != "ok-1" {
			t.Errorf("sane id = %q, want it reused", got)
		}

		req = httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("hi"))
		req.Header.Set(requestIDHeader, strings.Repeat("x", maxRequestIDLen+1))
		if got := do(router, req).Header().Get(requestIDHeader); len(got) != requestIDLength {
			t.Errorf("oversized id = %q, want a fresh generated one", got)
		}
	})

	t.Run("request id reaches the middleware chain, not just the handler", func(t *testing.T) {
		// Reading it off the *gin.Context silently yields "" unless engine.ContextWithFallback is set.
		router := gin.New()
		router.Use(RequestID())
		var got string
		router.GET("/id", func(c *gin.Context) { got = RequestIDFrom(c.Request.Context()) })
		rec := do(router, httptest.NewRequest(http.MethodGet, "/id", nil))

		if want := rec.Header().Get(requestIDHeader); got == "" || got != want {
			t.Errorf("id seen by the chain = %q, want the echoed %q", got, want)
		}
	})

	t.Run("recovers panics as 500", func(t *testing.T) {
		rec := do(router, httptest.NewRequest(http.MethodGet, "/boom", nil))
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})

	t.Run("lets ErrAbortHandler through instead of logging it as a 500", func(t *testing.T) {
		defer func() {
			if got := recover(); got != http.ErrAbortHandler {
				t.Errorf("recovered %v, want ErrAbortHandler to propagate", got)
			}
		}()
		do(router, httptest.NewRequest(http.MethodGet, "/abort", nil))
	})

	t.Run("cancels an overrunning request with 504", func(t *testing.T) {
		rec := do(router, httptest.NewRequest(http.MethodGet, "/slow", nil))
		if rec.Code != http.StatusGatewayTimeout {
			t.Errorf("status = %d, want 504", rec.Code)
		}
	})

	t.Run("sets security headers without hsts outside production", func(t *testing.T) {
		rec := do(router, httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("hi")))
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
		}
		if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
			t.Errorf("HSTS = %q, want none in development", got)
		}
	})

	t.Run("answers preflight only for allowed origins", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/echo", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rec := do(router, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
			t.Errorf("allow-origin = %q, want the request origin", got)
		}

		req = httptest.NewRequest(http.MethodOptions, "/echo", nil)
		req.Header.Set("Origin", "https://evil.example.com")
		if got := do(router, req).Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("allow-origin = %q, want empty for a disallowed origin", got)
		}
	})

	t.Run("preflight echoes the requested headers and exposes the request id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/echo", nil)
		req.Header.Set("Origin", "https://app.example.com")
		req.Header.Set("Access-Control-Request-Headers", "X-Api-Key")
		rec := do(router, req)

		if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "X-Api-Key" {
			t.Errorf("allow-headers = %q, want the requested header", got)
		}
		if got := rec.Header().Get("Access-Control-Expose-Headers"); got != requestIDHeader {
			t.Errorf("expose-headers = %q, want %s", got, requestIDHeader)
		}
	})

	t.Run("allows a simple cross-origin request through to the handler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("hi"))
		req.Header.Set("Origin", "https://app.example.com")
		rec := do(router, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Errorf("allow-credentials = %q, want true for an explicit origin", got)
		}
	})

	t.Run("replaces an inbound request id that is not printable", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("hi"))
		req.Header.Set(requestIDHeader, "bad\x01id")
		rec := do(router, req)

		if got := rec.Header().Get(requestIDHeader); got == "bad\x01id" || got == "" {
			t.Errorf("request id = %q, want a generated replacement", got)
		}
	})

	t.Run("bind maps body errors to 413 and 400", func(t *testing.T) {
		cases := []struct {
			name string
			body string
			want int
		}{
			{"valid", `{"name":"ok"}`, http.StatusOK},
			{"malformed", `{`, http.StatusBadRequest},
			{"over the cap", `{"name":"` + strings.Repeat("x", 32) + `"}`, http.StatusRequestEntityTooLarge},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, "/bind", strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
				if rec := do(router, req); rec.Code != tc.want {
					t.Errorf("status = %d, want %d", rec.Code, tc.want)
				}
			})
		}
	})

	t.Run("caps the request body", func(t *testing.T) {
		body := strings.NewReader(strings.Repeat("x", int(cfg.Server.MaxBodyBytes)+1))
		if rec := do(router, httptest.NewRequest(http.MethodPost, "/echo", body)); rec.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("status = %d, want 413", rec.Code)
		}
	})
}

// TestDefaultChainUnset covers the other side of every knob: production env, no timeout, no body cap.
func TestDefaultChainUnset(t *testing.T) {
	cfg := &config.Config{}
	cfg.App.Env = "production"
	router := newRouter(t, cfg)

	t.Run("sets hsts in production", func(t *testing.T) {
		rec := do(router, httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("hi")))
		if got := rec.Header().Get("Strict-Transport-Security"); got == "" {
			t.Error("HSTS header missing in production")
		}
	})

	t.Run("serves a request with the timeout disabled", func(t *testing.T) {
		rec := do(router, httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("hi")))
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("falls back to the default body cap", func(t *testing.T) {
		body := strings.NewReader(strings.Repeat("x", defaultMaxBodyBytes+1))
		if rec := do(router, httptest.NewRequest(http.MethodPost, "/echo", body)); rec.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("status = %d, want 413 past the %d byte default", rec.Code, defaultMaxBodyBytes)
		}
	})
}
