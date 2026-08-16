import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 故障列表查询参数
export interface FaultQuery {
  page?: number
  page_size?: number
  device_hw_id?: string
  status?: string
  fault_type?: string
  fault_level?: string
  start_date?: string
  end_date?: string
}

// 故障列表项（智能识别字段：仅做加法，兼容旧记录）
export interface FaultItem {
  id: number | string
  device_hw_id?: number | string
  err_code?: number
  fault_level?: string
  status?: string
  owner_id?: number | null
  repairer_id?: number | null
  owner_name?: string | null
  repairer_name?: string | null
  last_seen?: string
  // ---- 智能识别研判字段（范围B新增） ----
  confidence?: number | null
  recognition_source?: string
  recognition_status?: string // confirmed / pending_review / filtered
  is_false_positive?: boolean | null
  evidence_count?: number
  reviewed_at?: string | null
  [key: string]: any
}

// 获取故障列表（分页）
export function getFaults(params: FaultQuery): Promise<ApiResponse<{ list: FaultItem[]; total: number }>> {
  return request.get('/faults', { params }) as unknown as Promise<ApiResponse>
}

// 获取故障详情
export function getFault(id: number | string): Promise<ApiResponse> {
  return request.get(`/faults/${id}`) as unknown as Promise<ApiResponse>
}

// 更新故障（确认/负责人/维修人/状态）
export interface FaultUpdate {
  status?: string
  owner_id?: number | null
  repairer_id?: number | null
}
export function updateFault(id: number | string, data: FaultUpdate): Promise<ApiResponse> {
  return request.put(`/faults/${id}`, data) as unknown as Promise<ApiResponse>
}

// 从故障派发工单（指派维修人）
export function dispatchFault(id: number | string, assigneeId: number): Promise<ApiResponse> {
  return request.post(`/faults/${id}/dispatch`, { assignee_id: assigneeId }) as unknown as Promise<ApiResponse>
}

// 故障可用状态列表
export const FAULT_STATUSES = [
  { value: 'occurred', label: '发生', tag: 'danger' },
  { value: 'confirmed', label: '已确认', tag: 'warning' },
  { value: 'dispatched', label: '已派单', tag: '' },
  { value: 'resolved', label: '已解决', tag: 'success' },
]

export function faultStatusLabel(s: string): string {
  return FAULT_STATUSES.find((x) => x.value === s)?.label || s
}
export function faultStatusTag(s: string): string {
  return FAULT_STATUSES.find((x) => x.value === s)?.tag || 'info'
}
