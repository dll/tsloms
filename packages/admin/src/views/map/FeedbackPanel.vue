<template>
  <div class="feedback-panel">
    <!-- 提交反馈 -->
    <el-card shadow="never" class="submit-card">
      <template #header><span>提交问题反馈</span></template>
      <el-form :model="form" label-width="90px" style="max-width: 720px">
        <el-form-item label="关联设备">
          <el-select v-model="form.device_hw_id" filterable clearable placeholder="可空" style="width: 100%">
            <el-option v-for="d in devices" :key="'h'+d.hw_id" :label="(d.intersection || '#'+d.hw_id)" :value="d.hw_id" />
          </el-select>
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
          <el-button type="primary" :loading="submitting" @click="submit">提交</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 反馈列表 -->
    <el-card shadow="never">
      <template #header>
        <div class="list-header">
          <span>反馈记录</span>
          <el-select v-model="statusFilter" placeholder="全部状态" clearable size="small" style="width: 120px">
            <el-option label="待处理" value="open" />
            <el-option label="处理中" value="processing" />
            <el-option label="已解决" value="resolved" />
            <el-option label="已关闭" value="closed" />
          </el-select>
        </div>
      </template>
      <el-table :data="feedbacks" v-loading="loading" border stripe style="width: 100%">
        <el-table-column prop="id" label="ID" width="60" align="center" />
        <el-table-column prop="title" label="标题" min-width="140" />
        <el-table-column prop="intersection" label="路口" width="120" />
        <el-table-column prop="reporter" label="反馈人" width="90" />
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" width="160" align="center" />
        <el-table-column label="操作" width="140" align="center">
          <template #default="{ row }">
            <el-select :model-value="row.status" size="small" @change="(v:any)=>updateStatus(row, v)">
              <el-option label="待处理" value="open" />
              <el-option label="处理中" value="processing" />
              <el-option label="已解决" value="resolved" />
              <el-option label="已关闭" value="closed" />
            </el-select>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getFeedbacks, createFeedback, updateFeedback, type Feedback } from '@/api/feedback'
import { getAllDevices } from '@/api/map'

const devices = ref<any[]>([])
const feedbacks = ref<Feedback[]>([])
const loading = ref(false)
const submitting = ref(false)
const statusFilter = ref('')

const form = reactive({ device_hw_id: undefined as number | undefined, title: '', content: '', reporter: '', contact: '' })
const statusLabel = (s: string) => ({ open: '待处理', processing: '处理中', resolved: '已解决', closed: '已关闭' } as Record<string, string>)[s] || s
const statusTag = (s: string) => ({ open: 'danger', processing: 'warning', resolved: 'success', closed: 'info' } as Record<string, string>)[s] || 'info'

async function load() {
  loading.value = true
  try {
    const res = await getFeedbacks({ page_size: 100, status: statusFilter.value || undefined })
    feedbacks.value = res.data?.list || []
  } catch { /* 忽略 */ } finally { loading.value = false }
}

async function submit() {
  if (!form.title.trim()) { ElMessage.warning('请输入反馈标题'); return }
  submitting.value = true
  try {
    await createFeedback({ device_hw_id: form.device_hw_id, title: form.title, content: form.content, reporter: form.reporter, contact: form.contact })
    ElMessage.success('反馈已提交')
    Object.assign(form, { device_hw_id: undefined, title: '', content: '', reporter: '', contact: '' })
    load()
  } catch { /* 忽略 */ } finally { submitting.value = false }
}

async function updateStatus(row: Feedback, status: string) {
  try { await updateFeedback(row.id, { status }); row.status = status as any; ElMessage.success('已更新') } catch { /* 忽略 */ }
}

onMounted(async () => {
  const d = await getAllDevices()
  devices.value = d.data?.list || []
  await load()
})
</script>

<style scoped>
.submit-card { margin-bottom: 16px; }
.list-header { display: flex; justify-content: space-between; align-items: center; }
</style>
