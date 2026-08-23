// 操作日志服务：对齐 Go 版 recordOperation（写 operation_logs）。
package com.tsloms.server.web;

import com.tsloms.server.model.OpTypes;
import com.tsloms.server.model.OperationLog;
import com.tsloms.server.repository.OperationLogRepository;
import jakarta.servlet.http.HttpServletRequest;
import org.springframework.stereotype.Service;

@Service
public class OperationLogService {

    private final OperationLogRepository logs;

    public OperationLogService(OperationLogRepository logs) {
        this.logs = logs;
    }

    /**
     * 记录操作日志。用户信息从请求属性取（AuthInterceptor 注入）；未登录时跳过。
     */
    public void record(HttpServletRequest request, String opType, String target, String detail) {
        Long userId = AuthInterceptor.userId(request);
        if (userId == null) {
            return;
        }
        OperationLog log = new OperationLog();
        log.userId = userId;
        Object uname = request.getAttribute(AuthInterceptor.ATTR_USERNAME);
        log.username = uname == null ? "" : String.valueOf(uname);
        log.opType = opType;
        log.target = target;
        log.detail = detail;
        logs.save(log);
    }
}
