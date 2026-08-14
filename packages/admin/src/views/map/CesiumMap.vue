<template>
  <div class="cesium-map-page">
    <!-- 顶栏：模式切换 + 统计 -->
    <div class="map-toolbar">
      <div class="mode-group">
        <el-radio-group v-model="sceneMode" size="small" @change="changeSceneMode">
          <el-radio-button :value="2">2D 地图</el-radio-button>
          <el-radio-button :value="3">3D 球</el-radio-button>
          <el-radio-button :value="1">哥伦布视图</el-radio-button>
        </el-radio-group>
      </div>
      <div class="stats">
        <span class="st">设备 <b>{{ devices.length }}</b></span>
        <span class="st online">在线 <b>{{ onlineCount }}</b></span>
        <span class="st offline">离线 <b>{{ offlineCount }}</b></span>
        <span class="st fault">故障 <b>{{ faultCount }}</b></span>
        <span class="st">已定位 <b>{{ mappedCount }}</b></span>
      </div>
      <div class="actions">
        <el-button size="small" @click="flyToAll">全览</el-button>
        <el-button size="small" type="primary" @click="refreshDevices">刷新</el-button>
      </div>
    </div>

    <!-- Cesium 容器 -->
    <div class="cesium-container">
      <div ref="cesiumRef" class="cesium-viewer"></div>
    </div>

    <!-- 左侧：设备列表定位 -->
    <div class="side-panel">
      <div class="panel-title">设备定位</div>
      <el-input v-model="searchKw" placeholder="搜索路口/设备ID" size="small" clearable />
      <div class="dev-list">
        <div v-for="d in filteredDevices" :key="d.hw_id" class="dev-item" @click="focusDevice(d); openDrill(d)">
          <span class="dot" :class="d.online_status ? 'on' : 'off'"></span>
          <span class="nm">{{ d.intersection || '#' + d.hw_id }}</span>
          <span class="loc" v-if="d.lat != null && d.lng != null">{{ d.lat.toFixed(3) }},{{ d.lng.toFixed(3) }}</span>
          <span class="loc nl" v-else>未定位</span>
        </div>
        <el-empty v-if="filteredDevices.length === 0" description="暂无设备" :image-size="60" />
      </div>
    </div>

    <!-- 设备联动下钻面板：点击点位后展示 -->
    <div v-if="drill.visible" class="drill-panel">
      <div class="drill-head">
        <span class="drill-title">{{ drill.intersection || ('设备#' + drill.hw_id) }}</span>
        <el-button size="small" circle @click="closeDrill">×</el-button>
      </div>
      <div class="drill-body">
        <el-tabs v-model="drill.tab">
          <el-tab-pane label="派单参考" name="dispatch">
            <div class="sec">
              <div class="sec-title">活跃故障（{{ drill.ref.faults.length }}）</div>
              <div v-for="f in drill.ref.faults" :key="f.id" class="row">
                <el-tag size="small" :type="f.fault_level === 'critical' ? 'danger' : 'warning'">{{ errLabel(f.err_code) }}</el-tag>
                <span class="row-sub">{{ f.last_seen?.slice(0,16) }}</span>
              </div>
              <div v-if="drill.ref.faults.length === 0" class="empty">暂无活跃故障</div>
            </div>
            <div class="sec">
              <div class="sec-title">进行中工单（{{ drill.ref.work_orders.length }}）</div>
              <div v-for="w in drill.ref.work_orders" :key="w.id" class="row">
                <span class="row-main">{{ w.order_no }}</span>
                <el-tag size="small">{{ woStatus(w.status) }}</el-tag>
              </div>
              <div v-if="drill.ref.work_orders.length === 0" class="empty">暂无进行中工单</div>
            </div>
          </el-tab-pane>

          <el-tab-pane label="维修耗材" name="material">
            <div v-for="m in drill.ref.materials" :key="m.id" class="m-row">
              <span class="m-name">{{ m.name }}</span>
              <span class="m-part">{{ m.part_no || '-' }}</span>
              <el-tag size="small" :type="m.quantity <= m.threshold ? 'danger' : 'success'">库存 {{ m.quantity }} {{ m.unit || '' }}</el-tag>
            </div>
            <div v-if="drill.ref.materials.length === 0" class="empty">暂无耗材记录</div>
          </el-tab-pane>

          <el-tab-pane label="监控/视频" name="media">
            <div v-for="m in drill.ref.media" :key="m.id" class="media-row" @click="playDevMedia(m)">
              <span class="m-name">{{ m.title || (m.media_type === 'monitoring' ? '监控' : m.media_type) }}</span>
              <el-tag size="small">{{ m.source }}</el-tag>
            </div>
            <div v-if="drill.ref.media.length === 0" class="empty">暂无媒体记录</div>
          </el-tab-pane>
        </el-tabs>
      </div>
      <div class="drill-foot">
        <el-button size="small" @click="goPanel('video')">打开视频与监控</el-button>
        <el-button size="small" @click="goPanel('feedback')">打开问题反馈</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import * as Cesium from 'cesium'
