<template>
  <div class="firmware-page">
    <!-- 顶部提示 + 操作 -->
    <el-card shadow="never" class="head-card">
      <div class="head-bar">
        <div class="head-title">
          <el-icon><Monitor /></el-icon>
          <span>固件管理（OTA 升级）</span>
          <el-tag size="small" type="info">已发布 {{ publishedCount }} / 共 {{ totalCount }}</el-tag>
        </div>
        <div>
          <el-button v-if="isOperator" type="primary" :icon="Upload" @click="openUpload">上传固件包</el-button>
        </div>
      </div>
    </el-card>

    <!-- Tab：固件包 / 升级记录 -->
    <el-card shadow="never" class="main-card">
      <el-tabs v-model="activeTab">
        <!-- ============ 固件包列表 ============ -->
        <el-tab-pane label="固件包" name="firmware">
          <!-- 筛选 -->
          <div class="filter-bar">
            <el-radio-group v-model="firmwareFilter" @change="loadFirmwares(1)" size="small">
              <el-radio-button label="">全部</el-radio-button>
              <el-radio-button label="true">已发布</el-radio-button>
              <el-radio-button label="false">未发布</el-radio-button>
            </el-radio-group>
            <div class="flex1"></div>
            <el-button :icon="Refresh" size="small" @click="loadFirmwares()">刷新</el-button>
          </div>

          <el-table :data="firmwareList" v-loading="fwLoading" stripe style="width: 100%">
            <el-table-column prop="version" label="版本号" width="120">
              <template #default="{ row }">
                <span class="fw-version">{{ row.version }}</span>
                <el-tag v-if="row.published" size="small" type="success" style="margin-left: 6px">已发布</el-tag>
                <el-tag v-else size="small" type="info" style="margin-left: 6px">草稿</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="file_name" label="固件文件" min-width="180" show-overflow-tooltip />
            <el-table-column label="大小" width="100">
              <template #default="{ row }">{{ formatSize(row.size) }}</template>
            </el-table-column>
            <el-table-column prop="md5" label="MD5" width="150" show-overflow-tooltip>
              <template #default="{ row }">
                <code class="md5">{{ row.md5 || '-' }}</code>
              </template>
            </el-table-column>
            <el-table-column prop="description" label="升级说明" min-width="200" show-overflow-tooltip />
            <el-table-column prop="uploader" label="上传人" width="100" />
            <el-table-column label="发布时间" width="170">
              <template #default="{ row }">{{ row.published_at ? fmtTime(row.published_at) : '—' }}</template>
            </el-table-column>
            <el-table-column label="操作" width="210" fixed="right">
              <template #default="{ row }">
                <el-button v-if="isOperator" :type="row.published ? 'warning' : 'success'" plain size="small"
                           @click="togglePublish(row)">
                  {{ row.published ? '下线' : '发布' }}
                </el-button>
                <el-button v-if="isOperator && row.published" type="primary" plain size="small"
                           @click="openUpgrade(row)">升级</el-button>
                <el-button v-if="isAdmin" type="danger" plain size="small" @click="handleDeleteFw(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>

          <div class="pager">
            <el-pagination background layout="total, prev, pager, next" :total="fwTotal"
                           :page-size="fwPageSize" :current-page="fwPage" @current-change="loadFirmwares" />
          </div>
        </el-tab-pane>

        <!-- ============ 升级记录 ============ -->
        <el-tab-pane label="升级记录" name="upgrade">
          <div class="filter-bar">
            <el-input v-model="upgFilter.device_hw_id" placeholder="设备硬件ID" clearable size="small"
                      style="width: 160px" @keyup.enter="loadUpgrades(1)" />
            <el-select v-model="upgFilter.status" placeholder="升级状态" clearable size="small" style="width: 130px"
                       @change="loadUpgrades(1)">
              <el-option label="等待升级" value="pending" />
              <el-option label="升级中" value="upgrading" />
              <el-option label="升级成功" value="success" />
              <el-option label="升级失败" value="failed" />
            </el-select>
            <el-button type="primary" size="small" :icon="Search" @click="loadUpgrades(1)">查询</el-button>
            <el-button :icon="Refresh" size="small" @click="resetUpgFilter">重置</el-button>
            <div class="flex1"></div>
            <el-link type="info" :underline="false">提示：发起升级后，设备下次签到时若检测到新版本将自动升级</el-link>
          </div>

          <el-table :data="upgradeList" v-loading="upgLoading" stripe style="width: 100%">
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="device_hw_id" label="设备ID" width="110" />
            <el-table-column prop="target_version" label="目标版本" width="120" />
            <el-table-column label="状态" width="120">
              <template #default="{ row }">
                <el-tag :type="statusType(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="error_msg" label="失败原因" min-width="160" show-overflow-tooltip />
            <el-table-column label="开始时间" width="170">
              <template #default="{ row }">{{ row.started_at ? fmtTime(row.started_at) : '—' }}</template>
            </el-table-column>
            <el-table-column label="完成时间" width="170">
              <template #default="{ row }">{{ row.finished_at ? fmtTime(row.finished_at) : '—' }}</template>
            </el-table-column>
            <el-table-column label="操作" width="90" fixed="right">
              <template #default="{ row }">
                <el-button v-if="isAdmin" type="danger" plain size="small" @click="handleDeleteUpg(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>

          <div class="pager">
            <el-pagination background layout="total, prev, pager, next" :total="upgTotal"
                           :page-size="upgPageSize" :current-page="upgPage" @current-change="loadUpgrades" />
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <!-- 上传固件对话框 -->
    <el-dialog v-model="uploadVisible" title="上传固件包" width="520px" :close-on-click-modal="false">
      <el-form ref="uploadFormRef" :model="uploadForm" :rules="uploadRules" label-width="100px">
        <el-form-item label="版本号" prop="version">
          <el-input v-model="uploadForm.version" placeholder="如 v1.2.3 / 1.2.3" clearable />
        </el-form-item>
        <el-form-item label="升级说明" prop="description">
          <el-input v-model="uploadForm.description" type="textarea" :rows="3"
                    placeholder="本次升级的变更说明（可选）" />
        </el-form-item>
        <el-form-item label="固件文件" prop="file">
          <el-upload drag :auto-upload="false" :limit="1" :on-change="onFileChange"
                     :on-remove="() => (uploadForm.file = null)" accept=".bin,.hex,.fw,.elf,.img,.dat">
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">拖拽文件到此处或 <em>点击选择</em></div>
            <template #tip>
              <div class="el-upload__tip">支持 bin/hex/fw/elf/img/dat，最大 50MB</div>
            </template>
          </el-upload>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="uploadVisible = false">取消</el-button>
        <el-button type="primary" :loading="uploading" @click="submitUpload">上传</el-button>
      </template>
    </el-dialog>

    <!-- 发起升级对话框 -->
    <el-dialog v-model="upgradeVisible" title="发起设备固件升级" width="480px" :close-on-click-modal="false">
      <el-form :model="upgradeForm" label-width="100px">
        <el-form-item label="目标版本">
          <el-tag>{{ upgradeForm.targetVersion }}</el-tag>
        </el-form-item>
        <el-form-item label="升级说明">
          <el-tooltip :content="upgradeForm.description" placement="top">
            <el-text type="info" truncated>{{ upgradeForm.description || '无' }}</el-text>
          </el-tooltip>
        </el-form-item>
        <el-form-item label="设备">
          <el-select v-model="upgradeForm.device_hw_id" placeholder="搜索设备ID或路口" filterable clearable
                     :loading="devLoading" style="width: 100%">
            <el-option-group v-for="g in deviceGroups" :key="g.label" :label="g.label">
              <el-option v-for="d in g.options" :key="d.hw_id"
                         :label="d.intersection ? d.intersection + ' (#'+d.hw_id+')' : '#'+d.hw_id"
                         :value="d.hw_id" />
            </el-option-group>
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="upgradeVisible = false">取消</el-button>
        <el-button type="primary" :loading="upgrading" @click="submitUpgrade">发起升级</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Upload, UploadFilled, Monitor, Refresh, Search } from '@element-plus/icons-vue'
