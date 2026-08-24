// 授权状态/多源证据/案例库实体（对齐 Go 版 license.go/fault_evidence.go/fault_case.go）。
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

/** 授权状态持久化单例（id=1）。 */
@Entity
@Table(name = "license_state")
public class LicenseState {

    @Id
    public Long id;

    @Column(name = "core_activated_at")
    public Instant coreActivatedAt;

    @Column(name = "core_unlocked", nullable = false)
    public boolean coreUnlocked;

    /** 可选模块授权状态 JSON。 */
    @Column(name = "module_json", columnDefinition = "text")
    public String moduleJson;

    @Column(name = "last_check_time")
    public Instant lastCheckTime;

    @Column(name = "updated_at", nullable = false)
    public Instant updatedAt;

    @PrePersist
    void onCreate() {
        if (updatedAt == null) {
            updatedAt = Instant.now();
        }
    }

    @PreUpdate
    void onUpdate() {
        updatedAt = Instant.now();
    }
}
