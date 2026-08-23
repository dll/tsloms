// 物料/库存接口：档案 CRUD + 统计 + 出入库流水 + 手动调整 + 工单领料。
// 契约对齐 Go 版 handler/inventory.go（模块开关 gating 待统一拦截器阶段补齐）。
package com.tsloms.server.inventory;

import com.tsloms.server.model.ExpenseTypes;
import com.tsloms.server.model.Material;
import com.tsloms.server.model.MaterialStock;
import com.tsloms.server.model.OpTypes;
import com.tsloms.server.model.RepairExpense;
import com.tsloms.server.model.StockTypes;
import com.tsloms.server.model.WorkOrder;
import com.tsloms.server.repository.MaterialRepository;
import com.tsloms.server.repository.MaterialStockRepository;
import com.tsloms.server.repository.RepairExpenseRepository;
import com.tsloms.server.repository.UserRepository;
import com.tsloms.server.repository.WorkOrderRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.AuthInterceptor;
import com.tsloms.server.web.OperationLogService;
import com.tsloms.server.web.Pagination;
import com.tsloms.server.web.RequirePerm;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.data.jpa.domain.Specification;
import org.springframework.http.ResponseEntity;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.bind.annotation.DeleteMapping;
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
public class InventoryController {

    private final MaterialRepository materials;
    private final MaterialStockRepository stocks;
    private final RepairExpenseRepository expenses;
    private final WorkOrderRepository workOrders;
    private final UserRepository users;
    private final OperationLogService opLog;

    public InventoryController(MaterialRepository materials, MaterialStockRepository stocks,
                               RepairExpenseRepository expenses, WorkOrderRepository workOrders,
                               UserRepository users, OperationLogService opLog) {
        this.materials = materials;
        this.stocks = stocks;
        this.expenses = expenses;
        this.workOrders = workOrders;
        this.users = users;
        this.opLog = opLog;
    }

    // ---------------- 视图（键名对齐 Go 版 materialView） ----------------

    static Map<String, Object> view(Material m) {
        Map<String, Object> v = new LinkedHashMap<>();
        v.put("id", m.id);
        v.put("code", m.code);
        v.put("name", m.name);
        v.put("category", m.category);
        v.put("spec", m.spec);
        v.put("unit", m.unit);
        v.put("unit_price", m.unitPrice);
        v.put("stock", m.stock);
        v.put("threshold", m.threshold);
        v.put("supplier_id", m.supplierId);
        v.put("note", m.note);
        v.put("device_hw_id", m.deviceHwId);
        v.put("status", m.status);
        v.put("low_stock", m.threshold > 0 && m.stock <= m.threshold);
        v.put("created_at", m.createdAt);
        v.put("updated_at", m.updatedAt);
        return v;
    }

    /** GET /inv/materials：分页多条件筛选。 */
    @GetMapping("/inv/materials")
    public ApiResponse<Map<String, Object>> list(
            @RequestParam(required = false) String keyword,
            @RequestParam(required = false) String category,
            @RequestParam(required = false) String status,
            @RequestParam(name = "low_stock", required = false) String lowStock,
            @RequestParam(name = "device_hw_id", required = false) String deviceHwId,
            HttpServletRequest request) {
        Pagination.Page pg = Pagination.of(request);

        Specification<Material> spec = (root, query, cb) -> {
            var preds = new ArrayList<jakarta.persistence.criteria.Predicate>();
            if (keyword != null && !keyword.isBlank()) {
                String like = "%" + keyword.trim() + "%";
                preds.add(cb.or(cb.like(root.get("code"), like),
                        cb.like(root.get("name"), like)));
            }
            if (category != null && !category.isBlank()) {
                preds.add(cb.equal(root.get("category"), category));
            }
            if (status != null && !status.isBlank()) {
                preds.add(cb.equal(root.get("status"), status));
            }
            if ("1".equals(lowStock) || "true".equals(lowStock)) {
                preds.add(cb.and(
                        cb.lessThanOrEqualTo(root.get("stock"), root.get("threshold")),
                        cb.greaterThan(root.get("threshold"), 0)));
            }
            if (deviceHwId != null && !deviceHwId.isBlank()) {
                preds.add(cb.equal(root.get("deviceHwId"), deviceHwId));
            }
            return cb.and(preds.toArray(new jakarta.persistence.criteria.Predicate[0]));
        };

        long total = materials.count(spec);
        List<Map<String, Object>> items = new ArrayList<>();
        materials.findAll(spec, PageRequest.of(pg.page() - 1, pg.pageSize(),
                        Sort.by(Sort.Direction.DESC, "createdAt")))
                .forEach(m -> items.add(view(m)));
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("list", items);
        data.put("total", total);
        data.put("page", pg.page());
        data.put("page_size", pg.pageSize());
        return ApiResponse.ok(data);
    }

