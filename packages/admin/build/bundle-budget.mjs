// TSLOMS 前端 bundle budget（CI-P2-01）
// =============================================================================
// Vite 构建体积预算插件：
//   - 对「主包(entry)」「Cesium」「首屏(entry+vendor)」分块设体积预算（minified，raw bytes）。
//   - 构建结束后统计 dist 内各 chunk 体积，超预算抛错（使 CI 失败）。
//   - 同时写入 dist/.bundle-report.json 并打印相对预算的增量报告（供 PR 审计）。
//
// 预算值基于当前 baseline（见下）并留 ~10% 余量；CI 中任一分块超限即失败。
//   entry   : ~<100kB   → budget 400
//   vendor  : ~1238kB   → budget 1600   (vue/router/pinia/axios/element-plus)
//   echarts : ~1128kB   → budget 1400
//   cesium  : ~4173kB   → budget 4700   (大图/三维渲染，最大头；余量最大)
// 需要调预算时：
//   1) 确认为合理增长（新增特性），2) 更新 budgets 表，3) 在 commit 注明理由。
// =============================================================================
import { writeFileSync } from 'fs'
import { resolve } from 'path'
import { fileURLToPath } from 'url'

const __dirname = fileURLToPath(new URL('.', import.meta.url))

// 预算表（kB，minified）
const BUDGETS = {
  cesium: { label: 'Cesium', budgetKb: 4700 },
  vendor: { label: '首屏依赖 (vendor)', budgetKb: 1600 },
  echarts: { label: 'ECharts', budgetKb: 1400 },
  entry: { label: '主包/首屏入口 (entry)', budgetKb: 400 },
}

// 识别 chunk 归属：按 manualChunks 生成的文件名前缀（含 content hash）与入口 chunk。
function classifyChunk(name) {
  if (/^cesium-/.test(name)) return 'cesium'
  if (/^vendor-/.test(name)) return 'vendor'
  if (/^echarts-/.test(name)) return 'echarts'
  // 入口 chunk 通常是 assets 下不带明显业务前缀的 index-*.js 之一（体积较小）；
  // 更稳妥：通过 rollup chunk.isEntry 判定，这里用 generateBundle 的 chunk 对象能力。
  return null
}

export function bundleBudgetPlugin() {
  return {
    name: 'tsloms-bundle-budget',
    generateBundle(outputOptions, bundle) {
      let outDir = outputOptions.dir || 'dist'
      const sizes = {} // key -> { kb, isEntry }

      for (const name of Object.keys(bundle)) {
        const file = bundle[name]
        if (file.type !== 'chunk') continue
        const bytes = typeof file.code === 'string' ? Buffer.byteLength(file.code, 'utf8') : (file.code ? file.code.length : 0)
        const kb = bytes / 1024
        const key = classifyChunk(name) || (file.isEntry ? 'entry' : null)
        if (!key) continue
        if (!sizes[key]) sizes[key] = { kb, chunks: [] }
        sizes[key].kb += kb
        sizes[key].chunks.push(name)
      }

      // 首屏 = entry + vendor（关键路径）
      const entryKb = sizes.entry ? sizes.entry.kb : 0
      const vendorKb = sizes.vendor ? sizes.vendor.kb : 0
      const firstScreenKb = entryKb + vendorKb

      let ok = true
      const lines = []
      const report = { budgets: {}, measured: {}, firstScreenKb, passed: true }

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

      console.log('\n===== Bundle Budget（CI-P2-01） =====')
      lines.forEach((l) => console.log(l))
      console.log('====================================')

      // 写报告供 CD/审计留存
      try {
        writeFileSync(
          resolve(__dirname, ...outDir.split('/'), '.bundle-report.json'),
          JSON.stringify(report, null, 2),
        )
      } catch {
        // 报告写入失败不阻断构建（尽力而为）
      }

      if (!ok) {
        this.error('bundle budget 超限：请拆分首屏/主包或将非首屏依赖改为异步，具体见上方报告')
      }
    },
  }
}
