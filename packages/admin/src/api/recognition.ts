import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// ============================================================================
// 智能多源故障识别研判引擎 —— 前端 API 封装（范围B）
// 后端 handler/recognition.go 提供的 REST 接口一一对应，兼容既有 /faults* 契约。
// ============================================================================

// ---------------- 多源证据 ----------------

// GET /faults/:id/evidence —— 拉取某起故障的多源证据明细
export interface FaultEvidence {
  id: number
  fault_id: number | null
  evaluation_id: string
  device_hw_id: string
  source_type: string
  err_code?: number | null
  led_state?: number | null
  current_r?: number | null
  current_y?: number | null
  current_g?: number | null
  raw_data: string
  ref_media_id?: number | null
  ref_feedback_id?: number | null
  captured_at: string
  confidence: number
  created_at: string
}

export interface FaultEvidenceResult {
  fault_id: number
  list: FaultEvidence[]
  total: number
}

export function getFaultEvidence(id: number | string): Promise<ApiResponse<FaultEvidenceResult>> {
  return request.get(`/faults/${id}/evidence`) as unknown as Promise<ApiResponse<FaultEvidenceResult>>
}

// POST /evidence/ingest —— 预留外部数据源证据写入
export interface IngestEvidencePayload {
  device_hw_id: string
  source_type: string
  err_code?: number | null
  led_state?: number | null
  current_r?: number | null
  current_y?: number | null
  current_g?: number | null
  raw_data?: string
  ref_media_id?: number | null
  ref_feedback_id?: number | null
  captured_at?: string
  fault_id?: number | null
  confidence?: number | null
}
export function ingestEvidence(payload: IngestEvidencePayload): Promise<ApiResponse> {
  return request.post('/evidence/ingest', payload) as unknown as Promise<ApiResponse>
}

// GET /evidence/sources —— 多源证据类型/来源枚举
export function listEvidenceSources(): Promise<ApiResponse<{ list: string[] }>> {
  return request.get('/evidence/sources') as unknown as Promise<ApiResponse<{ list: string[] }>>
}

// ---------------- 案例库 ----------------

export interface FaultCase {
  id: number
  device_hw_id: string
  fault_type: string
  fault_level: string
  input_signature: string
  evidence_summary: string
  expected_result: string
  judged_result: string
  judge_confidence?: number | null
  is_correct?: boolean | null
  source_evaluation_id: string
  status: string
  created_at: string
  updated_at: string
}

export interface FaultCasesResult {
  list: FaultCase[]
  total: number
  page: number
  page_size: number
}

// GET /fault-cases —— 案例库检索/列表
export function listFaultCases(params: Record<string, any> = {}): Promise<ApiResponse<FaultCasesResult>> {
  return request.get('/fault-cases', { params }) as unknown as Promise<ApiResponse<FaultCasesResult>>
}

// POST /fault-cases —— 案例库新增/人工回标
export interface CreateFaultCasePayload {
  device_hw_id: string
  input_signature?: string
  fault_type?: string
  fault_level?: string
  expected_result?: string
  judged_result?: string
  evidence_summary?: string
  source_evaluation_id?: string
  status?: string
}
export function createFaultCase(payload: CreateFaultCasePayload): Promise<ApiResponse> {
  return request.post('/fault-cases', payload) as unknown as Promise<ApiResponse>
}

// POST /fault-cases/train —— 触发案例库训练
export interface TrainFaultCasesResult {
  total_cases: number
  correct_cases: number
  accuracy: number
  recognize_100pct: boolean
  score_mode: string
  training_status: string
}
export function trainFaultCases(): Promise<ApiResponse<TrainFaultCasesResult>> {
  return request.post('/fault-cases/train') as unknown as Promise<ApiResponse<TrainFaultCasesResult>>
}

// ---------------- 识别统计 ----------------

export interface RecognitionStats {
  total_cases: number
  accuracy: number
  false_positive: number
  false_negative: number
  false_positive_rate: number
  false_negative_rate: number
  confirmed_or_seed: number
  filtered_as_normal: number
}

// GET /recognition/stats —— 识别准确率/误报/漏报统计
export function getRecognitionStats(): Promise<ApiResponse<RecognitionStats>> {
  return request.get('/recognition/stats') as unknown as Promise<ApiResponse<RecognitionStats>>
}

// ---------------- 待确认复核 ----------------

// POST /faults/:id/review —— 确认真故障 / 标记误报
export function reviewFault(id: number | string, confirmed: boolean): Promise<ApiResponse> {
  return request.post(`/faults/${id}/review`, { confirmed }) as unknown as Promise<ApiResponse>
}

// ============================================================================
// 展示辅助：研判状态 / 证据来源映射（与后端常量对齐）
// ============================================================================

// 研判分流状态
export const RECOGNITION_STATUSES = [
  { value: 'confirmed', label: '已确认', tag: 'success' },
  { value: 'pending_review', label: '待复核', tag: 'warning' },
  { value: 'filtered', label: '已过滤', tag: 'info' },
]

export function recognitionStatusLabel(s?: string): string {
  return RECOGNITION_STATUSES.find((x) => x.value === s)?.label || s || '-'
}
export function recognitionStatusTag(s?: string): string {
  return RECOGNITION_STATUSES.find((x) => x.value === s)?.tag || 'info'
}

// 证据来源枚举（EV_source 常量 → 中文）
export const EVIDENCE_SOURCES: Record<string, string> = {
  firmware: '固件',
  current: '电流',
  led_state: '灯态',
  citizen: '群众反映',
  photo_evidence: '手机举证',
  video_monitor: '视频监控',
}
export function evidenceSourceLabel(s?: string): string {
  return (s && EVIDENCE_SOURCES[s]) || s || '-'
}

// 判定来源（rule / multi-source / case）
export const RECOGNITION_SOURCES: Record<string, string> = {
  rule: '规则',
  'multi-source': '多源融合',
  case: '案例命中',
}
export function recognitionSourceLabel(s?: string): string {
  return (s && RECOGNITION_SOURCES[s]) || s || '-'
}

// 置信度格式化（0-1 → 百分比）
export function fmtConfidence(c?: number | null): string {
  if (c == null || Number.isNaN(c)) return '-'
  return `${Math.round(c * 100)}%`
}
