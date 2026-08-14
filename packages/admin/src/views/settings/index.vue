<template>
  <div class="settings-page">
    <el-tabs v-model="activeTab" type="card">
      <!-- 用户信息 -->
      <el-tab-pane label="用户信息" name="profile">
        <el-card shadow="never" class="info-card">
          <template #header><span>我的资料</span></template>
          <el-descriptions :column="2" border>
            <el-descriptions-item label="用户名">{{ authStore.user?.username || '-' }}</el-descriptions-item>
            <el-descriptions-item label="角色">{{ roleLabel }}</el-descriptions-item>
            <el-descriptions-item label="手机号">{{ authStore.user?.phone || '-' }}</el-descriptions-item>
            <el-descriptions-item label="用户ID">{{ authStore.user?.id || '-' }}</el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card shadow="never" class="form-card">
          <template #header><span>修改手机号</span></template>
          <el-form ref="formRef" :model="phoneForm" :rules="rules" label-width="100px" style="max-width: 500px">
            <el-form-item label="新手机号" prop="phone">
              <el-input v-model="phoneForm.phone" placeholder="请输入新手机号" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card shadow="never" class="form-card">
          <template #header><span>地图中心点（地图大屏以当前用户为中心定位）</span></template>
          <el-form label-width="100px" style="max-width: 500px">
            <el-form-item label="经度">
              <el-input v-model="centerForm.lng" placeholder="如 121.4737" />
            </el-form-item>
            <el-form-item label="纬度">
              <el-input v-model="centerForm.lat" placeholder="如 31.2304" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="centerSaving" @click="handleSaveCenter">保存中心点</el-button>
              <el-button v-if="hasCenter" @click="handleClearCenter">清除</el-button>
              <span class="tip">不设置时地图自动聚焦到设备分布区域</span>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <!-- 用户管理（仅管理员） -->
      <el-tab-pane v-if="authStore.user?.role === 'admin'" label="用户管理" name="users">
        <el-card shadow="never">
          <div class="user-toolbar">
            <div class="user-search">
              <el-input v-model="query.keyword" placeholder="搜索用户名" clearable style="width: 200px" @keyup.enter="loadUsers" />
              <el-select v-model="query.role" placeholder="全部角色" clearable style="width: 140px">
                <el-option label="管理员" value="admin" />
                <el-option label="运维人员" value="operator" />
                <el-option label="查看人员" value="viewer" />
              </el-select>
              <el-button @click="loadUsers">查询</el-button>
            </div>
            <el-button type="primary" @click="openCreate">新增用户</el-button>
          </div>

          <el-table :data="users" border stripe style="width: 100%" v-loading="loading">
            <el-table-column prop="id" label="ID" width="70" align="center" />
            <el-table-column prop="username" label="用户名" min-width="120" />
            <el-table-column label="角色" width="110" align="center">
              <template #default="{ row }">
                <el-tag :type="roleTagType(row.role)" size="small">{{ roleText(row.role) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="phone" label="手机号" width="140" align="center" />
            <el-table-column prop="created_at" label="创建时间" width="170" align="center" />
            <el-table-column label="操作" width="200" align="center">
              <template #default="{ row }">
                <el-button size="small" @click="openEdit(row)">编辑</el-button>
                <el-button size="small" type="warning" @click="openResetPwd(row)">重置密码</el-button>
                <el-button size="small" type="danger" :disabled="row.username === 'admin'" @click="handleDelete(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-pagination
            style="margin-top: 12px; justify-content: flex-end"
            layout="total, prev, pager, next"
            :total="total"
            :page-size="query.page_size"
            v-model:current-page="query.page"
            @current-change="loadUsers"
          />
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 新增/编辑用户对话框 -->
    <el-dialog v-model="editVisible" :title="editingId ? '编辑用户' : '新增用户'" width="460px">
      <el-form :model="editForm" label-width="90px">
        <el-form-item label="用户名" required>
          <el-input v-model="editForm.username" :disabled="!!editingId" placeholder="用户名" />
        </el-form-item>
        <el-form-item v-if="!editingId" label="密码" required>
          <el-input v-model="editForm.password" type="password" show-password placeholder="至少6位" />
        </el-form-item>
        <el-form-item label="角色" required>
          <el-select v-model="editForm.role" style="width: 100%">
            <el-option label="管理员" value="admin" />
            <el-option label="运维人员" value="operator" />
            <el-option label="查看人员" value="viewer" />
          </el-select>
        </el-form-item>
        <el-form-item label="手机号">
          <el-input v-model="editForm.phone" placeholder="手机号（选填）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingUser" @click="saveUser">保存</el-button>
      </template>
    </el-dialog>

    <!-- 重置密码对话框 -->
    <el-dialog v-model="resetVisible" title="重置密码" width="420px">
      <el-form label-width="90px">
        <el-form-item label="新密码" required>
          <el-input v-model="resetPassword" type="password" show-password placeholder="至少6位" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetVisible = false">取消</el-button>
        <el-button type="warning" :loading="resetting" @click="doResetPwd">确认重置</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { useAuthStore } from '@/store/auth'
import { updateMyPhone } from '@/api/auth'
import { getUsers, createUser, updateUser, resetUserPassword, deleteUser, type UserItem } from '@/api/user'

const authStore = useAuthStore()
const activeTab = ref('profile')

const roleLabel = computed(() => {
  const role = authStore.user?.role
  const map: Record<string, string> = { admin: '管理员', operator: '运维人员', viewer: '查看人员' }
  return role ? (map[role] || role) : '-'
})

// 修改手机号
const formRef = ref<FormInstance>()
const saving = ref(false)
const phoneForm = reactive({ phone: '' })
const rules: FormRules = {
  phone: [
    { required: true, message: '请输入手机号', trigger: 'blur' },
    { pattern: /^1[3-9]\d{9}$/, message: '请输入正确的手机号', trigger: 'blur' },
  ],
}
async function handleSave() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    saving.value = true
    try {
      await updateMyPhone(phoneForm.phone)
      authStore.user!.phone = phoneForm.phone
      ElMessage.success('手机号修改成功')
    } catch { /* 忽略 */ } finally { saving.value = false }
  })
}

