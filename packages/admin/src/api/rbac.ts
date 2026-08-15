import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 当前用户有效功能权限
export function getMyPermissions(): Promise<ApiResponse> {
  return request.get('/my/permissions') as unknown as Promise<ApiResponse>
}

// 权限点字典（按模块分组）
export function listPermissions(): Promise<ApiResponse> {
  return request.get('/rbac/permissions') as unknown as Promise<ApiResponse>
}

// 角色信息
export interface RoleItem {
  id: number
  code: string
  name: string
  builtin: boolean
  description: string
  permissions: string[]
  created_at: string
}

// 角色列表
export function listRoles(): Promise<ApiResponse> {
  return request.get('/rbac/roles') as unknown as Promise<ApiResponse>
}

// 创建角色
export function createRole(data: {
  code: string; name: string; description?: string; permissions: string[]
}): Promise<ApiResponse> {
  return request.post('/rbac/roles', data) as unknown as Promise<ApiResponse>
}

// 更新角色（自定义角色）
export function updateRole(id: number, data: {
  name?: string; description?: string; permissions?: string[]
}): Promise<ApiResponse> {
  return request.put(`/rbac/roles/${id}`, data) as unknown as Promise<ApiResponse>
}

// 删除角色（自定义角色）
export function deleteRole(id: number): Promise<ApiResponse> {
  return request.delete(`/rbac/roles/${id}`) as unknown as Promise<ApiResponse>
}

// 用户功能权限详情（角色默认 + 用户覆写）
export function getUserPermissions(id: number): Promise<ApiResponse> {
  return request.get(`/rbac/users/${id}/permissions`) as unknown as Promise<ApiResponse>
}

// 设置用户功能权限
export function setUserPermissions(id: number, data: {
  grants: string[]; denies: string[]
}): Promise<ApiResponse> {
  return request.put(`/rbac/users/${id}/permissions`, data) as unknown as Promise<ApiResponse>
}
