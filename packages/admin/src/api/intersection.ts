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

// 重命名路口（批量影响该路口下所有设备）
export function renameIntersection(oldName: string, newName: string): Promise<ApiResponse> {
  return request.put('/intersections/rename', { old: oldName, new: newName }) as unknown as Promise<ApiResponse>
}

// 设置路口经纬度（同步该路口下所有设备）
export function setIntersectionLocation(intersection: string, lat: number, lng: number): Promise<ApiResponse> {
  return request.put('/intersections/location', { intersection, lat, lng }) as unknown as Promise<ApiResponse>
}

// 清空路口（该路口设备回到未分配）
export function clearIntersection(intersection: string): Promise<ApiResponse> {
  return request.delete('/intersections/clear', { params: { intersection } }) as unknown as Promise<ApiResponse>
}
