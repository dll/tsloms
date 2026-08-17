import { defineStore } from 'pinia'
import { ref } from 'vue'
import { login as loginApi, getUserInfo } from '@/api/auth'
import { getMyPermissions } from '@/api/rbac'
import { getEnabledModules } from '@/api/module'

// 用户信息接口
// 含登录账号 + 人事核心字段（工作照/工号/性别/身份证/住址/文化程度/工程等级）
export interface UserInfo {
  id: number
  username: string
  role: string
  real_name?: string
  phone?: string
  phone_login?: string
  phone_verified?: boolean
  email?: string
  department_id?: number | null
  status?: string
  center_lat?: number | null
  center_lng?: number | null
  work_no?: string
  avatar?: string
  gender?: string
  id_card?: string
  address?: string
  education?: string
  engineer_level?: string
}

export const useAuthStore = defineStore('auth', () => {
  // 从 localStorage 恢复 token
  const token = ref<string>(localStorage.getItem('token') || '')
  const user = ref<UserInfo | null>(null)
  // 当前用户有效功能权限集合（如 ['device:create', ...]）
  const permissions = ref<string[]>([])
  // 当前实例已启模块 key 集合（模块化/插件化；核心恒启 + 已购可选）
  const enabledModules = ref<string[]>([])

  // 登录：账号(可手机号) + 密码（登录接口内含算术验证码校验，由调用方先取 captcha）
  async function login(loginPayload: { username: string; password: string; captcha_uuid: string; captcha_code: string }) {
    const res = await loginApi(loginPayload)
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

  return { token, user, permissions, enabledModules, login, loadPermissions, loadModules, fetchUserInfo, logout, hasPerm, hasModule }
})
