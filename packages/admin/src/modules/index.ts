import type { Component } from 'vue'

// ============================================================================
// 模块注册表（模块化 / 插件化）
// ----------------------------------------------------------------------------
// 主菜单按模块划分，甲方购买（启用）的模块才加载，否则不可用。
//   - 核心基础模块：恒启（无论配置如何都展示）。
//   - 可选模块：默认不加载；启用与否由后端返回的 enabled_modules 决定
//     （后端经 ENABLED_MODULES 环境变量配置本实例购买的可选模块）。
//
// 可选模块支持「菜单分组」：多个可选子模块可合并为一个侧边栏子菜单（如
// 库存与成本组），但每个子模块仍可独立启停（甲方可只购买其中部分）。
// ============================================================================

export interface ModuleRouteMeta {
  path: string
  name: string
  component: () => Promise<Component>
  title: string
  /** 可选：进入该路由所需 RBAC 权限点（与模块启用是「且」关系） */
  perm?: string
}

export interface ModuleDef {
  /** 模块标识，与后端 EnabledModules 对齐 */
  key: string
  title: string
  icon: Component
  path: string
  /** 是否核心基础模块（恒启） */
  core: boolean
  /** 该模块包含的路由条目 */
  routes: ModuleRouteMeta[]
  /** 可选：菜单分组名（多个可选模块共享一个侧边栏子菜单时设置，如「库存与成本」） */
  group?: string
}

// 图标（Element Plus 内置）
import {
  Odometer,
  Cpu,
  Location,
  MapLocation,
  ChatDotRound,
  Warning,
  Tickets,
  Upload,
  Document,
  Setting,
  VideoCamera,
  Monitor,
  Box,
  ShoppingCart,
  Money,
  Van,
  TrendCharts,
} from '@element-plus/icons-vue'

/**
 * 核心基础模块（恒启）
 */
const coreModules: ModuleDef[] = [
  {
    key: 'dashboard', title: '仪表盘', icon: Odometer, path: '/dashboard', core: true,
    routes: [{ path: '/dashboard', name: 'Dashboard', component: () => import('@/views/dashboard/index.vue'), title: '仪表盘' }],
  },
  {
    key: 'device', title: '设备管理', icon: Cpu, path: '/device', core: true,
    routes: [{ path: '/device', name: 'Device', component: () => import('@/views/device/index.vue'), title: '设备管理' }],
  },
  {
    key: 'intersection', title: '路口管理', icon: Location, path: '/intersection', core: true,
    routes: [{ path: '/intersection', name: 'Intersection', component: () => import('@/views/intersection/index.vue'), title: '路口管理' }],
  },
  {
    key: 'map', title: '地图大屏', icon: MapLocation, path: '/map', core: true,
    routes: [{ path: '/map', name: 'Map', component: () => import('@/views/map/index.vue'), title: '地图大屏' }],
  },
  {
    key: 'feedback', title: '问题反馈', icon: ChatDotRound, path: '/feedback', core: true,
    routes: [{ path: '/feedback', name: 'Feedback', component: () => import('@/views/map/FeedbackPanel.vue'), title: '问题反馈' }],
  },
  {
    key: 'fault', title: '故障管理', icon: Warning, path: '/fault', core: true,
    routes: [
      { path: '/fault', name: 'Fault', component: () => import('@/views/fault/index.vue'), title: '故障管理' },
      { path: '/fault/cases', name: 'FaultCases', component: () => import('@/views/fault/cases.vue'), title: '识别案例库' },
    ],
  },
  {
    key: 'warning', title: '预警管理', icon: Warning, path: '/warning', core: true,
    routes: [
      { path: '/warning', name: 'Warning', component: () => import('@/views/warning/index.vue'), title: '预警管理' },
      { path: '/warning/rules', name: 'WarningRules', component: () => import('@/views/warning/rules.vue'), title: '预警配置' },
    ],
  },
  {
    key: 'workorder', title: '工单管理', icon: Tickets, path: '/workorder', core: true,
    routes: [{ path: '/workorder', name: 'Workorder', component: () => import('@/views/workorder/index.vue'), title: '工单管理' }],
  },
  {
    key: 'firmware', title: '固件管理', icon: Upload, path: '/firmware', core: true,
    routes: [{ path: '/firmware', name: 'Firmware', component: () => import('@/views/firmware/index.vue'), title: '固件管理' }],
  },
  {
    key: 'log', title: '系统日志', icon: Document, path: '/log', core: true,
    routes: [{ path: '/log', name: 'Log', component: () => import('@/views/log/index.vue'), title: '系统日志' }],
  },
  {
    key: 'settings', title: '系统设置', icon: Setting, path: '/settings', core: true,
    routes: [
      { path: '/settings', name: 'Settings', component: () => import('@/views/settings/index.vue'), title: '系统设置' },
      { path: '/settings/license', name: 'License', component: () => import('@/views/settings/license.vue'), title: '授权与试用', perm: 'module:manage' },
    ],
  },
]

/**
 * 可选模块（默认不加载，甲方购买后启用）
 */
