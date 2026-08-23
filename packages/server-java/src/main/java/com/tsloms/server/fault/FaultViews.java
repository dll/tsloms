// 故障/工单视图构建：字段名与条件性姓名回填对齐 Go 版 faultView/workOrderView。
package com.tsloms.server.fault;

import com.tsloms.server.model.FaultRecord;
import com.tsloms.server.model.User;
import com.tsloms.server.model.WorkOrder;
import com.tsloms.server.workorder.WorkOrders;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

public final class FaultViews {

    private FaultViews() {
    }

    /** 批量预取负责人/维修人用户名（避免 N+1）。 */
    public static Map<Long, String> userNames(
            com.tsloms.server.repository.UserRepository users, List<FaultRecord> rows) {
        Map<Long, String> out = new HashMap<>();
        rows.stream()
                .flatMap(f -> java.util.stream.Stream.of(f.ownerId, f.repairerId))
                .filter(java.util.Objects::nonNull)
                .distinct()
                .forEach(id -> users.findById(id).ifPresent(u -> out.put(u.id, u.username)));
        return out;
    }

    /** 故障视图：键名对齐 Go 版 faultViewWithNames；有匹配姓名才输出 *_name。 */
    public static Map<String, Object> view(com.tsloms.server.repository.UserRepository users,
                                           FaultRecord f, Map<Long, String> names) {
        Map<String, Object> v = new LinkedHashMap<>();
        v.put("id", f.id);
        v.put("device_hw_id", f.deviceHwId);
        v.put("err_code", f.errCode);
        v.put("fault_type", f.faultType);
        v.put("fault_level", f.faultLevel);
        v.put("led_state", f.ledState);
        v.put("current_r", f.currentR);
        v.put("current_y", f.currentY);
        v.put("current_g", f.currentG);
        v.put("first_seen", f.firstSeen);
        v.put("last_seen", f.lastSeen);
        v.put("status", f.status);
        v.put("owner_id", f.ownerId);
        v.put("repairer_id", f.repairerId);
        v.put("confirmed_at", f.confirmedAt);
        v.put("dispatched_at", f.dispatchedAt);
        v.put("resolved_at", f.resolvedAt);
        v.put("work_order_id", f.workOrderId);
        v.put("created_at", f.createdAt);
        v.put("updated_at", f.updatedAt);
        // 识别研判可选字段
        v.put("confidence", f.confidence);
        v.put("recognition_source", f.recognitionSource);
        v.put("recognition_status", f.recognitionStatus);
        v.put("is_false_positive", f.isFalsePositive);
        v.put("evidence_count", f.evidenceCount);
        v.put("last_evaluation_id", f.lastEvaluationId);
        if (f.ownerId != null && names.containsKey(f.ownerId)) {
            String n = names.get(f.ownerId);
            if (n != null && !n.isEmpty()) {
                v.put("owner_name", n);
            }
        }
        if (f.repairerId != null && names.containsKey(f.repairerId)) {
            String n = names.get(f.repairerId);
            if (n != null && !n.isEmpty()) {
                v.put("repairer_name", n);
            }
        }
        return v;
    }

    /** 工单视图（含处理人姓名与超时状态），对齐 Go 版 workOrderView。 */
    public static Map<String, Object> workOrderView(
            com.tsloms.server.repository.UserRepository users, WorkOrder o) {
        String assigneeName = o.assigneeId == null ? "" : users.findById(o.assigneeId)
                .map(u -> u.username).orElse("");
        double overdueHours = WorkOrders.overdueHours(o);
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("id", o.id);
        m.put("order_no", o.orderNo);
        m.put("fault_id", o.faultId);
        m.put("device_hw_id", o.deviceHwId);
        m.put("status", o.status);
        m.put("assignee_id", o.assigneeId);
        m.put("assignee_name", assigneeName);
        m.put("result", o.result == null ? "" : o.result);
        m.put("created_at", o.createdAt);
        m.put("closed_at", o.closedAt);
        m.put("overdue", overdueHours > 0);
        m.put("overdue_hours", overdueHours);
        return m;
    }
}
