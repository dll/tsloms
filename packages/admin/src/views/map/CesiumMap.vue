<template>
  <div class="map-screen">
    <!-- 顶部极简工具条 -->
    <div class="map-toolbar">
      <div class="tb-group">
        <el-radio-group v-model="sceneMode" size="small" @change="changeSceneMode">
          <el-radio-button :value="2">2D</el-radio-button>
          <el-radio-button :value="3">3D</el-radio-button>
          <el-radio-button :value="1">哥伦布</el-radio-button>
        </el-radio-group>
        <el-radio-group v-model="baseLayer" size="small" @change="switchBaseLayer">
          <el-radio-button value="osm">OSM</el-radio-button>
          <el-radio-button value="gaode">高德</el-radio-button>
          <el-radio-button value="satellite">卫星</el-radio-button>
          <el-radio-button value="baidu">百度</el-radio-button>
        </el-radio-group>
      </div>
      <div class="tb-stats">
        <span class="s on">在线 <b>{{ onlineCount }}</b></span>
        <span class="s off">离线 <b>{{ offlineCount }}</b></span>
        <span class="s">已定位 <b>{{ mappedCount }}</b></span>
      </div>
      <div class="tb-actions">
        <el-button size="small" @click="togglePanel">{{ panelOpen ? '收起设备' : '设备列表' }}</el-button>
        <el-button size="small" @click="flyToAll">全览</el-button>
      </div>
    </div>

    <!-- 地图区域（占满） -->
    <div class="map-wrap">
      <div ref="cesiumRef" class="cesium-viewer" :class="{ fullscreen: isFullscreen }"></div>

      <!-- 地图工具（右下角） -->
      <div class="map-tools">
        <div class="t" title="放大" @click="zoomIn"><el-icon><ZoomIn /></el-icon></div>
        <div class="t" title="缩小" @click="zoomOut"><el-icon><ZoomOut /></el-icon></div>
        <div class="t" title="复位" @click="resetView"><el-icon><Aim /></el-icon></div>
        <div class="t" title="全屏" @click="toggleFullscreen"><el-icon><FullScreen /></el-icon></div>
      </div>

      <!-- 左侧设备列表（可收起） -->
      <div class="dev-panel" v-show="panelOpen">
        <div class="panel-head">设备定位</div>
        <el-input v-model="searchKw" placeholder="搜索路口/设备ID" size="small" clearable style="padding: 8px 10px" />
        <div class="dev-list">
          <div v-for="d in filteredDevices" :key="d.hw_id" class="dev-item"
               :class="{ active: selDev && selDev.hw_id === d.hw_id }"
               @click="focusDevice(d); openInfo(d)">
            <span class="dot" :class="d.online_status ? 'on' : 'off'"></span>
            <span class="nm">{{ d.intersection || '#' + d.hw_id }}</span>
            <span v-if="d.lat != null && d.lng != null" class="loc">已定位</span>
            <span v-else class="loc nl">未定位</span>
          </div>
          <el-empty v-if="filteredDevices.length === 0" description="暂无设备" :image-size="50" />
        </div>
      </div>

      <!-- 设备信息卡（右侧 DOM，中文正常显示） -->
      <div v-if="selDev" class="info-card">
        <div class="info-head">
          <span>{{ selDev.intersection || ('设备#' + selDev.hw_id) }}</span>
          <el-button size="small" circle @click="closeInfo">×</el-button>
        </div>
        <div class="info-body">
          <div class="kv"><span>硬件ID</span><b>#{{ selDev.hw_id }}</b></div>
          <div class="kv"><span>在线状态</span>
            <el-tag size="small" :type="selDev.online_status ? 'success' : 'info'">{{ selDev.online_status ? '在线' : '离线' }}</el-tag>
          </div>
          <div class="kv"><span>坐标</span>
            <b v-if="selDev.lat != null">{{ selDev.lat?.toFixed(5) }}, {{ selDev.lng?.toFixed(5) }}</b>
            <b v-else class="muted">未设置</b>
          </div>
          <div class="kv"><span>活跃故障</span><el-tag size="small" :type="selDev.faultCount > 0 ? 'danger' : 'success'">{{ selDev.faultCount }}</el-tag></div>
        </div>
        <div class="info-actions">
          <el-button size="small" @click="goFault">故障</el-button>
          <el-button size="small" @click="goWorkOrder">工单</el-button>
          <el-button size="small" @click="goVideo">监控</el-button>
          <el-button size="small" @click="goFeedback">反馈</el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import * as Cesium from 'cesium'
import { getAllDevices } from '@/api/map'
import { getUserInfo } from '@/api/auth'
// @ts-ignore 百度/高德瓦片
import BaiduImageryProvider from './BaiduImagery.js'
// @ts-ignore
import GaodeImageryProvider from './GaodeImagery.js'

