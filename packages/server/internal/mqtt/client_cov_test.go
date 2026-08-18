package mqtt

import "testing"

// 覆盖 client.go 的无 Broker 纯逻辑分支（nil 客户端安全路径）。

func TestMQTTClient_NilSafePaths(t *testing.T) {
	c := NewMQTTClient()
	if c == nil {
		t.Fatal("NewMQTTClient 返回 nil")
	}
	if c.client != nil {
		t.Fatal("新客户端 c.client 应为 nil")
	}
	if c.IsConnected() {
		t.Error("未连接时 IsConnected 应为 false")
	}
	if c.GetClient() != nil {
		t.Error("未连接时 GetClient 应为 nil")
	}
	// Disconnect 在 nil 客户端时应 no-op 不 panic
	c.Disconnect(100)
}