    /** GET /inv/materials/stats：库存概览。 */
    @GetMapping("/inv/materials/stats")
    public ApiResponse<Map<String, Object>> stats() {
        long mCount = materials.findAll().stream()
                .filter(m -> "active".equals(m.status)).count();
        long lowCount = materials.findAll().stream()
                .filter(m -> "active".equals(m.status) && m.threshold > 0
                        && m.stock <= m.threshold).count();
        long stockRecords = stocks.count();

        double totalValue = 0;
        for (Material m : materials.findAll()) {
            if ("active".equals(m.status)) {
                totalValue += m.stock * m.unitPrice;
            }
        }
        return ApiResponse.ok(Map.of(
                "material_count", mCount,
                "low_stock_count", lowCount,
                "stock_record_count", stockRecords,
                "total_value", Math.round(totalValue * 100.0) / 100.0));
    }

    /** 新增/更新请求体（id>0 为更新）。 */
    public record SaveRequest(Long id, String code, String name, String category,
                              String spec, String unit, Double unitPrice, Integer stock,
                              Integer threshold, String deviceHwId, Long supplierId,
                              String note, String status) {
    }

    /** POST /inv/materials：新增（编码唯一+初始库存流水）。 */
    @PostMapping("/inv/materials")
    @RequirePerm("inventory:manage")
    public ResponseEntity<?> create(@RequestBody SaveRequest req, HttpServletRequest request) {
        if (req.name() == null || req.name().isBlank()) {
            return badRequest("参数错误（name 必填）");
        }
        String operator = operatorOf(request);
        if (req.id() == null || req.id() == 0) {
            if (req.code() == null || req.code().isBlank()) {
                return badRequest("请填写物料编码");
            }
            if (materials.existsByCode(req.code())) {
                return badRequest("物料编码已存在");
            }
        } else {
            return update(req, request);
        }

        Material m = new Material();
        applySave(m, req, "active");
        m.stock = req.stock() == null ? 0 : req.stock(); // 新增：初始库存
        try {
            materials.save(m);
        } catch (Exception e) {
            return serverError();
        }
        // 初始库存>0 写入库流水
        if (m.stock > 0) {
            MaterialStock s = new MaterialStock();
            s.materialId = m.id;
            s.materialName = m.name;
            s.type = StockTypes.IN;
            s.quantity = m.stock;
            s.price = m.unitPrice;
            s.amount = m.stock * m.unitPrice;
            s.refType = "adjust";
            s.refId = 0L;
            s.operator = operator;
            s.note = "初始库存";
            stocks.save(s);
        }
        opLog.record(request, OpTypes.CREATE, "material/" + m.id, "新增物料 " + m.name);
        return ok(Map.of("material", view(m), "message", "物料已新增"));
    }

    /** PUT /inv/materials/{id}：更新。 */
    @PutMapping("/inv/materials/{id}")
    @RequirePerm("inventory:manage")
    public ResponseEntity<?> updatePath(@PathVariable Long id, @RequestBody SaveRequest req,
                                        HttpServletRequest request) {
        Material updated = new Material();
        updated.id = id;
        return doUpdate(updated, req, request);
    }

    private ResponseEntity<?> update(SaveRequest req, HttpServletRequest request) {
        Material ref = new Material();
        ref.id = req.id();
        return doUpdate(ref, req, request);
    }

    private ResponseEntity<?> doUpdate(Material ref, SaveRequest req,
                                       HttpServletRequest request) {
        var opt = materials.findById(ref.id);
        if (opt.isEmpty()) {
            return notFound("物料不存在");
        }
        Material m = opt.get();
        applySave(m, req, m.status);
        materials.save(m);
        opLog.record(request, OpTypes.UPDATE, "material/" + m.id, "更新物料 " + m.name);
        return ok(Map.of("message", "物料已更新"));
    }

    private void applySave(Material m, SaveRequest req, String defaultStatus) {
        m.code = nz(req.code());
        m.name = req.name();
        m.category = nz(req.category());
        m.spec = nz(req.spec());
        m.unit = nz(req.unit());
        m.unitPrice = req.unitPrice() == null ? 0.0 : req.unitPrice();
        // 注意：stock 不在此处理——新增时显式设置初始库存；更新路径不改动库存（对齐 Go 版）
        m.threshold = req.threshold() == null ? 0 : req.threshold();
        m.deviceHwId = req.deviceHwId();
        m.supplierId = req.supplierId();
        m.note = nz(req.note());
        m.status = (req.status() == null || req.status().isBlank())
                ? defaultStatus : req.status();
    }

