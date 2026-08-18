<template>
  <div class="video-overlay">
    <div class="overlay-head">
      <span>{{ title }}</span>
      <el-button link type="info" size="small" @click="$emit('close')">×</el-button>
    </div>
    <div class="overlay-body">
      <template v-if="playing">
        <video v-if="playing.category === 'video' && isPlayable(playing.url)" :src="mediaUrl(playing.url)" controls autoplay muted style="width:100%" />
        <img v-else-if="playing.category === 'photo'" :src="mediaUrl(playing.url)" style="width:100%" />
        <div v-else class="rtsp-hint">
          <p>监控流（{{ playing.url }}）</p>
          <p class="tip">RTSP 无法在浏览器直接播放，请使用 HLS/FLV 兼容地址。</p>
        </div>
      </template>
      <el-empty v-else description="该设备暂无监控视频" :image-size="50" />
      <div v-if="list.length > 1" class="media-list">
        <el-tag v-for="m in list" :key="m.id" size="small" effect="plain" :type="m.id === playing?.id ? 'primary' : 'info'" style="cursor:pointer" @click="playing = m">
          {{ m.title || m.media_type }}
        </el-tag>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { getDeviceMedia, type DeviceMedia } from '@/api/media'

const props = defineProps<{ deviceHwId?: number; title?: string }>()
defineEmits<{ (e: 'close'): void }>()

const list = ref<DeviceMedia[]>([])
const playing = ref<DeviceMedia | null>(null)

function mediaUrl(url: string) {
  if (!url) return ''
  if (url.startsWith('http') || url.startsWith('/media')) return url
  return '/media/' + url
}
function isPlayable(url: string) {
  return !!url && !/^rtsp:/i.test(url)
}

async function load() {
  playing.value = null
  if (!props.deviceHwId) { list.value = []; return }
  try {
    const res = await getDeviceMedia({ device_hw_id: String(props.deviceHwId), page_size: 50 })
    list.value = (res.data?.list || []).filter((m: DeviceMedia) => m.media_type === 'monitoring')
    if (list.value.length) playing.value = list.value[0]
  } catch {
    list.value = []
  }
}

watch(() => props.deviceHwId, load)
onMounted(load)
</script>

<style scoped>
.video-overlay {
  position: absolute; right: 12px; top: 12px; width: 360px; z-index: 120;
  background: #fff; border-radius: 8px; box-shadow: 0 4px 16px rgba(0,0,0,.2); overflow: hidden;
}
.overlay-head { display: flex; justify-content: space-between; align-items: center; padding: 8px 10px; font-weight: 600; background: #f5f7fa; }
.overlay-body { padding: 10px; }
.rtsp-hint p { margin: 4px 0; font-size: 12px; color: #909399; }
.media-list { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
</style>
