<template>
  <div class="map-screen">
    <!-- 顶部工具条 -->
    <div class="map-toolbar">
      <!-- 场景/底图 -->
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

      <!-- 图层管理 -->
      <div class="tb-group layer-group">
        <span class="layer-label">图层</span>
        <el-checkbox v-model="layerSignal" @change="onLayerChange" title="信号灯（在线/离线）">信号灯</el-checkbox>
        <el-checkbox v-model="layerFault" @change="onLayerChange" title="故障信号灯（红色警示）">故障</el-checkbox>
        <el-checkbox v-model="layerWatched" @change="onLayerChange" title="锁定/关注的信号灯（金色星标）">锁定</el-checkbox>
        <el-checkbox v-model="layerGradient" @change="onLayerChange" title="路口故障分级渐变着色(绿→黄→红,按故障比例)">路口分级</el-checkbox>
      </div>

      <div class="tb-stats">
        <span class="s on">在线 <b>{{ onlineCount }}</b></span>
        <span class="s off">离线 <b>{{ offlineCount }}</b></span>
        <span class="s">已定位 <b>{{ mappedCount }}</b></span>
        <span class="s warn">关注 <b>{{ watchedCount }}</b></span>
        <span class="s fault">故障 <b>{{ faultDeviceCount }}</b></span>
      </div>
      <div class="tb-actions">
        <el-button size="small" @click="togglePanel">{{ panelOpen ? '收起设备' : '设备列表' }}</el-button>
        <el-button size="small" @click="flyToAll">全览</el-button>
        <el-button size="small" :type="realtime ? 'danger' : 'default'" @click="toggleRealtime">实时</el-button>
      </div>
    </div>

    <!-- 地图区域 -->
    <div class="map-wrap">
      <div ref="cesiumRef" class="cesium-viewer" :class="{ fullscreen: isFullscreen }"></div>

      <!-- 缩放级别快捷定位（右下角，含 tooltip + 快捷键提示） -->
      <div class="map-zoom" v-show="zoomPanelOpen">
        <div class="zoom-head">
          <span>快速定位</span>
          <span class="zoom-close" @click="zoomPanelOpen = false">×</span>
        </div>
        <div class="zoom-item" title="缩放到单个信号灯（500m）" @click="zoomToSignal">
          <b>1</b><span>单个信号灯</span><i>500m</i>
        </div>
        <div class="zoom-item" title="缩放到路口（1500m）" @click="zoomToIntersection">
          <b>2</b><span>路口</span><i>1500m</i>
        </div>
        <div class="zoom-item" title="聚合信号灯/片区（5000m）" @click="zoomToAggregate">
          <b>3</b><span>聚合信号灯</span><i>5km</i>
        </div>
        <div class="zoom-item" title="整条路（15000m）" @click="zoomToRoute">
          <b>4</b><span>整条路</span><i>15km</i>
        </div>
        <div class="zoom-item" title="街区（40000m）" @click="zoomToBlock">
          <b>5</b><span>街区</span><i>40km</i>
        </div>
        <div class="zoom-item" title="全览：套住所有已定位设备" @click="flyToAll">
          <b>6</b><span>全览</span><i>自动</i>
        </div>
        <div class="zoom-custom">
          <el-input-number v-model="customHeight" :min="100" :max="3000000" :step="500" size="small" controls-position="right" style="width: 120px" />
          <el-button size="small" type="primary" @click="zoomToCustom">定位</el-button>
        </div>
        <div class="zoom-tip">数字键 1-6 快捷缩放 · 自定义填高度(米)</div>
      </div>
      <!-- 悬浮缩放按钮 + 自定义高度 -->
      <div class="map-tools">
        <div class="t" title="快速定位（缩放级别）" @click="zoomPanelOpen = !zoomPanelOpen"><el-icon><Position /></el-icon></div>
        <div class="t" title="放大 (+)" @click="zoomIn"><el-icon><ZoomIn /></el-icon></div>
        <div class="t" title="缩小 (-)" @click="zoomOut"><el-icon><ZoomOut /></el-icon></div>
        <div class="t" title="复位" @click="resetView"><el-icon><Aim /></el-icon></div>
        <div class="t" title="全屏 (F)" @click="toggleFullscreen"><el-icon><FullScreen /></el-icon></div>
      </div>

      <!-- 左侧设备列表 -->
      <div class="dev-panel" v-show="panelOpen">
        <div class="panel-head">
          设备定位
          <el-switch v-model="watchedFilter" size="small" style="margin-left:6px" title="只看关注设备" />
        </div>
        <el-input v-model="searchKw" placeholder="搜索路口/设备ID" size="small" clearable style="padding: 8px 10px" />
        <div class="dev-list">
          <div v-for="d in filteredDevices" :key="d.hw_id" class="dev-item"
               :class="{ active: selDev && selDev.hw_id === d.hw_id }"
               @click="focusDevice(d); openInfo(d)">
            <span class="dot" :class="d.online_status ? 'on' : 'off'"></span>
            <span class="nm">{{ d.intersection || '#' + d.hw_id }}</span>
            <span v-if="d.is_watched" class="star" title="关注">★</span>
            <span v-if="faultByDev[d.hw_id]" class="star warn" title="故障">!</span>
            <span v-if="d.lat != null && d.lng != null" class="loc">已定位</span>
            <span v-else class="loc nl">未定位</span>
          </div>
          <el-empty v-if="filteredDevices.length === 0" description="暂无设备" :image-size="50" />
        </div>
      </div>

      <!-- 设备信息卡 -->
      <div v-if="selDev" class="info-card">
        <div class="info-head">
          <span>{{ selDev.intersection || ('设备#' + selDev.hw_id) }}</span>
          <div class="info-head-ops">
            <el-button size="small" circle :type="selDev.is_watched ? 'warning' : 'default'" :title="selDev.is_watched ? '取消关注' : '关注/锁定'" @click="toggleWatch" style="margin-right:4px">{{ selDev.is_watched ? '★' : '☆' }}</el-button>
            <el-button size="small" circle @click="closeInfo">×</el-button>
          </div>
        </div>
        <div class="info-body">
          <div class="kv"><span>硬件ID</span><b>#{{ selDev.hw_id }}</b></div>
          <div class="kv"><span>在线状态</span>
            <el-tag size="small" :type="selDev.online_status ? 'success' : 'info'">{{ selDev.online_status ? '在线' : '离线' }}</el-tag>
          </div>
          <div class="kv"><span>关注/锁定</span>
            <el-tag size="small" :type="selDev.is_watched ? 'warning' : 'info'">{{ selDev.is_watched ? '已关注' : '未关注' }}</el-tag>
          </div>
          <div class="kv"><span>活跃故障</span><el-tag size="small" :type="faultByDev[selDev.hw_id] ? 'danger' : 'success'">{{ faultByDev[selDev.hw_id] || 0 }}</el-tag></div>
          <div class="kv"><span>坐标</span>
            <b v-if="selDev.lat != null">{{ selDev.lat?.toFixed(5) }}, {{ selDev.lng?.toFixed(5) }}</b>
            <b v-else class="muted">未设置</b>
          </div>
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
import { getFaults } from '@/api/fault'
import { updateDevice } from '@/api/device'
import { getUserInfo } from '@/api/auth'
import { getSignalIcon } from './signalIcons'
import { getCrossingMapData, type CrossingPoly } from '@/api/map'
import { bus, consumePendingFocus } from '@/utils/eventBus'
// @ts-ignore 百度/高德瓦片
import BaiduImageryProvider from './BaiduImagery.js'
// @ts-ignore
import GaodeImageryProvider from './GaodeImagery.js'

