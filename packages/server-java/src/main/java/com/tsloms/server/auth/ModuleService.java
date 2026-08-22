// 模块目录与服务：核心/可选模块清单对齐 Go 版 handler/module.go。
package com.tsloms.server.auth;

import com.tsloms.server.repository.ModuleToggleRepository;
import java.util.ArrayList;
import java.util.List;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class ModuleService {

    /** 核心模块：始终启用，不可关闭。 */
    private static final List<String> CORE = List.of(
            "dashboard", "device", "intersection", "map", "feedback",
            "fault", "workorder", "firmware", "log", "settings");

    /** 可选模块：默认关闭，由 module_toggles 表控制。 */
    private static final List<String> OPTIONAL = List.of(
            "video", "inventory", "purchase", "expense",
            "supplier", "ai", "dispatch", "notification");

    private final ModuleToggleRepository toggles;

    public ModuleService(ModuleToggleRepository toggles) {
        this.toggles = toggles;
    }

    /**
     * 已启用模块 key 列表：核心恒在 + 可选中已启用的（对齐 Go 版 EnabledModuleList）。
     */
    @Transactional(readOnly = true)
    public List<String> enabledModuleList() {
        List<String> out = new ArrayList<>(CORE);
        toggles.findAllByEnabledTrue()
                .forEach(t -> {
                    if (OPTIONAL.contains(t.moduleKey) && !out.contains(t.moduleKey)) {
                        out.add(t.moduleKey);
                    }
                });
        return out;
    }
}
