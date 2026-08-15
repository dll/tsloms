<template>
  <div class="inv-page">
    <el-card shadow="never">
      <div class="filter-bar">
        <el-input v-model="filter.order_no" placeholder="采购单号" clearable size="small" style="width: 170px" @keyup.enter="load(1)" />
        <el-select v-model="filter.supplier_id" placeholder="供应商" clearable filterable size="small" style="width: 170px" @change="load(1)">
          <el-option v-for="s in supplierOpts" :key="s.id" :label="s.name" :value="s.id" />
        </el-select>
        <el-select v-model="filter.status" placeholder="状态" clearable size="small" style="width: 120px" @change="load(1)">
          <el-option label="草稿" value="draft" /><el-option label="部分入库" value="partial" />
          <el-option label="已完成" value="completed" /><el-option label="已取消" value="cancelled" />
        </el-select>
        <el-button type="primary" size="small" :icon="Search" @click="load(1)">查询</el-button>
        <div class="flex1"></div>
        <el-button v-if="isOperator" type="primary" size="small" :icon="Plus" @click="openCreate">新建采购单</el-button>
      </div>

      <el-table :data="list" v-loading="loading" stripe>
        <el-table-column prop="order_no" label="采购单号" width="140" />
        <el-table-column prop="supplier_name" label="供应商" min-width="120" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }"><el-tag :type="statusTag(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="金额(元)" width="110">
          <template #default="{ row }">{{ fmtMoney(row.total_amount) }}</template>
        </el-table-column>
        <el-table-column prop="operator" label="经办人" width="90" />
        <el-table-column prop="note" label="备注" min-width="140" show-overflow-tooltip />
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <el-button size="small" plain @click="viewDetail(row)">详情</el-button>
            <el-button v-if="isOperator && ['draft','partial'].includes(row.status)" size="small" type="warning" plain @click="openReceive(row)">入库</el-button>
            <el-button v-if="isOperator && ['draft','partial'].includes(row.status)" size="small" type="info" plain @click="doCancel(row)">取消</el-button>
            <el-button v-if="isAdmin && row.status === 'draft'" size="small" type="danger" plain @click="doDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pager"><el-pagination background layout="total, prev, pager, next" :total="total" :page-size="pageSize" :current-page="page" @current-change="load" /></div>
    </el-card>

    <!-- 新建采购单 -->
    <el-dialog v-model="createVisible" title="新建采购单" width="680px" :close-on-click-modal="false">
      <el-form :model="createForm" label-width="90px">
        <el-form-item label="供应商">
          <el-select v-model="createForm.supplier_id" placeholder="选择供应商" filterable style="width: 100%">
            <el-option v-for="s in supplierOpts" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="采购明细">
          <div style="width: 100%">
            <div v-for="(it, i) in createForm.items" :key="i" class="po-item">
              <el-select v-model="it.material_id" placeholder="选择物料" filterable style="width: 40%">
                <el-option v-for="m in materialOpts" :key="m.id" :label="`${m.name} (${m.code})`" :value="m.id" />
              </el-select>
              <el-input-number v-model="it.quantity" :min="1" placeholder="数量" style="width: 22%" />
              <el-input-number v-model="it.price" :min="0" :precision="2" placeholder="单价" style="width: 24%" />
              <el-button type="danger" link :icon="Delete" @click="createForm.items.splice(i, 1)" />
            </div>
            <el-button size="small" link type="primary" :icon="Plus" @click="addItem">添加明细</el-button>
          </div>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="createForm.note" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="create">保存</el-button>
      </template>
    </el-dialog>

    <!-- 采购详情 -->
    <el-dialog v-model="detailVisible" title="采购单详情" width="560px">
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="单号">{{ detail?.order_no }}</el-descriptions-item>
        <el-descriptions-item label="状态"><el-tag :type="statusTag(detail?.status)" size="small">{{ statusLabel(detail?.status) }}</el-tag></el-descriptions-item>
        <el-descriptions-item label="供应商">{{ detail?.supplier_name }}</el-descriptions-item>
        <el-descriptions-item label="金额">{{ fmtMoney(detail?.total_amount) }} 元</el-descriptions-item>
      </el-descriptions>
      <el-table :data="detail?.items || []" size="small" style="margin-top: 12px" border>
        <el-table-column prop="material_name" label="物料" />
        <el-table-column prop="quantity" label="数量" width="70" />
        <el-table-column label="单价" width="90"><template #default="{ row }">{{ fmtMoney(row.price) }}</template></el-table-column>
        <el-table-column label="小计" width="90"><template #default="{ row }">{{ fmtMoney(row.amount) }}</template></el-table-column>
        <el-table-column label="已入库" width="70"><template #default="{ row }">{{ row.received_qty }}</template></el-table-column>
      </el-table>
    </el-dialog>

    <!-- 入库 -->
    <el-dialog v-model="receiveVisible" title="采购入库" width="540px" :close-on-click-modal="false">
      <el-alert type="info" :closable="false" style="margin-bottom: 12px">待入库明细，填入本次入库数量后提交。可部分入库。</el-alert>
      <div v-for="it in receiveItems" :key="it.id" class="po-item">
        <el-text>{{ it.material_name }}（剩 {{ it.quantity - it.received_qty }}/{{ it.quantity }}）</el-text>
        <el-input-number v-model="it.recvQty" :min="0" :max="it.quantity - it.received_qty" placeholder="本批入库" style="width: 160px" />
      </div>
      <template #footer>
        <el-button @click="receiveVisible = false">取消</el-button>
        <el-button type="primary" :loading="receiving" @click="doReceive">提交入库</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Plus, Delete } from '@element-plus/icons-vue'
