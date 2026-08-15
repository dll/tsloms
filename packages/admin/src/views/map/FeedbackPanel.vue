<template>
  <div class="feedback-panel">
    <!-- 统计卡片 -->
    <el-row :gutter="12" class="stat-row">
      <el-col :span="5" v-for="s in statCards" :key="s.key">
        <el-card shadow="never" class="stat-card" @click="setStatusFilter(s.key)">
          <div class="stat-num" :style="{ color: s.color }">{{ statMap[s.key] || 0 }}</div>
          <div class="stat-label" :class="{ active: statusFilter === s.key }">{{ s.label }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 提交反馈 -->
    <el-card shadow="never" class="submit-card">
      <template #header><span>提交问题反馈</span></template>
      <el-form :model="form" label-width="90px" style="max-width: 720px">
        <el-form-item label="关联设备" required>
          <el-select v-model="form.device_hw_id" filterable placeholder="选择关联设备（必选）" style="width: 100%" @change="onDeviceChange">
            <el-option v-for="d in devices" :key="'h'+d.hw_id" :label="(d.intersection || '#'+d.hw_id) + ' (#'+d.hw_id+')'" :value="d.hw_id" />
          </el-select>
        </el-form-item>
        <el-form-item label="路口" v-if="form.intersection">
          <el-input :model-value="form.intersection" disabled placeholder="由设备自动带出" />
        </el-form-item>
        <el-form-item label="反馈标题" required>
          <el-input v-model="form.title" placeholder="如：人民路口红灯异常" />
        </el-form-item>
        <el-form-item label="反馈内容">
          <el-input v-model="form.content" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="反馈人/联系">
          <el-input v-model="form.reporter" placeholder="反馈人" style="width: 45%" />
          <el-input v-model="form.contact" placeholder="联系方式" style="width: 45%; margin-left: 8px" />
        </el-form-item>
        <el-form-item>
          <el-button v-if="canEdit" type="primary" :loading="submitting" @click="submit">提交</el-button>
          <span v-else class="tip">仅运维/管理员可提交反馈</span>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 反馈列表 -->
    <el-card shadow="never">
      <template #header>
        <div class="list-header">
          <span>反馈记录（{{ total }}）</span>
          <div class="list-filters">
            <el-select v-model="statusFilter" placeholder="全部状态" clearable size="small" style="width: 120px" @change="handleSearch">
              <el-option label="待处理" value="open" />
              <el-option label="处理中" value="processing" />
              <el-option label="已解决" value="resolved" />
              <el-option label="已关闭" value="closed" />
            </el-select>
            <el-date-picker
              v-model="dateRange" type="daterange" range-separator="至" size="small"
              start-placeholder="开始" end-placeholder="结束" value-format="YYYY-MM-DD"
              style="width: 240px" @change="handleSearch"
            />
          </div>
        </div>
      </template>
      <el-table :data="feedbacks" v-loading="loading" border stripe style="width: 100%">
        <el-table-column prop="id" label="ID" width="60" align="center" sortable />
        <el-table-column prop="title" label="标题" min-width="140" />
        <el-table-column prop="intersection" label="路口" width="130" />
        <el-table-column label="设备" width="100" align="center">
          <template #default="{ row }">#{{ row.device_hw_id }}</template>
        </el-table-column>
        <el-table-column prop="reporter" label="反馈人" width="90" />
        <el-table-column label="状态" width="100" align="center" sortable :sort-by="'status'">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" width="170" align="center" sortable />
        <el-table-column label="操作" width="140" align="center">
          <template #default="{ row }">
            <el-select v-if="canEdit" :model-value="row.status" size="small" @change="(v:any)=>updateStatus(row, v)">
              <el-option label="待处理" value="open" />
              <el-option label="处理中" value="processing" />
              <el-option label="已解决" value="resolved" />
              <el-option label="已关闭" value="closed" />
            </el-select>
            <span v-else>{{ statusLabel(row.status) }}</span>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination
        class="pagination" v-model:current-page="pagination.page"
        v-model:page-size="pagination.page_size" :page-sizes="[10,20,50,100]" :total="total"
        layout="total, sizes, prev, pager, next, jumper" @size-change="load" @current-change="load"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getFeedbacks, createFeedback, updateFeedback, type Feedback } from '@/api/feedback'
