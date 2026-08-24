// 巡检模块流程集成测试：建任务→执行→记录/排行→自检上报。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.tsloms.server.auth.CaptchaService;
import com.tsloms.server.model.Device;
import com.tsloms.server.model.PasswordHasher;
import com.tsloms.server.model.User;
import com.tsloms.server.repository.DeviceRepository;
import com.tsloms.server.repository.PatrolRecordRepository;
import com.tsloms.server.repository.PatrolTaskRepository;
import com.tsloms.server.repository.UserRepository;
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
class PatrolControllerTest {

    private static final String ADMIN = "patrol_admin";
    private static final ObjectMapper JSON = new ObjectMapper();

    @Autowired MockMvc mvc;
    @Autowired CaptchaService captchaSvc;
    @Autowired UserRepository users;
    @Autowired DeviceRepository devices;
    @Autowired PatrolTaskRepository tasks;
    @Autowired PatrolRecordRepository records;
    @Autowired PasswordHasher hasher;
    @Autowired TransactionTemplate tx;

    private String adminToken() throws Exception {
        tx.executeWithoutResult(s -> {
            if (users.findByUsername(ADMIN).isEmpty()) {
                User u = new User();
                u.username = ADMIN;
                u.passwordHash = hasher.hash("Passw0rd!");
                u.role = "admin"; // 拥有 patrol:manage/run/selfcheck
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

    private void seedDevices() {
        tx.executeWithoutResult(s -> {
            for (int i = 0; i < 3; i++) {
                String hw = "patrolhw" + i + UUID.randomUUID().toString().substring(0, 4);
                Device d = devices.findByHwId(hw).orElseGet(() -> {
                    Device nd = new Device();
                    nd.hwId = hw;
                    nd.onlineStatus = true;
                    nd.lastCheckinAt = Instant.now(); // 在线 → normal
                    return devices.save(nd);
                });
            }
        });
    }

    @Test
    void 任务创建_执行_记录与排行() throws Exception {
        seedDevices();
        String bearer = "Bearer " + adminToken();

        // 创建 random 模式任务（目标 2 台）
        mvc.perform(post("/api/v1/patrol/tasks").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"name\":\"夜间抽检\",\"mode\":\"random\",\"target_count\":2}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("巡检任务已创建"));

        Long taskId = tasks.findAll().stream()
                .filter(t -> "夜间抽检".equals(t.name)).findFirst().orElseThrow().id;

        // 详情
        mvc.perform(get("/api/v1/patrol/tasks/" + taskId).header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.name").value("夜间抽检"))
                .andExpect(jsonPath("$.data.status").value("planned"));

        // 执行（共享库下可能圈到其他用例的离线设备，abnormal 不做精确断言）
        mvc.perform(post("/api/v1/patrol/tasks/" + taskId + "/run")
                        .header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.checked").value(2));

        var t = tasks.findById(taskId).orElseThrow();
        assertThat(t.runCount).isEqualTo(1);
        assertThat(t.lastRunAt).isNotNull();
        assertThat(t.status).isEqualTo("done");

        // 记录列表按任务过滤
        mvc.perform(get("/api/v1/patrol/records").header("Authorization", bearer)
                        .param("task_id", String.valueOf(taskId)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.total").value(2));

        // 排行
        mvc.perform(get("/api/v1/patrol/ranking").header("Authorization", bearer)
                        .param("group_by", "device"))
                .andExpect(status().isOk());

        // 删除任务
        mvc.perform(delete("/api/v1/patrol/tasks/" + taskId).header("Authorization", bearer))
                .andExpect(status().isOk());
    }

    @Test
    void 自检上报_正常与异常() throws Exception {
        String bearer = "Bearer " + adminToken();

        mvc.perform(post("/api/v1/patrol/selfcheck").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"device_hw_id\":\"selfcheck-hw\",\"result\":\"normal\","
                                + "\"detail\":\"三色全亮正常\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.result").value("normal"));

        mvc.perform(post("/api/v1/patrol/selfcheck").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"device_hw_id\":\"selfcheck-hw\",\"result\":\"abnormal\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.result").value("abnormal"));

        long abnormalCnt = records.findAll().stream()
                .filter(r -> "selfcheck-hw".equals(r.deviceHwId)
                        && "abnormal".equals(r.checkResult))
                .count();
        assertThat(abnormalCnt).isGreaterThanOrEqualTo(1);
    }
}
