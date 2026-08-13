<template>
  <div class="log-page">
    <el-card shadow="never">
      <el-tabs v-model="activeTab" type="card">
        <!-- 报文日志 -->
        <el-tab-pane label="报文日志" name="packet">
          <el-table :data="packetLogs" border stripe style="width: 100%" v-loading="loading">
            <el-table-column prop="device_hw_id" label="设备ID" width="180" align="center" />
            <el-table-column prop="command_type" label="命令类型" width="120" align="center" />
            <el-table-column prop="seq_num" label="包序号" width="100" align="center" />
            <el-table-column label="是否有效" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="row.valid ? 'success' : 'danger'" size="small">
                  {{ row.valid ? '有效' : '无效' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="received_at" label="接收时间" width="180" align="center" />
            <el-table-column prop="payload" label="报文内容" min-width="200" show-overflow-tooltip />
          </el-table>
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
          <el-empty v-if="!loading && operationLogs.length === 0" description="暂无操作日志数据" />
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'

// 当前激活的标签页
const activeTab = ref('packet')
const loading = ref(false)

// 报文日志数据（预留，待后端API就绪后对接）
const packetLogs = ref<Record<string, any>[]>([])

// 操作日志数据（预留，待后端API就绪后对接）
const operationLogs = ref<Record<string, any>[]>([])

onMounted(() => {
  // 日志API预留，暂不加载数据
})
</script>

<style scoped>
.log-page {
  min-height: 400px;
}
</style>