import { getAllDevices } from '@/api/map'
import { useAuthStore } from '@/store/auth'

const authStore = useAuthStore()
const canEdit = computed(() => { const r = authStore.user?.role; return r === 'admin' || r === 'operator' })

const devices = ref<any[]>([])
const feedbacks = ref<Feedback[]>([])
const loading = ref(false)
const submitting = ref(false)
const statusFilter = ref('')
const dateRange = ref<[string, string] | null>(null)
const total = ref(0)
const pagination = reactive({ page: 1, page_size: 20 })

const statCards = [
  { key: '', label: '全部', color: '#409EFF' },
  { key: 'open', label: '待处理', color: '#F56C6C' },
  { key: 'processing', label: '处理中', color: '#E6A23C' },
  { key: 'resolved', label: '已解决', color: '#67C23A' },
  { key: 'closed', label: '已关闭', color: '#909399' },
]
const statMap = ref<Record<string, number>>({})

const form = reactive({ device_hw_id: undefined as number | undefined, intersection: '', title: '', content: '', reporter: '', contact: '' })
const statusLabel = (s: string) => ({ open: '待处理', processing: '处理中', resolved: '已解决', closed: '已关闭' } as Record<string, string>)[s] || s
const statusTag = (s: string) => ({ open: 'danger', processing: 'warning', resolved: 'success', closed: 'info' } as Record<string, string>)[s] || 'info'

function onDeviceChange(id: number) {
  const d = devices.value.find((x) => x.hw_id === id)
  form.intersection = d?.intersection || ''
}

async function loadStats() {
  statMap.value = {}
  for (const s of ['', 'open', 'processing', 'resolved', 'closed']) {
    try {
      const res = await getFeedbacks({ page_size: 1, status: s || undefined })
      statMap.value[s] = res.data?.total || 0
    } catch { /* 忽略 */ }
  }
}
function setStatusFilter(key: string) {
  statusFilter.value = key
  handleSearch()
}

async function load() {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: pagination.page, page_size: pagination.page_size,
      status: statusFilter.value || undefined,
    }
    if (dateRange.value && dateRange.value.length === 2) {
      params.start_time = dateRange.value[0]
      params.end_time = dateRange.value[1]
    }
    const res = await getFeedbacks(params)
    feedbacks.value = res.data?.list || []
    total.value = res.data?.total || 0
  } catch { /* 忽略 */ } finally { loading.value = false }
}
function handleSearch() { pagination.page = 1; load() }

async function submit() {
  if (!form.device_hw_id) { ElMessage.warning('请选择关联设备（所有问题都应关联设备）'); return }
  if (!form.title.trim()) { ElMessage.warning('请输入反馈标题'); return }
  submitting.value = true
  try {
    await createFeedback({
      device_hw_id: form.device_hw_id,
      intersection: form.intersection,
      title: form.title, content: form.content, reporter: form.reporter, contact: form.contact,
    })
    ElMessage.success('反馈已提交')
    Object.assign(form, { device_hw_id: undefined, intersection: '', title: '', content: '', reporter: '', contact: '' })
    load(); loadStats()
  } catch { /* 忽略 */ } finally { submitting.value = false }
}

async function updateStatus(row: Feedback, status: string) {
  try { await updateFeedback(row.id, { status }); row.status = status as any; ElMessage.success('已更新'); loadStats() } catch { /* 忽略 */ }
}

onMounted(async () => {
  const d = await getAllDevices()
  devices.value = d.data?.list || []
  if (!authStore.user) { try { await authStore.fetchUserInfo() } catch { /* 忽略 */ } }
  if (authStore.user) {
    form.reporter = authStore.user.username || form.reporter
    form.contact = authStore.user.phone || form.contact
  }
  await load(); await loadStats()
})
</script>

<style scoped>
.feedback-panel { padding: 12px; }
.stat-row { margin-bottom: 12px; }
.stat-card { text-align: center; cursor: pointer; }
.stat-num { font-size: 24px; font-weight: 700; line-height: 1.2; }
.stat-label { color: #909399; font-size: 13px; margin-top: 4px; }
.stat-label.active { color: #409eff; font-weight: 600; }
.submit-card { margin-bottom: 16px; }
.list-header { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; }
.list-filters { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.pagination { margin-top: 12px; display: flex; justify-content: flex-end; }
.tip { color: #909399; font-size: 12px; }
</style>
