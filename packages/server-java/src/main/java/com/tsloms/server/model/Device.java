// 设备实体：逐列对齐 Go 版 internal/model/device.go（表 devices）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Index;
import jakarta.persistence.Table;
import jakarta.persistence.UniqueConstraint;
import java.time.Instant;

/** 信号灯监控设备台账；hw_id 为出厂唯一硬件 ID。 */
@Entity
@Table(name = "devices",
        uniqueConstraints = @UniqueConstraint(name = "uk_devices_hw_id", columnNames = "hw_id"),
        indexes = {
                @Index(name = "idx_devices_intersection", columnList = "intersection"),
                @Index(name = "idx_devices_crossing", columnList = "crossing_id"),
                @Index(name = "idx_devices_lifecycle", columnList = "lifecycle_status"),
                @Index(name = "idx_devices_access", columnList = "access_status"),
        })
public class Device extends BaseEntity {

    @Column(name = "hw_id", nullable = false, length = 64)
    public String hwId;

    /** 路口位置描述。 */
    @Column(name = "intersection", length = 128)
    public String intersection;

    // ---- P0-4 路口/行政区划挂接（均可空）----
    @Column(name = "crossing_id")
    public Long crossingId;

    @Column(name = "province_id")
    public Long provinceId;

    @Column(name = "city_id")
    public Long cityId;

    @Column(name = "district_id")
    public Long districtId;

    @Column(name = "street_id")
    public Long streetId;

    @Column(name = "community_id")
    public Long communityId;

    @Column(name = "road_id")
    public Long roadId;

    @Column(name = "road_name", length = 128)
    public String roadName;

    @Column(name = "lat")
    public Double lat;

    @Column(name = "lng")
    public Double lng;

    /** 网络号。 */
    @Column(name = "network_code", nullable = false)
    public Integer networkCode = 0;

    /** 站点号。 */
    @Column(name = "station_code", nullable = false)
    public Integer stationCode = 0;

    /** 固件版本号位域（uint32 → Long）。 */
    @Column(name = "sw_version", nullable = false)
    public Long swVersion = 0L;

    /** 配置版本号位域（uint32 → Long）。 */
    @Column(name = "conf_version", nullable = false)
    public Long confVersion = 0L;

    @Column(name = "online_status", nullable = false)
    public boolean onlineStatus;

    /** 生命周期：active/retired。 */
    @Column(name = "lifecycle_status", nullable = false, length = 16)
    public String lifecycleStatus = "active";

    /** 接入状态：never/accessed/offline。 */
    @Column(name = "access_status", nullable = false, length = 16)
    public String accessStatus = "never";

    @Column(name = "first_access_at")
    public Instant firstAccessAt;

    @Column(name = "retired_at")
    public Instant retiredAt;

    @Column(name = "retired_reason", length = 255)
    public String retiredReason;

    /** 建账来源：manual/mqtt_auto。 */
    @Column(name = "registration_source", nullable = false, length = 16)
    public String registrationSource = "manual";

    @Column(name = "is_watched", nullable = false)
    public boolean isWatched;

    @Column(name = "last_checkin_at")
    public Instant lastCheckinAt;

    @Column(name = "installed_at")
    public Instant installedAt;

    // ---- 设备资料 ----
    @Column(name = "photo", length = 500)
    public String photo;

    @Column(name = "manual_url", length = 500)
    public String manualUrl;

    @Column(name = "manual_name", length = 255)
    public String manualName;

    @Column(name = "repair_manual_url", length = 500)
    public String repairManualUrl;

    @Column(name = "repair_manual_name", length = 255)
    public String repairManualName;

    // ---- 参考项目 a 对齐字段 ----
    /** 信号灯功能（灯组类型：直行/左转/右转）。 */
    @Column(name = "func", length = 32)
    public String func;

    /** 朝向（南/北/东/西等）。 */
    @Column(name = "orientation", length = 16)
    public String orientation;

    @Column(name = "direction", length = 16)
    public String direction;

    @Column(name = "batch", length = 32)
    public String batch;

    @Column(name = "remark", length = 255)
    public String remark;
}
