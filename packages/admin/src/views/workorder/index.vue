<template>
  <div class="workorder-page">
    <!-- 搜索栏 -->
    <el-card shadow="never" class="search-card">
      <el-form :inline="true" :model="searchForm" @submit.prevent="handleSearch">
        <el-form-item label="工单编号">
          <el-input
            v-model="searchForm.order_no"
            placeholder="请输入工单编号"
            clearable
            style="width: 200px"
          />
        </el-form-item>
        <el-form-item label="设备ID">
          <el-input
            v-model="searchForm.device_hw_id"
            placeholder="请输入设备硬件ID"
            clearable
            style="width: 180px"
          />
        </el-form-item>
        <el-form-item label="工单状态">
          <el-select v-model="searchForm.status" placeholder="全部" clearable style="width: 140px">
            <el-option label="待处理" value="pending" />
            <el-option label="处理中" value="processing" />
            <el-option label="已完成" value="completed" />
            <el-option label="已驳回" value="rejected" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">搜索</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 工单列表表格 -->
    <el-card shadow="never" class="table-card">
      <el-table :data="tableData" v-loading="loading" border stripe style="width: 100%">
        <el-table-column prop="order_no" label="工单编号" width="180" align="center" />
        <el-table-column prop="device_hw_id" label="设备ID" width="160" align="center" />
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)" size="small">
              {{ statusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="assignee_name" label="处理人" width="120" align="center" />
        <el-table-column prop="created_at" label="创建时间" width="180" align="center" />
        <el-table-column prop="closed_at" label="闭环时间" width="180" align="center">
          <template #default="{ row }">
            {{ row.closed_at || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="result" label="处理结果" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.result || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" align="center" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.status === 'pending'"
              type="primary"
              link
              size="small"
              @click="handleUpdateStatus(row, 'processing')"
            >
              处理
            </el-button>
            <el-button
              v-if="row.status === 'pending' || row.status === 'processing'"
              type="success"
              link
              size="small"
              @click="openCompleteDialog(row)"
            >
              完成
            </el-button>
            <el-button
              v-if="row.status === 'pending' || row.status === 'processing'"
              type="danger"
              link
              size="small"
              @click="handleUpdateStatus(row, 'rejected')"
            >
              驳回
            </el-button>
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

    <!-- 完成工单弹窗 - 填写维修结果 -->
    <el-dialog v-model="completeDialogVisible" title="完成工单" width="500px">
      <el-form :model="completeForm" label-width="80px">
        <el-form-item label="工单编号">
          <span>{{ completeForm.order_no }}</span>
        </el-form-item>
        <el-form-item label="维修结果">
          <el-input
            v-model="completeForm.result"
            type="textarea"
            :rows="4"
            placeholder="请输入维修处理结果"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="completeDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="handleComplete">确认完成</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Refresh } from '@element-plus/icons-vue'
import { getWorkOrders, updateWorkOrderStatus } from '@/api/workorder'

// 搜索表单
const searchForm = reactive({
  order_no: '',
  device_hw_id: '',
  status: '',
})

// 表格数据
const loading = ref(false)
const tableData = ref<Record<string, any>[]>([])

// 分页配置
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
})

// 完成工单弹窗
const completeDialogVisible = ref(false)
const submitLoading = ref(false)
const completeForm = reactive({
  id: 0,
  order_no: '',
  result: '',
})

// 状态标签类型映射
function statusTagType(status: string): string {
  const map: Record<string, string> = {
    pending: 'warning',
    processing: '',
    completed: 'success',
    rejected: 'info',
  }
  return map[status] || 'info'
}

// 状态标签文字映射
function statusLabel(status: string): string {
  const map: Record<string, string> = {
    pending: '待处理',
    processing: '处理中',
    completed: '已完成',
    rejected: '已驳回',
  }
  return map[status] || status
}

// 获取工单列表
async function fetchData() {
  loading.value = true
  try {
    const res = await getWorkOrders({
      page: pagination.page,
      page_size: pagination.page_size,
      order_no: searchForm.order_no || undefined,
      device_hw_id: searchForm.device_hw_id || undefined,
      status: searchForm.status || undefined,
    })
    tableData.value = res.list || []
    pagination.total = res.total || 0
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
  searchForm.order_no = ''
  searchForm.device_hw_id = ''
  searchForm.status = ''
  pagination.page = 1
  fetchData()
}

// 更新工单状态（处理/驳回）
async function handleUpdateStatus(row: Record<string, any>, status: string) {
  const actionLabel = status === 'processing' ? '处理' : status === 'rejected' ? '驳回' : '更新'
  try {
    await ElMessageBox.confirm(`确定要${actionLabel}该工单吗？`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await updateWorkOrderStatus(row.id, { status: status as 'processing' | 'rejected' })
    ElMessage.success(`${actionLabel}成功`)
    fetchData()
  } catch {
    // 用户取消或请求失败
  }
}

// 打开完成工单弹窗
function openCompleteDialog(row: Record<string, any>) {
  completeForm.id = row.id
  completeForm.order_no = row.order_no
  completeForm.result = ''
  completeDialogVisible.value = true
}

// 确认完成工单
async function handleComplete() {
  if (!completeForm.result.trim()) {
    ElMessage.warning('请输入维修结果')
    return
  }
  submitLoading.value = true
  try {
    await updateWorkOrderStatus(completeForm.id, {
      status: 'completed',
      result: completeForm.result,
    })
    ElMessage.success('工单已完成')
    completeDialogVisible.value = false
    fetchData()
  } catch {
    // 请求失败忽略
  } finally {
    submitLoading.value = false
  }
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
