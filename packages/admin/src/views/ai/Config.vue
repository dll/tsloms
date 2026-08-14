<template>
  <div class="ai-config-page">
    <el-row :gutter="16">
      <!-- 配置 -->
      <el-col :span="10">
        <el-card shadow="never">
          <template #header><span>AI 配置（额度控制）</span></template>
          <el-form :model="cfg" label-width="110px" v-loading="loading">
            <el-form-item label="启用">
              <el-switch v-model="cfg.enabled" />
            </el-form-item>
            <el-form-item label="提供商">
              <el-select v-model="cfg.provider" style="width: 100%">
                <el-option label="智谱 GLM" value="zhipu" />
                <el-option label="DeepSeek" value="deepseek" />
              </el-select>
            </el-form-item>
            <el-form-item label="文本模型">
              <el-input v-model="cfg.text_model" placeholder="glm-4-flash" />
            </el-form-item>
            <el-form-item label="多模态模型">
              <el-input v-model="cfg.vision_model" placeholder="glm-4v" />
            </el-form-item>
            <el-form-item label="API Key">
              <el-input v-model="cfg.api_key" type="password" show-password
                        :placeholder="cfg.has_key ? '已配置（' + cfg.api_key_masked + '），留空保持不变' : '输入智谱/DeepSeek API Key'" />
            </el-form-item>
            <el-form-item label="每日 Token 上限">
              <el-input-number v-model="cfg.day_token_limit" :min="1000" :step="100000" style="width: 100%" />
            </el-form-item>
            <el-form-item label="每日调用上限">
              <el-input-number v-model="cfg.day_call_limit" :min="10" :step="50" style="width: 100%" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="saving" @click="saveCfg">保存配置</el-button>
              <el-button v-if="isAdmin" type="danger" plain @click="doReset">重置今日额度</el-button>
            </el-form-item>
            <el-divider />
            <div class="quota-view">
              <div class="qv-row"><span>今日已用 Token</span><b>{{ myUsage.today_tokens || 0 }}</b></div>
              <div class="qv-row"><span>今日调用次数</span><b>{{ myUsage.today_calls || 0 }}</b></div>
            </div>
          </el-form>
        </el-card>
      </el-col>

      <!-- 使用流水 -->
      <el-col :span="14">
        <el-card shadow="never">
          <template #header>
            <div class="log-head">
              <span>AI 调用额度流水（近 {{ logs.length }} 条）</span>
              <el-input v-model="userKw" placeholder="按用户名过滤" size="small" clearable style="width: 160px" @input="loadLogs" />
            </div>
          </template>
          <el-table :data="logs" v-loading="logLoading" size="small" border stripe style="width: 100%">
            <el-table-column prop="user" label="用户" width="90" />
            <el-table-column prop="action" label="功能" width="90" />
            <el-table-column prop="model" label="模型" width="110" />
            <el-table-column prop="tokens" label="Token" width="80" align="center" />
            <el-table-column label="状态" width="70" align="center">
              <template #default="{ row }"><el-tag :type="row.ok ? 'success' : 'danger'" size="small">{{ row.ok ? '成功' : '失败' }}</el-tag></template>
            </el-table-column>
            <el-table-column prop="created_at" label="时间" width="140" />
            <el-table-column prop="error" label="错误" show-overflow-tooltip min-width="100" />
          </el-table>
          <el-empty v-if="!logLoading && logs.length === 0" description="暂无调用记录" :image-size="60" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getAIConfig, updateAIConfig, getMyAIUsage, getAIUsageLogs, resetAIUsage } from '@/api/ai'
import { useAuthStore } from '@/store/auth'

const authStore = useAuthStore()
const isAdmin = computed(() => authStore.user?.role === 'admin')

const loading = ref(false)
const saving = ref(false)
const cfg = reactive({
  enabled: true, provider: 'zhipu', text_model: '', vision_model: '',
  api_key: '', api_key_masked: '', has_key: false,
  day_token_limit: 1000000, day_call_limit: 200,
})

const myUsage = ref<Record<string, any>>({})
const logLoading = ref(false)
const logs = ref<any[]>([])
const userKw = ref('')

async function loadCfg() {
  loading.value = true
  try {
    const d = (await getAIConfig()).data || {}
    Object.assign(cfg, {
      enabled: d.enabled, provider: d.provider, text_model: d.text_model,
      vision_model: d.vision_model, api_key_masked: d.api_key_masked,
      has_key: d.has_key, day_token_limit: d.day_token_limit, day_call_limit: d.day_call_limit,
      api_key: '',
    })
    const u = (await getMyAIUsage()).data || {}
    myUsage.value = u
  } finally { loading.value = false }
}

async function saveCfg() {
  saving.value = true
  try {
    await updateAIConfig({ ...cfg, api_key: cfg.api_key })
    ElMessage.success('AI 配置已保存')
    loadCfg()
  } catch { /* 后端提示 */ }
  finally { saving.value = false }
}

async function loadLogs() {
  logLoading.value = true
  try {
    const d = (await getAIUsageLogs(userKw.value ? { username: userKw.value } : {})).data || {}
    logs.value = d.list || []
  } finally { logLoading.value = false }
}

async function doReset() {
  try {
    await ElMessageBox.confirm('确认重置今日全部 AI 额度使用记录？', '提示', { type: 'warning' })
    await resetAIUsage()
    ElMessage.success('已重置')
    loadCfg(); loadLogs()
  } catch { /* 取消 */ }
}

onMounted(() => { loadCfg(); loadLogs() })
</script>

<style scoped>
.log-head { display: flex; justify-content: space-between; align-items: center; }
.quota-view { display: flex; flex-direction: column; gap: 8px; }
.qv-row { display: flex; justify-content: space-between; color: #606266; font-size: 13px; }
.qv-row b { color: #409eff; font-size: 15px; }
</style>
