package model

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// 兼容历史台账中的短数字/十六进制 ID（协议值最终仍按 8 位大写十六进制归一化）。
	legacyHardwareIDPattern = regexp.MustCompile(`^[0-9A-F]{1,8}$`)
	laHardwareIDPattern     = regexp.MustCompile(`^LA[0-9A-Z]{8}$`)
)

// NormalizeHardwareID 规范化台账硬件 ID：统一大写并去除首尾空白。
// 支持协议历史格式 8 位十六进制，以及 IEM 规则生成的 LA+后 8 位编码。
func NormalizeHardwareID(value string) string { return strings.ToUpper(strings.TrimSpace(value)) }

// IsValidHardwareID 判断硬件 ID 是否符合平台支持的规则。
func IsValidHardwareID(value string) bool {
	v := NormalizeHardwareID(value)
	return legacyHardwareIDPattern.MatchString(v) || laHardwareIDPattern.MatchString(v)
}

// HardwareIDAliases 返回协议 8 位 ID 与 LA+8 位台账 ID 的兼容匹配集合。
func HardwareIDAliases(value string) []string {
	v := NormalizeHardwareID(value)
	if len(v) == 10 && strings.HasPrefix(v, "LA") {
		return []string{v, v[2:]}
	}
	if legacyHardwareIDPattern.MatchString(v) {
		padded := v
		if len(padded) < 8 {
			padded = strings.Repeat("0", 8-len(padded)) + padded
		}
		return []string{v, padded, "LA" + padded}
	}
	return []string{v}
}

// LAHardwareIDFromProtocol 将协议 uint32 硬件值映射为 LA+8 位显示编码。
// 注意：协议本身没有携带 LA 前缀，此函数只用于显示/预登记匹配，不改变历史故障 ID。
func LAHardwareIDFromProtocol(value uint32) string { return fmt.Sprintf("LA%08X", value) }
