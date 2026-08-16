<template>
  <div class="cases-page">
    <!-- 页头：案例库 + 训练 -->
    <div class="cases-head">
      <h3 class="cases-title"><el-icon><Collection /></el-icon> 识别案例库</h3>
      <el-button type="primary" :loading="training" @click="doTrain">训练案例库</el-button>
    </div>

    <el-card shadow="never" class="search-card">
      <el-form :inline="true" :model="searchForm" @submit.prevent="handleSearch">
        <el-form-item label="设备ID">
          <el-input v-model="searchForm.device_hw_id" placeholder="设备硬件ID" clearable style="width: 160px" />
        </el-form-item>
        <el-form-item label="故障类型">
          <el-select v-model="searchForm.fault_type" placeholder="全部" clearable style="width: 140px">
            <el-option label="灯灭" value="lamp_on_off" />
            <el-option label="异常同亮" value="abnormal_on" />
            <el-option label="亮灯超时" value="lamp_on_timeout" />
            <el-option label="缺亮" value="dim" />
            <el-option label="断电" value="power_loss" />
            <el-option label="正常(非故障)" value="normal" />
          </el-select>
        </el-form-item>
        <el-form-item label="正确性">
          <el-select v-model="searchForm.is_correct" placeholder="全部" clearable style="width: 120px">
            <el-option label="正确" value="true" />
            <el-option label="不正确" value="false" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="searchForm.status" placeholder="全部" clearable style="width: 120px">
            <el-option label="种子/自动沉淀" value="seed" />
            <el-option label="已确认" value="confirmed" />
            <el-option label="训练中" value="training" />
            <el-option label="测试集" value="test" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">搜索</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never" class="table-card">
      <el-table :data="tableData" v-loading="loading" border stripe style="width: 100%">
        <el-table-column prop="id" label="ID" width="60" align="center" />
        <el-table-column prop="device_hw_id" label="设备ID" width="110" align="center" />
        <el-table-column label="故障类型" width="130" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.expected_result==='normal'" type="info" size="small">正常(非故障)</el-tag>
            <span v-else>{{ faultTypeCN(row.fault_type || row.judged_result) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="预期结果(真值)" width="130" align="center">
          <template #default="{ row }">{{ faultTypeCN(row.expected_result) }}</template>
        </el-table-column>
        <el-table-column label="引擎判定" width="130" align="center">
          <template #default="{ row }">{{ faultTypeCN(row.judged_result) }}</template>
        </el-table-column>
        <el-table-column label="判定正确" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.is_correct ? 'success' : 'danger'" size="small">{{ row.is_correct == null ? '-' : (row.is_correct ? '正确' : '不正确') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="置信度" width="90" align="center">
          <template #default="{ row }">{{ fmtConfidence(row.judge_confidence) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="110" align="center">
          <template #default="{ row }">
            <el-tag :type="caseStatusTag(row.status)" size="small">{{ caseStatusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="研判批次" width="160" show-overflow-tooltip>
          <template #default="{ row }">{{ row.source_evaluation_id || '-' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="170" align="center">
          <template #default="{ row }">{{ fmtT(row.created_at) }}</template>
        </el-table-column>
      </el-table>

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
import { ElMessage } from 'element-plus'
import { Search, Refresh, Collection } from '@element-plus/icons-vue'
import { listFaultCases, trainFaultCases, fmtConfidence, type FaultCase } from '@/api/recognition'

const searchForm = reactive({ device_hw_id: '', fault_type: '', is_correct: '', status: '' })
const loading = ref(false)
const training = ref(false)
const tableData = ref<FaultCase[]>([])
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

const faultTypeMap: Record<string, string> = {
  normal: '正常(非故障)',
  lamp_on_off: '灯灭',
  lamp_off: '灯灭',
  abnormal_on: '异常同亮',
  lamp_on_timeout: '亮灯超时',
  timeout: '亮灯超时',
  dim: '缺亮',
  power_loss: '断电',
  lamp_red_off: '红灯灭',
  lamp_yellow_off: '黄灯灭',
  lamp_green_off: '绿灯灭',
}
function faultTypeCN(t?: string): string {
  return (t && faultTypeMap[t]) || t || '-'
}

function caseStatusLabel(s?: string): string {
  const map: Record<string, string> = { seed: '种子/自动沉淀', confirmed: '已确认', training: '训练中', test: '测试集' }
  return (s && map[s]) || s || '-'
}
function caseStatusTag(s?: string): string {
  const map: Record<string, string> = { seed: 'info', confirmed: 'success', training: 'warning', test: 'primary' }
  return (s && map[s]) || 'info'
}

function fmtT(t?: string): string {
  return t ? String(t).slice(0, 19).replace('T', ' ') : '-'
}

async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: pagination.page,
      page_size: pagination.page_size,
      device_hw_id: searchForm.device_hw_id || undefined,
      fault_type: searchForm.fault_type || undefined,
      status: searchForm.status || undefined,
    }
    if (searchForm.is_correct) params.is_correct = searchForm.is_correct
    const res = await listFaultCases(params)
    tableData.value = res.data?.list || []
    pagination.total = res.data?.total || 0
  } catch {
    // 拦截器提示
  } finally { loading.value = false }
}

function handleSearch() { pagination.page = 1; fetchData() }
function handleReset() {
  searchForm.device_hw_id = ''; searchForm.fault_type = ''; searchForm.is_correct = ''; searchForm.status = ''
  pagination.page = 1; fetchData()
}

async function doTrain() {
  training.value = true
  try {
    const res = await trainFaultCases()
    const d = res.data
    if (d && typeof d.total_cases === 'number') {
      ElMessage.success(`训练完成：正确率 ${Math.round((d.accuracy ?? 0) * 100)}%，${d.recognize_100pct ? '已达 100% 识别率' : '继续收敛中'}`)
    } else {
      ElMessage.success('训练完成')
    }
    fetchData()
  } catch {
    // 拦截器提示
  } finally { training.value = false }
}

onMounted(() => { fetchData() })
</script>

<style scoped>
.cases-title { display: flex; align-items: center; gap: 6px; margin: 0 0 14px; color: #303133; font-size: 16px; }
.cases-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
.search-card { margin-bottom: 16px; }
.table-card { border-radius: 4px; }
.pagination { margin-top: 16px; display: flex; justify-content: flex-end; }
</style>
