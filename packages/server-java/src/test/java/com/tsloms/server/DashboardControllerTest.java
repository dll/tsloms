// 仪表盘接口集成测试：造数据 → 六个端点契约断言。
package com.tsloms.server;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
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
import java.time.temporal.ChronoUnit;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.transaction.support.TransactionTemplate;

@SpringBootTest
@AutoConfigureMockMvc
class DashboardControllerTest {

    @Autowired MockMvc mvc;
    @Autowired CaptchaService captchaSvc;
    @Autowired UserRepository users;
    @Autowired DeviceRepository devices;
    @Autowired FaultRecordRepository faults;
    @Autowired WorkOrderRepository orders;
    @Autowired PasswordHasher hasher;
    @Autowired TransactionTemplate tx;

    private String viewerToken() throws Exception {
        tx.executeWithoutResult(s -> {
            if (users.findByUsername("dash_viewer").isEmpty()) {
                User u = new User();
                u.username = "dash_viewer";
                u.passwordHash = hasher.hash("Passw0rd!");
                u.role = "viewer"; // 只读角色即可访问仪表盘
                u.status = "enabled";
                users.save(u);
            }
        });
        String res = mvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(TestSupport.login(captchaSvc, "dash_viewer", "Passw0rd!")))
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        int i = res.indexOf("\"token\":\"") + "\"token\":\"".length();
        return res.substring(i, res.indexOf('"', i));
    }

    /** 造统计样本：2 设备（1 在线）+ 3 故障（2 类型/今日 1 条/两设备）+ 2 工单（pending/completed）。 */
    private void seedSamples() {
        tx.executeWithoutResult(s -> {
            if (devices.findByHwId("hw-a").isEmpty()) {
                Device a = new Device();
                a.hwId = "hw-a";
                a.onlineStatus = true;
                devices.save(a);
                Device b = new Device();
                b.hwId = "hw-b";
                b.onlineStatus = false;
                devices.save(b);

                Instant now = Instant.now();
                fault("hw-a", "lamp_off", "occurred", now);                 // 今日
                fault("hw-a", "lamp_off", "resolved", now.minus(3, ChronoUnit.DAYS));
                fault("hw-b", "dim", "occurred", now.minus(40, ChronoUnit.DAYS)); // 窗口外

                WorkOrder w1 = new WorkOrder();
                w1.orderNo = "WO-TEST-0001";
                w1.faultId = 9001L;
                w1.deviceHwId = "hw-a";
                w1.status = "pending";
                w1.createdAt = now.minus(48, ChronoUnit.HOURS); // pending 超 SLA → 超时
                orders.save(w1);
                WorkOrder w2 = new WorkOrder();
                w2.orderNo = "WO-TEST-0002";
                w2.faultId = 9002L;
                w2.deviceHwId = "hw-a";
                w2.status = "completed";
                w2.createdAt = now.minus(5, ChronoUnit.DAYS);
                w2.closedAt = now.minus(4, ChronoUnit.DAYS); // 闭环 24h
                orders.save(w2);
            }
        });
    }

    private void fault(String hw, String type, String status, Instant firstSeen) {
        FaultRecord f = new FaultRecord();
        f.deviceHwId = hw;
        f.faultType = type;
        f.faultLevel = "normal";
        f.errCode = (short) -1;
        f.firstSeen = firstSeen;
        f.lastSeen = firstSeen;
        f.status = status;
        faults.save(f);
    }

    @Test
    void overview_指标齐全() throws Exception {
        seedSamples();
        mvc.perform(get("/api/v1/dashboard/overview")
                        .header("Authorization", "Bearer " + viewerToken()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                // 共享库下其他用例会追加设备，这里做一致性断言而非绝对值
                .andExpect(jsonPath("$.data.devices.total")
                        .value(org.hamcrest.Matchers.greaterThanOrEqualTo(2)))
                .andExpect(jsonPath("$.data.devices.online")
                        .value(org.hamcrest.Matchers.greaterThanOrEqualTo(1)))
                .andExpect(jsonPath("$.data.faults.active").isNumber())
                .andExpect(jsonPath("$.data.work_orders.pending")
                        .value(org.hamcrest.Matchers.greaterThanOrEqualTo(1)))
                .andExpect(jsonPath("$.data.work_orders.completed")
                        .value(org.hamcrest.Matchers.greaterThanOrEqualTo(1)))
                .andExpect(jsonPath("$.data.work_orders.overdue").value(1));
    }

    @Test
    void faultTypeStats_窗口过滤与计数() throws Exception {
        seedSamples();
        // 近 30 天：lamp_off×2（hw-a 两条），dim 在 40 天前被排除
        mvc.perform(get("/api/v1/dashboard/fault-type-stats")
                        .header("Authorization", "Bearer " + viewerToken())
                        .param("days", "30"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.days").value(30))
                .andExpect(jsonPath("$.data.stats[?(@.fault_type=='lamp_off')].count").value(2))
                .andExpect(jsonPath("$.data.stats[?(@.fault_type=='dim')]").isEmpty());
    }

    @Test
    void workOrderStats_状态与超时() throws Exception {
        seedSamples();
        mvc.perform(get("/api/v1/dashboard/work-order-stats")
                        .header("Authorization", "Bearer " + viewerToken()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.overdue").value(1))
                .andExpect(jsonPath("$.data.stats[?(@.status=='rejected')].count").value(0));
    }

    @Test
    void faultTrend_按日分组升序() throws Exception {
        seedSamples();
        mvc.perform(get("/api/v1/dashboard/fault-trend")
                        .header("Authorization", "Bearer " + viewerToken())
                        .param("dimension", "day").param("days", "7"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.dimension").value("day"))
                .andExpect(jsonPath("$.data.trend[0].count").value(1));
    }

    @Test
    void deviceFaultRank_topN() throws Exception {
        seedSamples();
        mvc.perform(get("/api/v1/dashboard/device-fault-rank")
                        .header("Authorization", "Bearer " + viewerToken())
                        .param("limit", "5").param("days", "30"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.rank[0].device_hw_id").value("hw-a"))
                .andExpect(jsonPath("$.data.rank[0].count").value(2));
    }

    @Test
    void avgClosure_均值计算() throws Exception {
        seedSamples();
        mvc.perform(get("/api/v1/dashboard/work-order-avg-closure")
                        .header("Authorization", "Bearer " + viewerToken()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.completed_count").value(1))
                .andExpect(jsonPath("$.data.avg_hours").isNumber());
    }

    @Test
    void 无token访问仪表盘_401() throws Exception {
        mvc.perform(get("/api/v1/dashboard/overview"))
                .andExpect(status().isUnauthorized());
    }
}
