// 故障+工单核心业务闭环集成测试：确认→派单→处理→完成→故障解决，及各校验分支。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.tsloms.server.auth.CaptchaService;
import com.tsloms.server.model.Device;
import com.tsloms.server.model.FaultRecord;
import com.tsloms.server.model.PasswordHasher;
import com.tsloms.server.model.User;
import com.tsloms.server.model.WorkOrder;
import com.tsloms.server.repository.DeviceRepository;
import com.tsloms.server.repository.FaultRecordRepository;
import com.tsloms.server.repository.UserRepository;
import com.tsloms.server.repository.WorkOrderRepository;
import java.time.Instant;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.transaction.support.TransactionTemplate;

@SpringBootTest
@AutoConfigureMockMvc
class FaultWorkOrderFlowTest {

    private static final String ADMIN = "fw_admin";
    @Autowired MockMvc mvc;
    @Autowired CaptchaService captchaSvc;
    @Autowired UserRepository users;
    @Autowired DeviceRepository devices;
    @Autowired FaultRecordRepository faults;
    @Autowired WorkOrderRepository orders;
    @Autowired PasswordHasher hasher;
    @Autowired TransactionTemplate tx;

    private void ensureUser(String username, String role) {
        tx.executeWithoutResult(s -> {
            if (users.findByUsername(username).isEmpty()) {
                User u = new User();
                u.username = username;
                u.passwordHash = hasher.hash("Passw0rd!");
                u.role = role;
                u.status = "enabled";
                users.save(u);
            }
        });
    }

