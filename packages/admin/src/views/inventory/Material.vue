<template>
  <div class="inv-page">
    <!-- 统计卡片 -->
    <el-row :gutter="12" class="stat-row">
      <el-col :span="6"><el-card shadow="never" class="stat-card">
        <div class="stat-num">{{ stats.material_count ?? '-' }}</div><div class="stat-label">物料种类</div>
      </el-card></el-col>
      <el-col :span="6"><el-card shadow="never" class="stat-card">
        <div class="stat-num warn">{{ stats.low_stock_count ?? '-' }}</div><div class="stat-label">低库存预警</div>
      </el-card></el-col>
      <el-col :span="6"><el-card shadow="never" class="stat-card">
        <div class="stat-num">{{ stats.stock_record_count ?? '-' }}</div><div class="stat-label">出入库记录</div>
      </el-card></el-col>
      <el-col :span="6"><el-card shadow="never" class="stat-card">
        <div class="stat-num">{{ fmtMoney(stats.total_value) }}</div><div class="stat-label">库存总值(元)</div>
      </el-card></el-col>
    </el-row>

    <el-card shadow="never">
      <el-tabs v-model="tab">
        <!-- 物料档案 -->
        <el-tab-pane label="物料档案" name="material">
          <div class="filter-bar">
            <el-input v-model="filter.keyword" placeholder="编码/名称" clearable size="small" style="width: 180px" @keyup.enter="load(1)" />
            <el-select v-model="filter.category" placeholder="分类" clearable size="small" style="width: 120px" @change="load(1)">
              <el-option v-for="c in categories" :key="c" :label="c" :value="c" />
            </el-select>
            <el-switch v-model="onlyLow" active-text="仅看低库存" inactive-text="" size="small" @change="load(1)" />
            <el-button type="primary" size="small" :icon="Search" @click="load(1)">查询</el-button>
            <div class="flex1"></div>
            <el-button v-if="isOperator" type="primary" size="small" :icon="Plus" @click="openEdit()">新增物料</el-button>
          </div>

          <el-table :data="list" v-loading="loading" stripe>
            <el-table-column prop="code" label="编码" width="120" />
            <el-table-column prop="name" label="名称" min-width="130">
              <template #default="{ row }">
                <span>{{ row.name }}</span>
                <el-tag v-if="row.low_stock" type="danger" size="small" style="margin-left: 6px">低库存</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="category" label="分类" width="90" />
            <el-table-column prop="spec" label="规格" min-width="110" show-overflow-tooltip />
            <el-table-column label="绑定设备" width="90">
              <template #default="{ row }">{{ row.device_hw_id ? ('#' + row.device_hw_id) : '—' }}</template>
            </el-table-column>
            <el-table-column prop="unit" label="单位" width="70" />
            <el-table-column label="单价" width="90">
              <template #default="{ row }">{{ fmtMoney(row.unit_price) }}</template>
            </el-table-column>
            <el-table-column prop="stock" label="库存" width="80">
              <template #default="{ row }">
                <span :class="{ 'low-num': row.low_stock }">{{ row.stock }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="threshold" label="预警值" width="80" />
            <el-table-column label="操作" width="250" fixed="right">
              <template #default="{ row }">
                <el-button v-if="isOperator" size="small" plain @click="openEdit(row)">编辑</el-button>
                <el-button v-if="isOperator" size="small" type="warning" plain @click="openAdjust(row)">库存</el-button>
                <el-button v-if="isOperator" size="small" type="primary" plain @click="openUse(row)">领料</el-button>
                <el-button v-if="isAdmin" size="small" type="danger" plain @click="del(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div class="pager"><el-pagination background layout="total, prev, pager, next" :total="total" :page-size="pageSize" :current-page="page" @current-change="load" /></div>
        </el-tab-pane>

        <!-- 出入库流水 -->
        <el-tab-pane label="出入库流水" name="stock">
          <div class="filter-bar">
            <el-select v-model="stockFilter.type" placeholder="类型" clearable size="small" style="width: 130px" @change="loadStock(1)">
              <el-option label="采购入库" value="in" /><el-option label="领用出库" value="use" />
              <el-option label="退库" value="return" /><el-option label="盘盈" value="gain" />
              <el-option label="盘亏/报废" value="loss" /><el-option label="手动调整" value="adjust" />
            </el-select>
            <el-button type="primary" size="small" :icon="Search" @click="loadStock(1)">查询</el-button>
            <div class="flex1"></div>
          </div>
          <el-table :data="stockList" v-loading="stockLoading" stripe>
            <el-table-column prop="created_at" label="时间" width="170">
              <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column prop="material_name" label="物料" min-width="130" />
            <el-table-column label="类型" width="110">
              <template #default="{ row }"><el-tag :type="stockTypeTag(row.type)" size="small">{{ stockTypeLabel(row.type) }}</el-tag></template>
            </el-table-column>
            <el-table-column label="数量" width="80">
              <template #default="{ row }"><span :class="row.quantity < 0 ? 'red' : 'green'">{{ row.quantity > 0 ? '+' : '' }}{{ row.quantity }}</span></template>
            </el-table-column>
            <el-table-column label="金额" width="90">
              <template #default="{ row }">{{ fmtMoney(row.amount) }}</template>
            </el-table-column>
            <el-table-column prop="ref_type" label="来源" width="90" />
            <el-table-column prop="operator" label="操作人" width="90" />
            <el-table-column prop="note" label="备注" min-width="140" show-overflow-tooltip />
          </el-table>
          <div class="pager"><el-pagination background layout="total, prev, pager, next" :total="stockTotal" :page-size="pageSize" :current-page="stockPage" @current-change="loadStock" /></div>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <!-- 物料编辑 -->
    <el-dialog v-model="editVisible" :title="editForm.id ? '编辑物料' : '新增物料'" width="520px" :close-on-click-modal="false">
      <el-form ref="editRef" :model="editForm" :rules="editRules" label-width="90px">
        <el-form-item label="编码" prop="code"><el-input v-model="editForm.code" placeholder="如 LED-RED-01" /></el-form-item>
        <el-form-item label="名称" prop="name"><el-input v-model="editForm.name" /></el-form-item>
        <el-form-item label="分类"><el-select v-model="editForm.category" placeholder="选择或输入" filterable allow-create default-first-option style="width: 100%">
          <el-option v-for="c in categories" :key="c" :label="c" :value="c" /></el-select></el-form-item>
        <el-form-item label="规格"><el-input v-model="editForm.spec" /></el-form-item>
        <el-row>
          <el-col :span="12"><el-form-item label="单位"><el-input v-model="editForm.unit" placeholder="个/支/套/米" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="单价"><el-input-number v-model="editForm.unit_price" :min="0" :precision="2" style="width: 100%" /></el-form-item></el-col>
        </el-row>
        <el-row>
          <el-col :span="12"><el-form-item label="初始库存"><el-input-number v-model="editForm.stock" :min="0" :disabled="!!editForm.id" style="width: 100%" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="预警阈值"><el-input-number v-model="editForm.threshold" :min="0" style="width: 100%" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="绑定设备">
          <el-input-number v-model="editForm.device_hw_id" :min="0" placeholder="可选，设备耗材填写" style="width: 100%" />
        </el-form-item>
        <el-form-item label="默认供应商">
          <el-select v-model="editForm.supplier_id" placeholder="选择供应商" clearable filterable style="width: 100%">
            <el-option v-for="s in supplierOpts" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="editForm.note" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <!-- 库存调整 -->
    <el-dialog v-model="adjustVisible" title="手动调整库存" width="440px" :close-on-click-modal="false">
      <el-form label-width="90px">
        <el-form-item label="物料"><el-text>{{ adjustForm.materialName }}（当前 {{ adjustForm.currentStock }}）</el-text></el-form-item>
        <el-form-item label="类型">
          <el-select v-model="adjustForm.type" style="width: 100%">
            <el-option label="入库" value="in" /><el-option label="盘盈" value="gain" />
            <el-option label="盘亏/报废" value="loss" /><el-option label="退库" value="return" />
            <el-option label="手动调整" value="adjust" />
          </el-select>
        </el-form-item>
        <el-form-item label="数量"><el-input-number v-model="adjustForm.quantity" :min="1" style="width: 100%" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="adjustForm.note" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="adjustVisible = false">取消</el-button>
        <el-button type="primary" :loading="adjusting" @click="doAdjust">确认</el-button>
      </template>
    </el-dialog>

    <!-- 工单领料出库 -->
    <el-dialog v-model="useVisible" title="工单领料出库" width="460px" :close-on-click-modal="false">
      <el-form label-width="90px">
        <el-form-item label="物料"><el-text>{{ useForm.materialName }}（当前 {{ useForm.currentStock }}）</el-text></el-form-item>
        <el-form-item label="关联工单" required>
          <el-select v-model="useForm.work_order_id" filterable placeholder="选择工单" style="width: 100%">
            <el-option v-for="wo in workOrderOpts" :key="wo.id" :label="`${wo.order_no}${wo.assignee_name ? ' · ' + wo.assignee_name : ''}`" :value="wo.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="领用数量" required><el-input-number v-model="useForm.quantity" :min="1" style="width: 100%" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="useForm.note" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="useVisible = false">取消</el-button>
        <el-button type="primary" :loading="using" @click="doUse">确认领料</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Plus } from '@element-plus/icons-vue'
