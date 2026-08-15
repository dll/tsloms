import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 维修费用
export interface RepairExpense {
  id: number
  expense_no: string
  work_order_id?: number
  device_hw_id: number
  type: string
  amount: number
  description: string
  work_date?: string
  operator: string
  confirmed: boolean
  note: string
  created_at: string
}

// 维修费用列表
export function getExpenses(params: Record<string, any> = {}): Promise<ApiResponse> {
  return request.get('/expenses', { params }) as unknown as Promise<ApiResponse>
}

// 费用统计
export function getExpenseStats(): Promise<ApiResponse> {
  return request.get('/expenses/stats') as unknown as Promise<ApiResponse>
}

// 新增/更新费用
export function saveExpense(data: Partial<RepairExpense>): Promise<ApiResponse> {
  if (data.id) return request.put(`/expenses/${data.id}`, data) as unknown as Promise<ApiResponse>
  return request.post('/expenses', data) as unknown as Promise<ApiResponse>
}

// 确认入账
export function confirmExpense(id: number, confirmed: boolean): Promise<ApiResponse> {
  return request.put(`/expenses/${id}/confirm`, { confirmed }) as unknown as Promise<ApiResponse>
}

// 删除费用
export function deleteExpense(id: number): Promise<ApiResponse> {
  return request.delete(`/expenses/${id}`) as unknown as Promise<ApiResponse>
}
