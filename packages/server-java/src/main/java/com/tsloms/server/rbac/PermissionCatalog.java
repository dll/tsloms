// 全量功能权限点字典（种子数据）：逐项对齐 Go 版 rbac.go AllPermissions。
// 注意：code/name/module/sort 必须与 Go 版完全一致——前端按 module 分组展示、按 code 判权。
package com.tsloms.server.rbac;

import java.util.List;

public final class PermissionCatalog {

    /** 权限点定义（code / name / module / sort）。 */
    public record PermDef(String code, String name, String module, int sort) {
    }

    /** 模块设置权限码（仅超级管理员可持有，硬约束见 RbacService）。 */
    public static final String MODULE_MANAGE = "module:manage";

    public static final List<PermDef> ALL = List.of(
            // 设备管理
            new PermDef("device:create", "设备-新建", PermModules.DEVICE, 1),
            new PermDef("device:update", "设备-编辑", PermModules.DEVICE, 2),
            new PermDef("device:delete", "设备-删除", PermModules.DEVICE, 3),
            // 路口管理
            new PermDef("intersection:update", "路口-重命名/定位", PermModules.INTERSECTION, 4),
            new PermDef("intersection:delete", "路口-清空", PermModules.INTERSECTION, 5),
            // 故障管理
            new PermDef("fault:update", "故障-更新/确认", PermModules.FAULT, 6),
            new PermDef("fault:dispatch", "故障-派单", PermModules.FAULT, 7),
            new PermDef("fault:delete", "故障-删除", PermModules.FAULT, 8),
            // 故障识别研判（只增不删既有权限码）
            new PermDef("fault:review", "故障-待确认复核", PermModules.FAULT, 8),
            new PermDef("faultcase:manage", "识别案例库-管理/训练", PermModules.FAULT, 9),
            new PermDef("evidence:ingest", "多源证据-写入/注入", PermModules.FAULT, 10),
            // 工单管理
            new PermDef("workorder:create", "工单-新建", PermModules.WORKORDER, 8),
            new PermDef("workorder:update", "工单-状态流转", PermModules.WORKORDER, 9),
            new PermDef("workorder:assign", "工单-指派", PermModules.WORKORDER, 10),
            new PermDef("workorder:delete", "工单-删除", PermModules.WORKORDER, 11),
            // 媒体
            new PermDef("media:upload", "媒体-上传", PermModules.MEDIA, 12),
            new PermDef("media:delete", "媒体-删除", PermModules.MEDIA, 13),
            // 固件
            new PermDef("firmware:manage", "固件-上传/编辑/发布", PermModules.FIRMWARE, 14),
            new PermDef("firmware:delete", "固件-删除", PermModules.FIRMWARE, 15),
            // 库存（物料档案 + 出入库）
            new PermDef("inventory:manage", "物料-档案/出入库", PermModules.INVENTORY, 16),
            new PermDef("inventory:delete", "物料-删除", PermModules.INVENTORY, 17),
            // 供应商
            new PermDef("supplier:manage", "供应商-新建/编辑", PermModules.SUPPLIER, 18),
            new PermDef("supplier:delete", "供应商-删除", PermModules.SUPPLIER, 19),
            // 采购
            new PermDef("purchase:manage", "采购-下单/收货/取消", PermModules.PURCHASE, 20),
            new PermDef("purchase:delete", "采购-删除", PermModules.PURCHASE, 21),
            // 维修费用
            new PermDef("expense:manage", "费用-登记/确认", PermModules.EXPENSE, 22),
            new PermDef("expense:delete", "费用-删除", PermModules.EXPENSE, 23),
            // 用户/组织/角色
            new PermDef("user:manage", "用户-管理", PermModules.USER, 24),
            new PermDef("dept:manage", "组织-管理", PermModules.DEPT, 25),
            new PermDef("role:manage", "角色-管理", PermModules.ROLE, 26),
            // AI
            new PermDef("ai:config", "AI-配置/额度重置", PermModules.AI, 27),
            new PermDef("ai:ops", "AI-分析/报告/建议", PermModules.AI, 28),
            // 预警管理
            new PermDef("warning:manage", "预警-管理(忽略/转工单/导出)", PermModules.WARNING, 29),
            new PermDef("warning:rule", "预警-忽略规则配置", PermModules.WARNING, 30),
            // 路口/区划
            new PermDef("crossing:manage", "路口-新建/编辑/删除", PermModules.CROSSING, 31),
            new PermDef("area:manage", "区划-配置", PermModules.AREA, 32),
            // 自动巡检
            new PermDef("patrol:manage", "巡检-任务/记录管理", PermModules.PATROL, 33),
            new PermDef("patrol:run", "巡检-执行/自检", PermModules.PATROL, 34),
            new PermDef("patrol:selfcheck", "巡检-信号灯自检", PermModules.PATROL, 35),
            // 模块设置（仅超级管理员）
            new PermDef("module:manage", "模块-启用/停用设置", PermModules.SETTINGS, 36),
            // 系统演示（仅系统管理员）
            new PermDef("demo:use", "系统演示-生成/清理", PermModules.DEMO, 37));

    private PermissionCatalog() {
    }

    /** 返回全部权限编码（对齐 Go 版 allPermCodes）。 */
    public static List<String> allCodes() {
        return ALL.stream().map(PermDef::code).toList();
    }

    /** 返回 base 中剔除 exclude 后的编码列表（对齐 Go 版 withoutPerms）。 */
    public static List<String> without(List<String> base, String exclude) {
        return base.stream().filter(c -> !c.equals(exclude)).toList();
    }
}
