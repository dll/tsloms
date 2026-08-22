// 模块开关实体：对齐 Go 版 model/module_toggle.go（表 module_toggles）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import jakarta.persistence.UniqueConstraint;
import java.time.Instant;
import org.hibernate.annotations.UpdateTimestamp;

/** 可选模块临时启用/停用开关（超级管理员维护，只影响可选模块；核心模块不可关闭）。 */
@Entity
@Table(
        name = "module_toggles",
        uniqueConstraints = @UniqueConstraint(name = "uk_module_toggles_key", columnNames = "module_key"))
public class ModuleToggle {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "module_key", nullable = false, length = 32)
    public String moduleKey;

    @Column(name = "enabled", nullable = false)
    public boolean enabled;

    @UpdateTimestamp
    @Column(name = "updated_at", nullable = false)
    public Instant updatedAt;
}
