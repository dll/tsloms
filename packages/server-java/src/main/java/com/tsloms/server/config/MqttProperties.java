// MQTT 配置属性：环境变量与 Go 版 config 同名同默认。
package com.tsloms.server.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "mqtt")
public record MqttProperties(String broker, String username, String password,
                             String clientId, String topicPrefix, boolean enabled) {

    public MqttProperties {
        if (broker == null || broker.isBlank()) {
            broker = "tcp://127.0.0.1:1883";
        }
        if (clientId == null || clientId.isBlank()) {
            clientId = "tsloms-server";
        }
        if (topicPrefix == null || topicPrefix.isBlank()) {
            topicPrefix = "trafficLight";
        }
        // enabled 默认 true；测试环境通过 mqtt.enabled=false 关闭
    }
}
