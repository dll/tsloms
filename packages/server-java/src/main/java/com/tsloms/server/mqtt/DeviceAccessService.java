// 设备接入消息处理：解析→分发→设备 upsert→故障研判(规则基座+去重窗口)→回应下发。
//
// 与 Go 版差异说明：多源证据/案例库/预警联动属识别引擎阶段迁移范围；
// 本版研判采用确定性规则基座（errCode→类型/等级，置信度 1.0，来源 rule，confirmed），
// R2 状态机 / R3 去重窗口(30min) / R6 critical 自动工单 / 别名匹配 全部保持。
package com.tsloms.server.mqtt;

import com.tsloms.server.mqtt.protocol.Cmd;
import com.tsloms.server.mqtt.protocol.FaultCodes;
import com.tsloms.server.mqtt.protocol.HardwareIds;
import com.tsloms.server.mqtt.protocol.ProtocolParser;
import com.tsloms.server.model.Device;
import com.tsloms.server.model.FaultRecord;
import com.tsloms.server.model.WorkOrder;
import com.tsloms.server.repository.DeviceRepository;
import com.tsloms.server.repository.FaultRecordRepository;
import com.tsloms.server.repository.WorkOrderRepository;
import com.tsloms.server.workorder.WorkOrders;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.stereotype.Service;

@Service
public class DeviceAccessService {

    private static final Logger log = LoggerFactory.getLogger(DeviceAccessService.class);

    /** 故障去重窗口：30 分钟（R3）。 */
    private static final long DEDUP_WINDOW_MINUTES = 30;

    /** 下行发布网关抽象（测试可注入 fake）。 */
    public interface Gateway {
        void publish(String topic, byte[] payload);
    }

    private final DeviceRepository devices;
    private final FaultRecordRepository faults;
    private final WorkOrderRepository workOrders;
    private final com.tsloms.server.repository.FirmwarePackageRepository firmwares;
    private final Gateway gateway;
    private final com.tsloms.server.config.MqttProperties mqtt;

    public DeviceAccessService(DeviceRepository devices, FaultRecordRepository faults,
                               WorkOrderRepository workOrders,
                               com.tsloms.server.repository.FirmwarePackageRepository firmwares,
                               Gateway gateway,
                               com.tsloms.server.config.MqttProperties mqtt) {
        this.devices = devices;
        this.faults = faults;
        this.workOrders = workOrders;
        this.firmwares = firmwares;
        this.gateway = gateway;
        this.mqtt = mqtt;
    }

    /** MQTT 消息入口：解析→事件包→报文日志→分发。 */
    public void handleMessage(byte[] payload, String topic) {
        Cmd.Frame frame;
        try {
            frame = ProtocolParser.parseCmdFrame(payload);
        } catch (ProtocolParser.ProtocolException e) {
            log.warn("[TSLOMS] 命令帧解析失败 topic={} err={}", topic, e.getMessage());
            return;
        }

        Cmd.EventPak eventPak = null;
        boolean carriesEvents = frame.datLen > 0 && (frame.cmd == Cmd.CHECKIN
                || frame.cmd == Cmd.ALARM || frame.cmd == Cmd.POWER_ON);
        if (carriesEvents) {
            try {
                eventPak = ProtocolParser.parseEventPak(frame.data);
            } catch (ProtocolParser.ProtocolException e) {
                log.warn("[TSLOMS] 事件包解析失败 topic={} cmd={} err={}",
                        topic, frame.cmd, e.getMessage());
            }
        }

        switch (frame.cmd) {
            case Cmd.CHECKIN -> handleCheckin(frame, eventPak, topic);
            case Cmd.ALARM -> handleAlarm(frame, eventPak);
            case Cmd.POWER_ON -> handlePowerOn(frame, eventPak, topic);
            case Cmd.CHECK_FW, Cmd.GET_FW -> handleFirmwareQuery(frame, topic);
            default -> log.warn("[TSLOMS] 未知命令类型 topic={} cmd={}", topic, frame.cmd);
        }
    }

    // ---------------- 各命令处理 ----------------

    private void handleCheckin(Cmd.Frame frame, Cmd.EventPak pak, String uplinkTopic) {
        Instant now = Instant.now();
        if (pak != null) {
            upsertLastRecords(pak.records, now);
            for (Cmd.EventRecord rec : pak.records) {
                if (rec.errCode != FaultCodes.ERR_OK) {
                    processFault(rec);
                }
            }
        }
        sendTimeSyncAck(frame, uplinkTopic);
        log.info("[TSLOMS] 设备签到完成 swVer={} seq={}", frame.swVer, frame.cmdSeq);
    }

    private void handleAlarm(Cmd.Frame frame, Cmd.EventPak pak) {
        if (pak == null || pak.records.isEmpty()) {
            log.warn("[TSLOMS] 告警消息无事件记录 seq={}", frame.cmdSeq);
            return;
        }
        upsertLastRecords(pak.records, Instant.now());
        for (Cmd.EventRecord rec : pak.records) {
            processFault(rec);
        }
        log.info("[TSLOMS] 设备告警处理完成 seq={} count={}", frame.cmdSeq, pak.records.size());
    }