import { getAllDevices } from '@/api/map'
import { getDispatchReference } from '@/api/dispatch'

// 组件 emit：通知父级切换标签页（video/feedback）
const emit = defineEmits<{ (e: 'goPanel', name: string): void }>()

// Cesium 静态资源（vite 构建时拷贝到 /cesium，见 vite.config.js）
;(Cesium as any).buildModuleUrl.setBaseUrl('/tsloms/admin/cesium/')

interface Dev { id?: number; hw_id?: number; intersection?: string; lat: number | null; lng: number | null; online_status?: boolean }

const cesiumRef = ref<HTMLElement>()
let viewer: Cesium.Viewer | null = null

const sceneMode = ref(3)
const searchKw = ref('')
const devices = ref<Dev[]>([])

const onlineCount = computed(() => devices.value.filter((d) => d.online_status).length)
const offlineCount = computed(() => devices.value.filter((d) => !d.online_status).length)
const mappedCount = computed(() => devices.value.filter((d) => d.lat != null && d.lng != null).length)
const faultCount = computed(() => 0)
const filteredDevices = computed(() => {
  const kw = searchKw.value.trim()
  if (!kw) return devices.value
  return devices.value.filter((d) => (d.intersection || '').includes(kw) || String(d.hw_id).includes(kw))
})

// 设备联动下钻：选中设备的聚合信息（反馈参考）
const drill = ref({
  visible: false,
  hw_id: 0 as number,
  intersection: '' as string,
  tab: 'dispatch' as string,
  ref: {
    faults: [] as any[], work_orders: [] as any[], materials: [] as any[], media: [] as any[],
  },
})

// 故障码中文名
const errLabel = (code: number) => {
  const map: Record<number, string> = {
    [0]: '正常',
    [-1]: '红灯周期全灭', [-2]: '黄灯周期全灭', [-3]: '绿灯周期全灭',
    [-4]: '红黄同亮', [-5]: '红绿同亮', [-6]: '黄绿同亮', [-7]: '红黄绿同亮',
    [-8]: '红灯超时', [-9]: '黄灯超时', [-10]: '绿灯超时',
    [-11]: '红灯缺亮', [-12]: '黄灯缺亮', [-13]: '绿灯缺亮', [-14]: '断电',
  }
  return map[code] ?? '未知'
}
const woStatus = (s: string) => ({ pending: '待处理', processing: '处理中', completed: '已完成', rejected: '已驳回' } as Record<string, string>)[s] || s

// 点击设备点位 → 拉取派单参考并打开下钻面板
function openDrill(d: Dev) {
  drill.value.visible = true
  drill.value.hw_id = d.hw_id || 0
  drill.value.intersection = d.intersection || '设备#' + d.hw_id
  drill.value.tab = 'dispatch'
  // 拉取聚合数据
  getDispatchReference(d.hw_id || 0).then((res) => {
    const ref = res.data || {}
    drill.value.ref = {
      faults: ref.faults || [], work_orders: ref.work_orders || [],
      materials: ref.materials || [], media: ref.media || [],
    }
  }).catch(() => { ElMessage.error('下钻数据加载失败') })
}
function closeDrill() { drill.value.visible = false }
function goPanel(name: string) { emit('goPanel', name) }
function playDevMedia(m: any) {
  // 有可播放 url 时提示，否则跳转视频面板
  if (m.url && !/^rtsp?:\/\//i.test(m.url)) {
    window.open(m.url.startsWith('/media/') ? '/tsloms' + m.url : m.url, '_blank')
  } else {
    emit('goPanel', 'video')
  }
}

