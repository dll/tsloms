<template>
  <div class="demo-page">
    <el-alert
      type="warning"
      :closable="false"
      title="系统演示（仅系统管理员可见）"
      description="演示数据使用独立硬件ID段(900001-909999)并带 [演示] 标记，点击「开始演示」生成，点击「结束演示」一键清理，绝不影响生产数据。"
      style="margin-bottom:16px"
    />

    <el-row :gutter="16">
      <!-- 左：演示控制 -->
      <el-col :span="8">
        <el-card shadow="hover">
          <template #header><span>演示控制</span></template>
          <div class="demo-ctl">
            <div class="demo-stat">
              <el-tag :type="status?.running ? 'success' : 'info'">{{ status?.running ? '演示进行中' : '未开始' }}</el-tag>
              <span class="demo-stat-nums" v-if="status">
                设备 {{ status.devices ?? 0 }} · 路口 {{ status.intersection ?? 0 }}
              </span>
            </div>
            <el-form label-width="80px" style="margin-top:12px">
              <el-form-item label="演示设备数">
                <el-input-number v-model="demoN" :min="3" :max="20" style="width:120px" />
              </el-form-item>
            </el-form>
            <div class="demo-btns">
              <el-button type="primary" size="large" :loading="busy" :disabled="status?.running" @click="doStart">
                ▶ 开始演示（生成随机数据）
              </el-button>
              <el-button type="danger" size="large" :loading="busy" :disabled="!status?.running" @click="doEnd">
                ■ 结束演示（清理/回滚）
              </el-button>
            </div>
            <div v-if="lastResult" class="demo-result">
              <el-alert type="success" :closable="false" :title="lastResult.message" />
              <div class="demo-result-grid">
                <span>设备 {{ lastResult.devices ?? 0 }}</span>
                <span>路口 {{ lastResult.intersections ?? 0 }}</span>
                <span>故障 {{ lastResult.faults ?? 0 }}</span>
                <span>工单 {{ lastResult.work_orders ?? 0 }}</span>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>

      <!-- 中：运维全流程（带序号 + 跳转） -->
      <el-col :span="16">
        <el-card shadow="hover">
          <template #header><span>运维全流程演示（点击卡片跳转对应模块）</span></template>
          <div class="flow-grid">
            <div v-for="(s, idx) in flow" :key="s.path" class="flow-card" :class="{ done: idx < doneStep }" @click="go(s.path)">
              <div class="flow-step">{{ idx + 1 }}</div>
              <div class="flow-body">
                <p class="flow-title">{{ s.title }}</p>
                <p class="flow-desc">{{ s.desc }}</p>
                <el-button size="small" :type="idx < doneStep ? 'success' : 'primary'" text>前往 →</el-button>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getDemoStatus, demoStart, demoEnd } from '@/api/demo'

const router = useRouter()
const busy = ref(false)
const demoN = ref(5)
const status = ref<any>({ running: false, devices: 0, intersection: 0 })
const lastResult = ref<any>(null)
const doneStep = ref(0) // 演示进行中的进度（演示模式下逐步点亮）

// 运维全流程（带序号）
const flow = [
  { title: '检测器接入', desc: '真实硬件 / CSV导入 / Mock模拟', path: '/access' },
  { title: '设备管理', desc: '设备台账 / 资料 / 坐标', path: '/device' },
  { title: '路口管理', desc: '路口 / 行政区划 / 新增路口', path: '/intersection' },
  { title: '地图大屏', desc: '设备分布 / 路口信号灯', path: '/map' },
  { title: '故障管理', desc: '故障识别 / 研判 / 复核', path: '/fault' },
  { title: '预警管理', desc: '预警 / 规则 / 处理', path: '/warning' },
  { title: '工单管理', desc: '派单 / 处理 / 闭环 / SLA', path: '/workorder' },
]

function go(p: string) { router.push(p) }

async function load() {
  try { status.value = (await getDemoStatus()).data } catch { /* */ }
}
async function doStart() {
  busy.value = true
  try {
    lastResult.value = (await demoStart(demoN.value)).data
    ElMessage.success('演示数据已生成')
    await load()
    // 演示进行中：逐步点亮流程
    let i = 0
    doneStep.value = 0
    const t = setInterval(() => { i++; doneStep.value = i; if (i >= flow.length) clearInterval(t) }, 800)
  } catch { /* 后端提示 */ } finally { busy.value = false }
}
async function doEnd() {
  try {
    await ElMessageBox.confirm('将清理全部演示数据（独立ID段+标记），生产数据不受影响。确认结束演示？', '结束演示', { type: 'warning' })
    busy.value = true
    lastResult.value = (await demoEnd()).data
    doneStep.value = 0
    ElMessage.success('演示数据已清理')
    await load()
  } catch { /* 取消 */ } finally { busy.value = false }
}

onMounted(load)
</script>

<style scoped>
.demo-ctl .demo-stat { display: flex; align-items: center; gap: 10px; }
.demo-stat-nums { font-size: 13px; color: #909399; }
.demo-btns { display: flex; flex-direction: column; gap: 10px; margin-top: 8px; }
.demo-result { margin-top: 14px; }
.demo-result-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; margin-top: 8px; font-size: 13px; color:#606266; }
.flow-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(210px, 1fr)); gap: 12px; }
.flow-card { display: flex; align-items: flex-start; gap: 10px; border: 1px solid #ebeef5; border-radius: 8px; padding: 12px; cursor: pointer; transition: .2s; }
.flow-card:hover { border-color: #409EFF; box-shadow: 0 2px 8px rgba(0,0,0,.08); }
.flow-card.done { border-color: #67C23A; background: #f0f9eb; }
.flow-step { width: 28px; height: 28px; border-radius: 50%; background: #409EFF; color: #fff; display: flex; align-items: center; justify-content: center; font-weight: 600; flex-shrink: 0; }
.flow-card.done .flow-step { background: #67C23A; }
.flow-body { flex: 1; }
.flow-title { margin: 0 0 4px; font-weight: 600; color: #303133; }
.flow-desc { margin: 0 0 6px; font-size: 12px; color: #909399; }
</style>
