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
