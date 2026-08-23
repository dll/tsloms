// 供应商实体：对齐 Go 版 Supplier（表 suppliers）。
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
@Table(name = "suppliers", indexes = @Index(name = "idx_suppliers_name", columnList = "name"))
public class Supplier {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "name", nullable = false, length = 64)
    public String name;

    @Column(name = "contact", length = 32)
    public String contact;

    @Column(name = "phone", length = 32)
    public String phone;

    @Column(name = "address", length = 255)
    public String address;

    @Column(name = "email", length = 64)
    public String email;

    @Column(name = "status", nullable = false, length = 16)
    public String status = "active";

    @Column(name = "note", columnDefinition = "text")
    public String note;

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
