// 媒体接口集成测试：上传(MIME嗅探)/举证路口必填/流登记/删除清理。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.multipart;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.tsloms.server.auth.CaptchaService;
import com.tsloms.server.model.Device;
import com.tsloms.server.model.PasswordHasher;
import com.tsloms.server.model.User;
import com.tsloms.server.repository.DeviceRepository;
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
class MediaControllerTest {

    private static final String ADMIN = "media_admin";

    @Autowired MockMvc mvc;
    @Autowired CaptchaService captchaSvc;
    @Autowired UserRepository users;
    @Autowired DeviceRepository devices;
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

    /** 最小合法 PNG（1x1）。 */
    private byte[] pngBytes() {
        return new byte[]{
                (byte) 0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A,
                0, 0, 0, 13, 'I', 'H', 'D', 'R',
                0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0,
                (byte) 0x1F, 0x15, (byte) 0xC4, (byte) 0x89,
                0, 0, 0, 10, 'I', 'D', 'A', 'T', 0x78, (byte) 0x9C, 0x63, 0, 1, 0, 0, 5, 0, 1,
                0x0D, 0x0A, 0x2D, (byte) 0xB4,
                0, 0, 0, 0, 'I', 'E', 'N', 'D', (byte) 0xAE, 0x42, 0x60, (byte) 0x82};
    }

    @Test
    void 举证上传_缺路口400_成功带缩略图() throws Exception {
        String bearer = "Bearer " + adminToken();

        // 缺路口 → 400
        mvc.perform(multipart("/api/v1/media/upload")
                        .file(new MockMultipartFile("file", "evidence.png",
                                MediaType.IMAGE_PNG_VALUE, pngBytes()))
                        .param("device_hw_id", "mediahw1")
                        .param("media_type", "evidence")
                        .header("Authorization", bearer))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("请填写路口名称（便于定位与派单）"));

        // 补齐路口 → 成功；图片缩略图为自身
        mvc.perform(multipart("/api/v1/media/upload")
                        .file(new MockMultipartFile("file", "evidence.png",
                                MediaType.IMAGE_PNG_VALUE, pngBytes()))
                        .param("device_hw_id", "mediahw1")
                        .param("media_type", "evidence")
                        .param("intersection", "胜利路与湖心路")
                        .param("light_color", "red")
                        .param("is_active_fault", "true")
                        .header("Authorization", bearer))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.media.category").value("photo"))
                .andExpect(jsonPath("$.data.media.source").value("upload"))
                .andExpect(jsonPath("$.data.media.thumbnail").isNotEmpty());
    }

    @Test
    void 伪装文件_MIME嗅探拒绝() throws Exception {
        String bearer = "Bearer " + adminToken();
        // 文本内容伪装成 .png：嗅探为 text/plain → 拒绝
        mvc.perform(multipart("/api/v1/media/upload")
                        .file(new MockMultipartFile("file", "fake.png",
                                MediaType.IMAGE_PNG_VALUE,
                                "this is not an image".getBytes()))
                        .param("device_hw_id", "mediahw2")
                        .param("intersection", "嗅探测试路口")
                        .header("Authorization", bearer))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.msg").value("文件内容与扩展名不符，已拒绝上传"));
    }

    @Test
    void 流登记_协议校验与RTSP提示() throws Exception {
        tx.executeWithoutResult(s -> {
            if (devices.findByHwId("mediastreamhw").isEmpty()) {
                Device d = new Device();
                d.hwId = "mediastreamhw";
                devices.save(d);
            }
        });
        String bearer = "Bearer " + adminToken();

        // 非法类型（举证不能走登记）
        mvc.perform(post("/api/v1/media/streams").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"device_hw_id\":\"mediastreamhw\","
                                + "\"media_type\":\"evidence\",\"url\":\"http://x/live\"}"))
                .andExpect(status().isBadRequest());

        // 非法协议
        mvc.perform(post("/api/v1/media/streams").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"device_hw_id\":\"mediastreamhw\","
                                + "\"media_type\":\"monitoring\",\"url\":\"ftp://x/live\"}"))
                .andExpect(status().isBadRequest());

        // RTSP 无兼容地址 → 成功但带 warning
        mvc.perform(post("/api/v1/media/streams").header("Authorization", bearer)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"device_hw_id\":\"mediastreamhw\","
                                + "\"media_type\":\"monitoring\","
                                + "\"url\":\"rtsp://192.168.1.10:554/stream\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.warning").exists())
                .andExpect(jsonPath("$.data.media.source").value("rtsp"));
    }

    @Test
    void 列表筛选与删除() throws Exception {
        String bearer = "Bearer " + adminToken();

        // 先传一张确保有数据
        mvc.perform(multipart("/api/v1/media/upload")
                        .file(new MockMultipartFile("file", "del.png",
                                MediaType.IMAGE_PNG_VALUE, pngBytes()))
                        .param("device_hw_id", "mediahwdel")
                        .param("media_type", "evidence")
                        .param("intersection", "删除测试路口")
                        .header("Authorization", bearer))
                .andExpect(status().isOk());

        mvc.perform(get("/api/v1/media").header("Authorization", bearer)
                        .param("device_hw_id", "mediahwdel"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.total").value(1));

        Long id = new com.fasterxml.jackson.databind.ObjectMapper()
                .readTree(mvc.perform(get("/api/v1/media").header("Authorization", bearer)
                                        .param("device_hw_id", "mediahwdel"))
                                .andReturn().getResponse()
                                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8))
                .at("/data/list/0/id").asLong();

        mvc.perform(delete("/api/v1/media/" + id).header("Authorization", bearer))
                .andExpect(status().isOk());
        mvc.perform(get("/api/v1/media").header("Authorization", bearer)
                        .param("device_hw_id", "mediahwdel"))
                .andExpect(jsonPath("$.data.total").value(0));
    }

    @AfterAll
    static void cleanup() {
        try (var walk = Files.walk(Path.of("./uploads/media"))) {
            walk.filter(Files::isRegularFile).forEach(p -> p.toFile().delete());
        } catch (Exception ignored) {
            // 目录不存在时忽略
        }
    }
}
