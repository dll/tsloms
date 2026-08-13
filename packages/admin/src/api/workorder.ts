import request from '@/utils/request'
import type { ApiResponse } from '@/utils/request'

// 工单列表查询参数
export interface WorkOrderQuery {
  page?: number
  page_size?: number
  device_hw_id?: string
  status?: string
  assignee_id?: number
  order_no?: string
}

// 创建工单参数
export interface WorkOrderCreate {
  fault_id: number
  device_hw_id: string
  assignee_id: number
}

// 更新工单状态参数
export interface WorkOrderStatusUpdate {
  status: 'processing' | 'completed' | 'rejected'
  result?: string
}

// 获取工单列表（分页）
export function getWorkOrders(params: WorkOrderQuery): Promise<ApiResponse> {
  return request.get('/work-orders', { params }) as unknown as Promise<ApiResponse>
}

// 创建工单
export function createWorkOrder(data: WorkOrderCreate): Promise<ApiResponse> {
  return request.post('/work-orders', data) as unknown as Promise<ApiResponse>
}

// 更新工单状态
export function updateWorkOrderStatus(id: number | string, data: WorkOrderStatusUpdate): Promise<ApiResponse> {
  return request.put(`/work-orders/${id}/status`, data) as unknown as Promise<ApiResponse>
}
