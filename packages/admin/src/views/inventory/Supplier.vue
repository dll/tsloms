<template>
  <div class="inv-page">
    <el-card shadow="never">
      <div class="filter-bar">
        <el-input v-model="keyword" placeholder="名称/联系人/电话" clearable size="small" style="width: 200px" @keyup.enter="load(1)" />
        <el-button type="primary" size="small" :icon="Search" @click="load(1)">查询</el-button>
        <div class="flex1"></div>
        <el-button v-if="isOperator" type="primary" size="small" :icon="Plus" @click="openEdit()">新增供应商</el-button>
      </div>

      <el-table :data="list" v-loading="loading" stripe>
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column prop="contact" label="联系人" width="100" />
        <el-table-column prop="phone" label="电话" width="130" />
        <el-table-column prop="email" label="邮箱" min-width="150" />
        <el-table-column prop="address" label="地址" min-width="180" show-overflow-tooltip />
        <el-table-column label="状态" width="80">
          <template #default="{ row }"><el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">{{ row.status === 'active' ? '启用' : '停用' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button v-if="isOperator" size="small" plain @click="openEdit(row)">编辑</el-button>
            <el-button v-if="isAdmin" size="small" type="danger" plain @click="del(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pager"><el-pagination background layout="total, prev, pager, next" :total="total" :page-size="pageSize" :current-page="page" @current-change="load" /></div>
    </el-card>

    <el-dialog v-model="editVisible" :title="editForm.id ? '编辑供应商' : '新增供应商'" width="520px" :close-on-click-modal="false">
      <el-form ref="editRef" :model="editForm" :rules="editRules" label-width="90px">
        <el-form-item label="名称" prop="name"><el-input v-model="editForm.name" /></el-form-item>
        <el-row>
          <el-col :span="12"><el-form-item label="联系人"><el-input v-model="editForm.contact" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="电话"><el-input v-model="editForm.phone" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="邮箱"><el-input v-model="editForm.email" /></el-form-item>
        <el-form-item label="地址"><el-input v-model="editForm.address" /></el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="editForm.status">
            <el-radio-button value="active">启用</el-radio-button>
            <el-radio-button value="disabled">停用</el-radio-button>
          </el-radio-group>
        </el-form-item>
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
import { Search, Plus } from '@element-plus/icons-vue'
import { getSuppliers, saveSupplier, deleteSupplier } from '@/api/supplier'
import { useAuthStore } from '@/store/auth'
import type { FormInstance, FormRules } from 'element-plus'

const authStore = useAuthStore()
const isOperator = computed(() => { const r = authStore.user?.role; return r === 'admin' || r === 'operator' })
const isAdmin = computed(() => authStore.user?.role === 'admin')

const list = ref<any[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const keyword = ref('')

async function load(p = page.value) {
  loading.value = true
  page.value = p
  try {
    const res = await getSuppliers({ page: p, page_size: pageSize.value, keyword: keyword.value || undefined })
    list.value = res.data?.list || []
    total.value = res.data?.total || 0
  } catch { /* ignore */ } finally { loading.value = false }
}

const editVisible = ref(false)
const saving = ref(false)
const editRef = ref<FormInstance>()
const editForm = reactive<any>({ id: 0, name: '', contact: '', phone: '', email: '', address: '', status: 'active', note: '' })
const editRules: FormRules = { name: [{ required: true, message: '请填写名称', trigger: 'blur' }] }
function openEdit(row?: any) {
  Object.assign(editForm, row ? { ...row } : { id: 0, name: '', contact: '', phone: '', email: '', address: '', status: 'active', note: '' })
  editVisible.value = true
}
async function save() {
  await editRef.value?.validate().catch(() => { throw new Error('invalid') })
  saving.value = true
  try {
    const res = await saveSupplier({ ...editForm })
    if (res.code === 0) { ElMessage.success('保存成功'); editVisible.value = false; load() }
    else ElMessage.error(res.msg || '保存失败')
  } catch (e: any) { ElMessage.error(e?.response?.data?.msg || '保存失败') } finally { saving.value = false }
}
async function del(row: any) {
  try { await ElMessageBox.confirm(`确定删除供应商「${row.name}」吗？`, '警告', { type: 'warning' }) } catch { return }
  const res = await deleteSupplier(row.id)
  if (res.code === 0) { ElMessage.success('删除成功'); load() }
  else ElMessage.error(res.msg || '删除失败')
}

onMounted(() => load(1))
</script>

<style scoped>
.inv-page { padding: 4px; }
.filter-bar { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.flex1 { flex: 1; }
.pager { display: flex; justify-content: flex-end; margin-top: 12px; }
</style>
