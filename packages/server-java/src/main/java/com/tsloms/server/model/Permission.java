// 功能权限点字典实体：对齐 Go 版 internal/model/rbac.go Permission（表 permissions）。
package com.tsloms.server.model;

import com.tsloms.server.model.BaseEntity;
import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Index;
import jakarta.persistence.Table;
import jakarta.persistence.UniqueConstraint;

/** code 形如 "device:create"（模块:动作），唯一标识一项功能权限。 */
@Entity
@Table(
        name = "permissions",
        uniqueConstraints = @UniqueConstraint(name = "uk_permissions_code", columnNames = "code"),
        indexes = @Index(name = "idx_permissions_module", columnList = "module"))
public class Permission extends BaseEntity {

    @Column(name = "code", nullable = false, length = 64)
    public String code;

    /** 权限名称（如：设备-新建）。 */
    @Column(name = "name", length = 64)
    public String name;

    /** 所属模块。 */
    @Column(name = "module", length = 32)
    public String module;

    @Column(name = "sort", nullable = false)
    public int sort;
}