import { getMaterials, getMaterialStats, saveMaterial, deleteMaterial, getMaterialStocks, adjustStock, useStock } from '@/api/inventory'
import { getAllSuppliers } from '@/api/supplier'
import { getWorkOrders } from '@/api/workorder'
import { useAuthStore } from '@/store/auth'
import type { FormInstance, FormRules } from 'element-plus'

const authStore = useAuthStore()
const isOperator = computed(() => { const r = authStore.user?.role; return r === 'admin' || r === 'operator' })
const isAdmin = computed(() => authStore.user?.role === 'admin')

const categories = ['灯泡', '电源', '控制器', '线缆', '信号机', '其他']
const tab = ref('material')
const loading = ref(false)
const list = ref<any[]>([])
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const onlyLow = ref(false)
const filter = reactive({ keyword: '', category: '' })

const stats = reactive<any>({})

const stockList = ref<any[]>([])
const stockLoading = ref(false)
const stockPage = ref(1)
const stockTotal = ref(0)
const stockFilter = reactive({ type: '' })

const supplierOpts = ref<any[]>([])

async function load(p = page.value) {
  loading.value = true
  page.value = p
  try {
    const params: Record<string, any> = { page: p, page_size: pageSize.value }
    if (filter.keyword) params.keyword = filter.keyword
    if (filter.category) params.category = filter.category
    if (onlyLow.value) params.low_stock = 1
    const res = await getMaterials(params)
    list.value = res.data?.list || []
    total.value = res.data?.total || 0
  } catch { /* ignore */ } finally { loading.value = false }
}

