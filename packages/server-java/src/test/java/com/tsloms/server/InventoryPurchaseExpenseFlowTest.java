// 库存→采购→领料→费用 全链路集成测试。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.tsloms.server.auth.CaptchaService;
import com.tsloms.server.model.FaultRecord;
import com.tsloms.server.model.Supplier;
import com.tsloms.server.model.PasswordHasher;
import com.tsloms.server.model.User;
import com.tsloms.server.model.WorkOrder;
import com.tsloms.server.repository.FaultRecordRepository;
import com.tsloms.server.repository.MaterialRepository;
import com.tsloms.server.repository.RepairExpenseRepository;
import com.tsloms.server.repository.SupplierRepository;
import com.tsloms.server.repository.UserRepository;
import com.tsloms.server.repository.WorkOrderRepository;
import java.time.Instant;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.transaction.support.TransactionTemplate;

@SpringBootTest
@AutoConfigureMockMvc
class InventoryPurchaseExpenseFlowTest {

    private static final String ADMIN = "inv_admin";
    private static final ObjectMapper JSON = new ObjectMapper();

    @Autowired MockMvc mvc;
    @Autowired CaptchaService captchaSvc;
    @Autowired UserRepository users;
    @Autowired MaterialRepository materials;
    @Autowired SupplierRepository suppliers;
    @Autowired RepairExpenseRepository expenses;
    @Autowired WorkOrderRepository workOrders;
    @Autowired FaultRecordRepository faults;
    @Autowired PasswordHasher hasher;
    @Autowired TransactionTemplate tx;

