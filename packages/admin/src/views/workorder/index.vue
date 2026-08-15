<template>
  <div class="workorder-page">
    <!-- 统计卡片 -->
    <el-row :gutter="12" class="stat-row">
      <el-col :span="4" v-for="s in statCards" :key="s.key">
        <el-card shadow="never" class="stat-card" @click="setStatusFilter(s.key)">
          <div class="stat-num" :style="{ color: s.color }">{{ s.key === 'avg' ? avgClosure : (statMap[s.key] || 0) }}</div>
          <div class="stat-label" :class="{ active: searchForm.status === s.key }">{{ s.label }}</div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card shadow="never" class="stat-card">
          <div class="stat-num" style="color: #409EFF">{{ totalCount }}</div>
          <div class="stat-label">工单总数</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 搜索栏 -->
    <el-card shadow="never" class="search-card">
      <el-form :inline="true" :model="searchForm" @submit.prevent="handleSearch">
        <el-form-item label="工单编号">
          <el-input
            v-model="searchForm.order_no"
            placeholder="请输入工单编号"
            clearable
            style="width: 180px"
          />
        </el-form-item>
        <el-form-item label="设备">
          <el-select
            v-model="searchForm.device_hw_id"
            placeholder="搜索设备ID或路口"
            clearable
            filterable
            remote
            :remote-method="searchDevices"
            :loading="devLoading"
            style="width: 200px"
          >
            <el-option-group v-for="g in deviceGroups" :key="g.label" :label="g.label">
              <el-option
                v-for="d in g.options"
                :key="d.hw_id"
                :label="d.intersection ? d.intersection + ' (#'+d.hw_id+')' : '#'+d.hw_id"
                :value="d.hw_id"
              />
            </el-option-group>
          </el-select>
        </el-form-item>
        <el-form-item label="工单状态">
          <el-select v-model="searchForm.status" placeholder="全部" clearable style="width: 130px">
            <el-option label="待处理" value="pending" />
            <el-option label="处理中" value="processing" />
            <el-option label="已完成" value="completed" />
            <el-option label="已驳回" value="rejected" />
          </el-select>
        </el-form-item>
        <el-form-item label="创建时间">
          <el-date-picker
            v-model="dateRange" type="daterange" range-separator="至" size="small"
            start-placeholder="开始日期" end-placeholder="结束日期" value-format="YYYY-MM-DD"
            style="width: 230px" @change="handleDateChange"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">搜索</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 工单列表表格 -->
    <el-card shadow="never" class="table-card">
      <div class="table-toolbar">
        <div>
          <el-button v-if="isOperator" type="primary" :icon="Plus" @click="openCreate">新建工单</el-button>
        </div>
        <span v-if="!isOperator" class="toolbar-tip">查看角色只读</span>
      </div>
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
        <el-table-column label="SLA/超时" width="120" align="center">
          <template #default="{ row }">
            <el-tooltip v-if="row.overdue" :content="`超时 ${formatOverdue(row.overdue_hours)}，请优先处理`" placement="top">
              <el-tag type="danger" effect="dark" size="small">超时 {{ formatOverdue(row.overdue_hours) }}</el-tag>
            </el-tooltip>
            <span v-else class="sla-ok">·</span>
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
        <el-table-column label="操作" width="210" align="center" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="isOperator && (row.status === 'pending' || !row.assignee_id)"
              type="primary"
              size="small"
              @click="openAssignDialog(row)"
            >
              {{ row.assignee_id ? '改派' : '派单' }}
            </el-button>
            <el-button
              v-if="isOperator && row.status === 'pending' && row.assignee_id"
              type="warning"
              size="small"
              @click="handleUpdateStatus(row, 'processing')"
            >
              处理
            </el-button>
            <el-button
              v-if="isOperator && (row.status === 'processing')"
              type="success"
              size="small"
              @click="openCompleteDialog(row)"
            >
              完成
            </el-button>
            <el-button
              v-if="isOperator && (row.status === 'pending' || row.status === 'processing')"
              type="danger"
              size="small"
              @click="handleUpdateStatus(row, 'rejected')"
            >
              驳回
            </el-button>
            <el-button v-if="isAdmin" type="danger" plain size="small" @click="handleDelete(row)">删除</el-button>
            <el-button type="info" plain size="small" @click="openDetail(row)">详情</el-button>
            <span v-if="!isOperator" class="viewer-tip">只读</span>
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

    <!-- 派单弹窗，选择维修人员 -->
    <el-dialog v-model="assignDialogVisible" title="工单派单" width="460px">
      <el-form label-width="90px">
        <el-form-item label="工单编号">
          <span>{{ assignTarget?.order_no }}</span>
        </el-form-item>
        <el-form-item label="维修人员" required>
          <el-select v-model="assignUserId" placeholder="选择维修人员（运维/管理员）" style="width: 100%">
            <el-option
              v-for="u in assignableUsers"
              :key="u.id"
              :label="u.username + '（' + (u.role === 'admin' ? '管理员' : '运维') + '）'"
              :value="u.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="说明">
          <span class="assign-tip">派单后工单进入「处理中」，由所选维修人员处理</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="assignDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="assignLoading" @click="handleAssign">确认派单</el-button>
      </template>
    </el-dialog>

    <!-- 新建工单弹窗 -->
    <el-dialog v-model="createVisible" title="新建工单" width="560px">
      <el-form :model="createForm" label-width="90px">
        <el-form-item label="关联故障" required>
          <el-select
            v-model="createForm.fault_id"
            placeholder="选择活跃故障（自动带出设备）"
            filterable
            :loading="faultLoading"
            style="width: 100%"
            @change="onFaultChange"
          >
            <el-option v-for="f in activeFaults" :key="f.id" :value="f.id" :label="faultOptionLabel(f)" />
          </el-select>
        </el-form-item>
        <el-form-item label="设备ID" required>
          <el-input :model-value="String(createForm.device_hw_id)" disabled placeholder="选择故障后自动填充" />
        </el-form-item>
        <el-form-item label="维修人员">
          <el-select v-model="createForm.assignee_id" placeholder="选填，指派维修人员" clearable style="width: 100%">
            <el-option
              v-for="u in assignableUsers"
              :key="u.id"
              :label="u.username + '（' + (u.role === 'admin' ? '管理员' : '运维') + '）'"
              :value="u.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="说明">
          <span class="assign-tip">新建后工单状态为「待处理」，再行派单/处理</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="createLoading" @click="handleCreate">创建工单</el-button>
      </template>
    </el-dialog>

    <!-- 工单详情抽屉：SLA/关联故障/操作时间线 -->
    <el-drawer v-model="detailVisible" title="工单详情" size="520px" v-loading="detailLoading">
      <div v-if="detail" class="detail-body">
        <!-- 头部概览 -->
        <div class="detail-head">
          <div class="order-no">{{ detail.work_order?.order_no }}</div>
          <el-tag :type="statusTagType(detail.work_order?.status)" size="small">
            {{ statusLabel(detail.work_order?.status) }}
          </el-tag>
        </div>
        <el-descriptions :column="1" border class="detail-desc">
          <el-descriptions-item label="设备ID">{{ detail.work_order?.device_hw_id ?? '-' }}</el-descriptions-item>
          <el-descriptions-item label="处理人">{{ detail.assignee || '未指派' }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ detail.work_order?.created_at || '-' }}</el-descriptions-item>
          <el-descriptions-item label="闭环时间">{{ detail.work_order?.closed_at || '-' }}</el-descriptions-item>
          <el-descriptions-item label="处理结果">{{ detail.work_order?.result || '-' }}</el-descriptions-item>
        </el-descriptions>

        <!-- SLA 状态 -->
        <h4 class="section-title">SLA 状态</h4>
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="阶段">
            <el-tag :type="statusTagType(detail.work_order?.status)" size="small">{{ detail.sla?.stage || '-' }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="超时">
            <el-tag v-if="detail.sla?.overdue" type="danger" effect="dark" size="small">
              超时 {{ formatOverdue(detail.sla?.overdue_hours) }}
            </el-tag>
            <span v-else class="sla-ok">未超时</span>
          </el-descriptions-item>
        </el-descriptions>

        <!-- 关联故障 -->
        <h4 class="section-title">关联故障</h4>
        <div v-if="detail.fault?.id" class="fault-card">
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="故障类型">{{ errCodeLabel(detail.fault.err_code) }}</el-descriptions-item>
            <el-descriptions-item label="错误码">{{ detail.fault.err_code ?? '-' }}</el-descriptions-item>
            <el-descriptions-item label="故障级别">
              <el-tag :type="detail.fault.fault_level === 'critical' ? 'danger' : 'warning'" size="small">
                {{ detail.fault.fault_level === 'critical' ? '严重' : '一般' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="故障状态">
              <el-tag :type="detail.fault.status === 'active' ? 'danger' : 'success'" size="small">
                {{ detail.fault.status === 'active' ? '活跃' : '已解决' }}
              </el-tag>
            </el-descriptions-item>
          </el-descriptions>
          <div v-if="detail.fault.device?.intersection" class="fault-device">
            <el-icon><Location /></el-icon> 路口：{{ detail.fault.device.intersection }}
          </div>
        </div>
        <el-empty v-else description="暂无关联故障" :image-size="60" />

        <!-- 操作时间线 -->
        <h4 class="section-title">操作时间线</h4>
        <el-timeline v-if="detail.timeline?.length">
          <el-timeline-item
            v-for="item in detail.timeline"
            :key="item.id"
            :timestamp="item.created_at"
            placement="top"
            type="primary"
          >
            <div class="timeline-row">
              <span class="timeline-action">{{ actionLabel(item.action) }}</span>
              <span class="timeline-user">{{ item.username }}</span>
            </div>
            <div class="timeline-detail">{{ item.detail }}</div>
          </el-timeline-item>
        </el-timeline>
        <el-empty v-else description="暂无操作记录" :image-size="60" />
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Refresh, Plus, Location } from '@element-plus/icons-vue'
import { getWorkOrders, getWorkOrderDetail, updateWorkOrderStatus, assignWorkOrder, getAssignableUsers, createWorkOrder, deleteWorkOrder } from '@/api/workorder'
import { getWorkOrderStats, getWorkOrderAvgClosure } from '@/api/dashboard'
import { getDevices } from '@/api/device'
import { getFaults } from '@/api/fault'
import { useAuthStore } from '@/store/auth'

// 当前登录用户角色（用于按钮权限控制）
const authStore = useAuthStore()
const isOperator = computed(() => { const r = authStore.user?.role; return r === 'admin' || r === 'operator' })
const isAdmin = computed(() => authStore.user?.role === 'admin')

// 设备异步搜索（工单筛选用）：按关键字搜索设备并分组（在线/离线）
const devLoading = ref(false)
const deviceGroups = ref<{ label: string; options: any[] }[]>([])
async function searchDevices(keyword?: string) {
  devLoading.value = true
  try {
    const kw = (keyword || '').trim()
    const params: Record<string, any> = { page_size: 50 }
    if (kw && /^\d+$/.test(kw)) params.hw_id = kw
    else if (kw) params.intersection = kw
    const res = await getDevices(params)
    const list: any[] = res.data?.list || []
    const online = list.filter((d) => d.online_status)
    const offline = list.filter((d) => !d.online_status)
    const groups: { label: string; options: any[] }[] = []
    if (online.length) groups.push({ label: `在线（${online.length}）`, options: online })
    if (offline.length) groups.push({ label: `离线（${offline.length}）`, options: offline })
    deviceGroups.value = groups
  } catch { deviceGroups.value = [] }
  finally { devLoading.value = false }
}

// 搜索表单
const searchForm = reactive({
  order_no: '',
  device_hw_id: '',
  status: '',
})
const dateRange = ref<[string, string] | null>(null)

// 统计卡片
const statCards = [
  { key: 'pending', label: '待处理', color: '#F56C6C' },
  { key: 'processing', label: '处理中', color: '#E6A23C' },
  { key: 'completed', label: '已完成', color: '#67C23A' },
  { key: 'overdue', label: '超时', color: '#D03050' },
  { key: 'avg', label: '平均闭环(小时)', color: '#409EFF' },
]
const statMap = ref<Record<string, number>>({})
const avgClosure = ref<string>('-')
const totalCount = ref(0)

async function loadStats() {
  try {
    const [statsRes, avgRes] = await Promise.all([getWorkOrderStats(), getWorkOrderAvgClosure({ days: 30 })])
    const d = statsRes.data || {}
    statMap.value = {
      pending: d.pending || 0,
      processing: d.processing || 0,
      completed: d.completed || 0,
      overdue: d.overdue || 0,
    }
    totalCount.value = (d.pending || 0) + (d.processing || 0) + (d.completed || 0)
    const a = avgRes.data
    if (a && a.avg_hours != null) {
      avgClosure.value = (Math.round(a.avg_hours * 10) / 10).toString()
    } else if (a && a.avg_closure_hours != null) {
      avgClosure.value = (Math.round(a.avg_closure_hours * 10) / 10).toString()
    }
  } catch { /* 忽略统计失败 */ }
}

function setStatusFilter(key: string) {
  if (key === 'avg') return
  searchForm.status = key
  handleSearch()
}

function handleDateChange() {
  handleSearch()
}

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

// 派单弹窗（指派维修人员）
const assignDialogVisible = ref(false)
const assignLoading = ref(false)
const assignTarget = ref<Record<string, any> | null>(null)
const assignUserId = ref<number | undefined>()
const assignableUsers = ref<{ id: number; username: string; role: string }[]>([])

// 新建工单弹窗
const createVisible = ref(false)
const createLoading = ref(false)
const activeFaults = ref<any[]>([])
const faultLoading = ref(false)
const createForm = reactive({ fault_id: undefined as number | undefined, device_hw_id: '', assignee_id: undefined as number | undefined })

// 故障下拉文案
function faultOptionLabel(f: any): string {
  return `#${f.device_hw_id} ${f.err_code_name || ('故障码'+f.err_code)}（${f.fault_type || '-'}，${f.status === 'active' ? '活跃' : '已解决'}）`
}

async function loadActiveFaults() {
  faultLoading.value = true
  try {
    const res = await getFaults({ page_size: 200, status: 'active' })
    activeFaults.value = res.data?.list || []
  } catch { activeFaults.value = [] }
  finally { faultLoading.value = false }
}

function openCreate() {
  Object.assign(createForm, { fault_id: undefined, device_hw_id: '', assignee_id: undefined })
  loadActiveFaults()
  createVisible.value = true
}

function onFaultChange(fid: number) {
  const f = activeFaults.value.find((x) => x.id === fid)
  createForm.device_hw_id = f ? String(f.device_hw_id) : ''
}

async function handleCreate() {
  if (!createForm.fault_id) { ElMessage.warning('请选择关联故障'); return }
  if (!createForm.device_hw_id) { ElMessage.warning('请选择有效故障（需带设备ID）'); return }
  createLoading.value = true
  try {
    await createWorkOrder({
      fault_id: createForm.fault_id,
      device_hw_id: createForm.device_hw_id,
      assignee_id: createForm.assignee_id || 0,
    } as any)
    ElMessage.success('工单创建成功')
    createVisible.value = false
    fetchData()
  } catch { /* 后端提示 */ } finally { createLoading.value = false }
}

async function handleDelete(row: Record<string, any>) {
  try {
    await ElMessageBox.confirm(`确认删除工单「${row.order_no}」？关联故障记录会保留（仅解除绑定）。`, '提示', { type: 'warning' })
    await deleteWorkOrder(row.id)
    ElMessage.success('工单已删除')
    fetchData()
  } catch { /* 取消或失败 */ }
}

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

// 格式化超时时长：≥24h 显示天数，否则显示小时
function formatOverdue(hours: number): string {
  if (!hours || hours <= 0) return ''
  const h = Math.round(hours * 10) / 10
  if (h >= 24) {
    const days = Math.floor(h / 24)
    const rem = Math.round((h - days * 24) * 10) / 10
    return rem > 0 ? `${days}天${rem}h` : `${days}天`
  }
  return `${h}h`
}

// ----------------- 工单详情抽屉 -----------------
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<Record<string, any> | null>(null)

// 打开工单详情
async function openDetail(row: Record<string, any>) {
  detailVisible.value = true
  detailLoading.value = true
  detail.value = null
  try {
    const res = await getWorkOrderDetail(row.id)
    detail.value = res.data
  } catch {
    detail.value = null
    ElMessage.error('工单详情加载失败')
  } finally {
    detailLoading.value = false
  }
}

// 故障错误码 → 中文名（与故障页一致）
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

// 操作类型 → 中文名
function actionLabel(action: string): string {
  const map: Record<string, string> = {
    create: '创建', update: '更新', delete: '删除',
    dispatch: '派单', login: '登录', logout: '登出', read: '查看',
  }
  return map[action] || action
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
      start_time: dateRange.value?.[0] || undefined,
      end_time: dateRange.value?.[1] || undefined,
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
  searchForm.order_no = ''
  searchForm.device_hw_id = ''
  searchForm.status = ''
  dateRange.value = null
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

// 打开派单弹窗（管理员/运维可派单）
async function openAssignDialog(row: Record<string, any>) {
  assignTarget.value = row
  assignUserId.value = row.assignee_id || undefined
  assignDialogVisible.value = true
  if (assignableUsers.value.length === 0) {
    try {
      const res = await getAssignableUsers()
      assignableUsers.value = res.data?.list || []
    } catch { /* 忽略 */ }
  }
}

// 确认派单
async function handleAssign() {
  if (!assignTarget.value || !assignUserId.value) {
    ElMessage.warning('请选择维修人员')
    return
  }
  assignLoading.value = true
  try {
    await assignWorkOrder(assignTarget.value.id, assignUserId.value)
    ElMessage.success('派单成功')
    assignDialogVisible.value = false
    fetchData()
  } catch {
    // 失败由后端提示
  } finally {
    assignLoading.value = false
  }
}

onMounted(async () => {
  // 拉取用户信息以决定按钮权限（若尚未获取）
  if (authStore.token && !authStore.user) {
    try { await authStore.fetchUserInfo() } catch { /* 忽略 */ }
  }
  // 加载设备下拉与可派单人员
  searchDevices('')
  try { const r = await getAssignableUsers(); assignableUsers.value = r.data?.list || [] } catch { /* 忽略 */ }
  fetchData()
  loadStats()
})
</script>

<style scoped>
/* 统计卡片 */
.stat-row {
  margin-bottom: 12px;
}
.stat-card {
  text-align: center;
  cursor: pointer;
}
.stat-num {
  font-size: 24px;
  font-weight: 700;
  line-height: 1.2;
}
.stat-label {
  color: #909399;
  font-size: 13px;
  margin-top: 4px;
}
.stat-label.active {
  color: #409eff;
  font-weight: 600;
}

.search-card {
  margin-bottom: 16px;
}

.table-card {
  border-radius: 4px;
}

.sla-ok {
  color: #c0c4cc;
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
.assign-tip {
  font-size: 12px;
  color: #909399;
}
.viewer-tip {
  color: #c0c4cc;
  font-size: 12px;
}

/* 工单详情抽屉 */
.detail-body {
  padding-right: 6px;
}
.detail-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
}
.order-no {
  font-size: 18px;
  font-weight: 600;
  color: #303133;
}
.detail-desc {
  margin-bottom: 16px;
}
.section-title {
  margin: 18px 0 10px;
  color: #303133;
  font-size: 14px;
  font-weight: 600;
}
.fault-card {
  border: 1px solid #e4e7ed;
  border-radius: 6px;
  padding: 10px;
}
.fault-device {
  margin-top: 10px;
  color: #606266;
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
}
.timeline-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.timeline-action {
  font-weight: 600;
  color: #303133;
}
.timeline-user {
  color: #909399;
  font-size: 12px;
}
.timeline-detail {
  color: #606266;
  font-size: 13px;
  margin-top: 2px;
}
</style>
