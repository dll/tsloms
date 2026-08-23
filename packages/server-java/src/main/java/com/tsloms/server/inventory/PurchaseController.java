// 采购接口：创建/入库(支持部分)/取消/删除，事务一致对齐 Go 版 handler/purchase.go。
package com.tsloms.server.inventory;

import com.tsloms.server.model.Material;
import com.tsloms.server.model.MaterialStock;
import com.tsloms.server.model.OpTypes;
import com.tsloms.server.model.PurchaseOrder;
import com.tsloms.server.model.PurchaseOrderItem;
import com.tsloms.server.model.PurchaseStatuses;
import com.tsloms.server.model.StockTypes;
import com.tsloms.server.repository.MaterialRepository;
import com.tsloms.server.repository.MaterialStockRepository;
import com.tsloms.server.repository.PurchaseOrderItemRepository;
import com.tsloms.server.repository.PurchaseOrderRepository;
import com.tsloms.server.repository.SupplierRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.OperationLogService;
import com.tsloms.server.web.Pagination;
import com.tsloms.server.web.RequirePerm;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Instant;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.jpa.domain.Specification;
import org.springframework.data.domain.Sort;
import org.springframework.http.ResponseEntity;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1")
public class PurchaseController {

    private final PurchaseOrderRepository orders;
    private final PurchaseOrderItemRepository items;
    private final SupplierRepository suppliers;
    private final MaterialRepository materials;
    private final MaterialStockRepository stocks;
    private final OperationLogService opLog;

    public PurchaseController(PurchaseOrderRepository orders,
                              PurchaseOrderItemRepository items,
                              SupplierRepository suppliers, MaterialRepository materials,
                              MaterialStockRepository stocks, OperationLogService opLog) {
        this.orders = orders;
        this.items = items;
        this.suppliers = suppliers;
        this.materials = materials;
        this.stocks = stocks;
        this.opLog = opLog;
    }

    /** 视图（键名对齐 Go 版 purchaseView）。 */
    private Map<String, Object> view(PurchaseOrder po, String supplierName,
                                     List<PurchaseOrderItem> itemRows) {
        List<Map<String, Object>> ivs = new ArrayList<>();
        if (itemRows != null) {
            for (var it : itemRows) {
                Map<String, Object> m = new LinkedHashMap<>();
                m.put("id", it.id);
                m.put("material_id", it.materialId);
                m.put("material_name", it.materialName);
                m.put("quantity", it.quantity);
                m.put("price", it.price);
                m.put("amount", it.amount);
                m.put("received_qty", it.receivedQty);
                ivs.add(m);
            }
        }
        Map<String, Object> v = new LinkedHashMap<>();
        v.put("id", po.id);
        v.put("order_no", po.orderNo);
        v.put("supplier_id", po.supplierId);
        v.put("supplier_name", supplierName);
        v.put("status", po.status);
        v.put("total_amount", po.totalAmount);
        v.put("received_at", po.receivedAt);
        v.put("operator", po.operator);
        v.put("note", po.note);
        v.put("created_at", po.createdAt);
        v.put("items", ivs);
        return v;
    }

    /** GET /purchases：分页 + 单号/供应商/状态筛选。 */
    @GetMapping("/purchases")
    public ApiResponse<Map<String, Object>> list(
            @RequestParam(name = "order_no", required = false) String orderNo,
            @RequestParam(name = "supplier_id", required = false) String supplierId,
            @RequestParam(required = false) String status,
            HttpServletRequest request) {
        Pagination.Page pg = Pagination.of(request);

        Specification<PurchaseOrder> spec = (root, query, cb) -> {
            var preds = new ArrayList<jakarta.persistence.criteria.Predicate>();
            if (orderNo != null && !orderNo.isBlank()) {
                preds.add(cb.like(root.get("orderNo"), "%" + orderNo.trim() + "%"));
            }
            if (supplierId != null && !supplierId.isBlank()) {
                try {
                    preds.add(cb.equal(root.get("supplierId"), Long.parseLong(supplierId)));
                } catch (NumberFormatException ignored) {
                    // 与 Go 一致忽略非法值
                }
            }
            if (status != null && !status.isBlank()) {
                preds.add(cb.equal(root.get("status"), status));
            }
            return cb.and(preds.toArray(new jakarta.persistence.criteria.Predicate[0]));
        };

        long total = orders.count(spec);
        List<Object> list = new ArrayList<>();
        orders.findAll(spec, PageRequest.of(pg.page() - 1, pg.pageSize(),
                        Sort.by(Sort.Direction.DESC, "createdAt")))
                .forEach(po -> list.add(view(po, supplierName(po.supplierId), null)));

        Map<String, Object> data = new LinkedHashMap<>();
        data.put("list", list);
        data.put("total", total);
        data.put("page", pg.page());
        data.put("page_size", pg.pageSize());
        return ApiResponse.ok(data);
    }

