import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 获取 AI 配置（key 脱敏）
export function getAIConfig(): Promise<ApiResponse> {
  return request.get('/ai/config') as unknown as Promise<ApiResponse>
}

// 更新 AI 配置
export function updateAIConfig(data: Record<string, any>): Promise<ApiResponse> {
  return request.put('/ai/config', data) as unknown as Promise<ApiResponse>
}

// 我的今日额度
export function getMyAIUsage(): Promise<ApiResponse> {
  return request.get('/ai/usage') as unknown as Promise<ApiResponse>
}

// 额度使用流水（管理员）
export function getAIUsageLogs(params?: Record<string, any>): Promise<ApiResponse> {
  return request.get('/ai/usage/logs', { params }) as unknown as Promise<ApiResponse>
}

// 重置今日额度（管理员）
export function resetAIUsage(): Promise<ApiResponse> {
  return request.post('/ai/usage/reset') as unknown as Promise<ApiResponse>
}

// 运行全量故障预测（规则引擎 + 可选 LLM 预案增强）
export function runPrediction(): Promise<ApiResponse> {
  return request.post('/ai/predict/run') as unknown as Promise<ApiResponse>
}

// 查询历史预测
export function getPredictions(batchId?: string): Promise<ApiResponse> {
  return request.get('/ai/predict', { params: batchId ? { batch_id: batchId } : {} }) as unknown as Promise<ApiResponse>
}

// 生成单条预测的 LLM 增强预案
export function enhancePredictionPlan(id: number | string): Promise<ApiResponse> {
  return request.post(`/ai/predict/${id}/enhance`) as unknown as Promise<ApiResponse>
}

// AI 故障诊断（基于问题反馈，含图片）
export function diagnoseFeedback(id: number | string): Promise<ApiResponse> {
  return request.post(`/ai/diagnose/${id}`) as unknown as Promise<ApiResponse>
}

// AI 生命周期溯源
export function buildLifecycle(hwid: number | string): Promise<ApiResponse> {
  return request.get(`/ai/lifecycle/${hwid}`) as unknown as Promise<ApiResponse>
}
