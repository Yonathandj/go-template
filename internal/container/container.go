package container

import (
	"errors"
	"fmt"

	"github.com/supernurture/go-template/internal/config"
	"github.com/supernurture/go-template/pkg/logger"
)

// Container holds shared infrastructure dependencies used across the app.
type Container struct {
	Logger *logger.Logger

	shutdowns []func() error
}

// NewContainer builds the Container: logger, database connections, HTTP client, and their shutdown hooks. Call Close when done.
func NewContainer(cfg *config.Config) (*Container, error) {
	log, err := logger.New(logger.Config{
		ServiceName: cfg.App.Name,
		Env:         cfg.App.Env,
		Path:        cfg.Logger.Path,
		Level:       cfg.Logger.Level,
		Console:     cfg.Logger.Console,
		Rotation: logger.RotationOptions{
			Daily:      cfg.Logger.RotationPattern == "daily",
			MaxSizeMB:  cfg.Logger.RotationSizeMB,
			MaxAgeDays: cfg.Logger.RetentionDays,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}
	return &Container{
		Logger: log,
		// Shutdown hooks run in order, logger last so earlier hooks can still log.
		shutdowns: []func() error{log.Close},
	}, nil
}

// Close runs every registered shutdown hook, continuing past failures so one stuck dependency cannot strand the rest.
func (c *Container) Close() error {
	var errs []error
	for _, shutdown := range c.shutdowns {
		errs = append(errs, shutdown())
	}
	return errors.Join(errs...)
}
