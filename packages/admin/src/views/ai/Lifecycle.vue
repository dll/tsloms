<template>
  <div class="life-page">
    <el-card shadow="never">
      <div class="head-bar">
        <div class="title"><el-icon><DataAnalysis /></el-icon> AI 全流程生命周期溯源</div>
        <div class="sub">聚合单台信号灯的使用/故障/维修/耗材/报废全过程，生成溯源画像与时间线</div>
      </div>
      <el-form inline class="pick-form">
        <el-form-item label="设备">
          <el-select v-model="hwid" placeholder="搜索设备ID或路口" filterable remote :remote-method="searchDevices"
                     :loading="devLoading" style="width: 300px" @change="doBuild">
            <el-option v-for="d in devices" :key="d.hw_id" :value="d.hw_id"
                       :label="(d.intersection || '#'+d.hw_id) + (d.online_status ? '（在线）' : '（离线）')" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="building" :disabled="!hwid" @click="doBuild">
            <el-icon><Cpu /></el-icon> 生成溯源分析
          </el-button>
        </el-form-item>
      </el-form>

      <div v-if="result" class="result-area">
        <!-- 画像 -->
        <el-card shadow="never" class="profile-card">
          <div class="ring-head">
            <div class="ring-title">
              <el-icon><Cpu /></el-icon> 生命周期画像
              <el-tag size="small" :type="result.result.source.includes('LLM') ? 'success' : 'warning'" style="margin-left:8px">
                {{ result.result.source }}
              </el-tag>
              <span v-if="result.result.tokens_used" class="tok">消耗 {{ result.result.tokens_used }} token</span>
            </div>
            <div class="ring-meta">
              <el-tag>设备 #{{ result.result.device_hw_id }}</el-tag>
              <el-tag type="info">{{ result.result.intersection || '未指定路口' }}</el-tag>
              <div class="type-count">
                <span>故障 {{ typeCount.fault || 0 }}</span>
                <span>工单 {{ typeCount.workorder || 0 }}</span>
                <span>安装 {{ typeCount.install || 0 }}</span>
              </div>
            </div>
          </div>
          <div class="profile-text">{{ result.result.summary }}</div>
        </el-card>

        <!-- 时间线 -->
        <div class="tl-title">全流程时间线</div>
        <el-timeline class="tl">
          <el-timeline-item v-for="(e, i) in result.result.timeline" :key="i"
                            :timestamp="e.time" :type="tlType(e.type)" placement="top">
            <div class="tl-item">
              <span class="tl-tag" :class="e.type">{{ tlLabel(e.type) }}</span>
              <span class="tl-title-text">{{ e.title }}</span>
              <div v-if="e.desc" class="tl-desc">{{ e.desc }}</div>
            </div>
          </el-timeline-item>
        </el-timeline>
        <el-empty v-if="!result.result.timeline.length" description="该设备暂无生命周期事件" :image-size="60" />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Cpu, DataAnalysis } from '@element-plus/icons-vue'
import { getDevices } from '@/api/device'
import { buildLifecycle } from '@/api/ai'

const devices = ref<any[]>([])
const devLoading = ref(false)
const hwid = ref<number | undefined>()
const building = ref(false)
const result = ref<any>(null)

const typeCount = computed(() => {
  const m: Record<string, number> = {}
  const events = (result.value as any)?.result?.timeline as any[] | undefined
  ;(events || []).forEach((e: any) => { m[e.type] = (m[e.type] || 0) + 1 })
  return m
})

function tlLabel(t: string) {
  return { install: '安装', fault: '故障', workorder: '维修', scrap: '报废', offline: '离线' }[t] || t
}
function tlType(t: string): any {
  return { install: 'primary', fault: 'danger', workorder: 'success' }[t] || 'info'
}

async function searchDevices(keyword?: string) {
  devLoading.value = true
  try {
    const kw = (keyword || '').trim()
    const params: Record<string, any> = { page_size: 50 }
    if (kw && /^\d+$/.test(kw)) params.hw_id = kw
    else if (kw) params.intersection = kw
    const res = await getDevices(params)
    devices.value = res.data?.list || []
  } catch { devices.value = [] }
  finally { devLoading.value = false }
}

async function doBuild() {
  if (!hwid.value) return
  building.value = true
  result.value = null
  try {
    const res = await buildLifecycle(hwid.value)
    result.value = res.data || null
    if (!result.value?.result) ElMessage.warning('未返回生命周期数据')
  } catch {
    ElMessage.error('溯源分析失败（可能额度不足）')
  } finally {
    building.value = false
  }
}

onMounted(() => searchDevices(''))
</script>

<style scoped>
.head-bar { display: flex; align-items: baseline; gap: 12px; margin-bottom: 12px; }
.title { display: flex; align-items: center; gap: 8px; font-weight: 600; font-size: 16px; }
.sub { color: #909399; font-size: 12px; }
.pick-form { margin-bottom: 12px; }
.profile-card { margin-bottom: 16px; }
.ring-head { margin-bottom: 10px; }
.ring-title { display: flex; align-items: center; gap: 4px; font-weight: 600; }
.ring-meta { margin-top: 8px; display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.tok { font-size: 12px; color: #909399; margin-left: 8px; font-weight: normal; }
.type-count { color: #606266; font-size: 13px; margin-left: 8px; }
.type-count span { margin-right: 12px; }
.profile-text { color: #303133; line-height: 1.8; font-size: 14px; background: #f8f9fb; padding: 12px; border-radius: 8px; }
.tl-title { font-weight: 600; margin-bottom: 12px; }
.tl { padding-left: 4px; }
.tl-item { display: flex; align-items: baseline; gap: 8px; flex-wrap: wrap; }
.tl-tag { font-size: 11px; padding: 1px 8px; border-radius: 4px; color: #fff; }
.tl-tag.install { background: #409eff; }
.tl-tag.fault { background: #f56c6c; }
.tl-tag.workorder { background: #67c23a; }
.tl-tag.offline { background: #909399; }
.tl-tag.scrap { background: #000; }
.tl-title-text { font-weight: 600; font-size: 13px; }
.tl-desc { width: 100%; color: #909399; font-size: 12px; }
</style>
