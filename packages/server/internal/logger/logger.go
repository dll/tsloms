// Package logger 提供进程内统一的 zap 日志单例。
//
// 复用 config 的 sync.Once 单例模式：进程内所有 Handler/Service 共享同一 *zap.Logger，
// 避免 mqtt/service 各自 zap.NewProduction() 新建造成日志级别/链路配置不统一（A4）。
// 支持通过 LOG_LEVEL 环境变量调整日志级别（debug/info/warn/error，默认 info）。
// 注意：本包不改变任何业务行为，仅统一日志构造。
package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	once   sync.Once
	global *zap.Logger
)

// ParseLevel 将字符串日志级别映射为 zapcore.Level；未知值回退 info。
func ParseLevel(s string) zapcore.Level {
	switch s {
	case "debug":
		return zapcore.DebugLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// Get 返回进程内统一的 *zap.Logger 单例。
func Get() *zap.Logger {
	once.Do(func() {
		cfg := zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(ParseLevel(os.Getenv("LOG_LEVEL")))
		l, err := cfg.Build()
		if err != nil {
			// 理论上不会失败；兜底使用生产默认避免 nil logger
			l, _ = zap.NewProduction()
		}
		global = l
	})
	return global
}
