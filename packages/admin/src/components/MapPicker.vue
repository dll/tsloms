<template>
  <el-dialog :model-value="modelValue" :title="'地图选点' + (title ? ' · ' + title : '')" width="820px" append-to-body :destroy-on-close="true" @close="onClose" @open="initMap">
    <div class="mp-row">
      <el-input v-model="kw" placeholder="搜索地名/路口（可输入坐标 经度,纬度），或直接在地图上点击定位；鼠标拖动/滚轮缩放" clearable @keyup.enter="searchPlace" style="flex:1">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <el-button @click="searchPlace">搜索</el-button>
      <el-button @click="resetToInitial">复位</el-button>
    </div>

    <div class="mp-search-list" v-if="searchResults.length">
      <div v-for="r in searchResults" :key="r.id" class="mp-search-item" @click="flyTo(r.lng, r.lat)">
        <span>{{ r.name }}</span><span class="mp-coord">{{ fmt(r.lat) }}, {{ fmt(r.lng) }}</span>
      </div>
    </div>

    <div ref="mapContainer" class="mp-map"></div>

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
import { ref, onMounted, onUnmounted } from 'vue'
import { Search } from '@element-plus/icons-vue'
import * as Cesium from 'cesium'
import { getCrossings } from '@/api/warning'      // 复用 crossings 接口
import { getAreasTree } from '@/api/warning'       // 复用 areas 接口

const props = defineProps<{ modelValue: boolean; title?: string; initialLat?: number | null; initialLng?: number | null }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void; (e: 'pick', lat: number, lng: number): void }>()

;(Cesium as any).buildModuleUrl.setBaseUrl('/tsloms/admin/cesium/')

const mapContainer = ref<HTMLElement>()
let viewer: Cesium.Viewer | null = null
let marker: Cesium.Entity | null = null
let initCoord = { lat: 0, lng: 0 }

const kw = ref('')
const coordLat = ref('')
const coordLng = ref('')
const searchResults = ref<{ id: number; name: string; lat: number; lng: number }[]>([])

function fmt(v: any) { return v == null ? '' : Number(v).toFixed(6) }

function initMap() {
  const el = mapContainer.value
  if (!el) return
  // 初始：来自 props 或 默认全国（合肥约 31.8,117.2）
  const ilat = props.initialLat ?? 31.8
  const ilng = props.initialLng ?? 117.22
  initCoord = { lat: ilat, lng: ilng }
  coordLat.value = fmt(ilat); coordLng.value = fmt(ilng)

  if (!viewer) {
    viewer = new Cesium.Viewer(el, {
      imageryProvider: new Cesium.OpenStreetMapImageryProvider({ url: 'https://tile.openstreetmap.org/' }),
      baseLayerPicker: false, geocoder: false, homeButton: false, sceneModePicker: false,
      navigationHelpButton: false, animation: false, timeline: false, fullscreenButton: false,
      infoBox: false, selectionIndicator: false,
    } as any)
    // 点击地图选点
    const handler = new Cesium.ScreenSpaceEventHandler(viewer.scene.canvas)
    handler.setInputAction((e: any) => {
      const cart = viewer?.camera.pickEllipsoid(e.position, viewer?.scene.globe.ellipsoid)
      if (cart) {
        const carto = Cesium.Cartographic.fromCartesian(cart)
        const lat = Cesium.Math.toDegrees(carto.latitude)
        const lng = Cesium.Math.toDegrees(carto.longitude)
        placeMarker(lat, lng)
      }
    }, Cesium.ScreenSpaceEventType.LEFT_CLICK)
    // 默认支持左键拖动平移 + 滚轮缩放（不覆盖默认 zoomEventTypes）
  }
  // 定位初始
  viewer.camera.setView({ destination: Cesium.Cartesian3.fromDegrees(initCoord.lng, initCoord.lat, 40000) })
  placeMarker(initCoord.lat, initCoord.lng)
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

// 地名搜索：匹配 路口(crossings) 与 区划(areas) 名称/道路
async function searchPlace() {
  searchResults.value = []
  const k = kw.value.trim().toLowerCase()
  try {
    const [cr, ar] = await Promise.all([getCrossings({ page_size: 200 }), getAreasTree()])
    const flat = (list: any[], out: { id: number; name: string; lat: number; lng: number }[]) => {
      if (!list) return out
      for (const it of list) {
        if (k === '' || String(it.name || '').toLowerCase().includes(k) || String(it.full_name || '').toLowerCase().includes(k)) {
          if (it.lat != null && it.lng != null) out.push({ id: it.id, name: it.full_name || it.name, lat: Number(it.lat), lng: Number(it.lng) })
        }
        flat(it.children, out)
      }
      return out
    }
    const crossings = (cr.data?.list || []).map((c: any) => ({ id: c.id, name: c.name, lat: c.lat, lng: c.lng }))
    const areas = flat((ar.data?.tree || ar.data?.list || []), [])
    searchResults.value = [...crossings, ...areas].filter((x) => x.lat != null && x.lng != null).slice(0, 30)
  } catch {
    searchResults.value = []
  }
}

function confirmPick() {
  const lat = parseFloat(coordLat.value), lng = parseFloat(coordLng.value)
  if (!isFinite(lat) || !isFinite(lng)) return
  emit('pick', lat, lng)
  emit('update:modelValue', false)
}

function onClose() {
  emit('update:modelValue', false)
  if (viewer) { viewer.destroy(); viewer = null; marker = null }
}

onUnmounted(() => { if (viewer) { viewer.destroy(); viewer = null; marker = null } })
</script>

<style scoped>
.mp-row { display: flex; gap: 8px; margin-bottom: 8px; align-items: center; }
.mp-map { width: 100%; height: 420px; border: 1px solid #dcdfe6; border-radius: 6px; }
.mp-search-list { max-height: 120px; overflow: auto; border: 1px solid #ebeef5; border-radius: 4px; margin-bottom: 6px; }
.mp-search-item { display: flex; justify-content: space-between; padding: 4px 10px; cursor: pointer; font-size: 13px; }
.mp-search-item:hover { background: #f0f2f5; }
.mp-coord { color: #909399; }
.mp-coord-bar { display: flex; gap: 8px; align-items: center; margin-top: 8px; }
.mp-tip { font-size: 12px; color: #909399; margin-left: 8px; }
</style>
