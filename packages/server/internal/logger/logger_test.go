package logger

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]zapcore.Level{
		"debug":   zapcore.DebugLevel,
		"warn":    zapcore.WarnLevel,
		"warning": zapcore.WarnLevel,
		"error":   zapcore.ErrorLevel,
		"info":    zapcore.InfoLevel,
		"":        zapcore.InfoLevel,
		"nonsense": zapcore.InfoLevel,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %v, 期望 %v", in, got, want)
		}
	}
}

func TestGetReturnsNonNil(t *testing.T) {
	// Get 应保证返回非 nil 单例，且多次调用复用同一实例
	a := Get()
	b := Get()
	if a == nil {
		t.Fatal("Get() 返回 nil logger")
	}
	if a != b {
		t.Error("Get() 未复用单例")
	}
}