const optionalModules: ModuleDef[] = [
  {
    key: 'video', title: '视频监控', icon: VideoCamera, path: '/video', core: false,
    routes: [
      { path: '/video', name: 'Video', component: () => import('@/views/map/VideoPanel.vue'), title: '视频监控' },
      { path: '/monitor', name: 'Monitor', component: () => import('@/views/map/MonitorWall.vue'), title: '监控大屏' },
    ],
  },
  {
    key: 'inventory', title: '物料库存', icon: Box, path: '/inventory/material', core: false, group: '库存与成本',
    routes: [{ path: '/inventory/material', name: 'InvMaterial', component: () => import('@/views/inventory/Material.vue'), title: '物料库存' }],
  },
  {
    key: 'purchase', title: '采购管理', icon: ShoppingCart, path: '/inventory/purchase', core: false, group: '库存与成本',
    routes: [{ path: '/inventory/purchase', name: 'InvPurchase', component: () => import('@/views/inventory/Purchase.vue'), title: '采购管理' }],
  },
  {
    key: 'expense', title: '维修费用', icon: Money, path: '/inventory/expense', core: false, group: '库存与成本',
    routes: [{ path: '/inventory/expense', name: 'InvExpense', component: () => import('@/views/inventory/Expense.vue'), title: '维修费用' }],
  },
  {
    key: 'supplier', title: '供应商', icon: Van, path: '/inventory/supplier', core: false, group: '库存与成本',
    routes: [{ path: '/inventory/supplier', name: 'InvSupplier', component: () => import('@/views/inventory/Supplier.vue'), title: '供应商' }],
  },
  {
    key: 'ai', title: 'AI 分析', icon: TrendCharts, path: '/ai/predict', core: false,
    routes: [
      { path: '/ai/predict', name: 'AIPredict', component: () => import('@/views/ai/PredictMap.vue'), title: '故障预测' },
      { path: '/ai/workbench', name: 'AIWorkbench', component: () => import('@/views/ai/Workbench.vue'), title: 'AI 工作台' },
      { path: '/ai/diagnose', name: 'AIDiagnose', component: () => import('@/views/ai/Diagnose.vue'), title: 'AI 诊断' },
      { path: '/ai/lifecycle', name: 'AILifecycle', component: () => import('@/views/ai/Lifecycle.vue'), title: '生命周期' },
      { path: '/ai/config', name: 'AIConfig', component: () => import('@/views/ai/Config.vue'), title: '额度设置', perm: 'ai:config' },
    ],
  },
]

/** 全部模块（核心 + 可选） */
export const ALL_MODULES: ModuleDef[] = [...coreModules, ...optionalModules]

/** 模块 key → 定义 */
export const MODULE_BY_KEY: Record<string, ModuleDef> = Object.fromEntries(
  ALL_MODULES.map((m) => [m.key, m]),
)

export const CORE_MODULE_KEYS = coreModules.map((m) => m.key)
export const OPTIONAL_MODULE_KEYS = optionalModules.map((m) => m.key)

/** 后端返回的已启模块 key 集合 */
export type EnabledModules = string[]

/**
 * 构建侧边栏菜单树：按模块启用状态 + 权限点过滤，并将带 group 的可选模块聚合为子菜单。
 * 返回结构：
 *   [
 *     { type:'item', module: ModuleDef }                       // 单路由直链
 *     { type:'submenu', group:'库存与成本', items: ModuleDef[] } // 多路由子菜单/分组
 *   ]
 */
export interface MenuItemNode {
  type: 'item' | 'submenu'
  module?: ModuleDef
  group?: string
  items?: ModuleDef[]
  /** 分组/子菜单展示哪些子模块的扁平路由（用于渲染子项） */
  flatRoutes?: ModuleRouteMeta[]
}

export function buildMenuTree(enabledKeys: string[], hasPerm: (p: string) => boolean): MenuItemNode[] {
  const enabled = new Set(enabledKeys)
  CORE_MODULE_KEYS.forEach((k) => enabled.add(k)) // 核心恒启兜底

  // 已启模块（按注册顺序）
  const active: ModuleDef[] = []
  for (const m of ALL_MODULES) {
    if (!enabled.has(m.key)) continue
    const routes = m.routes.filter((r) => !r.perm || hasPerm(r.perm))
    if (routes.length > 0) active.push({ ...m, routes })
  }

  const nodes: MenuItemNode[] = []
  // 先处理无双路由且无分组的直链模块
  const groupedModules = new Map<string, ModuleDef[]>()
  for (const m of active) {
    if (m.group && m.routes.length > 0) {
      const arr = groupedModules.get(m.group) || []
      arr.push(m)
      groupedModules.set(m.group, arr)
    } else {
      nodes.push({ type: 'item', module: m })
    }
  }
  // 分组聚合为子菜单（如「库存与成本」）
  for (const [group, mods] of groupedModules) {
    const flatRoutes: ModuleRouteMeta[] = []
    for (const m of mods) flatRoutes.push(...m.routes)
    nodes.push({ type: 'submenu', group, items: mods, flatRoutes })
  }
  return nodes
}
