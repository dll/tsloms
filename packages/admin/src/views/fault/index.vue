<template>
  <div class="fault-page">
    <!-- 搜索栏 -->
    <el-card shadow="never" class="search-card">
      <el-form :inline="true" :model="searchForm" @submit.prevent="handleSearch">
        <el-form-item label="设备ID">
          <el-input
            v-model="searchForm.device_hw_id"
            placeholder="请输入设备硬件ID"
            clearable
            style="width: 180px"
          />
        </el-form-item>
        <el-form-item label="故障状态">
          <el-select v-model="searchForm.status" placeholder="全部" clearable style="width: 120px">
            <el-option label="活跃" value="active" />
            <el-option label="已解决" value="resolved" />
          </el-select>
        </el-form-item>
        <el-form-item label="故障类型">
          <el-select v-model="searchForm.fault_type" placeholder="全部" clearable style="width: 150px">
            <el-option label="灯灭" value="lamp_off" />
            <el-option label="异常同亮" value="abnormal_on" />
            <el-option label="亮灯超时" value="timeout" />
            <el-option label="缺亮" value="dim" />
            <el-option label="断电" value="power_loss" />
          </el-select>
        </el-form-item>
        <el-form-item label="故障级别">
          <el-select v-model="searchForm.fault_level" placeholder="全部" clearable style="width: 120px">
            <el-option label="严重" value="critical" />
            <el-option label="一般" value="normal" />
          </el-select>
        </el-form-item>
        <el-form-item label="日期范围">
          <el-date-picker
            v-model="dateRange"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            value-format="YYYY-MM-DD"
            style="width: 260px"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">搜索</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 故障列表表格 -->
    <el-card shadow="never" class="table-card">
      <el-table :data="tableData" v-loading="loading" border stripe style="width: 100%">
        <el-table-column prop="device_hw_id" label="设备ID" width="160" align="center" />
        <el-table-column label="故障类型" width="150" align="center">
          <template #default="{ row }">
            {{ errCodeLabel(row.err_code) }}
          </template>
        </el-table-column>
        <el-table-column label="故障级别" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.fault_level === 'critical' ? 'danger' : 'warning'" size="small">
              {{ row.fault_level === 'critical' ? '严重' : '一般' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="err_code" label="错误码" width="90" align="center" />
        <el-table-column label="灯组状态" width="110" align="center">
          <template #default="{ row }">
            {{ ledStateLabel(row.led_state) }}
          </template>
        </el-table-column>
        <el-table-column label="电流值(R/Y/G)" width="160" align="center">
          <template #default="{ row }">
            <span>{{ row.current_r ?? '-' }} / {{ row.current_y ?? '-' }} / {{ row.current_g ?? '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="first_seen_at" label="首次出现" width="180" align="center" />
        <el-table-column prop="last_seen_at" label="最后出现" width="180" align="center" />
        <el-table-column label="状态" width="90" align="center" fixed="right">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'danger' : 'success'" size="small">
              {{ row.status === 'active' ? '活跃' : '已解决' }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <el-pagination
        class="pagination"
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.page_size"
        :page-sizes="[10, 20, 50, 100]"
        :total="pagination.total"
        layout="total, sizes, prev, pager, next, jumper"
        @size-change="fetchData"
        @current-change="fetchData"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Search, Refresh } from '@element-plus/icons-vue'
import { getFaults } from '@/api/fault'

// errCode → 故障中文名（依据《信号灯检测器_故障含义》DOCX）
const errCodeMap: Record<number, string> = {
  [0]: '正常',
  [-1]: '红灯周期全灭', [-2]: '黄灯周期全灭', [-3]: '绿灯周期全灭',
  [-4]: '红黄同亮', [-5]: '红绿同亮', [-6]: '黄绿同亮', [-7]: '红黄绿同亮',
  [-8]: '红灯超时', [-9]: '黄灯超时', [-10]: '绿灯超时',
  [-11]: '红灯缺亮', [-12]: '黄灯缺亮', [-13]: '绿灯缺亮', [-14]: '断电',
}
function errCodeLabel(code: number): string {
  return errCodeMap[code] ?? `未知(${code})`
}

// ledState → 灯组状态中文名
const ledStateMap: Record<number, string> = { 0: '红灯', 1: '黄灯', 2: '绿灯', [-1]: '未知' }
function ledStateLabel(state: number): string {
  return ledStateMap[state] ?? `未知(${state})`
}

// 搜索表单
const searchForm = reactive({
  device_hw_id: '',
  status: '',
  fault_type: '',
  fault_level: '',
})

// 日期范围
const dateRange = ref<[string, string] | null>(null)

// 表格数据
const loading = ref(false)
const tableData = ref<Record<string, any>[]>([])

// 分页配置
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
})

// 获取故障列表
async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: pagination.page,
      page_size: pagination.page_size,
      device_hw_id: searchForm.device_hw_id || undefined,
      status: searchForm.status || undefined,
      fault_type: searchForm.fault_type || undefined,
      fault_level: searchForm.fault_level || undefined,
    }
    if (dateRange.value && dateRange.value.length === 2) {
      params.start_date = dateRange.value[0]
      params.end_date = dateRange.value[1]
    }
    const res = await getFaults(params)
    tableData.value = res.data?.list || []
    pagination.total = res.data?.total || 0
  } catch {
    // 请求失败忽略
  } finally {
    loading.value = false
  }
}

// 搜索
function handleSearch() {
  pagination.page = 1
  fetchData()
}

// 重置搜索
function handleReset() {
  searchForm.device_hw_id = ''
  searchForm.status = ''
  searchForm.fault_type = ''
  searchForm.fault_level = ''
  dateRange.value = null
  pagination.page = 1
  fetchData()
}

onMounted(() => {
  fetchData()
})
</script>

<style scoped>
.search-card {
  margin-bottom: 16px;
}

.table-card {
  border-radius: 4px;
}

.pagination {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
</style>
