package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeProfileAndTotal(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, "coverage.out")
	content := strings.Join([]string{
		"mode: set",
		"github.com/tsloms/server/cmd/coveragecheck/main.go:21.1,21.2 1 0",
		"github.com/tsloms/server/cmd/coveragecheck/main.go:21.1,21.2 1 1",
		"github.com/tsloms/server/cmd/coveragecheck/main.go:22.1,22.2 1 1",
	}, "\n") + "\n"
	if err := os.WriteFile(profile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	normalized, err := normalizeProfile(profile)
	if err != nil {
		t.Fatalf("normalizeProfile: %v", err)
	}
	defer os.Remove(normalized)
	b, err := os.ReadFile(normalized)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(b), "main.go:21.1,21.2") != 1 || !strings.Contains(string(b), "main.go:21.1,21.2 1 1") {
		t.Fatalf("重复块未正确合并:\n%s", b)
	}

	total, err := totalFromFunc(profile)
	if err != nil {
		t.Fatalf("totalFromFunc: %v", err)
	}
	if total != 100 {
		t.Fatalf("total=%.2f, want 100", total)
	}
}

func TestNormalizeProfileErrors(t *testing.T) {
	if _, err := normalizeProfile(filepath.Join(t.TempDir(), "missing.out")); err == nil {
		t.Error("不存在的 profile 应报错")
	}
	bad := filepath.Join(t.TempDir(), "bad.out")
	if err := os.WriteFile(bad, []byte("mode: set\ngithub.com/tsloms/server/cmd/coveragecheck/main.go:21.1,21.2 1 bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := normalizeProfile(bad); err == nil {
		t.Error("非法执行次数应报错")
	}
}
