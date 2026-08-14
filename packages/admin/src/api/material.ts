import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 维修耗材
export interface Material {
  id: number
  device_hw_id: number
  name: string
  part_no?: string
  spec?: string
  quantity: number
  unit?: string
  threshold?: number
  note?: string
}

// 耗材列表（可按设备）
export function getMaterials(device_hw_id?: string): Promise<ApiResponse> {
  return request.get('/materials', { params: { device_hw_id } }) as unknown as Promise<ApiResponse>
}

// 新增/更新耗材
export function saveMaterial(data: Material): Promise<ApiResponse> {
  return request.post('/materials', data) as unknown as Promise<ApiResponse>
}

// 删除耗材
export function deleteMaterial(id: number): Promise<ApiResponse> {
  return request.delete(`/materials/${id}`) as unknown as Promise<ApiResponse>
}
