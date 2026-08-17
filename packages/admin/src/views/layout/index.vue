<template>
  <el-container class="layout-container">
    <!-- 左侧侧边栏 -->
    <el-aside :width="isCollapse ? '64px' : '220px'" class="sidebar">
      <div class="logo">
        <el-icon :size="24" color="#409EFF"><Monitor /></el-icon>
        <span v-show="!isCollapse" class="logo-text">TSLOMS</span>
      </div>
      <el-menu
        :default-active="activeMenu"
        :collapse="isCollapse"
        :collapse-transition="false"
        router
        background-color="#001529"
        text-color="rgba(255, 255, 255, 0.65)"
        active-text-color="#fff"
      >
        <!-- 按已启模块动态渲染：核心恒启 + 甲方购买的可选模块；带 group 的合并为子菜单 -->
        <template v-for="node in menuTree" :key="node.type === 'submenu' ? node.group : node.module!.key">
          <!-- 分组子菜单（如库存与成本） -->
          <el-sub-menu v-if="node.type === 'submenu'" :index="node.group!">
            <template #title>
              <el-icon><Box /></el-icon>
              <span>{{ node.group }}</span>
            </template>
            <template v-for="m in node.items!" :key="m.key">
              <el-menu-item v-for="r in m.routes" :key="r.path" :index="r.path">
                {{ r.title }}
              </el-menu-item>
            </template>
          </el-sub-menu>
          <!-- 多路由子模块（如 AI 分析 / 视频监控） -->
          <el-sub-menu v-else-if="node.module!.routes.length > 1" :index="node.module!.path">
            <template #title>
              <el-icon><component :is="node.module!.icon" /></el-icon>
              <span>{{ node.module!.title }}</span>
            </template>
            <el-menu-item v-for="r in node.module!.routes" :key="r.path" :index="r.path">
              {{ r.title }}
            </el-menu-item>
          </el-sub-menu>
          <!-- 单路由直链模块 -->
          <el-menu-item v-else :index="node.module!.path">
            <el-icon><component :is="node.module!.icon" /></el-icon>
            <template #title>{{ node.module!.title }}</template>
          </el-menu-item>
        </template>
      </el-menu>

      <!-- 侧边栏收起/展开按钮：垂直右边中间，红黄绿信号灯图标 -->
      <div class="sidebar-toggle" @click="isCollapse = !isCollapse" :title="isCollapse ? '展开菜单' : '收起菜单'">
        <svg class="toggle-icon" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
          <!-- 红黄绿三色信号灯（垂直叠加） -->
          <circle cx="12" cy="5.5" r="3" fill="#ff4d4f" stroke="rgba(255,255,255,0.6)" stroke-width="1" />
          <circle cx="12" cy="12" r="3" fill="#fadb14" stroke="rgba(255,255,255,0.6)" stroke-width="1" />
          <circle cx="12" cy="18.5" r="3" fill="#52c41a" stroke="rgba(255,255,255,0.6)" stroke-width="1" />
        </svg>
      </div>
    </el-aside>

    <!-- 右侧主区域 -->
    <el-container>
      <!-- 顶部导航栏 -->
      <el-header class="header">
        <div class="header-left">
          <span class="system-title">TSLOMS 交通信号灯运维系统</span>
        </div>
        <div class="header-right">
          <!-- 顶部 AI 助手：自然语言查询/操作（L5，AI 模块启用时显示） -->
          <el-tooltip v-if="authStore.hasModule('ai')" content="AI 助手（自然语言查询/报修）" placement="bottom">
            <div class="ai-assist-entry" @click="assistantVisible = true">
              <el-icon :size="20"><MagicStick /></el-icon>
            </div>
          </el-tooltip>
          <!-- 通知铃铛：AI 主动巡检推送（notification 模块启用时显示） -->
          <el-badge v-if="authStore.hasModule('notification')" :value="unreadCount" :hidden="unreadCount === 0" :max="99" class="notify-badge">
            <el-popover placement="bottom-end" :width="380" trigger="click" @show="openNotify">
              <template #reference>
                <div class="notify-bell">
                  <el-icon :size="20"><Bell /></el-icon>
                </div>
              </template>
              <div class="notify-panel">
                <div class="notify-head">
                  <span class="notify-title">通知中心</span>
                  <el-button v-if="unreadCount > 0" type="primary" link size="small" @click="markAllRead">全部已读</el-button>
                </div>
                <el-empty v-if="!notifications.length" description="暂无通知" :image-size="60" />
                <div v-else class="notify-list">
                  <div v-for="n in notifications" :key="n.id" class="notify-item" @click="onNotifyClick(n)">
                    <el-tag :type="notifyTagType(n.type)" size="small" effect="plain" class="notify-tag">{{ notifyTypeLabel(n.type) }}</el-tag>
                    <div class="notify-body">
                      <div class="notify-item-title">{{ n.title }}<span v-if="!n.is_read" class="notify-dot" /></div>
                      <div class="notify-content">{{ n.content }}</div>
                      <div class="notify-time">{{ formatTime(n.created_at) }}</div>
                    </div>
                  </div>
                </div>
              </div>
            </el-popover>
          </el-badge>
          <el-dropdown @command="handleCommand">
            <span class="user-info">
              <el-avatar v-if="authStore.user?.avatar" :size="28" :src="authStore.user.avatar" class="user-avatar" />
              <el-icon v-else><User /></el-icon>
              <span class="username">{{ authStore.user?.real_name || authStore.user?.username || '用户' }}</span>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">个人资料</el-dropdown-item>
                <el-dropdown-item command="settings">系统设置</el-dropdown-item>
                <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <!-- 主内容区 -->
      <el-main class="main-content">
        <router-view />
      </el-main>
    </el-container>

    <!-- 顶部 AI 助手（L5 自然语言交互） -->
    <AiAssistant v-if="assistantVisible" v-model:visible="assistantVisible" />
  </el-container>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/store/auth'
