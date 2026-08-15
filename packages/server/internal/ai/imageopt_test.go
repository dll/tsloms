package ai

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// 生成一张 width x height 的 JPEG 测试图（带渐变噪点，避免纯色导致体积过小）
func genJPEG(t *testing.T, w, h, quality int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x*7 + y*3) % 256),
				G: uint8((x*5 + y*11) % 256),
				B: uint8((x*13 + y*17) % 256),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatalf("gen jpeg: %v", err)
	}
	return buf.Bytes()
}

// 大图应被压缩：ok=true 且输出显著变小、长边 ≤ imgMaxEdge
func TestOptimizeImageBytes_LargeImageScaled(t *testing.T) {
	src := genJPEG(t, 4000, 3000, 95)
	out, ok := optimizeImageBytes(src)
	if !ok {
		t.Fatalf("large valid image should be accepted, got ok=false")
	}
	if len(out) >= len(src) {
		t.Fatalf("compressed image not smaller: in=%d out=%d", len(src), len(out))
	}
	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode compressed: %v", err)
	}
	b := img.Bounds()
	if b.Dx() > imgMaxEdge || b.Dy() > imgMaxEdge {
		t.Fatalf("long edge %dx%d exceeds %d", b.Dx(), b.Dy(), imgMaxEdge)
	}
}

// 小图（有效但无需压缩）应放行：ok=true 且字节不变
func TestOptimizeImageBytes_SmallImageUnchanged(t *testing.T) {
	src := genJPEG(t, 200, 150, 80)
	out, ok := optimizeImageBytes(src)
	if !ok {
		t.Fatalf("small valid image should be accepted")
	}
	if !bytes.Equal(out, src) {
		t.Fatalf("small image bytes changed")
	}
}

// 空/无效输入应 ok=false
func TestOptimizeImageBytes_EmptyAndInvalid(t *testing.T) {
	out, ok := optimizeImageBytes(nil)
	if ok || out != nil {
		t.Fatalf("nil input should be invalid (ok=false), got ok=%v", ok)
	}
	garbage := []byte("this is not an image at all")
	out, ok = optimizeImageBytes(garbage)
	if ok {
		t.Fatalf("invalid bytes should be rejected (ok=false)")
	}
	if out != nil {
		t.Fatalf("invalid bytes should return nil output")
	}
	// 超过 4MB 但伪造(非图片)也不应被接受 → 验证修复：不会因放宽上限而放行脏数据
	fakeBig := make([]byte, 5*1024*1024) // 全 0x00，非合法图片
	out, ok = optimizeImageBytes(fakeBig)
	if ok {
		t.Fatalf(">4MB fake(non-image) bytes should be rejected")
	}
	if out != nil {
		t.Fatalf(">4MB fake bytes should return nil")
	}
}
