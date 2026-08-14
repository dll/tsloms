<template>
  <div class="map-full">
    <CesiumMap />
  </div>
</template>

<script setup lang="ts">
import CesiumMap from './CesiumMap.vue'
</script>

<style scoped>
.map-full { width: 100%; height: calc(100vh - 92px); }
</style>
