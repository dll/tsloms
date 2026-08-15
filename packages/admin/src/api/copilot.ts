import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 故障级 AI 建议（确认/派单辅助）
export interface FaultAdvice {
  fault_id: number
  device_hw_id: number
  summary: string
  priority: string
  priority_text: string
  plan: string
  parts: string[]
  content: string
  source: string
  tokens_used: number
}

// 工单级 AI 建议（AI 辅助）
export interface WorkOrderAdvice {
  work_order_id: number
  device_hw_id: number
  root_cause: string
  steps: string[]
  parts: string[]
  summary: string
  content: string
  source: string
  tokens_used: number
}

// 故障 AI 建议
export function getFaultAdvice(id: number): Promise<ApiResponse> {
  return request.get(`/ai/advice/fault/${id}`) as unknown as Promise<ApiResponse>
}

// 工单 AI 建议（stage: copilot / summary）
export function getWorkOrderAdvice(id: number, stage = 'copilot'): Promise<ApiResponse> {
  return request.get(`/ai/advice/workorder/${id}`, { params: { stage } }) as unknown as Promise<ApiResponse>
}

// 设备新建/编辑 AI 辅助（依据字段给填写/配置建议）
export interface DeviceAdvice {
  summary: string
  hints: string[]
  issues: string[]
  source: string
}
export function getDeviceAdvice(payload: Record<string, any>): Promise<ApiResponse> {
  return request.post('/ai/advice/device', payload) as unknown as Promise<ApiResponse>
}

// 建单 AI 辅助（基于关联故障推荐优先级/备件/步骤/维修人）
export interface WorkOrderCreateAdvice {
  fault_id: number
  device_hw_id: number
  priority: string
  priority_text: string
  parts: string[]
  steps: string[]
  repairer_hint: string
  summary: string
  source: string
}
export function getWorkOrderCreateAdvice(faultId: number): Promise<ApiResponse> {
  return request.post('/ai/advice/workorder/create', { fault_id: faultId }) as unknown as Promise<ApiResponse>
}

// 采购 AI 辅助（合理性校验 + 供应商建议）
export interface PurchaseAdvice {
  summary: string
  checks: string[]
  suggestions: string[]
  supplier_hint: string
  source: string
}
export function getPurchaseAdvice(items: { material_name: string; quantity: number; price: number }[], supplierId: number): Promise<ApiResponse> {
  return request.post('/ai/advice/purchase', { items, supplier_id: supplierId }) as unknown as Promise<ApiResponse>
}

// 建议历史
export function getAdvices(bizType?: string, bizId?: number): Promise<ApiResponse> {
  return request.get('/ai/advices', { params: { biz_type: bizType, biz_id: bizId } }) as unknown as Promise<ApiResponse>
}
