import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 采购单明细
export interface PurchaseItem {
  id?: number
  material_id: number
  material_name: string
  quantity: number
  price: number
  amount: number
  received_qty?: number
}

// 采购单
export interface PurchaseOrder {
  id: number
  order_no: string
  supplier_id: number
  supplier_name: string
  status: string
  total_amount: number
  received_at?: string
  operator: string
  note: string
  created_at: string
  items?: PurchaseItem[]
}

// 采购单列表
export function getPurchases(params: Record<string, any> = {}): Promise<ApiResponse> {
  return request.get('/purchases', { params }) as unknown as Promise<ApiResponse>
}

// 采购单详情
export function getPurchaseDetail(id: number): Promise<ApiResponse> {
  return request.get(`/purchases/${id}`) as unknown as Promise<ApiResponse>
}

// 创建采购单
export function createPurchase(data: { supplier_id: number; note?: string; items: { material_id: number; quantity: number; price: number }[] }): Promise<ApiResponse> {
  return request.post('/purchases', data) as unknown as Promise<ApiResponse>
}

// 采购入库（部分/全部）
export function receivePurchase(id: number, items: { item_id: number; quantity: number }[]): Promise<ApiResponse> {
  return request.post(`/purchases/${id}/receive`, { items }) as unknown as Promise<ApiResponse>
}

// 取消采购单
export function cancelPurchase(id: number): Promise<ApiResponse> {
  return request.post(`/purchases/${id}/cancel`) as unknown as Promise<ApiResponse>
}

// 删除采购单
export function deletePurchase(id: number): Promise<ApiResponse> {
  return request.delete(`/purchases/${id}`) as unknown as Promise<ApiResponse>
}
