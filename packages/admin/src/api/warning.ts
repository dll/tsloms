import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// ============================================================================
// 预警管理 + 路口/行政区划 + 地图分级数据（P0）
// 预警 record：列表/详情/忽略/批量忽略/转工单/导出/自动忽略
// 预警配置 rule：CRUD（忽略路口/设备/预警类型/生效时间段）
// 路口 crossings + 区划 areas + 地图聚合数据 map/crossing-data、map/road-data
// ============================================================================

// ---------------- 预警记录 ----------------
export interface WarningItem {
  id: number
  device_hw_id?: number | string
  crossing_id?: number | null
  crossing_name?: string
  warning_type?: string
  level?: string // critical/warning/info
  content?: string
  status?: string
  first_seen?: string
  last_seen?: string
  ignored?: boolean
  work_order_id?: number | null
  [key: string]: any
}

export interface WarningQuery {
  page?: number
  page_size?: number
  status?: string
  level?: string
  warning_type?: string
  device_hw_id?: string
  start_date?: string
  end_date?: string
}

export function getWarnings(params: WarningQuery): Promise<ApiResponse> {
  return request.get('/warnings', { params }) as unknown as Promise<ApiResponse>
}
export function getWarning(id: number | string): Promise<ApiResponse> {
  return request.get(`/warnings/${id}`) as unknown as Promise<ApiResponse>
}
export function ignoreWarning(id: number | string): Promise<ApiResponse> {
  return request.post(`/warnings/${id}/ignore`) as unknown as Promise<ApiResponse>
}
export function batchIgnoreWarnings(ids: number[]): Promise<ApiResponse> {
  return request.post('/warnings/batch-ignore', { ids }) as unknown as Promise<ApiResponse>
}
export function warningToWorkOrder(id: number | string): Promise<ApiResponse> {
  return request.post(`/warnings/${id}/to-workorder`) as unknown as Promise<ApiResponse>
}
export function autoIgnoreWarnings(): Promise<ApiResponse> {
  return request.post('/warnings/auto-ignore') as unknown as Promise<ApiResponse>
}
export function exportWarnings(params: WarningQuery): Promise<ApiResponse> {
  return request.get('/warnings/export', { params }) as unknown as Promise<ApiResponse>
}

// ---------------- 预警配置（忽略规则） ----------------
export interface WarningRule {
  id: number
  name?: string
  crossing_id?: number | null
  device_hw_id?: number | null
  ignore_type?: string // warning_type to ignore
  enabled?: boolean
  effect_mode?: string
  effect_time_start?: string
  effect_time_end?: string
  remark?: string
  [key: string]: any
}

export function getWarningRules(): Promise<ApiResponse> {
  return request.get('/warning-rules') as unknown as Promise<ApiResponse>
}
export function createWarningRule(data: Partial<WarningRule>): Promise<ApiResponse> {
  return request.post('/warning-rules', data) as unknown as Promise<ApiResponse>
}
export function updateWarningRule(id: number | string, data: Partial<WarningRule>): Promise<ApiResponse> {
  return request.put(`/warning-rules/${id}`, data) as unknown as Promise<ApiResponse>
}
export function deleteWarningRule(id: number | string): Promise<ApiResponse> {
  return request.delete(`/warning-rules/${id}`) as unknown as Promise<ApiResponse>
}

// ---------------- 路口 crossings ----------------
export interface CrossingItem {
  id: number
  name: string
  area_id?: number | null
  area_full_name?: string
  lat?: number | null
  lng?: number | null
  status?: string
  device_count?: number
  [key: string]: any
}

export function getCrossings(params?: Record<string, any>): Promise<ApiResponse> {
  return request.get('/crossings', { params }) as unknown as Promise<ApiResponse>
}
export function getCrossing(id: number | string): Promise<ApiResponse> {
  return request.get(`/crossings/${id}`) as unknown as Promise<ApiResponse>
}
export function createCrossing(data: Partial<CrossingItem>): Promise<ApiResponse> {
  return request.post('/crossings', data) as unknown as Promise<ApiResponse>
}
export function updateCrossing(id: number | string, data: Partial<CrossingItem>): Promise<ApiResponse> {
  return request.put(`/crossings/${id}`, data) as unknown as Promise<ApiResponse>
}
export function deleteCrossing(id: number | string): Promise<ApiResponse> {
  return request.delete(`/crossings/${id}`) as unknown as Promise<ApiResponse>
}
export function getCrossingDevices(id: number | string): Promise<ApiResponse> {
  return request.get(`/crossings/${id}/devices`) as unknown as Promise<ApiResponse>
}

// ---------------- 行政区划 areas ----------------
export interface AreaItem {
  id: number
  code: string
  name: string
  area_type: string
  full_name?: string
  parent_id?: number | null
  [key: string]: any
}

export function getAreasTree(): Promise<ApiResponse> {
  return request.get('/areas/tree') as unknown as Promise<ApiResponse>
}
export function createArea(data: Partial<AreaItem>): Promise<ApiResponse> {
  return request.post('/areas', data) as unknown as Promise<ApiResponse>
}
export function updateArea(id: number | string, data: Partial<AreaItem>): Promise<ApiResponse> {
  return request.put(`/areas/${id}`, data) as unknown as Promise<ApiResponse>
}
export function deleteArea(id: number | string): Promise<ApiResponse> {
  return request.delete(`/areas/${id}`) as unknown as Promise<ApiResponse>
}

// ---------------- 地图分级数据（P2 可视化前置接口，本期供后端验证） ----------------
export function getCrossingMapData(): Promise<ApiResponse> {
  return request.get('/map/crossing-data') as unknown as Promise<ApiResponse>
}
export function getRoadMapData(): Promise<ApiResponse> {
  return request.get('/map/road-data') as unknown as Promise<ApiResponse>
}
