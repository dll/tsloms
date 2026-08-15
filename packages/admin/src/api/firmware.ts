import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 固件包
export interface FirmwarePackage {
  id: number
  version: string
  major: number
  minor: number
  build: number
  sw_version: number
  file_name: string
  file_path: string
  size: number
  md5: string
  description: string
  published: boolean
  published_at?: string
  uploader: string
  created_at: string
}

// 设备固件升级记录
export interface FirmwareUpgrade {
  id: number
  firmware_id: number
  device_hw_id: number
  target_version: string
  status: string
  error_msg?: string
  started_at?: string
  finished_at?: string
  created_at: string
}

// 固件包列表（分页）
export function getFirmwares(params: { page?: number; page_size?: number; published?: boolean } = {}): Promise<ApiResponse> {
  return request.get('/firmwares', { params }) as unknown as Promise<ApiResponse>
}

// 固件包详情
export function getFirmwareDetail(id: number): Promise<ApiResponse> {
  return request.get(`/firmwares/${id}`) as unknown as Promise<ApiResponse>
}

// 上传固件包（multipart: version, description, file）
export function uploadFirmware(form: FormData): Promise<ApiResponse> {
  return request.post('/firmwares/upload', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  }) as unknown as Promise<ApiResponse>
}

// 更新固件包说明
export function updateFirmware(id: number, data: { version?: string; description?: string }): Promise<ApiResponse> {
  return request.put(`/firmwares/${id}`, data) as unknown as Promise<ApiResponse>
}

// 发布/下线固件
export function publishFirmware(id: number, published: boolean): Promise<ApiResponse> {
  return request.put(`/firmwares/${id}/publish`, { published }) as unknown as Promise<ApiResponse>
}

// 删除固件包
export function deleteFirmware(id: number): Promise<ApiResponse> {
  return request.delete(`/firmwares/${id}`) as unknown as Promise<ApiResponse>
}

// 升级记录列表
export function getFirmwareUpgrades(params: { page?: number; page_size?: number; device_hw_id?: string; status?: string } = {}): Promise<ApiResponse> {
  return request.get('/firmware-upgrades', { params }) as unknown as Promise<ApiResponse>
}

// 发起设备升级
export function createFirmwareUpgrade(data: { device_hw_id: number; firmware_id: number }): Promise<ApiResponse> {
  return request.post('/firmware-upgrades', data) as unknown as Promise<ApiResponse>
}

// 删除升级记录
export function deleteFirmwareUpgrade(id: number): Promise<ApiResponse> {
  return request.delete(`/firmware-upgrades/${id}`) as unknown as Promise<ApiResponse>
}
