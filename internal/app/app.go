package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	aiapplication "github.com/spray272598/soundstage/internal/ai/application"
	aiagent "github.com/spray272598/soundstage/internal/ai/infrastructure/agent"
	aillm "github.com/spray272598/soundstage/internal/ai/infrastructure/llm"
	airag "github.com/spray272598/soundstage/internal/ai/infrastructure/rag"
	aitransport "github.com/spray272598/soundstage/internal/ai/transport"
	"github.com/spray272598/soundstage/internal/connection/application"
	connDomain "github.com/spray272598/soundstage/internal/connection/domain"
	conninfra "github.com/spray272598/soundstage/internal/connection/infrastructure"
	conntransport "github.com/spray272598/soundstage/internal/connection/transport"
	"github.com/spray272598/soundstage/internal/infrastructure/kafka"
	interactionapp "github.com/spray272598/soundstage/internal/interaction/application"
	asynqworker "github.com/spray272598/soundstage/internal/interaction/infrastructure/asynqworker"
	interactioncache "github.com/spray272598/soundstage/internal/interaction/infrastructure/cache"
	interactionmsg "github.com/spray272598/soundstage/internal/interaction/infrastructure/messaging"
	interactionpersist "github.com/spray272598/soundstage/internal/interaction/infrastructure/persistence"
	interactiontask "github.com/spray272598/soundstage/internal/interaction/task"
	interactiontransport "github.com/spray272598/soundstage/internal/interaction/transport"
	miclinkapp "github.com/spray272598/soundstage/internal/miclink/application"
	miclinkworker "github.com/spray272598/soundstage/internal/miclink/infrastructure/asynqworker"
	miclinkcache "github.com/spray272598/soundstage/internal/miclink/infrastructure/cache"
	miclinkmsg "github.com/spray272598/soundstage/internal/miclink/infrastructure/messaging"
	miclinkpersist "github.com/spray272598/soundstage/internal/miclink/infrastructure/persistence"
	miclinktransport "github.com/spray272598/soundstage/internal/miclink/transport"
	"github.com/spray272598/soundstage/internal/pkg/config"
	"github.com/spray272598/soundstage/internal/pkg/id"
	pkgkafka "github.com/spray272598/soundstage/internal/pkg/kafka"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
	pkgredis "github.com/spray272598/soundstage/internal/pkg/redis"
	roomapp "github.com/spray272598/soundstage/internal/room/application"
	roompersist "github.com/spray272598/soundstage/internal/room/infrastructure/persistence"
	roomtransport "github.com/spray272598/soundstage/internal/room/transport"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Application holds the runtime dependencies of the modular monolith.
type Application struct {
	Config       *config.Config
	DB           *gorm.DB
	Redis        *pkgredis.Client
	Hub          connDomain.Hub
	Producer     pkgkafka.Producer
	Consumer     pkgkafka.Consumer
	Asynq        *asynq.Client
	RoomHandler  *roomtransport.RoomHandler
	WSHandler    *conntransport.WSHandler
	InterHandler *interactiontransport.InteractionHandler
	MicHandler   *miclinktransport.MiclinkHandler
	AIHandler    *aitransport.Handler

	ingestConsumer  *interactionapp.IngestConsumer
	miclinkConsumer *kafka.Consumer
	miclinkIngest   *miclinkapp.MiclinkIngestConsumer
	asynqMux        *asynq.ServeMux
	asynqServer     *asynq.Server
	scheduler       *asynq.Scheduler
	batchPersister  *asynqworker.BatchPersister
}