;(Cesium as any).buildModuleUrl.setBaseUrl('/tsloms/admin/cesium/')

const router = useRouter()
const cesiumRef = ref<HTMLElement>()
let viewer: Cesium.Viewer | null = null
let resizeHandler: () => void = () => {}

const sceneMode = ref(2) // 默认 2D
const baseLayer = ref('gaode') // 默认高德
const searchKw = ref('')
const panelOpen = ref(true)
const isFullscreen = ref(false)
const devices = ref<any[]>([])
const selDev = ref<any | null>(null)
const userCenterRef = ref<{ lat: number; lng: number } | null>(null)

// ---- 图层开关 ----
const layerSignal = ref(true)   // 信号灯
const layerFault = ref(true)    // 故障
const layerWatched = ref(false) // 锁定/关注
const layerGradient = ref(true) // 路口故障分级渐变着色(按故障比例 绿→黄→红)
const crossings = ref<CrossingPoly[]>([])
const watchedFilter = ref(false)// 设备列表只看关注

// ---- 缩放级别 ----
const zoomPanelOpen = ref(false)
const customHeight = ref(1000)

// ---- 故障映射（device_hw_id → 故障数）----
const faultByDev = ref<Record<number, number>>({})
const faultDeviceCount = computed(() => Object.keys(faultByDev.value).length)

