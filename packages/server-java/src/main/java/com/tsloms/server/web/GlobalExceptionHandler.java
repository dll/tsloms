// 全局异常处理：错误响应契约对齐 Go 版 fail()/serverError()。
package com.tsloms.server.web;

import com.tsloms.server.config.AppProperties;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

/**
 * 统一异常出口。
 *
 * <p>对齐 Go 版行为：
 * <ul>
 *   <li>参数校验失败 → 400 {"code":-1,"msg":&lt;原因&gt;,"error":"bad_request"}
 *   <li>未捕获异常 → 500，生产环境固定文案"服务器内部错误"，非生产回显异常信息（serverError 同语义）
 * </ul>
 */
@RestControllerAdvice
public class GlobalExceptionHandler {

    private static final Logger log = LoggerFactory.getLogger(GlobalExceptionHandler.class);

    private final AppProperties app;

    public GlobalExceptionHandler(AppProperties app) {
        this.app = app;
    }

    /** 路径/请求参数类型不匹配（如 id 非数字）→ 对齐 Go 版 parseUint 失败的 badRequest。 */
    @ExceptionHandler(org.springframework.web.method.annotation.MethodArgumentTypeMismatchException.class)
    public ResponseEntity<ApiResponse<Void>> onTypeMismatch(
            org.springframework.web.method.annotation.MethodArgumentTypeMismatchException ex) {
        return ResponseEntity.badRequest().body(ApiResponse.fail("bad_request", "参数错误"));
    }

    /** 参数校验失败（对应 Go 版 badRequest）。 */
    @ExceptionHandler(MethodArgumentNotValidException.class)
    public ResponseEntity<ApiResponse<Void>> onValidation(MethodArgumentNotValidException ex) {
        return ResponseEntity.badRequest().body(ApiResponse.fail("bad_request", firstMessage(ex)));
    }

    /**
     * 方法级校验失败（Spring 6.1+ 对 record/@Valid 参数抛出此异常而非
     * MethodArgumentNotValidException），同样按 bad_request 处理。
     */
    @ExceptionHandler(org.springframework.web.method.annotation.HandlerMethodValidationException.class)
    public ResponseEntity<ApiResponse<Void>> onHandlerValidation(
            org.springframework.web.method.annotation.HandlerMethodValidationException ex) {
        String msg = "参数错误";
        if (!ex.getAllErrors().isEmpty()) {
            msg = ex.getAllErrors().get(0).getDefaultMessage();
        }
        return ResponseEntity.badRequest().body(ApiResponse.fail("bad_request", msg));
    }

    private String firstMessage(MethodArgumentNotValidException ex) {
        var fe = ex.getBindingResult().getFieldError();
        return fe != null ? fe.getDefaultMessage() : "参数错误";
    }

    /** 未捕获异常（对应 Go 版 serverError：生产不回显内部细节）。 */
    @ExceptionHandler(Exception.class)
    public ResponseEntity<ApiResponse<Void>> onUnexpected(Exception ex) {
        log.error("[TSLOMS] 未捕获异常", ex);
        String msg = app.isProduction() ? "服务器内部错误" : String.valueOf(ex.getMessage());
        return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
                .body(ApiResponse.fail("internal_error", msg));
    }
}
