// 区划实体：对齐 Go 版 Area（表 areas），parent_id 递归构建同层树。
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
@Table(name = "areas", indexes = {
        @Index(name = "idx_areas_name", columnList = "name"),
        @Index(name = "idx_areas_parent", columnList = "parent_id"),
        @Index(name = "idx_areas_type", columnList = "area_type"),
})
public class Area {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    /** 区划编码（如 340103）。 */
    @Column(name = "code", length = 32)
    public String code;

    @Column(name = "name", nullable = false, length = 64)
    public String name;

    /** 上级区划 ID（空=顶级省）。 */
    @Column(name = "parent_id")
    public Long parentId;

    /** province/city/district/street/community/road。 */
    @Column(name = "area_type", nullable = false, length = 16)
    public String areaType;

    /** 全称（省市区街道…拼接）。 */
    @Column(name = "full_name", length = 255)
    public String fullName;

    @Column(name = "area_sort", nullable = false)
    public Integer areaSort = 0;

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
