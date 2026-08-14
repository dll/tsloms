<template>
  <div class="dashboard">
    <!-- 顶部统计卡片 -->
    <div class="toolbar">
      <span class="toolbar-label">统计范围：</span>
      <el-radio-group v-model="rangeDays" @change="handleRangeChange">
        <el-radio-button :value="7">近7天</el-radio-button>
        <el-radio-button :value="30">近30天</el-radio-button>
        <el-radio-button :value="90">近90天</el-radio-button>
      </el-radio-group>
      <el-button size="small" style="margin-left: 12px" @click="exportFaultStats">导出故障统计 CSV</el-button>
    </div>
    <el-row :gutter="20" class="stat-row">
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card stat-device">
          <div class="stat-content">
            <div class="stat-info">
              <p class="stat-label">设备总数 / 在线</p>
              <p class="stat-value">
                {{ overview.devices?.total ?? 0 }}
                <span class="stat-sub">/ {{ overview.devices?.online ?? 0 }}</span>
              </p>
            </div>
            <el-icon :size="40" color="#409EFF"><Monitor /></el-icon>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card stat-fault">
          <div class="stat-content">
            <div class="stat-info">
              <p class="stat-label">活跃故障</p>
              <p class="stat-value">{{ overview.faults?.active ?? 0 }}</p>
            </div>
            <el-icon :size="40" color="#F56C6C"><Warning /></el-icon>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card stat-today">
          <div class="stat-content">
            <div class="stat-info">
              <p class="stat-label">今日新增故障</p>
              <p class="stat-value">{{ overview.faults?.today ?? 0 }}</p>
            </div>
            <el-icon :size="40" color="#E6A23C"><WarningFilled /></el-icon>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card stat-order">
          <div class="stat-content">
            <div class="stat-info">
              <p class="stat-label">待处理工单</p>
              <p class="stat-value">{{ overview.work_orders?.pending ?? 0 }}</p>
            </div>
            <el-icon :size="40" color="#67C23A"><Tickets /></el-icon>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card stat-overdue">
          <div class="stat-content">
            <div class="stat-info">
              <p class="stat-label">超时工单</p>
              <p class="stat-value">
                {{ overview.work_orders?.overdue ?? 0 }}
                <span v-if="(overview.work_orders?.overdue ?? 0) > 0" class="stat-sub stat-sub-red">需优先处理</span>
              </p>
            </div>
            <el-icon :size="40" color="#F56C6C"><AlarmClock /></el-icon>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 图表区域 -->
    <el-row :gutter="20" class="chart-row">
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header><span>故障类型占比</span></template>
          <div ref="pieChartRef" class="chart-container"></div>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header><span>故障趋势（近{{ rangeDays }}天）</span></template>
          <div ref="barChartRef" class="chart-container"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="chart-row">
      <el-col :span="8">
        <el-card shadow="hover">
          <template #header><span>工单状态分布</span></template>
          <div ref="woPieRef" class="chart-container"></div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover">
          <template #header><span>设备故障排行 Top{{ rangeDays === 7 ? 5 : 10 }}</span></template>
          <div ref="rankBarRef" class="chart-container"></div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover">
          <template #header><span>工单平均闭环时长</span></template>
          <div class="avg-closure">
            <div class="avg-value">{{ avgClosureHours.toFixed(2) }}<span class="avg-unit"> 小时</span></div>
            <div class="avg-sub">已完成 {{ closureCount }} 单（近{{ rangeDays }}天）</div>
            <div class="avg-note">平均从创建到完成的时间</div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts'
import {
  getOverview, getFaultTypeStats, getFaultTrend, getWorkOrderStats,
  getDeviceFaultRank, getWorkOrderAvgClosure,
} from '@/api/dashboard'

// 看板概览数据
const overview = reactive({
  devices: { online: 0, offline: 0, total: 0 },
  faults: { active: 0, resolved: 0, today: 0 },
  work_orders: { pending: 0, processing: 0, completed: 0, overdue: 0 },
})

// 统计范围（天）
const rangeDays = ref(30)
const avgClosureHours = ref(0)
const closureCount = ref(0)

