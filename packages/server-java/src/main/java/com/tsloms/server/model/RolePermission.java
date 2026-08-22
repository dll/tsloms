// 角色-功能权限关联实体：对齐 Go 版 rbac.go RolePermission（表 role_permissions）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Index;
import jakarta.persistence.Table;

@Entity
@Table(
        name = "role_permissions",
        indexes = {
                @Index(name = "idx_rp_role_id", columnList = "role_id"),
                @Index(name = "idx_rp_permission_id", columnList = "permission_id"),
                @Index(name = "idx_rp_role_code", columnList = "role_code"),
        })
public class RolePermission {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "role_id", nullable = false)
    public Long roleId;

    @Column(name = "permission_id", nullable = false)
    public Long permissionId;

    /** 冗余角色编码，便于批量查询（对齐 Go 版注释）。 */
    @Column(name = "role_code", nullable = false, length = 32)
    public String roleCode;
}