// ---- 地图中心点（当前用户管辖区域） ----
const centerForm = reactive({ lat: '', lng: '' })
const centerSaving = ref(false)
const hasCenter = ref(false)
async function loadCenter() {
  // 尽量从服务端拉取最新用户信息（含地图中心点），避免依赖登录时的快照
  if (authStore.token) {
    try { await authStore.fetchUserInfo() } catch { /* 忽略 */ }
  }
  const u = authStore.user as any
  if (u && u.center_lat != null && u.center_lng != null) {
    centerForm.lat = String(u.center_lat)
    centerForm.lng = String(u.center_lng)
    hasCenter.value = true
  } else {
    centerForm.lat = ''
    centerForm.lng = ''
    hasCenter.value = false
  }
}
async function handleSaveCenter() {
  const lat = parseFloat(centerForm.lat)
  const lng = parseFloat(centerForm.lng)
  if (isNaN(lat) || lat < -90 || lat > 90) { ElMessage.warning('纬度范围 -90~90'); return }
  if (isNaN(lng) || lng < -180 || lng > 180) { ElMessage.warning('经度范围 -180~180'); return }
  centerSaving.value = true
  try {
    const { updateMyCenter } = await import('@/api/auth')
    await updateMyCenter(lat, lng)
    ;(authStore.user as any).center_lat = lat
    ;(authStore.user as any).center_lng = lng
    hasCenter.value = true
    ElMessage.success('地图中心点已保存（地图大屏将以该点为中心）')
  } catch { /* 后端提示 */ } finally { centerSaving.value = false }
}
async function handleClearCenter() {
  try {
    await ElMessageBox.confirm('确认清除地图中心点？清除后地图自动聚焦到设备分布区域。', '提示', { type: 'warning' })
    const { updateMyCenter } = await import('@/api/auth')
    await updateMyCenter(null, null)
    ;(authStore.user as any).center_lat = null
    ;(authStore.user as any).center_lng = null
    loadCenter()
    ElMessage.success('已清除')
  } catch { /* 取消 */ }
}

