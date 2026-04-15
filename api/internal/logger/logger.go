package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger
var Sugar *zap.SugaredLogger

// Init 初始化日志系统
func Init(debug bool) error {
	var config zap.Config

	if debug {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	// 设置日志输出格式
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	// 创建 logger
	var err error
	Logger, err = config.Build()
	if err != nil {
		return err
	}

	Sugar = Logger.Sugar()
	return nil
}

// Sync 同步日志
func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}

// Info 记录信息日志
func Info(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Info(msg, fields...)
	}
}

// Error 记录错误日志
func Error(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Error(msg, fields...)
	}
}

// Warn 记录警告日志
func Warn(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Warn(msg, fields...)
	}
}

// Debug 记录调试日志
func Debug(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Debug(msg, fields...)
	}
}

// Fatal 记录致命错误并退出
func Fatal(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Fatal(msg, fields...)
	} else {
		// Logger 未初始化时，至少输出到 stderr，避免容器“无日志直接退出”
		_, _ = fmt.Fprintf(os.Stderr, "FATAL: %s fields=%+v\n", msg, fields)
		os.Exit(1)
	}
}
