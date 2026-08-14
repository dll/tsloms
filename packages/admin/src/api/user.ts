import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 用户查询参数
export interface UserQuery {
  page?: number
  page_size?: number
  role?: string
  keyword?: string
}

// 用户信息
export interface UserItem {
  id: number
  username: string
  role: string
  phone: string
  created_at: string
}

// 获取用户列表
export function getUsers(params: UserQuery): Promise<ApiResponse> {
  return request.get('/users', { params }) as unknown as Promise<ApiResponse>
}

// 创建用户
export function createUser(data: { username: string; password: string; role: string; phone?: string }): Promise<ApiResponse> {
  return request.post('/users', data) as unknown as Promise<ApiResponse>
}

// 更新用户
export function updateUser(id: number, data: { role?: string; phone?: string }): Promise<ApiResponse> {
  return request.put(`/users/${id}`, data) as unknown as Promise<ApiResponse>
}

// 重置用户密码
export function resetUserPassword(id: number, password: string): Promise<ApiResponse> {
  return request.put(`/users/${id}/password`, { password }) as unknown as Promise<ApiResponse>
}

// 删除用户
export function deleteUser(id: number): Promise<ApiResponse> {
  return request.delete(`/users/${id}`) as unknown as Promise<ApiResponse>
}