    private String supplierName(Long supplierId) {
        return suppliers.findById(supplierId).map(s -> s.name).orElse("");
    }

    /** GET /purchases/{id}：详情含明细。 */
    @GetMapping("/purchases/{id}")
    public ResponseEntity<?> get(@PathVariable Long id) {
        var opt = orders.findById(id);
        if (opt.isEmpty()) {
            return notFound("采购单不存在");
        }
        PurchaseOrder po = opt.get();
        return ok(Map.of("purchase", view(po, supplierName(po.supplierId),
                items.findByOrderIdOrderByIdAsc(id))));
    }

    /** 采购明细请求。 */
    public record ItemRequest(Long materialId, Integer quantity, Double price) {
    }

    /** 创建请求体。 */
    public record CreateRequest(Long supplierId, String note, List<ItemRequest> items) {
    }

    /** POST /purchases（purchase:manage）：草稿单+明细（价格缺省取物料单价）。 */
    @PostMapping("/purchases")
    @RequirePerm("purchase:manage")
    @Transactional
    public ResponseEntity<?> create(@RequestBody CreateRequest req,
                                    HttpServletRequest request) {
        if (req.supplierId() == null || req.items() == null || req.items().isEmpty()) {
            return badRequest("参数错误（supplier_id、items 必填）");
        }
        if (suppliers.findById(req.supplierId()).isEmpty()) {
            return badRequest("供应商不存在");
        }
        String operator = InventoryController.operatorOf(request);

        double totalAmount = 0;
        List<PurchaseOrderItem> itemRows = new ArrayList<>();
        for (var it : req.items()) {
            if (it.quantity() == null || it.quantity() <= 0) {
                return badRequest("采购数量必须大于0");
            }
            var mOpt = materials.findById(it.materialId());
            if (mOpt.isEmpty()) {
                return badRequest("物料 #" + it.materialId() + " 不存在");
            }
            Material m = mOpt.get();
            double price = it.price() != null && it.price() > 0 ? it.price() : m.unitPrice;
            double amt = it.quantity() * price;
            totalAmount += amt;
            PurchaseOrderItem row = new PurchaseOrderItem();
            row.materialId = m.id;
            row.materialName = m.name;
            row.quantity = it.quantity();
            row.price = price;
            row.amount = amt;
            itemRows.add(row);
        }

        PurchaseOrder po = new PurchaseOrder();
        po.orderNo = BizNo.next(orders::countByOrderNoStartingWith, "PO");
        po.supplierId = req.supplierId();
        po.status = PurchaseStatuses.DRAFT;
        po.totalAmount = totalAmount;
        po.operator = operator;
        po.note = nz(req.note());
        orders.saveAndFlush(po);

        for (PurchaseOrderItem row : itemRows) {
            row.orderId = po.id;
            items.save(row);
        }
        opLog.record(request, OpTypes.CREATE, "purchase/" + po.id,
                "创建采购单 " + po.orderNo);
        return ok(Map.of("purchase", po, "message", "采购单已创建"));
    }

    /** 入库请求体。 */
    public record ReceiveItemRequest(Long itemId, Integer quantity) {
    }

    public record ReceiveRequest(List<ReceiveItemRequest> items) {
    }

