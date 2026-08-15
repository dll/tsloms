// 通用展示格式化工具（纯函数，便于单元测试）

// 异常/告警等级 → Element Plus tag 类型映射
export function levelTag(lvl: string): string {
  const map: Record<string, { label: string; type: string }> = {
    critical: { label: '严重', type: 'danger' },
    major: { label: '重要', type: 'warning' },
    minor: { label: '次要', type: 'info' },
    info: { label: '提示', type: 'success' },
  }
  const m = map[lvl] || { label: lvl, type: 'info' }
  return m.type
}

// 时间字符串 → 紧凑展示（MM-DD HH:mm）
// 输入为 ISO 字符串（如 2026-08-15T23:38:13+08:00）；无法解析时原样返回
export function fmtTime(t: string): string {
  if (!t) return ''
  const d = new Date(t)
  if (isNaN(d.getTime())) return t
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}
