import { defineStore } from 'pinia'
import { ref } from 'vue'
import { login as loginApi, getUserInfo } from '@/api/auth'

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

  // 登录：调用后端接口，存储 token
  async function login(username: string, password: string) {
    const res = await loginApi({ username, password })
    token.value = res.data.token
    user.value = res.data.user
    localStorage.setItem('token', res.data.token)
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
    localStorage.removeItem('token')
  }

  return { token, user, login, fetchUserInfo, logout }
})
