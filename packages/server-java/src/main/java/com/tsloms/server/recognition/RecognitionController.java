// 识别数据面接口：证据查询/来源枚举/证据注入/复核/案例库。
// 契约对齐 Go 版 handler/recognition.go 路由组（引擎判定以规则基座兜底）。
package com.tsloms.server.recognition;

import com.tsloms.server.model.CaseStatuses;
import com.tsloms.server.model.FaultCase;
import com.tsloms.server.model.FaultEvidence;
import com.tsloms.server.model.FaultRecord;
import com.tsloms.server.model.FaultRecord;
import com.tsloms.server.model.OpTypes;
import com.tsloms.server.repository.FaultCaseRepository;
import com.tsloms.server.repository.FaultEvidenceRepository;
import com.tsloms.server.repository.FaultRecordRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.OperationLogService;
import com.tsloms.server.web.RequirePerm;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Instant;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.springframework.data.domain.Sort;
import org.springframework.http.ResponseEntity;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1")
public class RecognitionController {

    private final FaultEvidenceRepository evidenceRepo;
    private final FaultCaseRepository cases;
    private final FaultRecordRepository faults;
    private final OperationLogService opLog;

    public RecognitionController(FaultEvidenceRepository evidenceRepo,
                                 FaultCaseRepository cases, FaultRecordRepository faults,
                                 OperationLogService opLog) {
        this.evidenceRepo = evidenceRepo;
        this.cases = cases;
        this.faults = faults;
        this.opLog = opLog;
    }

    /** GET /evidence/sources：证据来源枚举。 */
    @GetMapping("/evidence/sources")
    public ApiResponse<List<String>> sources() {
        return ApiResponse.ok(List.of(
                FaultEvidence.SRC_FIRMWARE, FaultEvidence.SRC_CURRENT,
                FaultEvidence.SRC_LED_STATE, FaultEvidence.SRC_CITIZEN,
                FaultEvidence.SRC_PHOTO_EVIDENCE, FaultEvidence.SRC_VIDEO_MONITOR));
    }

    /** GET /faults/{id}/evidence：某故障的证据列表（含同批次）。 */
    @GetMapping("/faults/{id}/evidence")
    public ResponseEntity<?> listByFault(@PathVariable Long id) {
        var faultOpt = faults.findById(id);
        if (faultOpt.isEmpty()) {
            return notFound("故障记录不存在");
        }
        FaultRecord f = faultOpt.get();
        List<FaultEvidence> rows =
                evidenceRepo.findByFaultIdOrderByIdAsc(id);
        if (rows.isEmpty() && f.lastEvaluationId != null && !f.lastEvaluationId.isBlank()) {
            rows = evidenceRepo.findByEvaluationIdOrderByIdAsc(f.lastEvaluationId);
        }
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("list", rows);
        data.put("total", rows.size());
        return ResponseEntity.ok(ApiResponse.ok(data));
    }

    /** 证据注入请求体。 */
    public record IngestRequest(Long faultId, String sourceType, String rawData,
                                Long refMediaId, Long refFeedbackId, Double confidence) {
    }

