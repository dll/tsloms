package ai

import (
	"bytes"
	"image"
	"image/jpeg"

	// 注册 jpeg/png/gif 解码器给 image.Decode 使用（匿名导入触发其 init）
	_ "image/gif"
	_ "image/png"

	"golang.org/x/image/draw"
)

// 图片预压缩参数（送 LLM 前调用，节省 token/带宽/内存）：
//   - 长边上限 1024px（主流 VLM 输入窗口，超宽按比例缩小）
//   - 目标体积 ≤ 512KB（压缩后仍超标的降低 JPEG 质量重试）
//   - 统一重编码为 JPEG（gif/png 亦转 JPEG 以收敛体积）
const (
	imgMaxEdge     = 1024
	imgTargetBytes = 512 * 1024
	imgMinQuality  = 60
	imgInitQuality = 82
)

// optimizeImageBytes 对图片字节做"长边缩放 + JPEG 重编码 + 体积收敛"压缩。
// 纯 Go 实现（image + x/image/draw），无 cgo，可在 CGO_ENABLED=0 下交叉编译。
// 返回压缩后的 JPEG 字节；第 2 个返回值 ok=false 表示输入不是可解码的有效图片(应跳过)。
// 对有效但无需压缩的小图返回原字节且 ok=true。
func optimizeImageBytes(src []byte) ([]byte, bool) {
	if len(src) == 0 {
		return src, false
	}
	// 原图本身不算大（≤ 目标体积，且 < 300KB）→ 也需校验确为有效图片后放行；
	// 避免把几字节的垃圾当图片送出去。小图解码开销低，直接校验一次。
	if len(src) < 300*1024 && len(src) <= imgTargetBytes {
		if _, _, err := image.Decode(bytes.NewReader(src)); err != nil {
			return nil, false
		}
		return src, true
	}

	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		// 非标准/损坏图片（如 BMP/TIFF 无内置解码器，或伪造文件）→ 视为无效，跳过
		return nil, false
	}

	// 1) 长边缩放
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return src, true
	}
	scale := 1.0
	switch {
	case w > imgMaxEdge && w >= h:
		scale = float64(imgMaxEdge) / float64(w)
	case h > imgMaxEdge:
		scale = float64(imgMaxEdge) / float64(h)
	}
	dw, dh := int(float64(w)*scale), int(float64(h)*scale)
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}

	// 转 RGBA 统一重编码路径（CatmullRom 双三次高质量缩放）
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)

	// 2) JPEG 重编码，质量收敛到目标体积
	best := src
	for q := imgInitQuality; q >= imgMinQuality; q -= 10 {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: q}); err != nil {
			break
		}
		if buf.Len() <= imgTargetBytes {
			best = buf.Bytes()
			break
		}
		// 记录更小者，即便最后仍超标也可用
		if buf.Len() < len(best) {
			best = buf.Bytes()
		}
	}
	if len(best) < len(src) {
		return best, true
	}
	return src, true
}