;(Cesium as any).buildModuleUrl.setBaseUrl('/tsloms/admin/cesium/')

const router = useRouter()
const cesiumRef = ref<HTMLElement>()
let viewer: Cesium.Viewer | null = null
let resizeHandler: () => void = () => {}

const sceneMode = ref(2) // 默认 2D 地图
const baseLayer = ref('gaode') // 默认高德地图
const searchKw = ref('')
const panelOpen = ref(true)
const isFullscreen = ref(false)
const devices = ref<any[]>([])
const selDev = ref<any | null>(null)
const userCenterRef = ref<{ lat: number; lng: number } | null>(null)

const onlineCount = computed(() => devices.value.filter((d) => d.online_status).length)
const offlineCount = computed(() => devices.value.filter((d) => !d.online_status).length)
const mappedCount = computed(() => devices.value.filter((d) => d.lat != null && d.lng != null).length)
const filteredDevices = computed(() => {
  const kw = searchKw.value.trim()
  if (!kw) return devices.value
  return devices.value.filter((d) => (d.intersection || '').includes(kw) || String(d.hw_id).includes(kw))
})

function initCesium() {
  if (!cesiumRef.value || viewer) return
  // 等容器有真实尺寸后再初始化，避免 Cesium 回落到 300x150 默认
  const container = cesiumRef.value
  if (container.clientWidth < 50 || container.clientHeight < 50) {
    requestAnimationFrame(() => initCesium())
    return
  }
  Cesium.Ion.defaultAccessToken = ''
  viewer = new Cesium.Viewer(container, {
    baseLayerPicker: false,
    geocoder: true,
    homeButton: true,
    sceneModePicker: false, // 用顶部按钮切换
    navigationHelpButton: false,
    animation: false,
    timeline: false,
    fullscreenButton: false, // 用自定义全屏
    infoBox: false,
    selectionIndicator: false,
  })
  viewer.scene.screenSpaceCameraController.minimumZoomDistance = 100
  viewer.scene.screenSpaceCameraController.maximumZoomDistance = 30000000
  ;(window as any).__tslomsViewer = viewer // 便于调试/验证相机
  // 默认视角：先定位到中国东部（避免全世界），设备加载后自动聚焦到设备分布
  viewer.camera.setView({ destination: Cesium.Cartesian3.fromDegrees(104.0, 30.0, 3000000) })
  applySceneMode()
  // 点击设备点位
  const handler = new Cesium.ScreenSpaceEventHandler(viewer.scene.canvas)
  handler.setInputAction((evt: any) => {
    const picked = viewer?.scene.pick(evt.position)
    if (picked && picked.id && (picked.id as any).id && String((picked.id as any).id).startsWith('dev-')) {
      const props = (picked.id as any).properties
      const hw = props && props.hw_id !== undefined ? props.hw_id.getValue() : undefined
      const dev = devices.value.find((d) => d.hw_id === hw)
      if (dev) openInfo(dev)
    }
  }, Cesium.ScreenSpaceEventType.LEFT_CLICK)
  // 布局稳定后强制重算尺寸
  requestAnimationFrame(() => { try { viewer?.resize() } catch { /* 忽略 */ } })
  setTimeout(() => { try { viewer?.resize() } catch { /* 忽略 */ } }, 200)
}

// 设备点位：仅用圆点/3D标记，不带 Canvas 文字（避免乱码与遮挡）
function plotDevices() {
  if (!viewer) return
  viewer.entities.removeAll()
  for (const d of devices.value) {
    if (d.lat == null || d.lng == null) continue
    const color = d.online_status ? Cesium.Color.GREEN : Cesium.Color.RED
    viewer.entities.add({
      id: 'dev-' + d.hw_id,
      position: Cesium.Cartesian3.fromDegrees(d.lng, d.lat),
      point: {
        pixelSize: 10,
        color, outlineColor: Cesium.Color.WHITE, outlineWidth: 2,
        heightReference: Cesium.HeightReference.CLAMP_TO_GROUND,
      },
      properties: d as any,
    })
  }
}

function openInfo(d: any) {
  // 统计该设备活跃故障数（从 devices 无，这里从派单参考拉取或简单置0）
  selDev.value = { ...d, faultCount: 0 }
  focusDevice(d)
}
function closeInfo() { selDev.value = null }

function focusDevice(d: any) {
  if (!viewer || d.lat == null || d.lng == null) return
  viewer.camera.flyTo({
    destination: Cesium.Cartesian3.fromDegrees(d.lng, d.lat, 2000),
    orientation: { heading: 0, pitch: -Math.PI / 2.2, roll: 0 },
    duration: 0.8,
  })
}
function flyToAll() {
  const mapped = devices.value.filter((d) => d.lat != null && d.lng != null)
  if (mapped.length && viewer) {
    const pos = mapped.map((d) => Cesium.Cartesian3.fromDegrees(d.lng, d.lat))
    viewer.camera.flyToBoundingSphere(Cesium.BoundingSphere.fromPoints(pos), { duration: 1 })
  }
}

