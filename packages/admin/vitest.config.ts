import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// Vitest 配置：单元测试用 jsdom 环境，复用 @ 别名与 vue 插件。
// 注意：不引入 vite.config.js 中的 Cesium 拷贝插件，避免测试期拷贝大资源。
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    include: ['tests/**/*.test.ts'],
    exclude: ['src/**', 'node_modules/**'],
    // Vitest4：线程池（forks 在某些环境启动超时）
    pool: 'threads',
  },
})
