package config

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config struct defines the structure of the configuration file.
type Config struct {
	App       App                `mapstructure:"app"       validate:"required"`
	Server    Server             `mapstructure:"server"    validate:"required"`
	Databases Databases          `mapstructure:"databases"`
	Services  map[string]Service `mapstructure:"services"  validate:"omitempty,dive"`
	Worker    map[string]Worker  `mapstructure:"worker"    validate:"omitempty,dive"`
	Logger    Logger             `mapstructure:"logger"    validate:"required"`
}

// App holds application identity and environment.
type App struct {
	Name    string `mapstructure:"name"    validate:"required"`
	Version string `mapstructure:"version" validate:"required"`
	Env     string `mapstructure:"env"     validate:"required,oneof=development staging production"`
}

// Server holds HTTP server mode, port, request timeout, and middleware settings.
type Server struct {
	Mode           string        `mapstructure:"mode"            validate:"required,oneof=test debug release"`
	Port           int           `mapstructure:"port"            validate:"required,min=1,max=65535"`
	Timeout        time.Duration `mapstructure:"timeout"         validate:"required,gt=0"`
	CORSOrigins    []string      `mapstructure:"cors_origins"`
	TrustedProxies []string      `mapstructure:"trusted_proxies" validate:"omitempty,dive,ip|cidr"`
	MaxBodyBytes   int64         `mapstructure:"max_body_bytes"  validate:"gte=0"`
}

// Databases holds every configured datastore, keyed by logical name, plus the pool settings shared by all of them.
type Databases struct {
	Pool      Pool                 `mapstructure:"pool"`
	Postgres  map[string]Postgres  `mapstructure:"postgres"   validate:"omitempty,dive"`
	SQLServer map[string]SQLServer `mapstructure:"sql_server" validate:"omitempty,dive"`
}

// Pool holds shared connection-pool settings; a zero value keeps the driver default.
type Pool struct {
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// Postgres holds connection settings for a PostgreSQL database.
type Postgres struct {
	Host     string `mapstructure:"host"     validate:"required"`
	Port     int    `mapstructure:"port"     validate:"required,min=1,max=65535"`
	User     string `mapstructure:"user"     validate:"required"`
	Password string `mapstructure:"password" validate:"required"`
	Database string `mapstructure:"database" validate:"required"`
	Opts     string `mapstructure:"opts"`
}

// SQLServer holds connection settings for a SQL Server database.
type SQLServer struct {
	Host     string `mapstructure:"host"     validate:"required"`
	Port     int    `mapstructure:"port"     validate:"required,min=1,max=65535"`
	User     string `mapstructure:"user"     validate:"required"`
	Password string `mapstructure:"password" validate:"required"`
	Database string `mapstructure:"database" validate:"required"`
	Opts     string `mapstructure:"opts"`
}

// Service holds the base URL, endpoints, timeout, and auth for an upstream service.
type Service struct {
	BaseURL   string            `mapstructure:"base_url"  validate:"required"`
	Endpoints map[string]string `mapstructure:"endpoints" validate:"required"`
	Timeout   time.Duration     `mapstructure:"timeout"   validate:"required,gt=0"`
	Auth      ServiceAuth       `mapstructure:"auth"`
}

// ServiceAuth holds basic-auth credentials for a service.
type ServiceAuth struct {
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
}

// Worker holds a scheduled worker's enable flag and run interval.
type Worker struct {
	Enabled      bool          `mapstructure:"enabled"`
	TimeInterval time.Duration `mapstructure:"time_interval" validate:"required,gt=0"`
}

// Logger holds log output, level, and rotation settings.
type Logger struct {
	Path            string `mapstructure:"path"`
	Level           string `mapstructure:"level"            validate:"omitempty,oneof=DEBUG INFO WARN ERROR"`
	RotationPattern string `mapstructure:"rotation_pattern" validate:"omitempty,oneof=daily size"`
	RotationSizeMB  int    `mapstructure:"rotation_size_mb" validate:"gte=0"`
	RetentionDays   int    `mapstructure:"retention_days"   validate:"gte=0"`
	Console         bool   `mapstructure:"console"`
}

const (
	dotEnvPath = ".env"

	configName = "config"
	configType = "yaml"
	configPath = "configs/"
)

// Load reads configs/config.yaml, overlays .env and environment variables, and validates the result before returning it.
func Load() (*Config, error) {
	if err := godotenv.Load(dotEnvPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("load %s: %w", dotEnvPath, err)
	}

	vip := viper.New()
	vip.AutomaticEnv()
	vip.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	vip.SetConfigName(configName)
	vip.SetConfigType(configType)
	vip.AddConfigPath(configPath)

	if err := vip.ReadInConfig(); err != nil {
		if _, notFound := errors.AsType[viper.ConfigFileNotFoundError](err); notFound {
			return nil, fmt.Errorf("no %s%s.%s: copy %[1]sconfig.example.%[3]s and fill it in", configPath, configName, configType)
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := vip.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := validator.New().Struct(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}
