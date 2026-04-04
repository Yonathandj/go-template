package config

import (
	"fmt"
	"strings"
	"time"

	env "github.com/joho/godotenv"
	"github.com/spf13/viper"
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

func loadFile() error {
	viper.SetConfigName("")
	viper.SetConfigType("")
	viper.AddConfigPath("")

	err := viper.ReadInConfig()
	if err != nil {
		return fmt.Errorf("unable to load the configuration file: %w", err)
	}

	return nil
}

func loadConsul() error {
	return nil
}

func Load() (*Config, error) {
	var (
		config      Config
		environment string
	)

	err := env.Load("")
	if err != nil {
		fmt.Print("unable to load the .env file, relying on system environment variables")
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	environment = viper.GetString("application.environment")
	switch environment {
	case "development":
		loadFile()
	case "sit", "uat", "production":
		loadConsul()
	default:
		loadFile()
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal the config into the struct: %w", err)
	}

	return &config, nil
}
