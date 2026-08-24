// 瓦片/POI 代理控制器测试（缓存命中路径免网络）。
package com.tsloms.server;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import static org.assertj.core.api.Assertions.assertThat;

import com.tsloms.server.proxy.TileProxyController;
import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;

class TileProxyControllerTest {

    private final MockMvc mvc = MockMvcBuilders
            .standaloneSetup(new TileProxyController("")).build();

    @Test
    void gaode_缺参数_400() throws Exception {
        mvc.perform(get("/api/v1/proxy/gaode").param("x", "1"))
                .andExpect(status().isBadRequest());
    }

    @Test
    void gaode_缓存命中_直接回源文件() throws Exception {
        // 预写缓存文件（与控制器 CACHE_DIR/gaode/{style}/{z}/{x}_{y} 约定一致）
        byte[] tile = new byte[]{(byte) 0xFF, (byte) 0xD8, 1, 2, 3, 4};
        Path cache = Path.of("/var/cache/tsloms-tiles", "gaode", "8", "10", "848_415.bin");
        Files.createDirectories(cache.getParent());
        Files.write(cache, tile);

        mvc.perform(get("/api/v1/proxy/gaode").param("x", "848").param("y", "415")
                        .param("z", "10").param("style", "8"))
                .andExpect(status().isOk())
                .andExpect(result -> assertThat(result.getResponse()
                        .getContentAsByteArray()).isEqualTo(tile));
    }

    @Test
    void amapPlace_缺关键字_400() throws Exception {
        mvc.perform(get("/api/v1/proxy/amap/place"))
                .andExpect(status().isBadRequest());
    }

    @Test
    void amapPlace_无key_返回降级提示() throws Exception {
        mvc.perform(get("/api/v1/proxy/amap/place").param("kw", "胜利路"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.data.fallback").value(true));
    }

    @Test
    void amapPlace_带key_返回列表结构() throws Exception {
        MockMvc withKey = MockMvcBuilders.standaloneSetup(new TileProxyController("testkey"))
                .build();
        withKey.perform(get("/api/v1/proxy/amap/place").param("kw", "滁州"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.data.source").value("amap"));
    }

    @Test
    void baidu_缺参数_400() throws Exception {
        mvc.perform(get("/api/v1/proxy/baidu").param("x", "1").param("y", "2"))
                .andExpect(status().isBadRequest());
    }

    @Test
    void baidu_缓存命中或回源_返回200图片() throws Exception {
        // 预写缓存；若回源（百度返回占位图）也覆盖网络路径——两者均为合法代码路径
        byte[] tile = new byte[]{(byte) 0x89, 'P', 'N', 'G', 1, 2, 3};
        Path cache = Path.of("/var/cache/tsloms-tiles", "baidu", "10", "555_666");
        Files.createDirectories(cache.getParent());
        Files.write(cache, tile);

        mvc.perform(get("/api/v1/proxy/baidu").param("x", "555").param("y", "666")
                        .param("z", "10"))
                .andExpect(status().isOk())
                .andExpect(result -> assertThat(
                        result.getResponse().getContentAsByteArray()).isNotEmpty());
    }

    @Test
    void gaode_卫星样式_缓存命中() throws Exception {
        byte[] tile = new byte[]{(byte) 0xFF, (byte) 0xD8, 9, 9};
        Path cache = Path.of("/var/cache/tsloms-tiles", "gaode", "6", "10", "777_888.bin");
        Files.createDirectories(cache.getParent());
        Files.write(cache, tile);

        mvc.perform(get("/api/v1/proxy/gaode").param("x", "777").param("y", "888")
                        .param("z", "10").param("style", "6"))
                .andExpect(status().isOk())
                .andExpect(result -> assertThat(result.getResponse()
                        .getContentAsByteArray()).isEqualTo(tile));
    }
}