import { getFirmwares, uploadFirmware, publishFirmware, deleteFirmware,
  getFirmwareUpgrades, createFirmwareUpgrade, deleteFirmwareUpgrade } from '@/api/firmware'
import { getDevices } from '@/api/device'
import { useAuthStore } from '@/store/auth'
import type { FormInstance, FormRules } from 'element-plus'

const authStore = useAuthStore()
const isOperator = computed(() => { const r = authStore.user?.role; return r === 'admin' || r === 'operator' })
const isAdmin = computed(() => authStore.user?.role === 'admin')

const activeTab = ref('firmware')

// ===== 固件包列表 =====
const firmwareList = ref<any[]>([])
const firmwareFilter = ref('')
const fwLoading = ref(false)
const fwPage = ref(1)
const fwPageSize = ref(10)
const fwTotal = ref(0)
const publishedCount = ref(0)
const totalCount = ref(0)

async function loadFirmwares(page = fwPage.value) {
  fwLoading.value = true
  fwPage.value = page
  try {
    const params: Record<string, any> = { page: page, page_size: fwPageSize.value }
    if (firmwareFilter.value !== '') params.published = firmwareFilter.value === 'true'
    const res = await getFirmwares(params)
    firmwareList.value = res.data?.list || []
    fwTotal.value = res.data?.total || 0
  } catch { /* 忽略 */ } finally { fwLoading.value = false }
  // 统计（全部）
  try {
    const all = await getFirmwares({ page: 1, page_size: 100 })
    const list: any[] = all.data?.list || []
    totalCount.value = list.length
    publishedCount.value = list.filter((f) => f.published).length
  } catch { /* 忽略 */ }
}

