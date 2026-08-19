package model

// versioned_migrate_util_test.go —— 版本化迁移工具函数守护测试（提升 CD-P0-01 新增代码覆盖率）
// 覆盖 no-MySQL 可测分支：字符串/env 解析、backupTargetDir、fileExists。

import (
	"os"
	"path/filepath"
	"testing"
)

// envValue / splitLines / trimSpace / indexOf 解析
func TestVersionedMigrate_envValue(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "tmp.env")
	content := "DB_HOST=127.0.0.1\n#comment\nDB_PASSWORD=secret\n\nKEY_WITH_EQ=a=b\n"
	if err := os.WriteFile(f, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if v, ok := envValue(f, "DB_HOST"); !ok || v != "127.0.0.1" {
		t.Errorf("envValue DB_HOST=%q ok=%v", v, ok)
	}
	if v, ok := envValue(f, "DB_PASSWORD"); !ok || v != "secret" {
		t.Errorf("envValue DB_PASSWORD=%q", v)
	}
	if v, ok := envValue(f, "KEY_WITH_EQ"); !ok || v != "a=b" {
		t.Errorf("envValue KEY_WITH_EQ=%q", v)
	}
	if _, ok := envValue(f, "NOPE"); ok {
		t.Error("不存在键应 ok=false")
	}
	if _, ok := envValue(filepath.Join(dir, "missing.env"), "X"); ok {
		t.Error("文件不存在应 ok=false")
	}
}

// splitLines / trimSpace / indexOf / fileExists
func TestVersionedMigrate_tools(t *testing.T) {
	if got := splitLines(""); len(got) != 0 {
		t.Errorf("splitLines('') len=%d", len(got))
	}
	if got := splitLines("a"); len(got) != 1 || got[0] != "a" {
		t.Errorf("splitLines('a')=%v", got)
	}
	if got := splitLines("a\r\nb\nc"); len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Errorf("splitLines='%v'", got)
	}
	if got := splitLines("x\r"); len(got) != 1 || got[0] != "x" {
		t.Errorf("splitLines trailing CR=%v", got)
	}
	for in, want := range map[string]string{" a ": "a", "\t b \t": "b", "c": "c", "  ": ""} {
		if got := trimSpace(in); got != want {
			t.Errorf("trimSpace(%q)=%q want %q", in, got, want)
		}
	}
	if indexOf("abcdef", "cd") != 2 {
		t.Error("indexOf cd 应=2")
	}
	if indexOf("abc", "z") != -1 {
		t.Error("indexOf missing 应=-1")
	}
	if fileExists(filepath.Join(t.TempDir(), "..")) != true {
		t.Error("上级目录应存在")
	}
	if fileExists("/nonexistent-path-xyz") {
		t.Error("不存在路径应 false")
	}
}

// backupTargetDir 不落在 immutable releases（HIGH-2 守护）
func TestVersionedMigrate_backupTargetDir_NotReleases(t *testing.T) {
	dir := filepath.ToSlash(backupTargetDir())
	if dir == "" {
		t.Fatal("backupTargetDir 为空")
	}
	if dir == "/opt/tsloms/backups/db" {
		return // 生产固定路径，符合预期
	}
	if containsSlash(dir, "releases") {
		t.Errorf("backupTargetDir 不应落在 releases 目录: %s", dir)
	}
}

func containsSlash(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
