package config

import (
	"time"
)

type ServerConfig struct {
	Port    int
	Timeout time.Duration
}

type DatabaseConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Name     string
}

type DatabasesConfig struct {
	Postgre map[string]DatabaseConfig
}

type LoggerConfig struct {
	Format string
	Level  string
	Output string
}

type Config struct {
	Server    ServerConfig
	Databases DatabasesConfig
	Logger    LoggerConfig
}

func Load() (*Config, error) {
	return nil, nil
}