    /** DELETE /inv/materials/{id}（inventory:delete）。 */
    @DeleteMapping("/inv/materials/{id}")
    @RequirePerm("inventory:delete")
    public ResponseEntity<?> delete(@PathVariable Long id, HttpServletRequest request) {
        var opt = materials.findById(id);
        if (opt.isEmpty()) {
            return notFound("物料不存在");
        }
        materials.delete(opt.get());
        opLog.record(request, OpTypes.DELETE, "material/" + id,
                "删除物料 " + opt.get().name);
        return ok(Map.of("message", "删除成功"));
    }

    /** GET /inv/stocks：流水筛选（物料/类型/日期）。 */
    @GetMapping("/inv/stocks")
    public ApiResponse<Map<String, Object>> stockList(
            @RequestParam(name = "material_id", required = false) String materialId,
            @RequestParam(required = false) String type,
            @RequestParam(required = false) String from,
            @RequestParam(required = false) String to,
            HttpServletRequest request) {
        Pagination.Page pg = Pagination.of(request);

        Specification<MaterialStock> spec = (root, query, cb) -> {
            var preds = new ArrayList<jakarta.persistence.criteria.Predicate>();
            if (materialId != null && !materialId.isBlank()) {
                try {
                    preds.add(cb.equal(root.get("materialId"), Long.parseLong(materialId)));
                } catch (NumberFormatException ignored) {
                    // 与 Go 一致忽略非法值
                }
            }
            if (type != null && !type.isBlank()) {
                preds.add(cb.equal(root.get("type"), type));
            }
            Instant f = parseDate(from, false);
            if (f != null) {
                preds.add(cb.greaterThanOrEqualTo(root.get("createdAt"), f));
            }
            Instant t = parseDate(to, true);
            if (t != null) {
                preds.add(cb.lessThanOrEqualTo(root.get("createdAt"), t));
            }
            return cb.and(preds.toArray(new jakarta.persistence.criteria.Predicate[0]));
        };

        long total = stocks.count(spec);
        List<MaterialStock> rows = stocks.findAll(spec,
                PageRequest.of(pg.page() - 1, pg.pageSize(),
                        Sort.by(Sort.Direction.DESC, "createdAt")))
                .getContent();
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("list", rows);
        data.put("total", total);
        data.put("page", pg.page());
        data.put("page_size", pg.pageSize());
        return ApiResponse.ok(data);
    }

    /** 调整请求体。 */
    public record AdjustRequest(Long materialId, String type, Integer quantity, String note) {
    }

    /** POST /inv/stocks/adjust（inventory:manage）：手动调整（事务内改库存+写流水）。 */
    @PostMapping("/inv/stocks/adjust")
    @RequirePerm("inventory:manage")
    @Transactional
    public ResponseEntity<?> adjust(@RequestBody AdjustRequest req, HttpServletRequest request) {
        if (req.materialId() == null || req.type() == null || req.quantity() == null) {
            return badRequest("参数错误（material_id、type、quantity 必填）");
        }
        List<String> okTypes = List.of(StockTypes.IN, StockTypes.RETURN, StockTypes.GAIN,
                StockTypes.LOSS, StockTypes.ADJUST);
        if (!okTypes.contains(req.type())) {
            return badRequest("库存变动类型不合法");
        }
        if (req.quantity() == 0) {
            return badRequest("变动数量不能为0");
        }
        var mOpt = materials.findById(req.materialId());
        if (mOpt.isEmpty()) {
            return notFound("物料不存在");
        }
        Material m = mOpt.get();

        int delta = "loss".equals(req.type()) ? -req.quantity() : req.quantity();
        int newStock = m.stock + delta;
        if (newStock < 0) {
            return badRequest("库存不足，无法执行该变动");
        }
        double amount = delta * m.unitPrice;
        m.stock = newStock;
        materials.save(m);

        MaterialStock s = new MaterialStock();
        s.materialId = m.id;
        s.materialName = m.name;
        s.type = req.type();
        s.quantity = delta;
        s.price = m.unitPrice;
        s.amount = amount;
        s.refType = "adjust";
        s.refId = 0L;
        s.operator = operatorOf(request);
        s.note = nz(req.note());
        stocks.save(s);

        opLog.record(request, OpTypes.CREATE, "material-stock/" + m.id,
                "调整库存 " + m.name + " " + delta);
        return ok(Map.of("message", "库存已调整", "stock", newStock));
    }

