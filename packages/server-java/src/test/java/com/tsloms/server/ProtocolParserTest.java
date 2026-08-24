// 协议解析/构建测试：移植 Go 版 parser_test.go 关键用例（小端+校验和逐字节对齐）。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.tsloms.server.mqtt.protocol.Cmd;
import com.tsloms.server.mqtt.protocol.FaultCodes;
import com.tsloms.server.mqtt.protocol.ProtocolParser;
import org.junit.jupiter.api.Test;

class ProtocolParserTest {

    /** 构建合法帧（自动校验和）。 */
    private byte[] build(int cmd, long swVer, int seq, long userVal, byte[] data) {
        return ProtocolParser.buildCmdFrame(cmd, swVer, seq, userVal, data);
    }

    /** 组 EVENT_PAK 数据部分。 */
    private byte[] eventPak(byte[][] records) {
        int n = records.length;
        java.nio.ByteBuffer buf = java.nio.ByteBuffer
                .allocate(4 + n * Cmd.EVENT_RECORD_LEN)
                .order(java.nio.ByteOrder.LITTLE_ENDIAN);
        buf.putShort((short) n);
        buf.putShort((short) (n * Cmd.EVENT_RECORD_LEN));
        for (byte[] r : records) {
            buf.put(r);
        }
        return buf.array();
    }

    private byte[] record(long ledHw, long subHw, long swVer, long confVer,
                          byte state, byte errCode, int cr, int cy, int cg) {
        java.nio.ByteBuffer buf = java.nio.ByteBuffer.allocate(Cmd.EVENT_RECORD_LEN)
                .order(java.nio.ByteOrder.LITTLE_ENDIAN);
        buf.putInt((int) ledHw).putInt((int) subHw).putInt((int) swVer).putInt((int) confVer);
        buf.put(state).put(errCode);
        buf.putShort((short) cr).putShort((short) cy).putShort((short) cg);
        return buf.array();
    }

    @Test
    void 签到帧往返_字段与校验和() {
        byte[] data = eventPak(new byte[][]{record(1001, 1, 0x01020304, 0x26081401,
                FaultCodes.STATE_R, FaultCodes.ERR_OK, 800, 700, 600)});
        byte[] frame = build(Cmd.CHECKIN, 0x01020304L, 1, 0, data);

        Cmd.Frame f = ProtocolParser.parseCmdFrame(frame);
        assertThat(f.token).isEqualTo(Cmd.TOKEN);
        assertThat(f.cmd).isEqualTo(Cmd.CHECKIN);
        assertThat(f.ver).isEqualTo(Cmd.VER);
        assertThat(f.swVer).isEqualTo(0x01020304L);
        assertThat(f.cmdSeq).isEqualTo(1);
        assertThat(f.datLen).isEqualTo(data.length);

        // 校验和自检：整包累加低 8 位 == 0xFF
        int sum = 0;
        for (byte b : frame) {
            sum += b & 0xFF;
        }
        assertThat(sum & 0xFF).isEqualTo(0xFF);

        // 事件包解析回读（小端）
        Cmd.EventPak pak = ProtocolParser.parseEventPak(f.data);
        assertThat(pak.recordNum).isEqualTo(1);
        Cmd.EventRecord r = pak.records.get(0);
        assertThat(r.ledHwId).isEqualTo(1001);
        assertThat(r.subHwId).isEqualTo(1);
        assertThat(r.swVer).isEqualTo(0x01020304L);
        assertThat(r.confVer).isEqualTo(0x26081401L);
        assertThat(r.ledState).isEqualTo(FaultCodes.STATE_R);
        assertThat(r.errCode).isEqualTo(FaultCodes.ERR_OK);
        assertThat(r.currentR).isEqualTo(800);
        assertThat(r.currentY).isEqualTo(700);
        assertThat(r.currentG).isEqualTo(600);
    }

