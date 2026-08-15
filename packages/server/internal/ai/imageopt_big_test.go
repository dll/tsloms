package ai

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 合成一张 >4MB 的真实大 JPEG，验证 imagePathsToDataURLs 现在能容纳（上限已放宽到 20MB）
// 且压缩后 data URL 显著小于原文件、格式为 jpeg
func TestImagePathsToDataURLs_BigImageNowPasses(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "big.jpg")

	// 合成 3000x2250 高噪点 JPEG(质量92)，体积应远超 4MB
	img := image.NewRGBA(image.Rect(0, 0, 3000, 2250))
	for y := 0; y < 2250; y++ {
		for x := 0; x < 3000; x++ {
			img.Set(x, y, color.RGBA{uint8((x*13 + y*5) % 256), uint8((x*7 + y*19) % 256), uint8((x*23 + y*11) % 256), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	fi, _ := os.Stat(p)
	if fi.Size() <= 4*1024*1024 {
		t.Fatalf("precondition failed: 测试图仅 %d bytes, 未超旧 4MB 上限", fi.Size())
	}
	t.Logf("orig file = %d bytes (>4MB)", fi.Size())

	urls := imagePathsToDataURLs([]string{p})
	if len(urls) == 0 {
		t.Fatalf(">4MB 大图应被压缩后送入, 但 data URL 为空(旧 4MB 上限仍生效?)")
	}
	u := urls[0]
	if !strings.HasPrefix(u, "data:image/jpeg;base64,") {
		t.Fatalf("压缩后应为 jpeg data URL, got prefix: %.40s", u)
	}
	const hdr = len("data:image/jpeg;base64,")
	payload := len(u) - hdr // base64 字节数
	origPayload := fi.Size()
	if int64(payload) >= origPayload {
		t.Fatalf("压缩未减小: base64=%d >= orig=%d", payload, origPayload)
	}
	t.Logf("dataURL base64=%d bytes vs orig=%d (%.1f%% reduction)",
		payload, origPayload, (1-float64(payload)/float64(origPayload))*100)
}
