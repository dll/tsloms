<template>
  <div class="monitor-wall" :class="{ fullscreen: isFullscreen }">
    <!-- 顶部控制栏 -->
    <div class="wall-toolbar">
      <div class="left">
        <span class="title">📡 多路监控大屏</span>
        <el-select v-model="selDeviceIds" multiple filterable collapse-tags collapse-tags-tooltip
                   placeholder="选择设备（多路口）" style="width: 320px" :max-collapse-tags="2">
          <el-option v-for="d in deviceOptions" :key="d.hw_id"
                     :label="(d.intersection || '#'+d.hw_id) + ' (#'+d.hw_id+')'" :value="d.hw_id" />
        </el-select>
        <el-button type="primary" :icon="VideoPlay" @click="startPlay">开始监控</el-button>
        <el-button :icon="Refresh" @click="reloadAll">刷新</el-button>
      </div>
      <div class="right">
        <el-button-group>
          <el-button v-for="n in [1,4,9,16]" :key="n" :type="gridSize===n ? 'primary' : ''" size="small" @click="setGrid(n)">{{ n }}宫格</el-button>
        </el-button-group>
        <el-button size="small" :type="isFullscreen ? 'danger' : ''" @click="toggleFullscreen">
          {{ isFullscreen ? '退出全屏' : '全屏' }}
        </el-button>
      </div>
    </div>

    <!-- 宫格区域 -->
    <div class="wall-body" :class="'grid-' + gridSize" :ref="bodyRef">
      <div v-for="item in gridCells" :key="item.key" class="cell" :class="{ live: item.streaming }" @dblclick="toggleCellFullscreen(item)">
        <div class="cell-header">
          <span class="cell-name">{{ item.label }}</span>
          <span class="cell-status" :class="item.state">
            {{ item.state === 'live' ? '● 直播' : item.state === 'loading' ? '连接中...' : item.state === 'error' ? '播放失败' : '未配置源' }}
          </span>
          <span class="cell-extra">
            <el-button v-if="item.fullscreen" size="mini" text @click.stop="toggleCellFullscreen(item)">还原</el-button>
            <el-button v-else size="mini" text @click.stop="toggleCellFullscreen(item)" title="放大此路">⛶</el-button>
          </span>
        </div>
        <div class="cell-video">
          <!-- HLS(m3u8) 用 video 标签 + 转换？此处直接用浏览器可播地址 -->
          <video v-if="item.playable && item.url" :src="item.url" autoplay muted controls playsinline style="width:100%;height:100%;object-fit:contain" />
          <div v-else-if="item.state === 'loading'" class="cell-placeholder loading-box">
            <el-icon class="is-loading"><Loading /></el-icon>
            <span>连接 {{ item.label }}</span>
          </div>
          <div v-else class="cell-placeholder">
            <el-icon :size="28"><VideoCamera /></el-icon>
            <span>{{ placeholderText(item) }}</span>
            <span v-if="item.url && /^rtsp:/i.test(item.url)" class="tip">RTSP 需兼容地址(HLS/FLV)</span>
          </div>
        </div>
      </div>
      <el-empty v-if="gridCells.length===0 && started" description="请选择设备后开始监控" style="grid-column:1/-1" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { VideoPlay, Refresh, VideoCamera, Loading } from '@element-plus/icons-vue'
import { getAllDevices } from '@/api/map'
import { getDeviceMedia } from '@/api/media'

// ---------------- 状态 ----------------
const deviceOptions = ref<any[]>([])
const mediaMap = ref<Record<string, any[]>>({}) // hw_id -> monitoring media list
const selDeviceIds = ref<string[]>([])
const gridSize = ref(4) // 1/4/9/16
const started = ref(false)
const isFullscreen = ref(false)

interface Cell {
  key: string
  hwId?: string
  label: string
  url?: string
  playable: boolean
  state: 'idle' | 'loading' | 'live' | 'error'
  streaming: boolean
  fullscreen: boolean
}

const gridCells = ref<Cell[]>([])

// 宫格占位总数
function gridCount() { return gridSize.value }

// 生成当前宫格数据（按选中设备顺序填充，不足补占位）
function buildCells() {
  const n = gridCount()
  const cells: Cell[] = []
  for (let i = 0; i < n; i++) {
    const hwId = selDeviceIds.value[i]
    if (hwId === undefined) {
      cells.push({ key: 'empty-' + i, label: '空闲', playable: false, state: 'idle', streaming: false, fullscreen: false })
      continue
    }
    const dev = deviceOptions.value.find((d) => d.hw_id === hwId)
    const media = (mediaMap.value[hwId] || []).find((m) => m.media_type === 'monitoring')
    const label = dev?.intersection || '#' + hwId
    const url = media?.compatible_url || media?.url
    const playable = !!url && !/^rtsp:/i.test(url) && !/\.m3u8$/i.test(url || '') // 浏览器直接可播
    cells.push({
      key: 'cell-' + hwId,
      hwId,
      label,
      url,
      playable,
      state: url ? (playable ? 'live' : 'error') : 'idle',
      streaming: playable,
      fullscreen: false,
    })
  }
  return cells
}

// 切换宫格数：更新布局并保留尽量多的选中
function setGrid(n: number) {
  gridSize.value = n
  // 若设备不够宫格数，提示可继续选择（不强制填充）
  gridCells.value = buildCells()
}
function setCellState(cell: Cell, state: Cell['state']) {
  cell.state = state
  cell.streaming = state === 'live'
}

