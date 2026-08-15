// 全局事件总线（基于 mitt，极轻量、类型友好、无组件重挂载）
// 用于跨组件通信：地图聚焦、设备定位等
import mitt, { type Emitter } from 'mitt'

// 事件类型定义
export type MapFocusPayload = {
  // 聚焦类型
  kind: 'device' | 'intersection' | 'route' | 'block' | 'aggregate'
  // 目标名称（路口名/路名/街区名），用于回显
  name?: string
  // 目标坐标（路口/设备聚合中心）
  lat?: number
  lng?: number
  // 聚焦高度（米）；不传则由 kind 决定
  height?: number
  // 设备相关
  hw_id?: number
}

export type BusEvents = {
  /** 地图聚焦定位 */
  'map:focus': MapFocusPayload
  /** 设备关注状态变更（is_watched） */
  'device:watched': { hw_id: number; is_watched: boolean }
}

// 单例事件总线
export const bus: Emitter<BusEvents> = mitt<BusEvents>()

// 待处理的地图聚焦（跨路由导航）：
// 从路口/设备列表“去地图”时，先 router.push 再 emit，地图组件尚未挂载会错过事件。
// 因此额外缓存最后一次聚焦，供 CesiumMap 挂载时读取。
let pendingFocus: MapFocusPayload | null = null

// 便捷封装：触发地图聚焦（同时缓存供后续挂载的地图读取）
export function emitMapFocus(payload: MapFocusPayload) {
  pendingFocus = { ...payload }
  bus.emit('map:focus', payload)
}

// 读取并清除待处理聚焦（CesiumMap 挂载时调用）
export function consumePendingFocus(): MapFocusPayload | null {
  const p = pendingFocus
  pendingFocus = null
  return p
}
