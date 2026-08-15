<template>
  <div class="intersection-page">
    <el-card shadow="never">
      <template #header>
        <div class="header-bar">
          <span>路口管理（{{ total }} 个路口）</span>
          <div class="header-actions">
            <el-input v-model="keyword" placeholder="搜索路口名称" clearable style="width: 200px" @input="applyFilter" />
            <el-button type="primary" @click="goMap">地图大屏</el-button>
            <span v-if="!canEdit" class="readonly-tip">只读（查看人员）</span>
          </div>
        </div>
      </template>

      <el-table :data="filteredList" v-loading="loading" border stripe style="width: 100%">
        <el-table-column prop="intersection" label="路口名称" min-width="160" />
        <el-table-column prop="device_total" label="设备总数" width="100" align="center" />
        <el-table-column label="在线" width="90" align="center">
          <template #default="{ row }"><el-tag type="success" size="small">{{ row.online }}</el-tag></template>
        </el-table-column>
        <el-table-column label="离线" width="90" align="center">
          <template #default="{ row }"><el-tag type="info" size="small">{{ row.offline }}</el-tag></template>
        </el-table-column>
        <el-table-column label="活跃故障" width="100" align="center">
          <template #default="{ row }"><el-tag :type="row.fault > 0 ? 'danger' : 'success'" size="small">{{ row.fault }}</el-tag></template>
        </el-table-column>
        <el-table-column label="经纬度" min-width="140" align="center">
          <template #default="{ row }">
            <span v-if="row.lat !== null && row.lat !== undefined">{{ row.lat?.toFixed(5) }}, {{ row.lng?.toFixed(5) }}</span>
            <span v-else class="no-coord">未设置</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="320" align="center" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="goDevices(row.intersection)">设备</el-button>
            <el-button size="small" type="primary" @click="openSetLocation(row)">设坐标</el-button>
            <el-button size="small" type="warning" @click="openRename(row)">重命名</el-button>
            <el-button size="small" type="danger" :disabled="!isAdmin" @click="handleClear(row)">清空</el-button>
            <el-button size="small" type="success" @click="goMapFocus(row)" style="margin-left:4px">去地图</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && filteredList.length === 0" description="暂无路口数据" />
      <div v-if="canEdit" class="foot-tip">提示：路口由设备的路口字段聚合而成，改动会批量应用到该路口下所有设备。</div>
    </el-card>

    <!-- 设置路口坐标弹窗 -->
    <el-dialog v-model="locVisible" title="设置路口经纬度" width="440px">
      <el-form :model="locForm" label-width="90px">
        <el-form-item label="路口名称"><el-input :value="target?.intersection" disabled /></el-form-item>
        <el-form-item label="纬度" required>
          <el-input v-model="locForm.lat" placeholder="如：31.2304" />
        </el-form-item>
        <el-form-item label="经度" required>
          <el-input v-model="locForm.lng" placeholder="如：121.4737" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="locVisible = false">取消</el-button>
        <el-button type="primary" :loading="busy" @click="saveLocation">保存</el-button>
      </template>
    </el-dialog>

    <!-- 重命名路口弹窗 -->
    <el-dialog v-model="renameVisible" title="重命名路口" width="440px">
      <el-form :model="renameForm" label-width="90px">
        <el-form-item label="旧名称"><el-input :value="target?.intersection" disabled /></el-form-item>
        <el-form-item label="新名称" required>
          <el-input v-model="renameForm.newName" placeholder="新路口名称" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="renameVisible = false">取消</el-button>
        <el-button type="primary" :loading="busy" @click="saveRename">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getIntersections, renameIntersection, setIntersectionLocation, clearIntersection, type IntersectionItem } from '@/api/intersection'
import { useAuthStore } from '@/store/auth'
import { emitMapFocus } from '@/utils/eventBus'

const router = useRouter()
const authStore = useAuthStore()
const canEdit = computed(() => { const r = authStore.user?.role; return r === 'admin' || r === 'operator' })
const isAdmin = computed(() => authStore.user?.role === 'admin')