// 初始化 Cesium 视图
function initCesium() {
  if (!cesiumRef.value || viewer) return
  // 去掉默认的 Cesium Ion token（离线使用默认地球）
  Cesium.Ion.defaultAccessToken = ''
  viewer = new Cesium.Viewer(cesiumRef.value, {
    baseLayerPicker: false,
    geocoder: false,
    homeButton: false,
    sceneModePicker: true,
    navigationHelpButton: false,
    animation: false,
    timeline: false,
    fullscreenButton: false,
  })
  // 使用真实影像底图（OpenStreetMap，无需 token）
  try {
    viewer.scene.imageryLayers.removeAll()
    viewer.imageryLayers.addImageryProvider(
      new Cesium.OpenStreetMapImageryProvider({ url: 'https://tile.openstreetmap.org/' })
    )
  } catch {
    /* 底图加载失败则保留默认 */
  }
  // 默认视角：中国东部
  viewer.camera.setView({ destination: Cesium.Cartesian3.fromDegrees(105, 35, 6000000) })
  applySceneMode()

  // 点击设备点位 → 联动下钻
  const handler = new Cesium.ScreenSpaceEventHandler(viewer.scene.canvas)
  handler.setInputAction((evt: any) => {
    const picked = viewer?.scene.pick(evt.position)
    if (picked && picked.id && (picked.id as any).id && String((picked.id as any).id).startsWith('dev-')) {
      const props = (picked.id as any).properties
      const hw = props && props.hw_id !== undefined ? props.hw_id.getValue() : undefined
      const dev = devices.value.find((d) => d.hw_id === hw)
      if (dev) openDrill(dev)
    }
  }, Cesium.ScreenSpaceEventType.LEFT_CLICK)
}

// 应用 2D/3D/哥伦布 模式
function applySceneMode() {
  if (!viewer) return
  const src = viewer.scene
  if (sceneMode.value === 2) src.morphTo2D(2.0)
  else if (sceneMode.value === 1) src.morphToColumbusView(2.0)
  else src.morphTo3D(2.0)
}
function changeSceneMode() { applySceneMode() }

// 设备点位（把设备按经纬度投影到地图）
function plotDevices() {
  if (!viewer) return
  // 清除旧点位
  viewer.entities.removeAll()
  for (const d of devices.value) {
    if (d.lat != null && d.lng != null) {
      const color = d.online_status ? Cesium.Color.GREEN : Cesium.Color.RED
      viewer.entities.add({
        id: 'dev-' + d.hw_id,
        position: Cesium.Cartesian3.fromDegrees(d.lng!, d.lat!),
        point: { pixelSize: 12, color, outlineColor: Cesium.Color.WHITE, outlineWidth: 2 },
        label: {
          text: d.intersection || ('#' + d.hw_id),
          font: 'bold 13px "Microsoft YaHei","PingFang SC","Noto Sans CJK SC",sans-serif',
          pixelOffset: new Cesium.Cartesian2(0, -18),
          fillColor: Cesium.Color.WHITE,
          outlineColor: Cesium.Color.BLACK,
          outlineWidth: 2,
          showBackground: true,
          backgroundColor: Cesium.Color.fromCssColorString('rgba(0,21,41,0.7)'),
          backgroundPadding: new Cesium.Cartesian2(6, 4),
          disableDepthTestDistance: Number.POSITIVE_INFINITY,
        },
        properties: d as any,
      })
    }
  }
}

// 定位到某设备
function focusDevice(d: Dev) {
  if (!viewer || d.lat == null || d.lng == null) { return }
  viewer.camera.flyTo({
    destination: Cesium.Cartesian3.fromDegrees(d.lng!, d.lat!, 2000),
  })
}

