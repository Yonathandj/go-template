// Package middleware holds the standard gin chain every API route gets:
// request ID, access log, panic recovery, security headers, CORS, body cap.
package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/supernurture/go-template/internal/config"
	"github.com/supernurture/go-template/internal/pkg/util"
	"github.com/supernurture/go-template/pkg/logger"
)

const (
	requestIDHeader = "X-Request-ID"
	requestIDLength = 16
	maxRequestIDLen = 64

	// defaultMaxBodyBytes caps request bodies when server.max_body_bytes is unset.
	defaultMaxBodyBytes = 1 << 20 // 1 MiB
)

type requestIDCtxKey struct{}

// Default returns the standard chain in execution order.
// Mount it with router.Use(middleware.Default(cfg, log)...).
func Default(cfg *config.Config, log *logger.Logger) []gin.HandlerFunc {
	maxBody := cfg.Server.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = defaultMaxBodyBytes
	}

	return []gin.HandlerFunc{
		RequestID(),
		AccessLog(log), // outside Recovery so it logs the 500 Recovery writes
		Recovery(log),
		Timeout(cfg.Server.Timeout),
		SecurityHeaders(cfg.App.Env == "production"),
		CORS(cfg.Server.CORSOrigins),
		MaxBodyBytes(maxBody),
	}
}

// RequestID reuses a sane inbound X-Request-ID, generates one otherwise, and
// echoes it on the response. Read it back with RequestIDFrom.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if !validRequestID(id) {
			// ponytail: only errors if crypto/rand fails; not worth aborting a request over.
			id, _ = util.GenerateUniqueID(requestIDLength)
		}

		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), requestIDCtxKey{}, id))
		c.Header(requestIDHeader, id)
		c.Next()
	}
}

// RequestIDFrom returns the request ID, or "" outside a request. Pass
// c.Request.Context(), not the *gin.Context: gin.Context.Value only falls
// through to the request context when engine.ContextWithFallback is set.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDCtxKey{}).(string)
	return id
}

// AccessLog logs one line per request: 5xx as error, 4xx as warn, rest as info.
func AccessLog(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		fields := map[string]any{
			"request_id": RequestIDFrom(c.Request.Context()),
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path, // ponytail: no query string, it often carries secrets
			"status":     status,
			"latency_ms": time.Since(start).Milliseconds(),
			"client_ip":  c.ClientIP(),
		}
		if errs := c.Errors.String(); errs != "" {
			fields["error"] = errs
		}

		switch {
		case status >= http.StatusInternalServerError:
			log.Error("request", fields)
		case status >= http.StatusBadRequest:
			log.Warn("request", fields)
		default:
			log.Info("request", fields)
		}
	}
}

// Recovery turns a panic into a logged 500 instead of a dropped connection.
func Recovery(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("panic recovered", map[string]any{
					"request_id": RequestIDFrom(c.Request.Context()),
					"panic":      fmt.Sprint(err),
					"stack":      string(debug.Stack()),
				})
				// ponytail: no broken-pipe special case; a dead peer just costs one failed write.
				c.AbortWithStatusJSON(
					http.StatusInternalServerError, gin.H{"message": "internal server error"})
			}
		}()
		c.Next()
	}
}

// Timeout puts a deadline on the request context so a slow query or upstream
// call is cancelled once the client is no longer waiting. A non-positive
// duration disables it.
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if timeout <= 0 {
			c.Next()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		// ponytail: the deadline only bites for handlers that pass ctx down, which is
		// where the waiting happens anyway; run the handler in a goroutine (see
		// gin-contrib/timeout) only if a hard cut for blocking handlers is needed.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) && !c.Writer.Written() {
			c.AbortWithStatusJSON(http.StatusGatewayTimeout, gin.H{"message": "request timeout"})
		}
	}
}

// SecurityHeaders sets the baseline response headers. hsts should only be true
// when the service is served over HTTPS.
func SecurityHeaders(hsts bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		if hsts {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// CORS echoes an allowed Origin and answers preflight. An empty allowlist
// disables CORS; "*" allows any origin but then never allows credentials.
func CORS(allowed []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		// Vary before the allow check: the response body/headers depend on Origin
		// either way, so a shared cache must not reuse one origin's response for another.
		c.Writer.Header().Add("Vary", "Origin")
		if !originAllowed(allowed, origin) {
			c.Next()
			return
		}

		c.Header("Access-Control-Allow-Origin", origin)
		if !slices.Contains(allowed, "*") {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, "+requestIDHeader)
			c.Header("Access-Control-Max-Age", "600")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// MaxBodyBytes caps the request body so a huge upload cannot exhaust memory.
func MaxBodyBytes(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limit > 0 && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}

// validRequestID accepts only short, printable, single-line IDs: the header is
// attacker-controlled and ends up in both the logs and a response header.
func validRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	for _, r := range id {
		if r < '!' || r > '~' {
			return false
		}
	}
	return true
}

func originAllowed(allowed []string, origin string) bool {
	for _, a := range allowed {
		if a == "*" || strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}
