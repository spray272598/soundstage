package config

import (
	"time"

	"github.com/spf13/viper"
)

// Config is the root configuration object.
type Config struct {
	App         AppConfig         `mapstructure:"app"`
	HTTP        HTTPConfig        `mapstructure:"http"`
	WebSocket   WebSocketConfig   `mapstructure:"websocket"`
	Merger      MergerConfig      `mapstructure:"merger"`
	MySQL       MySQLConfig       `mapstructure:"mysql"`
	Redis       RedisConfig       `mapstructure:"redis"`
	Kafka       KafkaConfig       `mapstructure:"kafka"`
	Asynq       AsynqConfig       `mapstructure:"asynq"`
	Interaction InteractionConfig `mapstructure:"interaction"`
	MicLink     MicLinkConfig     `mapstructure:"miclink"`
	Hub         HubConfig         `mapstructure:"hub"`
	Metrics     MetricsConfig     `mapstructure:"metrics"`
	Log         LogConfig         `mapstructure:"log"`
	AI          AIConfig          `mapstructure:"ai"`
	Tracing     TracingConfig     `mapstructure:"tracing"`
	Storage     StorageConfig     `mapstructure:"storage"`
}

// AppConfig holds application metadata.
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Env     string `mapstructure:"env"`
	Version string `mapstructure:"version"`
}

// HTTPConfig holds the HTTP server configuration.
type HTTPConfig struct {
	Addr         string `mapstructure:"addr"`
	ReadTimeout  string `mapstructure:"read_timeout"`
	WriteTimeout string `mapstructure:"write_timeout"`
}

// WebSocketConfig holds the WebSocket server configuration.
type WebSocketConfig struct {
	Addr              string `mapstructure:"addr"`
	ReadTimeout       string `mapstructure:"read_timeout"`
	WriteTimeout      string `mapstructure:"write_timeout"`
	MaxMessageSize    int64  `mapstructure:"max_message_size"`
	EnableCompression bool   `mapstructure:"enable_compression"`
	ReadBufferSize    int    `mapstructure:"read_buffer_size"`
	WriteBufferSize   int    `mapstructure:"write_buffer_size"`
}

// ReadTimeoutDuration returns the parsed read timeout duration.
func (c *WebSocketConfig) ReadTimeoutDuration() time.Duration {
	if c.ReadTimeout == "" {
		return 60 * time.Second
	}
	if d, err := time.ParseDuration(c.ReadTimeout); err == nil {
		return d
	}
	return 60 * time.Second
}

// WriteTimeoutDuration returns the parsed write timeout duration.
func (c *WebSocketConfig) WriteTimeoutDuration() time.Duration {
	if c.WriteTimeout == "" {
		return 10 * time.Second
	}
	if d, err := time.ParseDuration(c.WriteTimeout); err == nil {
		return d
	}
	return 10 * time.Second
}

// MergerConfig holds the message merger configuration.
type MergerConfig struct {
	// WorkerCount is the number of merge workers for room messages.
	// Each worker handles a subset of rooms via consistent hashing.
	WorkerCount int `mapstructure:"worker_count"`
	// ChannelSize is the buffer size for the merger input channel.
	ChannelSize int `mapstructure:"channel_size"`
	// MaxBatchSize is the maximum number of messages to batch before flushing.
	MaxBatchSize int `mapstructure:"max_batch_size"`
	// MaxDelay is the maximum time to wait before flushing a partial batch.
	MaxDelay time.Duration `mapstructure:"max_delay"`
}

// MySQLConfig holds the MySQL configuration.
type MySQLConfig struct {
	DSN         string `mapstructure:"dsn"`
	MaxOpen     int    `mapstructure:"max_open"`
	MaxIdle     int    `mapstructure:"max_idle"`
	MaxLifetime string `mapstructure:"max_lifetime"`  // e.g. "5m"
	MaxIdleTime string `mapstructure:"max_idle_time"` // e.g. "1m"
}

// RedisConfig holds the Redis configuration.
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// KafkaConfig holds the Kafka configuration.
type KafkaConfig struct {
	Brokers       []string `mapstructure:"brokers"`
	TopicPrefix   string   `mapstructure:"topic_prefix"`
	ConsumerCount int      `mapstructure:"consumer_count"` // Number of parallel consumers per topic
}

// AsynqConfig holds the asynq configuration.
type AsynqConfig struct {
	RedisAddr string `mapstructure:"redis_addr"`
}

// InteractionConfig holds the interaction-context tunables.
type InteractionConfig struct {
	// DanmakuRateLimit is the max danmaku per user per room within DanmakuRateWindow.
	DanmakuRateLimit int `mapstructure:"danmaku_rate_limit"`
	// DanmakuRateWindow is the sliding/fixed window for danmaku rate limiting.
	DanmakuRateWindow time.Duration `mapstructure:"danmaku_rate_window"`
	// ModerationKeywords is the demo keyword blocklist. Phase 4 swaps in AI moderation.
	ModerationKeywords []string `mapstructure:"moderation_keywords"`
	// GiftIdempotencyTTL is how long a gift idempotency key is remembered.
	GiftIdempotencyTTL time.Duration `mapstructure:"gift_idempotency_ttl"`
}