const onlineCount = computed(() => devices.value.filter((d) => d.online_status).length)
const offlineCount = computed(() => devices.value.filter((d) => !d.online_status).length)
const mappedCount = computed(() => devices.value.filter((d) => d.lat != null && d.lng != null).length)
const watchedCount = computed(() => devices.value.filter((d) => d.is_watched).length)
const filteredDevices = computed(() => {
  let arr = devices.value
  if (watchedFilter.value) arr = arr.filter((d) => d.is_watched)
  const kw = searchKw.value.trim()
  if (kw) arr = arr.filter((d) => (d.intersection || '').includes(kw) || String(d.hw_id).includes(kw))
  return arr
})

function initCesium() {
  if (!cesiumRef.value || viewer) return
  const container = cesiumRef.value
  if (container.clientWidth < 50 || container.clientHeight < 50) {
    requestAnimationFrame(() => initCesium())
    return
  }
  Cesium.Ion.defaultAccessToken = ''
  viewer = new Cesium.Viewer(container, {
    baseLayerPicker: false,
    baseLayer: false,
    terrainProvider: new (Cesium.EllipsoidTerrainProvider as any)(),
    geocoder: true,
    homeButton: true,
    sceneModePicker: false,
    navigationHelpButton: false,
    animation: false,
    timeline: false,
    fullscreenButton: false,
    infoBox: false,
    selectionIndicator: false,
  })
  viewer.terrainProvider = new (Cesium.EllipsoidTerrainProvider as any)()
  viewer.scene.screenSpaceCameraController.minimumZoomDistance = 100
  viewer.scene.screenSpaceCameraController.maximumZoomDistance = 30000000
  ;(window as any).__tslomsViewer = viewer
  switchBaseLayer()
  viewer.camera.setView({ destination: Cesium.Cartesian3.fromDegrees(104.0, 30.0, 3000000) })
  applySceneMode()
  // 点击设备点位 / 路口(分级)点位
  const handler = new Cesium.ScreenSpaceEventHandler(viewer.scene.canvas)
  handler.setInputAction((evt: any) => {
    const picked = viewer?.scene.pick(evt.position)
    if (!picked || !(picked.id as any)?.id) return
    const id = String((picked.id as any).id)
    // 路口(分级)点位 → 下钻聚焦该路口并高亮其设备
    if (id.startsWith('cross-grad-')) {
      const data = (picked.id as any).properties?.data?.getValue()
      if (data) drillCrossing(data)
      return
    }
    // 设备点位
    if (id.startsWith('dev-')) {
      const props = (picked.id as any).properties
      const hw = props && props.hw_id !== undefined ? props.hw_id.getValue() : undefined
      const dev = devices.value.find((d) => d.hw_id === hw)
      if (dev) openInfo(dev)
    }
  }, Cesium.ScreenSpaceEventType.LEFT_CLICK)
  requestAnimationFrame(() => { try { viewer?.resize() } catch { /* 忽略 */ } })
  setTimeout(() => { try { viewer?.resize() } catch { /* 忽略 */ } }, 200)
}

// 图层重绘：按当前开关 + 状态绘制信号灯图标
// 由故障比例推导着色：全绿(0)→黄(中)→红(≥1) 渐变（对齐后端 level）
function gradientColor(ratio: number): string {
  if (ratio <= 0) return '#3ecf6a'      // 全绿/正常
  if (ratio >= 1) return '#e02020'      // 全红/停电
  // 0<ratio<1：绿→黄→橙→红
  if (ratio < 0.34) return '#f5e62b'    // 黄低
  if (ratio < 0.67) return '#f5a623'    // 黄/橙
  return '#e02020'                       // 红
}

