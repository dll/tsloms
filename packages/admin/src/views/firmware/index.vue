<template>
  <div class="firmware-page">
    <el-card shadow="never">
      <el-empty description="固件OTA升级功能开发中" :image-size="200">
        <template #image>
          <el-icon :size="120" color="#C0C4CC"><Upload /></el-icon>
        </template>
      </el-empty>
    </el-card>
  </div>
</template>

<script setup lang="ts">
// 固件管理页面 - OTA升级功能占位
</script>

<style scoped>
.firmware-page {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 400px;
}
</style>
