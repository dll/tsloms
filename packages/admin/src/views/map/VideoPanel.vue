<template>
  <div class="video-panel">
    <!-- 顶部工具条 -->
    <div class="toolbar">
      <div class="toolbar-grid">
        <el-select v-model="selIntersections" multiple filterable collapse-tags placeholder="按路口筛选（多选）" style="width: 240px" :clearable="true">
          <el-option v-for="i in intersectionOptions" :key="i" :label="i" :value="i" />
        </el-select>
        <el-select v-model="deviceId" placeholder="上传/登记设备" filterable clearable style="width: 180px" @change="onDeviceChange">
          <el-option v-for="d in deviceOptions" :key="'hw'+d.hw_id" :label="(d.intersection || '#'+d.hw_id) + ' (#'+d.hw_id+')'" :value="d.hw_id" />
        </el-select>
        <el-radio-group v-model="mediaType" size="small">
          <el-radio-button value="evidence">举证</el-radio-button>
          <el-radio-button value="monitoring">监控实时</el-radio-button>
          <el-radio-button value="timelapse">时间视频</el-radio-button>
        </el-radio-group>
      </div>
      <div class="toolbar-actions">
        <el-button :icon="Monitor" @click="goMonitor">监控大屏</el-button>
        <el-upload
          v-if="canEdit"
          :show-file-list="false"
          :http-request="doUpload"
          accept=".mp4,.mov,.webm,.avi,.jpg,.png"
        >
          <el-button type="primary">手机上传举证</el-button>
        </el-upload>
        <el-button v-if="canEdit" @click="openStreamDialog">登记RTSP/URL</el-button>
      </div>
    </div>

    <!-- 类型/状态筛选 -->
    <div class="filter-bar">
      <el-radio-group v-model="filterType" size="small">
        <el-radio-button value="">全部</el-radio-button>
        <el-radio-button value="evidence">举证</el-radio-button>
        <el-radio-button value="monitoring">监控</el-radio-button>
        <el-radio-button value="timelapse">时间视频</el-radio-button>
      </el-radio-group>
      <el-checkbox v-model="onlyFault" size="small">仅看故障中</el-checkbox>
      <span class="count-hint">共 {{ filteredMedia.length }} 条 · {{ intersectionCount }} 个路口</span>
    </div>

    <!-- 媒体网格（多路口多视频） -->
    <div class="media-grid" v-loading="loading">
      <div v-for="m in filteredMedia" :key="m.id" class="media-card" :class="{ fault: m.is_active_fault }">
        <div class="media-preview" @click="playMedia(m)">
          <video v-if="m.category === 'video' && isPlayable(m.url)" :src="mediaUrl(m.url)" muted preload="metadata" />
          <img v-else-if="m.category === 'photo'" :src="mediaUrl(m.url)" />
          <div v-else class="placeholder">视频</div>
          <span class="badge">{{ typeLabel(m.media_type) }}</span>
          <span class="src-badge">{{ sourceLabel(m.source) }}</span>
          <span v-if="m.is_active_fault" class="fault-badge">故障中</span>
        </div>
        <div class="media-info">
          <div class="mt">{{ m.title || '未命名' }}</div>
          <div class="md">
            <span v-if="m.intersection" class="mi">📍 {{ m.intersection }}</span>
            <span class="dev">#{{ m.device_hw_id }} · {{ m.created_at?.slice(0, 16) }}</span>
          </div>
          <!-- 信号灯信息 -->
          <div v-if="m.intersection || m.light_color || m.fault_desc" class="signal-info">
            <el-tag v-if="m.light_color" size="small" :type="lightColorTag(m.light_color)">{{ lightColorLabel(m.light_color) }}灯</el-tag>
            <span v-if="m.fault_desc" class="fdesc">{{ m.fault_desc }}</span>
          </div>
          <div class="ops">
            <el-button size="small" @click="playMedia(m)">播放</el-button>
            <el-button v-if="canEdit" size="small" type="danger" @click="del(m)">删除</el-button>
          </div>
        </div>
      </div>
      <el-empty v-if="!loading && filteredMedia.length === 0" description="暂无视频/图片" />
    </div>

    <!-- 播放弹窗 -->
    <el-dialog v-model="playVisible" :title="playing?.title || '播放'" width="720px" append-to-body>
      <div class="player">
        <div v-if="playing && playing.intersection" class="player-meta">
          <span>📍 {{ playing.intersection }}</span>
          <el-tag v-if="playing.light_color" size="small" :type="lightColorTag(playing.light_color)">{{ lightColorLabel(playing.light_color) }}灯</el-tag>
          <span v-if="playing.is_active_fault" class="pc-fault">故障中</span>
        </div>
        <video v-if="playing && playing.category === 'video'" :src="mediaUrl(playing.url)" controls autoplay style="width:100%" />
        <img v-else-if="playing && playing.category === 'photo'" :src="mediaUrl(playing.url)" style="width:100%" />
        <div v-else class="rtsp-hint">
          <p>当前为监控流（{{ playing?.url }}）</p>
          <p v-if="playing?.compatible_url">兼容播放地址：{{ playing?.compatible_url }}</p>
          <p class="tip">RTSP 无法在浏览器直接播放，请使用兼容地址（HLS/FLV）或本地播放器打开。</p>
        </div>
      </div>
    </el-dialog>

    <!-- 登记 RTSP/URL 弹窗 -->
    <el-dialog v-model="streamVisible" title="登记监控/时间视频源" width="520px" append-to-body>
      <el-form label-width="110px">
        <el-form-item label="设备">
          <el-select v-model="streamForm.device_hw_id" filterable style="width: 100%">
            <el-option v-for="d in deviceOptions" :key="d.hw_id" :label="(d.intersection || '#'+d.hw_id)" :value="d.hw_id" />
          </el-select>
        </el-form-item>
        <el-form-item label="路口名称">
          <el-input v-model="streamForm.intersection" placeholder="便于定位路口" />
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="streamForm.media_type" style="width: 100%">
            <el-option label="监控实时" value="monitoring" />
            <el-option label="时间视频（监控截取片段）" value="timelapse" />
          </el-select>
        </el-form-item>
        <el-form-item label="视频地址">
          <el-input v-model="streamForm.url" placeholder="rtsp://... 或 https://... " />
        </el-form-item>
        <el-form-item label="兼容播放地址">
          <el-input v-model="streamForm.compatible_url" placeholder="HLS(m3u8)/FLV地址，便于浏览器播放" />
        </el-form-item>
        <el-form-item label="标题">
          <el-input v-model="streamForm.title" />
        </el-form-item>
        <el-form-item label="时长(秒)">
          <el-input-number v-model="streamForm.duration" :min="0" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="streamForm.note" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="streamVisible = false">取消</el-button>
        <el-button type="primary" @click="saveStream">保存</el-button>
      </template>
    </el-dialog>

    <!-- 手机上传举证（含必要信号灯信息） -->
    <el-dialog v-model="uploadVisible" title="手机上传举证" width="520px" append-to-body>
      <el-alert type="info" :closable="false" show-icon style="margin-bottom: 12px"
                title="请填写必要的信号灯信息，否则难以定位路口、识别故障与派发工单" />
      <el-form label-width="110px">
        <el-form-item label="关联设备" required>
          <el-select v-model="uploadForm.device_hw_id" filterable style="width: 100%" @change="onUploadDeviceChange">
            <el-option v-for="d in deviceOptions" :key="d.hw_id" :label="(d.intersection || '#'+d.hw_id) + ' (#'+d.hw_id+')'" :value="d.hw_id" />
          </el-select>
        </el-form-item>
        <el-form-item label="路口名称" required>
          <el-input v-model="uploadForm.intersection" placeholder="如：人民路与建设路交叉口" />
        </el-form-item>
        <el-form-item label="故障灯色" required>
          <el-radio-group v-model="uploadForm.light_color">
            <el-radio-button value="red">红灯</el-radio-button>
            <el-radio-button value="yellow">黄灯</el-radio-button>
            <el-radio-button value="green">绿灯</el-radio-button>
            <el-radio-button value="unknown">不确定</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="故障现象" required>
          <el-input v-model="uploadForm.fault_desc" placeholder="如：红灯常亮不熄 / 不亮 / 闪烁异常" />
        </el-form-item>
        <el-form-item label="是否故障中">
          <el-switch v-model="uploadForm.is_active_fault" />
        </el-form-item>
        <el-form-item label="标题">
          <el-input v-model="uploadForm.title" placeholder="如：人民路口红灯异常" />
        </el-form-item>
        <el-form-item label="上传文件" required>
          <el-upload :show-file-list="true" :limit="1" :auto-upload="false" accept=".mp4,.mov,.webm,.avi,.jpg,.jpeg,.png,.gif"
                     :on-change="onFileChange" :on-remove="() => (uploadFile = null)">
            <el-button>选择文件</el-button>
          </el-upload>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="uploadVisible = false">取消</el-button>
        <el-button type="primary" :loading="uploading" @click="confirmUpload">提交上传</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import type { UploadFile } from 'element-plus'
