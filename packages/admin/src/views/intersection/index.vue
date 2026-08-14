<template>
  <div class="intersection-page">
    <el-card shadow="never">
      <template #header>
        <div class="header-bar">
          <span>路口管理（{{ total }} 个路口）</span>
          <div class="header-actions">
            <el-input v-model="keyword" placeholder="搜索路口名称" clearable style="width: 200px" @input="applyFilter" />
            <el-button type="primary" @click="goMap">地图大屏</el-button>
          </div>
        </div>
      </template>

      <el-table :data="filteredList" v-loading="loading" border stripe style="width: 100%">
        <el-table-column prop="intersection" label="路口名称" min-width="160" />
        <el-table-column prop="device_total" label="设备总数" width="100" align="center" />
        <el-table-column label="在线" width="90" align="center">
          <template #default="{ row }">
            <el-tag type="success" size="small">{{ row.online }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="离线" width="90" align="center">
          <template #default="{ row }">
            <el-tag type="info" size="small">{{ row.offline }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="活跃故障" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.fault > 0 ? 'danger' : 'success'" size="small">{{ row.fault }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="经纬度" min-width="140" align="center">
          <template #default="{ row }">
            <span v-if="row.lat !== null && row.lat !== undefined">{{ row.lat?.toFixed(5) }}, {{ row.lng?.toFixed(5) }}</span>
            <span v-else class="no-coord">未设置</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" align="center">
          <template #default="{ row }">
            <el-button size="small" @click="goDevices(row.intersection)">查看设备</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && filteredList.length === 0" description="暂无路口数据" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getIntersections, type IntersectionItem } from '@/api/intersection'

const router = useRouter()
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
  try {
    const res = await getIntersections()
    list.value = res.data?.list || []
  } catch {
    ElMessage.error('路口数据加载失败')
  } finally {
    loading.value = false
  }
}

function goDevices(intersection: string) {
  router.push({ path: '/device', query: { intersection } })
}

function goMap() {
  router.push('/map')
}

onMounted(load)
</script>

<style scoped>
.header-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.header-actions {
  display: flex;
  gap: 8px;
}
.no-coord {
  color: #c0c4cc;
}
</style>