import { getPurchases, getPurchaseDetail, createPurchase, receivePurchase, cancelPurchase, deletePurchase } from '@/api/purchase'
import { getAllSuppliers } from '@/api/supplier'
import { getMaterials } from '@/api/inventory'
import { useAuthStore } from '@/store/auth'

const authStore = useAuthStore()
const isOperator = computed(() => { const r = authStore.user?.role; return r === 'admin' || r === 'operator' })
const isAdmin = computed(() => authStore.user?.role === 'admin')

const list = ref<any[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const filter = reactive({ order_no: '', supplier_id: undefined as number | undefined, status: '' })

const supplierOpts = ref<any[]>([])
const materialOpts = ref<any[]>([])

async function load(p = page.value) {
  loading.value = true
  page.value = p
  try {
    const params: Record<string, any> = { page: p, page_size: pageSize.value }
    if (filter.order_no) params.order_no = filter.order_no
    if (filter.supplier_id) params.supplier_id = filter.supplier_id
    if (filter.status) params.status = filter.status
    const res = await getPurchases(params)
    list.value = res.data?.list || []
    total.value = res.data?.total || 0
  } catch { /* ignore */ } finally { loading.value = false }
}

// 新建
const createVisible = ref(false)
const creating = ref(false)
const createForm = reactive<any>({ supplier_id: undefined, note: '', items: [] })
const emptyTpl = () => ({ material_id: undefined, quantity: 1, price: 0 })
function openCreate() {
  Object.assign(createForm, { supplier_id: undefined, note: '', items: [emptyTpl()] })
  createVisible.value = true
}
function addItem() { createForm.items.push(emptyTpl()) }
async function create() {
  if (!createForm.supplier_id) { ElMessage.warning('请选择供应商'); return }
  if (!createForm.items.length || !createForm.items[0].material_id) { ElMessage.warning('请添加采购明细'); return }
  creating.value = true
  try {
    const items = createForm.items.filter((it: any) => it.material_id).map((it: any) => ({ material_id: it.material_id, quantity: it.quantity, price: it.price }))
    const res = await createPurchase({ supplier_id: createForm.supplier_id, note: createForm.note, items })
    if (res.code === 0) { ElMessage.success('采购单已创建'); createVisible.value = false; load(1) }
    else ElMessage.error(res.msg || '创建失败')
  } catch (e: any) { ElMessage.error(e?.response?.data?.msg || '创建失败') } finally { creating.value = false }
}

// 详情
const detailVisible = ref(false)
const detail = ref<any>(null)
async function viewDetail(row: any) {
  const res = await getPurchaseDetail(row.id)
  detail.value = res.data?.purchase || null
  detailVisible.value = true
}

// 入库
const receiveVisible = ref(false)
const receiving = ref(false)
const receiveItems = ref<any[]>([])
const receiveOrderId = ref(0)
async function openReceive(row: any) {
  receiveOrderId.value = row.id
  const res = await getPurchaseDetail(row.id)
  const items: any[] = (res.data?.purchase?.items || []).map((it: any) => ({ ...it, recvQty: it.quantity - (it.received_qty || 0) }))
  receiveItems.value = items
  receiveVisible.value = true
}
async function doReceive() {
  const items = receiveItems.value.filter((it) => it.recvQty > 0).map((it) => ({ item_id: it.id, quantity: it.recvQty }))
  if (!items.length) { ElMessage.warning('请填写入库数量'); return }
  receiving.value = true
  try {
    const res = await receivePurchase(receiveOrderId.value, items)
    if (res.code === 0) { ElMessage.success('入库成功'); receiveVisible.value = false; load() }
    else ElMessage.error(res.msg || '入库失败')
  } catch (e: any) { ElMessage.error(e?.response?.data?.msg || '入库失败') } finally { receiving.value = false }
}

async function doCancel(row: any) {
  try { await ElMessageBox.confirm(`确定取消采购单 ${row.order_no} 吗？`, '提示', { type: 'warning' }) } catch { return }
  const res = await cancelPurchase(row.id)
  if (res.code === 0) { ElMessage.success('已取消'); load() } else ElMessage.error(res.msg || '取消失败')
}
async function doDelete(row: any) {
  try { await ElMessageBox.confirm(`确定删除采购单 ${row.order_no} 吗？`, '警告', { type: 'warning' }) } catch { return }
  const res = await deletePurchase(row.id)
  if (res.code === 0) { ElMessage.success('已删除'); load() } else ElMessage.error(res.msg || '删除失败')
}

function statusLabel(s: string) { return ({ draft: '草稿', partial: '部分入库', completed: '已完成', cancelled: '已取消' } as Record<string, string>)[s] || s }
function statusTag(s: string) { return ({ draft: 'info', partial: 'warning', completed: 'success', cancelled: 'danger' } as Record<string, any>)[s] || 'info' }
function fmtMoney(v: any) { return v == null ? '0.00' : Number(v).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }
function fmtTime(s: string) { return (s || '').replace('T', ' ').slice(0, 19) }

onMounted(async () => {
  load(1)
  try { const s = await getAllSuppliers(); supplierOpts.value = s.data?.list || [] } catch { /* ignore */ }
  try { const m = await getMaterials({ page_size: 200 }); materialOpts.value = m.data?.list || [] } catch { /* ignore */ }
})
</script>

<style scoped>
.inv-page { padding: 4px; }
.filter-bar { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.flex1 { flex: 1; }
.pager { display: flex; justify-content: flex-end; margin-top: 12px; }
.po-item { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
</style>