async function startPlay() {
  if (selDeviceIds.value.length === 0) {
    ElMessage.warning('请先选择设备')
    return
  }
  started.value = true
  // 预取所选设备的 monitoring 媒体
  for (const hwId of selDeviceIds.value) {
    if (!mediaMap.value[hwId]) {
      try {
        const res = await getDeviceMedia({ device_hw_id: hwId, media_type: 'monitoring', page_size: 5 })
        mediaMap.value[hwId] = res.data?.list || []
      } catch { mediaMap.value[hwId] = [] }
    }
  }
  gridCells.value = buildCells()
}

function reloadAll() {
  gridCells.value.forEach((c) => { if (c.hwId) setCellState(c, 'loading') })
  // 重新生成以刷新 video src
  setTimeout(() => { gridCells.value = buildCells() }, 50)
}

function placeholderText(item: Cell): string {
  if (item.url && /^rtsp:/i.test(item.url)) return 'RTSP 源已配置'
  if (item.url) return '源不可浏览器直播'
  return '未配置监控源'
}

// 单格放大（在当前布局内切换 fullscreen 标志，CSS 使其跨列）
function toggleCellFullscreen(item: Cell) {
  const found = gridCells.value.find((c) => c.key === item.key)
  if (found) found.fullscreen = !found.fullscreen
}

// ---------------- 系统全屏 ----------------
const bodyEl = ref<HTMLElement | null>(null)
const bodyRef = (el: any) => { bodyEl.value = el }
function toggleFullscreen() {
  if (!isFullscreen.value) {
    const el = bodyEl.value as any
    const req = el?.requestFullscreen || el?.webkitRequestFullscreen
    if (req) { req.call(el); isFullscreen.value = true }
  } else {
    const doc = document as any
    const exit = doc.exitFullscreen || doc.webkitExitFullscreen
    if (exit) { exit.call(doc); isFullscreen.value = false }
  }
}
function onFsChange() { isFullscreen.value = !!(document as any).fullscreenElement }

// ---------------- 键盘快捷键 ----------------
function onKey(e: KeyboardEvent) {
  if ((e.target as HTMLElement)?.tagName === 'INPUT') return
  const k = Number(e.key)
  if (k === 1) setGrid(1)
  else if (k === 4) setGrid(4)
  else if (k === 9) setGrid(9)
  else if (k === 16) setGrid(16)
  else if (e.key === 'f' || e.key === 'F') toggleFullscreen()
}

onMounted(async () => {
  const res = await getAllDevices()
  deviceOptions.value = res.data?.list || []
  document.addEventListener('fullscreenchange', onFsChange)
  window.addEventListener('keydown', onKey)
})
onBeforeUnmount(() => {
  document.removeEventListener('fullscreenchange', onFsChange)
  window.removeEventListener('keydown', onKey)
})
</script>

<style scoped>
.monitor-wall {
  display: flex; flex-direction: column; height: 100%; background: #0a0e17; color: #eee; border-radius: 6px; overflow: hidden;
}
.monitor-wall.fullscreen { border-radius: 0; }
.wall-toolbar {
  display: flex; justify-content: space-between; align-items: center; gap: 12px;
  padding: 10px 14px; background: #121826; border-bottom: 1px solid #1f2a3d; flex-wrap: wrap;
}
.left { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.right { display: flex; align-items: center; gap: 8px; }
.title { font-size: 15px; font-weight: 600; }
.wall-body { flex: 1; display: grid; gap: 6px; padding: 8px; background: #0a0e17; overflow: auto; }
.grid-1 { grid-template-columns: repeat(1, 1fr); grid-template-rows: 1fr; }
.grid-4 { grid-template-columns: repeat(2, 1fr); grid-template-rows: repeat(2, 1fr); }
.grid-9 { grid-template-columns: repeat(3, 1fr); grid-template-rows: repeat(3, 1fr); }
.grid-16 { grid-template-columns: repeat(4, 1fr); grid-template-rows: repeat(4, 1fr); }
.cell {
  position: relative; background: #000; border: 1px solid #1f2a3d; border-radius: 4px;
  display: flex; flex-direction: column; min-height: 140px; transition: all .2s;
}
.cell.live { border-color: #2f6f4f; }
.cell.fullscreen { grid-column: 1 / -1; grid-row: 1 / -1; z-index: 10; }
.cell-header {
  display: flex; align-items: center; gap: 8px; padding: 4px 8px; background: rgba(0,0,0,.5);
  font-size: 12px; color: #eee; white-space: nowrap;
}
.cell-name { overflow: hidden; text-overflow: ellipsis; flex: 1; }
.cell-status { font-size: 11px; }
.cell-status.live { color: #67C23A; }
.cell-status.loading { color: #E6A23C; }
.cell-status.error { color: #F56C6C; }
.cell-status.idle { color: #909399; }
.cell-extra { display: flex; }
.cell-video { flex: 1; display: flex; align-items: center; justify-content: center; min-height: 0; }
.cell-placeholder { display: flex; flex-direction: column; align-items: center; gap: 6px; color: #909399; font-size: 12px; }
.cell-placeholder .tip { font-size: 11px; color: #5c6b8a; }
.loading-box .el-icon { font-size: 22px; }
</style>
