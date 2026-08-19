<template>
  <el-dialog :model-value="modelValue" :title="'地图选点' + (title ? ' · ' + title : '')" width="820px" append-to-body :destroy-on-close="true" @close="onClose" @opened="initMap">
    <div class="mp-row">
      <el-input v-model="kw" placeholder="搜索地名/路口（可输入坐标 经度,纬度），或直接在地图上点击定位；鼠标拖动/滚轮缩放" clearable @keyup.enter="searchPlace" style="flex:1">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <el-button @click="searchPlace">搜索</el-button>
      <el-button @click="resetToInitial">复位</el-button>
    </div>

    <div class="mp-search-list" v-if="searchResults.length">
      <div v-for="r in searchResults" :key="r.name + r.lat" class="mp-search-item" @click="flyTo(r.lng, r.lat)">
        <span class="mp-search-name">{{ r.name }}<el-tag v-if="r.src === 'amap'" size="small" type="warning" effect="plain">POI</el-tag></span>
        <span class="mp-coord">{{ r.address ? r.address + ' · ' : '' }}{{ fmt(r.lat) }}, {{ fmt(r.lng) }}</span>
      </div>
    </div>

    <div class="mp-map-shell">
      <div ref="mapContainer" class="mp-map"></div>
      <div v-if="mapLoading" class="mp-map-state"><el-icon class="is-loading"><Loading /></el-icon><span>地图加载中…</span></div>
      <div v-else-if="mapError" class="mp-map-state mp-map-error">
        <el-icon><WarningFilled /></el-icon><span>{{ mapError }}</span><el-button size="small" type="primary" @click="initMap">重新加载</el-button>
      </div>
    </div>

    <div class="mp-coord-bar">
      <span>经度</span><el-input v-model="coordLng" size="small" style="width:130px" />
      <span>纬度</span><el-input v-model="coordLat" size="small" style="width:130px" />
      <el-button size="small" @click="applyCoord">定位该坐标</el-button>
      <span class="mp-tip">绿点=选定位置；点击地图任意位置更新</span>
    </div>

    <template #footer>
      <el-button @click="onClose">取消</el-button>
      <el-button type="primary" @click="confirmPick">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, onUnmounted, nextTick } from 'vue'
import { Search, Loading, WarningFilled } from '@element-plus/icons-vue'
import * as Cesium from 'cesium'
import { getCrossings } from '@/api/warning'      // 复用 crossings 接口
import { getAreasTree } from '@/api/warning'       // 复用 areas 接口
import { getAllDevices } from '@/api/map'          // 复用设备（含路口名）
import GaodeImageryProvider from '@/views/map/GaodeImagery.js'

const props = defineProps<{ modelValue: boolean; title?: string; initialLat?: number | null; initialLng?: number | null }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void; (e: 'pick', lat: number, lng: number): void }>()

;(Cesium as any).buildModuleUrl.setBaseUrl('/tsloms/admin/cesium/')

const mapContainer = ref<HTMLElement>()
let viewer: Cesium.Viewer | null = null
let marker: Cesium.Entity | null = null
let inputHandler: Cesium.ScreenSpaceEventHandler | null = null
let resizeObserver: ResizeObserver | null = null
let initCoord = { lat: 0, lng: 0 }
const mapLoading = ref(false)
const mapError = ref('')

const kw = ref('')
const coordLat = ref('')
const coordLng = ref('')
const searchResults = ref<{ id: number; name: string; lat: number; lng: number; address?: string; src?: string }[]>([])

function fmt(v: any) { return v == null ? '' : Number(v).toFixed(6) }

