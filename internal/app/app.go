package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/spray272598/soundstage/internal/connection/application"
	connDomain "github.com/spray272598/soundstage/internal/connection/domain"
	conninfra "github.com/spray272598/soundstage/internal/connection/infrastructure"
	conntransport "github.com/spray272598/soundstage/internal/connection/transport"
	interactionapp "github.com/spray272598/soundstage/internal/interaction/application"
	interactioncache "github.com/spray272598/soundstage/internal/interaction/infrastructure/cache"
	interactionmod "github.com/spray272598/soundstage/internal/interaction/infrastructure/moderation"
	interactionmsg "github.com/spray272598/soundstage/internal/interaction/infrastructure/messaging"
	interactionpersist "github.com/spray272598/soundstage/internal/interaction/infrastructure/persistence"
	asynqworker "github.com/spray272598/soundstage/internal/interaction/infrastructure/asynqworker"
	interactiontask "github.com/spray272598/soundstage/internal/interaction/task"
	interactiontransport "github.com/spray272598/soundstage/internal/interaction/transport"
	"github.com/spray272598/soundstage/internal/infrastructure/kafka"
	"github.com/spray272598/soundstage/internal/pkg/config"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
	pkgkafka "github.com/spray272598/soundstage/internal/pkg/kafka"
	"github.com/spray272598/soundstage/internal/pkg/redis"
	roomapp "github.com/spray272598/soundstage/internal/room/application"
	roompersist "github.com/spray272598/soundstage/internal/room/infrastructure/persistence"
	roomtransport "github.com/spray272598/soundstage/internal/room/transport"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Application holds the runtime dependencies of the modular monolith.
type Application struct {
	Config    *config.Config
	DB        *gorm.DB
	Redis     *redis.Client
	Hub       connDomain.Hub
	Producer  pkgkafka.Producer
	Consumer  pkgkafka.Consumer
	Asynq     *asynq.Client
	RoomHandler *roomtransport.RoomHandler
	WSHandler   *conntransport.WSHandler
	InterHandler *interactiontransport.InteractionHandler

	ingestConsumer *interactionapp.IngestConsumer
	asynqMux       *asynq.ServeMux
	asynqServer    *asynq.Server
	scheduler      *asynq.Scheduler
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

	// --- room context ---
	roomRepo := roompersist.NewGormRoomRepository(db)
	roomSvc := roomapp.NewRoomService(roomRepo)
	roomHandler := roomtransport.NewRoomHandler(roomSvc)

	// --- connection context ---
	hub := conninfra.NewHub()
	producer := kafka.NewProducer(cfg.Kafka.Brokers)
	broadcastTopic := cfg.Kafka.TopicPrefix + "broadcast"
	ingestTopic := cfg.Kafka.TopicPrefix + "ingest"
	connSvc := application.NewConnectionService(hub, producer, ingestTopic)
	wsHandler := conntransport.NewWSHandler(connSvc)
	consumer := kafka.NewConsumer(cfg.Kafka.Brokers, "soundstage-gateway")

	// --- interaction context ---
	giftRepo := interactionpersist.NewGormGiftRepository(db)
	orderRepo := interactionpersist.NewGormGiftOrderRepository(db)
	danmakuRepo := interactionpersist.NewGormDanmakuRepository(db)
	roomStatsRepo := interactionpersist.NewGormRoomStatsRepository(db)

	rankStore := interactioncache.NewRedisRankStore(rdb)
	likeCounter := interactioncache.NewRedisLikeCounter(rdb)
	rateLimiter := interactioncache.NewRedisRateLimiter(rdb)
	moderator := interactionmod.NewKeywordModerator(cfg.Interaction.ModerationKeywords)
	broadcaster := interactionmsg.NewKafkaBroadcaster(producer, broadcastTopic)

	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.Asynq.RedisAddr})
	enqueuer := interactionmsg.NewAsynqTaskEnqueuer(asynqClient)

	interSvc := interactionapp.NewInterService(
		giftRepo, orderRepo, danmakuRepo,
		moderator, rateLimiter, rankStore, likeCounter,
		broadcaster, enqueuer,
		interactionapp.InterServiceConfig{
			DanmakuRateLimit:  cfg.Interaction.DanmakuRateLimit,
			DanmakuRateWindow: cfg.Interaction.DanmakuRateWindow,
		},
	)
	interHandler := interactiontransport.NewInteractionHandler(interSvc)
	ingestConsumer := interactionapp.NewIngestConsumer(interSvc)

	// --- asynq worker + scheduler ---
	asynqServer := asynqworker.NewServer(cfg.Asynq.RedisAddr)
	workerDeps := asynqworker.Deps{
		Danmaku: danmakuRepo,
		Orders:  orderRepo,
		Rank:    rankStore,
		Likes:   likeCounter,
		Stats:   roomStatsRepo,
		RDB:     rdb,
	}
	mux := asynq.NewServeMux()
	workerDeps.Register(mux)

	scheduler := asynqworker.NewScheduler(cfg.Asynq.RedisAddr)
	if _, err := scheduler.Register("@every 30s", asynq.NewTask(interactiontask.TypeFlushLikes, nil)); err != nil {
		return nil, fmt.Errorf("scheduler register: %w", err)
	}

	return &Application{
		Config:       cfg,
		DB:           db,
		Redis:        rdb,
		Hub:          hub,
		Producer:     producer,
		Consumer:     consumer,
		Asynq:        asynqClient,
		RoomHandler:  roomHandler,
		WSHandler:    wsHandler,
		InterHandler: interHandler,
		ingestConsumer: ingestConsumer,
		asynqMux:       mux,
		asynqServer:    asynqServer,
		scheduler:      scheduler,
	}, nil
}

// Run starts all servers and blocks until the context is canceled.
func (a *Application) Run(ctx context.Context) error {
	logger.L().Info("soundstage starting",
		zap.String("version", a.Config.App.Version),
		zap.String("env", a.Config.App.Env))

	// Auto-migrate domain models (dev only; migrations should be explicit in production).
	if err := a.DB.AutoMigrate(
		&roompersist.RoomModel{},
		&interactionpersist.GiftModel{},
		&interactionpersist.GiftOrderModel{},
		&interactionpersist.RoomStatsModel{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	// Kafka: broadcast (gateway fan-out) + ingest (interaction processing).
	broadcastTopic := a.Config.Kafka.TopicPrefix + "broadcast"
	ingestTopic := a.Config.Kafka.TopicPrefix + "ingest"
	broadcastHandler := conninfra.NewBroadcastHandler(a.Hub)
	go func() {
		if err := a.Consumer.Subscribe(ctx, []string{broadcastTopic}, broadcastHandler); err != nil {
			logger.L().Error("kafka consumer error", zap.Error(err))
		}
	}()
	go func() {
		if err := a.Consumer.Subscribe(ctx, []string{ingestTopic}, a.ingestConsumer); err != nil {
			logger.L().Error("kafka ingest consumer error", zap.Error(err))
		}
	}()

	// Asynq worker + periodic scheduler.
	go func() {
		if err := a.asynqServer.Run(a.asynqMux); err != nil {
			logger.L().Error("asynq server error", zap.Error(err))
		}
	}()
	go func() {
		if err := a.scheduler.Run(); err != nil {
			logger.L().Error("asynq scheduler error", zap.Error(err))
		}
	}()

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
	a.InterHandler.Register(router)

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
	a.scheduler.Shutdown()
	a.asynqServer.Shutdown()
	_ = a.Asynq.Close()
	_ = a.Consumer.Close()
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
