<template>
  <div class="video-panel">
    <!-- 上传与登记 -->
    <div class="toolbar">
      <div class="toolbar-grid">
        <el-select v-model="deviceId" placeholder="选择设备（硬件ID）" filterable style="width: 180px">
          <el-option v-for="d in deviceOptions" :key="'hw'+d.hw_id" :label="(d.intersection || '#'+d.hw_id)" :value="d.hw_id" />
        </el-select>
        <el-radio-group v-model="mediaType" size="small">
          <el-radio-button value="evidence">举证视频</el-radio-button>
          <el-radio-button value="monitoring">监控实时</el-radio-button>
          <el-radio-button value="timelapse">时间视频</el-radio-button>
        </el-radio-group>
      </div>
      <div class="toolbar-actions">
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

    <!-- 类型筛选 -->
    <el-radio-group v-model="filterType" size="small" style="margin: 8px 0">
      <el-radio-button value="">全部</el-radio-button>
      <el-radio-button value="evidence">举证</el-radio-button>
      <el-radio-button value="monitoring">监控</el-radio-button>
      <el-radio-button value="timelapse">时间视频</el-radio-button>
    </el-radio-group>

    <!-- 媒体网格 -->
    <div class="media-grid" v-loading="loading">
      <div v-for="m in filteredMedia" :key="m.id" class="media-card">
        <div class="media-preview" @click="playMedia(m)">
          <video v-if="m.category === 'video' && isPlayable(m.url)" :src="mediaUrl(m.url)" muted preload="metadata" />
          <img v-else-if="m.category === 'photo'" :src="mediaUrl(m.url)" />
          <div v-else class="placeholder">视频</div>
          <span class="badge">{{ typeLabel(m.media_type) }}</span>
          <span class="src-badge">{{ sourceLabel(m.source) }}</span>
        </div>
        <div class="media-info">
          <div class="mt">{{ m.title || '未命名' }}</div>
          <div class="md">设备#{{ m.device_hw_id }} · {{ m.created_at?.slice(0, 16) }}</div>
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
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getAllDevices } from '@/api/map'
import { getDeviceMedia, uploadDeviceMedia, createStreamMedia, deleteDeviceMedia, type DeviceMedia } from '@/api/media'
import { useAuthStore } from '@/store/auth'

// 登录角色（运维/管理员可上传/删除）
const authStore = useAuthStore()
const canEdit = computed(() => { const r = authStore.user?.role; return r === 'admin' || r === 'operator' })

const BASE_URL = '/tsloms'

function mediaUrl(url?: string): string {
  if (!url) return ''
  // /media/... → /tsloms/media/...（经 nginx 反代）
  if (url.startsWith('/media/')) return BASE_URL + url
  return url
}
function isPlayable(url?: string): boolean {
  return !!url && !/^rtsp?:\/\//i.test(url) && !/\.m3u8$/i.test(url || '')
}

const deviceOptions = ref<any[]>([])
const deviceId = ref<number | undefined>()
const mediaType = ref('evidence')
const filterType = ref('')
const mediaList = ref<DeviceMedia[]>([])
const loading = ref(false)

const filteredMedia = computed(() => {
  if (!filterType.value) return mediaList.value
  return mediaList.value.filter((m) => m.media_type === filterType.value)
})

const typeLabel = (t: string) => ({ evidence: '举证', monitoring: '监控', timelapse: '时间视频' } as Record<string, string>)[t] || t
const sourceLabel = (s: string) => ({ upload: '上传', rtsp: 'RTSP', url: 'URL' } as Record<string, string>)[s] || s

async function loadDevices() {
  const res = await getAllDevices()
  deviceOptions.value = res.data?.list || []
}
async function loadMedia() {
  loading.value = true
  try {
    const res = await getDeviceMedia({ page_size: 100 })
    mediaList.value = res.data?.list || []
  } catch { ElMessage.error('媒体列表加载失败') } finally { loading.value = false }
}

// 手机上传
async function doUpload(opt: any) {
  if (!deviceId.value) { ElMessage.warning('请先选择设备'); return }
  const fd = new FormData()
  fd.append('device_hw_id', String(deviceId.value))
  fd.append('media_type', mediaType.value)
  fd.append('file', opt.file)
  try {
    await uploadDeviceMedia(fd)
    ElMessage.success('上传成功')
    loadMedia()
  } catch { ElMessage.error('上传失败') }
}

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
const streamForm = reactive({ device_hw_id: undefined as number | undefined, media_type: 'monitoring', url: '', compatible_url: '', title: '', duration: 0, note: '' })
function openStreamDialog() { streamVisible.value = true }
async function saveStream() {
  if (!streamForm.device_hw_id || !streamForm.url) { ElMessage.warning('设备与地址必填'); return }
  try {
    await createStreamMedia({ device_hw_id: streamForm.device_hw_id, media_type: streamForm.media_type as any, url: streamForm.url, compatible_url: streamForm.compatible_url, title: streamForm.title, duration: streamForm.duration, note: streamForm.note })
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
.video-panel { padding: 4px; }
.toolbar { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; margin-bottom: 8px; }
.toolbar-grid { display: flex; gap: 8px; align-items: center; }
.toolbar-actions { display: flex; gap: 8px; }
.media-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 12px; }
.media-card { border: 1px solid #ebeef5; border-radius: 6px; overflow: hidden; background: #fff; }
.media-preview { position: relative; height: 130px; background: #000; cursor: pointer; display: flex; align-items: center; justify-content: center; }
.media-preview video, .media-preview img { width: 100%; height: 100%; object-fit: cover; }
.placeholder { color: #909399; font-size: 14px; }
.badge { position: absolute; top: 6px; left: 6px; background: rgba(64,158,255,0.9); color: #fff; font-size: 12px; padding: 1px 6px; border-radius: 3px; }
.src-badge { position: absolute; top: 6px; right: 6px; background: rgba(0,0,0,0.6); color: #fff; font-size: 12px; padding: 1px 6px; border-radius: 3px; }
.media-info { padding: 8px; }
.mt { font-size: 13px; font-weight: 600; }
.md { font-size: 12px; color: #909399; margin: 4px 0; }
.ops { display: flex; gap: 6px; }
.tip { color: #909399; font-size: 12px; }
</style>
