// 认证接口：登录/注册/验证码/用户信息，契约对齐 Go 版 handler/auth.go。
package com.tsloms.server.auth;

import com.tsloms.server.model.Department;
import com.tsloms.server.model.PasswordHasher;
import com.tsloms.server.model.User;
import com.tsloms.server.model.UserRoles;
import com.tsloms.server.model.UserStatuses;
import com.tsloms.server.repository.DepartmentRepository;
import com.tsloms.server.repository.UserRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.AuthInterceptor;
import jakarta.validation.Valid;
import jakarta.validation.constraints.NotBlank;
import java.time.Instant;
import java.util.Map;
import java.util.regex.Pattern;
import org.springframework.http.ResponseEntity;
import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1")
@Validated
public class AuthController {

    /** 手机号格式（11 位，1 开头大陆手机号）——对齐 Go 版 phoneRegex。 */
    private static final Pattern PHONE_REGEX = Pattern.compile("^1[3-9]\\d{9}$");

    private final UserRepository users;
    private final DepartmentRepository departments;
    private final CaptchaService captcha;
    private final JwtService jwt;
    private final PasswordHasher hasher;
    private final ModuleService modules;

    public AuthController(UserRepository users, DepartmentRepository departments,
                          CaptchaService captcha, JwtService jwt, PasswordHasher hasher,
                          ModuleService modules) {
        this.users = users;
        this.departments = departments;
        this.captcha = captcha;
        this.jwt = jwt;
        this.hasher = hasher;
        this.modules = modules;
    }

    /** 登录请求体（对齐 Go 版 LoginRequest）。 */
    public record LoginRequest(
            @NotBlank String username,
            @NotBlank String password,
            String captchaUuid,
            String captchaCode) {
    }

    /** GET /api/v1/auth/captcha —— 算术验证码（防暴力破解）。 */
    @GetMapping("/auth/captcha")
    public ApiResponse<CaptchaService.Captcha> captcha() {
        return ApiResponse.ok(captcha.generate());
    }

    /** POST /api/v1/auth/login —— 登录（用户名或手机号 + 密码 + 算术验证码）。 */
    @PostMapping("/auth/login")
    public ResponseEntity<?> login(@Valid @RequestBody LoginRequest req) {
        if (!captcha.verify(req.captchaUuid(), req.captchaCode())) {
            return unauthorized("算术验证码错误或已过期");
        }

        User user = findUserByLogin(req.username());
        if (user == null || !hasher.matches(req.password(), user.passwordHash)) {
            return unauthorized("用户名或密码错误");
        }
        if (user.status != null && UserStatuses.DISABLED.equals(user.status)) {
            return unauthorized("账号已停用，请联系管理员");
        }

        // 更新最后登录时间
        user.lastLoginAt = Instant.now();
        users.save(user);

        Map<String, Object> data = Map.of(
                "token", jwt.issue(user.id, user.role),
                "user", UserPayload.of(user),
                "enabled_modules", modules.enabledModuleList());
        return ResponseEntity.ok(ApiResponse.ok(data));
    }

    /** 注册请求体（对齐 Go 版 RegisterRequest）。 */
    public record RegisterRequest(
            @NotBlank String username,
            @NotBlank String password,
            String realName,
            String phone,
            Long departmentId,
            String captchaUuid,
            String captchaCode) {
    }

    /** POST /api/v1/auth/register —— 自助注册：默认 viewer 只读角色。 */
    @PostMapping("/auth/register")
    public ResponseEntity<?> register(@Valid @RequestBody RegisterRequest req) {
        String uname = req.username().trim();
        if (req.password().length() < 6) {
            return badRequest("密码长度至少 6 位");
        }
        String realName = (req.realName() == null || req.realName().isBlank())
                ? uname : req.realName().trim();
        if (!captcha.verify(req.captchaUuid(), req.captchaCode())) {
            return unauthorized("算术验证码错误或已过期");
        }

        // 归属部门（若填）需存在
        Long deptId = req.departmentId();
        if (deptId != null && deptId > 0) {
            if (departments.findById(deptId).isEmpty()) {
                return badRequest("归属部门不存在");
            }
        } else {
            deptId = null;
        }

        // 手机号格式（若填）
        String phone = req.phone() == null ? "" : req.phone().trim();
        if (!phone.isEmpty() && !PHONE_REGEX.matcher(phone).matches()) {
            return badRequest("手机号格式不正确");
        }

        // 用户名唯一
        if (users.existsByUsername(uname)) {
            return badRequest("用户名已存在");
        }
        // 手机号登录账号唯一（非空时）
        if (!phone.isEmpty() && users.findByPhoneLogin(phone).isPresent()) {
            return badRequest("该手机号已注册");
        }

        User u = new User();
        u.username = uname;
        u.phoneLogin = phone;
        u.phone = phone;
        u.realName = realName;
        u.departmentId = deptId;
        u.role = UserRoles.VIEWER;
        u.status = UserStatuses.ENABLED;
        u.passwordHash = hasher.hash(req.password());
        users.save(u);

        return ResponseEntity.ok(ApiResponse.ok(Map.of(
                "message", "注册成功，请登录",
                "user", UserPayload.of(u))));
    }

    /** GET /api/v1/user/info —— 当前登录用户信息。 */
    @GetMapping("/user/info")
    public ApiResponse<Map<String, Object>> userInfo(jakarta.servlet.http.HttpServletRequest request) {
        Long userId = AuthInterceptor.userId(request);
        User user = users.findById(userId).orElseThrow();
        return ApiResponse.ok(Map.of(
                "user", UserPayload.of(user),
                "enabled_modules", modules.enabledModuleList()));
    }

    /** 按手机号登录账号或用户名定位用户（对齐 Go 版 findUserByLogin）。 */
    private User findUserByLogin(String login) {
        if (login == null || login.isEmpty()) {
            return null;
        }
        return users.findByPhoneLogin(login).or(() -> users.findByUsername(login)).orElse(null);
    }

    private ResponseEntity<?> unauthorized(String msg) {
        return ResponseEntity.status(401).body(ApiResponse.fail("unauthorized", msg));
    }

    private ResponseEntity<?> badRequest(String msg) {
        return ResponseEntity.badRequest().body(ApiResponse.fail("bad_request", msg));
    }
}
