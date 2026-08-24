// 设备接入流程集成测试：签到建账/上电/告警去重/critical 自动工单/报废不复活。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;

import com.tsloms.server.mqtt.DeviceAccessService;
import com.tsloms.server.mqtt.protocol.Cmd;
import com.tsloms.server.mqtt.protocol.FaultCodes;
import com.tsloms.server.mqtt.protocol.HardwareIds;
import com.tsloms.server.mqtt.protocol.ProtocolParser;
import com.tsloms.server.model.Device;
import com.tsloms.server.model.FaultRecord;
import com.tsloms.server.repository.DeviceRepository;
import com.tsloms.server.repository.FaultRecordRepository;
import com.tsloms.server.repository.WorkOrderRepository;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.context.TestConfiguration;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Primary;

@SpringBootTest
class DeviceAccessFlowTest {

    /** 下行捕获记录器（测试专用 @Primary 网关）。 */
    public static final class Recorder implements DeviceAccessService.Gateway {
        public final List<Sent> sent = new ArrayList<>();

        @Override
        public void publish(String topic, byte[] payload) {
            sent.add(new Sent(topic, payload));
        }
    }

    public record Sent(String topic, byte[] payload) {
    }

    public static final Recorder RECORDER = new Recorder();

    @TestConfiguration
    static class Cfg {
        @Bean
        @Primary
        DeviceAccessService.Gateway recordingGateway() {
            return RECORDER;
        }
    }

    @Autowired DeviceAccessService handler;
    @Autowired DeviceRepository devices;
    @Autowired FaultRecordRepository faults;
    @Autowired WorkOrderRepository workOrders;

    private byte[] eventPak(byte[][] records) {
        ByteBuffer buf = ByteBuffer.allocate(4 + records.length * Cmd.EVENT_RECORD_LEN)
                .order(ByteOrder.LITTLE_ENDIAN);
        buf.putShort((short) records.length);
        buf.putShort((short) (records.length * Cmd.EVENT_RECORD_LEN));
        for (byte[] r : records) {
            buf.put(r);
        }
        return buf.array();
    }

    private byte[] record(long ledHw, byte state, byte errCode) {
        return ByteBuffer.allocate(Cmd.EVENT_RECORD_LEN)
                .order(ByteOrder.LITTLE_ENDIAN)
                .putInt((int) ledHw).putInt(1).putInt(0x01020304).putInt(0x26081401)
                .put(state).put(errCode)
                .putShort((short) 800).putShort((short) 700).putShort((short) 600)
                .array();
    }

    @Test
    void 签到_自动建设备在线并发送时间同步回应() {
        RECORDER.sent.clear();
        long hw = 0x1234ABCDL;
        String hwId = HardwareIds.ledUuid(hw);

        byte[] frame = ProtocolParser.buildCmdFrame(Cmd.CHECKIN, 0x01020304L, 11, 0,
                eventPak(new byte[][]{record(hw, FaultCodes.STATE_G, FaultCodes.ERR_OK)}));
        handler.handleMessage(frame, "trafficLight/1/2/" + hwId + "/U");

        Device d = devices.findByHwId(hwId).orElseThrow();
        assertThat(d.onlineStatus).isTrue();
        assertThat(d.accessStatus).isEqualTo("accessed");
        assertThat(d.registrationSource).isEqualTo("mqtt_auto");
        assertThat(d.firstAccessAt).isNotNull();
        assertThat(d.lastCheckinAt).isNotNull();

        assertThat(RECORDER.sent).hasSize(1);
        Cmd.Frame ack = ProtocolParser.parseCmdFrame(RECORDER.sent.get(0).payload());
        assertThat(Cmd.isAck(ack.cmd)).isTrue();
        assertThat(ack.userVal).isPositive();
        assertThat(RECORDER.sent.get(0).topic())
                .isEqualTo("trafficLight/1/2/" + hwId + "/D");

        devices.delete(d);
    }

