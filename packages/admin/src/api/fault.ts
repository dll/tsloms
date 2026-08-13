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

// 获取故障列表（分页）
export function getFaults(params: FaultQuery): Promise<ApiResponse> {
  return request.get('/faults', { params }) as unknown as Promise<ApiResponse>
}

// 获取故障详情
export function getFault(id: number | string): Promise<ApiResponse> {
  return request.get(`/faults/${id}`) as unknown as Promise<ApiResponse>
}
