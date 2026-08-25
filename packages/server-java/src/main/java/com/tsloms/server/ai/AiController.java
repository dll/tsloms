// AI 模块端点（规则基座兜底）：所有 AI 推理端点返回结构化空数据/提示，
// 前端不会 404。后续接入多源研判引擎时替换实现。
package com.tsloms.server.ai;

import com.tsloms.server.web.ApiResponse;
import java.util.List;
import java.util.Map;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/v1/ai")
public class AiController {

    // ==================== GET 查询端点 ====================

    @GetMapping("/advices")
    public ResponseEntity<?> advices() {
        return ok(Map.of("list", List.of(), "total", 0));
    }

    @GetMapping("/config")
    public ResponseEntity<?> config() {
        return ok(Map.of("enabled", false, "provider", "rule",
                "day_token_limit", 0, "day_call_limit", 0));
    }

    @GetMapping("/predict")
    public ResponseEntity<?> predict() {
        return ok(Map.of("list", List.of(), "total", 0));
    }

    @GetMapping("/predict/by-intersection")
    public ResponseEntity<?> predictByIntersection() {
        return ok(Map.of("list", List.of(), "total", 0));
    }

    @GetMapping("/reports")
    public ResponseEntity<?> reports() {
        return ok(Map.of("list", List.of(), "total", 0));
    }

    @GetMapping("/usage")
    public ResponseEntity<?> usage() {
        return ok(Map.of("tokens", 0, "calls", 0));
    }

    @GetMapping("/usage/logs")
    public ResponseEntity<?> usageLogs() {
        return ok(Map.of("list", List.of(), "total", 0));
    }

    @GetMapping("/advice/fault/{id}")
    public ResponseEntity<?> adviceFault(@PathVariable Long id) {
        return stub("故障 AI 建议暂不可用（规则基座模式）");
    }

    @GetMapping("/advice/workorder/{id}")
    public ResponseEntity<?> adviceWorkOrder(@PathVariable Long id) {
        return stub("工单 AI 建议暂不可用（规则基座模式）");
    }

    @GetMapping("/lifecycle/{hwid}")
    public ResponseEntity<?> lifecycle(@PathVariable String hwid) {
        return stub("生命周期预测暂不可用（规则基座模式）");
    }

    @GetMapping("/anomaly/stream")
    public ResponseEntity<?> anomalyStream() {
        return ResponseEntity.ok().contentType(
                org.springframework.http.MediaType.TEXT_EVENT_STREAM)
                .body("data: {\"type\":\"ping\"}\n\n");
    }

    // ==================== POST 操作端点 ====================

    @PostMapping("/advice/device")
    public ResponseEntity<?> adviceDevice(@RequestBody(required = false) Map<String, Object> body) {
        return stub("设备 AI 建议暂不可用");
    }

    @PostMapping("/advice/purchase")
    public ResponseEntity<?> advicePurchase(@RequestBody(required = false) Map<String, Object> body) {
        return stub("采购 AI 建议暂不可用");
    }

    @PostMapping("/advice/workorder/create")
    public ResponseEntity<?> adviceWorkOrderCreate(@RequestBody(required = false) Map<String, Object> body) {
        return stub("工单创建 AI 建议暂不可用");
    }

    @PostMapping("/decision/adopt")
    public ResponseEntity<?> decisionAdopt(@RequestBody(required = false) Map<String, Object> body) {
        return stub("AI 决策采纳暂不可用");
    }

    @PostMapping("/decision/center")
    public ResponseEntity<?> decisionCenter(@RequestBody(required = false) Map<String, Object> body) {
        return stub("AI 决策中心暂不可用");
    }

    @PostMapping("/diagnose/{id}")
    public ResponseEntity<?> diagnose(@PathVariable Long id,
                                      @RequestBody(required = false) Map<String, Object> body) {
        return stub("AI 诊断暂不可用");
    }

    @PostMapping("/nl/interact")
    public ResponseEntity<?> nlInteract(@RequestBody(required = false) Map<String, Object> body) {
        return stub("自然语言交互暂不可用");
    }

    @PostMapping("/predict/{id}/enhance")
    public ResponseEntity<?> predictEnhance(@PathVariable Long id,
                                            @RequestBody(required = false) Map<String, Object> body) {
        return stub("预测增强暂不可用");
    }

    @PostMapping("/predict/run")
    public ResponseEntity<?> predictRun(@RequestBody(required = false) Map<String, Object> body) {
        return stub("预测执行暂不可用（规则基座模式）");
    }

    @PostMapping("/report/generate")
    public ResponseEntity<?> reportGenerate(@RequestBody(required = false) Map<String, Object> body) {
        return stub("AI 报告生成暂不可用");
    }

    @PostMapping("/usage/reset")
    public ResponseEntity<?> usageReset(@RequestBody(required = false) Map<String, Object> body) {
        return ok(Map.of("message", "AI 用量已重置"));
    }

    @PutMapping("/config")
    public ResponseEntity<?> updateConfig(@RequestBody(required = false) Map<String, Object> body) {
        return ok(Map.of("message", "AI 配置已更新"));
    }

    // ------------------------------------------------------------------

    private ResponseEntity<?> stub(String msg) {
        return ResponseEntity.ok(ApiResponse.ok(Map.of(
                "message", msg, "stub", true)));
    }

    private ResponseEntity<?> ok(Map<String, ?> data) {
        return ResponseEntity.ok(ApiResponse.ok(data));
    }
}
