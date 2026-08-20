package model

import "testing"

func TestHardwareIDRules(t *testing.T) {
	if !IsValidHardwareID("la82533848") {
		t.Fatal("LA+8位编码应通过")
	}
	if !IsValidHardwareID("1114006C") {
		t.Fatal("历史8位十六进制编码应通过")
	}
	if IsValidHardwareID("LA123") || IsValidHardwareID("IEM12345678") {
		t.Fatal("非法编码不应通过")
	}
	aliases := HardwareIDAliases("LA82533848")
	if len(aliases) != 2 || aliases[1] != "82533848" {
		t.Fatalf("别名映射错误: %#v", aliases)
	}
	if LAHardwareIDFromProtocol(0x82533848) != "LA82533848" {
		t.Fatal("协议映射错误")
	}
}
