// 统一信封与异常出口的单元测试。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.tsloms.server.web.ApiResponse;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;

@SpringBootTest
@AutoConfigureMockMvc
class ApiResponseAndErrorTest {

    @Autowired
    private MockMvc mvc;

    @Test
    void ok信封字段形状() {
        ApiResponse<String> resp = ApiResponse.ok("x");
        assertThat(resp.code()).isZero();
        assertThat(resp.msg()).isEqualTo("success");
        assertThat(resp.data()).isEqualTo("x");
        assertThat(resp.error()).isNull();
    }

    @Test
    void fail信封字段形状() {
        ApiResponse<Void> resp = ApiResponse.fail("bad_request", "参数错误");
        assertThat(resp.code()).isEqualTo(-1);
        assertThat(resp.error()).isEqualTo("bad_request");
        assertThat(resp.msg()).isEqualTo("参数错误");
        assertThat(resp.data()).isNull();
    }

    @Test
    void 未捕获异常_非生产回显信息() throws Exception {
        mvc.perform(get("/api/v1/_test/boom"))
                .andExpect(status().isInternalServerError())
                .andExpect(jsonPath("$.code").value(-1))
                .andExpect(jsonPath("$.error").value("internal_error"))
                .andExpect(jsonPath("$.msg").value("boom 测试异常"));
    }

    @Test
    void 参数校验失败_返回bad_request() throws Exception {
        mvc.perform(post("/api/v1/_test/validate")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"name\":\"\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value(-1))
                .andExpect(jsonPath("$.error").value("bad_request"))
                .andExpect(jsonPath("$.msg").value("名称不能为空"));
    }
}
