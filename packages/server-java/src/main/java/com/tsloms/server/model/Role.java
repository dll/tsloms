// 角色实体：对齐 Go 版 rbac.go Role（表 roles）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Table;
import jakarta.persistence.UniqueConstraint;

/** 内置角色 code: admin/operator/viewer；也支持自定义角色。 */
@Entity
@Table(
        name = "roles",
        uniqueConstraints = @UniqueConstraint(name = "uk_roles_code", columnNames = "code"))
public class Role extends BaseEntity {

    @Column(name = "code", nullable = false, length = 32)
    public String code;

    @Column(name = "name", length = 64)
    public String name;

    @Column(name = "builtin", nullable = false)
    public boolean builtin;

    @Column(name = "description", length = 255)
    public String description;
}
