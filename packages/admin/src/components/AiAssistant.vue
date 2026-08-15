<template>
  <el-dialog
    :model-value="visible"
    title="🤖 AI 助手（自然语言操作）"
    width="560px"
    :append-to-body="true"
    :close-on-click-modal="false"
    @update:model-value="$emit('update:visible', $event)"
  >
    <div class="nl-body">
      <!-- 快捷建议 -->
      <div class="nl-quick" v-if="!messages.length">
        <div v-for="s in quickSuggests" :key="s" class="quick-chip" @click="ask(s)">{{ s }}</div>
      </div>

      <!-- 对话消息列表 -->
      <div class="nl-list" ref="listRef">
        <div v-for="(m, i) in messages" :key="i" class="msg" :class="m.role">
          <div class="bubble">
            <div class="msg-text">{{ m.text }}</div>
            <!-- 查询类结构化数据 -->
            <div v-if="m.data && m.table" class="msg-table">
              <el-table :data="m.table" size="small" border>
                <el-table-column v-for="col in m.columns" :key="col" :prop="col" :label="colLabel(col)" />
              </el-table>
            </div>
            <div v-if="m.tool === 'create_fault' || m.tool === 'create_workorder'" class="msg-created">
              <el-tag size="small" type="success">已创建 #{{ m.created_id }}</el-tag>
            </div>
            <div class="msg-meta">
              <el-tag v-if="m.source" size="mini" :type="m.source === 'LLM' ? 'primary' : 'info'" effect="plain">{{ m.source }}</el-tag>
              <el-tag v-if="m.tool" size="mini" type="warning" effect="plain">{{ toolLabel(m.tool) }}</el-tag>
            </div>
          </div>
        </div>
        <div v-if="loading" class="msg assistant"><div class="bubble typing">正在思考…</div></div>
      </div>
    </div>

    <!-- 输入框 -->
    <div class="nl-input">
      <el-input
        v-model="input"
        placeholder="例如：最近7天哪些路口故障最多 / 查询设备123456状态 / 报修：人民路口红灯不亮"
        :disabled="loading"
        @keyup.enter="ask(input)"
      />
      <el-button type="primary" :loading="loading" :disabled="!input.trim()" @click="ask(input)">发送</el-button>
    </div>

    <template #footer>
      <div class="nl-foot-tip">
        <el-icon><InfoFilled /></el-icon>
        <span>AI 可查询故障排行/设备状态/工单统计/费用，也可报修建单（建单为写操作）。</span>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, nextTick } from 'vue'
import { nlInteract, type NLAnswer } from '@/api/copilot'

// props (visible) 与 $emit('update:visible') 由模板直接使用

defineProps<{ visible: boolean }>()
defineEmits<{ (e: 'update:visible', v: boolean): void }>()

interface Msg {
  role: 'user' | 'assistant'
  text: string
  source?: string
  tool?: string
  did_write?: boolean
  created_id?: number
  data?: Record<string, any>
  table?: Record<string, any>[]
  columns?: string[]
}

const input = ref('')
const loading = ref(false)
const messages = ref<Msg[]>([])
const listRef = ref<HTMLDivElement>()

const quickSuggests = [
  '最近7天哪些路口故障最多',
  '查询设备123456状态',
  '工单统计一下',
  '最近30天维修费用',
  '运维健康评分',
  '给出决策建议',
  '怎么新建工单？',
]

function toolLabel(t: string) {
  const map: Record<string, string> = {
    fault_rank: '路口故障排行',
    device_status: '设备状态',
    workorder_stats: '工单统计',
    expense_summary: '费用归因',
    ops_health: '运维健康/决策',
    create_fault: '报修建故障单',
    create_workorder: '命令式建工单',
    kb: '知识库',
  }
  return map[t] || t || '咨询'
}

function colLabel(col: string) {
  const map: Record<string, string> = {
    intersection: '路口',
    count: '故障数',
    hw_id: '设备',
    online: '状态',
    last_checkin: '最后签到',
    watched: '关注',
  }
  return map[col] || col
}

function buildTable(ans: NLAnswer): { table?: Record<string, any>[]; columns?: string[] } {
  const list = ans.data?.list
  if (Array.isArray(list) && list.length) {
    const cols = Object.keys(list[0])
    return {
      table: list.map((r) => {
        const o: Record<string, any> = { ...r }
        if ('online' in o) o.online = o.online ? '在线' : '离线'
        if ('watched' in o) o.watched = o.watched ? '是' : '否'
        return o
      }),
      columns: cols,
    }
  }
  return {}
}

async function ask(text: string) {
  const t = (text || '').trim()
  if (!t || loading.value) return
  messages.value.push({ role: 'user', text: t })
  input.value = ''
  loading.value = true
  scrollBottom()
  try {
    const res = await nlInteract(t)
    const ans: NLAnswer = res.data?.result
    const extra = buildTable(ans)
    messages.value.push({
      role: 'assistant',
      text: ans?.reply || 'AI 未能理解，请换一种说法试试。',
      source: ans?.source,
      tool: ans?.tool,
      did_write: ans?.did_write,
      created_id: ans?.created_id,
      data: ans?.data,
      ...extra,
    })
  } catch {
    messages.value.push({ role: 'assistant', text: '请求失败，请稍后重试。' })
  } finally {
    loading.value = false
    scrollBottom()
  }
}

function scrollBottom() {
  nextTick(() => {
    if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight
  })
}
</script>

<style scoped>
.nl-body {
  min-height: 340px;
  max-height: 460px;
  display: flex;
  flex-direction: column;
}
.nl-quick {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding-bottom: 12px;
}
.quick-chip {
  padding: 5px 12px;
  background: #f0f7ff;
  border: 1px solid #d3e6ff;
  color: #409eff;
  border-radius: 14px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s;
}
.quick-chip:hover {
  background: #409eff;
  color: #fff;
}
.nl-list {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 4px 2px;
}
.msg {
  display: flex;
}
.msg.user {
  justify-content: flex-end;
}
.msg.assistant {
  justify-content: flex-start;
}
.bubble {
  max-width: 88%;
  padding: 10px 12px;
  border-radius: 10px;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}
.msg.user .bubble {
  background: #409eff;
  color: #fff;
  border-top-right-radius: 2px;
}
.msg.assistant .bubble {
  background: #f4f4f5;
  color: #303133;
  border-top-left-radius: 2px;
}
.msg-text {
  margin-bottom: 4px;
}
.msg-table {
  margin-top: 8px;
}
.msg-created {
  margin-top: 6px;
}
.msg-meta {
  display: flex;
  gap: 6px;
  margin-top: 6px;
}
.bubble.typing {
  color: #909399;
}
.nl-input {
  display: flex;
  gap: 8px;
  margin-top: 12px;
}
.nl-foot-tip {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 4px;
  width: 100%;
  font-size: 12px;
  color: #909399;
}
</style>
