<template>
  <div class="patrol-page">
    <el-tabs v-model="tab">
      <!-- 任务管理 -->
      <el-tab-pane label="巡检任务" name="tasks">
        <el-card shadow="never">
          <template #header>
            <div style="display:flex;justify-content:space-between;align-items:center">
              <span>自动巡检任务</span>
              <el-button type="primary" size="small" :icon="Plus" @click="openCreate">新建任务</el-button>
            </div>
          </template>
          <el-table :data="tasks" v-loading="taskLoading" border stripe>
            <el-table-column prop="id" label="ID" width="60" align="center" />
            <el-table-column prop="name" label="任务名称" min-width="140" />
            <el-table-column label="模式" width="110" align="center">
              <template #default="{ row }">{{ modeLabel(row.mode) }}</template>
            </el-table-column>
            <el-table-column label="状态" width="90" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="statusTag(row.status)">{{ statusLabel(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="target_count" label="目标数" width="80" align="center">
              <template #default="{ row }">{{ row.target_count || '-' }}</template>
            </el-table-column>
            <el-table-column prop="run_count" label="执行次数" width="90" align="center" />
            <el-table-column prop="last_run_at" label="最近执行" width="165" />
            <el-table-column label="操作" width="180" align="center">
              <template #default="{ row }">
                <el-button link type="primary" size="small" @click="handleRun(row)">执行</el-button>
                <el-button link type="danger" size="small" @click="handleDelete(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- 巡检记录 -->
      <el-tab-pane label="巡检记录" name="records">
        <el-card shadow="never">
          <el-table :data="records" v-loading="recLoading" border stripe>
            <el-table-column prop="id" label="ID" width="60" align="center" />
            <el-table-column prop="device_hw_id" label="设备ID" width="100" align="center" />
            <el-table-column prop="intersection" label="路口" width="160" show-overflow-tooltip />
            <el-table-column label="结果" width="90" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="row.check_result === 'normal' ? 'success' : 'danger'">
                  {{ row.check_result === 'normal' ? '正常' : '异常' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="check_detail" label="详情" min-width="180" show-overflow-tooltip />
            <el-table-column prop="patrol_by" label="巡检人" width="100" align="center" />
            <el-table-column prop="patrol_at" label="时间" width="165" />
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- 排行 -->
      <el-tab-pane label="巡检排行" name="ranking">
        <el-card shadow="never">
          <el-radio-group v-model="rankDim" size="small" style="margin-bottom:10px">
            <el-radio-button value="person">人员</el-radio-button>
            <el-radio-button value="intersection">路口</el-radio-button>
          </el-radio-group>
          <el-table :data="ranking" v-loading="rankLoading" border stripe>
            <el-table-column type="index" label="#" width="60" align="center" />
            <el-table-column :prop="rankDim === 'person' ? 'name' : 'name'" label="维度" min-width="160" />
            <el-table-column prop="count" label="巡检人次" width="100" align="center" />
            <el-table-column prop="abnormal" label="异常数" width="100" align="center" />
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- 即时自检 -->
      <el-tab-pane label="信号灯自检" name="selfcheck">
        <el-card shadow="never">
          <div style="margin-bottom:10px">
            <el-input v-model="selfcheckHw" placeholder="输入设备硬件ID（逗号分隔，如 8001,8002）" style="width:320px;margin-right:8px" />
            <el-button type="primary" :loading="scLoading" @click="handleSelfCheck">自检</el-button>
          </div>
          <el-table :data="selfcheckList" v-loading="scLoading" border stripe>
            <el-table-column prop="hw_id" label="设备ID" width="110" align="center" />
            <el-table-column label="在线" width="80" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="row.online ? 'success' : 'info'">{{ row.online ? '在线' : '离线' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="结果" width="90" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="row.result === 'normal' ? 'success' : 'danger'">{{ row.result }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="detail" label="详情" min-width="200" show-overflow-tooltip />
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 新建任务对话框 -->
    <el-dialog v-model="createDlg" title="新建巡检任务" width="520px" append-to-body>
      <el-form :model="form" label-width="90px">
        <el-form-item label="任务名称"><el-input v-model="form.name" placeholder="如：庐阳区早高峰巡检" /></el-form-item>
        <el-form-item label="模式">
          <el-select v-model="form.mode" style="width:100%">
            <el-option v-for="m in PATROL_MODES" :key="m.value" :label="m.label" :value="m.value" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.mode === 'random'" label="抽检数量">
          <el-input-number v-model="form.target_count" :min="1" :max="500" />
        </el-form-item>
        <el-form-item label="时间窗口">
          <el-input v-model="form.time_window" placeholder="如 08:00-10:00（可选）" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDlg = false">取消</el-button>
        <el-button type="primary" :loading="busy" @click="handleCreate">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import {
  getPatrolTasks, createPatrolTask, deletePatrolTask, runPatrolTask,
  getPatrolRecords, getPatrolRanking, postPatrolSelfCheck, PATROL_MODES, PATROL_STATUS,
  type PatrolTask,
} from '@/api/patrol'

const tab = ref('tasks')
const taskLoading = ref(false)
const recLoading = ref(false)
const rankLoading = ref(false)
const scLoading = ref(false)
const busy = ref(false)
const tasks = ref<PatrolTask[]>([])
const records = ref<any[]>([])
const ranking = ref<any[]>([])
const rankDim = ref('person')
const selfcheckHw = ref('')
const selfcheckList = ref<any[]>([])
const createDlg = ref(false)
const form = reactive<any>({ name: '', mode: 'street', target_count: 10, time_window: '', remark: '' })

function modeLabel(m: string) { return PATROL_MODES.find((x) => x.value === m)?.label || m }
function statusLabel(s: string) { return PATROL_STATUS.find((x) => x.value === s)?.label || s || '-' }
function statusTag(s: string) { return ({ planned: 'info', running: 'warning', done: 'success' } as any)[s as any] || 'info' }

async function loadTasks() {
  taskLoading.value = true
  try { const res = await getPatrolTasks({ page: 1, page_size: 100 }); tasks.value = res.data?.list || [] }
  finally { taskLoading.value = false }
}
async function loadRecords() {
  recLoading.value = true
  try { const res = await getPatrolRecords({ page: 1, page_size: 100 }); records.value = res.data?.list || [] }
  finally { recLoading.value = false }
}
async function loadRanking() {
  rankLoading.value = true
  try { const res = await getPatrolRanking(rankDim.value); ranking.value = res.data?.list || [] }
  finally { rankLoading.value = false }
}
watch(tab, (v) => { if (v === 'records') loadRecords(); if (v === 'ranking') loadRanking() })

function openCreate() {
  Object.assign(form, { name: '', mode: 'street', target_count: 10, time_window: '', remark: '' })
  createDlg.value = true
}
async function handleCreate() {
  if (!form.name || !form.mode) { ElMessage.warning('请填写名称与模式'); return }
  busy.value = true
  try { await createPatrolTask({ ...form }); ElMessage.success('创建成功'); createDlg.value = false; loadTasks() }
  finally { busy.value = false }
}
async function handleRun(row: PatrolTask) {
  busy.value = true
  try {
    const res = await runPatrolTask(row.id)
    ElMessage.success(`执行完成：生成 ${res.data?.created || 0} 条，异常 ${res.data?.abnormal || 0}`)
    loadTasks()
  } finally { busy.value = false }
}
async function handleDelete(row: PatrolTask) {
  await ElMessageBox.confirm(`确定删除任务「${row.name}」？`, '提示', { type: 'warning' })
  await deletePatrolTask(row.id); ElMessage.success('已删除'); loadTasks()
}
async function handleSelfCheck() {
  const hws = selfcheckHw.value.split(/[,\s，]+/).filter(Boolean).map(Number)
  if (!hws.length) { ElMessage.warning('请输入设备ID'); return }
  scLoading.value = true
  try { const res = await postPatrolSelfCheck(hws); selfcheckList.value = res.data?.list || [] }
  finally { scLoading.value = false }
}

onMounted(loadTasks)
</script>
