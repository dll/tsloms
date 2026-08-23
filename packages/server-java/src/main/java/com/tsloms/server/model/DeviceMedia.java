// 设备媒体实体：逐列对齐 Go 版 internal/model/media.go DeviceMedia（表 device_media）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Index;
import jakarta.persistence.PrePersist;
import jakarta.persistence.Table;
import java.time.Instant;

/** 记录手机举证视频/图片与路口监控流。 */
@Entity
@Table(name = "device_media", indexes = {
        @Index(name = "idx_media_device_hw", columnList = "device_hw_id"),
})
public class DeviceMedia {

    /** 媒体类型常量。 */
    public static final String TYPE_EVIDENCE = "evidence";
    public static final String TYPE_MONITORING = "monitoring";
    public static final String TYPE_TIMELAPSE = "timelapse";
    public static final String CATEGORY_PHOTO = "photo";
    public static final String CATEGORY_VIDEO = "video";
    public static final String CATEGORY_DOC = "doc";
    public static final String SOURCE_UPLOAD = "upload";
    public static final String SOURCE_RTSP = "rtsp";
    public static final String SOURCE_URL = "url";

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "device_hw_id", nullable = false, length = 64)
    public String deviceHwId;

    /** media_type：evidence/monitoring/timelapse。 */
    @Column(name = "media_type", length = 16)
    public String mediaType;

    /** photo/video/doc。 */
    @Column(name = "category", length = 32)
    public String category;

    @Column(name = "title", length = 128)
    public String title;

    /** upload/rtsp/url。 */
    @Column(name = "source", length = 16)
    public String source;

    @Column(name = "url", length = 512)
    public String url;

    @Column(name = "compatible_url", length = 512)
    public String compatibleUrl;

    @Column(name = "thumbnail", length = 512)
    public String thumbnail;

    /** 时长（秒，视频）。 */
    @Column(name = "duration", nullable = false)
    public Integer duration;

    @Column(name = "note", columnDefinition = "text")
    public String note;

    @Column(name = "uploaded_by", length = 64)
    public String uploadedBy;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    // ---- 信号灯信息（举证上传必填路口，便于定位派单）----
    @Column(name = "intersection", length = 128)
    public String intersection;

    /** 灯色：red/yellow/green/unknown。 */
    @Column(name = "light_color", length = 16)
    public String lightColor;

    @Column(name = "fault_desc", length = 255)
    public String faultDesc;

    @Column(name = "is_active_fault", nullable = false)
    public boolean isActiveFault;

    @PrePersist
    void onCreate() {
        if (createdAt == null) {
            createdAt = Instant.now();
        }
        if (duration == null) {
            duration = 0;
        }
    }
}
