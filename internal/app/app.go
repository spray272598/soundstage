package app

import (
	"context"

	"github.com/spray272598/soundstage/internal/pkg/config"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
	"github.com/spray272598/soundstage/internal/pkg/redis"
	"go.uber.org/zap"
)

// Application holds the runtime dependencies of the modular monolith.
type Application struct {
	Config *config.Config
	Redis  *redis.Client
}

// New builds and wires the application.
func New(cfg *config.Config) (*Application, error) {
	if err := logger.Init(cfg.Log.Level, cfg.Log.Format); err != nil {
		return nil, err
	}

	rdb := redis.New(cfg.Redis.Addr, cfg.Redis.DB, cfg.Redis.PoolSize)
	if err := rdb.Ping(context.Background()); err != nil {
		return nil, err
	}

	return &Application{
		Config: cfg,
		Redis:  rdb,
	}, nil
}

// Run starts all servers and blocks until the context is canceled.
func (a *Application) Run(ctx context.Context) error {
	logger.L().Info("soundstage starting",
		zap.String("version", a.Config.App.Version),
		zap.String("env", a.Config.App.Env))
	_ = metrics.Registry()
	// TODO: start HTTP, WebSocket, metrics servers and task workers.
	<-ctx.Done()
	return nil
}

// Shutdown releases resources gracefully.
func (a *Application) Shutdown(ctx context.Context) error {
	logger.L().Info("soundstage shutting down")
	return a.Redis.Close()
}
