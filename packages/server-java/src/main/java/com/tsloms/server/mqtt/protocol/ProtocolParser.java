// 协议解析/构建器：小端序，校验和=整包 uint8 累加低 8 位 == 0xFF。
// 与 Go 版 parser.go 逐字节语义一致（含错误分支顺序：长度→魔术字→datLen→校验和）。
package com.tsloms.server.mqtt.protocol;

import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.util.ArrayList;
import java.util.List;

public final class ProtocolParser {

    private ProtocolParser() {
    }

    /** 解析命令帧；非法帧抛 ProtocolException。 */
    public static Cmd.Frame parseCmdFrame(byte[] data) {
        if (data == null || data.length < Cmd.FRAME_HEADER_LEN) {
            throw new ProtocolException("数据长度不足: 需要 " + Cmd.FRAME_HEADER_LEN
                    + " 字节, 实际 " + (data == null ? 0 : data.length) + " 字节");
        }
        Cmd.Frame f = new Cmd.Frame();
        f.token = data[0] & 0xFF;
        f.cmd = data[1] & 0xFF;
        f.ver = data[2] & 0xFF;
        f.checksum = data[3] & 0xFF;
        ByteBuffer buf = ByteBuffer.wrap(data).order(ByteOrder.LITTLE_ENDIAN);
        buf.position(4);
        f.swVer = buf.getInt() & 0xFFFFFFFFL;
        f.cmdSeq = buf.getShort() & 0xFFFF;
        f.datLen = buf.getShort() & 0xFFFF;
        f.userVal = buf.getInt() & 0xFFFFFFFFL;

        if (f.token != Cmd.TOKEN) {
            throw new ProtocolException(String.format(
                    "魔术字错误: 期望 0x%02X, 实际 0x%02X", Cmd.TOKEN, f.token));
        }
        if (f.datLen != data.length - Cmd.FRAME_HEADER_LEN) {
            throw new ProtocolException("数据长度不匹配: datLen=" + f.datLen
                    + ", 实际数据部分=" + (data.length - Cmd.FRAME_HEADER_LEN));
        }
        int sum = 0;
        for (byte b : data) {
            sum = (sum + (b & 0xFF)) & 0x1FF; // 累加按 uint16 承载，取低 8 位比较
        }
        if ((sum & 0xFF) != Cmd.CHECKSUM_VALID) {
            throw new ProtocolException(String.format(
                    "校验和错误: 期望 0x%02X, 实际 0x%02X", Cmd.CHECKSUM_VALID, sum & 0xFF));
        }
        if (f.datLen > 0) {
            f.data = new byte[f.datLen];
            System.arraycopy(data, Cmd.FRAME_HEADER_LEN, f.data, 0, f.datLen);
        }
        return f;
    }

    /** 解析事件包（签到/告警数据部分）。 */
    public static Cmd.EventPak parseEventPak(byte[] data) {
        if (data == null || data.length < Cmd.EVENT_PAK_HEADER_LEN) {
            throw new ProtocolException("事件包数据长度不足: 需要 " + Cmd.EVENT_PAK_HEADER_LEN
                    + " 字节, 实际 " + (data == null ? 0 : data.length) + " 字节");
        }
        Cmd.EventPak pak = new Cmd.EventPak();
        ByteBuffer buf = ByteBuffer.wrap(data).order(ByteOrder.LITTLE_ENDIAN);
        pak.recordNum = buf.getShort() & 0xFFFF;
        pak.datLen = buf.getShort() & 0xFFFF;
        int expectedLen = pak.recordNum * Cmd.EVENT_RECORD_LEN;
        if (pak.datLen != expectedLen) {
            throw new ProtocolException("事件包数据长度不匹配: datLen=" + pak.datLen
                    + ", 期望 " + expectedLen + " (记录数=" + pak.recordNum + "*"
                    + Cmd.EVENT_RECORD_LEN + ")");
        }
        byte[] recordData = new byte[data.length - Cmd.EVENT_PAK_HEADER_LEN];
        System.arraycopy(data, Cmd.EVENT_PAK_HEADER_LEN, recordData, 0, recordData.length);
        if (recordData.length < expectedLen) {
            throw new ProtocolException("事件记录数据不足: 需要 " + expectedLen
                    + " 字节, 实际 " + recordData.length + " 字节");
        }
        List<Cmd.EventRecord> records = new ArrayList<>(pak.recordNum);
        for (int i = 0; i < pak.recordNum; i++) {
            records.add(parseEventRecord(recordData, i * Cmd.EVENT_RECORD_LEN));
        }
        pak.records = records;
        return pak;
    }

    /** 解析单条事件记录（24 字节，1 字节对齐）。 */
    public static Cmd.EventRecord parseEventRecord(byte[] data, int offset) {
        if (data.length - offset < Cmd.EVENT_RECORD_LEN) {
            throw new ProtocolException("事件记录数据长度不足: 需要 " + Cmd.EVENT_RECORD_LEN
                    + " 字节");
        }
        ByteBuffer buf = ByteBuffer.wrap(data, offset, Cmd.EVENT_RECORD_LEN)
                .order(ByteOrder.LITTLE_ENDIAN);
        Cmd.EventRecord r = new Cmd.EventRecord();
        r.ledHwId = buf.getInt() & 0xFFFFFFFFL;
        r.subHwId = buf.getInt() & 0xFFFFFFFFL;
        r.swVer = buf.getInt() & 0xFFFFFFFFL;
        r.confVer = buf.getInt() & 0xFFFFFFFFL;
        r.ledState = buf.get();
        r.errCode = buf.get();
        r.currentR = buf.getShort() & 0xFFFF;
        r.currentY = buf.getShort() & 0xFFFF;
        r.currentG = buf.getShort() & 0xFFFF;
        return r;
    }

    /**
     * 构建命令帧：自动计算校验和使整包累加低 8 位 == 0xFF。
     */
    public static byte[] buildCmdFrame(int cmd, long swVer, int cmdSeq, long userVal,
                                       byte[] data) {
        int datLen = data == null ? 0 : data.length;
        byte[] frame = new byte[Cmd.FRAME_HEADER_LEN + datLen];
        frame[0] = (byte) Cmd.TOKEN;
        frame[1] = (byte) cmd;
        frame[2] = (byte) Cmd.VER;
        // checksum 暂填 0
        ByteBuffer buf = ByteBuffer.wrap(frame).order(ByteOrder.LITTLE_ENDIAN);
        buf.position(4);
        buf.putInt((int) swVer);
        buf.putShort((short) cmdSeq);
        buf.putShort((short) datLen);
        buf.putInt((int) userVal);
        if (datLen > 0) {
            System.arraycopy(data, 0, frame, Cmd.FRAME_HEADER_LEN, datLen);
        }
        int sum = 0;
        for (byte b : frame) {
            sum += b & 0xFF;
        }
        frame[3] = (byte) ((Cmd.CHECKSUM_VALID - (sum & 0xFF)) & 0xFF);
        return frame;
    }

    /** 时间同步回应帧：ack 标记 + userVal=epoch 秒（无数据部分）。 */
    public static byte[] buildTimeSyncAck(int cmd, long swVer, int cmdSeq,
                                          long epochSeconds) {
        return buildCmdFrame(Cmd.makeAck(cmd), swVer, cmdSeq, epochSeconds, null);
    }

    /** 解析失败异常（供上层记录无效报文）。 */
    public static class ProtocolException extends RuntimeException {
        public ProtocolException(String message) {
            super(message);
        }
    }
}
