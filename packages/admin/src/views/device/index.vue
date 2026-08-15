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
      <div class="table-toolbar">
        <el-button v-if="canEdit" type="primary" :icon="Plus" @click="openCreate">新增设备</el-button>
        <span class="toolbar-tip">{{ canEdit ? '可新增/编辑设备；删除仅管理员' : '只读（查看人员）' }}</span>
      </div>
      <el-table :data="tableData" v-loading="loading" border stripe style="width: 100%">
        <el-table-column prop="id" label="设备ID" width="80" align="center" />
        <el-table-column prop="hw_id" label="硬件ID" width="140" align="center" />
        <el-table-column prop="intersection" label="路口位置" min-width="140" show-overflow-tooltip />
        <el-table-column label="在线状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.online_status ? 'success' : 'info'" size="small">{{ row.online_status ? '在线' : '离线' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="sw_version" label="固件版本" width="110" align="center" />
        <el-table-column label="最后签到" width="170" align="center">
          <template #default="{ row }">{{ row.last_checkin_at || '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="170" align="center" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="handleDetail(row)">详情</el-button>
            <el-button v-if="canEdit" type="primary" link size="small" @click="openEdit(row)">编辑</el-button>
            <el-button v-if="isAdmin" type="danger" link size="small" @click="handleDelete(row)">删除</el-button>
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
    <el-dialog v-model="detailVisible" title="设备详情" width="620px">
      <el-descriptions :column="2" border v-if="currentDevice">
        <el-descriptions-item label="设备ID">{{ currentDevice.id }}</el-descriptions-item>
        <el-descriptions-item label="硬件ID">{{ currentDevice.hw_id }}</el-descriptions-item>
        <el-descriptions-item label="路口位置">{{ currentDevice.intersection || '-' }}</el-descriptions-item>
        <el-descriptions-item label="在线状态">
          <el-tag :type="currentDevice.online_status ? 'success' : 'info'" size="small">
            {{ currentDevice.online_status ? '在线' : '离线' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="固件版本">{{ currentDevice.sw_version }}</el-descriptions-item>
        <el-descriptions-item label="配置版本">{{ currentDevice.conf_version }}</el-descriptions-item>
        <el-descriptions-item label="网络编码">{{ currentDevice.network_code }}</el-descriptions-item>
        <el-descriptions-item label="站点编码">{{ currentDevice.station_code }}</el-descriptions-item>
        <el-descriptions-item label="纬度">{{ currentDevice.lat ?? '-' }}</el-descriptions-item>
        <el-descriptions-item label="经度">{{ currentDevice.lng ?? '-' }}</el-descriptions-item>
        <el-descriptions-item label="最后签到">{{ currentDevice.last_checkin_at }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ currentDevice.created_at }}</el-descriptions-item>
      </el-descriptions>

      <!-- 编辑路口与坐标（供地图打点） -->
      <el-divider content-position="left">编辑路口与坐标</el-divider>
      <el-form v-if="currentDevice" :model="editForm" label-width="90px">
        <el-form-item label="路口名称">
          <el-input v-model="editForm.intersection" placeholder="如：人民路口" />
        </el-form-item>
        <el-form-item label="纬度">
          <el-input v-model="editForm.lat" placeholder="如：31.2304" />
        </el-form-item>
        <el-form-item label="经度">
          <el-input v-model="editForm.lng" placeholder="如：121.4737" />
        </el-form-item>
        <el-form-item>
          <el-button v-if="canEdit" type="primary" :loading="saving" @click="saveEdit">保存坐标</el-button>
          <span v-else class="coord-tip">仅运维/管理员可编辑坐标</span>
          <span class="coord-tip">录入经纬度后可在「地图大屏」查看设备分布</span>
        </el-form-item>
      </el-form>
    </el-dialog>

    <!-- 新增/编辑设备弹窗 -->
    <el-dialog v-model="editVisible" :title="editId ? '编辑设备' : '新增设备'" width="520px">
      <el-form :model="editFormDev" label-width="90px">
        <el-form-item label="硬件ID" required>
          <el-input v-model="editFormDev.hw_id" :disabled="!!editId" placeholder="出厂唯一硬件ID" />
        </el-form-item>
        <el-form-item label="路口位置">
          <el-input v-model="editFormDev.intersection" placeholder="如：人民路口" />
        </el-form-item>
        <el-form-item label="网络编码">
          <el-input-number v-model="editFormDev.network_code" :min="0" :max="254" style="width:100%" />
        </el-form-item>
        <el-form-item label="站点编码">
          <el-input-number v-model="editFormDev.station_code" :min="0" :max="65534" style="width:100%" />
        </el-form-item>
        <el-form-item label="纬度">
          <el-input v-model="editFormDev.lat" placeholder="如：31.2304" />
        </el-form-item>
        <el-form-item label="经度">
          <el-input v-model="editFormDev.lng" placeholder="如：121.4737" />
        </el-form-item>
        <!-- AI 辅助：依据当前录入字段给出填写/配置建议 -->
        <AiCopilot :load-fn="() => loadDeviceAdvice()" :fill-fn="() => {}" />
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveDevice">{{ editId ? '保存' : '新增' }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Refresh, Plus } from '@element-plus/icons-vue'
import { getDevices, updateDevice, createDevice, deleteDevice } from '@/api/device'
import { getDeviceAdvice } from '@/api/copilot'
import AiCopilot from '@/components/AiCopilot.vue'
import { useAuthStore } from '@/store/auth'

// 登录角色（用于按钮权限控制）
const authStore = useAuthStore()
const canEdit = computed(() => { const r = authStore.user?.role; return r === 'admin' || r === 'operator' })
const isAdmin = computed(() => authStore.user?.role === 'admin')

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

// 坐标/路口编辑
const saving = ref(false)
const editForm = reactive({ intersection: '', lat: '', lng: '' })

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
  // 初始化编辑表单
  editForm.intersection = row.intersection || ''
  editForm.lat = row.lat != null ? String(row.lat) : ''
  editForm.lng = row.lng != null ? String(row.lng) : ''
  detailVisible.value = true
}

// 保存路口与坐标
async function saveEdit() {
  if (!currentDevice.value) return
  saving.value = true
  const data: Record<string, any> = {}
  if (editForm.intersection.trim()) data.intersection = editForm.intersection.trim()
  if (editForm.lat.trim() !== '') {
    const lat = parseFloat(editForm.lat)
    if (isNaN(lat) || lat < -90 || lat > 90) { ElMessage.warning('纬度范围应为 -90 ~ 90'); saving.value = false; return }
    data.lat = lat
  }
  if (editForm.lng.trim() !== '') {
    const lng = parseFloat(editForm.lng)
    if (isNaN(lng) || lng < -180 || lng > 180) { ElMessage.warning('经度范围应为 -180 ~ 180'); saving.value = false; return }
    data.lng = lng
  }
  try {
    await updateDevice(currentDevice.value.id, data)
    ElMessage.success('路口与坐标已保存')
    // 更新行数据
    Object.assign(currentDevice.value, data)
    fetchData()
  } catch { /* 后端已提示 */ } finally { saving.value = false }
}

// ---- 新增/编辑/删除设备 ----
const editVisible = ref(false)
const editId = ref<number | null>(null)
const editFormDev = reactive({ hw_id: '', intersection: '', network_code: 0, station_code: 0, lat: '', lng: '' })

function openCreate() {
  editId.value = null
  Object.assign(editFormDev, { hw_id: '', intersection: '', network_code: 0, station_code: 0, lat: '', lng: '' })
  editVisible.value = true
}
function openEdit(row: Record<string, any>) {
  editId.value = row.id
  Object.assign(editFormDev, {
    hw_id: String(row.hw_id || ''), intersection: row.intersection || '',
    network_code: row.network_code || 0, station_code: row.station_code || 0,
    lat: row.lat != null ? String(row.lat) : '', lng: row.lng != null ? String(row.lng) : '',
  })
  editVisible.value = true
}
async function saveDevice() {
  const hw = editFormDev.hw_id.trim()
  if (!hw) { ElMessage.warning('硬件ID必填'); return }
  saving.value = true
  const data: Record<string, any> = {
    hw_id: hw, intersection: editFormDev.intersection.trim(),
    network_code: editFormDev.network_code, station_code: editFormDev.station_code,
  }
  if (editFormDev.lat.trim() !== '') { const v = parseFloat(editFormDev.lat); if (!isNaN(v)) data.lat = v }
  if (editFormDev.lng.trim() !== '') { const v = parseFloat(editFormDev.lng); if (!isNaN(v)) data.lng = v }
  try {
    if (editId.value) await updateDevice(editId.value, data)
    else await createDevice(data)
    ElMessage.success(editId.value ? '设备已更新' : '设备已新增')
    editVisible.value = false
    fetchData()
  } catch { /* 后端已提示 */ } finally { saving.value = false }
}
// AI 辅助：依据当前录入设备字段生成填写/配置建议
async function loadDeviceAdvice() {
  return getDeviceAdvice({
    hw_id: editFormDev.hw_id,
    intersection: editFormDev.intersection,
    network_code: editFormDev.network_code,
    station_code: editFormDev.station_code,
    lat: parseFloat(editFormDev.lat) || 0,
    lng: parseFloat(editFormDev.lng) || 0,
  })
}
async function handleDelete(row: Record<string, any>) {
  try {
    await ElMessageBox.confirm(`确认删除设备（硬件ID #${row.hw_id}）？该设备关联的故障/工单数据会保留，仅移除台账。`, '提示', { type: 'warning' })
    await deleteDevice(row.id)
    ElMessage.success('设备已删除')
    fetchData()
  } catch { /* 取消或失败 */ }
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
.table-toolbar {
  margin-bottom: 12px;
  display: flex;
  align-items: center;
  gap: 10px;
}
.toolbar-tip {
  font-size: 12px;
  color: #909399;
}

.pagination {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
.coord-tip {
  margin-left: 12px;
  font-size: 12px;
  color: #909399;
}
</style>
