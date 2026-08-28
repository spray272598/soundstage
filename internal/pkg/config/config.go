package config

import (
	"time"

	"github.com/spf13/viper"
)

// Config is the root configuration object.
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	HTTP      HTTPConfig      `mapstructure:"http"`
	WebSocket WebSocketConfig `mapstructure:"websocket"`
	MySQL     MySQLConfig     `mapstructure:"mysql"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Kafka      KafkaConfig      `mapstructure:"kafka"`
	Asynq      AsynqConfig      `mapstructure:"asynq"`
	Interaction InteractionConfig `mapstructure:"interaction"`
	MicLink     MicLinkConfig     `mapstructure:"miclink"`
	Metrics     MetricsConfig    `mapstructure:"metrics"`
	Log        LogConfig        `mapstructure:"log"`
	AI         AIConfig         `mapstructure:"ai"`
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
	Addr           string `mapstructure:"addr"`
	ReadTimeout    string `mapstructure:"read_timeout"`
	WriteTimeout   string `mapstructure:"write_timeout"`
	MaxMessageSize int64  `mapstructure:"max_message_size"`
}

// MySQLConfig holds the MySQL configuration.
type MySQLConfig struct {
	DSN     string `mapstructure:"dsn"`
	MaxOpen int    `mapstructure:"max_open"`
	MaxIdle int    `mapstructure:"max_idle"`
}

// RedisConfig holds the Redis configuration.
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// KafkaConfig holds the Kafka configuration.
type KafkaConfig struct {
	Brokers     []string `mapstructure:"brokers"`
	TopicPrefix string   `mapstructure:"topic_prefix"`
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
