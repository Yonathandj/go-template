package httpclient

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/supernurture/go-template/internal/config"
	"github.com/supernurture/go-template/pkg/logger"
)

func newTestLogger(t *testing.T) (*logger.Logger, func() string) {
	t.Helper()

	dir := t.TempDir()
	log, err := logger.New(logger.Config{ServiceName: "test", Path: dir, Level: "DEBUG"})
	if err != nil {
		t.Fatalf("logger.New: %v", err)
	}
	t.Cleanup(func() { _ = log.Close() })

	return log, func() string {
		if err := log.Close(); err != nil {
			t.Fatalf("close logger: %v", err)
		}
		files, err := filepath.Glob(filepath.Join(dir, "test", "*"))
		if err != nil {
			t.Fatalf("glob %s: %v", dir, err)
		}
		if len(files) == 0 {
			return ""
		}
		contents, err := os.ReadFile(files[0])
		if err != nil {
			t.Fatalf("read log: %v", err)
		}
		return string(contents)
	}
}

func testConfig(baseURL string) *config.Config {
	cfg := &config.Config{}
	cfg.Services = map[string]config.Service{
		"example": {
			BaseURL: baseURL,
			Timeout: 5 * time.Second,
			Auth:    config.ServiceAuth{User: "user", Password: "password"},
		},
	}
	return cfg
}

func TestNewHTTPClientBuildsEveryUpstream(t *testing.T) {
	log, _ := newTestLogger(t)

	clients := NewHTTPClient(testConfig("https://api.example.com"), log)

	if clients.Example == nil {
		t.Error("Example client was not built")
	}
}

func TestWarnsWhenBaseURLIsNotHTTPS(t *testing.T) {
	tests := map[string]struct {
		baseURL  string
		wantWarn bool
	}{
		"plaintext":    {"http://api.example.com", true},
		"no scheme":    {"api.example.com", true},
		"tls":          {"https://api.example.com", false},
		"tls any case": {"HTTPS://api.example.com", false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			log, logged := newTestLogger(t)

			NewHTTPClient(testConfig(tc.baseURL), log)

			warned := strings.Contains(logged(), "not HTTPS")
			if warned != tc.wantWarn {
				t.Errorf("warned = %v, want %v for %q", warned, tc.wantWarn, tc.baseURL)
			}
		})
	}
}

func TestNewHTTPClientWithoutServiceConfig(t *testing.T) {
	log, _ := newTestLogger(t)

	if clients := NewHTTPClient(&config.Config{}, log); clients.Example == nil {
		t.Error("Example client was not built from an empty config")
	}
}
