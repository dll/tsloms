<template>
  <div class="diag-page">
    <el-card shadow="never">
      <div class="head-bar">
        <div class="title"><el-icon><MagicStick /></el-icon> AI 故障诊断</div>
        <div class="sub">依据问题反馈（文字/图片）+ 设备最近故障记录，由多模态大模型自动识别问题并输出诊断与解决方案</div>
      </div>

      <!-- 选择反馈 -->
      <el-form inline class="pick-form">
        <el-form-item label="问题反馈">
          <el-select v-model="fbId" placeholder="选择一条问题反馈" filterable style="width: 420px" :loading="fbLoading">
            <el-option v-for="f in feedbacks" :key="f.id" :value="f.id"
                       :label="(f.intersection || '#'+f.device_hw_id) + '｜' + f.title" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="diagLoading" :disabled="!fbId" @click="doDiagnose">
            <el-icon><Cpu /></el-icon> AI 诊断
          </el-button>
        </el-form-item>
      </el-form>

      <!-- 选中反馈详情 -->
      <el-card v-if="selFb" shadow="never" class="fb-card">
        <div class="fb-title">{{ selFb.title }}</div>
        <div class="fb-meta">
          <el-tag size="small">{{ selFb.intersection || '未指定路口' }}</el-tag>
          <el-tag size="small" type="info">设备 #{{ selFb.device_hw_id || '-' }}</el-tag>
          <el-tag size="small" :type="fbStatusType(selFb.status)">{{ fbStatusLabel(selFb.status) }}</el-tag>
          <span class="fb-reporter">反馈人：{{ selFb.reporter || '-' }}</span>
        </div>
        <div class="fb-content">{{ selFb.content || '（无文字描述）' }}</div>
      </el-card>

      <!-- 诊断结果 -->
      <div v-if="result" class="result-area">
        <div class="res-head">
          <el-icon><Cpu /></el-icon> 诊断结果
          <el-tag size="small" :type="result.source.includes('LLM') ? 'success' : 'warning'" style="margin-left:8px">
            {{ result.source }}
          </el-tag>
          <span v-if="result.tokens_used" class="tok">消耗 {{ result.tokens_used }} token</span>
        </div>
        <el-row :gutter="16">
          <el-col :span="12">
            <div class="res-block">
              <div class="rb-label">诊断结论</div>
              <div class="rb-text">{{ result.summary }}</div>
            </div>
          </el-col>
          <el-col :span="12">
            <div class="res-block">
              <div class="rb-label">成因分析</div>
              <div class="rb-text">{{ result.cause }}</div>
            </div>
          </el-col>
        </el-row>
        <div class="res-block full">
          <div class="rb-label">解决方案 / 建议</div>
          <div class="rb-text sol">{{ result.solution }}</div>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Cpu, MagicStick } from '@element-plus/icons-vue'
import { getFeedbacks } from '@/api/feedback'
import { diagnoseFeedback } from '@/api/ai'

const feedbacks = ref<any[]>([])
const fbLoading = ref(false)
const fbId = ref<number | undefined>()
const diagLoading = ref(false)
const result = ref<any>(null)

const selFb = computed(() => feedbacks.value.find((f) => f.id === fbId.value))

function fbStatusType(s: string) {
  return { open: 'danger', processing: 'warning', resolved: 'success', closed: 'info' }[s] || 'info'
}
function fbStatusLabel(s: string) {
  return { open: '待处理', processing: '处理中', resolved: '已解决', closed: '已关闭' }[s] || s
}

async function loadFeedbacks() {
  fbLoading.value = true
  try {
    const res = await getFeedbacks({ page_size: 100 })
    feedbacks.value = res.data?.list || []
  } catch { /* 忽略 */ }
  finally { fbLoading.value = false }
}

async function doDiagnose() {
  if (!fbId.value) return
  diagLoading.value = true
  result.value = null
  try {
    const res = await diagnoseFeedback(fbId.value)
    result.value = res.data?.result || null
    if (!result.value) ElMessage.warning('未返回诊断结果')
  } catch {
    ElMessage.error('诊断失败（可能额度不足）')
  } finally {
    diagLoading.value = false
  }
}

onMounted(loadFeedbacks)
</script>

<style scoped>
.head-bar { display: flex; align-items: baseline; gap: 12px; margin-bottom: 12px; }
.title { display: flex; align-items: center; gap: 8px; font-weight: 600; font-size: 16px; }
.sub { color: #909399; font-size: 12px; }
.pick-form { margin-bottom: 8px; }
.fb-card { margin-bottom: 16px; border-left: 4px solid #409eff; }
.fb-title { font-size: 15px; font-weight: 600; }
.fb-meta { margin: 8px 0; display: flex; gap: 8px; align-items: center; }
.fb-reporter { color: #909399; font-size: 12px; }
.fb-content { color: #606266; white-space: pre-wrap; line-height: 1.7; background: #f8f9fb; padding: 10px; border-radius: 6px; }
.result-area { margin-top: 8px; }
.res-head { font-weight: 600; margin-bottom: 12px; display: flex; align-items: center; gap: 4px; }
.tok { font-size: 12px; color: #909399; margin-left: 8px; font-weight: normal; }
.res-block { background: #f8f9fb; border-radius: 8px; padding: 14px; margin-bottom: 12px; height: 100%; }
.rb-label { font-size: 13px; font-weight: 600; color: #409eff; margin-bottom: 8px; }
.rb-text { font-size: 13px; color: #303133; line-height: 1.7; white-space: pre-wrap; }
.rb-text.sol { color: #1f7a3d; }
.res-block.full { height: auto; }
</style>
