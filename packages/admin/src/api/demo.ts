import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 系统演示状态
export function getDemoStatus(): Promise<ApiResponse> {
  return request.get('/demo/status') as unknown as Promise<ApiResponse>
}
// 生成随机演示数据
export function demoStart(n?: number): Promise<ApiResponse> {
  return request.post('/demo/start', { n: n ?? 5 }) as unknown as Promise<ApiResponse>
}
// 一键清理演示数据（回滚）
export function demoEnd(): Promise<ApiResponse> {
  return request.post('/demo/end') as unknown as Promise<ApiResponse>
}