    private void handlePowerOn(Cmd.Frame frame, Cmd.EventPak pak, String uplinkTopic) {
        if (pak != null) {
            upsertLastRecords(pak.records, Instant.now());
        }
        sendTimeSyncAck(frame, uplinkTopic);
        log.info("[TSLOMS] 上电报告完成 swVer={} seq={}", frame.swVer, frame.cmdSeq);
    }

    /** 固件查询回应：ack 帧 data=目标 swVersion 位域；0 表示无新版本。 */
    private void handleFirmwareQuery(Cmd.Frame frame, String uplinkTopic) {
        long targetSw = firmwares
                .findOne(com.tsloms.server.repository.FirmwarePackageRepository.PUBLISHED)
                .map(f -> f.swVersion).orElse(0L);
        byte[] ack = ProtocolParser.buildTimeSyncAck(frame.cmd, frame.swVer,
                frame.cmdSeq, targetSw); // userVal 承载目标版本（与 Go 版 sendFWCheckAck 一致）
        gateway.publish(buildDownTopic(uplinkTopic, frame.cmdSeq), ack);
    }

    private void sendTimeSyncAck(Cmd.Frame frame, String uplinkTopic) {
        long epoch = Instant.now().getEpochSecond();
        byte[] ack = ProtocolParser.buildTimeSyncAck(frame.cmd, frame.swVer,
                frame.cmdSeq, epoch);
        gateway.publish(buildDownTopic(uplinkTopic, frame.cmdSeq), ack);
    }

    /** 从上行 topic 构造下行：末段 /U → /D。 */
    String buildDownTopic(String uplinkTopic, int cmdSeq) {
        String down = uplinkTopic != null && uplinkTopic.endsWith("/U")
                ? uplinkTopic.substring(0, uplinkTopic.length() - 2) + "/D"
                : uplinkTopic + "/D";
        if ("/D".equals(down)) {
            down = mqtt.topicPrefix() + "/down/" + cmdSeq + "/ack";
        }
        return down;
    }

    // ---------------- 设备 upsert ----------------

    /** 同帧同硬件 ID 取末条合并为一次 upsert（对齐 Go 版 lastRecords）。 */
    static List<Cmd.EventRecord> lastRecords(List<Cmd.EventRecord> records) {
        Map<Long, Integer> lastIdx = new HashMap<>();
        for (int i = 0; i < records.size(); i++) {
            lastIdx.put(records.get(i).ledHwId, i);
        }
        List<Cmd.EventRecord> out = new ArrayList<>();
        for (Integer i : lastIdx.values()) {
            out.add(records.get(i));
        }
        return out;
    }

    private void upsertLastRecords(List<Cmd.EventRecord> records, Instant now) {
        for (Cmd.EventRecord rec : lastRecords(records)) {
            upsertDevice(rec, now);
        }
    }

    private void upsertDevice(Cmd.EventRecord rec, Instant checkinTime) {
        String legacyId = HardwareIds.ledUuid(rec.ledHwId);
        List<String> aliases = HardwareIds.aliases(legacyId);
        var found = devices.findAll().stream()
                .filter(d -> aliases.contains(d.hwId))
                .findFirst();

        if (found.isEmpty()) {
            Device d = new Device();
            d.hwId = legacyId;
            d.swVersion = rec.swVer;
            d.confVersion = rec.confVer;
            d.onlineStatus = true;
            d.lastCheckinAt = checkinTime;
            d.lifecycleStatus = "active";
            d.accessStatus = "accessed";
            d.firstAccessAt = checkinTime;
            d.registrationSource = "mqtt_auto";
            devices.save(d);
            return;
        }
        Device device = found.get();
        // 报废设备不复活，仅记录最后上报时间供审计
        if ("retired".equals(device.lifecycleStatus)) {
            device.lastCheckinAt = checkinTime;
            devices.save(device);
            log.warn("[TSLOMS] 已报废设备仍在上报 hwId={}，忽略自动恢复", legacyId);
            return;
        }
        device.swVersion = rec.swVer;
        device.confVersion = rec.confVer;
        device.onlineStatus = true;
        device.lastCheckinAt = checkinTime;
        device.accessStatus = "accessed";
        if (device.firstAccessAt == null) {
            device.firstAccessAt = checkinTime;
        }
        if (device.lifecycleStatus == null || device.lifecycleStatus.isEmpty()) {
            device.lifecycleStatus = "active";
        }
        devices.save(device);
    }

    /** 台账规范 ID：已登记优先保留（如 LA+8 位），未登记回退协议历史编码。 */
    private String canonicalHardwareId(long ledHwId) {
        String legacy = HardwareIds.ledUuid(ledHwId);
        List<String> aliases = HardwareIds.aliases(legacy);
        return devices.findAll().stream()
                .filter(d -> aliases.contains(d.hwId))
                .map(d -> d.hwId)
                .findFirst()
                .orElse(legacy);
    }

