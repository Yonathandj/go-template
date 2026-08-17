package container

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/supernurture/go-template/internal/config"
	"github.com/supernurture/go-template/pkg/database"
	"github.com/supernurture/go-template/pkg/redis"
)

// stubOpeners replaces every dependency opener with one that succeeds without a server.
func stubOpeners(t *testing.T) {
	t.Helper()

	openMock := func(string, int, string, string, string, string, database.PoolConfig) (*gorm.DB, error) {
		sqlDB, mock, err := sqlmock.New()
		if err != nil {
			return nil, err
		}
		mock.ExpectClose() // Close unwinds every hook, so the pool is closed too
		return gorm.Open(
			postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}),
			&gorm.Config{DisableAutomaticPing: true},
		)
	}

	postgresOrig, sqlServerOrig, redisOrig := newPostgres, newSQLServer, newRedis
	newPostgres, newSQLServer = openMock, openMock
	newRedis = func(string, int, string, string, int, bool, redis.PoolConfig) (*goredis.Client, error) {
		return goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"}), nil
	}
	t.Cleanup(func() { newPostgres, newSQLServer, newRedis = postgresOrig, sqlServerOrig, redisOrig })
}

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

// Every configured dependency ends up in its map with a shutdown hook behind it.
func TestNewContainerOpensEveryDependency(t *testing.T) {
	stubOpeners(t)

	cfg := testConfig(t)
	cfg.Databases.Postgres = map[string]config.Postgres{"primary": {}}
	cfg.Databases.SQLServer = map[string]config.SQLServer{"legacy": {}}
	cfg.Redis = map[string]config.Redis{"cache": {}}

	c, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer: %v", err)
	}

	if c.Postgres["primary"] == nil || c.SQLServer["legacy"] == nil || c.Redis["cache"] == nil {
		t.Errorf("not every dependency was stored: %+v", c)
	}
	// One hook per dependency, plus the logger's.
	if len(c.shutdowns) != 4 {
		t.Errorf("shutdown hooks = %d, want 4", len(c.shutdowns))
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// Each opener must name which dependency failed, so a bad host is traceable.
func TestNewContainerNamesTheFailingDependency(t *testing.T) {
	tests := map[string]func(cfg *config.Config){
		`postgres "primary"`: func(cfg *config.Config) {
			cfg.Databases.Postgres = map[string]config.Postgres{"primary": {Host: "127.0.0.1", Port: 2, Opts: "connect_timeout=1"}}
		},
		`sql server "legacy"`: func(cfg *config.Config) {
			cfg.Databases.SQLServer = map[string]config.SQLServer{"legacy": {Host: "127.0.0.1", Port: 2}}
		},
		`redis "cache"`: func(cfg *config.Config) {
			cfg.Redis = map[string]config.Redis{"cache": {Host: "127.0.0.1", Port: 2}}
		},
	}

	for want, configure := range tests {
		t.Run(want, func(t *testing.T) {
			cfg := testConfig(t)
			configure(cfg)

			_, err := NewContainer(cfg)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("error = %v, want it to name %s", err, want)
			}
		})
	}
}

// A logger that cannot be built must stop startup before anything else is opened.
func TestNewContainerReportsLoggerFailure(t *testing.T) {
	cfg := testConfig(t)
	// Put a file where the logger wants its directory.
	blocked := filepath.Join(cfg.Logger.Path, cfg.App.Name)
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write %s: %v", blocked, err)
	}

	if _, err := NewContainer(cfg); err == nil || !strings.Contains(err.Error(), "build logger") {
		t.Fatalf("error = %v, want a logger failure", err)
	}
}

func TestCloseGorm(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	mock.ExpectClose()
	db, err := gorm.Open(
		postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}),
		&gorm.Config{DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	if err := closeGorm(db)(); err != nil {
		t.Errorf("closeGorm: %v", err)
	}

	// A connection with no pool underneath must report the failure, not panic.
	if err := closeGorm(&gorm.DB{Config: &gorm.Config{}})(); err == nil {
		t.Error("expected an error from a connection with no pool")
	}
}