// ===== 上传固件 =====
const uploadVisible = ref(false)
const uploading = ref(false)
const uploadFormRef = ref<FormInstance>()
const uploadForm = reactive<{ version: string; description: string; file: File | null }>({
  version: '', description: '', file: null,
})
const uploadRules: FormRules = {
  version: [{ required: true, message: '请填写版本号', trigger: 'blur' }],
  file: [{ required: true, message: '请选择固件文件', trigger: 'change' }],
}

function openUpload() {
  uploadForm.version = ''
  uploadForm.description = ''
  uploadForm.file = null
  uploadVisible.value = true
}
function onFileChange(file: any) {
  uploadForm.file = file.raw || null
}
async function submitUpload() {
  await uploadFormRef.value?.validate().catch(() => { throw new Error('invalid') })
  if (!uploadForm.file) { ElMessage.warning('请选择固件文件'); return }
  uploading.value = true
  try {
    const fd = new FormData()
    fd.append('version', uploadForm.version)
    if (uploadForm.description) fd.append('description', uploadForm.description)
    fd.append('file', uploadForm.file)
    const res = await uploadFirmware(fd)
    if (res.code === 0) {
      ElMessage.success('固件包上传成功')
      uploadVisible.value = false
      loadFirmwares(1)
    } else {
      ElMessage.error(res.msg || '上传失败')
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '上传失败')
  } finally { uploading.value = false }
}

// ===== 发布/下线 =====
async function togglePublish(row: any) {
  const action = row.published ? '下线' : '发布'
  try {
    await ElMessageBox.confirm(`确定${action}固件 ${row.version} 吗？`, '提示', { type: 'warning' })
  } catch { return }
  const res = await publishFirmware(row.id, !row.published)
  if (res.code === 0) {
    ElMessage.success(`${action}成功`)
    loadFirmwares()
  } else ElMessage.error(res.msg || '操作失败')
}