    /** POST /evidence/ingest（evidence:ingest）：外部证据写入，关联故障批次。 */
    @PostMapping("/evidence/ingest")
    @RequirePerm("evidence:ingest")
    @Transactional
    public ResponseEntity<?> ingest(@RequestBody IngestRequest req,
                                    HttpServletRequest request) {

        java.util.Optional<FaultRecord> faultRec =
                req.faultId() == null ? java.util.Optional.empty()
                        : faults.findById(req.faultId());
        if (faultRec.isEmpty() && (req.refMediaId() == null && req.refFeedbackId() == null)) {
            return badRequest("参数错误：fault_id 或 媒体/反馈引用 至少一项");
        }
        List<String> okSources = List.of(FaultEvidence.SRC_FIRMWARE,
                FaultEvidence.SRC_CURRENT, FaultEvidence.SRC_LED_STATE,
                FaultEvidence.SRC_CITIZEN, FaultEvidence.SRC_PHOTO_EVIDENCE,
                FaultEvidence.SRC_VIDEO_MONITOR);
        if (req.sourceType() == null || !okSources.contains(req.sourceType())) {
            return badRequest("证据来源不合法");
        }
        FaultEvidence e = new FaultEvidence();
        if (faultRec.isPresent()) {
            FaultRecord f = faultRec.get();
            e.faultId = f.id;
            e.deviceHwId = nz(f.deviceHwId);
            e.evaluationId = nz(f.lastEvaluationId);
        } else {
            e.deviceHwId = "unknown";
            e.evaluationId = UUID.randomUUID().toString().replace("-", "").substring(0, 32);
        }
        e.sourceType = req.sourceType();
        e.rawData = nz(req.rawData());
        e.refMediaId = req.refMediaId();
        e.refFeedbackId = req.refFeedbackId();
        e.confidence = req.confidence() == null ? 0.5 : req.confidence();
        evidenceRepo.save(e);

        opLog.record(request, OpTypes.CREATE, "evidence/" + e.id, "注入多源证据");
        return ok(Map.of("id", e.id, "message", "证据已登记"));
    }

    /** 复核请求体。 */
    public record ReviewRequest(boolean approve, String remark) {
    }

    /**
     * POST /faults/{id}/review（fault:review）：待确认复核。
     * approve=true → 确认；critical 自动派单（pending 工单）；false → 判定误报过滤。
     */
    @PostMapping("/faults/{id}/review")
    @RequirePerm("fault:review")
    @Transactional
    public ResponseEntity<?> review(@PathVariable Long id, @RequestBody ReviewRequest req,
                                    HttpServletRequest request) {
        var opt = faults.findById(id);
        if (opt.isEmpty()) {
            return notFound("故障记录不存在");
        }
        FaultRecord f = opt.get();

        if (req.approve()) {
            f.recognitionStatus = "confirmed";
            f.reviewedAt = Instant.now();
            faults.save(f);
            boolean woCreated = false;
            Long woId = null;
            if ("critical".equals(f.faultLevel) && f.workOrderId == null) {
                // critical 自动建单（复用活跃位防重）
                com.tsloms.server.model.WorkOrder wo =
                        new com.tsloms.server.model.WorkOrder();
                wo.orderNo = com.tsloms.server.workorder.WorkOrders.nextOrderNo(
                        workOrders);
                wo.faultId = f.id;
                wo.deviceHwId = nz(f.deviceHwId);
                wo.status = "pending";
                wo.faultActiveScope = f.id;
                try {
                    workOrders.saveAndFlush(wo);
                    woCreated = true;
                    woId = wo.id;
                    f.workOrderId = wo.id;
                    faults.save(f);
                } catch (Exception ignored) {
                    // 冲突时已有活跃单
                    woId = workOrders.findFirstByFaultIdAndFaultActiveScope(f.id, f.id)
                            .map(w -> w.id).orElse(null);
                }
            }
            opLog.record(request, OpTypes.UPDATE, "fault/" + id, "复核确认");
            Map<String, Object> data = new LinkedHashMap<>();
            data.put("status", f.status);
            data.put("work_order_id", woId);
            data.put("auto_dispatched", woCreated);
            data.put("message", "复核已确认");
            return ResponseEntity.ok(ApiResponse.ok(data));
        }

        // 误报过滤：标记后保留记录供案例学习
        f.recognitionStatus = "filtered";
        f.isFalsePositive = true;
        f.reviewedAt = Instant.now();
        f.status = "resolved";
        f.resolvedAt = Instant.now();
        faults.save(f);
        // 沉淀误报案例
        FaultCase c = new FaultCase();
        c.faultType = nz(f.faultType);
        c.faultLevel = nz(f.faultLevel);
        c.deviceHwId = nz(f.deviceHwId);
        c.inputSignature = Integer.toHexString(java.util.Objects.hash(
                f.errCode, f.ledState, f.currentR, f.currentY, f.currentG));
        c.evidenceSummary = "{\"review\":\"rejected\",\"remark\":"
                + jsonQuote(nz(req.remark())) + "}";
        c.expectedResult = "normal";
        c.judgedResult = nz(f.faultType);
        c.judgeConfidence = f.confidence;
        c.isCorrect = false;
        c.sourceEvaluationId = nz(f.lastEvaluationId);
        c.status = CaseStatuses.CONFIRMED;
        cases.save(c);

        opLog.record(request, OpTypes.UPDATE, "fault/" + id, "复核判为误报过滤");
        return ok(Map.of("message", "已判为误报并过滤"));
    }