// New builds and wires the application.
func New(cfg *config.Config) (*Application, error) {
	if err := logger.Init(cfg.Log.Level, cfg.Log.Format); err != nil {
		return nil, err
	}

	db, err := newDB(&cfg.MySQL)
	if err != nil {
		return nil, fmt.Errorf("mysql: %w", err)
	}

	rdb := pkgredis.New(cfg.Redis.Addr, cfg.Redis.DB, cfg.Redis.PoolSize)
	if err := rdb.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}

	// --- room context ---
	roomRepo := roompersist.NewGormRoomRepository(db)
	roomSvc := roomapp.NewRoomService(roomRepo)
	roomHandler := roomtransport.NewRoomHandler(roomSvc)

	// --- connection context ---
	var hub connDomain.Hub
	if cfg.Hub.Mode == "redis" {
		gatewayID := cfg.Hub.GatewayID
		if gatewayID == "" {
			gatewayID = fmt.Sprintf("gw-%s", id.New())
		}
		redisHub := conninfra.NewRedisHub(rdb.RDB(), gatewayID)
		if err := redisHub.Start(); err != nil {
			return nil, fmt.Errorf("redis hub start: %w", err)
		}
		hub = redisHub
	} else {
		hub = conninfra.NewHub()
	}
	producer := kafka.NewProducer(cfg.Kafka.Brokers)
	broadcastTopic := cfg.Kafka.TopicPrefix + "broadcast"
	ingestTopic := cfg.Kafka.TopicPrefix + "ingest"
	connSvc := application.NewConnectionService(hub, producer, ingestTopic, cfg.WebSocket)
	wsHandler := conntransport.NewWSHandler(connSvc, cfg.WebSocket)
	consumer := kafka.NewConsumer(cfg.Kafka.Brokers, "soundstage-gateway").
		WithConsumerCount(cfg.Kafka.ConsumerCount)

	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.Asynq.RedisAddr})

	// --- miclink context (co-host + cross-room PK) ---
	// Built before interaction so the AI moderator can read room/PK state.
	micRepo := miclinkpersist.NewGormMicLinkRepository(db)
	pkRepo := miclinkpersist.NewGormPKSessionRepository(db)
	micBroadcaster := miclinkmsg.NewKafkaBroadcaster(producer, broadcastTopic)
	signalingRelay := miclinkmsg.NewKafkaSignalingRelay(producer, broadcastTopic)
	micLocker := miclinkcache.NewRedisLocker(rdb, cfg.MicLink.LockTTL)
	micEnqueuer := miclinkmsg.NewAsynqTaskEnqueuer(asynqClient)

	micSvc := miclinkapp.NewMicLinkService(micRepo, signalingRelay, micBroadcaster)
	pkSvc := miclinkapp.NewPKService(
		pkRepo,
		micBroadcaster,
		micEnqueuer,
		micLocker,
		miclinkapp.PKServiceConfig{
			Duration:        cfg.MicLink.PKDuration,
			CountdownNotice: cfg.MicLink.PKCountdownNotice,
		},
	)
	micHandler := miclinktransport.NewMiclinkHandler(micSvc, pkSvc)
	miclinkIngest := miclinkapp.NewMiclinkIngestConsumer(micSvc, pkSvc)
	miclinkConsumer := kafka.NewConsumer(cfg.Kafka.Brokers, "soundstage-miclink").
		WithConsumerCount(cfg.Kafka.ConsumerCount)

	// --- shared interaction infra (also used by the AI moderator) ---
	broadcaster := interactionmsg.NewKafkaBroadcaster(producer, broadcastTopic)
	rankStore := interactioncache.NewRedisRankStore(rdb)
	likeCounter := interactioncache.NewRedisLikeCounter(rdb)
	rateLimiter := interactioncache.NewRedisRateLimiter(rdb)
	muter := interactioncache.NewRedisMuter(rdb)

	// --- AI room-moderator context (Phase 4) ---
	// LLM gateway falls back to an offline mock when no API key is configured.
	realLLM := cfg.AI.APIKey != ""
	aiLLM := aillm.NewFromConfig(cfg.AI.APIKey, cfg.AI.BaseURL, cfg.AI.Model, cfg.AI.MockOnEmptyKey)
	aiEmbedder := aillm.NewEmbedderFromConfig(cfg.AI.APIKey, cfg.AI.EmbeddingBaseURL, cfg.AI.EmbeddingModel)
	aiKB := airag.NewService(aiEmbedder)
	if err := airag.SeedDefaultKnowledge(context.Background(), aiKB); err != nil {
		logger.L().Warn("seed default knowledge failed", zap.Error(err))
	}
	aiReg := aiagent.NewMapRegistry()
	aiDeps := aiagent.Dependencies{
		Status:    &roomStatusAdapter{rooms: roomSvc, hub: hub, micSvc: micSvc, pkSvc: pkSvc},
		Leader:    &leaderboardAdapter{rank: rankStore},
		Muted:     &roomModeratorAdapter{muter: muter},
		Broadcast: aiBroadcasterAdapter{inner: broadcaster},
		KB:        aiKB,
	}
	aiagent.RegisterBuiltinTools(aiReg, aiDeps)
	aiLoop := aiagent.NewLoop(aiLLM, aiReg, aiagent.Config{MaxRounds: cfg.AI.AgentMaxRounds, Timeout: cfg.AI.AgentTimeout})
	aiSvc := aiapplication.NewService(aiLLM, aiKB, aiLoop, cfg.AI.ModerationKeywords, realLLM)
	aiHandler := aitransport.NewHandler(aiSvc, modeOf(realLLM), modelOr(cfg.AI.Model))

	// --- interaction context ---
	// The AI service is the moderator: it runs an LLM audit (with a keyword
	// fast-path) and replaces the old keyword-only moderator transparently.
	giftRepo := interactionpersist.NewGormGiftRepository(db)
	orderRepo := interactionpersist.NewGormGiftOrderRepository(db)
	danmakuRepo := interactionpersist.NewGormDanmakuRepository(db)
	roomStatsRepo := interactionpersist.NewGormRoomStatsRepository(db)

	enqueuer := interactionmsg.NewAsynqTaskEnqueuer(asynqClient)

	// Batch persister for high-throughput async persistence
	batchPersister := asynqworker.NewBatchPersister(asynqClient, asynqworker.DefaultBatchConfig())
	enqueuer.WithBatchPersister(batchPersister)

	interSvc := interactionapp.NewInterService(
		giftRepo, orderRepo, danmakuRepo,
		aiSvc, rateLimiter, muter, rankStore, likeCounter,
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
	miclinkworker.Register(mux, miclinkworker.Deps{
		PKs:         pkRepo,
		Broadcaster: micBroadcaster,
		Locker:      micLocker,
		RDB:         rdb,
	})

	scheduler := asynqworker.NewScheduler(cfg.Asynq.RedisAddr)
	if _, err := scheduler.Register("@every 30s", asynq.NewTask(interactiontask.TypeFlushLikes, nil)); err != nil {
		return nil, fmt.Errorf("scheduler register: %w", err)
	}

	return &Application{
		Config:          cfg,
		DB:              db,
		Redis:           rdb,
		Hub:             hub,
		Producer:        producer,
		Consumer:        consumer,
		Asynq:           asynqClient,
		RoomHandler:     roomHandler,
		WSHandler:       wsHandler,
		InterHandler:    interHandler,
		MicHandler:      micHandler,
		AIHandler:       aiHandler,
		ingestConsumer:  ingestConsumer,
		miclinkConsumer: miclinkConsumer,
		miclinkIngest:   miclinkIngest,
		asynqMux:        mux,
		asynqServer:     asynqServer,
		scheduler:       scheduler,
		batchPersister:  batchPersister,
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
		&miclinkpersist.MicLinkModel{},
		&miclinkpersist.PKSessionModel{},
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
	go func() {
		if err := a.miclinkConsumer.Subscribe(ctx, []string{ingestTopic}, a.miclinkIngest); err != nil {
			logger.L().Error("kafka miclink ingest consumer error", zap.Error(err))
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
	a.MicHandler.Register(router)
	a.AIHandler.Register(router)

	httpServer := &http.Server{
		Addr:    a.Config.HTTP.Addr,
		Handler: router,
		// ReadTimeout guards against slowloris on request headers; WriteTimeout
		// is deliberately generous (>= ai.agent_timeout) so legitimate SSE
		// streams that span a multi-round agent run are not cut off mid-stream.
		ReadTimeout:  mustDuration(a.Config.HTTP.ReadTimeout, 10*time.Second),
		WriteTimeout: mustDuration(a.Config.HTTP.WriteTimeout, 30*time.Second),
		IdleTimeout:  mustDuration(a.Config.HTTP.WriteTimeout, 120*time.Second),
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
	if a.batchPersister != nil {
		a.batchPersister.Stop()
	}
	_ = a.Asynq.Close()
	_ = a.Consumer.Close()
	_ = a.miclinkConsumer.Close()
	_ = a.Producer.Close()
	_ = a.Hub.Close()
	sqlDB, err := a.DB.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
	return a.Redis.Close()
}

// modeOf returns the AI mode label for the health endpoint.
func modeOf(realLLM bool) string {
	if realLLM {
		return "llm"
	}
	return "mock"
}

// modelOr returns the configured model or a mock label.
func modelOr(model string) string {
	if model == "" {
		return "mock"
	}
	return model
}

// mustDuration parses a duration string, falling back to def on empty/invalid
// input so a malformed config can never crash startup.
func mustDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return def
}

func newDB(cfg *config.MySQLConfig) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	if d := mustDuration(cfg.MaxLifetime, 5*time.Minute); d > 0 {
		sqlDB.SetConnMaxLifetime(d)
	}
	if d := mustDuration(cfg.MaxIdleTime, 1*time.Minute); d > 0 {
		sqlDB.SetConnMaxIdleTime(d)
	}
	return db, nil
}
