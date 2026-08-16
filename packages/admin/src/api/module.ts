import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 模块元信息（后端返回）
export interface ModuleInfo {
  key: string
  name: string
  core: boolean
}

// 已启模块列表
export function getEnabledModules(): Promise<ApiResponse<{ modules: ModuleInfo[] }>> {
  return request.get('/modules') as unknown as Promise<ApiResponse<{ modules: ModuleInfo[] }>>
}
