import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 故障级 AI 建议（确认/派单辅助）
export interface FaultAdvice {
  fault_id: number
  device_hw_id: string
  summary: string
  priority: string
  priority_text: string
  plan: string
  parts: string[]
  content: string
  source: string
  tokens_used: number
}

// 工单级 AI 建议（AI 辅助）
export interface WorkOrderAdvice {
  work_order_id: number
  device_hw_id: string
  root_cause: string
  steps: string[]
  parts: string[]
  summary: string
  content: string
  source: string
  tokens_used: number
}

// 故障 AI 建议
export function getFaultAdvice(id: number): Promise<ApiResponse> {
  return request.get(`/ai/advice/fault/${id}`) as unknown as Promise<ApiResponse>
}

// 工单 AI 建议（stage: copilot / summary）
export function getWorkOrderAdvice(id: number, stage = 'copilot'): Promise<ApiResponse> {
  return request.get(`/ai/advice/workorder/${id}`, { params: { stage } }) as unknown as Promise<ApiResponse>
}

// 设备新建/编辑 AI 辅助（依据字段给填写/配置建议）
export interface DeviceAdvice {
  summary: string
  hints: string[]
  issues: string[]
  source: string
}
export function getDeviceAdvice(payload: Record<string, any>): Promise<ApiResponse> {
  return request.post('/ai/advice/device', payload) as unknown as Promise<ApiResponse>
}

// 建单 AI 辅助（基于关联故障推荐优先级/备件/步骤/维修人）
export interface WorkOrderCreateAdvice {
  fault_id: number
  device_hw_id: string
  priority: string
  priority_text: string
  parts: string[]
  steps: string[]
  repairer_hint: string
  summary: string
  source: string
}
export function getWorkOrderCreateAdvice(faultId: number): Promise<ApiResponse> {
  return request.post('/ai/advice/workorder/create', { fault_id: faultId }) as unknown as Promise<ApiResponse>
}

// 采购 AI 辅助（合理性校验 + 供应商建议）
export interface PurchaseAdvice {
  summary: string
  checks: string[]
  suggestions: string[]
  supplier_hint: string
  source: string
}
export function getPurchaseAdvice(items: { material_name: string; quantity: number; price: number }[], supplierId: number): Promise<ApiResponse> {
  return request.post('/ai/advice/purchase', { items, supplier_id: supplierId }) as unknown as Promise<ApiResponse>
}

// 建议历史
export function getAdvices(bizType?: string, bizId?: number): Promise<ApiResponse> {
  return request.get('/ai/advices', { params: { biz_type: bizType, biz_id: bizId } }) as unknown as Promise<ApiResponse>
}

// ---- L5 AI 自然语言交互（对话级） ----
// 顶部 AI 助手：用户自然语言 → 意图识别 → 工具执行 → 结构化回答

export interface NLAnswer {
  reply: string
  intent: 'query' | 'command' | 'fallback'
  tool: string
  data?: Record<string, any>
  source: 'LLM' | '规则'
  tokens_used: number
  did_write: boolean
  created_id: number
  confidence: number
}

// AI 助手对话
export function nlInteract(text: string): Promise<ApiResponse> {
  return request.post('/ai/nl/interact', { text }) as unknown as Promise<ApiResponse>
}

// ---- L6 AI 自主决策（决策建议中心） ----

// 运维健康评分维度
interface HealthDimension {
  key: string
  name: string
  score: number
  level: 'good' | 'warn' | 'bad'
  hint: string
}

// 运维健康评分
interface OpsHealth {
  total: number
  level: 'good' | 'warn' | 'bad'
  grade: string
  dimensions: HealthDimension[]
  summary: string
  at: string
}

// 决策建议
interface DecisionSuggestion {
  category: string
  title: string
  detail: string
  priority: 'high' | 'medium' | 'low'
  action: 'purchase' | 'assign' | 'none'
  action_hint: string
  data: { name: string; value: number }[]
}

// 决策中心结果
interface DecisionCenterResult {
  health: OpsHealth
  decisions: DecisionSuggestion[]
  summary: string
  source: string
  tokens_used: number
}

// 运维健康评分 + 决策建议
function getDecisionCenter(): Promise<ApiResponse> {
  return request.post('/ai/decision/center', {}) as unknown as Promise<ApiResponse>
}

// 一键采纳建议（备件采购 → 生成采购草稿单）
function adoptDecision(payload: {
  category: string
  title: string
  supplier_id: number
  items: { material_name: string; quantity: number; price: number }[]
}): Promise<ApiResponse> {
  return request.post('/ai/decision/adopt', payload) as unknown as Promise<ApiResponse>
}

// ---- L6 实时异常流检测 ----

// 异常事件
interface AnomalyEvent {
  id: number
  time: string
  kind: string
  level: 'critical' | 'major' | 'minor' | 'info'
  device_hw_id: string
  title: string
  detail: string
  biz_type: string
  biz_id: number
}

// 异常流结果
interface AnomalyStreamResult {
  events: AnomalyEvent[]
  total: number
  by_level: Record<string, number>
  summary: string
  source: string
  tokens_used: number
  generated_at: string
}

// 实时异常流检测（含报文告警/故障/超时工单/离线设备）
function getAnomalyStream(hours = 24, limit = 50): Promise<ApiResponse> {
  return request.get('/ai/anomaly/stream', { params: { hours, limit } }) as unknown as Promise<ApiResponse>
}

export {
  getDecisionCenter,
  adoptDecision,
  getAnomalyStream,
  type DecisionCenterResult,
  type OpsHealth,
  type DecisionSuggestion,
  type AnomalyEvent,
  type AnomalyStreamResult,
}