// 场景模式
function applySceneMode() {
  if (!viewer) return
  if (sceneMode.value === 2) viewer.scene.morphTo2D(2.0)
  else if (sceneMode.value === 1) viewer.scene.morphToColumbusView(2.0)
  else viewer.scene.morphTo3D(2.0)
}
function changeSceneMode() { applySceneMode() }

// 底图切换
function switchBaseLayer() {
  if (!viewer) return
  viewer.imageryLayers.removeAll()
  try {
    if (baseLayer.value === 'gaode') viewer.imageryLayers.addImageryProvider(new (GaodeImageryProvider as any)())
    else if (baseLayer.value === 'satellite') {
      // 高德纯卫星影像（style=6，无路网、更清晰）
      viewer.imageryLayers.addImageryProvider(new (GaodeImageryProvider as any)({ style: 6 }))
    }
    else if (baseLayer.value === 'baidu') viewer.imageryLayers.addImageryProvider(new (BaiduImageryProvider as any)())
    else viewer.imageryLayers.addImageryProvider(new Cesium.OpenStreetMapImageryProvider({ url: 'https://tile.openstreetmap.org/' }))
  } catch { ElMessage.error('底图加载失败') }
}

// 工具
function zoomIn() { if (viewer) viewer.camera.zoomIn(viewer.camera.positionCartographic.height * 0.3) }
function zoomOut() { if (viewer) viewer.camera.zoomOut(viewer.camera.positionCartographic.height * 0.3) }
function resetView() { if (viewer) viewer.camera.flyTo({ destination: Cesium.Cartesian3.fromDegrees(105, 35, 5000000), duration: 1 }) }
function toggleFullscreen() {
  const el = document.querySelector('.cesium-viewer') as HTMLElement
  if (!el) return
  if (!document.fullscreenElement) { el.requestFullscreen?.().then(() => { isFullscreen.value = true }); }
  else { document.exitFullscreen?.(); isFullscreen.value = false }
}
function togglePanel() { panelOpen.value = !panelOpen.value }

// 跳转
function goFault() { router.push({ path: '/fault', query: { hw_id: selDev.value?.hw_id } }) }
function goWorkOrder() { router.push({ path: '/workorder', query: { device_hw_id: selDev.value?.hw_id } }) }
function goVideo() { router.push({ path: '/video', query: { device_hw_id: selDev.value?.hw_id } }) }
function goFeedback() { router.push({ path: '/feedback', query: { device_hw_id: selDev.value?.hw_id } }) }

async function load() {
  try {
    const res = await getAllDevices(1000)
    devices.value = res.data?.list || []
    plotDevices()
    // 自动聚焦：优先以当前登录用户为中心点，否则聚焦到设备分布
    focusWithRetry(0)
  } catch { /* 忽略 */ }
}

// 自动聚焦（带重试）：2D/3D 变形与相机就绪需时间，多次尝试直到相机真正移动
function focusWithRetry(attempt: number) {
  if (!viewer) return
  if (attempt > 5) return
  setTimeout(() => {
    autoFocusUserOrDevices()
    // 判断相机是否已离开初始默认视角（未移动则重试）
    const c = viewer?.camera.positionCartographic
    const stillHome = c && c.longitude === 0 && c.latitude === 0
    if (stillHome) focusWithRetry(attempt + 1)
  }, attempt === 0 ? 400 : attempt * 700)
}

// 自动聚焦：当前用户设置了中心点则以其为中心，否则聚焦设备分布
function autoFocusUserOrDevices() {
  if (!viewer) return
  const userCenter = userCenterRef.value
  if (userCenter && userCenter.lat != null && userCenter.lng != null) {
    viewer.camera.flyTo({
      destination: Cesium.Cartesian3.fromDegrees(userCenter.lng, userCenter.lat, 5000),
      orientation: { heading: 0, pitch: -Math.PI / 2.2, roll: 0 },
      duration: 1.2,
    })
  } else {
    autoFocusDevices()
  }
}

// 自动聚焦到设备分布：有设备则贴合设备范围；单台设备聚焦到清晰高度
function autoFocusDevices() {
  if (!viewer) return
  const mapped = devices.value.filter((d) => d.lat != null && d.lng != null)
  if (mapped.length === 0) return
  if (mapped.length === 1) {
    const d = mapped[0]
    viewer.camera.flyTo({
      destination: Cesium.Cartesian3.fromDegrees(d.lng, d.lat, 4000),
      orientation: { heading: 0, pitch: -Math.PI / 2.2, roll: 0 },
      duration: 1.2,
    })
  } else {
    const pos = mapped.map((d) => Cesium.Cartesian3.fromDegrees(d.lng, d.lat))
    viewer.camera.flyToBoundingSphere(Cesium.BoundingSphere.fromPoints(pos), { duration: 1.2 })
  }
}

