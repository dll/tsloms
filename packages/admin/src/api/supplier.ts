import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 供应商
export interface Supplier {
  id: number
  name: string
  contact: string
  phone: string
  address: string
  email: string
  status: string
  note: string
  created_at: string
}

// 供应商列表
export function getSuppliers(params: Record<string, any> = {}): Promise<ApiResponse> {
  return request.get('/suppliers', { params }) as unknown as Promise<ApiResponse>
}

// 全量供应商（下拉）
export function getAllSuppliers(): Promise<ApiResponse> {
  return request.get('/suppliers', { params: { all: 1 } }) as unknown as Promise<ApiResponse>
}

// 新增/更新供应商
export function saveSupplier(data: Partial<Supplier>): Promise<ApiResponse> {
  if (data.id) return request.put(`/suppliers/${data.id}`, data) as unknown as Promise<ApiResponse>
  return request.post('/suppliers', data) as unknown as Promise<ApiResponse>
}

// 删除供应商
export function deleteSupplier(id: number): Promise<ApiResponse> {
  return request.delete(`/suppliers/${id}`) as unknown as Promise<ApiResponse>
}
