// 路口实体：对齐 Go 版 Crossing（表 crossings）。设备 devices.crossing_id 多对一。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Index;
import jakarta.persistence.PrePersist;
import jakarta.persistence.PreUpdate;
import jakarta.persistence.Table;
import java.time.Instant;

@Entity
@Table(name = "crossings", indexes = {
        @Index(name = "idx_crossings_point_no", columnList = "point_no"),
        @Index(name = "idx_crossings_name", columnList = "name"),
})
public class Crossing {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    /** 点位编号。 */
    @Column(name = "point_no", length = 64)
    public String pointNo;

    @Column(name = "name", nullable = false, length = 128)
    public String name;

    /** 路口类型（1直行/2丁字/3+多路口）。 */
    @Column(name = "type", length = 16)
    public String type;

    // ---- 行政区划挂接 ----
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

    /** 综合状态缓存（normal/abnormal/offline/maintain/flashing/monitor）。 */
    @Column(name = "status", nullable = false, length = 16)
    public String status = "normal";

    /** 故障上报比率（异常设备/全部）。 */
    @Column(name = "fault_ratio", nullable = false)
    public Double faultRatio = 0.0;

    /** 绿灯比率（正常率）。 */
    @Column(name = "green_ratio", nullable = false)
    public Double greenRatio = 0.0;

    @Column(name = "remark", length = 255)
    public String remark;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    @Column(name = "updated_at", nullable = false)
    public Instant updatedAt;

    @PrePersist
    void onCreate() {
        Instant now = Instant.now();
        if (createdAt == null) {
            createdAt = now;
        }
        if (updatedAt == null) {
            updatedAt = now;
        }
    }

    @PreUpdate
    void onUpdate() {
        updatedAt = Instant.now();
    }
}
