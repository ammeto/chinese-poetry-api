// Package logger 基于 zap 提供结构化日志的封装。
package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// L 是全局日志实例
	L    *zap.Logger
	once sync.Once
)

// Init 初始化全局日志实例。
// debug 为 true 时采用开发配置，日志级别为 DEBUG；
// 否则采用生产配置，级别为 INFO。
func Init(debug bool) {
	once.Do(func() {
		var err error
		if debug {
			config := zap.NewDevelopmentConfig()
			config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
			L, err = config.Build()
		} else {
			config := zap.NewProductionConfig()
			config.EncoderConfig.TimeKey = "timestamp"
			config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
			L, err = config.Build()
		}
		if err != nil {
			// 初始化失败则退化为空实现，保证调用方不会 panic
			L = zap.NewNop()
		}
	})
}

// Sync 刷新缓冲中的日志，应在程序退出前调用。
func Sync() {
	if L != nil {
		_ = L.Sync()
	}
}

// Default 返回全局日志实例，尚未初始化时先按环境自动初始化。
func Default() *zap.Logger {
	if L == nil {
		Init(os.Getenv("GIN_MODE") != "release")
	}
	return L
}

// With 创建带附加字段的子 logger。
func With(fields ...zap.Field) *zap.Logger {
	return Default().With(fields...)
}

// Debug 记录 debug 级别日志。
func Debug(msg string, fields ...zap.Field) {
	Default().Debug(msg, fields...)
}

// Info 记录 info 级别日志。
func Info(msg string, fields ...zap.Field) {
	Default().Info(msg, fields...)
}

// Warn 记录 warn 级别日志。
func Warn(msg string, fields ...zap.Field) {
	Default().Warn(msg, fields...)
}

// Error 记录 error 级别日志。
func Error(msg string, fields ...zap.Field) {
	Default().Error(msg, fields...)
}

// Fatal 记录 fatal 级别日志并退出进程。
func Fatal(msg string, fields ...zap.Field) {
	Default().Fatal(msg, fields...)
}
