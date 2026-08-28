package logger

import "go.uber.org/zap"

var global *zap.Logger

// Init configures the global logger.
func Init(level string, format string) error {
	cfg := zap.NewProductionConfig()
	switch format {
	case "console":
		cfg.Encoding = "console"
	case "json":
		cfg.Encoding = "json"
	}
	if err := cfg.Level.UnmarshalText([]byte(level)); err != nil {
		return err
	}
	l, err := cfg.Build()
	if err != nil {
		return err
	}
	global = l
	zap.ReplaceGlobals(global)
	return nil
}

// L returns the global logger.
func L() *zap.Logger {
	if global == nil {
		return zap.NewNop()
	}
	return global
}
