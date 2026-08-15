package handler

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveFeedbackImages(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name  string
		url   string
		want  string   // 期望的最终相对路径片段（"" 表示应返回 nil）
		isNil bool
	}{
		{name: "带nginx前缀+月份子目录", url: "/tsloms/media/202608/1_1786759162670.jpg", want: filepath.Join("202608", "1_1786759162670.jpg")},
		{name: "带静态路由前缀子目录", url: "/media/202607/2_abc.png", want: filepath.Join("202607", "2_abc.png")},
		{name: "根目录文件", url: "/tsloms/media/3_x.jpg", want: "3_x.jpg"},
		{name: "空URL", url: "", isNil: true},
		{name: "空mediaDir", url: "/tsloms/media/a.jpg", isNil: false, /* 空目录用 dir 替换下方验证 */},
		{name: "路径穿越", url: "/tsloms/media/../../etc/passwd", isNil: true},
		{name: "绝对路径", url: "/etc/passwd", isNil: true},
		{name: "含..文件名", url: "/tsloms/media/a../b.jpg", isNil: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			md := dir
			if strings.Contains(c.name, "空mediaDir") {
				md = ""
			}
			got := resolveFeedbackImages(c.url, md)
			if c.isNil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if md == "" {
				// 空 mediaDir 应返回 nil
				if got != nil {
					t.Fatalf("empty mediaDir should return nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected path, got nil")
			}
			// 验证：返回路径 = mediaDir/期望相对片段
			expected := filepath.Join(dir, c.want)
			// Windows 路径分隔符与 URL 可能不同，统一比较 Clean 后结果
			if filepath.Clean(got[0]) != filepath.Clean(expected) {
				t.Fatalf("got %s, want %s", got[0], expected)
			}
			// 防穿越：必须落在 mediaDir 内
			base := filepath.Clean(dir)
			if !strings.HasPrefix(filepath.Clean(got[0]), base+string(filepath.Separator)) {
				t.Fatalf("path escapes mediaDir: %s", got[0])
			}
		})
	}
}

