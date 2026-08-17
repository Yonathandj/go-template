package container

import (
	"errors"
	"strings"
	"testing"

	"github.com/supernurture/go-template/internal/config"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.App.Name = "test"
	cfg.Logger.Path = t.TempDir()
	return cfg
}

// Nothing configured is a valid setup: the template has to start without a database or cache.
func TestNewContainerWithoutDependencies(t *testing.T) {
	c, err := NewContainer(testConfig(t))
	if err != nil {
		t.Fatalf("NewContainer: %v", err)
	}
	if c.Logger == nil || c.HTTPClient == nil {
		t.Error("logger and HTTP client should always be built")
	}
	if len(c.Postgres) != 0 || len(c.SQLServer) != 0 || len(c.Redis) != 0 {
		t.Errorf("unconfigured dependencies should be absent: %+v", c)
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestCloseUnwindsInReverse(t *testing.T) {
	var order []string
	c := &Container{shutdowns: []func() error{
		func() error { order = append(order, "logger"); return nil },
		func() error { order = append(order, "postgres"); return errors.New("boom") },
		func() error { order = append(order, "redis"); return nil },
	}}

	err := c.Close()

	// Reverse order keeps the logger alive until every hook that might log has finished.
	if got, want := strings.Join(order, ","), "redis,postgres,logger"; got != want {
		t.Errorf("shutdown order = %q, want %q", got, want)
	}
	// A failing hook must not stop the ones after it.
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("Close error = %v, want it to carry the hook failure", err)
	}
}

func TestNewContainerReportsBadDependency(t *testing.T) {
	cfg := testConfig(t)
	cfg.Redis = map[string]config.Redis{"example": {Host: "127.0.0.1", Port: 2}}

	if _, err := NewContainer(cfg); err == nil || !strings.Contains(err.Error(), `redis "example"`) {
		t.Fatalf("error = %v, want it to name the dependency that failed", err)
	}
}
