<template>
  <div class="dashboard">
    <!-- 顶部统计卡片 -->
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
    </el-row>

    <!-- 图表区域 -->
    <el-row :gutter="20" class="chart-row">
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>
            <span>故障类型占比</span>
          </template>
          <div ref="pieChartRef" class="chart-container"></div>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>
            <span>故障趋势（近7天）</span>
          </template>
          <div ref="barChartRef" class="chart-container"></div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, nextTick } from 'vue'
import * as echarts from 'echarts'
import { getOverview, getFaultTypeStats, getFaultTrend } from '@/api/dashboard'

// 看板概览数据
const overview = reactive({
  devices: { online: 0, offline: 0, total: 0 },
  faults: { active: 0, resolved: 0, today: 0 },
  work_orders: { pending: 0, processing: 0, completed: 0 },
})

// 图表 DOM 引用
const pieChartRef = ref<HTMLElement>()
const barChartRef = ref<HTMLElement>()

// ECharts 实例
let pieChart: echarts.ECharts | null = null
let barChart: echarts.ECharts | null = null

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
    const res = await getFaultTypeStats(30)
    const stats = res.data.stats || []
    const pieData = stats.map((item: { fault_type: string; count: number }) => ({
      name: item.fault_type,
      value: item.count,
    }))

    await nextTick()
    if (!pieChartRef.value) return
    pieChart = echarts.init(pieChartRef.value)
    pieChart.setOption({
      tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
      legend: { bottom: 0, left: 'center' },
      series: [
        {
          name: '故障类型',
          type: 'pie',
          radius: ['40%', '70%'],
          center: ['50%', '45%'],
          avoidLabelOverlap: false,
          itemStyle: { borderRadius: 8, borderColor: '#fff', borderWidth: 2 },
          label: { show: false },
          emphasis: { label: { show: true, fontSize: 14, fontWeight: 'bold' } },
          data: pieData.length > 0 ? pieData : [{ name: '暂无数据', value: 0 }],
        },
      ],
    })
  } catch {
    // 请求失败忽略
  }
}

// 初始化柱状图 - 故障趋势
async function initBarChart() {
  try {
    const res = await getFaultTrend({ dimension: 'day', days: 7 })
    const trend = res.data.trend || []
    const periods = trend.map((item: { period: string; count: number }) => item.period)
    const counts = trend.map((item: { period: string; count: number }) => item.count)

    await nextTick()
    if (!barChartRef.value) return
    barChart = echarts.init(barChartRef.value)
    barChart.setOption({
      tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
      grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
      xAxis: {
        type: 'category',
        data: periods,
        axisLabel: { rotate: 30 },
      },
      yAxis: { type: 'value', minInterval: 1 },
      series: [
        {
          name: '故障数量',
          type: 'bar',
          data: counts,
          itemStyle: { color: '#409EFF', borderRadius: [4, 4, 0, 0] },
          barWidth: '50%',
        },
      ],
    })
  } catch {
    // 请求失败忽略
  }
}

// 窗口大小变化时重绘图表
function handleResize() {
  pieChart?.resize()
  barChart?.resize()
}

onMounted(async () => {
  await fetchOverview()
  await Promise.all([initPieChart(), initBarChart()])
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  pieChart?.dispose()
  barChart?.dispose()
  pieChart = null
  barChart = null
})
</script>

<style scoped>
.stat-row {
  margin-bottom: 20px;
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

.chart-row {
  margin-bottom: 20px;
}

.chart-container {
  height: 320px;
}
</style>
