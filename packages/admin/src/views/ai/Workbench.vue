<template>
  <div class="ai-workbench">
    <el-tabs v-model="activeTab" type="border-card">
      <!-- 库存健康分析 -->
      <el-tab-pane label="库存健康分析" name="inventory">
        <div class="toolbar">
          <el-button type="primary" :loading="invLoading" @click="loadInventory">
            <el-icon><MagicStick /></el-icon>&nbsp;运行库存 AI 分析
          </el-button>
          <el-tag v-if="invData" :type="invData.source === 'LLM' ? 'success' : 'info'" class="ml">
            {{ invData.source === 'LLM' ? 'AI 洞察' : '规则分析' }}
          </el-tag>
        </div>
        <el-empty v-if="!invData" description="点击「运行库存 AI 分析」生成库存健康洞察与补货建议" />
        <template v-else>
          <el-row :gutter="16" class="stat-row">
            <el-col :span="6"><el-card><div class="stat-num">{{ invData.snapshot.total_kinds }}</div><div class="stat-label">物料种类</div></el-card></el-col>
            <el-col :span="6"><el-card><div class="stat-num">{{ invData.snapshot.total_stock }}</div><div class="stat-label">库存总件数</div></el-card></el-col>
            <el-col :span="6"><el-card><div class="stat-num">¥{{ money(invData.snapshot.total_value) }}</div><div class="stat-label">库存总金额</div></el-card></el-col>
            <el-col :span="6"><el-card><div class="stat-num warn">{{ invData.snapshot.low_stock.length }}</div><div class="stat-label">预警/缺口物料</div></el-card></el-col>
          </el-row>

          <el-alert v-if="invData.insight" :title="invData.insight" type="success" :closable="false" class="insight" />

          <el-row :gutter="16">
            <el-col :span="12">
              <el-card shadow="never"><template #header>低库存 / 缺货预警</template>
                <el-table :data="invData.snapshot.low_stock" size="small">
                  <el-table-column prop="name" label="物料" />
                  <el-table-column prop="stock" label="库存" width="70" />
                  <el-table-column prop="threshold" label="阈值" width="70" />
                  <el-table-column label="状态" width="80">
                    <template #default="{ row }">
                      <el-tag v-if="row.stock === 0" type="danger">缺货</el-tag>
                      <el-tag v-else type="warning">偏低</el-tag>
                    </template>
                  </el-table-column>
                </el-table>
              </el-card>
            </el-col>
            <el-col :span="12">
              <el-card shadow="never"><template #header>滞销积压（近90天无领用）</template>
                <el-table :data="invData.snapshot.slow_moving" size="small">
                  <el-table-column prop="name" label="物料" />
                  <el-table-column prop="stock" label="库存" width="70" />
                  <el-table-column prop="unit_price" label="单价" width="80"><template #default="{ row }">{{ money(row.unit_price) }}</template></el-table-column>
                  <el-table-column prop="last_use" label="最近领用" width="100" />
                </el-table>
              </el-card>
            </el-col>
          </el-row>
          <el-card shadow="never" class="mt"><template #header>近6月领用趋势</template>
            <div ref="invTrendRef" class="chart"></div>
          </el-card>
        </template>
      </el-tab-pane>

      <!-- 成本归因分析 -->
      <el-tab-pane label="成本归因分析" name="cost">
        <div class="toolbar">
          <el-button type="primary" :loading="costLoading" @click="loadCost">
            <el-icon><TrendCharts /></el-icon>&nbsp;运行成本 AI 分析
          </el-button>
          <el-select v-model="costDays" size="default" style="width:130px" class="ml">
            <el-option label="近30天" :value="30" />
            <el-option label="近90天" :value="90" />
            <el-option label="近180天" :value="180" />
          </el-select>
          <el-tag v-if="costData" :type="costData.source === 'LLM' ? 'success' : 'info'" class="ml">
            {{ costData.source === 'LLM' ? 'AI 归因' : '规则分析' }}
          </el-tag>
        </div>
        <el-empty v-if="!costData" description="点击「运行成本 AI 分析」生成成本归因与降本建议" />
        <template v-else>
          <el-alert v-if="costData.insight" :title="costData.insight" type="success" :closable="false" class="insight" />
          <el-row :gutter="16">
            <el-col :span="10">
              <el-card shadow="never"><template #header>成本结构（按类型）</template>
                <div ref="costPieRef" class="chart"></div>
              </el-card>
            </el-col>
            <el-col :span="14">
              <el-card shadow="never"><template #header>高成本设备 TOP</template>
                <el-table :data="costData.snapshot.top_devices" size="small">
                  <el-table-column prop="device_hw_id" label="设备ID" />
                  <el-table-column label="总成本"><template #default="{ row }">{{ money(row.total) }}</template></el-table-column>
                  <el-table-column prop="count" label="费用单数" width="90" />
                </el-table>
              </el-card>
              <el-card shadow="never" class="mt"><template #header>高成本故障类型</template>
                <el-table :data="costData.snapshot.top_fault_type" size="small">
                  <el-table-column prop="fault_type" label="故障类型" />
                  <el-table-column label="成本"><template #default="{ row }">{{ money(row.total) }}</template></el-table-column>
                  <el-table-column prop="count" label="笔数" width="90" />
                </el-table>
              </el-card>
            </el-col>
          </el-row>
        </template>
      </el-tab-pane>

      <!-- 运维报告 -->
      <el-tab-pane label="运维报告" name="report">
        <div class="toolbar">
          <el-button type="primary" :loading="rptLoading" @click="genReport('daily')"><el-icon><Document /></el-icon>&nbsp;生成运维日报</el-button>
          <el-button :loading="rptLoading" @click="genReport('inventory')" class="ml">库存报告</el-button>
          <el-button :loading="rptLoading" @click="genReport('cost')" class="ml">成本报告</el-button>
          <el-button :loading="rptLoading" @click="genReport('fault')" class="ml">故障报告</el-button>
          <el-button :loading="rptLoading" @click="genReport('workorder')" class="ml">工单报告</el-button>
          <el-button :loading="rptLoading" @click="genReport('device')" class="ml">设备报告</el-button>
        </div>
        <div class="toolbar mt">
          <el-button @click="loadReports" :loading="listLoading"><el-icon><Refresh /></el-icon>&nbsp;刷新历史报告</el-button>
          <el-select v-model="reportModule" size="default" style="width:140px" class="ml" @change="loadReports">
            <el-option label="全部" value="" />
            <el-option label="日报" value="daily" />
            <el-option label="库存" value="inventory" />
            <el-option label="成本" value="cost" />
            <el-option label="故障" value="fault" />
            <el-option label="工单" value="workorder" />
            <el-option label="设备" value="device" />
          </el-select>
        </div>
        <el-table v-loading="listLoading" :data="reports" size="small" class="mt">
          <el-table-column prop="title" label="报告" min-width="160" />
          <el-table-column label="模块" width="90">
            <template #default="{ row }">
              <el-tag size="small">{{ moduleLabel(row.module) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="period" label="周期" width="70" />
          <el-table-column prop="range_from" label="起始" width="90" />
          <el-table-column prop="range_to" label="截止" width="90" />
          <el-table-column label="来源" width="70">
            <template #default="{ row }">
              <el-tag size="small" :type="row.source === 'LLM' ? 'success' : 'info'">{{ row.source }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="tokens_used" label="Token" width="80" />
          <el-table-column prop="created_at" label="生成时间" width="170" />
          <el-table-column label="操作" width="90" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="viewReport(row)">查看</el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-dialog v-model="reportDialog" title="报告详情" width="720px">
          <div v-if="currentReport">
            <el-alert :title="currentReport.summary" type="success" :closable="false" class="report-summary" />
            <h4>结构化数据</h4>
            <pre class="report-pre">{{ prettyJSON(currentReport.data) }}</pre>
          </div>
        </el-dialog>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts'
import {
  analyzeInventory, analyzeCost, generateReport, listReports,
} from '@/api/ai'

const activeTab = ref('inventory')

// ---- 库存 ----
const invLoading = ref(false)
const invData = ref<any>(null)
const invTrendRef = ref<HTMLElement>()

// ---- 成本 ----
const costLoading = ref(false)
const costData = ref<any>(null)
const costDays = ref(90)
const costPieRef = ref<HTMLElement>()

// ---- 报告 ----
const rptLoading = ref(false)
const listLoading = ref(false)
const reports = ref<any[]>([])
const reportModule = ref('')
const reportDialog = ref(false)
const currentReport = ref<any>(null)

let charts: echarts.ECharts[] = []

function disposeCharts() {
  charts.forEach(c => c.dispose())
  charts = []
}

function money(v: any): string {
  const n = Number(v || 0)
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

async function loadInventory() {
  invLoading.value = true
  try {
    const res: any = await analyzeInventory()
    invData.value = res.result
    await nextTick()
    renderInvTrend()
  } catch (e: any) {
    ElMessage.error(e.message || '库存分析失败')
  } finally {
    invLoading.value = false
  }
}

function renderInvTrend() {
  if (!invTrendRef.value || !invData.value) return
  const chart = echarts.init(invTrendRef.value)
  charts.push(chart)
  const trend = invData.value.snapshot.month_use_trend || []
  chart.setOption({
    tooltip: { trigger: 'axis' },
    grid: { left: 40, right: 20, top: 20, bottom: 30 },
    xAxis: { type: 'category', data: trend.map((t: any) => t.month) },
    yAxis: { type: 'value' },
    series: [{ type: 'bar', data: trend.map((t: any) => t.value), itemStyle: { color: '#409eff' } }],
  })
}

async function loadCost() {
  costLoading.value = true
  try {
    const res: any = await analyzeCost(costDays.value)
    costData.value = res.result
    await nextTick()
    renderCostPie()
  } catch (e: any) {
    ElMessage.error(e.message || '成本分析失败')
  } finally {
    costLoading.value = false
  }
}

function renderCostPie() {
  if (!costPieRef.value || !costData.value) return
  const chart = echarts.init(costPieRef.value)
  charts.push(chart)
  const byType = costData.value.snapshot.by_type || []
  chart.setOption({
    tooltip: { trigger: 'item', formatter: '{b}: ¥{c} ({d}%)' },
    series: [{
      type: 'pie', radius: '60%',
      data: byType.map((t: any) => ({ name: t.name, value: t.value })),
      label: { formatter: '{b}\n¥{c}' },
    }],
  })
}

async function genReport(module: string) {
  rptLoading.value = true
  try {
    const res: any = await generateReport(module, module === 'daily' ? 'day' : 'month')
    ElMessage.success('报告已生成')
    currentReport.value = res.result
    reportDialog.value = true
    await loadReports()
  } catch (e: any) {
    ElMessage.error(e.message || '报告生成失败')
  } finally {
    rptLoading.value = false
  }
}

async function loadReports() {
  listLoading.value = true
  try {
    const res: any = await listReports(reportModule.value || undefined)
    reports.value = res.list || []
  } catch (e: any) {
    ElMessage.error(e.message || '加载报告失败')
  } finally {
    listLoading.value = false
  }
}

function viewReport(row: any) {
  currentReport.value = row
  reportDialog.value = true
}

function moduleLabel(m: string): string {
  const map: Record<string, string> = { daily: '日报', inventory: '库存', cost: '成本', fault: '故障', workorder: '工单', device: '设备' }
  return map[m] || m
}

function prettyJSON(s: string): string {
  try { return JSON.stringify(JSON.parse(s), null, 2) } catch { return s }
}

onMounted(async () => {
  await loadReports()
})

onBeforeUnmount(() => disposeCharts())
</script>

<style scoped>
.ai-workbench { padding: 4px; }
.toolbar { display: flex; align-items: center; margin-bottom: 12px; }
.ml { margin-left: 10px; }
.mt { margin-top: 12px; }
.stat-row { margin-bottom: 14px; }
.stat-num { font-size: 24px; font-weight: 600; color: #303133; }
.stat-num.warn { color: #e6a23c; }
.stat-label { color: #909399; font-size: 13px; margin-top: 4px; }
.insight { margin-bottom: 14px; white-space: pre-line; }
.chart { height: 260px; }
.report-summary { white-space: pre-line; margin-bottom: 8px; }
.report-pre { background: #f5f7fa; padding: 12px; border-radius: 6px; max-height: 320px; overflow: auto; font-size: 12px; }
</style>