    /** 领用请求体。 */
    public record UseRequest(Long materialId, Integer quantity, Long workOrderId, String note) {
    }

    /**
     * POST /inv/stocks/use（inventory:manage）：工单领料出库。
     * 扣减库存 + use 流水 + 自动生成耗材费用单（事务一致）。
     */
    @PostMapping("/inv/stocks/use")
    @RequirePerm("inventory:manage")
    @Transactional
    public ResponseEntity<?> use(@RequestBody UseRequest req, HttpServletRequest request) {
        if (req.materialId() == null || req.quantity() == null || req.workOrderId() == null) {
            return badRequest("参数错误（material_id、quantity、work_order_id 必填）");
        }
        if (req.quantity() <= 0) {
            return badRequest("领用数量必须大于0");
        }
        var woOpt = workOrders.findById(req.workOrderId());
        if (woOpt.isEmpty()) {
            return notFound("工单不存在");
        }
        WorkOrder wo = woOpt.get();
        var mOpt = materials.findById(req.materialId());
        if (mOpt.isEmpty()) {
            return notFound("物料不存在");
        }
        Material m = mOpt.get();
        if (m.stock < req.quantity()) {
            return badRequest("库存不足，无法领用");
        }
        int newStock = m.stock - req.quantity;
        double amount = -req.quantity() * m.unitPrice;
        String operator = operatorOf(request);

        m.stock = newStock;
        materials.save(m);

        MaterialStock s = new MaterialStock();
        s.materialId = m.id;
        s.materialName = m.name;
        s.type = StockTypes.USE;
        s.quantity = -req.quantity();
        s.price = m.unitPrice;
        s.amount = amount;
        s.refType = "repair";
        s.refId = req.workOrderId();
        s.workOrderId = req.workOrderId();
        s.operator = operator;
        s.note = nz(req.note());
        stocks.save(s);

        // 自动生成耗材费用单，归集维修成本
        RepairExpense e = new RepairExpense();
        e.expenseNo = BizNo.next(expenses::countByExpenseNoStartingWith, "FE");
        e.workOrderId = req.workOrderId();
        e.deviceHwId = nz(wo.deviceHwId);
        e.type = ExpenseTypes.MATERIAL;
        e.amount = req.quantity() * m.unitPrice;
        e.description = "工单领料: " + m.name + " x" + req.quantity();
        e.operator = operator;
        e.confirmed = false;
        e.note = nz(req.note());
        expenses.save(e);

        opLog.record(request, OpTypes.CREATE, "material-stock/use/" + m.id,
                "工单#" + wo.orderNo + " 领用 " + m.name + " x" + req.quantity());
        return ok(Map.of("message", "领料出库成功", "stock", newStock,
                "material_id", m.id));
    }

    // ------------------------------------------------------------------

    static String nz(String s) {
        return s == null ? "" : s;
    }

    static String operatorOf(HttpServletRequest request) {
        Object u = request.getAttribute(AuthInterceptor.ATTR_USERNAME);
        String name = u == null ? "" : String.valueOf(u);
        return name.isEmpty() ? "system" : name;
    }

    private static Instant parseDate(String s, boolean endOfDay) {
        if (s == null || s.isBlank()) {
            return null;
        }
        try {
            LocalDate d = LocalDate.parse(s.trim(), DateTimeFormatter.ofPattern("yyyy-MM-dd"));
            return endOfDay
                    ? d.plusDays(1).atStartOfDay(ZoneId.systemDefault()).toInstant().minusNanos(1)
                    : d.atStartOfDay(ZoneId.systemDefault()).toInstant();
        } catch (Exception e) {
            return null;
        }
    }

    private ResponseEntity<?> badRequest(String msg) {
        return ResponseEntity.badRequest().body(ApiResponse.fail("bad_request", msg));
    }

    private ResponseEntity<?> notFound(String msg) {
        return ResponseEntity.status(404).body(ApiResponse.fail("not_found", msg));
    }

    private ResponseEntity<?> serverError() {
        return ResponseEntity.internalServerError()
                .body(ApiResponse.fail("internal_error", "服务器内部错误"));
    }

    private ResponseEntity<?> ok(Map<String, ?> data) {
        return ResponseEntity.ok(ApiResponse.ok(data));
    }
}