    /**
     * POST /purchases/{id}/receive（purchase:manage）：入库。
     * 校验明细归属/数量上限 → 更新已收数量 + 加库存 + in 流水；全部到齐置 completed。
     */
    @PostMapping("/purchases/{id}/receive")
    @RequirePerm("purchase:manage")
    @Transactional
    public ResponseEntity<?> receive(@PathVariable Long id, @RequestBody ReceiveRequest req,
                                     HttpServletRequest request) {
        var poOpt = orders.findById(id);
        if (poOpt.isEmpty()) {
            return notFound("采购单不存在");
        }
        PurchaseOrder po = poOpt.get();
        if (PurchaseStatuses.COMPLETED.equals(po.status)
                || PurchaseStatuses.CANCELLED.equals(po.status)) {
            return badRequest("采购单已完成或已取消，无法入库");
        }
        if (req.items() == null || req.items().isEmpty()) {
            return badRequest("参数错误");
        }
        String operator = InventoryController.operatorOf(request);

        // 明细归属映射
        Map<Long, PurchaseOrderItem> itemMap = new HashMap<>();
        for (PurchaseOrderItem it : items.findByOrderIdOrderByIdAsc(id)) {
            itemMap.put(it.id, it);
        }
        for (var r : req.items()) {
            var it = r.itemId() == null ? null : itemMap.get(r.itemId());
            if (it == null) {
                return badRequest("明细 #" + r.itemId() + " 不属于该采购单");
            }
            if (r.quantity() == null || r.quantity() <= 0) {
                return badRequest("入库数量必须大于0");
            }
            if (it.receivedQty + r.quantity() > it.quantity) {
                return badRequest("物料「" + it.materialName + "」入库数量超过采购数量");
            }
        }

        boolean allComplete = true;
        for (var r : req.items()) {
            PurchaseOrderItem it = itemMap.get(r.itemId());
            it.receivedQty += r.quantity();
            items.save(it);
            Material m = materials.findById(it.materialId).orElse(null);
            if (m != null) {
                m.stock += r.quantity();
                materials.save(m);
            }
            MaterialStock s = new MaterialStock();
            s.materialId = it.materialId;
            s.materialName = it.materialName;
            s.type = StockTypes.IN;
            s.quantity = r.quantity();
            s.price = it.price;
            s.amount = r.quantity() * it.price;
            s.refType = "purchase";
            s.refId = po.id;
            s.operator = operator;
            s.note = "采购入库 " + po.orderNo;
            stocks.save(s);
        }
        // 全部明细到齐？
        for (PurchaseOrderItem a : items.findByOrderIdOrderByIdAsc(id)) {
            if (a.receivedQty < a.quantity) {
                allComplete = false;
                break;
            }
        }
        po.status = allComplete ? PurchaseStatuses.COMPLETED : PurchaseStatuses.PARTIAL;
        if (allComplete) {
            po.receivedAt = Instant.now();
        }
        orders.save(po);

        String opText = PurchaseStatuses.DRAFT.equals(po.status)
                ? "采购单部分入库" : "采购入库";
        opLog.record(request, OpTypes.UPDATE, "purchase/" + po.id,
                opText + " " + po.orderNo);
        return ok(Map.of("message", "入库成功"));
    }

    /** POST /purchases/{id}/cancel：仅草稿/部分可取消。 */
    @PostMapping("/purchases/{id}/cancel")
    @RequirePerm("purchase:manage")
    public ResponseEntity<?> cancel(@PathVariable Long id, HttpServletRequest request) {
        var opt = orders.findById(id);
        if (opt.isEmpty()) {
            return notFound("采购单不存在");
        }
        PurchaseOrder po = opt.get();
        if (PurchaseStatuses.COMPLETED.equals(po.status)
                || PurchaseStatuses.CANCELLED.equals(po.status)) {
            return badRequest("采购单已完成或已取消");
        }
        po.status = PurchaseStatuses.CANCELLED;
        orders.save(po);
        opLog.record(request, OpTypes.UPDATE, "purchase/" + po.id,
                "取消采购单 " + po.orderNo);
        return ok(Map.of("message", "采购单已取消"));
    }

    /** DELETE /purchases/{id}：仅草稿可删，级联删明细。 */
    @DeleteMapping("/purchases/{id}")
    @RequirePerm("purchase:delete")
    @Transactional
    public ResponseEntity<?> delete(@PathVariable Long id, HttpServletRequest request) {
        var opt = orders.findById(id);
        if (opt.isEmpty()) {
            return notFound("采购单不存在");
        }
        PurchaseOrder po = opt.get();
        if (!PurchaseStatuses.DRAFT.equals(po.status)) {
            return badRequest("仅草稿状态的采购单可删除");
        }
        items.deleteByOrderId(id);
        orders.delete(po);
        opLog.record(request, OpTypes.DELETE, "purchase/" + po.id,
                "删除采购单 " + po.orderNo);
        return ok(Map.of("message", "删除成功"));
    }

    private static String nz(String s) {
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
