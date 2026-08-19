<template>
  <div class="access-page">
    <div class="page-heading">
      <div>
        <h1>检测器接入</h1>
        <p>统一管理真实硬件、CSV 回放与 Mock 模拟数据接入，实时查看协议链路状态。</p>
      </div>
      <el-button :icon="Refresh" @click="loadStatus">刷新状态</el-button>
    </div>
    <!-- 接入方式总览 -->
    <el-row :gutter="16" class="overview">
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="hover">
          <div class="ov-item" :class="st.mqtt?.connected ? 'ok' : 'warn'">
            <div class="ov-icon"><el-icon :size="30"><Connection /></el-icon></div>
            <div class="ov-meta">
              <p class="ov-label">真实硬件接入（MQTT）</p>
              <p class="ov-val">{{ st.mqtt?.connected ? '已连接' : '未连接' }}</p>
              <p class="ov-sub">{{ st.mqtt?.connected ? '在线设备 ' + (st.real_hardware?.online_devices ?? 0) + ' · 活跃检测器 ' + (st.real_hardware?.active_detectors ?? 0) : '配置 MQTT_BROKER 后自动订阅设备上行' }}</p>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="hover">
          <div class="ov-item ok">
            <div class="ov-icon"><el-icon :size="30"><Upload /></el-icon></div>
            <div class="ov-meta">
              <p class="ov-label">CSV 数据导入</p>
              <p class="ov-val">可用</p>
              <p class="ov-sub">导入设备状态/故障/电流数据并回放研判</p>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="hover">
          <div class="ov-item ok">
            <div class="ov-icon"><el-icon :size="30"><Aim /></el-icon></div>
            <div class="ov-meta">
              <p class="ov-label">Mock 数据模拟</p>
              <p class="ov-val">可用</p>
              <p class="ov-sub">构造合法协议帧，走真实研判/故障/工单链路</p>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="hover">
          <div class="ov-item">
            <div class="ov-icon"><el-icon :size="30"><Monitor /></el-icon></div>
            <div class="ov-meta">
              <p class="ov-label">订阅 Topic</p>
              <p class="ov-val small">{{ st.mqtt?.subscribe || '-' }}</p>
              <p class="ov-sub">协议：trafficLight/{网络号}/{站点号}/{硬件ID}/U</p>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-card class="access-tabs" shadow="never">
      <el-tabs v-model="tab">
        <!-- Tab 1: 真实硬件接入 -->
        <el-tab-pane name="real">
          <template #label><span class="tab-label"><el-icon><Connection /></el-icon>真实硬件接入</span></template>
          <el-alert type="info" :closable="false" title="设备端按协议上报到 MQTT Broker，后台自动订阅并识别设备" style="margin-bottom:12px" />
          <el-descriptions :column="2" border>
            <el-descriptions-item label="接入方式">MQTT Broker（真实检测器）</el-descriptions-item>
            <el-descriptions-item label="当前连接">
              <el-tag :type="st.mqtt?.connected ? 'success' : 'danger'">
                {{ st.mqtt?.connected ? '已连接' : '未连接' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="订阅 Topic">{{ st.mqtt?.subscribe || '-' }}</el-descriptions-item>
            <el-descriptions-item label="在线设备数">{{ st.real_hardware?.online_devices ?? 0 }}</el-descriptions-item>
            <el-descriptions-item label="协议版本">基于 MQTT 3.1.1；上行 Topic：trafficLight/{网络号}/{站点号}/{硬件ID}/U</el-descriptions-item>
            <el-descriptions-item label="自动建账">新硬件ID首次上报自动创建设备档案</el-descriptions-item>
          </el-descriptions>
          <div class="block-title">接入步骤</div>
          <el-steps :active="4" finish-status="success" direction="vertical" class="access-steps">
            <el-step title="配置 MQTT" description="在 server/.env 配置 MQTT_BROKER、MQTT_USERNAME、MQTT_PASSWORD（可选鉴权）。" />
            <el-step title="设备上报" description="检测器按协议发布到 trafficLight/{网络}/{站点}/{硬件ID}/U。" />
            <el-step title="后台研判" description="自动解析 CHECKIN、ALARM、POWER_ON，进入故障与工单链路。" />
            <el-step title="下行应答" description="时间同步、固件查询与升级应答由后台自动下发（/U → /D）。" />
          </el-steps>
        </el-tab-pane>

        <!-- Tab 2: CSV 数据导入 -->
        <el-tab-pane name="csv">
          <template #label><span class="tab-label"><el-icon><Upload /></el-icon>CSV 数据导入</span></template>
          <div class="block-title">按 CSV 批量回放（每行构造一条协议帧，走真实研判链路）</div>
          <el-input
            v-model="csvText"
            type="textarea"
            :rows="7"
            placeholder="hw_id,cmd,err_code,led_state,current_r,current_y,current_g
9001,alarm,-5,0,600,50,40
9002,checkin,-1,0,0,0,0
9003,alarm,-14,0,0,0,0
（cmd: checkin / alarm / power_on；err_code见下方故障码表）"
          />
          <div style="margin-top:12px; display:flex; gap:10px; align-items:center">
            <el-button type="primary" :loading="csvLoading" @click="doCsvImport">导入并回放</el-button>
            <el-button @click="loadCsvSample">填入示例</el-button>
            <span v-if="csvResult" class="csv-msg">
              <el-tag type="success">导入 {{ csvResult.data?.imported ?? 0 }}</el-tag>
              <el-tag v-if="(csvResult.data?.failed ?? 0) > 0" type="danger">失败 {{ csvResult.data?.failed }}</el-tag>
            </span>
          </div>
        </el-tab-pane>

        <!-- Tab 3: Mock 数据模拟 -->
        <el-tab-pane name="mock">
          <template #label><span class="tab-label"><el-icon><Aim /></el-icon>Mock 数据模拟</span></template>
          <div class="block-title">构造一条合法协议帧并投递（无需硬件 / 无需 Broker 在线）</div>
          <el-form :inline="true" :model="mock" label-width="90px">
            <el-form-item label="硬件ID" required>
              <el-input-number v-model="mock.hw_id" :min="1" :max="99999999" style="width:160px" />
            </el-form-item>
            <el-form-item label="命令">
              <el-select v-model="mock.cmd" style="width:130px">
                <el-option label="ALARM 报警" value="alarm" />
                <el-option label="CHECKIN 签到" value="checkin" />
                <el-option label="POWER_ON 上电" value="power_on" />
              </el-select>
            </el-form-item>
            <el-form-item label="故障码">
              <el-select v-model="mock.err_code" style="width:220px" filterable>
                <el-option v-for="f in faultOptions" :key="f.v" :label="f.label" :value="f.v" />
              </el-select>
            </el-form-item>
            <el-form-item label="当前灯态">
              <el-select v-model="mock.led_state" style="width:120px">
                <el-option label="红灯" :value="0" />
                <el-option label="黄灯" :value="1" />
                <el-option label="绿灯" :value="2" />
                <el-option label="未知" :value="-1" />
              </el-select>
            </el-form-item>
            <el-form-item label="电流 R/Y/G">
              <el-input-number v-model="mock.current_r" :min="0" :max="2048" style="width:110px" />
              <el-input-number v-model="mock.current_y" :min="0" :max="2048" style="width:110px; margin-left:6px" />
              <el-input-number v-model="mock.current_g" :min="0" :max="2048" style="width:110px; margin-left:6px" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="mockLoading" @click="doMockSend">发送模拟帧</el-button>
            </el-form-item>
          </el-form>
          <div v-if="mockResult" class="mock-result">
            <el-alert type="success" :closable="false"
                      :title="`已投递：${mockResult.data?.cmd} #${mockResult.data?.hw_id} → ${mockResult.data?.topic}`" />
          </div>
          <div class="block-title">故障码速查（errCode）</div>
          <el-table :data="faultOptions.slice(1)" size="small" border style="max-width:560px">
            <el-table-column prop="v" label="errCode" width="80" />
            <el-table-column prop="label" label="故障含义" />
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Connection, Upload, Aim, Monitor, Refresh } from '@element-plus/icons-vue'
import { getAccessStatus, mockSend, csvImport } from '@/api/access'
import { getDevices } from '@/api/device'

const tab = ref('real')
const st = reactive<any>({})

const faultOptions = [
  { v: 0, label: '0 正常（无错误）' },
  { v: -1, label: '-1 红灯周期全灭' },
  { v: -2, label: '-2 黄灯周期全灭' },
  { v: -3, label: '-3 绿灯周期全灭' },
  { v: -4, label: '-4 红黄同亮' },
  { v: -5, label: '-5 红绿同亮' },
  { v: -6, label: '-6 黄绿同亮' },
  { v: -7, label: '-7 红黄绿同亮' },
  { v: -8, label: '-8 红灯亮灯超时' },
  { v: -9, label: '-9 黄灯亮灯超时' },
  { v: -10, label: '-10 绿灯亮灯超时' },
  { v: -11, label: '-11 红灯缺亮' },
  { v: -12, label: '-12 黄灯缺亮' },
  { v: -13, label: '-13 绿灯缺亮' },
  { v: -14, label: '-14 断电' },
]

const mock = reactive({ hw_id: 9001, cmd: 'alarm', err_code: -5, led_state: 0, current_r: 600, current_y: 50, current_g: 40 })
const mockLoading = ref(false)
const mockResult = ref<any>(null)
async function doMockSend() {
  mockLoading.value = true
  try {
    mockResult.value = await mockSend({ ...mock })
    ElMessage.success(`已投递模拟帧 #${mock.hw_id} ${mock.cmd}`)
    loadStatus()
  } catch { /* 后端提示 */ } finally { mockLoading.value = false }
}

const csvText = ref('')
const csvLoading = ref(false)
const csvResult = ref<any>(null)
function loadCsvSample() {
  csvText.value = [
    'hw_id,cmd,err_code,led_state,current_r,current_y,current_g',
    '9001,alarm,-5,0,600,50,40',
    '9002,checkin,-1,0,0,0,0',
    '9003,alarm,-8,0,820,60,55',
    '9004,alarm,-14,0,0,0,0',
    '9005,power_on,,,0,0,0',
  ].join('\n')
}
async function doCsvImport() {
  if (!csvText.value.trim()) { ElMessage.warning('请填写 CSV 内容'); return }
  csvLoading.value = true
  try {
    csvResult.value = await csvImport({ content: csvText.value })
    ElMessage.success(`已导入 ${csvResult.value?.data?.imported ?? 0} 条`)
    loadStatus()
  } catch { /* 后端提示 */ } finally { csvLoading.value = false }
}

async function loadStatus() {
  try { Object.assign(st, (await getAccessStatus()).data) } catch { /* 忽略 */ }
}

// 预填一个已存在设备（便于模拟该设备）
async function loadDevice() {
  try {
    const res = await getDevices({ page: 1, page_size: 1, online_status: 'true' })
    const first = res.data?.list?.[0]
    if (first?.hw_id && mock.hw_id === 9001) {
      // 数据库 hw_id 为 uuid 字符串；协议模拟帧需要 uint32，仅当纯数字时预填
      const n = Number(first.hw_id)
      if (/^\d+$/.test(first.hw_id) && Number.isFinite(n) && n >= 1 && n <= 99999999) mock.hw_id = n
    }
  } catch { /* 忽略 */ }
}

onMounted(() => { loadStatus(); loadDevice() })
</script>

<style scoped>
.access-page { max-width: 1440px; margin: 0 auto; }
.access-page :deep(.el-descriptions__label),
.access-page :deep(.el-descriptions__content) { font-size: 13px; }
.access-page :deep(.el-alert__title) { font-size: 14px; font-weight: 400; }
.access-page :deep(.el-tabs__item) { font-size: 14px; font-weight: 500; }
.overview { margin-bottom: 18px; }
.overview :deep(.el-col) { margin-bottom: 16px; }
.overview :deep(.el-card) { height: 100%; }
.ov-item { display: flex; align-items: center; gap: 12px; }
.ov-item.ok .ov-icon { color: #67C23A; }
.ov-item.warn .ov-icon { color: #E6A23C; }
.ov-icon { font-size: 18px; color: #409EFF; }
.ov-label { margin: 0; font-size: 13px; color: #64748b; }
.ov-val { margin: 4px 0 0; font-size: 18px; font-weight: 600; color: #1f2937; }
.ov-val.small { font-size: 13px; font-weight: 500; }
.ov-sub { margin: 2px 0 0; font-size: 13px; color: #909399; line-height: 1.4; }
.access-tabs { padding: 4px 8px 14px; }
.block-title { margin: 18px 0 10px; font-weight: 600; color: #1f2937; font-size: 14px; }
.access-steps { max-width: 760px; padding: 4px 0 8px 4px; }
.access-steps :deep(.el-step__title) { font-size: 14px; }
.access-steps :deep(.el-step__description) { line-height: 1.7; }
.csv-msg { margin-left: 8px; }
.mock-result { margin-bottom: 12px; }
.tab-label { display: inline-flex; align-items: center; gap: 6px; font-weight: 600; }
.access-tabs :deep(.el-tabs__header) { margin: 0 0 18px; }
.access-tabs :deep(.el-tabs__item) { height: 44px; }
.access-tabs :deep(.el-tabs__nav-wrap::after) { background-color: #e5eaf2; }
.access-tabs :deep(.el-tabs__item.is-active) { color: var(--ts-primary); }
.access-tabs :deep(.el-form) { max-width: 1080px; }
.access-tabs :deep(.el-table) { max-width: 760px !important; }
</style>
