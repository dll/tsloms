// 部门/组织实体：字段逐列对齐 Go 版 internal/model/user.go Department（表 departments）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Table;
import jakarta.persistence.UniqueConstraint;

@Entity
@Table(
        name = "departments",
        uniqueConstraints = @UniqueConstraint(name = "uk_departments_name", columnNames = "name"))
public class Department extends BaseEntity {

    @Column(name = "name", nullable = false, length = 64)
    public String name;

    /** 上级部门 ID（空为顶级；GORM *uint 同义）。 */
    @Column(name = "parent_id")
    public Long parentId;

    @Column(name = "leader", length = 64)
    public String leader;

    @Column(name = "description", length = 255)
    public String description;
}
