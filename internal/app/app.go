package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	conninfra "github.com/spray272598/soundstage/internal/connection/infrastructure"
	connapp "github.com/spray272598/soundstage/internal/connection/application"
	conntransport "github.com/spray272598/soundstage/internal/connection/transport"
	"github.com/spray272598/soundstage/internal/infrastructure/kafka"
	"github.com/spray272598/soundstage/internal/pkg/config"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
	pkgkafka "github.com/spray272598/soundstage/internal/pkg/kafka"
	"github.com/spray272598/soundstage/internal/pkg/redis"
	"github.com/spray272598/soundstage/internal/room/application"
	"github.com/spray272598/soundstage/internal/room/infrastructure/persistence"
	"github.com/spray272598/soundstage/internal/room/transport"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Application holds the runtime dependencies of the modular monolith.
type Application struct {
	Config      *config.Config
	DB          *gorm.DB
	Redis       *redis.Client
	Producer    pkgkafka.Producer
	RoomHandler *transport.RoomHandler
	WSHandler   *conntransport.WSHandler
}

// New builds and wires the application.
func New(cfg *config.Config) (*Application, error) {
	if err := logger.Init(cfg.Log.Level, cfg.Log.Format); err != nil {
		return nil, err
	}

	db, err := newDB(cfg.MySQL.DSN)
	if err != nil {
		return nil, fmt.Errorf("mysql: %w", err)
	}

	rdb := redis.New(cfg.Redis.Addr, cfg.Redis.DB, cfg.Redis.PoolSize)
	if err := rdb.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}

	roomRepo := persistence.NewGormRoomRepository(db)
	roomSvc := application.NewRoomService(roomRepo)
	roomHandler := transport.NewRoomHandler(roomSvc)

	hub := conninfra.NewHub()
	connSvc := connapp.NewConnectionService(hub)
	wsHandler := conntransport.NewWSHandler(connSvc)

	producer := kafka.NewProducer(cfg.Kafka.Brokers)

	return &Application{
		Config:      cfg,
		DB:          db,
		Redis:       rdb,
		Producer:    producer,
		RoomHandler: roomHandler,
		WSHandler:   wsHandler,
	}, nil
}

// Run starts all servers and blocks until the context is canceled.
func (a *Application) Run(ctx context.Context) error {
	logger.L().Info("soundstage starting",
		zap.String("version", a.Config.App.Version),
		zap.String("env", a.Config.App.Env))

	// Auto-migrate domain models (dev only; migrations should be explicit in production).
	if err := a.DB.AutoMigrate(&persistence.RoomModel{}); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	// Metrics server.
	go func() {
		mux := http.NewServeMux()
		mux.Handle(a.Config.Metrics.Path, metrics.HTTPHandler())
		addr := a.Config.Metrics.Addr
		logger.L().Info("metrics server listening", zap.String("addr", addr))
		if err := http.ListenAndServe(addr, mux); err != nil && err != http.ErrServerClosed {
			logger.L().Error("metrics server error", zap.Error(err))
		}
	}()

	// HTTP API server.
	router := gin.New()
	router.Use(gin.Recovery())
	a.RoomHandler.Register(router)
	a.WSHandler.Register(router)

	httpServer := &http.Server{
		Addr:    a.Config.HTTP.Addr,
		Handler: router,
	}

	go func() {
		logger.L().Info("http server listening", zap.String("addr", a.Config.HTTP.Addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.L().Error("http server error", zap.Error(err))
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

// Shutdown releases resources gracefully.
func (a *Application) Shutdown(ctx context.Context) error {
	logger.L().Info("soundstage shutting down")
	_ = a.Producer.Close()
	sqlDB, err := a.DB.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
	return a.Redis.Close()
}

func newDB(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	return db, nil
}
