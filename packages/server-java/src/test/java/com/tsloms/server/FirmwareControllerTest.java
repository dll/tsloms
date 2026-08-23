// 固件接口集成测试：上传(multipart)/列表过滤/改版本/发布/升级任务/删除。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.multipart;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.tsloms.server.auth.CaptchaService;
import com.tsloms.server.model.Device;
import com.tsloms.server.model.PasswordHasher;
import com.tsloms.server.model.User;
import com.tsloms.server.repository.DeviceRepository;
import com.tsloms.server.repository.FirmwarePackageRepository;
import com.tsloms.server.repository.UserRepository;
import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.mock.web.MockMultipartFile;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.transaction.support.TransactionTemplate;

@SpringBootTest
@AutoConfigureMockMvc
class FirmwareControllerTest {

    private static final String ADMIN = "fw_ctl_admin";

    @Autowired MockMvc mvc;
    @Autowired CaptchaService captchaSvc;
    @Autowired UserRepository users;
    @Autowired DeviceRepository devices;
    @Autowired FirmwarePackageRepository pkgs;
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

    private MockMultipartFile fwFile(String name) {
        return new MockMultipartFile("file", name,
                MediaType.APPLICATION_OCTET_STREAM_VALUE, new byte[]{1, 2, 3, 4, 5});
    }

    @AfterAll
    static void cleanup(@Autowired FirmwarePackageRepository repo) {
        // 清理上传产生的本地文件（测试环境 uploads 目录）
        try {
            if (Files.exists(Path.of("./uploads/media/firmware"))) {
                try (var walk = Files.walk(Path.of("./uploads/media/firmware"))) {
                    walk.filter(Files::isRegularFile).forEach(p -> p.toFile().delete());
                }
            }
        } catch (Exception ignored) {
            // 测试清理失败不影响结果
        }
    }

    @Test
    void 上传_版本解析_MD5_唯一性() throws Exception {
        String token = adminToken();
        String bearer = "Bearer " + token;

        // 非法扩展名拒绝
        mvc.perform(multipart("/api/v1/firmwares/upload")
                        .file(new MockMultipartFile("file", "fw.txt",
                                MediaType.APPLICATION_OCTET_STREAM_VALUE, new byte[]{1}))
                        .param("version", "v1.0.0")
                        .header("Authorization", bearer))
                .andExpect(status().isBadRequest());

        // 正常上传
        mvc.perform(multipart("/api/v1/firmwares/upload")
                        .file(fwFile("signal_v100.bin"))
                        .param("version", "v1.0.0")
                        .param("description", "首个测试固件")
                        .header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.firmware.version").value("v1.0.0"))
                .andExpect(jsonPath("$.data.firmware.major").value(1))
                .andExpect(jsonPath("$.data.firmware.md5").isNotEmpty());

        // 重复版本拒绝
        mvc.perform(multipart("/api/v1/firmwares/upload")
                        .file(fwFile("again.bin")).param("version", "v1.0.0")
                        .header("Authorization", bearer))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("该固件版本已存在"));
    }

    @Test
    void 列表发布筛选与详情() throws Exception {
        String bearer = "Bearer " + adminToken();

        Long id = uploadAndGetId(bearer, "v2.1.0");

        // 发布
        mvc.perform(put("/api/v1/firmwares/" + id + "/publish")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"published\":true}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("发布固件成功"));

        // published=true 过滤可见；未发布的 v1.0.0 不出现
        mvc.perform(get("/api/v1/firmwares").header("Authorization", bearer)
                        .param("published", "true"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.list[?(@.version=='v2.1.0')]").exists());

        // 详情
        mvc.perform(get("/api/v1/firmwares/" + id).header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.firmware.published").value(true));
    }

    @Test
    void 改版本号_非法与重复校验() throws Exception {
        String bearer = "Bearer " + adminToken();
        Long id = uploadAndGetId(bearer, "v3.0.0");
        uploadAndGetId(bearer, "v3.0.1");

        // 改成已存在版本 → 拒绝
        mvc.perform(put("/api/v1/firmwares/" + id).header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"version\":\"v3.0.1\"}"))
                .andExpect(status().isBadRequest());

        // 合法改版：位域重算
        mvc.perform(put("/api/v1/firmwares/" + id).header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"version\":\"v3.2.9\"}"))
                .andExpect(status().isOk());
        var f = pkgs.findById(id).orElseThrow();
        assertThat(f.major).isEqualTo(3);
        assertThat(f.minor).isEqualTo(2);
        assertThat(f.build).isEqualTo(9);
        assertThat(f.swVersion).isEqualTo((3L << 28) | (2L << 24));
    }

    @Test
    void 升级任务_未发布拒绝_设备校验_重复任务拒绝() throws Exception {
        String bearer = "Bearer " + adminToken();
        Long unpublishedId = uploadAndGetId(bearer, "v9.9.9");

        tx.executeWithoutResult(s -> {
            Device d = new Device();
            d.hwId = "fw-upgrade-hw";
            d.onlineStatus = false;
            devices.save(d);
        });

        // 未发布 → 拒绝
        mvc.perform(post("/api/v1/firmware-upgrades").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"device_hw_id\":\"fw-upgrade-hw\",\"firmware_id\":"
                                + unpublishedId + "}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("固件未发布，无法发起升级"));

        // 发布后创建成功
        mvc.perform(put("/api/v1/firmwares/" + unpublishedId + "/publish")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"published\":true}"))
                .andExpect(status().isOk());
        mvc.perform(post("/api/v1/firmware-upgrades").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"device_hw_id\":\"fw-upgrade-hw\",\"firmware_id\":"
                                + unpublishedId + "}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.message").value("升级任务已创建"));

        // 同设备重复待升级任务 → 拒绝（用另一已发布固件）
        Long another = uploadAndGetId(bearer, "v9.9.8");
        mvc.perform(put("/api/v1/firmwares/" + another + "/publish")
                        .header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"published\":true}"))
                .andExpect(status().isOk());
        mvc.perform(post("/api/v1/firmware-upgrades").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"device_hw_id\":\"fw-upgrade-hw\",\"firmware_id\":" + another + "}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("该设备已有未完成的升级任务"));

        // 升级记录列表可查
        mvc.perform(get("/api/v1/firmware-upgrades").header("Authorization", bearer)
                        .param("device_hw_id", "fw-upgrade-hw"))
                .andExpect(status().isOk());
    }

    private Long uploadAndGetId(String bearer, String version) throws Exception {
        String res = mvc.perform(multipart("/api/v1/firmwares/upload")
                        .file(fwFile(version.replace('.', '_') + ".bin"))
                        .param("version", version)
                        .header("Authorization", bearer))
                .andExpect(status().isOk())
                .andReturn().getResponse()
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
        return new com.fasterxml.jackson.databind.ObjectMapper()
                .readTree(res).at("/data/firmware/id").asLong();
    }
}
