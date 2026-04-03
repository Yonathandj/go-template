package config

import (
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type ServerConfig struct {
	Port    int
	Timeout time.Duration
}

type DatabaseConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	Name     string
}

type DatabasesConfig struct {
	Postgre map[string]DatabaseConfig
}

type Config struct {
	Server    ServerConfig
	Databases DatabasesConfig
}

const (
	ENV_LOCATION = "../../.env"

	CONFIG_NAME     = "config"
	CONFIG_TYPE     = "yaml"
	CONFIG_LOCATION = "../../configs/"
)

func Load() (*Config, error) {
	var (
		config *Config
		err    error
	)

	err = godotenv.Load(ENV_LOCATION)
	if err != nil {
	}

	viper.SetConfigName(CONFIG_NAME)
	viper.SetConfigType(CONFIG_TYPE)
	viper.AddConfigPath(CONFIG_LOCATION)

	err = viper.ReadInConfig()
	if err != nil {
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err = viper.Unmarshal(&config)
	if err != nil {
	}

	return config, nil
}
