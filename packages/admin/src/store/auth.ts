import { defineStore } from 'pinia'
import { ref } from 'vue'
import { login as loginApi, getUserInfo } from '@/api/auth'
import { getMyPermissions } from '@/api/rbac'

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

  // 登录：调用后端接口，存储 token
  async function login(username: string, password: string) {
    const res = await loginApi({ username, password })
    token.value = res.data.token
    user.value = res.data.user
    localStorage.setItem('token', res.data.token)
    await loadPermissions()
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
    localStorage.removeItem('token')
  }

  return { token, user, permissions, login, loadPermissions, fetchUserInfo, logout, hasPerm }
})
