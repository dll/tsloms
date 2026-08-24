// 意见反馈接口：列表/创建/状态更新。契约对齐 Go 版 /feedbacks 路由组。
package com.tsloms.server.feedback;

import com.tsloms.server.model.Feedback;
import com.tsloms.server.model.FeedbackStatuses;
import com.tsloms.server.repository.FeedbackRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.OperationLogService;
import com.tsloms.server.web.Pagination;
import jakarta.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1")
public class FeedbackController {

    private final FeedbackRepository feedbacks;
    private final OperationLogService opLog;

    public FeedbackController(FeedbackRepository feedbacks, OperationLogService opLog) {
        this.feedbacks = feedbacks;
        this.opLog = opLog;
    }

    /** GET /feedbacks：分页 + 状态筛选。 */
    @GetMapping("/feedbacks")
    public ApiResponse<Map<String, Object>> list(
            @RequestParam(required = false) String status,
            HttpServletRequest request) {
        Pagination.Page pg = Pagination.of(request);
        long total = status == null || status.isBlank()
                ? feedbacks.count()
                : feedbacks.count((root, query, cb) ->
                        cb.equal(root.get("status"), status));
        List<Object> rows = new java.util.ArrayList<>();
        var pageable = PageRequest.of(pg.page() - 1, pg.pageSize(),
                Sort.by(Sort.Direction.DESC, "createdAt"));
        (status == null || status.isBlank()
                ? feedbacks.findAll(pageable)
                : feedbacks.findAll((root, query, cb) ->
                        cb.equal(root.get("status"), status), pageable))
                .forEach(rows::add);
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("list", rows);
        data.put("total", total);
        data.put("page", pg.page());
        data.put("page_size", pg.pageSize());
        return ApiResponse.ok(data);
    }

    /** 创建请求体。 */
    public record CreateRequest(String deviceHwId, String intersection, String title,
                                String content, String reporter, String contact,
                                String imageUrl) {
    }

    /** POST /feedbacks：提交反馈（登录用户）。 */
    @PostMapping("/feedbacks")
    public ResponseEntity<?> create(@RequestBody CreateRequest req,
                                    HttpServletRequest request) {
        if (req.title() == null || req.title().isBlank()) {
            return badRequest("请填写问题标题");
        }
        Feedback f = new Feedback();
        f.deviceHwId = nz(req.deviceHwId());
        f.intersection = nz(req.intersection());
        f.title = req.title();
        f.content = nz(req.content());
        f.reporter = nz(req.reporter());
        f.contact = nz(req.contact());
        f.imageUrl = nz(req.imageUrl());
        f.status = FeedbackStatuses.OPEN;
        feedbacks.save(f);
        opLog.record(request, com.tsloms.server.model.OpTypes.CREATE,
                "feedback/" + f.id, "提交问题反馈");
        return ok(Map.of("message", "反馈已提交", "id", f.id));
    }

    /** 状态更新请求体。 */
    public record UpdateRequest(String status, Long workOrderId) {
    }

    /** PUT /feedbacks/{id}：更新状态（open/processing/resolved/closed）。 */
    @PutMapping("/feedbacks/{id}")
    public ResponseEntity<?> updateStatus(@PathVariable Long id,
                                          @RequestBody UpdateRequest req,
                                          HttpServletRequest request) {
        var opt = feedbacks.findById(id);
        if (opt.isEmpty()) {
            return notFound("反馈不存在");
        }
        List<String> valid = List.of("open", "processing", "resolved", "closed");
        if (req.status() == null || !valid.contains(req.status())) {
            return badRequest("无效的反馈状态");
        }
        Feedback f = opt.get();
        f.status = req.status();
        if (req.workOrderId() != null) {
            f.workOrderId = req.workOrderId();
        }
        feedbacks.save(f);
        opLog.record(request, com.tsloms.server.model.OpTypes.UPDATE,
                "feedback/" + id, "更新反馈状态为 " + req.status());
        return ok(Map.of("message", "反馈已更新"));
    }

    static String nz(String s) {
        return s == null ? "" : s;
    }

    private ResponseEntity<?> badRequest(String msg) {
        return ResponseEntity.badRequest().body(ApiResponse.fail("bad_request", msg));
    }

    private ResponseEntity<?> notFound(String msg) {
        return ResponseEntity.status(404).body(ApiResponse.fail("not_found", msg));
    }

    private ResponseEntity<?> ok(Map<String, ?> data) {
        return ResponseEntity.ok(ApiResponse.ok(data));
    }
}
