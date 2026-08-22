// 统一响应信封：与 Go 版 handler.ok/fail 逐字段对齐。
package com.tsloms.server.web;

import com.fasterxml.jackson.annotation.JsonInclude;

/**
 * 统一响应信封。
 *
 * <p>成功：{"code":0,"msg":"success","data":{...}}（Go 版 handler.ok）
 * <p>失败：{"code":-1,"msg":"...","error":"err_code"}（Go 版 handler.fail）
 *
 * @param code 业务码：0 成功，-1 失败
 * @param msg 提示信息
 * @param data 成功时的业务数据（失败时为 null，序列化时省略）
 * @param error 失败时的错误码（成功时为 null，序列化时省略）
 */
@JsonInclude(JsonInclude.Include.NON_NULL)
public record ApiResponse<T>(int code, String msg, T data, String error) {

    /** 成功信封（对齐 Go 版 ok()）。 */
    public static <T> ApiResponse<T> ok(T data) {
        return new ApiResponse<>(0, "success", data, null);
    }

    /** 失败信封（对齐 Go 版 fail()）。 */
    public static ApiResponse<Void> fail(String errCode, String message) {
        return new ApiResponse<>(-1, message, null, errCode);
    }
}
