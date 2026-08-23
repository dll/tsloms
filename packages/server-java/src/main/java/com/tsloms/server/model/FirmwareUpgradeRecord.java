// 设备固件升级记录实体：对齐 Go 版 FirmwareUpgradeRecord（表 firmware_upgrade_records）。
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
@Table(name = "firmware_upgrade_records", indexes = {
        @Index(name = "idx_fur_firmware_id", columnList = "firmware_id"),
        @Index(name = "idx_fur_device_hw", columnList = "device_hw_id"),
})
public class FirmwareUpgradeRecord {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "firmware_id", nullable = false)
    public Long firmwareId;

    @Column(name = "device_hw_id", nullable = false, length = 64)
    public String deviceHwId;

    @Column(name = "target_version", length = 32)
    public String targetVersion;

    /** pending/upgrading/success/failed。 */
    @Column(name = "status", nullable = false, length = 24)
    public String status = "pending";

    @Column(name = "error_msg", length = 500)
    public String errorMsg;

    @Column(name = "started_at")
    public Instant startedAt;

    @Column(name = "finished_at")
    public Instant finishedAt;

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