// 图表 DOM 引用
const pieChartRef = ref<HTMLElement>()
const barChartRef = ref<HTMLElement>()
const woPieRef = ref<HTMLElement>()
const rankBarRef = ref<HTMLElement>()

// ECharts 实例
let pieChart: echarts.ECharts | null = null
let barChart: echarts.ECharts | null = null
let woPieChart: echarts.ECharts | null = null
let rankBarChart: echarts.ECharts | null = null

// 故障类型中文映射
const faultTypeCN: Record<string, string> = {
  lamp_off: '灯灭', abnormal_on: '异常同亮', timeout: '亮灯超时', dim: '缺亮', power_loss: '断电', unknown: '未知',
}
const woStatusCN: Record<string, string> = { pending: '待处理', processing: '处理中', completed: '已完成', rejected: '已驳回' }

// 获取概览数据
async function fetchOverview() {
  try {
    const res = await getOverview()
    Object.assign(overview, res.data)
  } catch {
    // 请求失败忽略
  }
}

// 初始化饼图 - 故障类型占比
async function initPieChart() {
  try {
    const res = await getFaultTypeStats(rangeDays.value)
    const stats = res.data.stats || []
    const pieData = stats.map((item: { fault_type: string; count: number }) => ({
      name: faultTypeCN[item.fault_type] || item.fault_type,
      value: item.count,
    }))

    await nextTick()
    if (!pieChartRef.value) return
    pieChart = echarts.init(pieChartRef.value)
    pieChart.setOption({
      tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
      legend: { bottom: 0, left: 'center' },
      series: [{
        name: '故障类型', type: 'pie', radius: ['40%', '70%'], center: ['50%', '45%'],
        avoidLabelOverlap: false, itemStyle: { borderRadius: 8, borderColor: '#fff', borderWidth: 2 },
        label: { show: false }, emphasis: { label: { show: true, fontSize: 14, fontWeight: 'bold' } },
        data: pieData.length > 0 ? pieData : [{ name: '暂无数据', value: 0 }],
      }],
    })
  } catch {
    // 请求失败忽略
  }
}

// 初始化柱状图 - 故障趋势
async function initBarChart() {
  try {
    const res = await getFaultTrend({ dimension: 'day', days: rangeDays.value })
    const trend = res.data.trend || []
    const periods = trend.map((item: { period: string; count: number }) => item.period)
    const counts = trend.map((item: { period: string; count: number }) => item.count)

    await nextTick()
    if (!barChartRef.value) return
    barChart = echarts.init(barChartRef.value)
    barChart.setOption({
      tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
      grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
      xAxis: { type: 'category', data: periods, axisLabel: { rotate: 30 } },
      yAxis: { type: 'value', minInterval: 1 },
      series: [{ name: '故障数量', type: 'bar', data: counts, itemStyle: { color: '#409EFF', borderRadius: [4, 4, 0, 0] }, barWidth: '50%' }],
    })
  } catch {
    // 请求失败忽略
  }
}

// 初始化工单状态饼图
async function initWoPie() {
  try {
    const res = await getWorkOrderStats()
    const stats = res.data.stats || []
    const pieData = stats.map((item: { status: string; count: number }) => ({
      name: woStatusCN[item.status] || item.status, value: item.count,
    }))
    await nextTick()
    if (!woPieRef.value) return
    woPieChart = echarts.init(woPieRef.value)
    woPieChart.setOption({
      tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
      legend: { bottom: 0, left: 'center' },
      series: [{
        name: '工单状态', type: 'pie', radius: ['40%', '70%'], center: ['50%', '45%'],
        avoidLabelOverlap: false, itemStyle: { borderRadius: 8, borderColor: '#fff', borderWidth: 2 },
        label: { show: false }, emphasis: { label: { show: true, fontSize: 14, fontWeight: 'bold' } },
        data: pieData.length > 0 ? pieData : [{ name: '暂无数据', value: 0 }],
      }],
    })
  } catch {
    // 请求失败忽略
  }
}

