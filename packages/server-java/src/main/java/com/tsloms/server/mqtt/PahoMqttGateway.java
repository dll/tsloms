// Paho MQTT 网关：连接/订阅/发布，消息回调委托 DeviceAccessService。
// 启动即连接并订阅 {prefix}/+/+/+/U 上行通配；断线由 paho 自动重连。
package com.tsloms.server.mqtt;

import com.tsloms.server.config.MqttProperties;
import java.nio.charset.StandardCharsets;
import org.eclipse.paho.client.mqttv3.IMqttDeliveryToken;
import org.eclipse.paho.client.mqttv3.MqttCallbackExtended;
import org.eclipse.paho.client.mqttv3.MqttClient;
import org.eclipse.paho.client.mqttv3.MqttConnectOptions;
import org.eclipse.paho.client.mqttv3.MqttMessage;
import org.eclipse.paho.client.mqttv3.persist.MemoryPersistence;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.ApplicationArguments;
import org.springframework.boot.ApplicationRunner;
import org.springframework.stereotype.Component;

@Component
public class PahoMqttGateway implements ApplicationRunner, DeviceAccessService.Gateway {

    private static final Logger log = LoggerFactory.getLogger(PahoMqttGateway.class);

    private final MqttProperties props;
    // 懒解析打破与 DeviceAccessService 的循环依赖
    private final org.springframework.beans.factory.ObjectProvider<DeviceAccessService> handlerProvider;
    private MqttClient client;

    public PahoMqttGateway(MqttProperties props,
                           org.springframework.beans.factory.ObjectProvider<DeviceAccessService> handlerProvider) {
        this.props = props;
        this.handlerProvider = handlerProvider;
    }

    @Override
    public void run(ApplicationArguments args) throws Exception {
        if (!props.enabled()) {
            log.info("[TSLOMS] MQTT 接入未启用（MQTT_ENABLED=false）");
            return;
        }
        client = new MqttClient(props.broker(), props.clientId(), new MemoryPersistence());
        client.setCallback(new MqttCallbackExtended() {
            @Override
            public void connectComplete(boolean reconnect, String serverURI) {
                String filter = props.topicPrefix() + "/+/+/+/U";
                try {
                    client.subscribe(filter, 1);
                    log.info("[TSLOMS] MQTT {} 订阅 {}（QoS1）", reconnect ? "重连" : "已连接",
                            filter);
                } catch (Exception e) {
                    log.error("[TSLOMS] MQTT 订阅失败 {}", filter, e);
                }
            }

            @Override
            public void connectionLost(Throwable cause) {
                log.warn("[TSLOMS] MQTT 连接断开，等待自动重连: {}", cause == null ? "" : cause.getMessage());
            }

            @Override
            public void messageArrived(String topic, MqttMessage message) {
                try {
                    handlerProvider.getObject().handleMessage(message.getPayload(), topic);
                } catch (Exception e) {
                    // 单条消息异常不中断订阅循环
                    log.error("[TSLOMS] 消息处理异常 topic={}", topic, e);
                }
            }

            @Override
            public void deliveryComplete(IMqttDeliveryToken token) {
                // 下行完成无需处理
            }
        });

        MqttConnectOptions opts = new MqttConnectOptions();
        opts.setAutomaticReconnect(true);
        opts.setCleanSession(false); // 持久会话：离线期间上行不丢失
        opts.setConnectionTimeout(15);
        opts.setKeepAliveInterval(60);
        if (props.username() != null && !props.username().isBlank()) {
            opts.setUserName(props.username());
            if (props.password() != null) {
                opts.setPassword(props.password().toCharArray());
            }
        }
        client.connect(opts);
        log.info("[TSLOMS] MQTT 连接中 broker={} clientId={}", props.broker(), props.clientId());
    }

    /** QoS1 下行发布。 */
    @Override
    public void publish(String topic, byte[] payload) {
        if (client == null || !client.isConnected()) {
            log.warn("[TSLOMS] MQTT 未连接，丢弃下行 topic={} len={}",
                    topic, payload == null ? 0 : payload.length);
            return;
        }
        try {
            MqttMessage msg = new MqttMessage(payload);
            msg.setQos(1);
            client.publish(topic, msg);
        } catch (Exception e) {
            log.error("[TSLOMS] MQTT 发布失败 topic={} payload(hex前16)={}", topic,
                    hexHead(payload), e);
        }
    }

    private static String hexHead(byte[] b) {
        StringBuilder sb = new StringBuilder();
        for (int i = 0; i < Math.min(16, b.length); i++) {
            sb.append(String.format("%02X", b[i]));
        }
        return sb + (b.length > 16 ? "..." : "");
    }
}
