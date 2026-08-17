<template>
  <div class="warning-page">
    <!-- 搜索栏 -->
    <el-card shadow="never" class="filter-card">
      <el-form :inline="true" @submit.prevent="handleSearch">
        <el-form-item label="级别">
          <el-select v-model="query.level" placeholder="全部" clearable style="width: 120px">
            <el-option label="严重" value="critical" />
            <el-option label="警告" value="warning" />
            <el-option label="提示" value="info" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" placeholder="全部" clearable style="width: 130px">
            <el-option label="待处理" value="pending" />
            <el-option label="已忽略" value="ignored" />
            <el-option label="已转工单" value="workorder" />
          </el-select>
        </el-form-item>
        <el-form-item label="设备">
          <el-input v-model="query.device_hw_id" placeholder="设备ID" clearable style="width: 140px" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">搜索</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
          <el-button :icon="CircleCheck" @click="handleAutoIgnore" :loading="busy">自动忽略</el-button>
          <el-button :icon="Download" @click="handleExport" :disabled="!selection.length && rows.length === 0">导出</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 预警列表 -->
    <el-card shadow="never">
      <div class="toolbar">
        <el-button type="danger" plain size="small" :disabled="!selection.length" :loading="busy" @click="handleBatchIgnore">
          批量忽略({{ selection.length }})
        </el-button>
      </div>
      <el-table :data="rows" v-loading="loading" border stripe @selection-change="(s: any) => (selection = s)">
        <el-table-column type="selection" width="46" />
        <el-table-column prop="id" label="ID" width="70" align="center" />
        <el-table-column prop="device_hw_id" label="设备ID" width="110" align="center" />
        <el-table-column prop="crossing_name" label="路口" width="160" show-overflow-tooltip />
        <el-table-column prop="warning_label" label="类型" width="130" show-overflow-tooltip />
        <el-table-column label="级别" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="levelTag(row.level)" size="small">{{ levelLabel(row.level) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="内容" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">{{ row.warning_label || row.remark || '-' }}</template>
        </el-table-column>
        <el-table-column prop="occurred_at" label="发生时间" width="170" />
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">{{ statusLabel(row.status, row.deal_state) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="170" align="center" fixed="right">
          <template #default="{ row }">
            <el-button v-if="!isDealt(row)" link type="primary" size="small" @click="handleToWorkorder(row)">转工单</el-button>
            <el-button v-if="!isDealt(row)" link type="warning" size="small" @click="handleIgnore(row)">忽略</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination
        v-model:current-page="query.page"
        :page-size="query.page_size"
        :total="total"
        layout="total, prev, pager, next"
        style="margin-top: 12px; justify-content: flex-end"
        @current-change="fetchData"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Refresh, Download, CircleCheck } from '@element-plus/icons-vue'
import {
  getWarnings, ignoreWarning, batchIgnoreWarnings, warningToWorkOrder, autoIgnoreWarnings, exportWarnings,
  type WarningItem,
} from '@/api/warning'

const loading = ref(false)
const busy = ref(false)
const rows = ref<WarningItem[]>([])
const total = ref(0)
const selection = ref<WarningItem[]>([])

const query = reactive<any>({ page: 1, page_size: 20, status: '', level: '', device_hw_id: '' })

function levelLabel(l: string) {
  return { critical: '严重', warning: '警告', info: '提示' }[l as string] || l || '-'
}
function levelTag(l: string) {
  return { critical: 'danger', warning: 'warning', info: 'info' }[l as string] || 'info'
}
function isDealt(row: any): boolean {
  const s = String(row.status || row.deal_state || '').toUpperCase()
  return s === 'IGNORE' || s === 'IGNORED' || s === 'WORKORDER' || s === 'DONE' || s === 'RESOLVED'
}
function statusLabel(s?: string, deal?: string) {
  // 兼容后端 status / deal_state 组合；无法识别时返回原始值
  if (deal === 'IGNORE' || s === 'ignored' || s === 'IGNORE') return '已忽略'
  if (deal === 'WORKORDER' || s === 'workorder' || s === 'WORKORDER') return '已转工单'
  if (s === 'pending' || s === 'PENDING' || s === '待处理') return '待处理'
  return s || '-' }

async function fetchData() {
  loading.value = true
  try {
    const res = await getWarnings(query)
    rows.value = res.data?.list || []
    total.value = res.data?.total || 0
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  query.page = 1
  fetchData()
}
function handleReset() {
  Object.assign(query, { page: 1, status: '', level: '', device_hw_id: '' })
  fetchData()
}

async function handleIgnore(row: WarningItem) {
  await ignoreWarning(row.id)
  ElMessage.success('已忽略')
  fetchData()
}

async function handleBatchIgnore() {
  if (!selection.value.length) return
  await ElMessageBox.confirm(`确定忽略选中的 ${selection.value.length} 条预警？`, '提示', { type: 'warning' })
  busy.value = true
  try {
    await batchIgnoreWarnings(selection.value.map((r) => Number(r.id)))
    ElMessage.success('批量忽略完成')
    selection.value = []
    fetchData()
  } finally {
    busy.value = false
  }
}

async function handleToWorkorder(row: WarningItem) {
  // 转工单：填写备注后生成维修工单（参考项目 a 的 flowWorkOrder 填 remark）
  let remark = ''
  try {
    const r = await ElMessageBox.prompt('填写转工单备注（故障描述/位置等）', '转为维修工单', {
      confirmButtonText: '确认转单', cancelButtonText: '取消',
      inputType: 'textarea', inputPlaceholder: '例如：长江中路 3 号灯断电',
      inputValidator: (v) => (!!v || '请填写备注以转工单'),
    })
    remark = r.value || ''
  } catch { return } // 用户取消
  busy.value = true
  try {
    await warningToWorkOrder(row.id, remark)
    ElMessage.success('已转工单')
    fetchData()
  } finally {
    busy.value = false
  }
}

async function handleAutoIgnore() {
  busy.value = true
  try {
    const res = await autoIgnoreWarnings()
    ElMessage.success(`自动忽略完成，忽略 ${(res.data?.ignored ?? 0)} 条`)
    fetchData()
  } finally {
    busy.value = false
  }
}

async function handleExport() {
  const res = await exportWarnings(query)
  ElMessage.success(`导出 ${(res.data?.count ?? 0)} 条（详见后端日志）`)
}

onMounted(fetchData)
</script>