    /** GET /fault-cases：案例列表。 */
    @GetMapping("/fault-cases")
    public ApiResponse<Map<String, Object>> listCases() {
        List<Object> rows = new ArrayList<>();
        cases.findAll(Sort.by(Sort.Direction.DESC, "createdAt"))
                .forEach(rows::add);
        return ApiResponse.ok(Map.of("list", rows, "total", rows.size()));
    }

    /** 案例创建请求体。 */
    public record CaseRequest(String faultType, String faultLevel, String deviceHwId,
                              String inputSignature, String evidenceSummary,
                              String expectedResult, String judgedResult) {
    }

    /** POST /fault-cases（faultcase:manage）：人工录入基准案例。 */
    @PostMapping("/fault-cases")
    @RequirePerm("faultcase:manage")
    public ResponseEntity<?> createCase(@RequestBody CaseRequest req,
                                        HttpServletRequest request) {
        if (req.faultType() == null || req.faultType().isBlank()
                || req.expectedResult() == null || req.expectedResult().isBlank()) {
            return badRequest("参数错误（fault_type、expected_result 必填）");
        }
        FaultCase c = new FaultCase();
        c.faultType = req.faultType();
        c.faultLevel = nz(req.faultLevel());
        c.deviceHwId = nz(req.deviceHwId());
        c.inputSignature = nz(req.inputSignature());
        c.evidenceSummary = nz(req.evidenceSummary());
        c.expectedResult = req.expectedResult();
        c.judgedResult = nz(req.judgedResult());
        c.status = CaseStatuses.SEED;
        cases.save(c);
        opLog.record(request, OpTypes.CREATE, "fault-case/" + c.id,
                "录入案例 " + c.faultType);
        return ok(Map.of("id", c.id, "message", "案例已创建"));
    }

    /**
     * POST /fault-cases/train（faultcase:manage）：训练。
     * 规则基座下训练=回算 is_correct（judged==expected），并把 confirmed 样本置 training。
     */
    @PostMapping("/fault-cases/train")
    @RequirePerm("faultcase:manage")
    @Transactional
    public ResponseEntity<?> train(HttpServletRequest request) {
        int checked = 0;
        int corrected = 0;
        for (FaultCase c : cases.findAll()) {
            checked++;
            boolean correct = nz(c.expectedResult).equalsIgnoreCase(nz(c.judgedResult))
                    || "normal".equals(nz(c.expectedResult));
            Boolean prev = c.isCorrect;
            c.isCorrect = correct;
            if (!java.util.Objects.equals(prev, c.isCorrect)) {
                corrected++;
            }
            if (CaseStatuses.CONFIRMED.equals(c.status)
                    || CaseStatuses.SEED.equals(c.status)) {
                c.status = CaseStatuses.TRAINING;
            }
            cases.save(c);
        }
        opLog.record(request, OpTypes.UPDATE, "fault-cases/train",
                "训练完成 checked=" + checked + " corrected=" + corrected);
        return ok(Map.of("checked", checked, "corrected", corrected,
                "message", "训练完成"));
    }

    private com.tsloms.server.repository.WorkOrderRepository workOrders;

    /** 注入工单仓库（setter 避免与构造器顺序耦合）。 */
    @org.springframework.beans.factory.annotation.Autowired
    public void setWorkOrders(com.tsloms.server.repository.WorkOrderRepository repo) {
        this.workOrders = repo;
    }

    private static String jsonQuote(String s) {
        return "\"" + s.replace("\\", "\\\\").replace("\"", "\\\"") + "\"";
    }

    private ResponseEntity<?> ok(Map<String, ?> data) {
        return ResponseEntity.ok(ApiResponse.ok(data));
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
}
