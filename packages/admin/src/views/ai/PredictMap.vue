<template>
  <div class="predict-page">
    <el-card shadow="never" class="head-card">
      <div class="head-bar">
        <div class="title">
          <el-icon><Cpu /></el-icon>
          <span>AI 故障预测</span>
          <span class="sub">（规则引擎健康评分 + 地图风险可视化 + LLM 预案增强）</span>
        </div>
        <div class="actions">
          <el-radio-group v-model="viewMode" size="small" @change="onViewChange">
            <el-radio-button value="device">设备视图</el-radio-button>
            <el-radio-button value="intersection">路口聚合</el-radio-button>
          </el-radio-group>
          <el-button type="primary" :loading="running" @click="doRun">
            <el-icon><VideoPlay /></el-icon> 运行全量预测
          </el-button>
          <el-tag v-if="riskCount" type="warning" size="small" class="qtag">
            高/极高风险 {{ (riskCount.high || 0) + (riskCount.critical || 0) }} 台
          </el-tag>
        </div>
      </div>

      <!-- 风险统计 -->
      <div class="risk-stats" v-if="riskCount && totalDevices">
        <div class="rs" :style="{ color: '#67c23a' }">低风险 <b>{{ riskCount.low || 0 }}</b></div>
        <div class="rs" :style="{ color: '#e6a23c' }">中风险 <b>{{ riskCount.medium || 0 }}</b></div>
        <div class="rs" :style="{ color: '#f56c6c' }">高风险 <b>{{ riskCount.high || 0 }}</b></div>
        <div class="rs" :style="{ color: '#b91c1c' }">极高风险 <b>{{ riskCount.critical || 0 }}</b></div>
        <div class="rs muted">共 {{ totalDevices }} 台设备</div>
      </div>
    </el-card>

    <!-- 地图 + 清单 -->
    <el-card shadow="never" class="body-card">
      <div class="body-wrap">
        <!-- 左侧清单 -->
        <div class="list-panel">
          <div class="list-head">
            {{ viewMode === 'device' ? '预测清单（点击定位）' : '路口聚合（点击定位）' }}
            <span v-if="viewMode === 'intersection' && list.length" class="agg-note">{{ list.length }} 个路口 / {{ totalDevices }} 台</span>
          </div>
          <el-input v-model="kw" placeholder="搜索路口/设备" size="small" clearable style="padding: 8px 10px" />
          <div class="pred-list">
            <div v-for="p in filtered" :key="(viewMode==='device'?p.device_hw_id:p.intersection)" class="pred-item"
                 :class="{ active: sel && sel[viewMode==='device'?'device_hw_id':'intersection'] === p[viewMode==='device'?'device_hw_id':'intersection'] }"
                 :style="{ borderLeftColor: sel && sel[viewMode==='device'?'device_hw_id':'intersection'] === p[viewMode==='device'?'device_hw_id':'intersection'] ? '#409eff' : '' }"
                 @click="focusPred(p)">
              <span class="risk-dot" :class="'r-' + p.risk_level"></span>
              <div class="pi-main">
                <div class="pi-name">{{ p.intersection || '#' + p.device_hw_id }}
                  <span v-if="viewMode==='intersection'" class="dev-cnt">{{ p.device_count }}台</span>
                </div>
                <div class="pi-sub">
                  健康 {{ p.health_score }} · {{ p.risk_label }}风险
                  <template v-if="viewMode==='device'"> · 预计{{ p.predict_type }}</template>
                  <template v-else> · 高发{{ p.top_fault || '—' }}</template>
                </div>
                <div class="pi-plan" v-if="sel && sel[viewMode==='device'?'device_hw_id':'intersection'] === p[viewMode==='device'?'device_hw_id':'intersection']">
                  <template v-if="viewMode==='device'">{{ p.plan }}</template>
                  <template v-else>
                    <span v-for="d in (p.devices||[]).slice(0,3)" :key="d.device_hw_id" class="mini-dev">
                      #{{ d.device_hw_id }} 健康{{ d.health_score }} <span class="rd" :class="'r-'+d.risk_level"></span>
                    </span>
                  </template>
                </div>
              </div>
              <el-button v-if="viewMode==='device' && sel && sel.device_hw_id === p.device_hw_id" size="small" type="primary" text
                         :loading="enhancing" @click.stop="enhancePlan(p)">LLM预案</el-button>
            </div>
            <el-empty v-if="!loading && filtered.length === 0" description="暂无预测，点击右上角运行" :image-size="60" />
          </div>
        </div>

        <!-- 右侧地图 -->
        <div class="map-box">
          <div ref="mapRef" class="cesium-viewer"></div>
          <div class="map-legend">
            <span><i class="ld r-critical"></i>极高</span>
            <span><i class="ld r-high"></i>高</span>
            <span><i class="ld r-medium"></i>中</span>
            <span><i class="ld r-low"></i>低</span>
          </div>
        </div>
      </div>
    </el-card>

    <!-- LLM 预案展示弹窗 -->
    <el-dialog v-model="planVisible" title="AI 应对预案（LLM 生成）" width="520px">
      <div class="plan-content">{{ planText }}</div>
      <template #footer>
        <el-button @click="planVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { Cpu, VideoPlay } from '@element-plus/icons-vue'
