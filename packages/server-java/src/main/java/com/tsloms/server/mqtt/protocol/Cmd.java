// 二进制协议帧结构与解析/构建：逐字节对齐 Go 版 internal/mqtt parser.go+commands.go。
// 关键点：小端序；校验和=整包字节 uint8 累加低 8 位须为 0xFF。
package com.tsloms.server.mqtt.protocol;

public final class Cmd {

    /** 帧头长度：token+cmd+ver+checksum+swVer(4)+cmdSeq(2)+datLen(2)+userVal(4)。 */
    public static final int FRAME_HEADER_LEN = 16;
    /** 事件包头：eventRecordNum(2)+datLen(2)。 */
    public static final int EVENT_PAK_HEADER_LEN = 4;
    /** 单条事件记录长度（1 字节对齐）。 */
    public static final int EVENT_RECORD_LEN = 24;

    public static final int TOKEN = 0x55;           // 魔术字
    public static final int VER = 0x10;             // 协议版本 v1.0
    public static final int CHECKSUM_VALID = 0xFF;  // 累加和目标值

    // 命令码
    public static final int CHECKIN = 0x00;      // 定时签到（携带事件）
    public static final int ALARM = 0x01;        // 告警上报
    public static final int POWER_ON = 0x03;     // 上电报告
    public static final int UPDATE_CONFIG = 0x20;
    public static final int CHECK_FW = 0x30;     // 固件查询
    public static final int GET_FW = 0x31;       // 固件数据请求
    public static final int REBOOT = 0x7F;
    public static final int ACK_FLAG = 0x80;     // bit7=1 回应帧

    private Cmd() {
    }

    public static boolean isAck(int cmd) {
        return (cmd & ACK_FLAG) != 0;
    }

    public static int makeAck(int cmd) {
        return cmd | ACK_FLAG;
    }

    /** CMD_FRAME 解析结果。 */
    public static final class Frame {
        public int token;
        public int cmd;
        public int ver;
        public int checksum;
        public long swVer;    // uint32
        public int cmdSeq;    // uint16
        public int datLen;    // uint16
        public long userVal;  // uint32
        public byte[] data;
    }

    /** 事件包。 */
    public static final class EventPak {
        public int recordNum;
        public int datLen;
        public java.util.List<EventRecord> records = new java.util.ArrayList<>();
    }

    /** 单条事件记录（24 字节）。 */
    public static final class EventRecord {
        public long ledHwId;   // uint32 设备硬件 ID
        public long subHwId;
        public long swVer;
        public long confVer;
        public byte ledState;  // int8
        public byte errCode;   // int8
        public int currentR;   // uint16
        public int currentY;
        public int currentG;
    }
}
