// 仅测试用：异常出口验证探针（未捕获异常 / 参数校验失败）。
package com.tsloms.server;

import jakarta.validation.Valid;
import jakarta.validation.constraints.NotBlank;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class ProbeController {

    /** 校验请求体。 */
    public record ProbeDto(@NotBlank(message = "名称不能为空") String name) {
    }

    @GetMapping("/api/v1/_test/boom")
    public String boom() {
        throw new IllegalStateException("boom 测试异常");
    }

    @PostMapping("/api/v1/_test/validate")
    public String validate(@Valid @RequestBody ProbeDto dto) {
        return "ok:" + dto.name();
    }
}
