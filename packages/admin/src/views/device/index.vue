<template>
  <div class="device-page">
    <!-- 搜索栏 -->
    <el-card shadow="never" class="search-card">
      <el-form :inline="true" :model="searchForm" @submit.prevent="handleSearch">
        <el-form-item label="路口名称">
          <el-input
            v-model="searchForm.intersection"
            placeholder="请输入路口名称"
            clearable
            style="width: 200px"
          />
        </el-form-item>
        <el-form-item label="在线状态">
          <el-select
            v-model="searchForm.online_status"
            placeholder="全部"
            clearable
            style="width: 140px"
          >
            <el-option label="在线" value="online" />
            <el-option label="离线" value="offline" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">搜索</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 设备列表表格 -->
    <el-card shadow="never" class="table-card">
      <el-table :data="tableData" v-loading="loading" border stripe style="width: 100%">
        <el-table-column prop="id" label="设备ID" width="80" align="center" />
        <el-table-column prop="hw_id" label="硬件ID" width="180" align="center" />
        <el-table-column prop="intersection" label="路口位置" min-width="150" show-overflow-tooltip />
        <el-table-column label="在线状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.online_status === 'online' ? 'success' : 'info'" size="small">
              {{ row.online_status === 'online' ? '在线' : '离线' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="firmware_version" label="固件版本" width="120" align="center" />
        <el-table-column prop="last_seen_at" label="最后签到时间" width="180" align="center" />
        <el-table-column label="操作" width="100" align="center" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="handleDetail(row)">查看详情</el-button>
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

    <!-- 设备详情弹窗 -->
    <el-dialog v-model="detailVisible" title="设备详情" width="600px">
      <el-descriptions :column="2" border v-if="currentDevice">
        <el-descriptions-item label="设备ID">{{ currentDevice.id }}</el-descriptions-item>
        <el-descriptions-item label="硬件ID">{{ currentDevice.hw_id }}</el-descriptions-item>
        <el-descriptions-item label="路口位置">{{ currentDevice.intersection }}</el-descriptions-item>
        <el-descriptions-item label="在线状态">
          <el-tag :type="currentDevice.online_status === 'online' ? 'success' : 'info'" size="small">
            {{ currentDevice.online_status === 'online' ? '在线' : '离线' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="固件版本">{{ currentDevice.firmware_version }}</el-descriptions-item>
        <el-descriptions-item label="网络编码">{{ currentDevice.network_code }}</el-descriptions-item>
        <el-descriptions-item label="站点编码">{{ currentDevice.station_code }}</el-descriptions-item>
        <el-descriptions-item label="安装时间">{{ currentDevice.installed_at }}</el-descriptions-item>
        <el-descriptions-item label="最后签到时间">{{ currentDevice.last_seen_at }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ currentDevice.created_at }}</el-descriptions-item>
      </el-descriptions>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Search, Refresh } from '@element-plus/icons-vue'
import { getDevices } from '@/api/device'

// 搜索表单
const searchForm = reactive({
  intersection: '',
  online_status: '',
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

// 设备详情弹窗
const detailVisible = ref(false)
const currentDevice = ref<Record<string, any> | null>(null)

// 获取设备列表
async function fetchData() {
  loading.value = true
  try {
    const res = await getDevices({
      page: pagination.page,
      page_size: pagination.page_size,
      intersection: searchForm.intersection || undefined,
      online_status: searchForm.online_status || undefined,
    })
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
  searchForm.intersection = ''
  searchForm.online_status = ''
  pagination.page = 1
  fetchData()
}

// 查看设备详情
async function handleDetail(row: Record<string, any>) {
  currentDevice.value = row
  detailVisible.value = true
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
