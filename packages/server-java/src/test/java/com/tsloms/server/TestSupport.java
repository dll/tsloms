// 测试公共支撑：登录请求体构造等。
package com.tsloms.server;

import com.tsloms.server.auth.CaptchaService;
import com.tsloms.server.model.Role;
import com.tsloms.server.repository.RoleRepository;
import java.util.Optional;

public final class TestSupport {

    /** 生成验证码并组装登录请求体（答案从服务侧窥探，仅供测试）。 */
    public static String login(CaptchaService captcha, String username, String password) {
        var cap = captcha.generate();
        int answer = captcha.peekAnswerForTest(cap.uuid());
        return "{\"username\":\"" + username + "\",\"password\":\"" + password
                + "\",\"captcha_uuid\":\"" + cap.uuid()
                + "\",\"captcha_code\":\"" + answer + "\"}";
    }

    /** 按角色编码查 ID（测试辅助）。 */
    public static Long roleIdByCode(RoleRepository roles, String code) {
        Optional<Role> r = roles.findByCode(code);
        return r.map(v -> v.id).orElseThrow();
    }

    private TestSupport() {
    }
}
