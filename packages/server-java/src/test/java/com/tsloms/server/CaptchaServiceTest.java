// 验证码服务测试：生成/答错重试/答对消费/过期失效。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;

import com.tsloms.server.auth.CaptchaService;
import org.junit.jupiter.api.Test;

class CaptchaServiceTest {

    @Test
    void 生成返回题目与uuid_答案可验证通过() {
        CaptchaService svc = new CaptchaService();
        var cap = svc.generate();
        assertThat(cap.uuid()).isNotBlank();
        assertThat(cap.question()).endsWith("= ?");
        int answer = svc.peekAnswerForTest(cap.uuid());
        assertThat(answer).isNotEqualTo(Integer.MIN_VALUE);
        assertThat(svc.verify(cap.uuid(), String.valueOf(answer))).isTrue();
        // 一次性：答对后销毁
        assertThat(svc.verify(cap.uuid(), String.valueOf(answer))).isFalse();
    }

    @Test
    void 答错不消费_可重试直至过期() {
        CaptchaService svc = new CaptchaService();
        var cap = svc.generate();
        int answer = svc.peekAnswerForTest(cap.uuid());
        assertThat(svc.verify(cap.uuid(), String.valueOf(answer + 100))).isFalse();
        assertThat(svc.verify(cap.uuid(), "abc")).isFalse();
        // 未被消费，仍可答对
        assertThat(svc.verify(cap.uuid(), String.valueOf(answer))).isTrue();
    }

    @Test
    void 过期验证码_不可用() {
        CaptchaService svc = new CaptchaService();
        svc.putExpiredForTest("expired-uuid", 7);
        assertThat(svc.verify("expired-uuid", "7")).isFalse();
        assertThat(svc.verify("no-such-uuid", "7")).isFalse();
        assertThat(svc.verify(null, "7")).isFalse();
        assertThat(svc.verify("x", "")).isFalse();
    }
}