    @Test
    void 校验和篡改_拒绝() {
        byte[] data = eventPak(new byte[][]{record(1, 1, 1, 1,
                FaultCodes.STATE_G, FaultCodes.ERR_R_OFF, 100, 100, 100)});
        byte[] frame = build(Cmd.ALARM, 1, 2, 0, data);
        frame[frame.length - 1] ^= (byte) 0xFF; // 破坏末字节
        assertThatThrownBy(() -> ProtocolParser.parseCmdFrame(frame))
                .isInstanceOf(ProtocolParser.ProtocolException.class);
    }

    @Test
    void 魔术字破坏_拒绝() {
        byte[] frame = build(Cmd.CHECKIN, 1, 1, 0, new byte[0]);
        frame[0] = 0x54;
        assertThatThrownBy(() -> ProtocolParser.parseCmdFrame(frame))
                .isInstanceOf(ProtocolParser.ProtocolException.class)
                .hasMessageContaining("魔术字");
    }

    @Test
    void 过短帧_拒绝() {
        assertThatThrownBy(() -> ProtocolParser.parseCmdFrame(new byte[]{0x55, 0x01}))
                .isInstanceOf(ProtocolParser.ProtocolException.class)
                .hasMessageContaining("长度不足");
    }

    @Test
    void datLen不匹配_拒绝() {
        byte[] frame = build(Cmd.ALARM, 1, 1, 0, new byte[]{1, 2, 3});
        // 篡改 datLen=0xFFFF 并重算校验和以让长度检查先触发
        frame[10] = (byte) 0xFF;
        frame[11] = (byte) 0xFF;
        int sum = 0;
        for (int i = 0; i < frame.length; i++) {
            if (i != 3) {
                sum += frame[i] & 0xFF;
            }
        }
        frame[3] = (byte) ((0xFF - (sum & 0xFF)) & 0xFF);
        assertThatThrownBy(() -> ProtocolParser.parseCmdFrame(frame))
                .hasMessageContaining("数据长度不匹配");
    }

    @Test
    void 时间同步回应帧_结构() {
        long epoch = 1787055200L;
        byte[] ack = ProtocolParser.buildTimeSyncAck(Cmd.CHECKIN, 0x01020304L, 7, epoch);
        Cmd.Frame f = ProtocolParser.parseCmdFrame(ack); // 回应帧自身也是合法帧
        assertThat(Cmd.isAck(f.cmd)).isTrue();
        assertThat(f.userVal).isEqualTo(epoch);
        assertThat(f.datLen).isZero();
        assertThat(f.swVer).isEqualTo(0x01020304L);
        assertThat(f.cmdSeq).isEqualTo(7);
    }

    @Test
    void 故障码映射_类型与等级() {
        assertThat(FaultCodes.faultTypeFromErrCode(FaultCodes.ERR_OK)).isEqualTo("unknown");
        assertThat(FaultCodes.faultTypeFromErrCode(FaultCodes.ERR_R_OFF)).isEqualTo("lamp_off");
        assertThat(FaultCodes.faultTypeFromErrCode(FaultCodes.ERR_RYG_ON)).isEqualTo("abnormal_on");
        assertThat(FaultCodes.faultTypeFromErrCode(FaultCodes.ERR_G_ON_TIMEOUT)).isEqualTo("timeout");
        assertThat(FaultCodes.faultTypeFromErrCode(FaultCodes.ERR_Y_DIM)).isEqualTo("dim");
        assertThat(FaultCodes.faultTypeFromErrCode(FaultCodes.ERR_POWER_LOSS)).isEqualTo("power_loss");

        assertThat(FaultCodes.faultLevelFromErrCode(FaultCodes.ERR_R_OFF)).isEqualTo("critical");
        assertThat(FaultCodes.faultLevelFromErrCode(FaultCodes.ERR_POWER_LOSS)).isEqualTo("critical");
        assertThat(FaultCodes.faultLevelFromErrCode(FaultCodes.ERR_G_ON_TIMEOUT)).isEqualTo("normal");
        assertThat(FaultCodes.faultLevelFromErrCode(FaultCodes.ERR_OK)).isEqualTo("normal");
    }
}
