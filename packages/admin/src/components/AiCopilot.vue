<template>
  <div class="ai-copilot">
    <div class="copilot-head">
      <span class="copilot-title">🤖 AI 助手</span>
      <el-button v-if="!loading && !advice && !error" type="primary" link size="small" :disabled="generating" @click="load">
        生成建议
      </el-button>
      <el-button v-if="advice && !generated" type="primary" link size="small" @click="fill">填入表单</el-button>
    </div>

    <el-skeleton v-if="loading || generating" :rows="3" animated style="padding: 4px 0" />

    <div v-else-if="error" class="copilot-empty">
      <el-text type="danger" size="small">{{ error }}</el-text>
    </div>

    <div v-else-if="advice" class="copilot-body">
      <div v-if="advice.summary || advice.priority" class="copilot-line">
        <el-tag v-if="advice.priority_text" :type="priorityTag(advice.priority)" size="small" effect="dark">{{ advice.priority_text }}</el-tag>
        <span v-if="advice.summary" class="copilot-summary">{{ advice.summary }}</span>
      </div>
      <div v-if="advice.plan" class="copilot-sec">
        <div class="copilot-sec-title">预案</div>
        <div class="copilot-text">{{ advice.plan }}</div>
      </div>
      <div v-if="advice.steps && advice.steps.length" class="copilot-sec">
        <div class="copilot-sec-title">处理步骤</div>
        <ol class="copilot-steps">
          <li v-for="(s, i) in advice.steps" :key="i">{{ s }}</li>
        </ol>
      </div>
      <div v-if="advice.hints && advice.hints.length" class="copilot-sec">
        <div class="copilot-sec-title">填写/配置建议</div>
        <ol class="copilot-steps">
          <li v-for="(s, i) in advice.hints" :key="i">{{ s }}</li>
        </ol>
      </div>
      <div v-if="advice.suggestions && advice.suggestions.length" class="copilot-sec">
        <div class="copilot-sec-title">改进建议</div>
        <ol class="copilot-steps">
          <li v-for="(s, i) in advice.suggestions" :key="i">{{ s }}</li>
        </ol>
      </div>
      <div v-if="advice.checks && advice.checks.length" class="copilot-sec">
        <div class="copilot-sec-title">校验提醒</div>
        <el-tag v-for="(p, i) in advice.checks" :key="i" size="small" type="danger" effect="plain" style="margin-right: 6px; margin-bottom: 4px">{{ p }}</el-tag>
      </div>
      <div v-if="advice.issues && advice.issues.length" class="copilot-sec">
        <div class="copilot-sec-title">潜在问题</div>
        <el-tag v-for="(p, i) in advice.issues" :key="i" size="small" type="warning" effect="plain" style="margin-right: 6px; margin-bottom: 4px">{{ p }}</el-tag>
      </div>
      <div v-if="advice.repairer_hint || advice.supplier_hint" class="copilot-sec">
        <div class="copilot-sec-title">推荐</div>
        <div class="copilot-text">{{ advice.repairer_hint || advice.supplier_hint }}</div>
      </div>
      <div v-if="advice.root_cause" class="copilot-sec">
        <div class="copilot-sec-title">根因预判</div>
        <div class="copilot-text">{{ advice.root_cause }}</div>
      </div>
      <div v-if="advice.parts && advice.parts.length" class="copilot-sec">
        <div class="copilot-sec-title">建议备件</div>
        <div class="copilot-parts">
          <el-tag v-for="(p, i) in advice.parts" :key="i" size="small" type="warning" effect="plain" style="margin-right: 6px; margin-bottom: 4px">{{ p }}</el-tag>
        </div>
      </div>
      <div v-if="advice.source" class="copilot-src">来源：{{ advice.source === 'LLM' ? 'AI 生成' : '规则引擎' }}</div>
    </div>

    <div v-else class="copilot-empty">
      <el-text type="info" size="small">AI 根据当前业务数据给出辅助建议，可一键填入表单，辅助快速处理。</el-text>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

// 通用 AI 辅助：接受任意建议结构（故障/工单/设备/采购），字段均可选按需展示
interface Advice {
  summary?: string
  priority?: string
  priority_text?: string
  plan?: string
  root_cause?: string
  steps?: string[]
  parts?: string[]
  hints?: string[]
  suggestions?: string[]
  checks?: string[]
  issues?: string[]
  repairer_hint?: string
  supplier_hint?: string
  content?: string
  source?: string
  tokens_used?: number
}

const props = defineProps<{
  loadFn: (() => Promise<{ data?: any }>)
  fillFn?: (advice: any) => void
}>()

const loading = ref(false)
const generating = ref(false)
const advice = ref<Advice | null>(null)
const error = ref('')
const generated = ref(false)

async function load() {
  loading.value = true
  generating.value = true
  error.value = ''
  try {
    const res = await props.loadFn()
    advice.value = res?.data?.result || null
    generated.value = !!advice.value
    if (!advice.value) error.value = '未获取到建议'
  } catch (e: any) {
    error.value = e?.message || '建议生成失败'
  } finally {
    loading.value = false
    generating.value = false
  }
}

function fill() {
  const a = advice.value
  if (a && props.fillFn) {
    props.fillFn(a)
    generated.value = true
  }
}

function priorityTag(p?: string) {
  if (p === 'P0') return 'danger'
  if (p === 'P1') return 'warning'
  if (p === 'P2') return 'primary'
  return 'info'
}
</script>

<style scoped>
.ai-copilot {
  border: 1px dashed #409eff;
  border-radius: 6px;
  padding: 10px 12px;
  margin-bottom: 12px;
  background: #f7faff;
}
.copilot-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}
.copilot-title {
  font-weight: 600;
  color: #409eff;
}
.copilot-body {
  font-size: 13px;
}
.copilot-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.copilot-summary { color: #303133; }
.copilot-sec { margin-top: 8px; }
.copilot-sec-title { font-weight: 600; color: #606266; font-size: 12px; margin-bottom: 2px; }
.copilot-text { color: #303133; line-height: 1.6; white-space: pre-wrap; }
.copilot-steps { margin: 0; padding-left: 18px; color: #303133; line-height: 1.6; }
.copilot-parts { margin-top: 2px; }
.copilot-src { margin-top: 6px; font-size: 12px; color: #c0c4cc; }
.copilot-empty { font-size: 13px; padding: 4px 0; }
</style>