    // ---------------- 故障研判（规则基座） ----------------

    /**
     * 故障研判与去重：
     * 同设备同 errCode 活跃记录在 30 分钟窗口内 → 更新 lastSeen（电流/灯态变化才附带更新）；
     * 超窗 → 旧故障置 resolved 并新建；critical 且 confirmed → 自动生成工单（R6）。
     */
    void processFault(Cmd.EventRecord rec) {
        Instant now = Instant.now();
        String canonicalId = canonicalHardwareId(rec.ledHwId);
        List<String> aliases = HardwareIds.aliases(canonicalId);
        List<String> activeStatuses = List.of("occurred", "confirmed", "dispatched");

        var existingOpt = faults.findAll().stream()
                .filter(f -> aliases.contains(f.deviceHwId)
                        && f.errCode != null && f.errCode == rec.errCode
                        && activeStatuses.contains(f.status))
                .findFirst();

        if (existingOpt.isPresent()) {
            FaultRecord existing = existingOpt.get();
            long minutesSince = DurationUntil(existing.lastSeen, now);
            if (minutesSince <= DEDUP_WINDOW_MINUTES) {
                // 在窗更新 lastSeen；电流/灯态变化才附带更新
                existing.lastSeen = now;
                if (existing.currentR == null || existing.currentR != rec.currentR
                        || existing.currentY == null || existing.currentY != rec.currentY
                        || existing.currentG == null || existing.currentG != rec.currentG
                        || existing.ledState == null || existing.ledState != rec.ledState) {
                    existing.currentR = rec.currentR;
                    existing.currentY = rec.currentY;
                    existing.currentG = rec.currentG;
                    existing.ledState = (short) rec.ledState;
                }
                faults.save(existing);
                return;
            }
            // 超窗：旧故障标记解决，落到下方新建
            existing.status = "resolved";
            faults.save(existing);
        }

        // 新建故障（规则基座判定：confirmed / confidence=1.0）
        FaultRecord f = new FaultRecord();
        f.deviceHwId = canonicalId;
        f.errCode = (short) rec.errCode;
        f.faultType = FaultCodes.faultTypeFromErrCode(rec.errCode);
        f.faultLevel = FaultCodes.faultLevelFromErrCode(rec.errCode);
        f.ledState = (short) rec.ledState;
        f.currentR = rec.currentR;
        f.currentY = rec.currentY;
        f.currentG = rec.currentG;
        f.firstSeen = now;
        f.lastSeen = now;
        f.status = "occurred";
        f.confidence = 1.0;
        f.recognitionSource = "rule";
        f.recognitionStatus = "confirmed";
        f.evidenceCount = 1;
        try {
            faults.saveAndFlush(f);
        } catch (Exception e) {
            log.error("[TSLOMS] 创建故障记录失败 hwId={} errCode={}", rec.ledHwId, rec.errCode, e);
            return;
        }

        // critical 自动工单（R6）
        if ("critical".equals(f.faultLevel)) {
            ensureActiveWorkOrder(f.id, canonicalId);
        }
        log.info("[TSLOMS] 故障研判完成 hwId={} type={} level={} faultId={}",
                rec.ledHwId, f.faultType, f.faultLevel, f.id);
    }

    /**
     * 为故障原子创建/复用活跃工单并回填 fault.work_order_id（M1 语义）。
     * 依赖唯一索引 uk_wo_active_scope 时由 DB 拦截冲突后复用既有活跃单。
     */
    private void ensureActiveWorkOrder(Long faultId, String deviceHwId) {
        WorkOrder wo = new WorkOrder();
        wo.orderNo = WorkOrders.nextOrderNo(workOrders);
        wo.faultId = faultId;
        wo.deviceHwId = deviceHwId;
        wo.status = "pending";
        wo.faultActiveScope = faultId;
        try {
            workOrders.saveAndFlush(wo);
        } catch (DataIntegrityViolationException e) {
            var reuse = workOrders.findFirstByFaultIdAndFaultActiveScope(faultId, faultId);
            if (reuse.isEmpty()) {
                return;
            }
            wo = reuse.get();
        }
        Long woId = wo.id;
        final WorkOrder effective = wo;
        faults.findById(faultId).ifPresent(f -> {
            if (f.workOrderId == null) { // 条件回填，不覆盖他人关联
                f.workOrderId = woId;
                f.status = "confirmed";
                f.confirmedAt = Instant.now();
                faults.save(f);
            }
        });
        log.info("[TSLOMS] 已确保活跃工单 orderNo={} fault={}", effective.orderNo, faultId);
    }

    private static long DurationUntil(Instant from, Instant to) {
        return from.until(to, ChronoUnit.MINUTES);
    }
}