    private String adminToken() throws Exception {
        tx.executeWithoutResult(s -> {
            if (users.findByUsername(ADMIN).isEmpty()) {
                User u = new User();
                u.username = ADMIN;
                u.passwordHash = hasher.hash("Passw0rd!");
                u.role = "admin";
                u.status = "enabled";
                users.save(u);
            }
        });
        String res = mvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(TestSupport.login(captchaSvc, ADMIN, "Passw0rd!")))
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        int i = res.indexOf("\"token\":\"") + "\"token\":\"".length();
        return res.substring(i, res.indexOf('"', i));
    }

    @Test
    void 供应商_物料_采购入库_领料费用_全链路() throws Exception {
        String bearer = "Bearer " + adminToken();

        // ---- 供应商 ----
        mvc.perform(post("/api/v1/suppliers").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"name\":\"畅网供应商A\",\"contact\":\"老王\",\"phone\":\"13800000001\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("供应商已创建"));
        Long supplierId = suppliers.findAll().stream()
                .filter(s -> "畅网供应商A".equals(s.name)).findFirst().orElseThrow().id;

        // ---- 物料（初始库存 5）----
        String code = "MAT-" + UUID.randomUUID().toString().substring(0, 8);
        String matRes = mvc.perform(post("/api/v1/inv/materials")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"code\":\"" + code + "\",\"name\":\"LED灯珠\","
                                + "\"category\":\"灯组\",\"unit\":\"个\","
                                + "\"unit_price\":2.5,\"stock\":5,\"threshold\":10}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.material.stock").value(5))
                .andExpect(jsonPath("$.data.material.low_stock").value(true)) // 5<=10
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        Long materialId = JSON.readTree(matRes).at("/data/material/id").asLong();
        assertThat(materialId).isPositive();

        // 编码重复拒绝
        mvc.perform(post("/api/v1/inv/materials").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"code\":\"" + code + "\",\"name\":\"重复物料\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("物料编码已存在"));

        // ---- 采购单（草稿）----
        String poRes = mvc.perform(post("/api/v1/purchases").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"supplier_id\":" + supplierId
                                + ",\"items\":[{\"material_id\":" + materialId
                                + ",\"quantity\":20}]}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.purchase.status").value("draft"))
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        Long poId = JSON.readTree(poRes).at("/data/purchase/id").asLong();

        // 从详情接口读取明细（创建响应仅返回主单，与 Go 版一致）
        String poDetail = mvc.perform(get("/api/v1/purchases/" + poId)
                        .header("Authorization", bearer))
                .andExpect(status().isOk())
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        Long itemId = JSON.readTree(poDetail).at("/data/purchase/items/0/id").asLong();
        double itemPrice = JSON.readTree(poDetail).at("/data/purchase/items/0/price").asDouble();
        assertThat(itemId).isPositive();
        assertThat(itemPrice).isEqualTo(2.5); // 缺省取物料单价

        // 部分入库 10 个 → partial，库存 15
        mvc.perform(post("/api/v1/purchases/" + poId + "/receive")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"items\":[{\"item_id\":" + itemId + ",\"quantity\":10}]}"))
                .andExpect(status().isOk());
        assertThat(materials.findById(materialId).orElseThrow().stock).isEqualTo(15);
        var poAfterPartial = orders(poId);
        assertThat(poAfterPartial.status).isEqualTo("partial");

        // 超量入库 → 拒绝（剩余 10，尝试 11）
        mvc.perform(post("/api/v1/purchases/" + poId + "/receive")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"items\":[{\"item_id\":" + itemId + ",\"quantity\":11}]}"))
                .andExpect(status().isBadRequest());

        // 其余 10 个入库 → completed，库存 25
        mvc.perform(post("/api/v1/purchases/" + poId + "/receive")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"items\":[{\"item_id\":" + itemId + ",\"quantity\":10}]}"))
                .andExpect(status().isOk());
        assertThat(materials.findById(materialId).orElseThrow().stock).isEqualTo(25);
        assertThat(orders(poId).status).isEqualTo("completed");

        // 已完成再入库 → 拒绝；取消已完成 → 拒绝
        mvc.perform(post("/api/v1/purchases/" + poId + "/cancel")
                        .header("Authorization", bearer))
                .andExpect(status().isBadRequest());

        // ---- 手动调整：盘亏 3 → 库存 22；超扣 → 400 ----
        mvc.perform(post("/api/v1/inv/stocks/adjust").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"material_id\":" + materialId
                                + ",\"type\":\"loss\",\"quantity\":3,\"note\":\"破损\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.stock").value(22));
        mvc.perform(post("/api/v1/inv/stocks/adjust").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"material_id\":" + materialId
                                + ",\"type\":\"loss\",\"quantity\":99999}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("库存不足，无法执行该变动"));

        // ---- 工单领料：故障+工单 → use 出库 → 自动生成耗材费用 ----
        Long woId = tx.execute(s -> {
            FaultRecord f = new FaultRecord();
            f.deviceHwId = "inv-flow-hw";
            f.faultType = "lamp_off";
            f.faultLevel = "critical";
            f.firstSeen = Instant.now();
            f.lastSeen = Instant.now();
            f.status = "occurred";
            faults.save(f);
            WorkOrder w = new WorkOrder();
            w.orderNo = "WO-INV-" + UUID.randomUUID().toString().substring(0, 6);
            w.faultId = f.id;
            w.deviceHwId = "inv-flow-hw";
            w.status = "processing";
            return workOrders.save(w).id;
        });

        mvc.perform(post("/api/v1/inv/stocks/use").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"material_id\":" + materialId
                                + ",\"quantity\":2,\"work_order_id\":" + woId + "}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.stock").value(20));

        // 自动生成耗材费用单（type=material，金额=数量*单价）
        var expenseOpt = expenses.findAll().stream()
                .filter(e -> woId.equals(e.workOrderId)).findFirst();
        assertThat(expenseOpt).isPresent();
        assertThat(expenseOpt.get().type).isEqualTo("material");
        assertThat(expenseOpt.get().amount).isEqualTo(5.0); // 2 × 2.5

        // ---- 费用统计与确认 ----
        mvc.perform(get("/api/v1/expenses/stats").header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.total_count")
                        .value(org.hamcrest.Matchers.greaterThanOrEqualTo(1)))
                .andExpect(jsonPath("$.data.material")
                        .value(org.hamcrest.Matchers.greaterThanOrEqualTo(5.0)));

        Long expenseId = expenseOpt.get().id;
        mvc.perform(put("/api/v1/expenses/" + expenseId + "/confirm")
                        .header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("费用已确认"));

        mvc.perform(get("/api/v1/expenses").header("Authorization", bearer)
                        .param("confirmed", "true"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.total")
                        .value(org.hamcrest.Matchers.greaterThanOrEqualTo(1)));

        mvc.perform(delete("/api/v1/expenses/" + expenseId).header("Authorization", bearer))
                .andExpect(status().isOk());
    }

    @Test
    void 分支覆盖_供应商物料采购费用的更新与守卫() throws Exception {
        String bearer = "Bearer " + adminToken();

        // ---- 供应商：all=1 全量 / 更新 / 删除 ----
        mvc.perform(get("/api/v1/suppliers").header("Authorization", bearer)
                        .param("all", "1"))
                .andExpect(status().isOk());
        mvc.perform(post("/api/v1/suppliers").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"name\":\"待改供应商\"}"))
                .andExpect(status().isOk());
        Long supId = suppliers.findAll().stream()
                .filter(s -> "待改供应商".equals(s.name)).findFirst().orElseThrow().id;
        mvc.perform(put("/api/v1/suppliers/" + supId).header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"name\":\"已改供应商\",\"status\":\"disabled\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("供应商已更新"));
        mvc.perform(delete("/api/v1/suppliers/" + supId).header("Authorization", bearer))
                .andExpect(status().isOk());

        // ---- 物料：更新路径（不改库存）+ 统计 ----
        String code = "MATU-" + UUID.randomUUID().toString().substring(0, 8);
        String matRes = mvc.perform(post("/api/v1/inv/materials")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"code\":\"" + code + "\",\"name\":\"更新前\","
                                + "\"stock\":7,\"threshold\":2,\"unit_price\":3.0}"))
                .andExpect(status().isOk()).andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        Long matId = JSON.readTree(matRes).at("/data/material/id").asLong();

        mvc.perform(put("/api/v1/inv/materials/" + matId).header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"code\":\"" + code + "\",\"name\":\"更新后\","
                                + "\"stock\":999,\"unit_price\":4.0,\"threshold\":0}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("物料已更新"));
        var m = materials.findById(matId).orElseThrow();
        assertThat(m.name).isEqualTo("更新后");
        assertThat(m.unitPrice).isEqualTo(4.0);
        assertThat(m.stock).isEqualTo(7); // 更新路径不改动库存（对齐 Go 版）

        mvc.perform(get("/api/v1/inv/materials/stats").header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.material_count").isNumber());

        // ---- 采购：取消草稿 → 删除；非草稿删除拒绝 ----
        tx.executeWithoutResult(s -> {
            if (suppliers.findByName("采购测试供应商").isEmpty()) {
                Supplier sp = new Supplier();
                sp.name = "采购测试供应商";
                sp.status = "active";
                suppliers.save(sp);
            }
        });
        Long sid = suppliers.findByName("采购测试供应商").orElseThrow().id;
        String poRes2 = mvc.perform(post("/api/v1/purchases")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"supplier_id\":" + sid
                                + ",\"items\":[{\"material_id\":" + matId + ",\"quantity\":1}]}"))
                .andExpect(status().isOk()).andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        Long poId2 = JSON.readTree(poRes2).at("/data/purchase/id").asLong();

        // 非草稿（partial）不可删：先入库 1 个变 partial
        Long itemId2 = JSON.readTree(mvc.perform(get("/api/v1/purchases/" + poId2)
                        .header("Authorization", bearer)).andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8))
                .at("/data/purchase/items/0/id").asLong();
        mvc.perform(post("/api/v1/purchases/" + poId2 + "/receive")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"items\":[{\"item_id\":" + itemId2 + ",\"quantity\":1}]}"))
                .andExpect(status().isOk());
        mvc.perform(delete("/api/v1/purchases/" + poId2).header("Authorization", bearer))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("仅草稿状态的采购单可删除"));

        // 新建草稿单 → 取消 → 删除成功
        String poRes3 = mvc.perform(post("/api/v1/purchases")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"supplier_id\":" + sid
                                + ",\"items\":[{\"material_id\":" + matId + ",\"quantity\":1}]}"))
                .andExpect(status().isOk()).andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        Long poId3 = JSON.readTree(poRes3).at("/data/purchase/id").asLong();
        mvc.perform(post("/api/v1/purchases/" + poId3 + "/cancel")
                        .header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("采购单已取消"));

        String poRes4 = mvc.perform(post("/api/v1/purchases")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"supplier_id\":" + sid
                                + ",\"items\":[{\"material_id\":" + matId + ",\"quantity\":1}]}"))
                .andExpect(status().isOk()).andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        Long poId4 = JSON.readTree(poRes4).at("/data/purchase/id").asLong();
        mvc.perform(delete("/api/v1/purchases/" + poId4).header("Authorization", bearer))
                .andExpect(status().isOk());

        // ---- 费用：日期格式校验 + 更新路径 ----
        String expRes = mvc.perform(post("/api/v1/expenses")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"type\":\"labor\",\"amount\":80,"
                                + "\"device_hw_id\":\"exp-hw\",\"work_date\":\"2026-08-23\"}"))
                .andExpect(status().isOk()).andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        String expenseNo = JSON.readTree(expRes).at("/data/expense_no").asText();
        assertThat(expenseNo).startsWith("FE");

        Long expId = expenses.findAll().stream()
                .filter(e -> expenseNo.equals(e.expenseNo)).findFirst().orElseThrow().id;
        mvc.perform(put("/api/v1/expenses/" + expId).header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"type\":\"labor\",\"amount\":100,\"confirmed\":true}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("费用已更新"));
        assertThat(expenses.findById(expId).orElseThrow().amount).isEqualTo(100.0);

        mvc.perform(post("/api/v1/expenses").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"type\":\"other\",\"amount\":10,\"work_date\":\"23/08/2026\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("日期格式应为 yyyy-MM-dd"));
    }

    @Test
    void 库存接口_校验分支与流水筛选() throws Exception {
        String bearer = "Bearer " + adminToken();

        // 创建参数缺失 → 400
        mvc.perform(post("/api/v1/inv/materials").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"name\":\"无编码\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("请填写物料编码"));

        // 更新不存在的物料 → 404
        mvc.perform(put("/api/v1/inv/materials/999999").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"code\":\"X\",\"name\":\"Y\"}"))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.msg").value("物料不存在"));

        // 调整：类型非法 / 数量 0 / 物料不存在
        mvc.perform(post("/api/v1/inv/stocks/adjust").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"material_id\":1,\"type\":\"steal\",\"quantity\":5}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("库存变动类型不合法"));
        mvc.perform(post("/api/v1/inv/stocks/adjust").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"material_id\":1,\"type\":\"gain\",\"quantity\":0}"))
                .andExpect(status().isBadRequest());
        mvc.perform(post("/api/v1/inv/stocks/use").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"material_id\":999999,\"quantity\":1,"
                                + "\"work_order_id\":888888}"))
                .andExpect(status().isNotFound());

        // 领料：数量为 0 → 400；工单不存在 → 404
        String code = "MATB-" + UUID.randomUUID().toString().substring(0, 8);
        String matRes = mvc.perform(post("/api/v1/inv/materials")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"code\":\"" + code + "\",\"name\":\"领料测试\","
                                + "\"stock\":3,\"unit_price\":1.0}"))
                .andExpect(status().isOk()).andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        Long matId = JSON.readTree(matRes).at("/data/material/id").asLong();

        mvc.perform(post("/api/v1/inv/stocks/use").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"material_id\":" + matId + ",\"quantity\":0,"
                                + "\"work_order_id\":1}"))
                .andExpect(status().isBadRequest());

        mvc.perform(post("/api/v1/inv/stocks/use").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"material_id\":" + matId + ",\"quantity\":2,"
                                + "\"work_order_id\":999999}"))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.msg").value("工单不存在"));

        // 领用超库存 → 400（先造一个真实工单）
        Long woId = tx.execute(s -> {
            WorkOrder w = new WorkOrder();
            w.orderNo = "WO-BR-" + UUID.randomUUID().toString().substring(0, 6);
            w.faultId = 1L;
            w.deviceHwId = "hw-x";
            w.status = "pending";
            return workOrders.save(w).id;
        });
        mvc.perform(post("/api/v1/inv/stocks/use").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"material_id\":" + matId + ",\"quantity\":99,"
                                + "\"work_order_id\":" + woId + "}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("库存不足，无法领用"));

        // 正常领用 + 流水筛选
        mvc.perform(post("/api/v1/inv/stocks/use").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"material_id\":" + matId + ",\"quantity\":1,"
                                + "\"work_order_id\":" + woId + ",\"note\":\"换灯\"}"))
                .andExpect(status().isOk());
        mvc.perform(get("/api/v1/inv/stocks").header("Authorization", bearer)
                        .param("material_id", String.valueOf(matId))
                        .param("type", "use"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.total").value(1));

        // 删除物料
        mvc.perform(delete("/api/v1/inv/materials/" + matId).header("Authorization", bearer))
                .andExpect(status().isOk());
    }

    private com.tsloms.server.model.PurchaseOrder orders(Long id) {
        return poRepo.findById(id).orElseThrow();
    }

    @Autowired com.tsloms.server.repository.PurchaseOrderRepository poRepo;
}
