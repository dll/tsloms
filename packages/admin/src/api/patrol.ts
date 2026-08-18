import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// ============================================================================
// P1 自动巡检：任务 / 记录 / 排行 / 信号灯自检
// ============================================================================

export interface PatrolTask {
  id: number
  name: string
  mode: string // area/street/random/selfcheck/ai
  area_id?: number | null
  street_id?: number | null
  time_window?: string
  target_count?: number
  status?: string // planned/running/done
  assignee_id?: number | null
  created_by?: number
  run_count?: number
  last_run_at?: string | null
  remark?: string
  created_at?: string
  [key: string]: any
}

export interface PatrolRecord {
  id: number
  task_id?: number | null
  device_id?: number
  device_hw_id?: string
  crossing_id?: number | null
  crossing_name?: string
  intersection?: string
  patrol_type?: string
  check_result?: string // normal/abnormal
  check_detail?: string
  selfcheck_result?: string
  evidences?: string
  patrol_by?: string
  patrol_at?: string
  [key: string]: any
}

// 任务列表 / 创建
export function getPatrolTasks(params?: Record<string, any>): Promise<ApiResponse> {
  return request.get('/patrol/tasks', { params }) as unknown as Promise<ApiResponse>
}
export function createPatrolTask(data: Partial<PatrolTask>): Promise<ApiResponse> {
  return request.post('/patrol/tasks', data) as unknown as Promise<ApiResponse>
}
export function updatePatrolTask(id: number, data: Partial<PatrolTask>): Promise<ApiResponse> {
  return request.put(`/patrol/tasks/${id}`, data) as unknown as Promise<ApiResponse>
}
export function deletePatrolTask(id: number): Promise<ApiResponse> {
  return request.delete(`/patrol/tasks/${id}`) as unknown as Promise<ApiResponse>
}
export function runPatrolTask(id: number): Promise<ApiResponse> {
  return request.post(`/patrol/tasks/${id}/run`) as unknown as Promise<ApiResponse>
}

// 巡检记录 / 排行
export function getPatrolRecords(params?: Record<string, any>): Promise<ApiResponse> {
  return request.get('/patrol/records', { params }) as unknown as Promise<ApiResponse>
}
export function getPatrolRanking(dimension?: string): Promise<ApiResponse> {
  return request.get('/patrol/ranking', { params: { dimension } }) as unknown as Promise<ApiResponse>
}

// 信号灯自检（即时）
export function postPatrolSelfCheck(hwIds: number[]): Promise<ApiResponse> {
  return request.post('/patrol/selfcheck', { device_hw_ids: hwIds }) as unknown as Promise<ApiResponse>
}

export const PATROL_MODES = [
  { value: 'area', label: '空间区域' },
  { value: 'street', label: '街道排查' },
  { value: 'random', label: '随机抽检' },
  { value: 'selfcheck', label: '信号灯自检' },
  { value: 'ai', label: 'AI 硬件' },
]
export const PATROL_STATUS = [
  { value: 'planned', label: '待执行' },
  { value: 'running', label: '执行中' },
  { value: 'done', label: '已完成' },
]
