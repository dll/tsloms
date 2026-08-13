<template>
  <div class="log-page">
    <el-card shadow="never">
      <el-tabs v-model="activeTab" type="card" @tab-change="handleTabChange">
        <!-- 报文日志 -->
        <el-tab-pane label="报文日志" name="packet">
          <el-table :data="packetLogs" border stripe style="width: 100%" v-loading="loading">
            <el-table-column prop="device_hw_id" label="设备ID" width="160" align="center" />
            <el-table-column label="命令类型" width="120" align="center">
              <template #default="{ row }">
                {{ cmdTypeLabel(row.cmd_type) }}
              </template>
            </el-table-column>
            <el-table-column prop="cmd_seq" label="包序号" width="100" align="center" />
            <el-table-column label="是否有效" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="row.valid ? 'success' : 'danger'" size="small">
                  {{ row.valid ? '有效' : '无效' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="parsed_result" label="解析结果" min-width="240" show-overflow-tooltip />
            <el-table-column prop="received_at" label="接收时间" width="180" align="center" />
          </el-table>
          <el-pagination
            v-if="packetTotal > packetPageSize"
            style="margin-top: 12px; justify-content: flex-end"
            layout="total, prev, pager, next"
            :total="packetTotal"
            :page-size="packetPageSize"
            v-model:current-page="packetPage"
            @current-change="fetchPacketLogs"
          />
          <el-empty v-if="!loading && packetLogs.length === 0" description="暂无报文日志数据" />
        </el-tab-pane>

        <!-- 操作日志 -->
        <el-tab-pane label="操作日志" name="operation">
          <el-table :data="operationLogs" border stripe style="width: 100%" v-loading="loading">
            <el-table-column prop="username" label="操作人" width="120" align="center" />
            <el-table-column prop="action" label="操作类型" width="150" align="center" />
            <el-table-column prop="target" label="操作对象" width="180" align="center" />
            <el-table-column prop="ip" label="IP地址" width="140" align="center" />
            <el-table-column prop="created_at" label="操作时间" width="180" align="center" />
            <el-table-column prop="detail" label="操作详情" min-width="200" show-overflow-tooltip />
          </el-table>
          <el-pagination
            v-if="operationTotal > operationPageSize"
            style="margin-top: 12px; justify-content: flex-end"
            layout="total, prev, pager, next"
            :total="operationTotal"
            :page-size="operationPageSize"
            v-model:current-page="operationPage"
            @current-change="fetchOperationLogs"
          />
          <el-empty v-if="!loading && operationLogs.length === 0" description="暂无操作日志数据" />
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getPacketLogs, getOperationLogs } from '@/api/log'

// 当前激活的标签页
const activeTab = ref('packet')
const loading = ref(false)

// 报文日志数据
const packetLogs = ref<Record<string, any>[]>([])
const packetTotal = ref(0)
const packetPage = ref(1)
const packetPageSize = ref(20)

// 操作日志数据
const operationLogs = ref<Record<string, any>[]>([])
const operationTotal = ref(0)
const operationPage = ref(1)
const operationPageSize = ref(20)

// 命令类型映射
const cmdTypeLabel = (cmd: number): string => {
  const map: Record<number, string> = {
    0x00: '签到', 0x01: '告警', 0x03: '上电',
    0x20: '配置下发', 0x30: '固件查询', 0x31: '固件请求', 0x7F: '重启',
  }
  return map[cmd] ?? '未知(0x' + (cmd ?? 0).toString(16).padStart(2, '0') + ')'
}

// 获取报文日志
async function fetchPacketLogs() {
  loading.value = true
  try {
    const res = await getPacketLogs({
      page: packetPage.value,
      page_size: packetPageSize.value,
    })
    packetLogs.value = res.data?.list || []
    packetTotal.value = res.data?.total || 0
  } catch {
    ElMessage.error('报文日志加载失败')
  } finally {
    loading.value = false
  }
}

// 获取操作日志
async function fetchOperationLogs() {
  loading.value = true
  try {
    const res = await getOperationLogs({ page: operationPage.value, page_size: operationPageSize.value })
    operationLogs.value = res.data?.list || []
    operationTotal.value = res.data?.total || 0
  } catch {
    ElMessage.error('操作日志加载失败')
  } finally {
    loading.value = false
  }
}

// 标签页切换时加载对应数据（避免重复请求）
let packetLoaded = false
let operationLoaded = false

function handleTabChange(name: string | number | undefined) {
  if (name === 'packet') {
    if (!packetLoaded) { packetLoaded = true; fetchPacketLogs() }
  } else if (name === 'operation') {
    if (!operationLoaded) { operationLoaded = true; fetchOperationLogs() }
  }
}

onMounted(() => {
  fetchPacketLogs()
})
</script>

<style scoped>
.log-page {
  min-height: 400px;
}
</style>