// 路口故障分级渐变着色图层：按 fault_ratio 给每个路口画彩色圆环（绿→黄→红）
function plotCrossings() {
  if (!viewer) return
  const v = viewer
  const map = crossings.value
  // 移除旧的渐变层（按实体 id 前缀）
  const old: string[] = []
  v.entities.values.forEach((e: any) => { if (e.id && String(e.id).startsWith('cross-grad-')) old.push(e.id) })
  old.forEach((id) => v.entities.remove(v.entities.getById(id) as any))
  if (!layerGradient.value) return

  for (const x of map) {
    if (x.lat == null || x.lng == null) continue
    const color = gradientColor(x.fault_ratio)
    const id = 'cross-grad-' + x.id
    const ent = v.entities.add({
      id,
      position: Cesium.Cartesian3.fromDegrees(x.lng, x.lat),
      ellipse: {
        semiMajorAxis: 90,
        semiMinorAxis: 90,
        material: Cesium.Color.fromCssColorString(color).withAlpha(0.55),
        outline: true,
        outlineColor: Cesium.Color.fromCssColorString(color).withAlpha(0.9),
        height: 0,
      },
      properties: { kind: 'crossing', data: x } as any,
    })
    // 点击路口 → 聚焦该路口（下钻到路口层）
    ;(ent as any).label = {
      text: x.name || ('路口#' + x.id),
      font: '11px sans-serif',
      fillColor: Cesium.Color.WHITE,
      outlineColor: Cesium.Color.BLACK,
      outlineWidth: 3,
      style: Cesium.LabelStyle.FILL_AND_OUTLINE,
      verticalOrigin: Cesium.VerticalOrigin.BOTTOM,
      pixelOffset: new Cesium.Cartesian2(0, -14),
      disableDepthTestDistance: Number.POSITIVE_INFINITY,
    }
  }
}

async function loadCrossings() {
  try {
    const res = await getCrossingMapData()
    crossings.value = (res.data?.list || []) as CrossingPoly[]
  } catch {
    crossings.value = []
  }
}

function plotDevices() {
  if (!viewer) return
  viewer.entities.removeAll()
  for (const d of devices.value) {
    if (d.lat == null || d.lng == null) continue
    const hw = d.hw_id
    const hasFault = !!faultByDev.value[hw]
    const watched = !!d.is_watched
    // 该设备是否在当前图层组合中可见
    let visible = false
    if (layerSignal.value) visible = true
    if (layerWatched.value && watched) visible = true
    if (layerFault.value && hasFault) visible = true
    if (!visible) continue

    const icon = getSignalIcon({ online: d.online_status, fault: hasFault, watched })
    const scale = hasFault ? 1.3 : watched ? 1.2 : 0.9
    viewer.entities.add({
      id: 'dev-' + hw,
      position: Cesium.Cartesian3.fromDegrees(d.lng, d.lat),
      billboard: {
        image: icon,
        width: 32 * scale,
        height: 36 * scale,
        verticalOrigin: Cesium.VerticalOrigin.BOTTOM,
        disableDepthTestDistance: Number.POSITIVE_INFINITY,
      },
      properties: d as any,
    })
  }
}

// 撤销图层调整后立即重绘
function onLayerChange() { plotDevices(); plotCrossings() }

// ---- 关注/锁定 ----
async function toggleWatch() {
  if (!selDev.value) return
  const dev = selDev.value
  const next = !dev.is_watched
  try {
    await updateDevice(dev.id, { is_watched: next })
    dev.is_watched = next
    // 同步 devices 列表
    const src = devices.value.find((x) => x.hw_id === dev.hw_id)
    if (src) src.is_watched = next
    bus.emit('device:watched', { hw_id: dev.hw_id, is_watched: next })
    ElMessage.success(next ? '已关注/锁定' : '已取消关注')
    plotDevices()
  } catch { ElMessage.error('操作失败') }
}

function openInfo(d: any) {
  selDev.value = { ...d, faultCount: faultByDev.value[d.hw_id] || 0 }
  focusDevice(d)
}
function closeInfo() { selDev.value = null }

