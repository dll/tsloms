import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 登录接口：username(可手机号) + password + 算术验证码(captcha_uuid/captcha_code)
export function login(data: { username: string; password: string; captcha_uuid: string; captcha_code: string }): Promise<ApiResponse> {
  return request.post('/auth/login', data) as unknown as Promise<ApiResponse>
}

// 获取算术验证码（参考项目 a 的图形验证码简化版：GET /auth/captcha 返回 uuid + 算式题目，如 "2 + 8 = ?"）
export function getCaptcha(): Promise<ApiResponse> {
  return request.get('/auth/captcha') as unknown as Promise<ApiResponse>
}

// 更新当前用户个人资料（人事字段/手机号等）
export function updateMyProfile(data: Record<string, unknown>): Promise<ApiResponse> {
  return request.put('/user/profile', data) as unknown as Promise<ApiResponse>
}

// 上传工作照/头像（multipart，字段名 file）
export function uploadMyAvatar(formData: FormData): Promise<ApiResponse> {
  return request.post('/user/avatar', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  }) as unknown as Promise<ApiResponse>
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
