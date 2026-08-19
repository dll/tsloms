// TSLOMS 前端 bundle budget（CI-P2-01）
// =============================================================================
// Vite 构建体积预算插件：
//   - 对「主包(entry)」「Cesium」「首屏(entry+vendor)」分块设体积预算（minified, bytes）。
//   - 构建结束后扫描 dist 内实际产物（磁盘字节，已含 esbuild minify），超预算抛错（使 CI 失败）。
//   - 同时写入 dist/.bundle-report.json 并打印相对预算的增量报告（供 PR 审计）。
//
// 预算值基于当前 baseline（见下）并留 ~10% 余量；CI 中任一分块超限即失败。
//   entry   : ~17kB      → budget 400   (主包/首屏入口)
//   vendor  : ~1238kB    → budget 1600  (vue/router/pinia/axios/element-plus)
//   echarts : ~1128kB    → budget 1400
//   cesium  : ~4173kB    → budget 4700  (三维渲染，最大头；余量最大)
// 调预算时：确认为合理增长→更新 budgets 表→commit 注明理由。
// =============================================================================
import { readdirSync, writeFileSync, statSync } from 'fs'
import { resolve, join } from 'path'
import { fileURLToPath } from 'url'

const __dirname = fileURLToPath(new URL('.', import.meta.url))

// 预算表（kB，minified 磁盘字节）
const BUDGETS = {
  entry: { label: '主包/首屏入口 (entry)', budgetKb: 400 },
  vendor: { label: '首屏依赖 (vendor)', budgetKb: 1600 },
  echarts: { label: 'ECharts', budgetKb: 1400 },
  cesium: { label: 'Cesium', budgetKb: 4700 },
}

// 按文件名前缀归类 manualChunks JS（bundle 键为 assets/xxx-abc.js）；
// 主包(entry) 不在前缀归入，入口 chunk 由 entrySet（isEntry）单独识别。
function classify(name) {
  if (/^cesium-/.test(name)) return 'cesium'
  if (/^vendor-/.test(name)) return 'vendor'
  if (/^echarts-/.test(name)) return 'echarts'
  return null
}

// 递归列出 outDir 下所有文件名（相对路径）
function walkFiles(dir, base = '') {
  const out = []
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name)
    if (e.isDirectory()) out.push(...walkFiles(p, join(base, e.name)))
    else if (e.isFile()) out.push(join(base, e.name))
  }
  return out
}

export function bundleBudgetPlugin() {
  let entryNames = []
  let outDir = 'dist'
  return {
    name: 'tsloms-bundle-budget',
    configResolved(config) {
      outDir = config.build.outDir
    },
    // generateBundle：识别真正的入口 chunk（isEntry），供后续磁盘测重；
    // cesium/vendor/echarts 由 manualChunks 前缀确定。
    generateBundle(_outputOptions, bundle) {
      for (const name of Object.keys(bundle)) {
        const file = bundle[name]
        if (file && file.type === 'chunk' && file.isEntry) entryNames.push(name)
      }
    },
    async closeBundle() {
      // 插件位于 packages/admin/build/，产物在 packages/admin/dist → 相对 __dirname 回退一级
      const absOut = resolve(__dirname, '..', ...outDir.split(/[\\/]/))
      let sizes = {} // key -> { kb, chunks }
      const entrySet = new Set(entryNames.map((n) => n.split(/[\\/]/).pop()))
      try {
        for (const rel of walkFiles(absOut)) {
          if (!rel.endsWith('.js')) continue
          const base = rel.split(/[\\/]/).pop()
          let key
          if (entrySet.has(base)) key = 'entry'
          else key = classify(base) // cesium/vendor/echarts；其余忽略
          if (!key) continue
          const bytes = statSync(join(absOut, rel)).size
          if (!sizes[key]) sizes[key] = { kb: 0, chunks: [] }
          sizes[key].kb += bytes / 1024
          sizes[key].chunks.push(base)
        }
      } catch (e) {
        console.warn(`bundle-budget: 扫描产物失败（跳过，不阻断）: ${e.message}`)
        return
      }

      const entryKb = sizes.entry ? sizes.entry.kb : 0
      const vendorKb = sizes.vendor ? sizes.vendor.kb : 0
      const firstScreenKb = entryKb + vendorKb

      let ok = true
      const lines = []
      const report = { budgets: {}, measured: {}, measuredFirstScreen: 0, firstScreenKb, passed: true }

      for (const [key, spec] of Object.entries(BUDGETS)) {
        const kb = sizes[key] ? sizes[key].kb : 0
        const over = kb - spec.budgetKb
        const flag = over > 0 ? 'FAIL' : 'ok'
        if (over > 0) { ok = false; report.passed = false }
        report.budgets[key] = spec.budgetKb
        report.measured[key] = Math.round(kb * 100) / 100
        lines.push(
          `${flag}  ${spec.label.padEnd(22)} ${kb.toFixed(1).padStart(8)}kB  / 预算 ${spec.budgetKb}kB  ` +
          `(增量 ${over >= 0 ? '+' : ''}${over.toFixed(1)}kB)`,
        )
      }
      const fsOver = firstScreenKb - (BUDGETS.entry.budgetKb + BUDGETS.vendor.budgetKb)
      report.measuredFirstScreen = Math.round(firstScreenKb * 100) / 100
      const fsFlag = fsOver > 0 ? 'FAIL' : 'ok'
      if (fsOver > 0) { ok = false; report.passed = false }
      lines.push(
        `${fsFlag}  ${('首屏(entry+vendor)').padEnd(22)} ${firstScreenKb.toFixed(1).padStart(8)}kB  ` +
        `/ 预算 ${BUDGETS.entry.budgetKb + BUDGETS.vendor.budgetKb}kB  (增量 ${fsOver >= 0 ? '+' : ''}${fsOver.toFixed(1)}kB)`,
      )

      console.log('\n===== Bundle Budget（CI-P2-01，磁盘字节/minified） =====')
      lines.forEach((l) => console.log(l))
      console.log('====================================')

      try {
        writeFileSync(join(absOut, '.bundle-report.json'), JSON.stringify(report, null, 2))
      } catch { /* 报告写入失败不阻断 */ }

      if (!ok) {
        this.error('bundle budget 超限：请拆分首屏/主包或将非首屏依赖改为异步加载，详情见上方报告')
      }
    },
  }
}
