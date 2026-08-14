import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 设备列表查询参数
export interface DeviceQuery {
  page?: number
  page_size?: number
  intersection?: string
  online_status?: string
  hw_id?: string
}

// 设备更新参数
export interface DeviceUpdate {
  intersection?: string
  network_code?: string
  station_code?: string
  installed_at?: string
  lat?: number
  lng?: number
}

// 获取设备列表（分页）
export function getDevices(params: DeviceQuery): Promise<ApiResponse> {
  return request.get('/devices', { params }) as unknown as Promise<ApiResponse>
}

// 获取设备统计数据
export function getDeviceStats(): Promise<ApiResponse> {
  return request.get('/devices/stats') as unknown as Promise<ApiResponse>
}

// 获取设备详情
export function getDevice(id: number | string): Promise<ApiResponse> {
  return request.get(`/devices/${id}`) as unknown as Promise<ApiResponse>
}

// 更新设备信息
export function updateDevice(id: number | string, data: DeviceUpdate): Promise<ApiResponse> {
  return request.put(`/devices/${id}`, data) as unknown as Promise<ApiResponse>
}