import * as Cesium from 'cesium'
import GaodeImageryProvider from '@/views/map/GaodeImagery.js'
import { runPrediction, runPredictionByIntersection, getPredictions, enhancePredictionPlan } from '@/api/ai'

let viewer: Cesium.Viewer | null = null
const mapRef = ref<HTMLDivElement>()
const running = ref(false)
const loading = ref(false)
const list = ref<any[]>([])
const kw = ref('')
const sel = ref<any>(null)
const enhancing = ref(false)
const planVisible = ref(false)
const planText = ref('')
const riskCount = ref<Record<string, number> | null>(null)
const totalDevices = ref(0)
const viewMode = ref<'device' | 'intersection'>('device')

const filtered = computed(() => {
  const k = kw.value.trim()
  if (!k) return list.value
  return list.value.filter((p) =>
    (p.intersection || '').includes(k) || String(p.device_hw_id || '').includes(k))
})

function onViewChange() {
  sel.value = null
  if (viewMode.value === 'intersection') loadIntersection()
  else loadHistory()
}

function riskColor(level: string): string {
  return { low: '#67c23a', medium: '#e6a23c', high: '#f56c6c', critical: '#b91c1c' }[level] || '#909399'
}

function initMap() {
  if (!mapRef.value || viewer) return
  Cesium.Ion.defaultAccessToken = ''
  viewer = new Cesium.Viewer(mapRef.value, {
    baseLayerPicker: false, geocoder: false, homeButton: false, sceneModePicker: false,
    navigationHelpButton: false, animation: false, timeline: false, fullscreenButton: false,
    infoBox: false, selectionIndicator: false,
  })
  viewer.imageryLayers.removeAll()
  viewer.imageryLayers.addImageryProvider(new (GaodeImageryProvider as any)({ style: 8 }))
  viewer.scene.globe.enableLighting = false
  // 聚焦中国
  viewer.camera.flyTo({ destination: Cesium.Cartesian3.fromDegrees(105, 35, 6000000) })
}

function drawPoints() {
  if (!viewer) return
  viewer.entities.removeAll()
  const isAgg = viewMode.value === 'intersection'
  list.value.forEach((p) => {
    if (p.lat == null || p.lng == null) return
    const color = Cesium.Color.fromCssColorString(riskColor(p.risk_level))
    // 路口聚合用更大的标记
    const size = isAgg ? 22 : 14
    viewer!.entities.add({
      position: Cesium.Cartesian3.fromDegrees(p.lng, p.lat),
      point: { pixelSize: size, color, outlineColor: Cesium.Color.WHITE, outlineWidth: isAgg ? 2 : 1, heightReference: Cesium.HeightReference.CLAMP_TO_GROUND },
      label: { text: isAgg ? (p.intersection || '') + '×' + p.device_count : (p.intersection || ('#' + p.device_hw_id)),
               font: (isAgg ? '13px' : '12px') + ' sans-serif', fillColor: Cesium.Color.WHITE,
               showBackground: true, backgroundColor: Cesium.Color.fromCssColorString('rgba(0,0,0,0.6)'),
               pixelOffset: new Cesium.Cartesian2(0, -16), heightReference: Cesium.HeightReference.CLAMP_TO_GROUND },
    })
  })
}

function focusPred(p: any) {
  sel.value = p
  if (viewer && p.lat != null && p.lng != null) {
    viewer.camera.flyTo({ destination: Cesium.Cartesian3.fromDegrees(p.lng, p.lat, 4000) })
  }
}

async function doRun() {
  running.value = true
  try {
    if (viewMode.value === 'intersection') {
      loadIntersection(true)
    } else {
      const res = await runPrediction()
      const d = res.data || {}
      list.value = d.list || []
      riskCount.value = d.risk_count || null
      totalDevices.value = d.count || 0
      sel.value = null
      drawPoints()
      ElMessage.success(`预测完成：${totalDevices.value} 台设备`)
    }
  } catch {
    ElMessage.error('预测失败')
  } finally {
    running.value = false
  }
}