async function loadStock(p = stockPage.value) {
  stockLoading.value = true
  stockPage.value = p
  try {
    const params: Record<string, any> = { page: p, page_size: pageSize.value }
    if (stockFilter.type) params.type = stockFilter.type
    const res = await getMaterialStocks(params)
    stockList.value = res.data?.list || []
    stockTotal.value = res.data?.total || 0
  } catch { /* ignore */ } finally { stockLoading.value = false }
}

async function loadStats() {
  try {
    const res = await getMaterialStats()
    Object.assign(stats, res.data || {})
  } catch { /* ignore */ }
}
async function loadSuppliers() {
  try { const res = await getAllSuppliers(); supplierOpts.value = res.data?.list || [] } catch { /* ignore */ }
}

// 编辑
const editVisible = ref(false)
const saving = ref(false)
const editRef = ref<FormInstance>()
const emptyForm = { id: 0, code: '', name: '', category: '', spec: '', unit: '', unit_price: 0, stock: 0, threshold: 0, device_hw_id: undefined, supplier_id: undefined, note: '', status: 'active' }
const editForm = reactive<any>({ ...emptyForm })
const editRules: FormRules = {
  code: [{ required: true, message: '请填写编码', trigger: 'blur' }],
  name: [{ required: true, message: '请填写名称', trigger: 'blur' }],
}
function openEdit(row?: any) {
  Object.assign(editForm, row ? { ...row, stock: row.stock } : { ...emptyForm })
  editVisible.value = true
}
async function save() {
  await editRef.value?.validate().catch(() => { throw new Error('invalid') })
  saving.value = true
  try {
    const payload: Record<string, any> = { ...editForm }
    // 设备绑定：空值发 null 以便清除绑定
    payload.device_hw_id = editForm.device_hw_id ? editForm.device_hw_id : null
    const res = await saveMaterial(payload)
    if (res.code === 0) { ElMessage.success('保存成功'); editVisible.value = false; load(); loadStats() }
    else ElMessage.error(res.msg || '保存失败')
  } catch (e: any) { ElMessage.error(e?.response?.data?.msg || '保存失败') } finally { saving.value = false }
}
async function del(row: any) {
  try { await ElMessageBox.confirm(`确定删除物料「${row.name}」吗？`, '警告', { type: 'warning' }) } catch { return }
  const res = await deleteMaterial(row.id)
  if (res.code === 0) { ElMessage.success('删除成功'); load(); loadStats() }
  else ElMessage.error(res.msg || '删除失败')
}

