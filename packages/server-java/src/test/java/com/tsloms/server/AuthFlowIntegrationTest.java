// 登录全链路集成测试：验证码→登录→token→userinfo→鉴权/403/401 路径。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
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
class AuthFlowIntegrationTest {

    @Autowired MockMvc mvc;
    @Autowired CaptchaService captcha;
    @Autowired UserRepository users;
    @Autowired PasswordHasher hasher;
    @Autowired TransactionTemplate tx;

    /** 事务模板外直写用户（绕过测试事务回滚，保证 HTTP 线程可见）。 */
    private Long seedUser(String username, String role, String phoneLogin) {
        return tx.execute(s -> {
            User u = new User();
            u.username = username;
            u.phoneLogin = phoneLogin;
            u.passwordHash = hasher.hash("Passw0rd!");
            u.role = role;
            u.status = "enabled";
            users.save(u);
            return u.id;
        });
    }

    private String loginBody(String login, String password) {
        var cap = captcha.generate();
        int answer = captcha.peekAnswerForTest(cap.uuid());
        return "{\"username\":\"" + login + "\",\"password\":\"" + password
                + "\",\"captcha_uuid\":\"" + cap.uuid()
                + "\",\"captcha_code\":\"" + answer + "\"}";
    }

    @Test
    void 登录成功_返回token用户与模块列表() throws Exception {
        seedUser("op_login_1", UserRoles.OPERATOR, null);
        mvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(loginBody("op_login_1", "Passw0rd!")))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.data.token").isNotEmpty())
                .andExpect(jsonPath("$.data.user.username").value("op_login_1"))
                .andExpect(jsonPath("$.data.user.role").value(UserRoles.OPERATOR))
                .andExpect(jsonPath("$.data.enabled_modules").isArray())
                .andExpect(jsonPath("$.data.enabled_modules[0]").value("dashboard"));
    }

    @Test
    void 手机号可作为登录账号() throws Exception {
        seedUser("phone_user_1", "viewer", "13900001111");
        mvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(loginBody("13900001111", "Passw0rd!")))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0));
    }

    @Test
    void 密码错误_401统一信封() throws Exception {
        seedUser("op_badpw", "operator", null);
        mvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(loginBody("op_badpw", "WrongPass")))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value(-1))
                .andExpect(jsonPath("$.error").value("unauthorized"))
                .andExpect(jsonPath("$.msg").value("用户名或密码错误"));
    }

    @Test
    void 验证码错误_401() throws Exception {
        var cap = captcha.generate();
        String body = "{\"username\":\"anyone\",\"password\":\"x\",\"captcha_uuid\":\""
                + cap.uuid() + "\",\"captcha_code\":\"9999\"}";
        mvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON).content(body))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.msg").value("算术验证码错误或已过期"));
    }

    @Test
    void 无token访问受保护接口_401裸形状() throws Exception {
        mvc.perform(get("/api/v1/user/info"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.error").value("unauthorized"))
                .andExpect(jsonPath("$.msg").doesNotExist());
    }

    @Test
    void 合法token访问userinfo_返回当前用户() throws Exception {
        Long id = seedUser("op_info_1", UserRoles.OPERATOR, null);
        String token = loginAndGetToken("op_info_1");
        assertThat(token).isNotBlank();
        mvc.perform(get("/api/v1/user/info").header("Authorization", "Bearer " + token))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.user.username").value("op_info_1"))
                .andExpect(jsonPath("$.data.user.id").value(id));
    }

    @Test
    void 停用用户既有token_401() throws Exception {
        Long id = seedUser("op_disabled", "operator", null);
        String token = loginAndGetToken("op_disabled");
        tx.executeWithoutResult(s ->
                users.findById(id).ifPresent(u -> {
                    u.status = "disabled";
                    users.save(u);
                }));
        mvc.perform(get("/api/v1/user/info").header("Authorization", "Bearer " + token))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.error").value("user disabled"))
                .andExpect(jsonPath("$.message").value("账号已停用，请联系管理员"));
    }

    @Test
    void RequirePerm注解_无权限403有权限通过() throws Exception {
        // viewer 无 device:create 权限 → 403
        seedUser("viewer_perm", "viewer", null);
        String token = loginAndGetToken("viewer_perm");
        mvc.perform(get("/api/v1/_perm/device-create").header("Authorization", "Bearer " + token))
                .andExpect(status().isForbidden())
                .andExpect(jsonPath("$.error").value("forbidden"))
                .andExpect(jsonPath("$.message").value("无此功能权限: device:create"));

        // operator 拥有 device:create（内置映射）→ 通过
        seedUser("operator_perm", "operator", null);
        String token2 = loginAndGetToken("operator_perm");
        mvc.perform(get("/api/v1/_perm/device-create").header("Authorization", "Bearer " + token2))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.granted").value(true));
    }

    private String loginAndGetToken(String username) throws Exception {
        String res = mvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(loginBody(username, "Passw0rd!")))
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        assertThat(res).contains("\"token\":\"");
        int i = res.indexOf("\"token\":\"") + "\"token\":\"".length();
        return res.substring(i, res.indexOf('"', i));
    }
}