const loading = ref(false)
const list = ref<IntersectionItem[]>([])
const keyword = ref('')

const total = computed(() => list.value.length)
const filteredList = computed(() => {
  const kw = keyword.value.trim()
  if (!kw) return list.value
  return list.value.filter((i) => i.intersection.includes(kw))
})

function applyFilter() { /* 响应式自动过滤 */ }

async function load() {
  loading.value = true
  try { const res = await getIntersections(); list.value = res.data?.list || [] }
  catch { ElMessage.error('路口数据加载失败') }
  finally { loading.value = false }
}

function goDevices(intersection: string) { router.push({ path: '/device', query: { intersection } }) }
function goMap() { router.push('/map') }

// 「去地图」：通过事件总线通知地图大屏，跳转并居中该路口
function goMapFocus(row: IntersectionItem) {
  if (row.lat === null || row.lat === undefined || row.lng === null || row.lng === undefined) {
    ElMessage.warning('该路口尚未设置坐标，请先「设坐标」')
    return
  }
  // 跳转地图页
  router.push('/map')
  // 缓存聚焦供地图页挂载后读取（mitt 事件在地图组件未挂载时会被错过）
  emitMapFocus({ kind: 'intersection', name: row.intersection, lat: row.lat, lng: row.lng, height: 1500 })
}

// ---- 操作 ----
const busy = ref(false)
const target = ref<IntersectionItem | null>(null)

// 设置坐标
const locVisible = ref(false)
const locForm = reactive({ lat: '', lng: '' })
function openSetLocation(row: IntersectionItem) { target.value = row; locForm.lat = ''; locForm.lng = ''; locVisible.value = true }
async function saveLocation() {
  if (!target.value) return
  const lat = parseFloat(locForm.lat); const lng = parseFloat(locForm.lng)
  if (isNaN(lat) || lat < -90 || lat > 90) { ElMessage.warning('纬度范围 -90~90'); return }
  if (isNaN(lng) || lng < -180 || lng > 180) { ElMessage.warning('经度范围 -180~180'); return }
  busy.value = true
  try {
    const res = await setIntersectionLocation(target.value.intersection, lat, lng)
    ElMessage.success(res.data?.message || '已设置')
    locVisible.value = false
    load()
  } catch { /* 后端提示 */ } finally { busy.value = false }
}

// 重命名
const renameVisible = ref(false)
const renameForm = reactive({ newName: '' })
function openRename(row: IntersectionItem) { target.value = row; renameForm.newName = ''; renameVisible.value = true }
async function saveRename() {
  if (!target.value || !renameForm.newName.trim()) { ElMessage.warning('请输入新名称'); return }
  busy.value = true
  try {
    const res = await renameIntersection(target.value.intersection, renameForm.newName.trim())
    ElMessage.success(res.data?.message || '已重命名')
    renameVisible.value = false
    load()
  } catch { /* 后端提示 */ } finally { busy.value = false }
}

// 清空路口
async function handleClear(row: IntersectionItem) {
  try {
    await ElMessageBox.confirm(`确认清空路口「${row.intersection}」？该路口下 ${row.device_total} 台设备将回到未分配。`, '提示', { type: 'warning' })
    await clearIntersection(row.intersection)
    ElMessage.success('路口已清空')
    load()
  } catch { /* 取消或失败 */ }
}

onMounted(async () => {
  if (authStore.token && !authStore.user) { try { await authStore.fetchUserInfo() } catch { /* 忽略 */ } }
  load()
})
</script>

<style scoped>
.header-bar { display: flex; justify-content: space-between; align-items: center; }
.header-actions { display: flex; gap: 8px; align-items: center; }
.readonly-tip { color: #c0c4cc; font-size: 12px; }
.foot-tip { margin-top: 10px; color: #909399; font-size: 12px; }
.no-coord { color: #c0c4cc; }
</style>
