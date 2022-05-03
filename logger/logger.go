package logger

import (
	"log"

	"github.com/MikelAlejoBR/sources-database-populator/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is the zap logger we will be using in the program.
var Logger *zap.SugaredLogger

// InitializeLogger builds and sets the logger ready to be used.
func InitializeLogger() {
	// Set up the logger so that we only obtain a "level", "info", "timestamp", "msg" and any additional fields.
	var atomicLogLevel zap.AtomicLevel
	switch config.LogLevel {
	case "error":
		atomicLogLevel = zap.NewAtomicLevelAt(zap.ErrorLevel)
	case "warn":
		atomicLogLevel = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "debug":
		atomicLogLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		atomicLogLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	default:
		atomicLogLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	cfg := zap.Config{
		Level:    atomicLogLevel,
		Encoding: "json",
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:  "msg",
			LevelKey:    "level",
			TimeKey:     "ts",
			EncodeLevel: zapcore.LowercaseLevelEncoder,
			EncodeTime:  zapcore.ISO8601TimeEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	// Build the configuration to ensure that we did it correctly.
	zapper, err := cfg.Build()
	if err != nil {
		log.Fatalf(`could not build logger: %s`, err)
	}

	// Get the default and recommended logger.
	Logger = zapper.Sugar()
}

// FlushLoggingBuffer flushes any buffered logs before exiting the program.
func FlushLoggingBuffer() {
	Logger.Sync()
}
