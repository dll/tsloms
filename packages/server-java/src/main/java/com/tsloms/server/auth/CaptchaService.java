// 算术验证码服务：内存存储 + 5 分钟 TTL，对齐 Go 版 handler/captcha.go。
// 规则：答错不消费（可重试直至过期）；答对即销毁（一次性）。
package com.tsloms.server.auth;

import java.security.SecureRandom;
import java.time.Instant;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import org.springframework.stereotype.Service;

@Service
public class CaptchaService {

    /** 验证码有效期：5 分钟（对齐 Go 版 captchaTTL）。 */
    private static final java.time.Duration TTL = java.time.Duration.ofMinutes(5);

    /** 一条验证码记录。 */
    record Entry(String question, int answer, Instant expiresAt) {
        boolean expired() {
            return Instant.now().isAfter(expiresAt);
        }
    }

    private final Map<String, Entry> store = new ConcurrentHashMap<>();
    private final SecureRandom random = new SecureRandom();

    /**
     * 生成验证码。
     *
     * @return uuid 与题目文本（如 "3 + 8 = ?"）
     */
    public Captcha generate() {
        int a = randomInt(20);
        int b = randomInt(20);
        boolean minus = random.nextBoolean();
        int answer = minus ? a - b : a + b;
        String question = (minus ? a + " - " + b : a + " + " + b) + " = ?";
        String uuid = UUID.randomUUID().toString();
        store.put(uuid, new Entry(question, answer, Instant.now().plus(TTL)));
        return new Captcha(uuid, question);
    }

    /**
     * 校验答案。过期/不存在返回 false；答对后立即销毁（防重放）。
     */
    public boolean verify(String uuid, String code) {
        if (uuid == null || code == null || code.isBlank()) {
            return false;
        }
        Entry entry = store.get(uuid);
        if (entry == null) {
            return false;
        }
        if (entry.expired()) {
            store.remove(uuid);
            return false;
        }
        try {
            if (Integer.parseInt(code.trim()) != entry.answer()) {
                return false;
            }
        } catch (NumberFormatException e) {
            return false;
        }
        store.remove(uuid);
        return true;
    }

    /** 测试辅助：读取某 uuid 的答案（生产代码勿用）。 */
    public int peekAnswerForTest(String uuid) {
        Entry e = store.get(uuid);
        return e == null ? Integer.MIN_VALUE : e.answer();
    }

    /** 测试辅助：直接注入一条已过期的验证码。 */
    public void putExpiredForTest(String uuid, int answer) {
        store.put(uuid, new Entry("0 + 0 = ?", answer, Instant.now().minusSeconds(1)));
    }

    private int randomInt(int max) {
        return random.nextInt(max + 1);
    }

    /** 对外暴露的验证码视图。 */
    public record Captcha(String uuid, String question) {
    }
}
