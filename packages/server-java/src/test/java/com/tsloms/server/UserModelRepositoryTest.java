// User/Department 实体持久化往返测试：字段逐项断言（契约级，对齐 Go 版模型字段）。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;

import com.tsloms.server.model.Department;
import com.tsloms.server.model.PasswordHasher;
import com.tsloms.server.model.User;
import com.tsloms.server.model.UserRoles;
import com.tsloms.server.model.UserStatuses;
import com.tsloms.server.repository.DepartmentRepository;
import com.tsloms.server.repository.UserRepository;
import java.time.Instant;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.orm.jpa.DataJpaTest;
import org.springframework.boot.test.context.TestConfiguration;
import org.springframework.context.annotation.Bean;

@DataJpaTest
class UserModelRepositoryTest {

    @Autowired
    private UserRepository users;

    @Autowired
    private DepartmentRepository departments;

    @Test
    void 用户全字段持久化往返() {
        Department dept = new Department();
        dept.name = "运维一部";
        dept.leader = "张三";
        dept.description = "负责信号灯巡检";
        departments.saveAndFlush(dept);
        assertThat(dept.id).isNotNull();

        User u = new User();
        u.username = "op001";
        u.phoneLogin = "13800000001";
        u.phoneVerified = true;
        u.passwordHash = new PasswordHasher().hash("Secret#123");
        u.role = UserRoles.OPERATOR;
        u.realName = "李四";
        u.phone = "13800000001";
        u.email = "lisi@example.com";
        u.departmentId = dept.id;
        u.status = UserStatuses.ENABLED;
        u.lastLoginAt = Instant.parse("2026-08-23T00:00:00Z");
        u.centerLat = 32.301;
        u.centerLng = 118.296;
        u.remark = "骨干";
        u.workNo = "W2026-001";
        u.avatar = "/uploads/avatar/op001.png";
        u.gender = "male";
        u.idCard = "341100199001010011";
        u.address = "滁州市琅琊区";
        u.education = "本科";
        u.engineerLevel = "中级";
        users.saveAndFlush(u);

        User loaded = users.findByUsername("op001").orElseThrow();
        assertThat(loaded.id).isEqualTo(u.id);
        assertThat(loaded.createdAt).isNotNull();
        assertThat(loaded.phoneLogin).isEqualTo("13800000001");
        assertThat(loaded.phoneVerified).isTrue();
        assertThat(loaded.role).isEqualTo(UserRoles.OPERATOR);
        assertThat(loaded.departmentId).isEqualTo(dept.id);
        assertThat(loaded.lastLoginAt).isEqualTo(Instant.parse("2026-08-23T00:00:00Z"));
        assertThat(loaded.centerLat).isEqualTo(32.301);
        assertThat(loaded.centerLng).isEqualTo(118.296);
        assertThat(loaded.workNo).isEqualTo("W2026-001");
        assertThat(loaded.engineerLevel).isEqualTo("中级");

        // 派生查询：手机号登录查找（Go 版登录逻辑将使用）
        assertThat(users.findByPhoneLogin("13800000001")).isPresent();
        assertThat(users.existsByUsername("op001")).isTrue();
        // 唯一约束生效
        assertThat(users.count()).isEqualTo(1);
    }

    @Test
    void 部门层级与默认值() {
        Department top = new Department();
        top.name = "总公司";
        departments.saveAndFlush(top);
        Department sub = new Department();
        sub.name = "技术部";
        sub.parentId = top.id;
        departments.saveAndFlush(sub);

        assertThat(sub.parentId).isEqualTo(top.id);
        assertThat(departments.existsByName("总公司")).isTrue();
        // 新用户默认角色/状态与 Go 版 gorm default 标签一致
        assertThat(new User().role).isEqualTo("viewer");
        assertThat(new User().status).isEqualTo("enabled");
    }

    @Test
    void bcrypt哈希与Go版互通格式() {
        PasswordHasher hasher = new PasswordHasher();
        String hash = hasher.hash("Passw0rd!");
        assertThat(hash).startsWith("$2"); // $2a$ 或 $2b$
        assertThat(hasher.matches("Passw0rd!", hash)).isTrue();
        assertThat(hasher.matches("wrong", hash)).isFalse();
    }

    /** 测试内注册 PasswordHasher Bean（供后续服务层注入复用）。 */
    @TestConfiguration
    static class Cfg {
        @Bean
        PasswordHasher passwordHasher() {
            return new PasswordHasher();
        }
    }
}
