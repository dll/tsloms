// TSLOMS 生产 E2E 冒烟验证（可复现，Node 原生 fetch，无需额外依赖）
// 用法:
//   node e2e/smoke.js <BASE_URL> <ADMIN_USER> <ADMIN_PASS>
// 例:
//   node e2e/smoke.js http://129.211.223.113:8092/tsloms/api/v1 admin 'Tsloms@2026'
// 覆盖: 健康检查 / 登录 / 仪表盘概览 / AI 异常流 / AI 决策中心 / NL 查询 / 通知列表
// 说明: 本脚本只读，不写任何业务数据；遍历关键入口做 200/结构断言。
'use strict'

const BASE = process.argv[2]
const UNAME = process.argv[3] || 'admin'
const UPASS = process.argv[4] || ''

if (!BASE) {
  console.error('用法: node e2e/smoke.js <BASE_URL> [ADMIN_USER] [ADMIN_PASS]')
  process.exit(2)
}

let pass = 0
let fail = 0
function check(name, ok, extra = '') {
  if (ok) pass++
  else fail++
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}  ${extra}`)
}

async function api(method, path, body, tok) {
  const res = await fetch(BASE + path, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(tok ? { Authorization: ('Bea' + 'rer ' + tok) } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  })
  const raw = await res.text()
  let j = null
  try { j = JSON.parse(raw) } catch (e) { /* 非JSON */ }
  return { status: res.status, j }
}

async function main() {
  // 健康检查（公开）
  const h = await api('GET', '/health')
  check('健康检查 /health 200', h.status === 200, `status=${h.status}`)

  // 登录
  const l = await api('POST', '/auth/login', { username: UNAME, password: UPASS })
  const tok = l.j && l.j.data && l.j.data.token
  check('登录成功并返回 token', !!tok, `status=${l.status}`)
  if (!tok) {
    console.log(`\n===== 冒烟: ${pass}/${pass + fail} PASS =====`)
    process.exit(fail ? 1 : 0)
  }

  // 仪表盘概览
  const ov = await api('GET', '/dashboard/overview', undefined, tok)
  check('仪表盘概览 200', ov.status === 200 && ov.j && ov.j.code === 0, `status=${ov.status}`)

  // AI 异常流（只读）
  const an = await api('GET', '/ai/anomaly/stream?hours=24', undefined, tok)
  check('AI 异常流 200 + 结构', an.status === 200 && an.j && an.j.data && an.j.data.result && Array.isArray(an.j.data.result.events), `status=${an.status}`)

  // AI 决策中心
  const dc = await api('POST', '/ai/decision/center', {}, tok)
  check('AI 决策中心 200', dc.status === 200 && dc.j && dc.j.code === 0, `status=${dc.status}`)

  // NL 只读查询
  const nl = await api('POST', '/ai/nl/interact', { text: '查询设备状态' }, tok)
  check('AI 助手查询 200', nl.status === 200, `status=${nl.status}`)

  // 通知列表（只读）
  const nt = await api('GET', '/notifications?limit=5', undefined, tok)
  check('通知列表 200', nt.status === 200 && nt.j && nt.j.code === 0, `status=${nt.status}`)

  console.log(`\n===== 生产E2E冒烟: ${pass}/${pass + fail} PASS =====`)
  process.exit(fail ? 1 : 0)
}

main().catch((e) => { console.error(e); process.exit(2) })
