import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/views/login/index.vue'),
    },
    {
      path: '/',
      component: () => import('@/views/layout/index.vue'),
      redirect: '/dashboard',
      meta: { requiresAuth: true },
      children: [
        // 仪表盘 - 数据概览与可视化图表
        { path: 'dashboard', name: 'Dashboard', component: () => import('@/views/dashboard/index.vue'), meta: { title: '仪表盘' } },
        // 设备管理 - 信号灯设备台账
        { path: 'device', name: 'Device', component: () => import('@/views/device/index.vue'), meta: { title: '设备管理' } },
        // 路口管理 - 路口维度设备统计
        { path: 'intersection', name: 'Intersection', component: () => import('@/views/intersection/index.vue'), meta: { title: '路口管理' } },
        // 地图大屏 - 设备分布地图
        { path: 'map', name: 'Map', component: () => import('@/views/map/index.vue'), meta: { title: '地图大屏' } },
        // 故障管理 - 设备故障记录与研判
        { path: 'fault', name: 'Fault', component: () => import('@/views/fault/index.vue'), meta: { title: '故障管理' } },
        // 工单管理 - 维修工单流转
        { path: 'workorder', name: 'Workorder', component: () => import('@/views/workorder/index.vue'), meta: { title: '工单管理' } },
        // 固件管理 - OTA 升级
        { path: 'firmware', name: 'Firmware', component: () => import('@/views/firmware/index.vue'), meta: { title: '固件管理' } },
        // 系统日志 - 操作日志与设备日志
        { path: 'log', name: 'Log', component: () => import('@/views/log/index.vue'), meta: { title: '系统日志' } },
        // 系统设置 - 用户权限与参数配置
        { path: 'settings', name: 'Settings', component: () => import('@/views/settings/index.vue'), meta: { title: '系统设置' } },
      ],
    },
    // 未知路由重定向到仪表盘
    { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
  ],
})

// 路由守卫 - 登录鉴权
router.beforeEach((to) => {
  const token = localStorage.getItem('token')
  if (to.meta.requiresAuth && !token) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  if (to.path === '/login' && token) {
    return { path: '/dashboard' }
  }
  return true
})

export default router
