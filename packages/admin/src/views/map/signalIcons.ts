// 信号灯 SVG 图标生成器
// 生成多种状态的信号灯图标（data URI），供 Cesium billboard 使用
// 状态：在线(亮三灯)/离线(灰调)/故障(红字/闪烁感)/关注(星标)

// 生成 SVG 字符串 → data URI（供 Cesium billboard ImageUrl）
export function svgToDataUri(svg: string): string {
  const encoded = encodeURIComponent(svg)
  return `data:image/svg+xml;charset=utf-8,${encoded}`
}

// 信号灯图标（中国交通灯：红黄绿纵向排列）
export interface SignalIconOptions {
  online?: boolean   // 是否在线
  fault?: boolean    // 是否故障
  watched?: boolean  // 是否关注/锁定
  size?: number      // 图标边长（px）
}

export function signalIcon(opts: SignalIconOptions = {}): string {
  const size = opts.size || 32
  const online = opts.online !== false
  const fault = !!opts.fault
  const watched = !!opts.watched

  // 灯色：在线亮、离线灰
  const r = online ? '#FF3B30' : '#c0c4cc'
  const y = online ? '#FFC300' : '#c0c4cc'
  const g = online ? '#34C759' : '#c0c4cc'

  // 背景：故障红边/关注金边
  const bg = fault ? 'rgba(255,59,48,0.15)' : watched ? 'rgba(255,195,0,0.15)' : 'transparent'
  const border = fault ? '#FF3B30' : watched ? '#FFB800' : 'rgba(0,0,0,0.15)'

  // 故障标 + 关注标
  const badge = fault
    ? `<rect x="${size-12}" y="0" width="12" height="12" rx="6" fill="#FF3B30"/><text x="${size-6}" y="10" font-size="9" fill="#fff" text-anchor="middle" font-weight="bold">!</text>`
    : watched
      ? `<path d="M ${size-12} 1 l 2 4 4.5 0.6 -3.2 3 0.9 4.4 -4-2.2 -4 2.2 0.9-4.4 -3.2-3 4.5-0.6 z" fill="#FFB800"/>`
      : ''

  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size+4}" viewBox="0 0 ${size} ${size+4}">
    <rect x="1" y="1" width="${size-2}" height="${size+2}" rx="7" fill="${bg}" stroke="${border}" stroke-width="1"/>
    <circle cx="${size/2}" cy="${size*0.22}" r="${size*0.11}" fill="${r}" stroke="rgba(0,0,0,0.2)"/>
    <circle cx="${size/2}" cy="${size*0.5}" r="${size*0.11}" fill="${y}" stroke="rgba(0,0,0,0.2)"/>
    <circle cx="${size/2}" cy="${size*0.78}" r="${size*0.11}" fill="${g}" stroke="rgba(0,0,0,0.2)"/>
    ${badge}
  </svg>`
  return svgToDataUri(svg)
}

// 缓存已生成的图标（避免重复编码）
const cache = new Map<string, string>()
export function getSignalIcon(opts: SignalIconOptions = {}): string {
  const key = `${opts.online !== false}|${!!opts.fault}|${!!opts.watched}|${opts.size || 32}`
  if (cache.has(key)) return cache.get(key)!
  const uri = signalIcon(opts)
  cache.set(key, uri)
  return uri
}
