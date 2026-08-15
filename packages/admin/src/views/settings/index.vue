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

      <!-- 用户管理（仅“用户-管理”权限） -->
      <el-tab-pane v-if="authStore.hasPerm('user:manage')" label="用户管理" name="users">
        <el-card shadow="never">
          <div class="user-toolbar">
            <div class="user-search">
              <el-input v-model="query.keyword" placeholder="搜索用户名/姓名" clearable style="width: 180px" @keyup.enter="loadUsers" />
              <el-select v-model="query.role" placeholder="全部角色" clearable style="width: 130px">
                <el-option label="管理员" value="admin" />
                <el-option label="运维人员" value="operator" />
                <el-option label="查看人员" value="viewer" />
              </el-select>
              <el-select v-model="query.department_id" placeholder="全部部门" clearable style="width: 150px">
                <el-option v-for="d in departments" :key="d.id" :label="d.name" :value="d.id" />
              </el-select>
              <el-select v-model="query.status" placeholder="全部状态" clearable style="width: 120px">
                <el-option label="启用" value="enabled" />
                <el-option label="停用" value="disabled" />
              </el-select>
              <el-button @click="loadUsers">查询</el-button>
            </div>
            <el-button type="primary" @click="openCreate">新增用户</el-button>
          </div>

          <el-table :data="users" border stripe style="width: 100%" v-loading="loading">
            <el-table-column prop="id" label="ID" width="60" align="center" />
            <el-table-column prop="username" label="用户名" min-width="100" />
            <el-table-column prop="real_name" label="姓名" width="100" align="center">
              <template #default="{ row }">{{ row.real_name || '-' }}</template>
            </el-table-column>
            <el-table-column label="角色" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="roleTagType(row.role)" size="small">{{ roleText(row.role) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="department" label="部门" width="120" align="center">
              <template #default="{ row }">{{ row.department || '-' }}</template>
            </el-table-column>
            <el-table-column prop="phone" label="手机号" width="120" align="center" />
            <el-table-column label="状态" width="90" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === 'enabled' ? 'success' : 'danger'" size="small" effect="plain">
                  {{ row.status === 'enabled' ? '启用' : '停用' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="最后登录" width="160" align="center">
              <template #default="{ row }">{{ row.last_login_at ? formatTime(row.last_login_at) : '-' }}</template>
            </el-table-column>
            <el-table-column prop="created_at" label="创建时间" width="150" align="center">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="210" align="center" fixed="right">
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

      <el-tab-pane v-if="authStore.hasPerm('role:manage')" label="角色管理" name="roles">
        <el-card shadow="never">
          <div class="user-toolbar">
            <span class="dept-tip">角色决定用户默认拥有的功能权限；可为用户单独覆写（更多粒度）</span>
            <el-button type="primary" @click="openRoleCreate">新增角色</el-button>
          </div>
          <el-table :data="roles" border stripe style="width: 100%" v-loading="roleLoading">
            <el-table-column prop="id" label="ID" width="60" align="center" />
            <el-table-column prop="name" label="角色名称" width="120" align="center">
              <template #default="{ row }">
                {{ row.name }}
                <el-tag v-if="row.builtin" size="small" effect="plain" type="info" style="margin-left:4px">内置</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="code" label="角色编码" width="130" align="center" />
            <el-table-column label="功能权限" min-width="260">
              <template #default="{ row }">{{ permNames(row.permissions) }}</template>
            </el-table-column>
            <el-table-column prop="description" label="描述" min-width="140">
              <template #default="{ row }">{{ row.description || '-' }}</template>
            </el-table-column>
            <el-table-column label="操作" width="150" align="center" fixed="right">
              <template #default="{ row }">
                <template v-if="!row.builtin">
                  <el-button size="small" @click="openRoleEdit(row)">编辑</el-button>
                  <el-button size="small" type="danger" @click="handleRoleDelete(row)">删除</el-button>
                </template>
                <el-tag v-else size="small" effect="plain">内置不可改</el-tag>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- 组织/部门管理（仅“组织-管理”权限） -->
      <el-tab-pane v-if="authStore.hasPerm('dept:manage')" label="组织管理" name="departments">
        <el-card shadow="never">
          <div class="user-toolbar">
            <span class="dept-tip">部门用于组织用户、按管辖区域分工管理</span>
            <el-button type="primary" @click="openDeptCreate">新增部门</el-button>
          </div>
          <el-table :data="departments" border stripe style="width: 100%" v-loading="deptLoading">
            <el-table-column prop="id" label="ID" width="60" align="center" />
            <el-table-column prop="name" label="部门名称" min-width="120" />
            <el-table-column label="上级部门" width="120" align="center">
              <template #default="{ row }">{{ parentName(row.parent_id) }}</template>
            </el-table-column>
            <el-table-column prop="leader" label="负责人" width="120" align="center">
              <template #default="{ row }">{{ row.leader || '-' }}</template>
            </el-table-column>
            <el-table-column prop="member_count" label="人数" width="80" align="center" />
            <el-table-column prop="description" label="描述" min-width="160">
              <template #default="{ row }">{{ row.description || '-' }}</template>
            </el-table-column>
            <el-table-column prop="created_at" label="创建时间" width="150" align="center">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="140" align="center" fixed="right">
              <template #default="{ row }">
                <el-button size="small" @click="openDeptEdit(row)">编辑</el-button>
                <el-button size="small" type="danger" @click="handleDeptDelete(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 新增/编辑用户对话框 -->
    <el-dialog v-model="editVisible" :title="editingId ? '编辑用户' : '新增用户'" width="520px">
      <el-form :model="editForm" label-width="90px">
        <el-form-item label="用户名" required>
          <el-input v-model="editForm.username" :disabled="!!editingId" placeholder="用户名" />
        </el-form-item>
        <el-form-item v-if="!editingId" label="密码" required>
          <el-input v-model="editForm.password" type="password" show-password placeholder="至少6位" />
        </el-form-item>
        <el-form-item label="姓名">
          <el-input v-model="editForm.real_name" placeholder="真实姓名（选填）" />
        </el-form-item>
        <el-form-item label="角色" required>
          <el-select v-model="editForm.role" style="width: 100%">
            <el-option label="管理员" value="admin" />
            <el-option label="运维人员" value="operator" />
            <el-option label="查看人员" value="viewer" />
          </el-select>
        </el-form-item>
        <el-form-item label="所属部门">
          <el-select v-model="editForm.department_id" clearable placeholder="选择部门（选填）" style="width: 100%">
            <el-option v-for="d in departments" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="手机号">
          <el-input v-model="editForm.phone" placeholder="手机号（选填）" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="editForm.email" placeholder="邮箱（选填）" />
        </el-form-item>
        <el-form-item v-if="editingId" label="状态">
          <el-radio-group v-model="editForm.status">
            <el-radio value="enabled">启用</el-radio>
            <el-radio value="disabled">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="功能权限">
          <div style="width: 100%">
            <el-checkbox v-model="permCustom">自定义功能权限（不勾选则继承角色默认）</el-checkbox>
            <div v-if="permCustom" class="perm-box">
              <div v-for="group in permGroups" :key="group.module" class="perm-group">
                <div class="perm-module">{{ moduleName(group.module) }}</div>
                <el-checkbox-group v-model="permSelected">
                  <el-checkbox v-for="p in group.permissions" :key="p.code" :value="p.code">{{ p.name }}</el-checkbox>
                </el-checkbox-group>
              </div>
            </div>
            <div v-else class="perm-box perm-inherit">当前角色将提供默认权限，保存后仍可在“角色管理”中查看</div>
          </div>
        </el-form-item>
      </el-form>
      <div class="role-hint" v-if="!editingId">
        <span>角色权限：</span>
        <el-tag size="small" type="danger">管理员</el-tag> 全部权限
        <el-tag size="small" type="warning" style="margin-left:8px">运维人员</el-tag> 故障处置/工单/派单/媒体
        <el-tag size="small" type="info" style="margin-left:8px">查看人员</el-tag> 仅查看
      </div>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingUser" @click="saveUser">保存</el-button>
      </template>
    </el-dialog>

    <!-- 部门新增/编辑对话框 -->
    <el-dialog v-model="deptVisible" :title="deptEditingId ? '编辑部门' : '新增部门'" width="480px">
      <el-form :model="deptForm" label-width="90px">
        <el-form-item label="部门名称" required>
          <el-input v-model="deptForm.name" placeholder="如：运维一部" />
        </el-form-item>
        <el-form-item label="上级部门">
          <el-select v-model="deptForm.parent_id" clearable placeholder="选择上级（选填）" style="width: 100%">
            <el-option v-for="d in departments.filter((x) => x.id !== deptEditingId)" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="负责人">
          <el-input v-model="deptForm.leader" placeholder="负责人姓名（选填）" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="deptForm.description" type="textarea" :rows="2" placeholder="部门职责描述（选填）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="deptVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingDept" @click="saveDept">保存</el-button>
      </template>
    </el-dialog>

    <!-- 角色新增/编辑对话框 -->
    <el-dialog v-model="roleVisible" :title="roleEditingId ? '编辑角色' : '新增角色'" width="640px">
      <el-form :model="roleForm" label-width="90px">
        <el-form-item label="角色编码" required>
          <el-input v-model="roleForm.code" :disabled="!!roleEditingId" placeholder="如：area_admin（小写字母/数字/下划线）" />
        </el-form-item>
        <el-form-item label="角色名称" required>
          <el-input v-model="roleForm.name" placeholder="如：片区管理员" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="roleForm.description" placeholder="角色职责描述（选填）" />
        </el-form-item>
        <el-form-item label="功能权限">
          <div class="perm-box">
            <div v-for="group in permGroups" :key="group.module" class="perm-group">
              <div class="perm-module">{{ moduleName(group.module) }}</div>
              <el-checkbox-group v-model="roleForm.permissions">
                <el-checkbox v-for="p in group.permissions" :key="p.code" :value="p.code">{{ p.name }}</el-checkbox>
              </el-checkbox-group>
            </div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="roleVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingRole" @click="saveRole">保存</el-button>
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
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { useAuthStore } from '@/store/auth'
import { updateMyPhone } from '@/api/auth'
import { getUsers, createUser, updateUser, resetUserPassword, deleteUser, type UserItem } from '@/api/user'
import { getDepartments, createDepartment, updateDepartment, deleteDepartment, type DepartmentItem } from '@/api/department'
import { listPermissions, listRoles, createRole, updateRole, deleteRole, getUserPermissions, setUserPermissions, type RoleItem } from '@/api/rbac'

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
const query = reactive({ page: 1, page_size: 20, role: '', status: '', department_id: undefined as number | undefined, keyword: '' })

function roleText(r: string) { return ({ admin: '管理员', operator: '运维人员', viewer: '查看人员' } as Record<string, string>)[r] || r }
function roleTagType(r: string) { return r === 'admin' ? 'danger' : r === 'operator' ? 'warning' : 'info' }
function formatTime(t: string) {
  if (!t) return '-'
  const d = new Date(t)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

async function loadUsers() {
  loading.value = true
  try {
    const res = await getUsers({
      page: query.page, page_size: query.page_size,
      role: query.role || undefined,
      status: query.status || undefined,
      department_id: query.department_id || undefined,
      keyword: query.keyword || undefined,
    })
    users.value = res.data?.list || []
    total.value = res.data?.total || 0
  } catch { ElMessage.error('用户列表加载失败') } finally { loading.value = false }
}

// 新增 / 编辑
const editVisible = ref(false)
const editingId = ref<number | null>(null)
const savingUser = ref(false)
const editForm = reactive({ username: '', password: '', role: 'viewer', real_name: '', phone: '', email: '', department_id: undefined as number | undefined, status: 'enabled' })

function openCreate() {
  editingId.value = null
  Object.assign(editForm, { username: '', password: '', role: 'viewer', real_name: '', phone: '', email: '', department_id: undefined, status: 'enabled' })
  permCustom.value = false
  permSelected.value = [...(rolePermMap.value['viewer'] || [])]
  editVisible.value = true
}
function openEdit(row: UserItem) {
  editingId.value = row.id
  Object.assign(editForm, {
    username: row.username, password: '', role: row.role,
    real_name: row.real_name || '', phone: row.phone || '', email: row.email || '',
    department_id: row.department_id || undefined, status: row.status || 'enabled',
  })
  editVisible.value = true
  // 异步预填该用户功能权限
  prefillUserPerms()
}
async function saveUser() {
  if (!editingId.value && (!editForm.username || editForm.password.length < 6)) {
    ElMessage.warning('用户名与至少6位密码必填')
    return
  }
  savingUser.value = true
  try {
    if (editingId.value) {
      await updateUser(editingId.value, { role: editForm.role, real_name: editForm.real_name, phone: editForm.phone, email: editForm.email, department_id: editForm.department_id, status: editForm.status })
      // 保存功能权限覆写
      await saveUserPerms(editingId.value)
      ElMessage.success('用户已更新')
    } else {
      const created = await createUser({ username: editForm.username, password: editForm.password, role: editForm.role, real_name: editForm.real_name, phone: editForm.phone, email: editForm.email, department_id: editForm.department_id })
      // 若选择了自定义权限，则对新建用户设置覆写
      if (permCustom.value) {
        const uid = created.data?.id
        if (uid) await saveUserPerms(uid)
      }
      ElMessage.success('用户已创建')
    }
    editVisible.value = false
    loadUsers()
  } catch { /* 后端已提示 */ } finally { savingUser.value = false }
}

// 保存用户功能权限覆写：
//  - 未自定义 → 清空覆写（继承角色默认）
//  - 自定义   → 以勾选项为显式授权；角色默认中未勾选的记为显式拒绝
async function saveUserPerms(uid: number) {
  if (!permCustom.value) {
    await setUserPermissions(uid, { grants: [], denies: [] })
    return
  }
  const roleDefaults = rolePermMap.value[editForm.role] || []
  const grants = [...permSelected.value]
  const denies = roleDefaults.filter((c) => !permSelected.value.includes(c))
  await setUserPermissions(uid, { grants, denies })
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

// ---- 功能权限（权限字典 + 用户权限覆写） ----
const permGroups = ref<{ module: string; permissions: { code: string; name: string }[] }[]>([])
const permCustom = ref(false) // 用户对话框是否自定义权限
const permSelected = ref<string[]>([]) // 用户对话框已选权限
// 角色默认权限映射（用于新建用户时预填）
const rolePermMap = ref<Record<string, string[]>>({})

function moduleName(mod: string) {
  const map: Record<string, string> = {
    device: '设备管理', intersection: '路口管理', fault: '故障管理', workorder: '工单管理',
    media: '媒体管理', firmware: '固件管理', inventory: '库存管理', supplier: '供应商',
    purchase: '采购管理', expense: '维修费用', user: '用户管理', dept: '组织管理', role: '角色管理', ai: 'AI 功能',
  }
  return map[mod] || mod
}

async function loadPermissionsDict() {
  try {
    const res = await listPermissions()
    permGroups.value = res.data?.list || []
  } catch { /* 权限字典加载失败 */ }
}

// 角色列表（含每个角色默认权限），用于新建用户预填 + 角色管理
const roleLoading = ref(false)
const roles = ref<RoleItem[]>([])
async function loadRoles() {
  roleLoading.value = true
  try {
    const res = await listRoles()
    roles.value = res.data?.list || []
    const map: Record<string, string[]> = {}
    for (const r of roles.value) map[r.code] = r.permissions || []
    rolePermMap.value = map
  } catch { /* 忽略 */ } finally { roleLoading.value = false }
}

function permNames(codes: string[]) {
  if (!codes || !codes.length) return '—'
  const byCode = new Map<string, string>()
  for (const g of permGroups.value) for (const p of g.permissions) byCode.set(p.code, p.name)
  return codes.map((c) => byCode.get(c) || c).join('、')
}

// 打开新增/编辑用户时，预填权限
function prefillUserPerms() {
  permCustom.value = false
  permSelected.value = []
  if (editingId.value) {
    // 编辑：拉取该用户当前有效权限
    getUserPermissions(editingId.value).then((res) => {
      const d = res.data
      if (d && d.user_grants && d.user_grants.length) {
        permSelected.value = d.user_grants
        permCustom.value = true
      } else if (d && d.user_denies && d.user_denies.length) {
        // 有显式拒绝：默认全选角色默认，去掉被拒项，并开启自定义
        const defs = [...(d.role_defaults || [])]
        permSelected.value = defs.filter((c) => !d.user_denies.includes(c))
        permCustom.value = true
      } else {
        permSelected.value = d?.role_defaults || []
      }
    }).catch(() => {})
  } else {
    // 新建：以当前选择角色默认权限预填
    permSelected.value = [...(rolePermMap.value[editForm.role] || [])]
  }
}

// ---- 角色管理 ----
const roleVisible = ref(false)
const roleEditingId = ref<number | null>(null)
const savingRole = ref(false)
const roleForm = reactive({ code: '', name: '', description: '', permissions: [] as string[] })

function openRoleCreate() {
  roleEditingId.value = null
  Object.assign(roleForm, { code: '', name: '', description: '', permissions: [] })
  roleVisible.value = true
}
function openRoleEdit(row: RoleItem) {
  roleEditingId.value = row.id
  Object.assign(roleForm, { code: row.code, name: row.name, description: row.description || '', permissions: [...(row.permissions || [])] })
  roleVisible.value = true
}
async function saveRole() {
  if (!roleForm.code || !roleForm.name) { ElMessage.warning('角色编码与名称必填'); return }
  savingRole.value = true
  try {
    if (roleEditingId.value) {
      await updateRole(roleEditingId.value, { name: roleForm.name, description: roleForm.description, permissions: roleForm.permissions })
      ElMessage.success('角色已更新')
    } else {
      await createRole({ code: roleForm.code, name: roleForm.name, description: roleForm.description, permissions: roleForm.permissions })
      ElMessage.success('角色已创建')
    }
    roleVisible.value = false
    loadRoles()
  } catch { /* 后端已提示 */ } finally { savingRole.value = false }
}
async function handleRoleDelete(row: RoleItem) {
  try {
    await ElMessageBox.confirm(`确认删除角色「${row.name}」？`, '提示', { type: 'warning' })
    await deleteRole(row.id)
    ElMessage.success('角色已删除')
    loadRoles()
  } catch { /* 取消或失败 */ }
}

// 新建模式下切换角色时，预填该角色默认权限
watch(() => editForm.role, (newRole) => {
  if (!editingId.value && editVisible.value && !permCustom.value) {
    permSelected.value = [...(rolePermMap.value[newRole] || [])]
  }
})

// ---- 组织/部门管理 ----
const deptLoading = ref(false)
const departments = ref<DepartmentItem[]>([])
const deptVisible = ref(false)
const deptEditingId = ref<number | null>(null)
const savingDept = ref(false)
const deptForm = reactive({ name: '', parent_id: undefined as number | undefined, leader: '', description: '' })

async function loadDepartments() {
  deptLoading.value = true
  try {
    const res = await getDepartments()
    departments.value = res.data?.list || []
  } catch { /* 忽略 */ } finally { deptLoading.value = false }
}
function parentName(pid: number | null) {
  if (!pid) return '-'
  const d = departments.value.find((x) => x.id === pid)
  return d ? d.name : '-'
}
function openDeptCreate() {
  deptEditingId.value = null
  Object.assign(deptForm, { name: '', parent_id: undefined, leader: '', description: '' })
  deptVisible.value = true
}
function openDeptEdit(row: DepartmentItem) {
  deptEditingId.value = row.id
  Object.assign(deptForm, { name: row.name, parent_id: row.parent_id || undefined, leader: row.leader || '', description: row.description || '' })
  deptVisible.value = true
}
async function saveDept() {
  if (!deptForm.name) { ElMessage.warning('部门名称必填'); return }
  savingDept.value = true
  try {
    if (deptEditingId.value) {
      await updateDepartment(deptEditingId.value, { name: deptForm.name, parent_id: deptForm.parent_id, leader: deptForm.leader, description: deptForm.description })
      ElMessage.success('部门已更新')
    } else {
      await createDepartment({ name: deptForm.name, parent_id: deptForm.parent_id, leader: deptForm.leader, description: deptForm.description })
      ElMessage.success('部门已创建')
    }
    deptVisible.value = false
    loadDepartments()
  } catch { /* 后端已提示 */ } finally { savingDept.value = false }
}
async function handleDeptDelete(row: DepartmentItem) {
  try {
    await ElMessageBox.confirm(`确认删除部门「${row.name}」？若有成员将无法删除。`, '提示', { type: 'warning' })
    await deleteDepartment(row.id)
    ElMessage.success('部门已删除')
    loadDepartments()
  } catch { /* 取消或失败 */ }
}

onMounted(async () => {
  if (authStore.token && !authStore.user) {
    try { await authStore.fetchUserInfo() } catch { /* 忽略 */ }
  }
  await authStore.loadPermissions()
  loadCenter()
  const hasUser = authStore.hasPerm('user:manage')
  const hasRole = authStore.hasPerm('role:manage')
  if (hasUser || hasRole) {
    loadPermissionsDict()
    loadRoles()
  }
  if (authStore.hasPerm('user:manage')) { loadUsers() }
  if (authStore.hasPerm('dept:manage')) { loadDepartments() }
})
</script>

<style scoped>
.settings-page { max-width: 960px; }
.info-card { margin-bottom: 20px; }
.form-card { border-radius: 4px; }
.user-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.user-search { display: flex; gap: 8px; flex-wrap: wrap; }
.dept-tip { color: #909399; font-size: 13px; }
.role-hint { font-size: 13px; color: #606266; padding: 4px 88px 0 90px; line-height: 1.9; }
.perm-box {
  width: 100%;
  max-height: 240px;
  overflow-y: auto;
  border: 1px solid #ebeef5;
  border-radius: 4px;
  padding: 8px 12px;
  background: #fafafa;
}
.perm-group { margin-bottom: 8px; }
.perm-module { font-weight: 600; font-size: 13px; color: #303133; margin-bottom: 4px; }
.perm-inherit { color: #909399; font-size: 13px; }
.perm-box .el-checkbox { margin-right: 18px; margin-bottom: 4px; }
</style>