// MicLinkConfig holds the mic-link / PK tunables.
type MicLinkConfig struct {
	// PKDuration is the default length of a cross-room PK battle.
	PKDuration time.Duration `mapstructure:"pk_duration"`
	// PKCountdownNotice fires this long before the deadline to warn clients.
	PKCountdownNotice time.Duration `mapstructure:"pk_countdown_notice"`
	// LockTTL is the TTL of the distributed lock guarding PK transitions.
	LockTTL time.Duration `mapstructure:"lock_ttl"`
}

// MetricsConfig holds the metrics server configuration.
type MetricsConfig struct {
	Addr string `mapstructure:"addr"`
	Path string `mapstructure:"path"`
}

// LogConfig holds the logging configuration.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// HubConfig holds the connection hub configuration.
type HubConfig struct {
	// Mode: "memory" (single instance) or "redis" (multi-gateway distributed)
	Mode string `mapstructure:"mode"`
	// GatewayID identifies this gateway instance. Auto-generated if empty.
	GatewayID string `mapstructure:"gateway_id"`
}

// AIConfig holds the AI provider configuration for the AI room moderator.
type AIConfig struct {
	Provider       string `mapstructure:"provider"`
	BaseURL        string `mapstructure:"base_url"`
	Model          string `mapstructure:"model"`
	APIKey         string `mapstructure:"api_key"`
	MockOnEmptyKey bool   `mapstructure:"mock_on_empty_key"`

	// EmbeddingModel / EmbeddingBaseURL configure the OpenAI-compatible
	// embedding endpoint used by the RAG knowledge base. When empty they fall
	// back to sensible defaults in the infrastructure layer.
	EmbeddingModel   string `mapstructure:"embedding_model"`
	EmbeddingBaseURL string `mapstructure:"embedding_base_url"`

	// VectorStore configures the vector database backend for RAG.
	// Type: "memory" (default, in-process), "pgvector", "qdrant"
	VectorStore VectorStoreConfig `mapstructure:"vector_store"`

	// ModerationKeywords is a cheap first-line keyword blocklist. The AI
	// moderator still runs an LLM audit, but messages matching these keywords
	// are rejected before any LLM call to save latency and cost.
	ModerationKeywords []string `mapstructure:"moderation_keywords"`

	// RagTopK is how many knowledge chunks to retrieve per query.
	RagTopK int `mapstructure:"rag_top_k"`
	// AgentMaxRounds caps the ReAct tool-calling loop to avoid runaway runs.
	AgentMaxRounds int `mapstructure:"agent_max_rounds"`
	// AgentTimeout caps a single agent run.
	AgentTimeout time.Duration `mapstructure:"agent_timeout"`
}

// VectorStoreConfig holds the vector store configuration.
type VectorStoreConfig struct {
	// Type: "memory" | "pgvector" | "qdrant"
	Type string `mapstructure:"type"`

	// PGVector holds pgvector-specific settings.
	PGVector PGVectorConfig `mapstructure:"pgvector"`

	// Qdrant holds Qdrant-specific settings.
	Qdrant QdrantConfig `mapstructure:"qdrant"`
}

// TracingConfig holds the OpenTelemetry tracing configuration.
type TracingConfig struct {
	// Enabled enables distributed tracing.
	Enabled bool `mapstructure:"enabled"`
	// ServiceName is the name of the service.
	ServiceName string `mapstructure:"service_name"`
	// OTLPEndpoint is the OTLP HTTP endpoint (e.g., "http://jaeger:4318").
	OTLPEndpoint string `mapstructure:"otlp_endpoint"`
	// SamplingRate is the fraction of traces to sample (0.0 to 1.0).
	SamplingRate float64 `mapstructure:"sampling_rate"`
	// Insecure disables TLS for the OTLP connection.
	Insecure bool `mapstructure:"insecure"`
}

// PGVectorConfig holds pgvector connection settings.
type PGVectorConfig struct {
	DSN          string `mapstructure:"dsn"`
	TableName    string `mapstructure:"table_name"`    // default: "ai_knowledge_base"
	VectorDims   int    `mapstructure:"vector_dims"`   // default: 1536
	PoolSize     int    `mapstructure:"pool_size"`     // default: 10
	HNSWEfSearch int    `mapstructure:"hnsw_ef_search"` // default: 64
}

// QdrantConfig holds Qdrant connection settings.
type QdrantConfig struct {
	URL        string `mapstructure:"url"`
	APIKey     string `mapstructure:"api_key"`
	Collection string `mapstructure:"collection"`    // default: "ai_knowledge_base"
	VectorDims int    `mapstructure:"vector_dims"`   // default: 1536
	Timeout    int    `mapstructure:"timeout"`       // default: 30 (seconds)
}

// StorageConfig holds the MinIO/S3 storage configuration.
type StorageConfig struct {
	// MinIO holds MinIO-specific settings.
	MinIO MinIOConfig `mapstructure:"minio"`
}

// MinIOConfig holds MinIO connection settings.
type MinIOConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	UseSSL    bool   `mapstructure:"use_ssl"`
	Region    string `mapstructure:"region"`
}

// Load reads configuration from the given file and environment.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("SOUNDSTAGE")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
