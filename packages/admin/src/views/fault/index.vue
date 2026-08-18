<template>
  <div class="fault-page">
    <!-- 顶部统计 -->
    <el-row :gutter="12" class="stat-row">
      <el-col :span="6" v-for="s in statCards" :key="s.value">
        <el-card shadow="never" class="stat-card" :class="{ on: s.value === 'occurred' }">
          <div class="stat-num" :style="{ color: s.color }">{{ statMap[s.value] || 0 }}</div>
          <div class="stat-label">{{ s.label }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 识别统计面板（范围B：智能识别准确率/误报/漏报） -->
    <el-card shadow="never" class="recog-card">
      <div class="recog-head">
        <el-icon><DataAnalysis /></el-icon>
        <span class="recog-title">识别统计（案例库回标真值）</span>
        <el-button size="small" text type="primary" :loading="statsLoading" @click="loadRecognitionStats">刷新</el-button>
      </div>
      <div class="recog-body" v-loading="statsLoading" element-loading-background="rgba(255,255,255,.5)">
        <el-row :gutter="12">
          <el-col :span="5">
            <div class="recog-item"><div class="recog-num">{{ recogStats?.total_cases ?? '-' }}</div><div class="recog-lab">总案例数</div></div>
          </el-col>
          <el-col :span="5">
            <div class="recog-item"><div class="recog-num" :style="{color: accColor}">{{ recogStats != null ? Math.round((recogStats.accuracy??0)*100)+'%' : '-' }}</div><div class="recog-lab">识别准确率</div></div>
          </el-col>
          <el-col :span="5">
            <div class="recog-item"><div class="recog-num" style="color:#F56C6C">{{ recogStats?.false_positive ?? '-' }}</div><div class="recog-lab">误报</div></div>
          </el-col>
          <el-col :span="5">
            <div class="recog-item"><div class="recog-num" style="color:#E6A23C">{{ recogStats?.false_negative ?? '-' }}</div><div class="recog-lab">漏报</div></div>
          </el-col>
          <el-col :span="4">
            <div class="recog-item">
              <div class="recog-num" style="color:#67C23A">{{ recogStats?.confirmed_or_seed ?? '-' }}</div>
              <div class="recog-lab">已确认案例</div>
            </div>
          </el-col>
        </el-row>
        <div class="recog-rate">
          误报率 {{ recogStats != null ? Math.round((recogStats.false_positive_rate??0)*100)+'%' : '-' }} ·
          漏报率 {{ recogStats != null ? Math.round((recogStats.false_negative_rate??0)*100)+'%' : '-' }}
        </div>
      </div>
    </el-card>

    <!-- 搜索栏 -->
    <el-card shadow="never" class="search-card">
      <el-form :inline="true" :model="searchForm" @submit.prevent="handleSearch">
        <el-form-item label="设备">
          <el-select
            v-model="searchForm.device_hw_id"
            placeholder="搜索设备ID或路口"
            clearable
            filterable
            remote
            :remote-method="searchDevices"
            :loading="devLoading"
            style="width: 220px"
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
        <el-form-item label="故障状态">
          <el-select v-model="searchForm.status" placeholder="全部" clearable style="width: 130px">
            <el-option v-for="s in FAULT_STATUSES" :key="s.value" :label="s.label" :value="s.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="研判状态">
          <el-select v-model="searchForm.recognition_status" placeholder="全部" clearable style="width: 130px">
            <el-option label="待复核" value="pending_review" />
          </el-select>
        </el-form-item>
        <el-form-item label="故障类型">
          <el-select v-model="searchForm.fault_type" placeholder="全部" clearable style="width: 150px">
            <el-option label="灯灭" value="lamp_off" />
            <el-option label="异常同亮" value="abnormal_on" />
            <el-option label="亮灯超时" value="timeout" />
            <el-option label="缺亮" value="dim" />
            <el-option label="断电" value="power_loss" />
          </el-select>
        </el-form-item>
        <el-form-item label="故障级别">
          <el-select v-model="searchForm.fault_level" placeholder="全部" clearable style="width: 120px">
            <el-option label="严重" value="critical" />
            <el-option label="一般" value="normal" />
          </el-select>
        </el-form-item>
        <el-form-item label="日期范围">
          <el-date-picker
            v-model="dateRange"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            value-format="YYYY-MM-DD"
            style="width: 260px"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">搜索</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
          <el-button :icon="Download" @click="exportCsv">导出 CSV</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 故障列表表格 -->
    <el-card shadow="never" class="table-card">
      <el-table :data="tableData" v-loading="loading" border stripe style="width: 100%">
        <el-table-column prop="id" label="ID" width="60" align="center" />
        <el-table-column prop="device_hw_id" label="设备ID" width="150" align="center" />
        <el-table-column label="故障类型" width="130" align="center">
          <template #default="{ row }">{{ errCodeLabel(row.err_code) }}</template>
        </el-table-column>
        <el-table-column label="级别" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.fault_level === 'critical' ? 'danger' : 'warning'" size="small">
              {{ row.fault_level === 'critical' ? '严重' : '一般' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="故障状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="faultStatusTag(row.status)" size="small">{{ faultStatusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="置信度" width="90" align="center">
          <template #default="{ row }">
            <span :class="confClass(row.confidence)">
              {{ fmtConfidence(row.confidence) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="研判状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="recognitionStatusTag(row.recognition_status)" size="small">{{ recognitionStatusLabel(row.recognition_status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="负责人" width="90" align="center">
          <template #default="{ row }">{{ row.owner_name || '-' }}</template>
        </el-table-column>
        <el-table-column label="维修人" width="90" align="center">
          <template #default="{ row }">{{ row.repairer_name || '-' }}</template>
        </el-table-column>
        <el-table-column prop="last_seen" label="最后出现" width="170" align="center" />
        <el-table-column label="操作" width="190" align="center" fixed="right">
          <template #default="{ row }">
            <el-button type="info" plain size="small" @click="openDetail(row)">详情</el-button>
            <el-button v-if="canEdit && row.recognition_status === 'pending_review'" type="warning" size="small" @click="openReview(row)">复核</el-button>
            <el-button v-if="canEdit" type="primary" size="small" @click="openManage(row)"
                       :disabled="row.status === 'resolved'">处理</el-button>
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

    <!-- 故障详情抽屉 -->
    <el-drawer v-model="detailVisible" title="故障详情" size="500px" v-loading="detailLoading">
      <div v-if="detail" class="detail-body">
        <el-descriptions :column="1" border class="detail-desc">
          <el-descriptions-item label="设备ID">{{ detail.fault?.device_hw_id ?? '-' }}</el-descriptions-item>
          <el-descriptions-item label="故障类型">{{ errCodeLabel(detail.fault?.err_code) }}</el-descriptions-item>
          <el-descriptions-item label="故障级别">
            <el-tag :type="detail.fault?.fault_level === 'critical' ? 'danger' : 'warning'" size="small">
              {{ detail.fault?.fault_level === 'critical' ? '严重' : '一般' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="故障状态">
            <el-tag :type="faultStatusTag(detail.fault?.status)" size="small">{{ faultStatusLabel(detail.fault?.status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="负责人">{{ detail.fault?.owner_name || '-' }}</el-descriptions-item>
          <el-descriptions-item label="维修人">{{ detail.fault?.repairer_name || '-' }}</el-descriptions-item>
          <el-descriptions-item label="确认时间">{{ detail.fault?.confirmed_at ? fmtT(detail.fault.confirmed_at) : '-' }}</el-descriptions-item>
          <el-descriptions-item label="派单时间">{{ detail.fault?.dispatched_at ? fmtT(detail.fault.dispatched_at) : '-' }}</el-descriptions-item>
          <el-descriptions-item label="解决时间">{{ detail.fault?.resolved_at ? fmtT(detail.fault.resolved_at) : '-' }}</el-descriptions-item>
          <el-descriptions-item label="首次出现">{{ fmtT(detail.fault?.first_seen) }}</el-descriptions-item>
          <el-descriptions-item label="最后出现">{{ fmtT(detail.fault?.last_seen) }}</el-descriptions-item>
          <el-descriptions-item label="电流值(R/Y/G)">
            {{ detail.fault?.current_r ?? '-' }} / {{ detail.fault?.current_y ?? '-' }} / {{ detail.fault?.current_g ?? '-' }}
          </el-descriptions-item>
        </el-descriptions>

        <h4 class="section-title">设备信息</h4>
        <div v-if="detail.device?.id" class="fault-card">
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="路口">📍 {{ detail.device.intersection || '-' }}</el-descriptions-item>
            <el-descriptions-item label="在线状态">
              <el-tag :type="detail.device.online_status ? 'success' : 'info'" size="small">
                {{ detail.device.online_status ? '在线' : '离线' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="经纬度">{{ fmtLngLat(detail.device) }}</el-descriptions-item>
          </el-descriptions>
        </div>
        <el-empty v-else description="暂无设备信息" :image-size="60" />

        <h4 class="section-title">关联工单</h4>
        <div v-if="detail.work_order?.id" class="fault-card">
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="工单编号">{{ detail.work_order.order_no }}</el-descriptions-item>
            <el-descriptions-item label="状态">
              <el-tag :type="woTagType(detail.work_order.status)" size="small">{{ woLabel(detail.work_order.status) }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="处理人">{{ detail.work_order.assignee_name || '未指派' }}</el-descriptions-item>
            <el-descriptions-item label="超时">
              <el-tag v-if="detail.work_order.overdue" type="danger" effect="dark" size="small">超时</el-tag>
              <span v-else class="sla-ok">未超时</span>
            </el-descriptions-item>
          </el-descriptions>
        </div>
        <el-empty v-else description="暂无关联工单" :image-size="60" />

        <h4 class="section-title">多源证据（识别研判）</h4>
        <div v-loading="evidenceLoading" class="evidence-area">
          <template v-if="evidenceList.length">
            <div v-for="ev in evidenceList" :key="ev.id" class="evidence-item">
              <div class="evidence-top">
                <el-tag :type="evTag(ev.source_type)" size="small">{{ evidenceSourceLabel(ev.source_type) }}</el-tag>
                <span class="evidence-time">{{ fmtT(ev.captured_at) }}</span>
                <span class="evidence-conf">置信 {{ fmtConfidence(ev.confidence) }}</span>
              </div>
              <div class="evidence-body">
                <template v-if="ev.source_type==='firmware'">
                  错误码 {{ errCodeLabel(ev.err_code ?? 0) }}<span v-if="ev.led_state != null"> · 灯态 {{ ev.led_state }}</span>
                  <span v-if="ev.current_r != null"> · 电流 {{ ev.current_r }}/{{ ev.current_y }}/{{ ev.current_g }}</span>
                </template>
                <template v-else-if="ev.source_type==='current'">电流 {{ ev.current_r }}/{{ ev.current_y }}/{{ ev.current_g }}</template>
                <template v-else-if="ev.source_type==='led_state'">灯态 {{ ev.led_state }}</template>
                <template v-else>{{ ev.raw_data || '（无文本内容）' }}</template>
                <div v-if="ev.evaluation_id" class="evidence-eval">研判批次 {{ ev.evaluation_id }}</div>
              </div>
            </div>
          </template>
          <el-empty v-else-if="!evidenceLoading" description="暂无多源证据" :image-size="60" />
        </div>
      </div>
    </el-drawer>

    <!-- 待确认复核弹窗 -->
    <el-dialog v-model="reviewVisible" title="待确认复核" width="480px" append-to-body v-loading="reviewSubmitting">
      <el-alert v-if="reviewCur" type="warning" :closable="false" show-icon style="margin-bottom: 12px"
                :title="`设备 #${reviewCur.device_hw_id ?? '-'} · 置信度 ${fmtConfidence(reviewCur.confidence)}`" />
      <p class="review-tip">该故障置信度待确认，请依据设备实际情况与多源证据进行复核：</p>
      <template #footer>
        <el-button @click="reviewVisible = false">取消</el-button>
        <el-button type="danger" :loading="reviewSubmitting" @click="submitReview(false)">标记误报</el-button>
        <el-button type="success" :loading="reviewSubmitting" @click="submitReview(true)">确认真故障</el-button>
      </template>
    </el-dialog>

    <!-- 故障处理弹窗 -->
    <el-dialog v-model="manageVisible" title="故障处理" width="560px" append-to-body>
      <el-alert v-if="curFault" type="warning" :closable="false" show-icon style="margin-bottom: 12px"
                :title="`当前状态：${faultStatusLabel(curFault.status)}`" />
      <!-- AI 处置建议（确认/派单辅助） -->
      <AiCopilot v-if="curFault" :load-fn="() => loadFaultAdvice(curFault!.id)" :fill-fn="applyFaultAdvice" />
      <el-form label-width="100px">
        <el-form-item label="故障状态">
          <el-select v-model="manageForm.status" style="width: 100%">
            <el-option v-for="s in FAULT_STATUSES" :key="s.value" :label="s.label" :value="s.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="负责人（确认）">
          <el-select v-model="manageForm.owner_id" filterable clearable placeholder="选择负责确认的运维" style="width: 100%">
            <el-option v-for="u in ops" :key="u.id" :label="u.username + (u.real_name ? '（'+u.real_name+'）' : '')" :value="u.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="manageForm.status === 'dispatched' || manageForm.status === 'resolved'" label="维修人">
          <el-select v-model="manageForm.repairer_id" filterable clearable placeholder="选择维修人" style="width: 100%">
            <el-option v-for="u in ops" :key="u.id" :label="u.username + (u.real_name ? '（'+u.real_name+'）' : '')" :value="u.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="manageVisible = false">取消</el-button>
        <el-button v-if="curFault && curFault.status !== 'dispatched' && manageForm.status === 'dispatched'" type="warning" @click="doDispatch">保存并派单</el-button>
        <el-button type="primary" @click="saveManage">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, Refresh, DataAnalysis, Download } from '@element-plus/icons-vue'
import { getFaults, getFault, updateFault, dispatchFault, FAULT_STATUSES, faultStatusLabel, faultStatusTag } from '@/api/fault'
import {
  getFaultEvidence, getRecognitionStats, reviewFault,
  recognitionStatusLabel, recognitionStatusTag,
  evidenceSourceLabel, fmtConfidence, type FaultEvidence,
} from '@/api/recognition'
import { getDevices } from '@/api/device'
import { getUsers } from '@/api/user'
import { getFaultAdvice, type FaultAdvice } from '@/api/copilot'
import AiCopilot from '@/components/AiCopilot.vue'
import { useAuthStore } from '@/store/auth'

const authStore = useAuthStore()
const canEdit = computed(() => { const r = authStore.user?.role; return r === 'admin' || r === 'operator' })
const faultAdvice = ref<FaultAdvice | null>(null)
async function loadFaultAdvice(id: number) {
  const res = await getFaultAdvice(id)
  faultAdvice.value = res.data?.result || null
  return res
}
// 故障确认建议：默认建议将故障置为“已确认”并推荐负责人（交由用户确认）
function applyFaultAdvice(a: FaultAdvice) {
  if (a?.priority && a.priority === 'P0' && manageForm.status === 'occurred') {
    manageForm.status = 'confirmed'
  }
  ElMessage.success('已应用 AI 建议（建议负责人请按实际情况选择）')
}

// ---------------- 顶部统计 ----------------
const statMap = ref<Record<string, number>>({})
const statCards = [
  { value: 'occurred', label: '发生中', color: '#F56C6C' },
  { value: 'confirmed', label: '已确认', color: '#E6A23C' },
  { value: 'dispatched', label: '已派单', color: '#409EFF' },
  { value: 'resolved', label: '已解决', color: '#67C23A' },
]
async function loadStats() {
  statMap.value = {}
  for (const s of FAULT_STATUSES) {
    try {
      const res = await getFaults({ page_size: 1, status: s.value })
      statMap.value[s.value] = res.data?.total || 0
    } catch { /* 忽略 */ }
  }
}

// ---------------- 设备搜索 ----------------
const devLoading = ref(false)
const deviceGroups = ref<{ label: string; options: any[] }[]>([])
async function searchDevices(keyword?: string) {
  devLoading.value = true
  try {
    const kw = (keyword || '').trim()
    const params: Record<string, any> = { page_size: 50 }
    if (kw && /^[A-Za-z0-9_-]+$/.test(kw)) params.hw_id = kw
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

// ---------------- 运维/管理员（负责人/维修人） ----------------
const ops = ref<any[]>([])
async function loadOps() {
  try {
    const res = await getUsers({ page_size: 200, status: 'active' })
    ops.value = (res.data?.list || []).filter((u: any) => u.role === 'admin' || u.role === 'operator')
  } catch { ops.value = [] }
}

// errCode → 故障中文名
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

function fmtT(t?: string): string {
  return t ? String(t).slice(0, 19).replace('T', ' ') : '-'
}

function fmtLngLat(device: Record<string, any>): string {
  const lat = device.lat, lng = device.lng
  if (lat == null || lng == null) return '-'
  return `${lng.toFixed(6)}, ${lat.toFixed(6)}`
}

function confClass(conf?: number | null): string {
  const c = conf ?? 0
  if (c >= 0.8) return 'conf-high'
  if (c >= 0.6) return 'conf-mid'
  return ''
}

function woTagType(status: string): string {
  const map: Record<string, string> = { pending: 'warning', processing: '', completed: 'success', rejected: 'info' }
  return map[status] || 'info'
}
function woLabel(status: string): string {
  const map: Record<string, string> = { pending: '待处理', processing: '处理中', completed: '已完成', rejected: '已驳回' }
  return map[status] || status
}

// ---------------- 详情 ----------------
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<Record<string, any> | null>(null)
async function openDetail(row: Record<string, any>) {
  detailVisible.value = true
  detailLoading.value = true
  detail.value = null
  try {
    const res = await getFault(row.id)
    detail.value = res.data
    await loadEvidence(row.id)
  } catch {
    detail.value = null
    ElMessage.error('故障详情加载失败')
  } finally { detailLoading.value = false }
}

// ---------------- 多源证据（识别研判） ----------------
const evidenceLoading = ref(false)
const evidenceList = ref<FaultEvidence[]>([])
async function loadEvidence(id: number | string) {
  evidenceLoading.value = true
  evidenceList.value = []
  try {
    const res = await getFaultEvidence(id)
    evidenceList.value = res.data?.list || []
  } catch {
    evidenceList.value = []
  } finally { evidenceLoading.value = false }
}
function evTag(source?: string): string {
  const map: Record<string, string> = { firmware: '', current: 'success', led_state: 'primary', citizen: 'warning', photo_evidence: '', video_monitor: 'info' }
  return map[source || ''] || 'info'
}

// ---------------- 待确认复核 ----------------
const reviewVisible = ref(false)
const reviewSubmitting = ref(false)
const reviewCur = ref<Record<string, any> | null>(null)
function openReview(row: Record<string, any>) {
  reviewCur.value = row
  reviewVisible.value = true
}
async function submitReview(confirmed: boolean) {
  if (!reviewCur.value) return
  reviewSubmitting.value = true
  try {
    const res = await reviewFault(reviewCur.value.id, confirmed)
    ElMessage.success(res.data?.message || (confirmed ? '已确认真故障' : '已标记误报'))
    reviewVisible.value = false
    reviewCur.value = null
    fetchData(); loadStats()
  } catch {
    // 拦截器已提示
  } finally { reviewSubmitting.value = false }
}

// ---------------- 识别统计面板 ----------------
const statsLoading = ref(false)
const recogStats = ref<Record<string, any> | null>(null)
const accColor = computed(() => {
  const a = recogStats.value?.accuracy ?? 0
  if (a >= 1) return '#67C23A'
  if (a >= 0.9) return '#409EFF'
  return '#E6A23C'
})
async function loadRecognitionStats() {
  statsLoading.value = true
  try {
    const res = await getRecognitionStats()
    recogStats.value = res.data
  } catch {
    recogStats.value = null
  } finally { statsLoading.value = false }
}

// ---------------- 处理 ----------------
const manageVisible = ref(false)
const manageForm = reactive({ status: 'confirmed', owner_id: undefined as number | undefined, repairer_id: undefined as number | undefined })
const curFault = ref<Record<string, any> | null>(null)
function openManage(row: Record<string, any>) {
  curFault.value = row
  Object.assign(manageForm, { status: row.status, owner_id: row.owner_id, repairer_id: row.repairer_id })
  manageVisible.value = true
}
async function saveManage() {
  if (!curFault.value) return
  try {
    await updateFault(curFault.value.id, {
      status: manageForm.status,
      owner_id: manageForm.owner_id || null,
      repairer_id: manageForm.repairer_id || null,
    })
    ElMessage.success('已保存')
    manageVisible.value = false
    fetchData(); loadStats()
  } catch { /* 忽略 */ }
}
async function doDispatch() {
  if (!curFault.value) return
  if (!manageForm.repairer_id) { ElMessage.warning('请选择维修人'); return }
  try {
    await dispatchFault(curFault.value.id, manageForm.repairer_id)
    ElMessage.success('已派单')
    manageVisible.value = false
    fetchData(); loadStats()
  } catch { /* 忽略 */ }
}

// ---------------- 搜索/列表 ----------------
const searchForm = reactive({ device_hw_id: '', status: '', fault_type: '', fault_level: '', recognition_status: '' })
const dateRange = ref<[string, string] | null>(null)
const loading = ref(false)
const tableData = ref<Record<string, any>[]>([])
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: pagination.page,
      page_size: pagination.page_size,
      device_hw_id: searchForm.device_hw_id || undefined,
      status: searchForm.status || undefined,
      fault_type: searchForm.fault_type || undefined,
      fault_level: searchForm.fault_level || undefined,
      recognition_status: searchForm.recognition_status || undefined,
    }
    if (dateRange.value && dateRange.value.length === 2) {
      params.start_date = dateRange.value[0]
      params.end_date = dateRange.value[1]
    }
    const res = await getFaults(params)
    tableData.value = res.data?.list || []
    pagination.total = res.data?.total || 0
  } catch { /* 忽略 */ } finally { loading.value = false }
}

// 导出当前筛选条件下的故障为 CSV（客户端生成下载；复用列表过滤参数）
const faultTypeCNExport: Record<string, string> = { lamp_off: '灯灭', abnormal_on: '异常同亮', timeout: '亮灯超时', dim: '缺亮', power_loss: '断电', unknown: '未知' }
async function exportCsv() {
  const params: Record<string, any> = {
    page: 1, page_size: 5000,
    device_hw_id: searchForm.device_hw_id || undefined,
    status: searchForm.status || undefined,
    fault_type: searchForm.fault_type || undefined,
    fault_level: searchForm.fault_level || undefined,
    recognition_status: searchForm.recognition_status || undefined,
  }
  if (dateRange.value && dateRange.value.length === 2) { params.start_date = dateRange.value[0]; params.end_date = dateRange.value[1] }
  try {
    const res = await getFaults(params)
    const list = res.data?.list || []
    const header = ['ID', '设备硬件ID', '错误码', '故障类型', '等级', '灯态', '红灯', '黄灯', '绿灯', '状态', '研判', '置信度', '首次', '末次']
    const rows = list.map((f: any) => [
      f.id, f.device_hw_id, f.err_code, faultTypeCNExport[f.fault_type] || f.fault_type, f.fault_level,
      f.led_state, f.current_r, f.current_y, f.current_g, faultStatusLabel(f.status), f.recognition_status,
      f.confidence != null ? Number(f.confidence).toFixed(2) : '', f.first_seen, f.last_seen,
    ])
    const csv = '\ufeff' + [header, ...rows].map((r) => r.join(',')).join('\n')
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = `故障_${new Date().toISOString().slice(0, 10)}.csv`
    a.click(); URL.revokeObjectURL(a.href)
    ElMessage.success(`已导出 ${list.length} 条故障`)
  } catch { ElMessage.error('导出失败') }
}

function handleSearch() { pagination.page = 1; fetchData() }
function handleReset() {
  searchForm.device_hw_id = ''; searchForm.status = ''; searchForm.fault_type = ''; searchForm.fault_level = ''; searchForm.recognition_status = ''
  dateRange.value = null; pagination.page = 1; fetchData()
}

onMounted(() => {
  fetchData(); loadStats(); loadOps(); searchDevices(''); loadRecognitionStats()
})
</script>

<style scoped>
.stat-row { margin-bottom: 16px; }
.stat-card { text-align: center; }
.stat-num { font-size: 28px; font-weight: 700; line-height: 1.2; }
.stat-label { color: #909399; font-size: 13px; margin-top: 4px; }
.search-card { margin-bottom: 16px; }
.table-card { border-radius: 4px; }
.pagination { margin-top: 16px; display: flex; justify-content: flex-end; }
.detail-body { padding-right: 6px; }
.detail-desc { margin-bottom: 16px; }
.section-title { margin: 18px 0 10px; color: #303133; font-size: 14px; font-weight: 600; }
.fault-card { border: 1px solid #e4e7ed; border-radius: 6px; padding: 10px; }
.sla-ok { color: #c0c4cc; }

/* 识别统计面板 */
.recog-card { margin-bottom: 16px; }
.recog-head { display: flex; align-items: center; gap: 6px; margin-bottom: 10px; }
.recog-title { font-weight: 600; color: #303133; font-size: 14px; }
.recog-body { }
.recog-item { text-align: center; padding: 6px 0; }
.recog-num { font-size: 22px; font-weight: 700; line-height: 1.2; }
.recog-lab { color: #909399; font-size: 12px; margin-top: 4px; }
.recog-rate { margin-top: 10px; color: #909399; font-size: 12px; text-align: right; }

/* 置信度着色 */
.conf-high { color: #67C23A; font-weight: 600; }
.conf-mid { color: #E6A23C; font-weight: 600; }

/* 多源证据 */
.evidence-area { min-height: 40px; }
.evidence-item { border: 1px solid #e4e7ed; border-radius: 6px; padding: 8px 10px; margin-bottom: 8px; }
.evidence-top { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.evidence-time { color: #909399; font-size: 12px; }
.evidence-conf { margin-left: auto; color: #606266; font-size: 12px; }
.evidence-body { font-size: 13px; color: #303133; }
.evidence-eval { margin-top: 4px; font-size: 12px; color: #909399; }

/* 复核弹窗 */
.review-tip { color: #606266; font-size: 13px; margin: 0 0 8px; }
</style>
