// 收尾域集成测试：授权/证据/案例/反馈/通知。
package com.tsloms.server;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.tsloms.server.auth.CaptchaService;
import static org.assertj.core.api.Assertions.assertThat;
import com.tsloms.server.model.FaultRecord;
import com.tsloms.server.model.PasswordHasher;
import com.tsloms.server.model.User;
import com.tsloms.server.repository.FaultRecordRepository;
import com.tsloms.server.repository.UserRepository;
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
class MiscDomainFlowTest {

    private static final String ADMIN = "misc_admin";
    private static final ObjectMapper JSON = new ObjectMapper();

    @Autowired MockMvc mvc;
    @Autowired CaptchaService captchaSvc;
    @Autowired UserRepository users;
    @Autowired FaultRecordRepository faults;
    @Autowired PasswordHasher hasher;
    @Autowired TransactionTemplate tx;

    private String adminToken() throws Exception {
        tx.executeWithoutResult(s -> {
            if (users.findByUsername(ADMIN).isEmpty()) {
                User u = new User();
                u.username = ADMIN;
                u.passwordHash = hasher.hash("Passw0rd!");
                u.role = "super_admin"; // 拥有 module:manage 等全部权限
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
    void 授权_试用开启与状态查询() throws Exception {
        String bearer = "Bearer " + adminToken();

        mvc.perform(post("/api/v1/license/trial/start/video")
                        .header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("试用已开启"));

        mvc.perform(get("/api/v1/license/status").header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.modules.video.unlocked").value(true));
    }

    @Test
    void 证据注入与查询() throws Exception {
        Long fid = tx.execute(s -> {
            FaultRecord f = new FaultRecord();
            f.deviceHwId = "misc-evi-hw";
            f.faultType = "lamp_off";
            f.faultLevel = "critical";
            f.firstSeen = Instant.now();
            f.lastSeen = Instant.now();
            f.status = "occurred";
            return faults.save(f).id;
        });
        String bearer = "Bearer " + adminToken();

        // 来源枚举
        mvc.perform(get("/api/v1/evidence/sources").header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data[0]").value("firmware"));

        // 注入
        mvc.perform(post("/api/v1/evidence/ingest")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fault_id\":" + fid + ","
                                + "\"source_type\":\"citizen\","
                                + "\"raw_data\":\"市民反映路口灯不亮\"}"))
                .andExpect(status().isOk());

        // 按故障查证据
        mvc.perform(get("/api/v1/faults/" + fid + "/evidence")
                        .header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.total")
                        .value(org.hamcrest.Matchers.greaterThanOrEqualTo(1)));
    }

    @Test
    void 复核确认_critical自动建单() throws Exception {
        Long fid = tx.execute(s -> {
            FaultRecord f = new FaultRecord();
            f.deviceHwId = "misc-review-hw";
            f.faultType = "lamp_off";
            f.faultLevel = "critical";
            f.recognitionStatus = "pending_review";
            f.firstSeen = Instant.now();
            f.lastSeen = Instant.now();
            f.status = "occurred";
            return faults.save(f).id;
        });
        String bearer = "Bearer " + adminToken();

        String res = mvc.perform(post("/api/v1/faults/" + fid + "/review")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"approve\":true}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.auto_dispatched").value(true))
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);

        assertThat(JSON.readTree(res).at("/data/work_order_id").asLong()).isPositive();
        assertThat(faults.findById(fid).orElseThrow().recognitionStatus)
                .isEqualTo("confirmed");
    }

    @Test
    void 反馈提交与状态更新() throws Exception {
        String bearer = "Bearer " + adminToken();

        mvc.perform(post("/api/v1/feedbacks").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"title\":\"路口灯闪\",\"intersection\":\"测试路口\","
                                + "\"reporter\":\"张三\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("反馈已提交"));

        mvc.perform(get("/api/v1/feedbacks").header("Authorization", bearer))
                .andExpect(status().isOk());
    }

    @Test
    void 案例_创建列表训练() throws Exception {
        String bearer = "Bearer " + adminToken();

        mvc.perform(post("/api/v1/fault-cases").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fault_type\":\"lamp_off\",\"expected_result\":\"lamp_off\","
                                + "\"device_hw_id\":\"case-hw\",\"judged_result\":\"lamp_off\"}"))
                .andExpect(status().isOk());

        mvc.perform(get("/api/v1/fault-cases").header("Authorization", bearer))
                .andExpect(status().isOk());

        mvc.perform(post("/api/v1/fault-cases/train").header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("训练完成"));
    }

    @Autowired com.tsloms.server.repository.NotificationRepository notifRepo;

    @Test
    void 通知_造广播_未读数_单条已读_全部已读() throws Exception {
        String bearer = "Bearer " + adminToken();

        // 造一条全员广播
        tx.executeWithoutResult(s -> {
            var n = new com.tsloms.server.model.Notification();
            n.userId = 0L;
            n.type = com.tsloms.server.model.NotificationTypes.SYSTEM;
            n.title = "系统维护通知";
            n.content = "测试广播";
            n.bizId = 0L;
            notifRepo.save(n);
        });

        // 未读数 >=1
        mvc.perform(get("/api/v1/notifications/unread-count")
                        .header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.unread")
                        .value(org.hamcrest.Matchers.greaterThanOrEqualTo(1)));

        // 单条标记已读（取列表第一条）
        String listRes = mvc.perform(get("/api/v1/notifications")
                        .header("Authorization", bearer))
                .andExpect(status().isOk())
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        Long nid = JSON.readTree(listRes).at("/data/list/0/id").asLong();
        assertThat(nid).isPositive();
        mvc.perform(put("/api/v1/notifications/" + nid + "/read")
                        .header("Authorization", bearer))
                .andExpect(status().isOk());

        // 全部已读
        mvc.perform(put("/api/v1/notifications/read-all")
                        .header("Authorization", bearer))
                .andExpect(status().isOk());

        // 已读后再查 → 0
        mvc.perform(get("/api/v1/notifications/unread-count")
                        .header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.unread").value(0));
    }
}