    private String loginAs(String username) throws Exception {
        String res = mvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(TestSupport.login(captchaSvc, username, "Passw0rd!")))
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        int i = res.indexOf("\"token\":\"") + "\"token\":\"".length();
        return res.substring(i, res.indexOf('"', i));
    }

    private String adminToken() throws Exception {
        ensureUser(ADMIN, "admin");
        return loginAs(ADMIN);
    }

    /** 造一台设备与一条 occurred 故障，返回故障 ID。 */
    private Long seedFault(String hwId, String type) {
        return tx.execute(s -> {
            Device d = devices.findByHwId(hwId).orElseGet(() -> {
                Device nd = new Device();
                nd.hwId = hwId;
                nd.intersection = "测试路口";
                nd.onlineStatus = true;
                return devices.save(nd);
            });
            assertThat(d.id).isNotNull();
            FaultRecord f = new FaultRecord();
            f.deviceHwId = hwId;
            f.faultType = type;
            f.faultLevel = "critical";
            f.errCode = (short) -2;
            f.firstSeen = Instant.now();
            f.lastSeen = Instant.now();
            f.status = "occurred";
            return faults.save(f).id;
        });
    }

    @Test
    void 故障列表筛选_active兼容语义() throws Exception {
        Long fid = seedFault("flt-list-hw", "lamp_off");
        String token = adminToken();

        mvc.perform(get("/api/v1/faults").header("Authorization", "Bearer " + token)
                        .param("hw_id", "flt-list-hw").param("status", "active"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.list[?(@.id==" + fid + ")]").exists());

        mvc.perform(get("/api/v1/faults").header("Authorization", "Bearer " + token)
                        .param("hw_id", "flt-list-hw").param("status", "resolved"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.list[?(@.id==" + fid + ")]").isEmpty());
    }

    @Test
    void 确认故障_设负责人自动推进confirmed() throws Exception {
        Long fid = seedFault("flt-cfm-hw", "dim");
        ensureUser("fw_owner", "operator");
        Long ownerId = users.findByUsername("fw_owner").orElseThrow().id;
        String token = adminToken();

        mvc.perform(put("/api/v1/faults/" + fid).header("Authorization", "Bearer " + token)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"owner_id\":" + ownerId + "}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.fault.status").value("confirmed"))
                .andExpect(jsonPath("$.data.fault.owner_name").value("fw_owner"));
        assertThat(faults.findById(fid).orElseThrow().confirmedAt).isNotNull();
    }

    @Test
    void 确认故障_无效状态400() throws Exception {
        Long fid = seedFault("flt-bad-status", "dim");
        String token = adminToken();
        mvc.perform(put("/api/v1/faults/" + fid).header("Authorization", "Bearer " + token)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"status\":\"frozen\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("无效的故障状态"));
    }

    @Test
    void 派单闭环_建单指派处理完成故障解决() throws Exception {
        Long fid = seedFault("flt-flow-hw", "lamp_off");
        ensureUser("fw_repairer", "operator");
        Long repairerId = users.findByUsername("fw_repairer").orElseThrow().id;
        String token = adminToken();
        String bearer = "Bearer " + token;

        // 1) 派单：新建 processing 工单，故障推进 dispatched
        String res = mvc.perform(post("/api/v1/faults/" + fid + "/dispatch")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"assignee_id\":" + repairerId + "}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.fault.status").value("dispatched"))
                .andExpect(jsonPath("$.data.work_order.status").value("processing"))
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        Long woId = new com.fasterxml.jackson.databind.ObjectMapper()
                .readTree(res).at("/data/work_order/id").asLong();

        // 工单编号格式 WOyyyyMMddNNNN
        String orderNo = new com.fasterxml.jackson.databind.ObjectMapper()
                .readTree(res).at("/data/work_order/order_no").asText();
        assertThat(orderNo).startsWith("WO").hasSize(14);

        // 2) 工单完成：closed_at 记录、活跃位释放、故障联动 resolved
        mvc.perform(put("/api/v1/work-orders/" + woId + "/status")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"status\":\"completed\",\"result\":\"已更换灯组\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.work_order.status").value("completed"));

        FaultRecord f = faults.findById(fid).orElseThrow();
        assertThat(f.status).isEqualTo("resolved");
        assertThat(f.resolvedAt).isNotNull();
        WorkOrder wo = orders.findById(woId).orElseThrow();
        assertThat(wo.closedAt).isNotNull();
        assertThat(wo.faultActiveScope).isNull();
        assertThat(wo.result).isEqualTo("已更换灯组");

        // 3) 删除已解决故障（无未完成工单）成功
        mvc.perform(delete("/api/v1/faults/" + fid).header("Authorization", bearer))
                .andExpect(status().isOk());
        assertThat(faults.findById(fid)).isEmpty();
    }

    @Test
    void 派单给viewer_400() throws Exception {
        Long fid = seedFault("flt-viewer-dispatch", "dim");
        ensureUser("fw_plain_viewer", "viewer");
        Long vid = users.findByUsername("fw_plain_viewer").orElseThrow().id;
        String token = adminToken();

        mvc.perform(post("/api/v1/faults/" + fid + "/dispatch")
                        .header("Authorization", "Bearer " + token)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"assignee_id\":" + vid + "}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("只能指派给运维人员或管理员"));
    }

    @Test
    void 有未完成工单的故障_禁止删除() throws Exception {
        Long fid = seedFault("flt-open-wo", "power_loss");
        String token = adminToken();
        String bearer = "Bearer " + token;

        mvc.perform(post("/api/v1/faults/" + fid + "/dispatch")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"assignee_id\":" + users.findByUsername(ADMIN).orElseThrow().id + "}"))
                .andExpect(status().isOk());

        mvc.perform(delete("/api/v1/faults/" + fid).header("Authorization", bearer))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value(
                        "该故障存在未完成工单，请先完结或删除关联工单后再删除故障"));
    }

    @Test
    void 手动建单_驳回重激活流转() throws Exception {
        Long fid = seedFault("flt-manual-wo", "timeout");
        ensureUser("fw_op2", "operator");
        Long opId = users.findByUsername("fw_op2").orElseThrow().id;
        String token = adminToken();
        String bearer = "Bearer " + token;

        // 手动创建 pending 工单
        String res = mvc.perform(post("/api/v1/work-orders").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fault_id\":" + fid + ",\"device_hw_id\":\"flt-manual-wo\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.work_order.status").value("pending"))
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        Long woId = new com.fasterxml.jackson.databind.ObjectMapper()
                .readTree(res).at("/data/work_order/id").asLong();

        // 指派 → processing；故障推进 dispatched
        mvc.perform(put("/api/v1/work-orders/" + woId + "/assign")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"assignee_id\":" + opId + "}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.work_order.status").value("processing"));
        assertThat(faults.findById(fid).orElseThrow().status).isEqualTo("dispatched");

        // 驳回 → 释放活跃位
        mvc.perform(put("/api/v1/work-orders/" + woId + "/status")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"status\":\"rejected\"}"))
                .andExpect(status().isOk());
        assertThat(orders.findById(woId).orElseThrow().faultActiveScope).isNull();

        // rejected → pending 重新激活：清空关闭时间、占据活跃位、故障回 confirmed
        mvc.perform(put("/api/v1/work-orders/" + woId + "/status")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"status\":\"pending\"}"))
                .andExpect(status().isOk());
        WorkOrder wo = orders.findById(woId).orElseThrow();
        assertThat(wo.closedAt).isNull();
        assertThat(wo.faultActiveScope).isEqualTo(fid);
        assertThat(faults.findById(fid).orElseThrow().status).isEqualTo("confirmed");

        // 详情：SLA stage 与时间线
        mvc.perform(get("/api/v1/work-orders/" + woId).header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.sla.stage").value("待处理"))
                .andExpect(jsonPath("$.data.sla.deadline_hours").value(24))
                .andExpect(jsonPath("$.data.timeline").isArray());

        // 删除工单：解除故障关联但保留故障
        mvc.perform(delete("/api/v1/work-orders/" + woId).header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("工单已删除"));
        assertThat(faults.findById(fid)).isPresent();
        assertThat(orders.findById(woId)).isEmpty();
    }

    @Test
    void 故障详情_含设备与工单摘要() throws Exception {
        Long fid = seedFault("flt-detail-hw", "abnormal_on");
        String token = adminToken();

        // 先派单生成关联工单
        ensureUser("fw_detail_op", "operator");
        mvc.perform(post("/api/v1/faults/" + fid + "/dispatch")
                        .header("Authorization", "Bearer " + token)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"assignee_id\":"
                                + users.findByUsername("fw_detail_op").orElseThrow().id + "}"))
                .andExpect(status().isOk());

        mvc.perform(get("/api/v1/faults/" + fid).header("Authorization", "Bearer " + token))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.fault.device_hw_id").value("flt-detail-hw"))
                .andExpect(jsonPath("$.data.device.hw_id").value("flt-detail-hw"))
                .andExpect(jsonPath("$.data.device.intersection").value("测试路口"))
                .andExpect(jsonPath("$.data.work_order.status").value("processing"));
    }

    @Test
    void 派单复用_未完成工单不重复建单() throws Exception {
        Long fid = seedFault("flt-reuse-hw", "dim");
        ensureUser("fw_reuse_op", "operator");
        Long opId = users.findByUsername("fw_reuse_op").orElseThrow().id;
        String token = adminToken();
        String body = "{\"assignee_id\":" + opId + "}";

        String r1 = mvc.perform(post("/api/v1/faults/" + fid + "/dispatch")
                        .header("Authorization", "Bearer " + token)
                        .contentType(MediaType.APPLICATION_JSON).content(body))
                .andExpect(status().isOk()).andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        String r2 = mvc.perform(post("/api/v1/faults/" + fid + "/dispatch")
                        .header("Authorization", "Bearer " + token)
                        .contentType(MediaType.APPLICATION_JSON).content(body))
                .andExpect(status().isOk()).andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);

        var om = new com.fasterxml.jackson.databind.ObjectMapper();
        Long wo1 = om.readTree(r1).at("/data/work_order/id").asLong();
        Long wo2 = om.readTree(r2).at("/data/work_order/id").asLong();
        assertThat(wo2).isEqualTo(wo1); // 复用同一工单
        long countForFault = orders.findAll().stream()
                .filter(w -> Long.valueOf(fid).equals(w.faultId)).count();
        assertThat(countForFault).isEqualTo(1);
    }

    @Test
    void 更新故障_无可更新字段400_维修人置空() throws Exception {
        Long fid = seedFault("flt-null-repairer", "dim");
        ensureUser("fw_null_rep", "operator");
        Long repId = users.findByUsername("fw_null_rep").orElseThrow().id;
        String token = adminToken();
        String bearer = "Bearer " + token;

        // 先设置维修人
        mvc.perform(put("/api/v1/faults/" + fid).header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"repairer_id\":" + repId + "}"))
                .andExpect(status().isOk());
        assertThat(faults.findById(fid).orElseThrow().repairerId).isEqualTo(repId);

        // repairer_id=0 → 置空
        mvc.perform(put("/api/v1/faults/" + fid).header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"repairer_id\":0}"))
                .andExpect(status().isOk());
        assertThat(faults.findById(fid).orElseThrow().repairerId).isNull();

        // 空更新体 → 400
        mvc.perform(put("/api/v1/faults/" + fid).header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("无可更新字段"));

        // 不存在的负责人 → 404
        mvc.perform(put("/api/v1/faults/" + fid).header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"owner_id\":999999}"))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.msg").value("负责人不存在"));
    }

    @Test
    void CSV导出_带BOM与表头() throws Exception {
        seedFault("flt-csv-hw", "lamp_off");
        String token = adminToken();
        var result = mvc.perform(get("/api/v1/faults/export")
                        .header("Authorization", "Bearer " + token))
                .andExpect(status().isOk())
                .andReturn();
        byte[] body = result.getResponse().getContentAsByteArray();
        assertThat(body[0]).isEqualTo((byte) 0xEF);
        assertThat(body[1]).isEqualTo((byte) 0xBB);
        assertThat(body[2]).isEqualTo((byte) 0xBF);
        String head = new String(body, 3, Math.min(60, body.length - 3),
                java.nio.charset.StandardCharsets.UTF_8);
        assertThat(head).startsWith("ID,设备硬件ID");
    }
}
