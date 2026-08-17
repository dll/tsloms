import { defineStore } from 'pinia'
import { ref } from 'vue'
import { login as loginApi, loginByPhone, getUserInfo } from '@/api/auth'
import { getMyPermissions } from '@/api/rbac'
import { getEnabledModules } from '@/api/module'

// 用户信息接口
export interface UserInfo {
  id: number
  username: string
  role: string
  phone: string
}

export const useAuthStore = defineStore('auth', () => {
  // 从 localStorage 恢复 token
  const token = ref<string>(localStorage.getItem('token') || '')
  const user = ref<UserInfo | null>(null)
  // 当前用户有效功能权限集合（如 ['device:create', ...]）
  const permissions = ref<string[]>([])
  // 当前实例已启模块 key 集合（模块化/插件化；核心恒启 + 已购可选）
  const enabledModules = ref<string[]>([])

  // 登录：username+password 通道
  async function login(username: string, password: string) {
    const res = await loginApi({ username, password })
    await applyLogin(res)
  }

  // 登录：手机号+验证码通道（P0）
  async function loginWithPhone(phone: string, code: string) {
    const res = await loginByPhone(phone, code)
    await applyLogin(res)
  }

  // 应用登录结果：存 token / user / 拉权限与模块
  async function applyLogin(res: any) {
    token.value = res.data.token
    user.value = res.data.user
    localStorage.setItem('token', res.data.token)
    await loadPermissions()
    await loadModules()
  }

  // 拉取当前用户功能权限
  async function loadPermissions() {
    if (!token.value) return
    try {
      const res = await getMyPermissions()
      permissions.value = res.data?.permissions || []
    } catch {
      permissions.value = []
    }
  }

  // 拉取当前实例已启模块（核心恒启 + 甲方购买的可选模块）
  async function loadModules() {
    if (!token.value) return
    try {
      const res = await getEnabledModules()
      const list = res.data?.modules || []
      enabledModules.value = list.map((m) => m.key)
    } catch {
      // 拉取失败时回退为核心模块，保证核心功能可用
      enabledModules.value = []
    }
  }

  // 判断某模块是否已启用（甲方购买）
  function hasModule(key: string) {
    return enabledModules.value.includes(key)
  }

  // 判断是否拥有某功能权限
  function hasPerm(perm: string) {
    return permissions.value.includes(perm)
  }

  // 获取当前用户信息
  async function fetchUserInfo() {
    const res = await getUserInfo()
    user.value = res.data.user
    return res.data.user
  }

  // 退出登录：清除本地状态
  function logout() {
    token.value = ''
    user.value = null
    permissions.value = []
    enabledModules.value = []
    localStorage.removeItem('token')
  }

  return { token, user, permissions, enabledModules, login, loginWithPhone, loadPermissions, loadModules, fetchUserInfo, logout, hasPerm, hasModule }
})
