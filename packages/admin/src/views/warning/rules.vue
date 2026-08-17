<template>
  <div class="rules-page">
    <el-card shadow="never">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span>预警配置（忽略规则）</span>
          <el-button type="primary" size="small" :icon="Plus" @click="openEdit()">新增规则</el-button>
        </div>
      </template>

      <el-table :data="rows" v-loading="loading" border stripe>
        <el-table-column prop="id" label="ID" width="70" align="center" />
        <el-table-column prop="name" label="规则名称" width="180" />
        <el-table-column prop="crossing_name" label="路口" width="160" show-overflow-tooltip />
        <el-table-column prop="device_hw_id" label="设备ID" width="110" align="center">
          <template #default="{ row }">{{ row.device_hw_id || '-' }}</template>
        </el-table-column>
        <el-table-column prop="ignore_type" label="忽略类型" width="130" />
        <el-table-column label="生效时间段" width="170">
          <template #default="{ row }">{{ row.effect_time_start || '全天' }} ~ {{ row.effect_time_end || '全天' }}</template>
        </el-table-column>
        <el-table-column label="启用" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="160" show-overflow-tooltip />
        <el-table-column label="操作" width="140" align="center" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" size="small" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 新增/编辑对话框 -->
    <el-dialog v-model="dlgVisible" :title="form.id ? '编辑规则' : '新增规则'" width="520px" append-to-body>
      <el-form :model="form" label-width="90px">
        <el-form-item label="规则名称">
          <el-input v-model="form.name" placeholder="如：夜间停电忽略" />
        </el-form-item>
        <el-form-item label="路口">
          <el-input v-model="form.crossing_name" placeholder="路口名称（可空=全部）" />
        </el-form-item>
        <el-form-item label="设备ID">
          <el-input v-model.number="form.device_hw_id" placeholder="设备硬件ID（可空=全部）" />
        </el-form-item>
        <el-form-item label="忽略类型">
          <el-input v-model="form.ignore_type" placeholder="要忽略的预警类型（可空=全部）" />
        </el-form-item>
        <el-form-item label="生效时段">
          <el-time-picker
            v-model="timeRange"
            is-range
            range-separator="至"
            start-placeholder="开始"
            end-placeholder="结束"
            value-format="HH:mm:ss"
            format="HH:mm"
          />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlgVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { getWarningRules, createWarningRule, updateWarningRule, deleteWarningRule, type WarningRule } from '@/api/warning'

const loading = ref(false)
const saving = ref(false)
const rows = ref<WarningRule[]>([])
const dlgVisible = ref(false)
const timeRange = ref<[string, string] | null>(null)
const form = reactive<any>({
  id: 0, name: '', crossing_id: null, crossing_name: '', device_hw_id: null,
  ignore_type: '', enabled: true, effect_time_start: '', effect_time_end: '', remark: '',
})

async function fetchData() {
  loading.value = true
  try {
    const res = await getWarningRules()
    rows.value = res.data?.list || []
  } finally {
    loading.value = false
  }
}

function openEdit(row?: WarningRule) {
  Object.assign(form, {
    id: row?.id || 0, name: row?.name || '', crossing_id: row?.crossing_id || null,
    crossing_name: row?.crossing_name || '', device_hw_id: row?.device_hw_id || null,
    ignore_type: row?.ignore_type || '', enabled: row?.enabled !== false,
    effect_time_start: row?.effect_time_start || '', effect_time_end: row?.effect_time_end || '',
    remark: row?.remark || '',
  })
  timeRange.value = form.effect_time_start ? [form.effect_time_start, form.effect_time_end] : null
  dlgVisible.value = true
}

async function handleSave() {
  const payload: any = { ...form }
  if (timeRange.value) {
    payload.effect_time_start = timeRange.value[0]
    payload.effect_time_end = timeRange.value[1]
  } else {
    payload.effect_time_start = ''
    payload.effect_time_end = ''
  }
  delete payload.crossing_name
  saving.value = true
  try {
    if (form.id) {
      await updateWarningRule(form.id, payload)
    } else {
      await createWarningRule(payload)
    }
    ElMessage.success('保存成功')
    dlgVisible.value = false
    fetchData()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row: WarningRule) {
  await ElMessageBox.confirm('确定删除该规则？', '提示', { type: 'warning' })
  await deleteWarningRule(row.id)
  ElMessage.success('已删除')
  fetchData()
}

onMounted(fetchData)
</script>
