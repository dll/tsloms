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

// 工单级 AI 建议（Copilot）
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

// 建议历史
export function getAdvices(bizType?: string, bizId?: number): Promise<ApiResponse> {
  return request.get('/ai/advices', { params: { biz_type: bizType, biz_id: bizId } }) as unknown as Promise<ApiResponse>
}
