import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 报文日志查询参数
export interface PacketLogQuery {
  page?: number
  page_size?: number
  device_hw_id?: string
  cmd_type?: string
  valid?: string
  start_time?: string
  end_time?: string
}

// 操作日志查询参数
export interface OperationLogQuery {
  page?: number
  page_size?: number
  username?: string
  action?: string
  start_time?: string
  end_time?: string
}

// 获取报文日志（分页）
export function getPacketLogs(params: PacketLogQuery): Promise<ApiResponse> {
  return request.get('/logs/packets', { params }) as unknown as Promise<ApiResponse>
}

// 获取操作日志（分页）
export function getOperationLogs(params: OperationLogQuery): Promise<ApiResponse> {
  return request.get('/logs/operations', { params }) as unknown as Promise<ApiResponse>
}