onMounted(async () => {
  await nextTick()
  initCesium()
  // 读取当前登录用户的地图中心点（该用户管辖区域）
  try {
    const ui = await getUserInfo()
    const u = ui.data?.user
    if (u && u.center_lat != null && u.center_lng != null) {
      userCenterRef.value = { lat: u.center_lat, lng: u.center_lng }
    }
  } catch { /* 忽略 */ }
  await load()
  resizeHandler = () => { try { viewer?.resize() } catch { /* 忽略 */ } }
  window.addEventListener('resize', resizeHandler)
  // 布局完全稳定后再次重算（避免 flex/calc 未定导致 canvas 尺寸错误）
  setTimeout(resizeHandler, 300)
  setTimeout(resizeHandler, 800)
})
onUnmounted(() => {
  window.removeEventListener('resize', resizeHandler)
  if (viewer) { viewer.destroy(); viewer = null }
})
</script>

<style scoped>
.map-screen { width: 100%; height: 100%; display: flex; flex-direction: column; }
.map-toolbar {
  display: flex; align-items: center; gap: 14px; padding: 6px 10px;
  background: #fff; border-radius: 4px; margin-bottom: 6px; flex-wrap: wrap;
}
.tb-group { display: flex; gap: 10px; }
.tb-stats { display: flex; gap: 14px; font-size: 13px; color: #606266; flex: 1; }
.tb-stats .s b { font-size: 15px; }
.s.on b { color: #67C23A; }
.s.off b { color: #F56C6C; }
.tb-actions { display: flex; gap: 8px; }

.map-wrap { position: relative; flex: 1; min-height: 0; border-radius: 6px; overflow: hidden; }
.cesium-viewer { width: 100%; height: 100%; }
/* Cesium 全链路高度 100%，避免 canvas 回落到默认 300x150 */
.cesium-viewer,
.cesium-viewer .cesium-viewer-cesiumWidget,
.cesium-viewer .cesium-widget,
.cesium-viewer .cesium-widget canvas,
.cesium-viewer canvas {
  width: 100% !important;
  height: 100% !important;
}
.cesium-viewer.fullscreen { position: fixed; inset: 0; z-index: 100; }

/* 地图工具（右下角） */
.map-tools {
  position: absolute; right: 12px; bottom: 24px; z-index: 20;
  display: flex; flex-direction: column; background: #fff; border-radius: 6px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.15); overflow: hidden;
}
.map-tools .t { width: 34px; height: 34px; display: flex; align-items: center; justify-content: center; cursor: pointer; color: #303133; font-size: 18px; }
.map-tools .t:hover { background: #409eff; color: #fff; }

/* 左侧设备列表 */
.dev-panel {
  position: absolute; left: 10px; top: 10px; bottom: 10px; width: 220px; z-index: 15;
  background: rgba(255,255,255,0.96); border-radius: 6px;
  box-shadow: 0 2px 12px rgba(0,21,41,0.12); display: flex; flex-direction: column;
}
.panel-head { font-weight: 600; padding: 10px 12px; border-bottom: 1px solid #eee; font-size: 14px; }
.dev-list { flex: 1; overflow-y: auto; padding: 0 8px 8px; }
.dev-item { display: flex; align-items: center; gap: 6px; padding: 6px 6px; cursor: pointer; border-radius: 4px; font-size: 13px; }
.dev-item:hover { background: #f0f7ff; }
.dev-item.active { background: #e8f3ff; }
.dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.dot.on { background: #67C23A; }
.dot.off { background: #F56C6C; }
.nm { flex: 1; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.loc { color: #909399; font-size: 12px; flex-shrink: 0; }
.nl { color: #c0c4cc; }

/* 右侧设备信息卡 */
.info-card {
  position: absolute; right: 12px; top: 12px; width: 260px; z-index: 15;
  background: #fff; border-radius: 6px; box-shadow: 0 2px 12px rgba(0,21,41,0.15);
}
.info-head { display: flex; justify-content: space-between; align-items: center; padding: 10px 12px; border-bottom: 1px solid #eee; font-weight: 600; }
.info-body { padding: 10px 12px; }
.kv { display: flex; justify-content: space-between; align-items: center; padding: 5px 0; font-size: 13px; color: #606266; }
.kv b { color: #303133; }
.kv .muted { color: #c0c4cc; }
.info-actions { border-top: 1px solid #eee; padding: 8px 12px; display: flex; gap: 6px; flex-wrap: wrap; }
</style>
