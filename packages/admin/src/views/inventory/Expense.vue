<template>
  <div class="inv-page">
    <!-- 统计卡片 -->
    <el-row :gutter="12" class="stat-row">
      <el-col :span="5"><el-card shadow="never" class="stat-card">
        <div class="stat-num">{{ fmtMoney(stats.total_amount) }}</div><div class="stat-label">费用总额(元)</div>
      </el-card></el-col>
      <el-col :span="4"><el-card shadow="never" class="stat-card">
        <div class="stat-num small"><el-icon><Goods /></el-icon>{{ fmtMoney(stats.material) }}</div><div class="stat-label">耗材</div>
      </el-card></el-col>
      <el-col :span="4"><el-card shadow="never" class="stat-card">
        <div class="stat-num small"><el-icon><User /></el-icon>{{ fmtMoney(stats.labor) }}</div><div class="stat-label">人工</div>
      </el-card></el-col>
      <el-col :span="4"><el-card shadow="never" class="stat-card">
        <div class="stat-num small"><el-icon><Van /></el-icon>{{ fmtMoney(stats.traffic) }}</div><div class="stat-label">交通</div>
      </el-card></el-col>
      <el-col :span="4"><el-card shadow="never" class="stat-card">
        <div class="stat-num small"><el-icon><MoreFilled /></el-icon>{{ fmtMoney(stats.other) }}</div><div class="stat-label">其它</div>
      </el-card></el-col>
    </el-row>

    <el-card shadow="never">
      <div class="filter-bar">
        <el-input v-model="filter.device_hw_id" placeholder="设备ID" clearable size="small" style="width: 130px" @keyup.enter="load(1)" />
        <el-select v-model="filter.type" placeholder="费用类型" clearable size="small" style="width: 120px" @change="load(1)">
          <el-option label="耗材" value="material" /><el-option label="人工" value="labor" />
          <el-option label="交通" value="traffic" /><el-option label="其它" value="other" />
        </el-select>
        <el-date-picker v-model="range" type="daterange" value-format="YYYY-MM-DD" start-placeholder="开始日期" end-placeholder="结束日期" size="small" style="width: 240px" @change="load(1)" />
        <el-button type="primary" size="small" :icon="Search" @click="load(1)">查询</el-button>
        <div class="flex1"></div>
        <el-button v-if="isOperator" type="primary" size="small" :icon="Plus" @click="openEdit()">登记费用</el-button>
      </div>

      <el-table :data="list" v-loading="loading" stripe>
        <el-table-column prop="expense_no" label="费用单号" width="140" />
        <el-table-column label="类型" width="80">
          <template #default="{ row }"><el-tag :type="typeTag(row.type)" size="small">{{ typeLabel(row.type) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="金额" width="100">
          <template #default="{ row }"><span class="amt">{{ fmtMoney(row.amount) }}</span></template>
        </el-table-column>
        <el-table-column prop="device_hw_id" label="设备ID" width="90" />
        <el-table-column prop="description" label="费用说明" min-width="160" show-overflow-tooltip />
        <el-table-column label="发生日期" width="110">
          <template #default="{ row }">{{ row.work_date || '—' }}</template>
        </el-table-column>
        <el-table-column prop="operator" label="经办人" width="90" />
        <el-table-column label="入账" width="80">
          <template #default="{ row }"><el-tag v-if="row.confirmed" type="success" size="small">已确认</el-tag><el-tag v-else type="info" size="small">待确认</el-tag></template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button v-if="isOperator" size="small" plain @click="openEdit(row)">编辑</el-button>
            <el-button v-if="isOperator && !row.confirmed" size="small" type="success" plain @click="doConfirm(row, true)">确认</el-button>
            <el-button v-if="isOperator && row.confirmed" size="small" type="info" plain @click="doConfirm(row, false)">取消确认</el-button>
            <el-button v-if="isAdmin" size="small" type="danger" plain @click="del(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pager"><el-pagination background layout="total, prev, pager, next" :total="total" :page-size="pageSize" :current-page="page" @current-change="load" /></div>
    </el-card>

    <!-- 登记/编辑费用 -->
    <el-dialog v-model="editVisible" :title="editForm.id ? '编辑费用' : '登记维修费用'" width="520px" :close-on-click-modal="false">
      <el-form :model="editForm" label-width="90px">
        <el-form-item label="费用类型">
          <el-radio-group v-model="editForm.type">
            <el-radio-button value="material">耗材</el-radio-button>
            <el-radio-button value="labor">人工</el-radio-button>
            <el-radio-button value="traffic">交通</el-radio-button>
            <el-radio-button value="other">其它</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="金额(元)"><el-input-number v-model="editForm.amount" :min="0" :precision="2" style="width: 200px" /></el-form-item>
        <el-form-item label="设备ID"><el-input-number v-model="editForm.device_hw_id" :min="0" style="width: 200px" /></el-form-item>
        <el-form-item label="关联工单"><el-input-number v-model="editForm.work_order_id" :min="0" placeholder="可选" style="width: 200px" /></el-form-item>
        <el-form-item label="发生日期"><el-date-picker v-model="editForm.work_date" type="date" value-format="YYYY-MM-DD" style="width: 200px" /></el-form-item>
        <el-form-item label="费用说明"><el-input v-model="editForm.description" placeholder="如：更换信号灯电源模块" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="editForm.note" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Plus, Goods, User, Van, MoreFilled } from '@element-plus/icons-vue'
