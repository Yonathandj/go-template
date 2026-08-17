package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// validConfig is the smallest config that passes validation: every optional block omitted.
const validConfig = `
app:
  name: template
  version: 1.0.0
  env: development
server:
  mode: test
  port: 8080
  timeout: 40s
  trusted_proxies: ["10.0.0.0/8"]
logger:
  level: INFO
`

// chdirTemp runs the test from an empty directory, so Load only ever sees files it writes.
func chdirTemp(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
}

func writeFile(t *testing.T, name, contents string) {
	t.Helper()

	if dir := filepath.Dir(name); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(name, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// configFile is the path Load reads, built from the same constants Load uses.
func configFile() string {
	return filepath.Join(configPath, configName+"."+configType)
}

func TestLoad(t *testing.T) {
	chdirTemp(t)
	writeFile(t, configFile(), validConfig)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := cfg.App.Name; got != "template" {
		t.Errorf("app.name = %q, want template", got)
	}
	if got := cfg.Server.Timeout; got != 40*time.Second {
		t.Errorf("server.timeout = %v, want 40s", got)
	}
	if got := cfg.Server.TrustedProxies; len(got) != 1 || got[0] != "10.0.0.0/8" {
		t.Errorf("server.trusted_proxies = %v, want [10.0.0.0/8]", got)
	}
	// Services, Worker and Databases are optional; omitting them must not fail validation.
	if len(cfg.Services) != 0 || len(cfg.Worker) != 0 {
		t.Errorf("services = %v, worker = %v, want both empty", cfg.Services, cfg.Worker)
	}
}

// Pool sizing now lives on each datastore, and a mistyped mapstructure tag would silently leave it zero.
const datastoreConfig = validConfig + `
databases:
  postgres:
    primary:
      host: db.internal
      port: 5432
      user: app
      password: secret
      database: app
      max_open_conns: 25
      max_idle_conns: 5
      conn_max_lifetime: 30m
redis:
  cache:
    host: cache.internal
    port: 6379
    db: 2
    pool_size: 20
    min_idle_conns: 4
    conn_max_lifetime: 1h
`

func TestLoadDatastorePools(t *testing.T) {
	chdirTemp(t)
	writeFile(t, configFile(), datastoreConfig)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	postgres, ok := cfg.Databases.Postgres["primary"]
	if !ok {
		t.Fatalf("databases.postgres = %v, want a %q entry", cfg.Databases.Postgres, "primary")
	}
	if postgres.MaxOpenConns != 25 || postgres.MaxIdleConns != 5 || postgres.ConnMaxLifetime != 30*time.Minute {
		t.Errorf("postgres pool = %+v, want 25/5/30m", postgres)
	}

	cache, ok := cfg.Redis["cache"]
	if !ok {
		t.Fatalf("redis = %v, want a %q entry", cfg.Redis, "cache")
	}
	if cache.DB != 2 || cache.PoolSize != 20 || cache.MinIdleConns != 4 || cache.ConnMaxLifetime != time.Hour {
		t.Errorf("redis pool = %+v, want db 2 and 20/4/1h", cache)
	}
}

func TestLoadEnvOverridesFile(t *testing.T) {
	chdirTemp(t)
	writeFile(t, configFile(), validConfig)
	t.Setenv("APP_NAME", "from-env")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.App.Name; got != "from-env" {
		t.Errorf("app.name = %q, want the environment to win", got)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name:  "unparsable .env",
			files: map[string]string{dotEnvPath: "neither an assignment nor a comment\n"},
			want:  dotEnvPath,
		},
		{
			name:  "no config file",
			files: nil,
			want:  "config.example",
		},
		{
			name:  "unparsable yaml",
			files: map[string]string{configFile(): "app: [2, 4\n"},
			want:  "read config file",
		},
		{
			name:  "value the struct cannot hold",
			files: map[string]string{configFile(): "server:\n  timeout: banana\n"},
			want:  "unmarshal config",
		},
		{
			name:  "fails validation",
			files: map[string]string{configFile(): "app:\n  name: template\n"},
			want:  "invalid config",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chdirTemp(t)
			for name, contents := range test.files {
				writeFile(t, name, contents)
			}

			cfg, err := Load()
			if err == nil {
				t.Fatalf("Load returned %+v, want an error", cfg)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("err = %v, want it to mention %q", err, test.want)
			}
		})
	}
}