async function initMap() {
  await nextTick()
  const el = mapContainer.value
  if (!el || !el.clientWidth || !el.clientHeight) {
    mapError.value = '地图容器尚未准备完成，请稍后重试'
    return
  }
  mapLoading.value = true
  mapError.value = ''
  // 初始：来自 props 或 默认全国（合肥约 31.8,117.2）
  const ilat = props.initialLat ?? 31.8
  const ilng = props.initialLng ?? 117.22
  initCoord = { lat: ilat, lng: ilng }
  coordLat.value = fmt(ilat); coordLng.value = fmt(ilng)

  try {
  if (!viewer) {
    // 默认高德卫星瓦片（高德路网 style=8 已被上游降级为 1x1 占位图不可用，卫星 style=6 可用，故默认影像）
    viewer = new Cesium.Viewer(el, {
      baseLayer: false,
      baseLayerPicker: false, geocoder: false, homeButton: false, sceneModePicker: false,
      navigationHelpButton: false, animation: false, timeline: false, fullscreenButton: false,
      infoBox: false, selectionIndicator: false,
    } as any)
    // Cesium 1.144 不再稳定兼容 Viewer.imageryProvider 构造参数，创建后显式加入同源图层。
    viewer.imageryLayers.removeAll()
    viewer.imageryLayers.addImageryProvider(new (GaodeImageryProvider as any)({ style: 6 }))
    // 点击地图选点
    inputHandler = new Cesium.ScreenSpaceEventHandler(viewer.scene.canvas)
    inputHandler.setInputAction((e: any) => {
      const cart = viewer?.camera.pickEllipsoid(e.position, viewer?.scene.globe.ellipsoid)
      if (cart) {
        const carto = Cesium.Cartographic.fromCartesian(cart)
        const lat = Cesium.Math.toDegrees(carto.latitude)
        const lng = Cesium.Math.toDegrees(carto.longitude)
        placeMarker(lat, lng)
      }
    }, Cesium.ScreenSpaceEventType.LEFT_CLICK)
    resizeObserver = new ResizeObserver(() => viewer?.resize())
    resizeObserver.observe(el)
  }
  // 定位初始
  viewer.camera.setView({ destination: Cesium.Cartesian3.fromDegrees(initCoord.lng, initCoord.lat, 40000) })
  placeMarker(initCoord.lat, initCoord.lng)
  viewer.resize()
  } catch (error) {
    mapError.value = `地图加载失败：${error instanceof Error ? error.message : '请检查网络或服务状态'}`
    if (viewer) { viewer.destroy(); viewer = null }
  } finally {
    mapLoading.value = false
  }
}

function placeMarker(lat: number, lng: number) {
  if (!viewer) return
  if (marker) viewer.entities.remove(marker)
  marker = viewer.entities.add({
    position: Cesium.Cartesian3.fromDegrees(lng, lat),
    point: { pixelSize: 14, color: Cesium.Color.LIME, outlineColor: Cesium.Color.BLACK, outlineWidth: 2, disableDepthTestDistance: Number.POSITIVE_INFINITY },
  })
  coordLat.value = fmt(lat); coordLng.value = fmt(lng)
}

function flyTo(lng: number, lat: number, height = 6000) {
  if (!viewer) return
  viewer.camera.flyTo({ destination: Cesium.Cartesian3.fromDegrees(lng, lat, height), duration: 0.7 })
  placeMarker(lat, lng)
}

function resetToInitial() { if (viewer) viewer.camera.setView({ destination: Cesium.Cartesian3.fromDegrees(initCoord.lng, initCoord.lat, 40000) }) }

function applyCoord() {
  const lng = parseFloat(coordLng.value), lat = parseFloat(coordLat.value)
  if (isFinite(lng) && isFinite(lat)) flyTo(lng, lat)
}