// 路口下钻：聚焦该路口，并突出显示其关联设备（按 intersection 名称匹配；无则按聚合设备定位）
function drillCrossing(x: any) {
  if (!viewer || x.lat == null || x.lng == null) return
  // 按路口名匹配设备
  const match = devices.value.filter((d) => d.intersection && String(d.intersection) === String(x.name))
  if (match.length > 0) {
    // 聚焦到这些设备的聚合范围
    const m = match.filter((d) => d.lat != null && d.lng != null)
    if (m.length > 0 && viewer) {
      const rect = Cesium.Rectangle.fromCartesianArray(m.map((d) => Cesium.Cartesian3.fromDegrees(d.lng, d.lat)))
      viewer.camera.flyTo({ destination: rect, duration: 0.9 })
    }
    ElMessage.success(`路口「${x.name}」共 ${match.length} 台设备（${match.filter((d) => d.online_status).length} 在线）`)
    return
  }
  // 无命名匹配：直接飞至路口中心并缩放到路口级（1500m）
  viewer.camera.flyTo({
    destination: Cesium.Cartesian3.fromDegrees(x.lng, x.lat, 1500),
    orientation: { heading: 0, pitch: -Math.PI / 2.2, roll: 0 },
    duration: 0.9,
  })
  ElMessage.info(`路口「${x.name}」故障比例 ${Math.round((x.fault_ratio || 0) * 100)}%（下钻到最近型号灯）`)
}

// 实时监控：定时刷新设备/故障/路口分级着色
let refreshTimer: ReturnType<typeof setInterval> | null = null
const refreshInterval = ref(30) // 秒
const realtime = ref(false)

function toggleRealtime() {
  realtime.value = !realtime.value
  if (realtime.value) {
    refreshTimer = setInterval(() => { load() }, refreshInterval.value * 1000)
    ElMessage.success(`实时监控已开启（每 ${refreshInterval.value}s 刷新）`)
  } else {
    if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
    ElMessage.info('实时监控已关闭')
  }
}

function focusDevice(d: any) {
  if (!viewer || d.lat == null || d.lng == null) return
  viewer.camera.flyTo({
    destination: Cesium.Cartesian3.fromDegrees(d.lng, d.lat, 2000),
    orientation: { heading: 0, pitch: -Math.PI / 2.2, roll: 0 },
    duration: 0.8,
  })
}

// ---- 缩放级别定位 ----
function flyToLevel(height: number, duration = 0.8) {
  if (!viewer) return
  const center = viewer.camera.positionCartographic
  viewer.camera.flyTo({
    destination: Cesium.Cartesian3.fromDegrees(
      (center.longitude * 180) / Math.PI,
      (center.latitude * 180) / Math.PI,
      height
    ),
    orientation: { heading: 0, pitch: -Math.PI / 2.2, roll: 0 },
    duration,
  })
}
function zoomToSignal() { flyToLevel(500) }
function zoomToIntersection() { flyToLevel(1500) }
function zoomToAggregate() { flyToLevel(5000) }
function zoomToRoute() { flyToLevel(15000) }
function zoomToBlock() { flyToLevel(40000) }
function zoomToCustom() {
  if (!customHeight.value) { ElMessage.warning('请输入高度'); return }
  flyToLevel(customHeight.value)
}
function flyToAll() {
  const mapped = devices.value.filter((d) => d.lat != null && d.lng != null)
  if (mapped.length && viewer) {
    const pos = mapped.map((d) => Cesium.Cartesian3.fromDegrees(d.lng, d.lat))
    viewer.camera.flyToBoundingSphere(Cesium.BoundingSphere.fromPoints(pos), { duration: 1 })
  }
}

// ---- 键盘快捷键 ----
function onKeydown(e: KeyboardEvent) {
  if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return
  const map: Record<string, () => void> = {
    '1': zoomToSignal,
    '2': zoomToIntersection,
    '3': zoomToAggregate,
    '4': zoomToRoute,
    '5': zoomToBlock,
    '6': flyToAll,
    '+': zoomIn, '=': zoomIn,
    '-': zoomOut, '_': zoomOut,
  }
  if (map[e.key]) { map[e.key](); e.preventDefault() }
}

// 场景模式
function applySceneMode() {
  if (!viewer) return
  if (sceneMode.value === 2) viewer.scene.mode = Cesium.SceneMode.SCENE2D
  else if (sceneMode.value === 1) viewer.scene.mode = Cesium.SceneMode.COLUMBUS_VIEW
  else viewer.scene.mode = Cesium.SceneMode.SCENE3D
  viewer.scene.screenSpaceCameraController.enableTilt = sceneMode.value !== 2
}
function changeSceneMode() { applySceneMode() }