import { useRouter } from 'vue-router'
import { Monitor } from '@element-plus/icons-vue'
import { getAllDevices } from '@/api/map'
import { getDeviceMedia, uploadDeviceMedia, createStreamMedia, deleteDeviceMedia, type DeviceMedia } from '@/api/media'
import { useAuthStore } from '@/store/auth'

const authStore = useAuthStore()
const canEdit = computed(() => { const r = authStore.user?.role; return r === 'admin' || r === 'operator' })
const router = useRouter()
function goMonitor() { router.push('/monitor') }

const BASE_URL = '/tsloms'
function mediaUrl(url?: string): string {
  if (!url) return ''
  if (url.startsWith('/media/')) return BASE_URL + url
  return url
}
function isPlayable(url?: string): boolean {
  return !!url && !/^rtsp?:\/\//i.test(url) && !/\.m3u8$/i.test(url || '')
}

const deviceOptions = ref<any[]>([])
const intersectionOptions = computed(() => {
  const set = new Set<string>()
  for (const d of deviceOptions.value) if (d.intersection) set.add(d.intersection)
  return Array.from(set)
})

const selIntersections = ref<string[]>([])
const deviceId = ref<string | undefined>()
const mediaType = ref('evidence')
const filterType = ref('')
const onlyFault = ref(false)
const mediaList = ref<DeviceMedia[]>([])
const loading = ref(false)

