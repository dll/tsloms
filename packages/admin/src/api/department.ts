import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 部门信息
export interface DepartmentItem {
  id: number
  name: string
  parent_id: number | null
  leader: string
  description: string
  member_count: number
  created_at: string
}

// 获取部门列表（所有人可读）
export function getDepartments(): Promise<ApiResponse> {
  return request.get('/departments') as unknown as Promise<ApiResponse>
}

// 新增部门（管理员）
export function createDepartment(data: { name: string; parent_id?: number | null; leader?: string; description?: string }): Promise<ApiResponse> {
  return request.post('/departments', data) as unknown as Promise<ApiResponse>
}

// 更新部门（管理员）
export function updateDepartment(id: number, data: { name?: string; parent_id?: number | null; leader?: string; description?: string }): Promise<ApiResponse> {
  return request.put(`/departments/${id}`, data) as unknown as Promise<ApiResponse>
}

// 删除部门（管理员）
export function deleteDepartment(id: number): Promise<ApiResponse> {
  return request.delete(`/departments/${id}`) as unknown as Promise<ApiResponse>
}