    @Test
    void 告警critical_自动生成故障与工单() {
        RECORDER.sent.clear();
        long hw = 0xAABB0001L;
        String hwId = HardwareIds.ledUuid(hw);

        byte[] frame = ProtocolParser.buildCmdFrame(Cmd.ALARM, 0x01020304L, 21, 0,
                eventPak(new byte[][]{record(hw, FaultCodes.STATE_NONE, FaultCodes.ERR_R_OFF)}));
        handler.handleMessage(frame, "x/U");

        var faultOpt = faults.findAll().stream()
                .filter(f -> hwId.equals(f.deviceHwId)
                        && f.errCode != null && f.errCode == FaultCodes.ERR_R_OFF)
                .findFirst();
        assertThat(faultOpt).isPresent();
        FaultRecord f = faultOpt.get();
        assertThat(f.faultType).isEqualTo("lamp_off");
        assertThat(f.faultLevel).isEqualTo("critical");
        assertThat(List.of("occurred", "confirmed")).contains(f.status);
        assertThat(f.recognitionStatus).isEqualTo("confirmed");

        var woOpt = workOrders.findAll().stream()
                .filter(w -> Long.valueOf(f.id).equals(w.faultId)).findFirst();
        assertThat(woOpt).isPresent();
        assertThat(woOpt.get().status).isEqualTo("pending");
        assertThat(woOpt.get().faultActiveScope).isEqualTo(f.id);

        // 清理
        workOrders.delete(woOpt.get());
        faults.delete(f);
        devices.findByHwId(hwId).ifPresent(devices::delete);
    }

    @Test
    void 告警去重窗口内_更新lastSeen不重复建单() {
        RECORDER.sent.clear();
        long hw = 0xAABB0002L;
        String hwId = HardwareIds.ledUuid(hw);

        byte[] frame = ProtocolParser.buildCmdFrame(Cmd.ALARM, 1, 31, 0,
                eventPak(new byte[][]{record(hw, FaultCodes.STATE_NONE, FaultCodes.ERR_R_OFF)}));
        handler.handleMessage(frame, "x/U");
        handler.handleMessage(frame, "x/U");

        long count = faults.findAll().stream()
                .filter(f -> hwId.equals(f.deviceHwId)).count();
        assertThat(count).isEqualTo(1);
        long woCount = workOrders.findAll().stream()
                .filter(w -> hwId.equals(w.deviceHwId)).count();
        assertThat(woCount).isEqualTo(1);

        faults.findAll().stream().filter(f -> hwId.equals(f.deviceHwId))
                .forEach(faults::delete);
        workOrders.findAll().stream().filter(w -> hwId.equals(w.deviceHwId))
                .forEach(workOrders::delete);
        devices.findByHwId(hwId).ifPresent(devices::delete);
    }

    @Test
    void 超过去重窗口_旧故障解决新建() {
        RECORDER.sent.clear();
        long hw = 0xAABB0003L;
        String hwId = HardwareIds.ledUuid(hw);

        FaultRecord old = new FaultRecord();
        old.deviceHwId = hwId;
        old.errCode = FaultCodes.ERR_G_DIM;
        old.faultType = "dim";
        old.faultLevel = "normal";
        old.firstSeen = Instant.now().minus(61, ChronoUnit.MINUTES);
        old.lastSeen = Instant.now().minus(31, ChronoUnit.MINUTES); // 超窗
        old.status = "confirmed";
        faults.saveAndFlush(old);

        byte[] frame = ProtocolParser.buildCmdFrame(Cmd.ALARM, 1, 41, 0,
                eventPak(new byte[][]{record(hw, FaultCodes.STATE_NONE, FaultCodes.ERR_G_DIM)}));
        handler.handleMessage(frame, "x/U");

        var all = faults.findAll().stream()
                .filter(f -> hwId.equals(f.deviceHwId)).toList();
        assertThat(all.size()).isEqualTo(2);
        boolean oldResolved = all.stream()
                .filter(x -> old.id.equals(x.id))
                .allMatch(x -> "resolved".equals(x.status));
        assertThat(oldResolved).isTrue();

        all.forEach(faults::delete);
        devices.findByHwId(hwId).ifPresent(devices::delete);
    }

    @Test
    void 报废设备上报_不复活() {
        RECORDER.sent.clear();
        long hw = 0xAABB0004L;
        String hwId = HardwareIds.ledUuid(hw);
        txSeedRetired(hwId);

        byte[] frame = ProtocolParser.buildCmdFrame(Cmd.POWER_ON, 1, 51, 0,
                eventPak(new byte[][]{record(hw, FaultCodes.STATE_R, FaultCodes.ERR_OK)}));
        handler.handleMessage(frame, "x/U");

        Device d = devices.findByHwId(hwId).orElseThrow();
        assertThat(d.lifecycleStatus).isEqualTo("retired"); // 不复活
        assertThat(d.lastCheckinAt).isNotNull();             // 记录最后上报时间
        devices.delete(d);
    }

    @org.springframework.beans.factory.annotation.Autowired
    org.springframework.transaction.support.TransactionTemplate tx;

    private void txSeedRetired(String hwId) {
        tx.executeWithoutResult(s -> {
            Device d = new Device();
            d.hwId = hwId;
            d.lifecycleStatus = "retired";
            d.retiredAt = Instant.now();
            d.retiredReason = "报废";
            d.onlineStatus = false;
            devices.save(d);
        });
    }
}
