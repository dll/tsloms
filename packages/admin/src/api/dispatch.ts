import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 派单参考：设备聚合信息（故障/工单/耗材/媒体）
export interface DispatchReference {
  device_hw_id: string
  faults: Array<{ id: number; err_code: number; fault_type: string; fault_level: string; last_seen: string }>
  work_orders: Array<{ id: number; order_no: string; status: string; created_at: string }>
  materials: Array<{ id: number; name: string; part_no?: string; quantity: number; unit?: string }>
  media: Array<{ id: number; media_type: string; category: string; title?: string; url: string }>
}

// 获取设备派单参考
export function getDispatchReference(device_hw_id: string): Promise<ApiResponse> {
  return request.get('/dispatch/reference', { params: { device_hw_id } }) as unknown as Promise<ApiResponse>
}
