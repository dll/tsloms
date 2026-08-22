// 用户级功能权限覆写实体：对齐 Go 版 rbac.go UserPermission（表 user_permissions）。
//
// 语义：若用户存在本表记录——
//   - 有任一条 granted=true：以授权集合【全量覆盖】角色默认；
//   - 仅 granted=false（显式拒绝）：从角色默认中剔除对应项。
// 若无任何记录则完全继承角色默认。
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
        name = "user_permissions",
        indexes = {
                @Index(name = "idx_up_user_id", columnList = "user_id"),
                @Index(name = "idx_up_permission", columnList = "permission"),
        })
public class UserPermission {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "user_id", nullable = false)
    public Long userId;

    @Column(name = "permission", nullable = false, length = 64)
    public String permission;

    /** true=授权 / false=显式拒绝。 */
    @Column(name = "granted", nullable = false)
    public boolean granted;
}
