<template>
  <div class="big-screen">
    <!-- 顶部标签：地图大屏（全屏）/ 视频与监控 / 问题反馈 -->
    <div class="screen-tabs">
      <button
        v-for="t in tabs" :key="t.name"
        class="tab-btn" :class="{ active: tab === t.name }"
        @click="goPanel(t.name)"
      >
        <el-icon style="margin-right: 4px"><component :is="t.icon" /></el-icon>{{ t.label }}
      </button>
    </div>

    <!-- 面板内容 -->
    <div class="screen-body">
      <KeepAlive>
        <CesiumMap v-if="tab === 'map'" class="fill" @go-panel="goPanel" />
        <VideoPanel v-else-if="tab === 'video'" class="fill" />
        <FeedbackPanel v-else-if="tab === 'feedback'" class="fill" />
      </KeepAlive>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import CesiumMap from './CesiumMap.vue'
import VideoPanel from './VideoPanel.vue'
import FeedbackPanel from './FeedbackPanel.vue'

const tab = ref('map')
const tabs = [
  { name: 'map', label: '地图大屏', icon: 'MapLocation' },
  { name: 'video', label: '视频与监控', icon: 'VideoCamera' },
  { name: 'feedback', label: '问题反馈', icon: 'ChatDotRound' },
]

// 面板切换（地图联动下钻跳转）
function goPanel(name: string) {
  if (name === 'map' || name === 'video' || name === 'feedback') tab.value = name
}
</script>

<style scoped>
.big-screen {
  width: 100%;
  height: calc(100vh - 92px);
  display: flex;
  flex-direction: column;
}
.screen-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 10px;
}
.tab-btn {
  display: inline-flex;
  align-items: center;
  padding: 8px 18px;
  border: 1px solid #dcdfe6;
  border-radius: 6px;
  background: #fff;
  color: #606266;
  cursor: pointer;
  font-size: 14px;
  transition: all 0.2s;
}
.tab-btn:hover { border-color: #409eff; color: #409eff; }
.tab-btn.active {
  background: #409eff; color: #fff; border-color: #409eff;
}
.screen-body { flex: 1; min-height: 0; }
.fill { width: 100%; height: 100%; }
</style>
