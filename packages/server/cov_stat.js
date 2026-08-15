const fs = require('fs')
// 用法: node cov_stat.js <coverage.out 绝对路径>
const file = process.argv[2] || 'coverage.out'
const lines = fs.readFileSync(file, 'utf8').split('\n')
const pkg = {}
let totalS = 0, totalC = 0
for (const l of lines) {
  if (!l || l.startsWith('mode:')) continue
  const i = l.lastIndexOf(' ')
  const j = l.lastIndexOf(' ', i - 1)
  const numStmt = parseInt(l.slice(j + 1, i))
  const count = parseInt(l.slice(i + 1))
  const f = l.slice(0, l.indexOf(':'))
  const mm = f.match(/\/internal\/([^/]+)\//)
  const p = mm ? mm[1] : (f.includes('/cmd/') ? 'cmd' : 'other')
  if (!pkg[p]) pkg[p] = { s: 0, c: 0 }
  pkg[p].s += numStmt
  if (count > 0) pkg[p].c += numStmt
  totalS += numStmt; if (count > 0) totalC += numStmt
}
console.log('=== 各包覆盖率 ===')
for (const [p, v] of Object.entries(pkg).sort((a, b) => b[1].s - a[1].s)) {
  console.log(`${p.padEnd(10)} total=${String(v.s).padStart(6)} covered=${String(v.c).padStart(6)}  ${((v.c / v.s) * 100).toFixed(1).padStart(5)}%`)
}
console.log(`\nTOTAL covered/total = ${totalC}/${totalS} = ${(totalC / totalS * 100).toFixed(1)}%`)
