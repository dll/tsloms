<template>
  <div class="big-screen">
    <el-tabs v-model="tab" type="border-card">
      <!-- 2D/3D 地图定位大屏 -->
      <el-tab-pane label="地图定位（2D/3D）" name="map">
        <CesiumMap />
      </el-tab-pane>

      <!-- 视频与监控 -->
      <el-tab-pane label="视频与监控" name="video">
        <VideoPanel />
      </el-tab-pane>

      <!-- 问题反馈 -->
      <el-tab-pane label="问题反馈" name="feedback">
        <FeedbackPanel />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import CesiumMap from './CesiumMap.vue'
import VideoPanel from './VideoPanel.vue'
import FeedbackPanel from './FeedbackPanel.vue'

const tab = ref('map')
</script>

<style scoped>
.big-screen { width: 100%; }
</style>
