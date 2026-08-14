<template>
  <div class="map-page">
    <!-- 顶部统计栏 -->
    <div class="map-stats">
      <div class="stat">
        <div class="num">{{ totalDevices }}</div>
        <div class="label">设备总数</div>
      </div>
      <div class="stat">
        <div class="num online">{{ onlineDevices }}</div>
        <div class="label">在线</div>
      </div>
      <div class="stat">
        <div class="num offline">{{ offlineDevices }}</div>
        <div class="label">离线</div>
      </div>
      <div class="stat">
        <div class="num mapped">{{ mappedDevices }}</div>
        <div class="label">已打点</div>
      </div>
    </div>

    <!-- 地图容器 -->
    <el-card shadow="never">
      <template #header>
        <div class="map-header">
          <span>设备分布地图</span>
          <el-button size="small" @click="refresh">刷新</el-button>
        </div>
      </template>
      <div ref="mapRef" class="map-container"></div>
      <div v-if="mappedDevices === 0" class="map-tip">
        提示：尚未为设备设置经纬度，请在「设备管理 → 编辑」中录入经纬度后刷新。未打点设备将以路口汇总显示。
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import * as echarts from 'echarts'
import chinaGeo from '@/assets/china-simple.json'
import { getAllDevices } from '@/api/map'
import { ElMessage } from 'element-plus'

const mapRef = ref<HTMLElement>()
let chart: echarts.ECharts | null = null

interface Dev { hw_id?: number; intersection?: string; lat: number | null; lng: number | null; online_status?: boolean }
const devices = ref<Dev[]>([])

const totalDevices = computed(() => devices.value.length)
const onlineDevices = computed(() => devices.value.filter((d) => d.online_status).length)
const offlineDevices = computed(() => devices.value.filter((d) => !d.online_status).length)
const mappedDevices = computed(() => devices.value.filter((d) => d.lat != null && d.lng != null).length)

// 中国地图散点（有经纬度） + 无经纬度的按路口文本汇总
function buildScatter() {
  const mapped = devices.value.filter((d) => d.lat != null && d.lng != null && d.intersection)
  return mapped.map((d) => ({
    name: d.intersection,
    value: [d.lng!, d.lat!],
    online: d.online_status,
  }))
}

async function refresh() {
  try {
    const res = await getAllDevices(1000)
    devices.value = res.data?.list || []
    draw()
  } catch {
    ElMessage.error('设备数据加载失败')
  }
}

function draw() {
  if (!mapRef.value) return
  if (!chart) chart = echarts.init(mapRef.value)
  const scatter = buildScatter()

  chart.setOption({
    tooltip: {
      trigger: 'item',
      formatter: (p: any) => p.name ? `${p.name}<br/>在线状态：${p.online ? '在线' : '离线'}` : '',
    },
    geo: {
      map: 'china-simple',
      roam: true,
      zoom: 1.2,
      itemStyle: { areaColor: '#e6f2ff', borderColor: '#409EFF', borderWidth: 0.5 },
      emphasis: { label: { show: false }, itemStyle: { areaColor: '#cce5ff' } },
    },
    series: [
      {
        name: '设备',
        type: 'scatter',
        coordinateSystem: 'geo',
        data: scatter,
        symbolSize: 10,
        itemStyle: {
          color: (p: any) => (p.data.online ? '#67C23A' : '#F56C6C'),
          borderColor: '#fff',
          borderWidth: 1,
        },
        label: { show: false },
        emphasis: { label: { show: true, formatter: (p: any) => p.name || '', position: 'top' } },
      },
    ],
  })
}

function handleResize() { chart?.resize() }

onMounted(async () => {
  // 注册中国地图
  echarts.registerMap('china-simple', chinaGeo as any)
  await refresh()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  chart?.dispose()
  chart = null
})
</script>

<style scoped>
.map-page { width: 100%; }
.map-stats { display: flex; gap: 16px; margin-bottom: 16px; flex-wrap: wrap; }
.stat {
  flex: 1; min-width: 140px; background: #fff; border-radius: 6px;
  padding: 16px; text-align: center; box-shadow: 0 1px 4px rgba(0, 21, 41, 0.06);
}
.stat .num { font-size: 28px; font-weight: bold; color: #409EFF; }
.stat .num.online { color: #67C23A; }
.stat .num.offline { color: #F56C6C; }
.stat .num.mapped { color: #E6A23C; }
.stat .label { font-size: 13px; color: #909399; margin-top: 4px; }
.map-header { display: flex; justify-content: space-between; align-items: center; }
.map-container { height: 70vh; min-height: 480px; }
.map-tip { margin-top: 12px; color: #909399; font-size: 13px; }
</style>