import { getNotifications, getUnreadCount, readNotification, readAllNotifications, type NotificationItem } from '@/api/notification'
import AiAssistant from '@/components/AiAssistant.vue'
import { buildMenuTree, type MenuItemNode } from '@/modules'
import { registerModuleRoutes } from '@/router'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

// AI 助手对话框（L5 自然语言交互）
const assistantVisible = ref(false)

// 站内通知（AI 主动巡检推送）
const notifications = ref<NotificationItem[]>([])
const unreadCount = ref(0)

async function openNotify() {
  try {
    const [list, un] = await Promise.all([getNotifications(30), getUnreadCount()])
    notifications.value = list.data?.list || []
    unreadCount.value = un.data?.unread || 0
  } catch { /* 忽略 */ }
}

async function refreshUnread() {
  try {
    const un = await getUnreadCount()
    unreadCount.value = un.data?.unread || 0
  } catch { /* 忽略 */ }
}

function onNotifyClick(n: NotificationItem) {
  if (!n.is_read) {
    readNotification(n.id).then(refreshUnread)
    n.is_read = true
  }
  if (n.link) router.push(n.link)
}

async function markAllRead() {
  await readAllNotifications()
  notifications.value.forEach((n) => (n.is_read = true))
  unreadCount.value = 0
}

function notifyTypeLabel(t: string) {
  const map: Record<string, string> = { report: '日报', alert: '预警', system: '系统' }
  return map[t] || t || '系统'
}
function notifyTagType(t: string) {
  if (t === 'alert') return 'danger'
  if (t === 'report') return 'success'
  return 'info'
}
function formatTime(s: string) {
  if (!s) return ''
  return s.replace('T', ' ').slice(0, 16)
}

// 侧边栏折叠状态（默认收起为窄条，点按钮展开完整菜单）
const isCollapse = ref(true)

// 当前激活的菜单项
const activeMenu = computed(() => route.path)

// 按已启模块 + 权限点构建菜单树（核心恒启 + 已购可选；带 group 合并为子菜单）
const menuTree = computed<MenuItemNode[]>(() =>
  buildMenuTree(authStore.enabledModules, (p) => authStore.hasPerm(p)),
)

// 下拉菜单命令处理
async function handleCommand(command: string) {
  if (command === 'logout') {
    try {
      await ElMessageBox.confirm('确定要退出登录吗？', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      })
      authStore.logout()
      router.push('/login')
    } catch {
      // 用户取消
    }
  } else if (command === 'settings') {
    router.push('/settings')
  } else if (command === 'profile') {
    router.push('/profile')
  }
}

// 页面加载：拉取用户信息 / 权限 / 已启模块，并动态注册路由
onMounted(async () => {
  if (authStore.token && !authStore.user) {
    try {
      await authStore.fetchUserInfo()
    } catch {
      // 获取用户信息失败，忽略
    }
  }
  // 拉取当前用户功能权限（供菜单/按钮联动）
  await authStore.loadPermissions()
  // 拉取已启模块（模块化/插件化），并动态注册路由
  await authStore.loadModules()
  registerModuleRoutes(authStore.enabledModules)
  // 通知模块启用时拉取未读数（每 2 分钟刷新）
  if (authStore.hasModule('notification')) {
    refreshUnread()
    setInterval(refreshUnread, 120000)
  }
})
</script>