// 初始化设备故障排行柱状图
async function initRankBar() {
  try {
    const limit = rangeDays.value === 7 ? 5 : 10
    const res = await getDeviceFaultRank({ limit, days: rangeDays.value })
    const rank = res.data.rank || []
    const labels = rank.map((item: { device_hw_id: number }) => '#' + item.device_hw_id)
    const counts = rank.map((item: { count: number }) => item.count)
    await nextTick()
    if (!rankBarRef.value) return
    rankBarChart = echarts.init(rankBarRef.value)
    rankBarChart.setOption({
      tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
      grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
      xAxis: { type: 'category', data: labels, axisLabel: { rotate: 20 } },
      yAxis: { type: 'value', minInterval: 1 },
      series: [{ name: '故障数', type: 'bar', data: counts, itemStyle: { color: '#F56C6C', borderRadius: [4, 4, 0, 0] }, barWidth: '50%' }],
    })
  } catch {
    // 请求失败忽略
  }
}

// 获取工单平均闭环时长
async function fetchAvgClosure() {
  try {
    const res = await getWorkOrderAvgClosure({ days: rangeDays.value })
    avgClosureHours.value = res.data.avg_hours || 0
    closureCount.value = res.data.completed_count || 0
  } catch {
    // 请求失败忽略
  }
}

// 刷新全部图表
async function refreshAll() {
  pieChart?.dispose(); pieChart = null
  barChart?.dispose(); barChart = null
  woPieChart?.dispose(); woPieChart = null
  rankBarChart?.dispose(); rankBarChart = null
  await Promise.all([initPieChart(), initBarChart(), initWoPie(), initRankBar(), fetchAvgClosure()])
}

// 统计范围切换
function handleRangeChange() {
  refreshAll()
}

// 导出故障类型统计 CSV
function exportFaultStats() {
  // 从后端重新拉取当前范围数据
  getFaultTypeStats(rangeDays.value).then((res: any) => {
    const stats = res.data.stats || []
    const header = ['故障类型', '数量']
    const rows = stats.map((item: { fault_type: string; count: number }) => [faultTypeCN[item.fault_type] || item.fault_type, String(item.count)])
    const csv = '\ufeff' + [header, ...rows].map((r) => r.join(',')).join('\n')
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = `故障类型统计_${rangeDays.value}天.csv`
    a.click()
    URL.revokeObjectURL(a.href)
    ElMessage.success('故障统计已导出')
  }).catch(() => ElMessage.error('导出失败'))
}

// 窗口大小变化时重绘图表
function handleResize() {
  pieChart?.resize()
  barChart?.resize()
  woPieChart?.resize()
  rankBarChart?.resize()
}

onMounted(async () => {
  await fetchOverview()
  await refreshAll()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  pieChart?.dispose()
  barChart?.dispose()
  woPieChart?.dispose()
  rankBarChart?.dispose()
})
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
}
.toolbar-label {
  margin-right: 8px;
  font-size: 14px;
  color: #606266;
}
.stat-row {
  margin-bottom: 20px;
}

.avg-closure {
  height: 320px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}
.avg-value {
  font-size: 40px;
  font-weight: bold;
  color: #409eff;
}
.avg-unit {
  font-size: 16px;
  color: #909399;
  font-weight: normal;
}
.avg-sub {
  margin-top: 8px;
  font-size: 14px;
  color: #606266;
}
.avg-note {
  margin-top: 4px;
  font-size: 12px;
  color: #c0c4cc;
}

.stat-card {
  border-radius: 8px;
}

.stat-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.stat-label {
  margin: 0 0 8px 0;
  font-size: 14px;
  color: #909399;
}

.stat-value {
  margin: 0;
  font-size: 28px;
  font-weight: bold;
  color: #303133;
}

.stat-sub {
  font-size: 16px;
  color: #909399;
  font-weight: normal;
}

.stat-sub-red {
  color: #F56C6C;
  font-size: 13px;
  margin-left: 6px;
}

.chart-row {
  margin-bottom: 20px;
}

.chart-container {
  height: 320px;
}
</style>
