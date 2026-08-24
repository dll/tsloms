// 硬件 ID 别名/规范化单测（对齐 Go 版 hardware_id_test 口径）。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;

import com.tsloms.server.mqtt.protocol.HardwareIds;
import org.junit.jupiter.api.Test;

class HardwareIdsTest {

    @Test
    void 协议值映射8位大写十六进制() {
        assertThat(HardwareIds.ledUuid(0x1234ABCDL)).isEqualTo("1234ABCD");
        assertThat(HardwareIds.ledUuid(0x00000001L)).isEqualTo("00000001");
        assertThat(HardwareIds.laFromProtocol(0x1234ABCDL)).isEqualTo("LA1234ABCD");
    }

    @Test
    void 别名集合_历史短码补零与LA互配() {
        // 8 位原样
        assertThat(HardwareIds.aliases("1234ABCD")).containsExactly(
                "1234ABCD", "LA1234ABCD");
        // 不足 8 位补零
        assertThat(HardwareIds.aliases("1A")).containsExactly(
                "1A", "0000001A", "LA0000001A");
        // LA 形式 → 自身 + 去前缀
        assertThat(HardwareIds.aliases("LA1234ABCD")).containsExactly(
                "LA1234ABCD", "1234ABCD");
        // 非法格式 → 仅自身
        assertThat(HardwareIds.aliases("hello-world")).containsExactly("HELLO-WORLD");
    }

    @Test
    void 规范化_大小写与空白() {
        assertThat(HardwareIds.normalize(" la1234abcd ")).isEqualTo("LA1234ABCD");
    }
}