<style scoped>
.layout-container {
  height: 100vh;
}

.sidebar {
  background-color: #001529;
  transition: width 0.3s;
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
  position: relative; /* 供中间的收起/展开按钮定位 */
}

.logo {
  height: 60px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #fff;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.logo-text {
  font-size: 18px;
  font-weight: bold;
  white-space: nowrap;
}

.sidebar .el-menu {
  border-right: none;
  flex: 1 1 auto;
  height: 0; /* 配合 overflow 让菜单在剩余空间内滚动 */
  overflow-y: auto;
  overflow-x: hidden;
}

/* 菜单滚动条样式（深色侧边栏上更协调） */
.sidebar .el-menu::-webkit-scrollbar {
  width: 6px;
}
.sidebar .el-menu::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.25);
  border-radius: 3px;
}
.sidebar .el-menu::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.4);
}
.sidebar .el-menu::-webkit-scrollbar-track {
  background: transparent;
}

/* 菜单项可读性优化：提升非激活项对比度，激活项底色高亮 */
.sidebar .el-menu-item {
  font-size: 14px;
}

.sidebar .el-menu-item:hover {
  background-color: rgba(64, 158, 255, 0.15) !important;
}

.sidebar .el-menu-item.is-active {
  background-color: #409eff !important;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background-color: #fff;
  border-bottom: 1px solid #e8e8e8;
  box-shadow: 0 1px 4px rgba(0, 21, 41, 0.08);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.system-title {
  font-size: 16px;
  font-weight: 600;
  color: #333;
}

.header-right {
  display: flex;
  align-items: center;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  color: #333;
  outline: none;
}

.username {
  font-size: 14px;
}

.main-content {
  background-color: #f0f2f5;
  padding: 20px;
  overflow-y: auto;
}

/* ---- 侧边栏中间收起/展开按钮 ---- */
.sidebar-toggle {
  position: absolute;
  right: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 24px;
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: rgba(255, 255, 255, 0.7);
  background-color: #2b3a55;
  border-radius: 6px 0 0 6px;
  box-shadow: -2px 0 6px rgba(0, 0, 0, 0.25);
  transition: color 0.2s, background-color 0.2s;
  z-index: 10;
}

.sidebar-toggle:hover {
  color: #fff;
  background-color: #409eff;
}

.toggle-icon {
  width: 16px;
  height: 22px;
}

/* ---- 顶部 AI 助手（L5）---- */
.ai-assist-entry {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border-radius: 6px;
  margin-right: 12px;
  color: #606266;
  cursor: pointer;
  transition: color 0.2s, background-color 0.2s;
}
.ai-assist-entry:hover {
  color: #409eff;
  background-color: #f0f2f5;
}

/* ---- 通知铃铛 ---- */
.notify-badge {
  margin-right: 18px;
  display: inline-flex;
  align-items: center;
}
.notify-bell {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border-radius: 6px;
  color: #606266;
  cursor: pointer;
  transition: color 0.2s, background-color 0.2s;
}
.notify-bell:hover {
  color: #409eff;
  background-color: #f0f2f5;
}
.notify-panel {
  padding: 0 4px;
}
.notify-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 4px 8px;
  border-bottom: 1px solid #ebeef5;
}
.notify-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}
.notify-list {
  max-height: 420px;
  overflow-y: auto;
}
.notify-item {
  display: flex;
  gap: 10px;
  padding: 12px 6px;
  border-bottom: 1px solid #f5f5f5;
  cursor: pointer;
  transition: background-color 0.15s;
}
.notify-item:hover {
  background-color: #f7f9fc;
}
.notify-tag {
  flex-shrink: 0;
  margin-top: 2px;
}
.notify-body {
  flex: 1;
  min-width: 0;
}
.notify-item-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  display: flex;
  align-items: center;
  gap: 6px;
}
.notify-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #f56c6c;
  flex-shrink: 0;
}
.notify-content {
  font-size: 13px;
  color: #606266;
  line-height: 1.5;
  margin-top: 2px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.notify-time {
  font-size: 12px;
  color: #c0c4cc;
  margin-top: 4px;
}
</style>
