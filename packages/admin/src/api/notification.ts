import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 通知项
export interface NotificationItem {
  id: number
  type: string // report/alert/system
  title: string
  content: string
  link: string
  biz_type: string
  biz_id: number
  is_read: boolean
  created_at: string
}

// 我的通知列表
export function getNotifications(limit = 30): Promise<ApiResponse> {
  return request.get('/notifications', { params: { limit } }) as unknown as Promise<ApiResponse>
}

// 未读数
export function getUnreadCount(): Promise<ApiResponse> {
  return request.get('/notifications/unread-count') as unknown as Promise<ApiResponse>
}

// 单条已读
export function readNotification(id: number): Promise<ApiResponse> {
  return request.put(`/notifications/${id}/read`) as unknown as Promise<ApiResponse>
}

// 全部已读
export function readAllNotifications(): Promise<ApiResponse> {
  return request.put('/notifications/read-all') as unknown as Promise<ApiResponse>
}
