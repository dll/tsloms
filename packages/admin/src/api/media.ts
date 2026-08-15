import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 媒体类型
export type MediaType = 'evidence' | 'monitoring' | 'timelapse'

// 设备媒体
export interface DeviceMedia {
  id: number
  device_hw_id: number
  media_type: MediaType
  category: 'photo' | 'video'
  title: string
  source: 'upload' | 'rtsp' | 'url'
  url: string
  compatible_url?: string
  thumbnail?: string
  duration?: number
  note?: string
  uploaded_by?: string
  created_at: string
  // 信号灯信息（举证必填）
  intersection?: string
  light_color?: string
  fault_desc?: string
  is_active_fault?: boolean
}

export interface MediaQuery {
  page?: number
  page_size?: number
  device_hw_id?: string
  media_type?: string
  source?: string
}

// 媒体查询
export function getDeviceMedia(params: MediaQuery): Promise<ApiResponse> {
  return request.get('/media', { params }) as unknown as Promise<ApiResponse>
}

// 手机上传媒体（举证/图片/短视频）
export function uploadDeviceMedia(formData: FormData): Promise<ApiResponse> {
  return request.post('/media/upload', formData, { headers: { 'Content-Type': 'multipart/form-data' } }) as unknown as Promise<ApiResponse>
}

// 登记 RTSP / 云URL 监控或时间视频
export interface StreamBody {
  device_hw_id: number
  media_type: MediaType
  title?: string
  url: string
  compatible_url?: string
  thumbnail?: string
  duration?: number
  note?: string
  intersection?: string
}
export function createStreamMedia(data: StreamBody): Promise<ApiResponse> {
  return request.post('/media/streams', data) as unknown as Promise<ApiResponse>
}

// 删除媒体
export function deleteDeviceMedia(id: number): Promise<ApiResponse> {
  return request.delete(`/media/${id}`) as unknown as Promise<ApiResponse>
}