// ===== 删除固件 =====
async function handleDeleteFw(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除固件 ${row.version} 吗？删除后不可恢复。`, '警告', { type: 'warning' })
  } catch { return }
  const res = await deleteFirmware(row.id)
  if (res.code === 0) { ElMessage.success('删除成功'); loadFirmwares() }
  else ElMessage.error(res.msg || '删除失败')
}

// ===== 升级记录 =====
const upgradeList = ref<any[]>([])
const upgLoading = ref(false)
const upgPage = ref(1)
const upgPageSize = ref(10)
const upgTotal = ref(0)
const upgFilter = reactive<{ device_hw_id: string; status: string }>({ device_hw_id: '', status: '' })

async function loadUpgrades(page = upgPage.value) {
  upgLoading.value = true
  upgPage.value = page
  try {
    const params: Record<string, any> = { page, page_size: upgPageSize.value }
    if (upgFilter.device_hw_id) params.device_hw_id = upgFilter.device_hw_id
    if (upgFilter.status) params.status = upgFilter.status
    const res = await getFirmwareUpgrades(params)
    upgradeList.value = res.data?.list || []
    upgTotal.value = res.data?.total || 0
  } catch { /* 忽略 */ } finally { upgLoading.value = false }
}
function resetUpgFilter() {
  upgFilter.device_hw_id = ''
  upgFilter.status = ''
  loadUpgrades(1)
}
function statusLabel(s: string) {
  return ({ pending: '等待升级', upgrading: '升级中', success: '升级成功', failed: '升级失败' } as Record<string, string>)[s] || s
}
function statusType(s: string) {
  return ({ pending: 'info', upgrading: 'warning', success: 'success', failed: 'danger' } as Record<string, any>)[s] || 'info'
}

// ===== 发起升级 =====
const upgradeVisible = ref(false)
const upgrading = ref(false)
const upgradeForm = reactive<{ device_hw_id: number | null; firmware_id: number; targetVersion: string; description: string }>({
  device_hw_id: null, firmware_id: 0, targetVersion: '', description: '',
})
const devLoading = ref(false)
const deviceGroups = ref<{ label: string; options: any[] }[]>([])

function openUpgrade(row: any) {
  upgradeForm.firmware_id = row.id
  upgradeForm.targetVersion = row.version
  upgradeForm.description = row.description || ''
  upgradeForm.device_hw_id = null
  upgradeVisible.value = true
  loadAllDevices()
}
async function loadAllDevices() {
  devLoading.value = true
  try {
    const res = await getDevices({ page_size: 200 })
    const list: any[] = res.data?.list || []
    const online = list.filter((d) => d.online_status)
    const offline = list.filter((d) => !d.online_status)
    const groups: { label: string; options: any[] }[] = []
    if (online.length) groups.push({ label: `在线（${online.length}）`, options: online })
    if (offline.length) groups.push({ label: `离线（${offline.length}）`, options: offline })
    deviceGroups.value = groups
  } catch { deviceGroups.value = [] } finally { devLoading.value = false }
}
async function submitUpgrade() {
  if (!upgradeForm.device_hw_id) { ElMessage.warning('请选择设备'); return }
  upgrading.value = true
  try {
    const res = await createFirmwareUpgrade({ device_hw_id: upgradeForm.device_hw_id, firmware_id: upgradeForm.firmware_id })
    if (res.code === 0) {
      ElMessage.success('升级任务已创建')
      upgradeVisible.value = false
      activeTab.value = 'upgrade'
      loadUpgrades(1)
    } else ElMessage.error(res.msg || '发起失败')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '发起失败')
  } finally { upgrading.value = false }
}

// ===== 删除升级记录 =====
async function handleDeleteUpg(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除设备 #${row.device_hw_id} 的升级记录吗？`, '提示', { type: 'warning' })
  } catch { return }
  const res = await deleteFirmwareUpgrade(row.id)
  if (res.code === 0) { ElMessage.success('删除成功'); loadUpgrades() }
  else ElMessage.error(res.msg || '删除失败')
}

// ===== 工具 =====
function formatSize(bytes: number) {
  if (!bytes) return '0 B'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1024 / 1024).toFixed(2) + ' MB'
}
function fmtTime(s: string) {
  return (s || '').replace('T', ' ').slice(0, 19)
}

onMounted(() => {
  loadFirmwares(1)
  loadUpgrades(1)
})
</script>

<style scoped>
.firmware-page { padding: 4px; }
.head-card { margin-bottom: 12px; }
.head-bar { display: flex; align-items: center; justify-content: space-between; }
.head-title { display: flex; align-items: center; gap: 8px; font-size: 16px; font-weight: 600; }
.main-card :deep(.el-card__body) { padding-top: 12px; }
.filter-bar { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.flex1 { flex: 1; }
.fw-version { font-weight: 600; }
.md5 { font-size: 11px; color: #909399; }
.pager { display: flex; justify-content: flex-end; margin-top: 14px; }
</style>