// 库存调整
const adjustVisible = ref(false)
const adjusting = ref(false)
const adjustForm = reactive<any>({ material_id: 0, materialName: '', currentStock: 0, type: 'in', quantity: 1, note: '' })
function openAdjust(row: any) {
  Object.assign(adjustForm, { material_id: row.id, materialName: row.name, currentStock: row.stock, type: 'in', quantity: 1, note: '' })
  adjustVisible.value = true
}
async function doAdjust() {
  adjusting.value = true
  try {
    const res = await adjustStock({ material_id: adjustForm.material_id, type: adjustForm.type, quantity: adjustForm.quantity, note: adjustForm.note })
    if (res.code === 0) { ElMessage.success('库存已调整'); adjustVisible.value = false; load(); loadStock(1); loadStats() }
    else ElMessage.error(res.msg || '调整失败')
  } catch (e: any) { ElMessage.error(e?.response?.data?.msg || '调整失败') } finally { adjusting.value = false }
}

// 工单领料出库
const useVisible = ref(false)
const using = ref(false)
const useForm = reactive<any>({ material_id: 0, materialName: '', currentStock: 0, work_order_id: undefined, quantity: 1, note: '' })
const workOrderOpts = ref<any[]>([])
async function loadWorkOrders() {
  try {
    const res = await getWorkOrders({ page: 1, page_size: 100 })
    workOrderOpts.value = res.data?.list || []
  } catch { /* ignore */ }
}
function openUse(row: any) {
  Object.assign(useForm, { material_id: row.id, materialName: row.name, currentStock: row.stock, work_order_id: undefined, quantity: 1, note: '' })
  loadWorkOrders()
  useVisible.value = true
}
async function doUse() {
  if (!useForm.work_order_id) { ElMessage.warning('请选择关联工单'); return }
  if (!useForm.quantity || useForm.quantity < 1) { ElMessage.warning('请填写领用数量'); return }
  using.value = true
  try {
    const res = await useStock({ material_id: useForm.material_id, quantity: useForm.quantity, work_order_id: useForm.work_order_id, note: useForm.note })
    if (res.code === 0) { ElMessage.success('领料出库成功'); useVisible.value = false; load(); loadStock(1); loadStats() }
    else ElMessage.error(res.msg || '领料失败')
  } catch (e: any) { ElMessage.error(e?.response?.data?.msg || '领料失败') } finally { using.value = false }
}

function stockTypeLabel(t: string) { return ({ in: '采购入库', use: '领用出库', return: '退库', gain: '盘盈', loss: '盘亏/报废', adjust: '手动调整' } as Record<string, string>)[t] || t }
function stockTypeTag(t: string) { return ({ in: 'success', use: 'warning', gain: '', loss: 'danger', adjust: 'info' } as Record<string, any>)[t] || 'info' }
function fmtMoney(v: any) { return v == null ? '0.00' : Number(v).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }
function fmtTime(s: string) { return (s || '').replace('T', ' ').slice(0, 19) }

onMounted(() => { load(1); loadStock(1); loadStats(); loadSuppliers() })
</script>

<style scoped>
.inv-page { padding: 4px; }
.stat-row { margin-bottom: 12px; }
.stat-card { text-align: center; }
.stat-num { font-size: 24px; font-weight: 700; color: #409EFF; }
.stat-num.warn { color: #E6A23C; }
.stat-label { color: #909399; font-size: 12px; margin-top: 4px; }
.filter-bar { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.flex1 { flex: 1; }
.pager { display: flex; justify-content: flex-end; margin-top: 12px; }
.low-num { color: #F56C6C; font-weight: 600; }
.red { color: #F56C6C; font-weight: 600; }
.green { color: #67C23A; font-weight: 600; }
</style>
