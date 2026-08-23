// 部门与 RBAC 管理接口集成测试。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.tsloms.server.auth.CaptchaService;
import com.tsloms.server.model.Department;
import com.tsloms.server.model.PasswordHasher;
import com.tsloms.server.model.User;
import com.tsloms.server.model.UserRoles;
import com.tsloms.server.repository.DepartmentRepository;
import com.tsloms.server.repository.UserRepository;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.transaction.support.TransactionTemplate;

@SpringBootTest
@AutoConfigureMockMvc
class DepartmentRbacAdminControllerTest {

    @Autowired MockMvc mvc;
    @Autowired CaptchaService captchaSvc;
    @Autowired UserRepository users;
    @Autowired DepartmentRepository departments;
    @Autowired PasswordHasher hasher;
    @Autowired TransactionTemplate tx;

    private String loginAs(String username, String password) throws Exception {
        String res = mvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(TestSupport.login(captchaSvc, username, password)))
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        int i = res.indexOf("\"token\":\"") + "\"token\":\"".length();
        return res.substring(i, res.indexOf('"', i));
    }

    private String adminToken() throws Exception {
        tx.executeWithoutResult(s -> {
            if (users.findByUsername("dept_admin").isEmpty()) {
                User u = new User();
                u.username = "dept_admin";
                u.passwordHash = hasher.hash("Passw0rd!x");
                u.role = UserRoles.ADMIN;
                u.status = "enabled";
                users.save(u);
            }
        });
        return loginAs("dept_admin", "Passw0rd!x");
    }

    // ==================== 部门 ====================

    @Test
    void 部门创建_重名拒绝_上级校验() throws Exception {
        String token = adminToken();
        String bearer = "Bearer " + token;

        mvc.perform(post("/api/v1/departments").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"name\":\"运维中心\",\"leader\":\"张三\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.name").value("运维中心"));

        mvc.perform(post("/api/v1/departments").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"name\":\"运维中心\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("部门名称已存在"));

        mvc.perform(post("/api/v1/departments").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"name\":\"子部门\",\"parent_id\":999999}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("上级部门不存在"));
    }

    @Test
    void 部门列表_含成员数_更新_删除保护() throws Exception {
        Long deptId = tx.execute(s -> {
            Department d = new Department();
            d.name = "有人的部门";
            departments.save(d);
            User u = new User();
            u.username = "dept_member";
            u.passwordHash = hasher.hash("Passw0rd!");
            u.role = "viewer";
            u.departmentId = d.id;
            users.save(u);
            return d.id;
        });
        String bearer = "Bearer " + adminToken();

        mvc.perform(get("/api/v1/departments").header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.list[?(@.name=='有人的部门')].member_count").value(1));

        // 有成员不可删
        mvc.perform(delete("/api/v1/departments/" + deptId).header("Authorization", bearer))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("该部门下仍有用户，无法删除"));

        // 更新：上级不能是自身
        mvc.perform(put("/api/v1/departments/" + deptId).header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"parent_id\":" + deptId + "}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("上级部门不能是自身"));

        // 正常更新 leader
        mvc.perform(put("/api/v1/departments/" + deptId).header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"leader\":\"李四\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("部门更新成功"));
    }

    // ==================== RBAC 管理 ====================

    @Test
    void 权限字典_按模块分组且种子齐全() throws Exception {
        String bearer = "Bearer " + adminToken();
        mvc.perform(get("/api/v1/rbac/permissions").header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.total").value(41))
                .andExpect(
                        jsonPath("$.data.list[?(@.module=='device')].permissions").exists());
    }

    @Test
    void 角色CRUD全流程() throws Exception {
        String bearer = "Bearer " + adminToken();

        // 创建（带权限关联）
        String res = mvc.perform(post("/api/v1/rbac/roles").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"code":"maintainer","name":"维护员",
                                 "permissions":["device:create","media:upload"]}"""))
                .andExpect(status().isOk())
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        try {
            Long roleId = new com.fasterxml.jackson.databind.ObjectMapper()
                    .readTree(res).at("/data/id").asLong();
            assertThat(roleId).isPositive();

            // 与内置角色同名 → 拒绝；重复 code → 拒绝
            mvc.perform(post("/api/v1/rbac/roles").header("Authorization", bearer)
                            .contentType(MediaType.APPLICATION_JSON)
                            .content("{\"code\":\"admin\",\"name\":\"x\"}"))
                    .andExpect(status().isBadRequest())
                    .andExpect(jsonPath("$.msg").value("不能创建与内置角色同名的角色"));
            mvc.perform(post("/api/v1/rbac/roles").header("Authorization", bearer)
                            .contentType(MediaType.APPLICATION_JSON)
                            .content("{\"code\":\"maintainer\",\"name\":\"x\"}"))
                    .andExpect(status().isBadRequest())
                    .andExpect(jsonPath("$.msg").value("角色编码已存在"));

            // 列表包含新角色及其权限码
            mvc.perform(get("/api/v1/rbac/roles").header("Authorization", bearer))
                    .andExpect(status().isOk())
                    .andExpect(jsonPath(
                            "$.data.list[?(@.code=='maintainer')].permissions").exists());

            // 内置角色不可编辑/删除
            Long builtinId = com.tsloms.server.TestSupport.roleIdByCode(roles(), "operator");
            mvc.perform(put("/api/v1/rbac/roles/" + builtinId).header("Authorization", bearer)
                            .contentType(MediaType.APPLICATION_JSON).content("{\"name\":\"x\"}"))
                    .andExpect(status().isBadRequest())
                    .andExpect(jsonPath("$.msg").value("内置角色不可编辑，可通过用户级权限覆写调整"));
            mvc.perform(delete("/api/v1/rbac/roles/" + builtinId).header("Authorization", bearer))
                    .andExpect(status().isBadRequest());

            // 自定义角色删除成功
            mvc.perform(delete("/api/v1/rbac/roles/" + roleId).header("Authorization", bearer))
                    .andExpect(status().isOk())
                    .andExpect(jsonPath("$.data.message").value("角色删除成功"));
        } catch (com.fasterxml.jackson.core.JsonProcessingException e) {
            throw new IllegalStateException("响应解析失败: " + res, e);
        }
    }

    private com.tsloms.server.repository.RoleRepository roles() {
        return roleRepo;
    }

    @Autowired com.tsloms.server.repository.RoleRepository roleRepo;

    @Test
    void 用户权限覆写_回显与设置语义() throws Exception {
        Long uid = tx.execute(s -> {
            User u = new User();
            u.username = "perm_target";
            u.passwordHash = hasher.hash("Passw0rd!");
            u.role = "viewer";
            users.save(u);
            return u.id;
        });
        String bearer = "Bearer " + adminToken();

        // 显式授权 → 有效权限为授权集合
        mvc.perform(put("/api/v1/rbac/users/" + uid + "/permissions")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"grants\":[\"fault:update\",\"device:create\"]}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("功能权限已更新"));
        mvc.perform(get("/api/v1/rbac/users/" + uid + "/permissions")
                        .header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.user_grants.length()").value(2));

        // 无 grants 仅 denies → 从默认剔除（viewer 默认空，denies 生效但无效果）
        mvc.perform(put("/api/v1/rbac/users/" + uid + "/permissions")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"denies\":[\"fault:update\"]}"))
                .andExpect(status().isOk());
        mvc.perform(get("/api/v1/rbac/users/" + uid + "/permissions")
                        .header("Authorization", bearer))
                .andExpect(jsonPath("$.data.user_denies.length()").value(1));

        // my permissions：viewer 被显式授权后有效权限=授权集
        String targetToken = loginAs("perm_target", "Passw0rd!");
        mvc.perform(get("/api/v1/my/permissions").header("Authorization", "Bearer " + targetToken))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.role").value("viewer"));
    }
}