const filteredMedia = computed(() => {
  let arr = mediaList.value
  if (filterType.value) arr = arr.filter((m) => m.media_type === filterType.value)
  if (selIntersections.value.length) arr = arr.filter((m) => selIntersections.value.includes(m.intersection || ''))
  if (onlyFault.value) arr = arr.filter((m) => m.is_active_fault)
  return arr
})
const intersectionCount = computed(() => new Set(filteredMedia.value.map((m) => m.intersection).filter(Boolean)).size)

const typeLabel = (t: string) => ({ evidence: '举证', monitoring: '监控', timelapse: '时间视频' } as Record<string, string>)[t] || t
const sourceLabel = (s: string) => ({ upload: '上传', rtsp: 'RTSP', url: 'URL' } as Record<string, string>)[s] || s
const lightColorLabel = (c: string) => ({ red: '红', yellow: '黄', green: '绿', unknown: '不确定' } as Record<string, string>)[c] || c
const lightColorTag = (c: string) => ({ red: 'danger', yellow: 'warning', green: 'success', unknown: 'info' } as Record<string, string>)[c] || 'info'

async function loadDevices() {
  const res = await getAllDevices()
  deviceOptions.value = res.data?.list || []
}
async function loadMedia() {
  loading.value = true
  try {
    const res = await getDeviceMedia({ page_size: 200 })
    mediaList.value = res.data?.list || []
  } catch { ElMessage.error('媒体列表加载失败') } finally { loading.value = false }
}

// ---- 多路口/多视频 ----
function onDeviceChange() { /* 选择设备用于上传/登记 */ }

// ---- 上传（举证必填信号灯信息） ----
const uploadVisible = ref(false)
const uploadForm = reactive({ device_hw_id: undefined as string | undefined, intersection: '', light_color: 'red', fault_desc: '', is_active_fault: false, title: '' })
const uploadFile = ref<File | null>(null)
const uploading = ref(false)

function openUpload() {
  Object.assign(uploadForm, { device_hw_id: deviceId.value, intersection: '', light_color: 'red', fault_desc: '', is_active_fault: false, title: '' })
  uploadFile.value = null
  uploadVisible.value = true
}
function onUploadDeviceChange(id: string) {
  const d = deviceOptions.value.find((x) => x.hw_id === id)
  if (d?.intersection) uploadForm.intersection = d.intersection
  if (d && !uploadForm.title) uploadForm.title = d.intersection ? `${d.intersection} 信号灯异常` : ''
}
function onFileChange(file: UploadFile) { uploadFile.value = file.raw as File }