async function loadIntersection(showMsg = false) {
  loading.value = true
  try {
    const res = await runPredictionByIntersection()
    const d = res.data || {}
    list.value = d.list || []
    riskCount.value = d.risk_count || null
    totalDevices.value = d.device_count || 0
    sel.value = null
    drawPoints()
    if (showMsg) ElMessage.success(`路口聚合：${list.value.length} 个路口 / ${totalDevices.value} 台设备`)
  } catch {
    ElMessage.error('路口聚合失败')
  } finally { loading.value = false }
}

async function enhancePlan(p: any) {
  enhancing.value = true
  try {
    const res = await enhancePredictionPlan(p.id)
    planText.value = res.data?.plan || ''
    ElMessage.success('预案已生成（LLM）')
    planVisible.value = true
  } catch {
    ElMessage.warning('LLM 预案生成失败（可能额度不足，已用规则预案）')
  } finally {
    enhancing.value = false
  }
}

async function loadHistory() {
  loading.value = true
  try {
    const res = await getPredictions()
    const d = res.data || {}
    if (d.list && d.list.length) {
      list.value = d.list
      const rc: Record<string, number> = { low: 0, medium: 0, high: 0, critical: 0 }
      list.value.forEach((p) => { rc[p.risk_level] = (rc[p.risk_level] || 0) + 1 })
      riskCount.value = rc
      totalDevices.value = list.value.length
      drawPoints()
    }
  } catch { /* 忽略 */ }
  finally { loading.value = false }
}

onMounted(() => { initMap(); loadHistory() })
onBeforeUnmount(() => { if (viewer) { viewer.destroy(); viewer = null } })
</script>

<style scoped>
.predict-page { display: flex; flex-direction: column; gap: 12px; height: calc(100vh - 90px); }
.head-card { flex: none; }
.head-bar { display: flex; justify-content: space-between; align-items: center; }
.title { display: flex; align-items: center; gap: 8px; font-weight: 600; }
.sub { font-weight: normal; color: #909399; font-size: 12px; }
.actions { display: flex; align-items: center; gap: 10px; }
.qtag { margin-left: 4px; }
.risk-stats { display: flex; gap: 18px; margin-top: 12px; flex-wrap: wrap; }
.rs b { font-size: 18px; margin-left: 4px; }
.rs.muted { color: #909399; }
.body-card { flex: 1; min-height: 0; }
.body-card :deep(.el-card__body) { height: 100%; padding: 0; }
.body-wrap { display: flex; height: 100%; }
.list-panel { width: 340px; border-right: 1px solid #ebeef5; display: flex; flex-direction: column; }
.list-head { padding: 12px; font-weight: 600; border-bottom: 1px solid #f0f0f0; }
.pred-list { flex: 1; overflow-y: auto; padding: 4px 0; }
.pred-item { display: flex; align-items: center; gap: 8px; padding: 10px 12px; cursor: pointer; border-left: 3px solid transparent; }
.pred-item:hover { background: #f5f7fa; }
.pred-item.active { background: #f0f7ff; border-left-color: #409eff; }
.risk-dot { width: 10px; height: 10px; border-radius: 50%; flex: none; }
.risk-dot.r-low { background: #67c23a; }
.risk-dot.r-medium { background: #e6a23c; }
.risk-dot.r-high { background: #f56c6c; }
.risk-dot.r-critical { background: #b91c1c; }
.pi-main { flex: 1; min-width: 0; }
.pi-name { font-size: 13px; font-weight: 600; }
.pi-sub { font-size: 12px; color: #606266; margin-top: 2px; }
.pi-plan { font-size: 12px; color: #909399; margin-top: 4px; white-space: normal; line-height: 1.5; }
.map-box { flex: 1; position: relative; min-height: 0; }
.cesium-viewer { width: 100%; height: 100%; }
.map-legend { position: absolute; bottom: 16px; left: 12px; background: rgba(255,255,255,0.9); padding: 6px 10px; border-radius: 6px; display: flex; gap: 12px; font-size: 12px; z-index: 10; }
.map-legend .ld { display: inline-block; width: 10px; height: 10px; border-radius: 50%; margin-right: 4px; vertical-align: -1px; }
.ld.r-critical { background: #b91c1c; } .ld.r-high { background: #f56c6c; }
.ld.r-medium { background: #e6a23c; } .ld.r-low { background: #67c23a; }
.plan-content { white-space: pre-wrap; line-height: 1.7; }
</style>
