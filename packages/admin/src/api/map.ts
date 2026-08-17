import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 设备列表查询（地图需要全量带经纬度设备）
export interface MapDevice {
  id?: number
  hw_id?: number
  intersection?: string
  lat: number | null
  lng: number | null
  online_status?: boolean
}

// 获取全部设备（供地图打点，default 大分页）
export function getAllDevices(pageSize = 1000): Promise<ApiResponse> {
  return request.get('/devices', { params: { page: 1, page_size: pageSize } }) as unknown as Promise<ApiResponse>
}

// 路口聚合数据（供地图按故障比例 绿→黄→红 分级渐变着色）
export function getCrossingMapData(): Promise<ApiResponse> {
  return request.get('/map/crossing-data') as unknown as Promise<ApiResponse>
}

// 道路级聚合（一段一色）
export function getRoadMapData(): Promise<ApiResponse> {
  return request.get('/map/road-data') as unknown as Promise<ApiResponse>
}

// 路口聚合项
export interface CrossingPoly {
  id: number
  name: string
  road_name?: string
  lat?: number | null
  lng?: number | null
  device_total: number
  fault_ratio: number
  green_ratio: number
  level: string // green/yellow_low/yellow/orange/red
}
