import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 物料档案
export interface Material {
  id: number
  code: string
  name: string
  category: string
  spec: string
  unit: string
  unit_price: number
  stock: number
  threshold: number
  device_hw_id?: number
  supplier_id?: number
  note: string
  status: string
  low_stock: boolean
}

// 出入库流水
export interface MaterialStock {
  id: number
  material_id: number
  material_name: string
  type: string
  quantity: number
  price: number
  amount: number
  ref_type: string
  work_order_id?: number
  operator: string
  note: string
  created_at: string
}

// 物料列表（分页）
export function getMaterials(params: Record<string, any> = {}): Promise<ApiResponse> {
  return request.get('/inv/materials', { params }) as unknown as Promise<ApiResponse>
}

// 库存统计
export function getMaterialStats(): Promise<ApiResponse> {
  return request.get('/inv/materials/stats') as unknown as Promise<ApiResponse>
}

// 新增/更新物料
export function saveMaterial(data: Partial<Material>): Promise<ApiResponse> {
  if (data.id) return request.put(`/inv/materials/${data.id}`, data) as unknown as Promise<ApiResponse>
  return request.post('/inv/materials', data) as unknown as Promise<ApiResponse>
}

// 删除物料
export function deleteMaterial(id: number): Promise<ApiResponse> {
  return request.delete(`/inv/materials/${id}`) as unknown as Promise<ApiResponse>
}

// 出入库流水列表
export function getMaterialStocks(params: Record<string, any> = {}): Promise<ApiResponse> {
  return request.get('/inv/stocks', { params }) as unknown as Promise<ApiResponse>
}

// 手动调整库存
export function adjustStock(data: { material_id: number; type: string; quantity: number; note?: string }): Promise<ApiResponse> {
  return request.post('/inv/stocks/adjust', data) as unknown as Promise<ApiResponse>
}

// 工单领料出库
export function useStock(data: { material_id: number; quantity: number; work_order_id: number; note?: string }): Promise<ApiResponse> {
  return request.post('/inv/stocks/use', data) as unknown as Promise<ApiResponse>
}