async function confirmUpload() {
  if (!uploadForm.device_hw_id) { ElMessage.warning('请选择关联设备'); return }
  if (!uploadForm.intersection.trim()) { ElMessage.warning('请填写路口名称（便于定位）'); return }
  if (!uploadForm.light_color) { ElMessage.warning('请选择故障灯色'); return }
  if (!uploadForm.fault_desc.trim()) { ElMessage.warning('请填写故障现象'); return }
  if (!uploadFile.value) { ElMessage.warning('请选择要上传的文件'); return }
  uploading.value = true
  try {
    const fd = new FormData()
    fd.append('device_hw_id', uploadForm.device_hw_id)
    fd.append('media_type', 'evidence')
    fd.append('intersection', uploadForm.intersection.trim())
    fd.append('light_color', uploadForm.light_color)
    fd.append('fault_desc', uploadForm.fault_desc.trim())
    fd.append('is_active_fault', String(uploadForm.is_active_fault))
    fd.append('title', uploadForm.title.trim())
    fd.append('file', uploadFile.value)
    await uploadDeviceMedia(fd)
    ElMessage.success('上传成功')
    uploadVisible.value = false
    loadMedia()
  } catch { ElMessage.error('上传失败') } finally { uploading.value = false }
}

// 原快捷上传入口改用带信号灯信息的新弹窗
async function doUpload() { openUpload() }

// 播放
const playVisible = ref(false)
const playing = ref<DeviceMedia | null>(null)
function playMedia(m: DeviceMedia) { playing.value = m; playVisible.value = true }

// 删除
async function del(m: DeviceMedia) {
  try { await deleteDeviceMedia(m.id); ElMessage.success('已删除'); loadMedia() } catch { /* 忽略 */ }
}

// 登记RTSP
const streamVisible = ref(false)
const streamForm = reactive({ device_hw_id: undefined as string | undefined, intersection: '', media_type: 'monitoring', url: '', compatible_url: '', title: '', duration: 0, note: '' })
function openStreamDialog() { streamVisible.value = true }
async function saveStream() {
  if (!streamForm.device_hw_id || !streamForm.url) { ElMessage.warning('设备与地址必填'); return }
  try {
    await createStreamMedia({ device_hw_id: streamForm.device_hw_id, media_type: streamForm.media_type as any, url: streamForm.url, compatible_url: streamForm.compatible_url, title: streamForm.title, duration: streamForm.duration, note: streamForm.note, intersection: streamForm.intersection })
    ElMessage.success('登记成功')
    streamVisible.value = false
    loadMedia()
  } catch { /* 忽略 */ }
}

onMounted(async () => {
  await loadDevices()
  await loadMedia()
})
</script>

<style scoped>
.video-panel { padding: 12px; background: #f0f2f5; border-radius: 6px; }
.toolbar { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; margin-bottom: 8px; }
.toolbar-grid { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.toolbar-actions { display: flex; gap: 8px; align-items: center; }
.filter-bar { display: flex; align-items: center; gap: 14px; margin: 8px 0; }
.count-hint { color: #909399; font-size: 13px; }
.media-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; }
.media-card { border: 1px solid #ebeef5; border-radius: 6px; overflow: hidden; background: #fff; }
.media-card.fault { border-color: #F56C6C; }
.media-preview { position: relative; height: 130px; background: #000; cursor: pointer; display: flex; align-items: center; justify-content: center; }
.media-preview video, .media-preview img { width: 100%; height: 100%; object-fit: cover; }
.placeholder { color: #909399; font-size: 14px; }
.badge { position: absolute; top: 6px; left: 6px; background: rgba(64,158,255,0.9); color: #fff; font-size: 12px; padding: 1px 6px; border-radius: 3px; }
.src-badge { position: absolute; top: 6px; right: 6px; background: rgba(0,0,0,0.6); color: #fff; font-size: 12px; padding: 1px 6px; border-radius: 3px; }
.fault-badge { position: absolute; bottom: 6px; right: 6px; background: #F56C6C; color: #fff; font-size: 12px; padding: 1px 6px; border-radius: 3px; }
.media-info { padding: 8px; }
.mt { font-size: 13px; font-weight: 600; }
.md { font-size: 12px; color: #909399; margin: 4px 0; display: flex; flex-direction: column; gap: 2px; }
.mi { color: #409eff; }
.signal-info { display: flex; align-items: center; gap: 6px; margin: 4px 0; flex-wrap: wrap; }
.fdesc { font-size: 12px; color: #606266; }
.ops { display: flex; gap: 6px; margin-top: 4px; }
.tip { color: #909399; font-size: 12px; }
.player-meta { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; color: #409eff; font-size: 14px; }
.pc-fault { color: #F56C6C; font-weight: 600; }
</style>
