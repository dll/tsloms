import { describe, it, expect } from 'vitest'
import { levelTag, fmtTime } from '../src/utils/format'

describe('levelTag 异常等级映射', () => {
  it('critical → danger', () => {
    expect(levelTag('critical')).toBe('danger')
  })
  it('major → warning', () => {
    expect(levelTag('major')).toBe('warning')
  })
  it('minor → info', () => {
    expect(levelTag('minor')).toBe('info')
  })
  it('info → success', () => {
    expect(levelTag('info')).toBe('success')
  })
  it('未知等级 → info 兜底', () => {
    expect(levelTag('unknown')).toBe('info')
  })
  it('空串 → info 兜底', () => {
    expect(levelTag('')).toBe('info')
  })
})

describe('fmtTime 时间格式化', () => {
  it('空输入 → 空串', () => {
    expect(fmtTime('')).toBe('')
  })
  it('合法本地时间 → MM-DD HH:mm', () => {
    // 构造本地时间，避免时区差异造成断言不稳
    const d = new Date(2026, 7, 15, 23, 38) // 2026-08-15 23:38 本地
    expect(fmtTime(d.toISOString())).toBe('08-15 23:38')
  })
  it('月份/日期补零', () => {
    const d = new Date(2026, 0, 5, 9, 7) // 2026-01-05 09:07 本地
    expect(fmtTime(d.toISOString())).toBe('01-05 09:07')
  })
  it('非法时间 → 原样返回', () => {
    expect(fmtTime('not-a-date')).toBe('not-a-date')
  })
})
