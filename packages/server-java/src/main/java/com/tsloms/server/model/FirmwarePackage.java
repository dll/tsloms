// 固件包实体：逐列对齐 Go 版 FirmwarePackage（表 firmware_packages）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.PrePersist;
import jakarta.persistence.PreUpdate;
import jakarta.persistence.Table;
import jakarta.persistence.UniqueConstraint;
import java.time.Instant;

/** 记录信号灯设备 OTA 升级用的固件版本与文件存储。 */
@Entity
@Table(name = "firmware_packages",
        uniqueConstraints = @UniqueConstraint(name = "uk_fw_version", columnNames = "version"))
public class FirmwarePackage {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    /** 固件版本号（如 v1.2.0）。 */
    @Column(name = "version", nullable = false, length = 32)
    public String version;

    // uint32 → Long
    @Column(name = "major", nullable = false)
    public Long major;

    @Column(name = "minor", nullable = false)
    public Long minor;

    @Column(name = "build", nullable = false)
    public Long build;

    /** 对应设备固件位域值 major<<28|minor<<24。 */
    @Column(name = "sw_version", nullable = false)
    public Long swVersion;

    @Column(name = "file_name", length = 128)
    public String fileName;

    @Column(name = "file_path", length = 255)
    public String filePath;

    /** 文件大小（字节）。 */
    @Column(name = "size", nullable = false)
    public Long size;

    @Column(name = "md5", length = 32)
    public String md5;

    @Column(name = "description", length = 500)
    public String description;

    @Column(name = "published", nullable = false)
    public boolean published;

    @Column(name = "published_at")
    public Instant publishedAt;

    @Column(name = "uploader", length = 64)
    public String uploader;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    @Column(name = "updated_at", nullable = false)
    public Instant updatedAt;

    @PrePersist
    void onCreate() {
        if (createdAt == null) {
            createdAt = Instant.now();
        }
        if (updatedAt == null) {
            updatedAt = Instant.now();
        }
    }

    @PreUpdate
    void onUpdate() {
        updatedAt = Instant.now();
    }
}