import { getExpenses, getExpenseStats, saveExpense, confirmExpense, deleteExpense } from '@/api/expense'
import { useAuthStore } from '@/store/auth'

const authStore = useAuthStore()
const isOperator = computed(() => { const r = authStore.user?.role; return r === 'admin' || r === 'operator' })
const isAdmin = computed(() => authStore.user?.role === 'admin')

const list = ref<any[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const filter = reactive({ device_hw_id: '', type: '' })
const range = ref<[string, string] | null>(null)

const stats = reactive<any>({})

async function load(p = page.value) {
  loading.value = true
  page.value = p
  try {
    const params: Record<string, any> = { page: p, page_size: pageSize.value }
    if (filter.device_hw_id) params.device_hw_id = filter.device_hw_id
    if (filter.type) params.type = filter.type
    if (range.value?.length === 2) { params.from = range.value[0]; params.to = range.value[1] }
    const res = await getExpenses(params)
    list.value = res.data?.list || []
    total.value = res.data?.total || 0
  } catch { /* ignore */ } finally { loading.value = false }
}

async function loadStats() {
  try { const res = await getExpenseStats(); Object.assign(stats, res.data || {}) } catch { /* ignore */ }
}

const editVisible = ref(false)
const saving = ref(false)
const editForm = reactive<any>({ id: 0, type: 'material', amount: 0, device_hw_id: undefined, work_order_id: undefined, work_date: '', description: '', note: '' })
function openEdit(row?: any) {
  if (row) Object.assign(editForm, { ...row, work_date: row.work_date || '' })
  else Object.assign(editForm, { id: 0, type: 'material', amount: 0, device_hw_id: undefined, work_order_id: undefined, work_date: '', description: '', note: '' })
  editVisible.value = true
}
async function save() {
  saving.value = true
  try {
    const payload: Record<string, any> = { ...editForm }
    if (payload.work_order_id === undefined || payload.work_order_id === null) delete payload.work_order_id
    if (payload.device_hw_id === undefined || payload.device_hw_id === null) delete payload.device_hw_id
    const res = await saveExpense(payload)
    if (res.code === 0) { ElMessage.success('保存成功'); editVisible.value = false; load(); loadStats() }
    else ElMessage.error(res.msg || '保存失败')
  } catch (e: any) { ElMessage.error(e?.response?.data?.msg || '保存失败') } finally { saving.value = false }
}

async function doConfirm(row: any, confirmed: boolean) {
  const res = await confirmExpense(row.id, confirmed)
  if (res.code === 0) { ElMessage.success(confirmed ? '已确认入账' : '已取消确认'); load(); loadStats() }
  else ElMessage.error(res.msg || '操作失败')
}
async function del(row: any) {
  try { await ElMessageBox.confirm(`确定删除费用单 ${row.expense_no} 吗？`, '提示', { type: 'warning' }) } catch { return }
  const res = await deleteExpense(row.id)
  if (res.code === 0) { ElMessage.success('删除成功'); load(); loadStats() }
  else ElMessage.error(res.msg || '删除失败')
}

function typeLabel(t: string) { return ({ material: '耗材', labor: '人工', traffic: '交通', other: '其它' } as Record<string, string>)[t] || t }
function typeTag(t: string) { return ({ material: 'success', labor: 'warning', traffic: '', other: 'info' } as Record<string, any>)[t] || 'info' }
function fmtMoney(v: any) { return v == null ? '0.00' : Number(v).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }

onMounted(() => { load(1); loadStats() })
</script>

<style scoped>
.inv-page { padding: 4px; }
.stat-row { margin-bottom: 12px; }
.stat-card { text-align: center; }
.stat-num { font-size: 22px; font-weight: 700; color: #409EFF; }
.stat-num.small { font-size: 18px; display: flex; align-items: center; justify-content: center; gap: 4px; }
.stat-label { color: #909399; font-size: 12px; margin-top: 4px; }
.filter-bar { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.flex1 { flex: 1; }
.pager { display: flex; justify-content: flex-end; margin-top: 12px; }
.amt { font-weight: 600; color: #F56C6C; }
</style>
