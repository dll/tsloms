import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 登录接口
// 双通道（P0）：username+password 或 phone+sms_code，后端多态兼容
export function login(data: { username?: string; password?: string; phone?: string; code?: string }): Promise<ApiResponse> {
  return request.post('/auth/login', data) as unknown as Promise<ApiResponse>
}

// 发送手机号登录验证码（P0：可插拔通道，开发环境 Console/测试码）
export function sendSmsCode(phone: string): Promise<ApiResponse> {
  return request.post('/auth/sms-code', { phone }) as unknown as Promise<ApiResponse>
}

// 手机号 + 验证码登录
export function loginByPhone(phone: string, code: string): Promise<ApiResponse> {
  return request.post('/auth/login', { phone, code }) as unknown as Promise<ApiResponse>
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
