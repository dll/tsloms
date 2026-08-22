// 用户实体：字段逐列对齐 Go 版 internal/model/user.go（表 users）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Index;
import jakarta.persistence.Table;
import jakarta.persistence.UniqueConstraint;
import java.time.Instant;

/**
 * 用户表。
 *
 * <p>role: admin/operator/viewer；status: enabled/disabled（停用后不可登录）。
 * 人事核心字段：工号/头像/性别/身份证号/住址/文化程度/工程等级。
 */
@Entity
@Table(
        name = "users",
        uniqueConstraints = @UniqueConstraint(name = "uk_users_username", columnNames = "username"),
        indexes = {
                @Index(name = "idx_users_phone_login", columnList = "phone_login"),
                @Index(name = "idx_users_work_no", columnList = "work_no"),
                @Index(name = "idx_users_id_card", columnList = "id_card"),
        })
public class User extends BaseEntity {

    @Column(name = "username", nullable = false, length = 64)
    public String username;

    /** 手机号登录账号（可空；绑定后应用层校验唯一，只增不删——对齐 Go 版注释约束）。 */
    @Column(name = "phone_login", length = 20)
    public String phoneLogin;

    @Column(name = "phone_verified", nullable = false)
    public boolean phoneVerified;

    /** 密码哈希（bcrypt）。 */
    @Column(name = "password_hash", nullable = false, length = 255)
    public String passwordHash;

    @Column(name = "role", nullable = false, length = 16)
    public String role = UserRoles.VIEWER;

    @Column(name = "real_name", length = 64)
    public String realName;

    @Column(name = "phone", length = 20)
    public String phone;

    @Column(name = "email", length = 64)
    public String email;

    /** 所属部门 ID（可空；GORM *uint 同义）。 */
    @Column(name = "department_id")
    public Long departmentId;

    @Column(name = "status", nullable = false, length = 16)
    public String status = UserStatuses.ENABLED;

    @Column(name = "last_login_at")
    public Instant lastLoginAt;

    /** 地图中心纬度（该用户管辖区域，可空）。 */
    @Column(name = "center_lat")
    public Double centerLat;

    @Column(name = "center_lng")
    public Double centerLng;

    @Column(name = "remark", length = 255)
    public String remark;

    // ---- 人事核心字段 ----
    @Column(name = "work_no", length = 64)
    public String workNo;

    /** 工作照/头像（上传图片 URL）。 */
    @Column(name = "avatar", length = 255)
    public String avatar;

    @Column(name = "gender", length = 8)
    public String gender;

    @Column(name = "id_card", length = 32)
    public String idCard;

    @Column(name = "address", length = 255)
    public String address;

    @Column(name = "education", length = 32)
    public String education;

    /** 工程等级（岗位/技能等级）。 */
    @Column(name = "engineer_level", length = 32)
    public String engineerLevel;
}
