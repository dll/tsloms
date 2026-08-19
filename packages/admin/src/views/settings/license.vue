<template>
  <div class="license-page">
    <el-card shadow="never">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span>授权与试用管理（超级管理员）</span>
          <el-button size="small" :icon="Refresh" @click="fetchStatus">刷新</el-button>
        </div>
      </template>
      <div class="hint">核心功能试用 100 天、可选功能试用 30 天；到期锁定，须经超级管理员授权解锁（一键或授权码）。</div>

      <el-table :data="items" v-loading="loading" border stripe style="margin-top:12px">
        <el-table-column prop="name" label="功能模块" width="180" />
        <el-table-column label="类型" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.core ? 'primary' : 'info'" size="small">{{ row.core ? '核心' : '可选' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110" align="center">
          <template #default="{ row }">
            <el-tag :type="stateTag(row.state)" size="small">{{ stateLabel(row.state) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="试用期至" width="120" align="center">
          <template #default="{ row }">{{ row.trial_expiry || '-' }}</template>
        </el-table-column>
        <el-table-column label="剩余天数" width="100" align="center">
          <template #default="{ row }">
            <span :style="{ color: row.remain_days != null && row.remain_days <= 7 ? '#f56c6c' : '' }">
              {{ row.remain_days != null ? row.remain_days + ' 天' : '-' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="操作" min-width="240" align="center">
          <template #default="{ row }">
            <el-button v-if="row.state === 'pending'" link type="primary" size="small" @click="handleStartTrial(row)">开始试用</el-button>
            <el-button v-if="row.state === 'trial'" link type="warning" size="small" @click="handleUnlock(row)">直接解锁</el-button>
            <el-button v-if="row.state === 'expired'" link type="danger" size="small" @click="handleUnlock(row)">授权解锁</el-button>
            <el-button v-if="row.state === 'trial' || row.state === 'expired'" link type="info" size="small" @click="handleCodeUnlock(row)">输入授权码</el-button>
            <el-tag v-if="row.state === 'unlocked'" type="success" size="small" effect="plain">已解锁{{ row.unlock_type === 'author' ? '（授权码）' : '' }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 授权码输入对话框 -->
    <el-dialog v-model="codeDlg" title="输入供应方授权码" width="560px" append-to-body>
      <div class="hint">授权码由供应方离线工具生成（Ed25519 签名），粘贴到此处即可解锁对应模块；授权码被篡改/过期将验签失败。</div>
      <el-input v-model="curCode" type="textarea" :rows="3" placeholder="粘贴授权码..." />
      <template #footer>
        <el-button @click="codeDlg = false">取消</el-button>
        <el-button type="primary" :loading="busy" @click="submitCode">解锁</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { getLicenseStatus, startTrial, unlockLicense, type LicenseStatusItem } from '@/api/license'

const loading = ref(false)
const busy = ref(false)
const items = ref<LicenseStatusItem[]>([])
const codeDlg = ref(false)
const curCode = ref('')
let curKey = ''

function stateLabel(s: string) {
  return { pending: '未开始', trial: '试用中', expired: '已过期', unlocked: '已解锁' }[s] || s
}
function stateTag(s: string) {
  return { pending: 'info', trial: 'warning', expired: 'danger', unlocked: 'success' }[s] || 'info'
}

async function fetchStatus() {
  loading.value = true
  try {
    const res = await getLicenseStatus()
    items.value = res.data?.list || []
  } finally {
    loading.value = false
  }
}

async function handleStartTrial(row: LicenseStatusItem) {
  busy.value = true
  try {
    const res = await startTrial(row.key === 'core' ? 'core' : row.key)
    ElMessage.success(res.data?.message || '试用已开始')
    fetchStatus()
  } finally {
    busy.value = false
  }
}

function handleUnlock(row: LicenseStatusItem) {
  busy.value = true
  unlockLicense(row.key)
    .then((res) => { ElMessage.success(res.data?.message || '已解锁'); fetchStatus() })
    .finally(() => { busy.value = false })
}

function handleCodeUnlock(row: LicenseStatusItem) {
  curKey = row.key
  curCode.value = ''
  codeDlg.value = true
}

async function submitCode() {
  if (!curCode.value.trim()) {
    ElMessage.warning('请粘贴授权码')
    return
  }
  busy.value = true
  try {
    const res = await unlockLicense(curKey, curCode.value.trim())
    ElMessage.success(res.data?.message || '解锁成功')
    codeDlg.value = false
    fetchStatus()
  } catch {
    // 拦截器已提示授权码无效
  } finally {
    busy.value = false
  }
}

onMounted(fetchStatus)
</script>

<style scoped>
.hint {
  font-size: 13px;
  color: #909399;
  margin-bottom: 4px;
}
</style>
