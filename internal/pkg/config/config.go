package config

import "github.com/spf13/viper"

// Config is the root configuration object.
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	HTTP      HTTPConfig      `mapstructure:"http"`
	WebSocket WebSocketConfig `mapstructure:"websocket"`
	MySQL     MySQLConfig     `mapstructure:"mysql"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Kafka     KafkaConfig     `mapstructure:"kafka"`
	Asynq     AsynqConfig     `mapstructure:"asynq"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
	Log       LogConfig       `mapstructure:"log"`
	AI        AIConfig        `mapstructure:"ai"`
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

// AIConfig holds the AI provider configuration.
type AIConfig struct {
	Provider       string `mapstructure:"provider"`
	BaseURL        string `mapstructure:"base_url"`
	Model          string `mapstructure:"model"`
	APIKey         string `mapstructure:"api_key"`
	MockOnEmptyKey bool   `mapstructure:"mock_on_empty_key"`
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
