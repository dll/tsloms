import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 登录接口
export function login(data: { username: string; password: string }): Promise<ApiResponse> {
  return request.post('/auth/login', data) as unknown as Promise<ApiResponse>
}

// 获取当前用户信息
export function getUserInfo(): Promise<ApiResponse> {
  return request.get('/user/info') as unknown as Promise<ApiResponse>
}

// 修改当前用户手机号
export function updateMyPhone(phone: string): Promise<ApiResponse> {
  return request.put('/user/phone', { phone }) as unknown as Promise<ApiResponse>
}

// 设置当前用户地图中心点（该用户管辖区域）
export function updateMyCenter(lat: number | null, lng: number | null): Promise<ApiResponse> {
  return request.put('/user/center', { lat, lng }) as unknown as Promise<ApiResponse>
}
