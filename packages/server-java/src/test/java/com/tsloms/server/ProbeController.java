// 仅测试用：异常出口与权限注解验证探针。
package com.tsloms.server;

import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.RequirePerm;
import jakarta.validation.Valid;
import jakarta.validation.constraints.NotBlank;
import java.util.Map;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class ProbeController {

    /** 未捕获异常探针（验证全局异常出口）。 */
    @GetMapping("/api/v1/_test/boom")
    public String boom() {
        throw new IllegalStateException("boom 测试异常");
    }

    /** 参数校验探针：@NotBlank 失败应返回 bad_request。 */
    public record ProbeDto(@NotBlank(message = "名称不能为空") String name) {
    }

    @PostMapping("/api/v1/_test/validate")
    public String validate(@Valid @RequestBody ProbeDto dto) {
        return "ok:" + dto.name();
    }

    /** 权限注解探针：要求 device:create（operator 内置拥有，viewer 无）。 */
    @GetMapping("/api/v1/_perm/device-create")
    @RequirePerm("device:create")
    public ApiResponse<Map<String, Object>> permProbe() {
        return ApiResponse.ok(Map.of("granted", true));
    }
}
