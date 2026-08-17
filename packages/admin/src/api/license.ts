import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// ============================================================================
// 授权/试用管理（仅超级管理员 module:manage）
// ============================================================================

// 查询整体授权状态（核心剩余天数 / 各模块状态）
export function getLicenseStatus(): Promise<ApiResponse> {
  return request.get('/license/status') as unknown as Promise<ApiResponse>
}

// 开始试用（core 或可选模块 key）；缺省 module=core
export function startTrial(module?: string): Promise<ApiResponse> {
  return request.post('/license/trial/start', { module: module || 'core' }) as unknown as Promise<ApiResponse>
}

// 解锁：code 为空=超管一键解锁(长期)；有=验签授权码
export function unlockLicense(module: string, code?: string): Promise<ApiResponse> {
  return request.post('/license/unlock', { module, code: code || '' }) as unknown as Promise<ApiResponse>
}

// 授权状态项
export interface LicenseStatusItem {
  key: string
  name: string
  core: boolean
  state: string // pending/trial/expired/unlocked
  activated_at?: string
  trial_expiry?: string
  unlock_type?: string
  remain_days?: number
}
