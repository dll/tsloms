// 路口+预警接口集成测试（覆盖 CRUD/树/忽略/转工单/自动忽略）。
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
import com.tsloms.server.model.PasswordHasher;
import com.tsloms.server.model.User;
import com.tsloms.server.repository.UserRepository;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.transaction.support.TransactionTemplate;

@SpringBootTest
@AutoConfigureMockMvc
class CrossingWarningControllerTest {

    private static final String ADMIN = "cw_admin";
    private static final ObjectMapper JSON = new ObjectMapper();

    @Autowired MockMvc mvc;
    @Autowired CaptchaService captchaSvc;
    @Autowired UserRepository users;
    @Autowired PasswordHasher hasher;
    @Autowired TransactionTemplate tx;

    private String adminToken() throws Exception {
        tx.executeWithoutResult(s -> {
            if (users.findByUsername(ADMIN).isEmpty()) {
                User u = new User();
                u.username = ADMIN;
                u.passwordHash = hasher.hash("Passw0rd!");
                u.role = "admin"; // admin 拥有 crossing:manage / area:manage / warning:manage
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
    void 路口CRUD_设备列表_区划树() throws Exception {
        String bearer = "Bearer " + adminToken();

        // 创建区划：省 → 区
        mvc.perform(post("/api/v1/areas").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"name\":\"安徽省\",\"area_type\":\"province\",\"code\":340000}"))
                .andExpect(status().isOk());
        Long provinceId = firstAreaId(bearer);
        mvc.perform(post("/api/v1/areas").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"name\":\"琅琊区\",\"area_type\":\"district\","
                                + "\"code\":\"340103\",\"parent_id\":" + provinceId + "}"))
                .andExpect(status().isOk());

        // 区划树
        mvc.perform(get("/api/v1/areas/tree").header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.total").value(2));

        // 创建路口（挂省）
        String crRes = mvc.perform(post("/api/v1/crossings")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"name\":\"胜利路口\",\"point_no\":\"P001\","
                                + "\"type\":\"1\",\"province_id\":" + provinceId
                                + ",\"lat\":32.3,\"lng\":118.3}"))
                .andExpect(status().isOk())
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        Long crossingId = JSON.readTree(crRes).at("/data/id").asLong();
        assertThat(crossingId).isPositive();

        // 列表筛选
        mvc.perform(get("/api/v1/crossings").header("Authorization", bearer)
                        .param("keyword", "胜利"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.total").value(1));

        // 详情含 devices 数组
        mvc.perform(get("/api/v1/crossings/" + crossingId)
                        .header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.devices").isArray());

        // 更新
        mvc.perform(put("/api/v1/crossings/" + crossingId)
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"status\":\"maintain\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("路口更新成功"));

        // 有子级的区划不可删
        mvc.perform(delete("/api/v1/areas/" + provinceId).header("Authorization", bearer))
                .andExpect(status().isBadRequest());
    }

    private Long firstAreaId(String bearer) throws Exception {
        String res = mvc.perform(get("/api/v1/areas/tree").header("Authorization", bearer))
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        return JSON.readTree(res).at("/data/tree/0/id").asLong();
    }

    @Test
    void 预警列表_忽略_批量忽略_自动忽略() throws Exception {
        String bearer = "Bearer " + adminToken();

        // 造两条预警（直接经仓库，模拟 MQTT 研判生成）
        tx.executeWithoutResult(s -> {
            for (int i = 1; i <= 2; i++) {
                com.tsloms.server.model.Warning w = new com.tsloms.server.model.Warning();
                w.deviceHwId = "warn-hw-" + i;
                w.warningCode = -i;
                w.warningLabel = "lamp_off";
                w.level = "critical";
                w.source = "fault";
                w.dealState = "unhandled";
                w.status = "untransferred";
                w.occurredAt = java.time.Instant.now();
                warningsRepo().save(w);
            }
        });

        Long id1 = latestWarningId("warn-hw-1");
        Long id2 = latestWarningId("warn-hw-2");

        // 单条忽略
        mvc.perform(post("/api/v1/warnings/" + id1 + "/ignore")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"reason\":\"测试忽略\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("预警已忽略"));

        // 批量忽略
        mvc.perform(post("/api/v1/warnings/batch-ignore")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"ids\":[" + id2 + "],\"reason\":\"批量\"}"))
                .andExpect(status().isOk());

        // 自动忽略：未处理的已被上面处理，返回 ignored=0 也合法；再造一条验证
        Long id3 = seedOne("warn-hw-3");
        mvc.perform(post("/api/v1/warnings/auto-ignore")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"ids\":[" + id3 + "]}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.ignored").value(1));

        // 筛选 unhandled 应为 0
        mvc.perform(get("/api/v1/warnings").header("Authorization", bearer)
                        .param("deal_state", "unhandled"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.total").value(0));
    }

    @Test
    void 预警转工单() throws Exception {
        Long id = seedOne("warn-to-wo-hw");
        String bearer = "Bearer " + adminToken();

        String res = mvc.perform(post("/api/v1/warnings/" + id + "/to-workorder")
                        .header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.order_no")
                        .value(org.hamcrest.Matchers.startsWith("WO")))
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);

        // 已转移的再转 → 400
        mvc.perform(post("/api/v1/warnings/" + id + "/to-workorder")
                        .header("Authorization", bearer))
                .andExpect(status().isBadRequest());
        assertThat(JSON.readTree(res).at("/data/work_order_id").asLong()).isPositive();

        // CSV 导出可达
        mvc.perform(get("/api/v1/warnings/export").header("Authorization", bearer))
                .andExpect(status().isOk());
    }

    @Autowired com.tsloms.server.repository.WarningRepository warningRepo;

    private com.tsloms.server.repository.WarningRepository warningsRepo() {
        return warningRepo;
    }

    private Long latestWarningId(String hw) {
        return warningRepo.findAll().stream()
                .filter(w -> hw.equals(w.deviceHwId))
                .map(w -> w.id)
                .max(Long::compare).orElseThrow();
    }

    private Long seedOne(String hw) {
        return tx.execute(s -> {
            var w = new com.tsloms.server.model.Warning();
            w.deviceHwId = hw;
            w.warningCode = -5;
            w.warningLabel = "abnormal_on";
            w.level = "warning";
            w.source = "fault";
            w.dealState = "unhandled";
            w.status = "untransferred";
            w.occurredAt = java.time.Instant.now();
            return warningRepo.save(w).id;
        });
    }
}
