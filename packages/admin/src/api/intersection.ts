import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 路口维度设备统计
export interface IntersectionItem {
  intersection: string
  device_total: number
  online: number
  offline: number
  fault: number
  lat: number | null
  lng: number | null
}

// 获取路口设备统计列表
export function getIntersections(): Promise<ApiResponse> {
  return request.get('/intersections') as unknown as Promise<ApiResponse>
}