// 地名搜索：优先高德 POI（真实地名，需 AMAP_WEB_KEY），否则降级本地(设备/路口/区划)点位
async function searchPlace() {
  searchResults.value = []
  const k = kw.value.trim().toLowerCase()
  // 若输入的是坐标“经度,纬度”，直接定位
  const coordMatch = kw.value.match(/^\s*(-?\d+(\.\d+)?)\s*[,，\s]+\s*(-?\d+(\.\d+)?)\s*$/)
  if (coordMatch) {
    const lng = parseFloat(coordMatch[1]), lat = parseFloat(coordMatch[3])
    if (isFinite(lng) && isFinite(lat)) { flyTo(lng, lat); return }
  }
  // 1) 高德 POI（服务器端代理，带 key）
  try {
    const res: any = await import('@/utils/request').then((m) => m.default.get('/proxy/amap/place', { params: { kw: kw.value.trim() } }))
    const pois = res?.data?.list || []
    if (pois.length) {
      searchResults.value = pois.map((p: any) => ({ id: p.name, name: p.name, lat: p.lat, lng: p.lng, address: p.address, src: 'amap' }))
      if (k === '') return
    }
  } catch { /* 无 key 或失败 → 本地降级 */ }
  // 2) 本地点位（设备/路口/区划）作为补充/降级
  try {
    const [cr, ar, dev] = await Promise.all([getCrossings({ page_size: 500 }), getAreasTree(), getAllDevices(1000)])
    const out: { id: number; name: string; lat: number; lng: number; src?: string }[] = []
    const add = (name: string, lat: any, lng: any) => {
      if (lat == null || lng == null) return
      if (k !== '' && !String(name || '').toLowerCase().includes(k)) return
      // 去重（高德已返回同名时不重复）
      if (!searchResults.value.some((s) => s.name === String(name) && Math.abs(s.lat - Number(lat)) < 0.0001)) {
        out.push({ id: out.length, name: String(name || ''), lat: Number(lat), lng: Number(lng), src: 'local' })
      }
    }
    ;(dev.data?.list || []).forEach((d: any) => add(d.intersection || ('设备#' + d.hw_id), d.lat, d.lng))
    ;(cr.data?.list || []).forEach((c: any) => add(c.name, c.lat, c.lng))
    const flat = (list: any[]) => { if (!list) return; list.forEach((it: any) => { add(it.full_name || it.name, it.lat, it.lng); flat(it.children) }) }
    flat(ar.data?.tree || ar.data?.list || [])
    searchResults.value = [...searchResults.value, ...out]
  } catch { /* 忽略 */ }
}

function confirmPick() {
  const lat = parseFloat(coordLat.value), lng = parseFloat(coordLng.value)
  if (!isFinite(lat) || !isFinite(lng)) return
  emit('pick', lat, lng)
  emit('update:modelValue', false)
}

function onClose() {
  emit('update:modelValue', false)
  destroyMap()
}

function destroyMap() {
  if (resizeObserver) { resizeObserver.disconnect(); resizeObserver = null }
  if (inputHandler) { inputHandler.destroy(); inputHandler = null }
  if (viewer) { viewer.destroy(); viewer = null; marker = null }
  mapLoading.value = false
}

onUnmounted(destroyMap)
</script>

<style scoped>
.mp-row { display: flex; gap: 8px; margin-bottom: 8px; align-items: center; }
.mp-map-shell { position: relative; width: 100%; height: 420px; border: 1px solid #e5eaf2; border-radius: 10px; overflow: hidden; background: #eef3f8; }
.mp-map { width: 100%; height: 100%; }
.mp-map-state { position: absolute; inset: 0; z-index: 5; display: flex; align-items: center; justify-content: center; gap: 8px; color: #64748b; background: rgba(248, 250, 252, .88); font-size: 14px; }
.mp-map-error { flex-direction: column; color: #b42318; }
.mp-search-list { max-height: 120px; overflow: auto; border: 1px solid #ebeef5; border-radius: 4px; margin-bottom: 6px; }
.mp-search-item { display: flex; justify-content: space-between; padding: 4px 10px; cursor: pointer; font-size: 13px; }
.mp-search-item:hover { background: #f0f2f5; }
.mp-search-name { display: inline-flex; align-items: center; gap: 6px; }
.mp-search-name .el-tag { margin-left: 4px; }
.mp-coord { color: #909399; }
.mp-coord-bar { display: flex; gap: 8px; align-items: center; margin-top: 8px; }
.mp-tip { font-size: 12px; color: #909399; margin-left: 8px; }
</style>