// 底图切换
function switchBaseLayer() {
  if (!viewer) return
  viewer.imageryLayers.removeAll()
  try {
    if (baseLayer.value === 'gaode') viewer.imageryLayers.addImageryProvider(new (GaodeImageryProvider as any)())
    else if (baseLayer.value === 'satellite') viewer.imageryLayers.addImageryProvider(new (GaodeImageryProvider as any)({ style: 6 }))
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

// ---- 事件总线订阅：地图聚焦 ----
function handleMapFocus(p: any) {
  if (!viewer) return
  if (p.lat != null && p.lng != null) {
    viewer.camera.flyTo({
      destination: Cesium.Cartesian3.fromDegrees(p.lng, p.lat, p.height || 1500),
      orientation: { heading: 0, pitch: -Math.PI / 2.2, roll: 0 },
      duration: 1,
    })
    if (p.name) ElMessage.success(`已定位：${p.name}`)
    // 若按设备聚焦，打开信息卡
    if (p.hw_id != null) {
      const dev = devices.value.find((d) => d.hw_id === p.hw_id)
      if (dev) openInfo(dev)
    }
  }
}

async function load() {
  try {
    const res = await getAllDevices(1000)
    devices.value = res.data?.list || []
    await Promise.all([loadFaults(), loadCrossings()])
    plotDevices()
    plotCrossings()
    focusWithRetry(0)
  } catch { /* 忽略 */ }
}

// 加载活跃故障，建立 device_hw_id → 故障数 映射
async function loadFaults() {
  try {
    const res = await getFaults({ status: 'active', page_size: 500 })
    const list = res.data?.list || []
    const map: Record<number, number> = {}
    for (const f of list) {
      // device_hw_id 可能为空/非数字，过滤无效后作为数值索引（修复 TS2538）
      const hw = Number(f.device_hw_id)
      if (!Number.isFinite(hw)) continue
      map[hw] = (map[hw] || 0) + 1
    }
    faultByDev.value = map
  } catch { faultByDev.value = {} }
}

function focusWithRetry(attempt: number) {
  if (!viewer) return
  if (attempt > 6) return
  setTimeout(() => {
    autoFocusUserOrDevices()
    const c = viewer?.camera.positionCartographic
    const home = c != null && Math.abs((c.longitude as number) * 180 / Math.PI) < 0.5 && Math.abs((c.latitude as number) * 180 / Math.PI) < 0.5 && (c.height as number) > 10000000
    if (home) focusWithRetry(attempt + 1)
  }, attempt === 0 ? 400 : attempt * 600)
}

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

// 应用待处理聚焦（从路口管理"去地图"跳过来时）
function applyPendingFocus() {
  const pending = consumePendingFocus()
  if (pending) {
    // 等 Cesium 就绪后聚焦
    const tryApply = (n: number) => {
      if (viewer) { handleMapFocus(pending) }
      else if (n < 20) setTimeout(() => tryApply(n + 1), 200)
    }
    tryApply(0)
  }
}

onMounted(async () => {
  await nextTick()
  initCesium()
  try {
    const ui = await getUserInfo()
    const u = ui.data?.user
    if (u && u.center_lat != null && u.center_lng != null) {
      userCenterRef.value = { lat: u.center_lat, lng: u.center_lng }
    }
  } catch { /* 忽略 */ }
  await load()
  // 订阅事件总线（点击设备/实时聚焦）
  bus.on('map:focus', handleMapFocus)
  applyPendingFocus()
  resizeHandler = () => { try { viewer?.resize() } catch { /* 忽略 */ } }
  window.addEventListener('resize', resizeHandler)
  window.addEventListener('keydown', onKeydown)
  setTimeout(resizeHandler, 300)
  setTimeout(resizeHandler, 800)
})
onUnmounted(() => {
  if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
  window.removeEventListener('resize', resizeHandler)
  window.removeEventListener('keydown', onKeydown)
  bus.off('map:focus', handleMapFocus)
  if (viewer) { viewer.destroy(); viewer = null }
})
</script>

<style scoped>
.map-screen { width: 100%; height: 100%; display: flex; flex-direction: column; }
.map-toolbar {
  display: flex; align-items: center; gap: 14px; padding: 6px 10px;
  background: #fff; border-radius: 4px; margin-bottom: 6px; flex-wrap: wrap;
}
.tb-group { display: flex; gap: 10px; align-items: center; }
.layer-group .layer-label { font-size: 13px; color: #909399; }
.tb-stats { display: flex; gap: 14px; font-size: 13px; color: #606266; flex: 1; }
.tb-stats .s b { font-size: 15px; }
.s.on b { color: #67C23A; }
.s.off b { color: #F56C6C; }
.s.warn b { color: #E6A23C; }
.s.fault b { color: #F56C6C; }
.tb-actions { display: flex; gap: 8px; align-items: center; }

.map-wrap { position: relative; flex: 1; min-height: 0; border-radius: 6px; overflow: hidden; }
.cesium-viewer { width: 100%; height: 100%; }
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

/* 快速定位面板（悬浮） */
.map-zoom {
  position: absolute; right: 12px; bottom: 210px; width: 190px; z-index: 21;
  background: #fff; border-radius: 6px; box-shadow: 0 2px 12px rgba(0,21,41,0.15); overflow: hidden;
}
.zoom-head { display: flex; justify-content: space-between; align-items: center; padding: 8px 10px; background: #f5f7fa; font-size: 13px; font-weight: 600; }
.zoom-close { cursor: pointer; color: #909399; font-size: 16px; }
.zoom-item { display: flex; align-items: center; gap: 8px; padding: 7px 10px; cursor: pointer; border-bottom: 1px solid #f2f2f2; font-size: 13px; }
.zoom-item:hover { background: #f0f7ff; }
.zoom-item b { width: 16px; height: 16px; border-radius: 3px; background: #409eff; color: #fff; font-size: 11px; display: flex; align-items: center; justify-content: center; }
.zoom-item span { flex: 1; }
.zoom-item i { font-style: normal; color: #909399; font-size: 12px; }
.zoom-custom { padding: 8px 10px; display: flex; gap: 6px; align-items: center; border-bottom: 1px solid #f2f2f2; }
.zoom-tip { padding: 6px 10px; color: #909399; font-size: 11px; }

/* 左侧设备列表 */
.dev-panel {
  position: absolute; left: 10px; top: 10px; bottom: 10px; width: 220px; z-index: 15;
  background: rgba(255,255,255,0.96); border-radius: 6px;
  box-shadow: 0 2px 12px rgba(0,21,41,0.12); display: flex; flex-direction: column;
}
.panel-head { font-weight: 600; padding: 10px 12px; border-bottom: 1px solid #eee; font-size: 14px; display: flex; align-items: center; justify-content: space-between; }
.dev-list { flex: 1; overflow-y: auto; padding: 0 8px 8px; }
.dev-item { display: flex; align-items: center; gap: 6px; padding: 6px 6px; cursor: pointer; border-radius: 4px; font-size: 13px; }
.dev-item:hover { background: #f0f7ff; }
.dev-item.active { background: #e8f3ff; }
.dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.dot.on { background: #67C23A; }
.dot.off { background: #F56C6C; }
.nm { flex: 1; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.star { color: #FFB800; font-weight: bold; flex-shrink: 0; }
.star.warn { color: #F56C6C; }
.loc { color: #909399; font-size: 12px; flex-shrink: 0; }
.nl { color: #c0c4cc; }

/* 右侧设备信息卡 */
.info-card {
  position: absolute; right: 12px; top: 12px; width: 260px; z-index: 15;
  background: #fff; border-radius: 6px; box-shadow: 0 2px 12px rgba(0,21,41,0.15);
}
.info-head { display: flex; justify-content: space-between; align-items: center; padding: 10px 12px; border-bottom: 1px solid #eee; font-weight: 600; }
.info-head-ops { display: flex; align-items: center; gap: 2px; }
.info-body { padding: 10px 12px; }
.kv { display: flex; justify-content: space-between; align-items: center; padding: 5px 0; font-size: 13px; color: #606266; }
.kv b { color: #303133; }
.kv .muted { color: #c0c4cc; }
.info-actions { border-top: 1px solid #eee; padding: 8px 12px; display: flex; gap: 6px; flex-wrap: wrap; }
</style>
