import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 问题反馈
export interface Feedback {
  id: number
  device_hw_id?: number
  intersection?: string
  title: string
  content?: string
  reporter?: string
  contact?: string
  status: 'open' | 'processing' | 'resolved' | 'closed'
  work_order_id?: number
  image_url?: string
  created_at: string
}

// 反馈列表
export function getFeedbacks(params: {
  page?: number
  page_size?: number
  status?: string
  device_hw_id?: string
  start_time?: string
  end_time?: string
  sort_by?: string
  order?: string
}): Promise<ApiResponse> {
  return request.get('/feedbacks', { params }) as unknown as Promise<ApiResponse>
}

// 提交反馈（地图/后台）——关联设备必填
export function createFeedback(data: { device_hw_id: number; intersection?: string; title: string; content?: string; reporter?: string; contact?: string }): Promise<ApiResponse> {
  return request.post('/feedbacks', data) as unknown as Promise<ApiResponse>
}

// 更新反馈状态
export function updateFeedback(id: number, data: { status?: string; work_order_id?: number }): Promise<ApiResponse> {
  return request.put(`/feedbacks/${id}`, data) as unknown as Promise<ApiResponse>
}
