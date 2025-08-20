package logger

import (
	"os"
	"strings"

	"github.com/amerfu/pllm/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Logger *zap.Logger
	Sugar  *zap.SugaredLogger
)

func Initialize(cfg config.LoggingConfig) (*zap.Logger, error) {
	var zapConfig zap.Config

	if cfg.Format == "json" {
		zapConfig = zap.NewProductionConfig()
	} else {
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Set log level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn", "warning":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	case "fatal":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.FatalLevel)
	default:
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	// Set output paths
	if cfg.OutputPath != "" && cfg.OutputPath != "stdout" {
		if cfg.OutputPath == "stderr" {
			zapConfig.OutputPaths = []string{"stderr"}
			zapConfig.ErrorOutputPaths = []string{"stderr"}
		} else {
			zapConfig.OutputPaths = []string{cfg.OutputPath}
			zapConfig.ErrorOutputPaths = []string{cfg.OutputPath}
		}
	}

	// Build logger
	logger, err := zapConfig.Build(zap.AddCallerSkip(1))
	if err != nil {
		return nil, err
	}

	Logger = logger
	Sugar = logger.Sugar()

	return logger, nil
}

func Get() *zap.Logger {
	if Logger == nil {
		logger, _ := zap.NewProduction()
		Logger = logger
		Sugar = logger.Sugar()
	}
	return Logger
}

func GetSugar() *zap.SugaredLogger {
	if Sugar == nil {
		Get()
	}
	return Sugar
}

func Debug(msg string, fields ...zap.Field) {
	Get().Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	Get().Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	Get().Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	Get().Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	Get().Fatal(msg, fields...)
}

func With(fields ...zap.Field) *zap.Logger {
	return Get().With(fields...)
}

func NewRequestLogger(requestID string) *zap.Logger {
	return Get().With(zap.String("request_id", requestID))
}

func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}

type GormLogger struct {
	ZapLogger *zap.Logger
}

func NewGormLogger(zapLogger *zap.Logger) *GormLogger {
	return &GormLogger{
		ZapLogger: zapLogger,
	}
}

func (l *GormLogger) Printf(format string, args ...interface{}) {
	l.ZapLogger.Sugar().Debugf(format, args...)
}

func GetLogLevel() zapcore.Level {
	if Logger == nil {
		return zapcore.InfoLevel
	}
	return Logger.Level()
}

func SetLogLevel(level string) {
	var zapLevel zapcore.Level
	switch strings.ToLower(level) {
	case "debug":
		zapLevel = zap.DebugLevel
	case "info":
		zapLevel = zap.InfoLevel
	case "warn", "warning":
		zapLevel = zap.WarnLevel
	case "error":
		zapLevel = zap.ErrorLevel
	case "fatal":
		zapLevel = zap.FatalLevel
	default:
		zapLevel = zap.InfoLevel
	}

	if Logger != nil {
		Logger.Core().Enabled(zapLevel)
	}
}

func IsDebugEnabled() bool {
	return GetLogLevel() <= zapcore.DebugLevel
}

func IsInfoEnabled() bool {
	return GetLogLevel() <= zapcore.InfoLevel
}

func IsWarnEnabled() bool {
	return GetLogLevel() <= zapcore.WarnLevel
}

func IsErrorEnabled() bool {
	return GetLogLevel() <= zapcore.ErrorLevel
}

func init() {
	// Initialize with default logger if not already initialized
	if Logger == nil {
		var logger *zap.Logger
		if os.Getenv("ENV") == "production" {
			logger, _ = zap.NewProduction()
		} else {
			logger, _ = zap.NewDevelopment()
		}
		Logger = logger
		Sugar = logger.Sugar()
	}
}
