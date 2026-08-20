import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 检测器接入状态
export function getAccessStatus(): Promise<ApiResponse> {
  return request.get('/access/status') as unknown as Promise<ApiResponse>
}
export function createMqttCredentials(): Promise<ApiResponse> {
  return request.post('/access/mqtt-credentials') as unknown as Promise<ApiResponse>
}

// Mock 模拟发送一条协议帧
export interface MockSendData {
  hw_id: number
  cmd?: string        // checkin / alarm / power_on
  err_code?: number
  led_state?: number  // 0红 / 1黄 / 2绿 / -1未知
  current_r?: number
  current_y?: number
  current_g?: number
}
export function mockSend(data: MockSendData): Promise<ApiResponse> {
  return request.post('/access/mock/send', data) as unknown as Promise<ApiResponse>
}

// CSV 导入并回放
export interface CsvRow {
  hw_id: number
  cmd?: string
  err_code?: number
  led_state?: number
  current_r?: number
  current_y?: number
  current_g?: number
}
export interface CsvImportData {
  content?: string
  rows?: CsvRow[]
}
export function csvImport(data: CsvImportData): Promise<ApiResponse> {
  return request.post('/access/csv/import', data) as unknown as Promise<ApiResponse>
}
