const fs = require('fs')
// 用法: node cov_file.js <coverage.out 绝对路径>
const file = process.argv[2] || 'coverage.out'
const lines = fs.readFileSync(file, 'utf8').split('\n')
const pkgFile = {}
let totalS = 0, totalC = 0
for (const l of lines) {
  if (!l || l.startsWith('mode:')) continue
  const i = l.lastIndexOf(' ')
  const j = l.lastIndexOf(' ', i - 1)
  const numStmt = parseInt(l.slice(j + 1, i))
  const count = parseInt(l.slice(i + 1))
  const file = l.slice(0, l.indexOf(':'))
  const mm = file.match(/\/internal\/([^/]+\/[^/]+)\.go$/)
  const key = mm ? mm[1] : file.split('/').slice(-2).join('/')
  if (!pkgFile[key]) pkgFile[key] = { s: 0, c: 0 }
  pkgFile[key].s += numStmt
  if (count > 0) pkgFile[key].c += numStmt
  totalS += numStmt; if (count > 0) totalC += numStmt
}
const pkgs = {}
for (const [k, v] of Object.entries(pkgFile)) {
  const p = k.split('/')[0]
  if (!pkgs[p]) pkgs[p] = []
  pkgs[p].push([k, v.s, v.c])
}
for (const [p, files] of Object.entries(pkgs).sort((a, b) => {
  const sa = a[1].reduce((s, f) => s + f[1], 0), sb = b[1].reduce((s, f) => s + f[1], 0)
  return sb - sa
})) {
  console.log(`\n===== ${p} =====`)
  files.sort((a, b) => b[1] - a[1])
  for (const [k, s, c] of files) {
    const pct = ((c / s) * 100).toFixed(0)
    console.log(`  ${String(s).padStart(5)} stmt  ${String(c).padStart(5)} cov  ${pct.padStart(3)}%  ${k}`)
  }
}