// ---- 用户管理 ----
const loading = ref(false)
const users = ref<UserItem[]>([])
const total = ref(0)
const query = reactive({ page: 1, page_size: 20, role: '', keyword: '' })

function roleText(r: string) { return ({ admin: '管理员', operator: '运维人员', viewer: '查看人员' } as Record<string, string>)[r] || r }
function roleTagType(r: string) { return r === 'admin' ? 'danger' : r === 'operator' ? 'warning' : 'info' }

async function loadUsers() {
  loading.value = true
  try {
    const res = await getUsers({
      page: query.page, page_size: query.page_size,
      role: query.role || undefined, keyword: query.keyword || undefined,
    })
    users.value = res.data?.list || []
    total.value = res.data?.total || 0
  } catch { ElMessage.error('用户列表加载失败') } finally { loading.value = false }
}

// 新增 / 编辑
const editVisible = ref(false)
const editingId = ref<number | null>(null)
const savingUser = ref(false)
const editForm = reactive({ username: '', password: '', role: 'viewer', phone: '' })

function openCreate() {
  editingId.value = null
  Object.assign(editForm, { username: '', password: '', role: 'viewer', phone: '' })
  editVisible.value = true
}
function openEdit(row: UserItem) {
  editingId.value = row.id
  Object.assign(editForm, { username: row.username, password: '', role: row.role, phone: row.phone || '' })
  editVisible.value = true
}
async function saveUser() {
  if (!editingId.value && (!editForm.username || editForm.password.length < 6)) {
    ElMessage.warning('用户名与至少6位密码必填')
    return
  }
  savingUser.value = true
  try {
    if (editingId.value) {
      await updateUser(editingId.value, { role: editForm.role, phone: editForm.phone })
      ElMessage.success('用户已更新')
    } else {
      await createUser({ username: editForm.username, password: editForm.password, role: editForm.role, phone: editForm.phone })
      ElMessage.success('用户已创建')
    }
    editVisible.value = false
    loadUsers()
  } catch { /* 后端已提示 */ } finally { savingUser.value = false }
}

// 重置密码
const resetVisible = ref(false)
const resetTarget = ref<UserItem | null>(null)
const resetPassword = ref('')
const resetting = ref(false)
function openResetPwd(row: UserItem) { resetTarget.value = row; resetPassword.value = ''; resetVisible.value = true }
async function doResetPwd() {
  if (!resetTarget.value || resetPassword.value.length < 6) { ElMessage.warning('密码至少6位'); return }
  resetting.value = true
  try {
    await resetUserPassword(resetTarget.value.id, resetPassword.value)
    ElMessage.success('密码已重置')
    resetVisible.value = false
  } catch { /* 忽略 */ } finally { resetting.value = false }
}

// 删除
async function handleDelete(row: UserItem) {
  try {
    await ElMessageBox.confirm(`确认删除用户「${row.username}」？`, '提示', { type: 'warning' })
    await deleteUser(row.id)
    ElMessage.success('用户已删除')
    loadUsers()
  } catch { /* 取消或失败 */ }
}

onMounted(async () => {
  if (authStore.token && !authStore.user) {
    try { await authStore.fetchUserInfo() } catch { /* 忽略 */ }
  }
  loadCenter()
  if (authStore.user?.role === 'admin') loadUsers()
})
</script>

<style scoped>
.settings-page { max-width: 960px; }
.info-card { margin-bottom: 20px; }
.form-card { border-radius: 4px; }
.user-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.user-search { display: flex; gap: 8px; }
</style>
