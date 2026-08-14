import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 看板概览数据
export function getOverview(): Promise<ApiResponse> {
  return request.get('/dashboard/overview') as unknown as Promise<ApiResponse>
}

// 故障类型统计
export function getFaultTypeStats(days?: number): Promise<ApiResponse> {
  return request.get('/dashboard/fault-type-stats', { params: { days } }) as unknown as Promise<ApiResponse>
}

// 工单状态统计
export function getWorkOrderStats(): Promise<ApiResponse> {
  return request.get('/dashboard/work-order-stats') as unknown as Promise<ApiResponse>
}

// 故障趋势统计
export function getFaultTrend(params: { dimension?: string; days?: number }): Promise<ApiResponse> {
  return request.get('/dashboard/fault-trend', { params }) as unknown as Promise<ApiResponse>
}

// 设备故障排行
export function getDeviceFaultRank(params: { limit?: number; days?: number }): Promise<ApiResponse> {
  return request.get('/dashboard/device-fault-rank', { params }) as unknown as Promise<ApiResponse>
}

// 工单平均闭环时长
export function getWorkOrderAvgClosure(params: { days?: number }): Promise<ApiResponse> {
  return request.get('/dashboard/work-order-avg-closure', { params }) as unknown as Promise<ApiResponse>
}