// 全览所有设备
function flyToAll() {
  const mapped = devices.value.filter((d) => d.lat != null && d.lng != null)
  if (mapped.length && viewer) {
    const positions = mapped.map((d) => Cesium.Cartesian3.fromDegrees(d.lng!, d.lat!))
    viewer.camera.flyToBoundingSphere(Cesium.BoundingSphere.fromPoints(positions), { duration: 1 })
  }
}

async function refreshDevices() {
  try {
    const res = await getAllDevices(1000)
    devices.value = res.data?.list || []
    plotDevices()
  } catch { /* 忽略 */ }
}

let resizeHandler: () => void = () => {}
onMounted(async () => {
  await nextTick()
  initCesium()
  await refreshDevices()
  resizeHandler = () => viewer?.resize()
  window.addEventListener('resize', resizeHandler)
})

onUnmounted(() => {
  window.removeEventListener('resize', resizeHandler)
  if (viewer) { viewer.destroy(); viewer = null }
})
</script>

<style scoped>
.cesium-map-page { position: relative; height: calc(100vh - 100px); min-height: 560px; }
.map-toolbar { display: flex; align-items: center; gap: 16px; padding: 8px 12px; background: #fff; border-radius: 4px; margin-bottom: 8px; flex-wrap: wrap; }
.stats { display: flex; gap: 12px; font-size: 13px; color: #606266; flex: 1; }
.st b { font-size: 15px; color: #303133; }
.st.online b { color: #67C23A; }
.st.offline b { color: #F56C6C; }
.st.fault b { color: #E6A23C; }
.actions { display: flex; gap: 8px; }
.cesium-container { position: absolute; left: 0; right: 0; top: 44px; bottom: 0; border-radius: 6px; overflow: hidden; border: 1px solid #e0e0e0; }
.cesium-viewer { width: 100%; height: 100%; }
.side-panel { position: absolute; left: 12px; top: 56px; width: 240px; max-height: 60%; background: rgba(255,255,255,0.95); border-radius: 6px; box-shadow: 0 2px 12px rgba(0,21,41,0.15); display: flex; flex-direction: column; z-index: 10; }
.panel-title { font-weight: 600; padding: 10px 12px; border-bottom: 1px solid #eee; font-size: 14px; }
.side-panel .el-input { margin: 8px 12px; width: auto; }
.dev-list { overflow-y: auto; flex: 1; padding: 0 8px 8px; }
.dev-item { display: flex; align-items: center; gap: 6px; padding: 6px 4px; cursor: pointer; border-radius: 4px; font-size: 13px; }
.dev-item:hover { background: #f0f7ff; }
.dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.dot.on { background: #67C23A; }
.dot.off { background: #F56C6C; }
.nm { flex: 1; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.loc { color: #909399; font-size: 12px; flex-shrink: 0; }
.nl { color: #c0c4cc; }

/* 联动下钻面板 */
.drill-panel {
  position: absolute; right: 12px; top: 56px; width: 320px; max-height: 72%;
  background: #fff; border-radius: 6px; box-shadow: 0 2px 12px rgba(0,21,41,0.15);
  display: flex; flex-direction: column; z-index: 10;
}
.drill-head { display: flex; justify-content: space-between; align-items: center; padding: 10px 12px; border-bottom: 1px solid #eee; }
.drill-title { font-weight: 600; font-size: 14px; }
.drill-body { flex: 1; overflow-y: auto; padding: 0 12px 8px; }
.drill-foot { border-top: 1px solid #eee; padding: 8px 12px; display: flex; gap: 8px; }
.sec { margin: 8px 0; }
.sec-title { font-size: 12px; color: #606266; margin-bottom: 6px; }
.row { display: flex; align-items: center; gap: 8px; padding: 4px 0; }
.row-main { font-size: 13px; flex: 1; }
.row-sub { font-size: 12px; color: #909399; }
.m-row { display: flex; align-items: center; gap: 8px; padding: 6px 0; border-bottom: 1px dashed #f0f0f0; }
.m-name { font-size: 13px; flex: 1; }
.m-part { font-size: 12px; color: #909399; }
.media-row { display: flex; align-items: center; gap: 8px; padding: 6px 0; cursor: pointer; }
.media-row:hover { background: #f0f7ff; }
.empty { color: #c0c4cc; font-size: 13px; padding: 10px 0; }
</style>
