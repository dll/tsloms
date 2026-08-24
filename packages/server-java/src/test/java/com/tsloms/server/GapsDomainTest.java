// 补齐域集成测试：设备列表/路口聚合/接入状态/模块列表/AI看板/巡检dimension/404语义。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.tsloms.server.auth.CaptchaService;
import com.tsloms.server.model.Device;
import com.tsloms.server.model.FaultRecord;
import com.tsloms.server.model.PasswordHasher;
import com.tsloms.server.model.User;
import com.tsloms.server.repository.DeviceRepository;
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
class GapsDomainTest {

    private static final String ADMIN = "gaps_admin";

    @Autowired MockMvc mvc;
    @Autowired CaptchaService captchaSvc;
    @Autowired UserRepository users;
    @Autowired DeviceRepository devices;
    @Autowired FaultRecordRepository faults;
    @Autowired PasswordHasher hasher;
    @Autowired TransactionTemplate tx;

    private String adminToken() throws Exception {
        tx.executeWithoutResult(s -> {
            if (users.findByUsername(ADMIN).isEmpty()) {
                User u = new User();
                u.username = ADMIN;
                u.passwordHash = hasher.hash("Passw0rd!");
                u.role = "super_admin";
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
        return "Bearer " + res.substring(i, res.indexOf('"', i));
    }

    private void seedDevice(String hw, String intersection, boolean online) {
        tx.executeWithoutResult(s -> {
            if (devices.findByHwId(hw).isPresent()) {
                return;
            }
            Device d = new Device();
            d.hwId = hw;
            d.intersection = intersection;
            d.onlineStatus = online;
            d.lastCheckinAt = online ? Instant.now() : null;
            devices.save(d);
        });
    }

    @Test
    void 设备列表_筛选与字段() throws Exception {
        seedDevice("gaps-hw-1", "琅琊路口", true);
        seedDevice("gaps-hw-2", "南谯路口", false);
        String bearer = "Bearer " + adminToken();

        mvc.perform(get("/api/v1/devices").header("Authorization", bearer)
                        .param("intersection", "琅琊"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.list[?(@.hw_id=='gaps-hw-1')]").exists())
                .andExpect(jsonPath("$.data.list[?(@.hw_id=='gaps-hw-2')]").isEmpty());

        mvc.perform(get("/api/v1/devices").header("Authorization", bearer)
                        .param("online_status", "false"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.list[?(@.hw_id=='gaps-hw-2')]").exists());

        // 详情
        mvc.perform(get("/api/v1/devices")
                        .header("Authorization", bearer)
                        .param("hw_id", "gaps-hw-1"))
                .andExpect(status().isOk());
    }

    @Test
    void 设备创建_更新_报废_删除链() throws Exception {
        String bearer = "Bearer " + adminToken();

        String res = mvc.perform(post("/api/v1/devices")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"hw_id\":\"gaps-crud-hw\",\"intersection\":\"新路口\"}"))
                .andExpect(status().isOk())
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        Long id = new com.fasterxml.jackson.databind.ObjectMapper()
                .readTree(res).at("/data/id").asLong();

        mvc.perform(post("/api/v1/devices/" + id + "/retire")
                        .header("Authorization", bearer))
                .andExpect(status().isOk());

        // 未报废不可删
        mvc.perform(delete("/api/v1/devices/" + id).header("Authorization", bearer))
                .andExpect(status().isBadRequest());

        mvc.perform(post("/api/v1/devices/" + id + "/restore")
                        .header("Authorization", bearer))
                .andExpect(status().isOk());

        // 重复硬件ID → 400
        mvc.perform(post("/api/v1/devices")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"hw_id\":\"gaps-crud-hw\"}"))
                .andExpect(status().isBadRequest());
    }

    @Test
    void 路口聚合_接入状态_模块列表() throws Exception {
        seedDevice("gaps-agg-hw", "聚合路口A", true);
        String bearer = "Bearer " + adminToken();

        mvc.perform(get("/api/v1/intersections").header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.list[?(@.name=='聚合路口A')].total").value(1));

        mvc.perform(get("/api/v1/access/status").header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.mqtt.subscribe").isNotEmpty())
                .andExpect(jsonPath("$.data.real_hardware.online_devices").isNumber());

        mvc.perform(get("/api/v1/modules").header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.modules[0].key").value("dashboard"));
    }

    @Test
    void AI看板_空批次结构() throws Exception {
        String bearer = "Bearer " + adminToken();
        mvc.perform(get("/api/v1/dashboard/ai-overview").header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.config.enabled").value(false))
                .andExpect(jsonPath("$.data.risk_distribution.low").value(0))
                .andExpect(jsonPath("$.data.batch_trend").isArray());
    }

    @Test
    void 巡检排行_dimension参数兼容() throws Exception {
        String bearer = "Bearer " + adminToken();
        mvc.perform(get("/api/v1/patrol/ranking").header("Authorization", bearer)
                        .param("dimension", "person"))
                .andExpect(status().isOk());
        mvc.perform(get("/api/v1/patrol/ranking").header("Authorization", bearer)
                        .param("dimension", "device"))
                .andExpect(status().isOk());
    }

    @Test
    void 未映射接口_返回404而非500() throws Exception {
        String bearer = "Bearer " + adminToken();
        mvc.perform(get("/api/v1/definitely-not-exist")
                        .header("Authorization", bearer))
                .andExpect(status().isNotFound());
    }
}
