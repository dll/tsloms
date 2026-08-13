package mqtt

import (
	"fmt"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

// MessageHandler MQTT 消息处理函数类型
type MessageHandler func(client MQTT.Client, msg MQTT.Message)

// MQTTClient MQTT 客户端管理
// 封装 paho.mqtt.golang 客户端，提供连接、订阅、发布功能
type MQTTClient struct {
	client MQTT.Client
	logger *zap.Logger
}

// NewMQTTClient 创建 MQTT 客户端实例
func NewMQTTClient() *MQTTClient {
	logger, _ := zap.NewProduction()
	return &MQTTClient{
		logger: logger,
	}
}

// Connect 连接 MQTT Broker
// 配置断线自动重连，连接超时 30 秒
func (c *MQTTClient) Connect(broker, username, password, clientID string) error {
	opts := MQTT.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetAutoReconnect(true) // 启用断线自动重连
	opts.SetMaxReconnectInterval(10 * time.Second)
	opts.SetConnectTimeout(30 * time.Second)
	opts.SetCleanSession(true)

	if username != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}

	// 连接成功回调
	opts.OnConnect = func(client MQTT.Client) {
		c.logger.Info("MQTT 连接成功",
			zap.String("broker", broker),
			zap.String("clientID", clientID),
		)
	}

	// 连接断开回调
	opts.OnConnectionLost = func(client MQTT.Client, err error) {
		c.logger.Error("MQTT 连接断开，将自动重连",
			zap.String("broker", broker),
			zap.Error(err),
		)
	}

	// 重连中回调
	opts.OnReconnecting = func(MQTT.Client, *MQTT.ClientOptions) {
		c.logger.Info("MQTT 正在重连...",
			zap.String("broker", broker),
		)
	}

	c.client = MQTT.NewClient(opts)

	token := c.client.Connect()
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT 连接失败: %w", token.Error())
	}

	return nil
}

// Subscribe 订阅指定 Topic
// qos=1 保证消息至少一次送达
func (c *MQTTClient) Subscribe(topic string, handler MQTT.MessageHandler) error {
	token := c.client.Subscribe(topic, 1, handler)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("订阅 Topic 失败 [%s]: %w", topic, token.Error())
	}
	c.logger.Info("MQTT 订阅成功", zap.String("topic", topic))
	return nil
}

// Publish 发布消息到指定 Topic
// qos=1 保证消息至少一次送达
func (c *MQTTClient) Publish(topic string, qos byte, payload []byte) error {
	token := c.client.Publish(topic, qos, false, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("发布消息失败 [%s]: %w", topic, token.Error())
	}
	return nil
}

// IsConnected 检查连接状态
func (c *MQTTClient) IsConnected() bool {
	if c.client == nil {
		return false
	}
	return c.client.IsConnected()
}

// Disconnect 断开 MQTT 连接
// waitMs 为等待断开完成的超时时间（毫秒）
func (c *MQTTClient) Disconnect(waitMs uint) {
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(waitMs)
		c.logger.Info("MQTT 连接已断开")
	}
}

// GetClient 获取底层 paho MQTT 客户端实例
func (c *MQTTClient) GetClient() MQTT.Client {
	return c.client
}
