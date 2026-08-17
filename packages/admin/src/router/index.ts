import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { MODULE_BY_KEY, CORE_MODULE_KEYS, type EnabledModules } from '@/modules'

// 将「核心恒启」模块的子路由同步注册为 Layout 的 children。
// 核心模块恒启，必须从一开始就存在，否则 catch-all /:pathMatch(.*)*
// 的 redirect: '/dashboard' 会因 /dashboard 未注册而再次命中 catch-all，
// 形成无限重定向 → pushWithRedirect 自递归 → Maximum call stack size exceeded。
function buildCoreChildren(): RouteRecordRaw[] {
  const children: RouteRecordRaw[] = []
  for (const key of CORE_MODULE_KEYS) {
    for (const r of MODULE_BY_KEY[key].routes) {
      children.push({
        path: r.path.replace(/^\//, ''),
        name: r.name,
        component: r.component,
        meta: { title: r.title, module: key },
      })
    }
  }
  // 个人资料（右上角头像下拉入口，恒注册）
  children.push({
    path: 'profile',
    name: 'Profile',
    component: () => import('@/views/settings/profile.vue'),
    meta: { title: '个人资料', module: 'profile' },
  })
  return children
}

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
      name: 'Layout',
      component: () => import('@/views/layout/index.vue'),
      redirect: '/dashboard',
      meta: { requiresAuth: true },
      children: buildCoreChildren(),
    },
    // 未知路由重定向到仪表盘（核心路由已同步注册，/dashboard 必然存在，可安全重定向）
    { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
  ],
})

// 已注册模块路由的 layout 路由索引（第 2 个顶层路由）
const layoutRouteIndex = 1

// 模块路由是否已注册
let registered = false

/**
 * 根据已启模块动态注册子路由（模块化/插件化）
 *  - 核心模块恒启（创建 router 时已同步注册）；可选模块仅当出现在 enabledKeys 中才注册
 *  - 未启用模块的路由不注册 → 无法直接访问（前端兜底，后端另有 RequireModule 拦截）
 *  - 幂等：可安全重复调用（每次都会基于核心路由重建 children）
 */
export function registerModuleRoutes(enabledKeys: EnabledModules) {
  const set = new Set(enabledKeys)
  CORE_MODULE_KEYS.forEach((k) => set.add(k)) // 核心恒启兜底
  const active: string[] = []
  set.forEach((k) => {
    if (MODULE_BY_KEY[k]) active.push(k)
  })

  // 始终以核心路由为基底，再叠加可选模块，避免丢掉同步注册的核心路由
  const children = buildCoreChildren()
  for (const key of active) {
    for (const r of MODULE_BY_KEY[key].routes) {
      children.push({
        path: r.path.replace(/^\//, ''),
        name: r.name,
        component: r.component,
        meta: { title: r.title, module: key },
      })
    }
  }

  const layout = router.options.routes[layoutRouteIndex]
  ;(layout as { children: RouteRecordRaw[] }).children = children
  // 幂等替换：先移除旧的、再以同一 name 重新挂载，避免重复匹配
  if (router.hasRoute('Layout')) router.removeRoute('Layout')
  router.addRoute({
    ...(layout as RouteRecordRaw),
    name: 'Layout',
  })
  registered = true
}

// 路由守卫 - 登录鉴权；已登录且模块路由尚未注册时，先加载模块并注册再放行
router.beforeEach(async (to) => {
  const token = localStorage.getItem('token')
  if (to.meta.requiresAuth && !token) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  if (to.path === '/login' && token) {
    return { path: '/dashboard' }
  }
  // 已登录：确保模块路由已注册（首次导航前完成，避免 dashboard 未注册导致循环重定向）
  if (token && !registered) {
    const { useAuthStore } = await import('@/store/auth')
    const auth = useAuthStore()
    if (auth.enabledModules.length === 0) {
      await auth.loadPermissions()
      await auth.loadModules()
    }
    registerModuleRoutes(auth.enabledModules)
    // 强制重新导航以命中新注册的子路由
    return { path: to.fullPath, replace: true }
  }
  return true
})

export default router
export type { EnabledModules }
export { MODULE_BY_KEY }
