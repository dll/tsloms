// 用户管理接口集成测试：CRUD 全链路（admin 登录 → 增删改查 + 校验分支）。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.tsloms.server.auth.CaptchaService;
import com.tsloms.server.model.PasswordHasher;
import com.tsloms.server.model.User;
import com.tsloms.server.model.UserRoles;
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
class UserAdminControllerTest {

    private static final String ADMIN_PW = "Passw0rd!x";

    @Autowired MockMvc mvc;
    @Autowired UserRepository users;
    @Autowired PasswordHasher hasher;
    @Autowired TransactionTemplate tx;
    @Autowired CaptchaService captchaSvc;

    /** 确保管理员账号存在并登录拿 token（admin 角色内置拥有 user:manage）。 */
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
            if (users.findByUsername("ua_admin").isEmpty()) {
                User u = new User();
                u.username = "ua_admin";
                u.passwordHash = hasher.hash(ADMIN_PW);
                u.role = UserRoles.ADMIN;
                u.status = "enabled";
                users.save(u);
            }
        });
        return loginAs("ua_admin", ADMIN_PW);
    }

    private void seedUser(String username, String role) {
        tx.executeWithoutResult(s -> {
            if (users.findByUsername(username).isPresent()) {
                return;
            }
            User u = new User();
            u.username = username;
            u.passwordHash = hasher.hash("Passw0rd!");
            u.role = role;
            u.status = "enabled";
            users.save(u);
        });
    }

    @Test
    void 创建用户_全字段成功() throws Exception {
        String token = adminToken();
        mvc.perform(post("/api/v1/users").header("Authorization", "Bearer " + token)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"username":"newop01","password":"Str0ngPass!","role":"operator",
                                 "real_name":"王五","phone":"13700002222","email":"w@example.com",
                                 "work_no":"W-100","gender":"male"}"""))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.data.username").value("newop01"))
                .andExpect(jsonPath("$.data.role").value("operator"));
        assertThat(users.findByUsername("newop01")).isPresent();
    }

    @Test
    void 创建用户_弱密码被拒() throws Exception {
        String token = adminToken();
        mvc.perform(post("/api/v1/users").header("Authorization", "Bearer " + token)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"username\":\"weakpw\",\"password\":\"123\",\"role\":\"viewer\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("密码至少10位"));
    }

    @Test
    void 创建用户_无效角色与重复用户名() throws Exception {
        String token = adminToken();
        mvc.perform(post("/api/v1/users").header("Authorization", "Bearer " + token)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"username\":\"badrole\",\"password\":\"Str0ngPass!\",\"role\":\"boss\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("无效的角色"));

        mvc.perform(post("/api/v1/users").header("Authorization", "Bearer " + token)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"username\":\"ua_admin\",\"password\":\"Str0ngPass!\",\"role\":\"viewer\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("用户名已存在"));
    }

    @Test
    void 用户列表_分页筛选且不含超管() throws Exception {
        seedUser("listop", "operator");
        seedUser("hidden_sa", UserRoles.SUPER_ADMIN);
        String token = adminToken();

        mvc.perform(get("/api/v1/users")
                        .header("Authorization", "Bearer " + token)
                        .param("keyword", "listop"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.data.page").value(1))
                .andExpect(jsonPath("$.data.list[?(@.username=='hidden_sa')]").isEmpty())
                .andExpect(jsonPath("$.data.list[?(@.username=='listop')]").exists());
    }

    @Test
    void 更新用户_局部字段生效() throws Exception {
        Long id = seedAndGet("upd_target", "operator");
        String token = adminToken();
        mvc.perform(put("/api/v1/users/" + id).header("Authorization", "Bearer " + token)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"real_name\":\"新名字\",\"status\":\"disabled\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("用户更新成功"));
        User u = users.findById(id).orElseThrow();
        assertThat(u.realName).isEqualTo("新名字");
        assertThat(u.status).isEqualTo("disabled");
        assertThat(u.role).isEqualTo("operator"); // 未提供不变
    }

    @Test
    void 更新用户_非法状态400() throws Exception {
        Long id = seedAndGet("upd_bad_status", "viewer");
        String token = adminToken();
        mvc.perform(put("/api/v1/users/" + id).header("Authorization", "Bearer " + token)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"status\":\"frozen\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("无效的用户状态"));
    }

    @Test
    void 重置密码_强度校验与生效() throws Exception {
        Long id = seedAndGet("resetpw", "viewer");
        String oldHash = users.findById(id).orElseThrow().passwordHash;
        String token = adminToken();

        mvc.perform(put("/api/v1/users/" + id + "/password")
                        .header("Authorization", "Bearer " + token)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"password\":\"short\"}"))
                .andExpect(status().isBadRequest());

        mvc.perform(put("/api/v1/users/" + id + "/password")
                        .header("Authorization", "Bearer " + token)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"password\":\"NewStr0ngPass\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("密码重置成功"));
        assertThat(users.findById(id).orElseThrow().passwordHash).isNotEqualTo(oldHash);
    }

    @Test
    void 删除用户_禁自删_内置admin保护() throws Exception {
        tx.executeWithoutResult(s -> {
            if (users.findByUsername("ua_admin").isEmpty()) {
                throw new IllegalStateException("admin 应已存在");
            }
        });
        Long selfId = users.findByUsername("ua_admin").orElseThrow().id;
        String token = adminToken();

        // 禁止删除自己
        mvc.perform(delete("/api/v1/users/" + selfId).header("Authorization", "Bearer " + token))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("不能删除当前登录用户"));

        // 内置 admin 不可删除
        Long builtinAdmin = seedAndGet("admin", "viewer");
        mvc.perform(delete("/api/v1/users/" + builtinAdmin).header("Authorization", "Bearer " + token))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("内置 admin 账号不可删除"));

        // 正常删除
        Long victim = seedAndGet("victim_user", "viewer");
        mvc.perform(delete("/api/v1/users/" + victim).header("Authorization", "Bearer " + token))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("用户删除成功"));
        assertThat(users.findById(victim)).isEmpty();
    }

    @Test
    void 可派单人员_仅管理员与运维() throws Exception {
        seedUser("asg_op", "operator");
        seedUser("asg_viewer", "viewer");
        String anyToken = adminToken();
        mvc.perform(get("/api/v1/users/assignable").header("Authorization", "Bearer " + anyToken))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.list[?(@.username=='asg_op')]").exists())
                .andExpect(jsonPath("$.data.list[?(@.username=='asg_viewer')]").isEmpty());
    }

    @Test
    void 无user_manage权限访问管理接口_403() throws Exception {
        seedUser("plain_viewer", "viewer");
        // viewer 密码为统一测试密码
        String viewerToken = loginAs("plain_viewer", "Passw0rd!");
        mvc.perform(get("/api/v1/users").header("Authorization", "Bearer " + viewerToken))
                .andExpect(status().isForbidden())
                .andExpect(jsonPath("$.error").value("forbidden"));
    }

    @Test
    void 自助接口_改手机号与地图中心() throws Exception {
        Long id = seedAndGet("self_user", "operator");
        String token = loginAs("self_user", "Passw0rd!");
        String bearer = "Bearer " + token;

        // 改手机号
        mvc.perform(put("/api/v1/user/phone").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"phone\":\"13600009999\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("手机号已更新"));
        assertThat(users.findById(id).orElseThrow().phone).isEqualTo("13600009999");

        // 空手机号 400
        mvc.perform(put("/api/v1/user/phone").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"phone\":\"\"}"))
                .andExpect(status().isBadRequest());

        // 设置合法中心点
        mvc.perform(put("/api/v1/user/center").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"lat\":32.3,\"lng\":118.3}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("地图中心点已更新"));
        User u = users.findById(id).orElseThrow();
        assertThat(u.centerLat).isEqualTo(32.3);
        assertThat(u.centerLng).isEqualTo(118.3);

        // 越界中心点 400
        mvc.perform(put("/api/v1/user/center").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"lat\":999,\"lng\":118.3}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("纬度范围 -90~90"));
    }

    private Long seedAndGet(String username, String role) {
        seedUser(username, role);
        return users.findByUsername(username).orElseThrow().id;
    }
}
