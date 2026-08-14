import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve, dirname } from 'path'
import { fileURLToPath } from 'url'
import { copyFileSync, mkdirSync, readdirSync, statSync } from 'fs'

const __dirname = fileURLToPath(new URL('.', import.meta.url))

// Cesium 静态资源目录（Build/Cesium 下的 Assets/Widgets/Workers/ThirdParty）
const CESIUM_DIR = resolve(__dirname, 'node_modules/cesium/Build/Cesium')

// 递归拷贝目录
function copyDir(src, dest) {
  mkdirSync(dest, { recursive: true })
  for (const entry of readdirSync(src)) {
    const sp = resolve(src, entry)
    const dp = resolve(dest, entry)
    if (statSync(sp).isDirectory()) {
      copyDir(sp, dp)
    } else {
      copyFileSync(sp, dp)
    }
  }
}

// 简易 vite 插件：构建后把 Cesium 静态资源拷到 outDir/cesium
function copyCesiumAssets() {
  let outDir = 'dist'
  return {
    name: 'copy-cesium-assets',
    configResolved(config) {
      outDir = config.build.outDir
    },
    writeBundle(config) {
      const dest = resolve(__dirname, outDir, 'cesium')
      copyDir(CESIUM_DIR, dest)
    },
  }
}

export default defineConfig({
  base: '/tsloms/admin/',
  plugins: [
    vue(),
    copyCesiumAssets(),
  ],
  define: {
    // Cesium 全局：让 Cesium 的 worker 使用我们拷贝的静态资源
    CESIUM_BASE_URL: JSON.stringify('/tsloms/admin/cesium/'),
  },
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  build: {
    chunkSizeWarningLimit: 2000,
  },
  server: {
    port: 3001,
    proxy: {
      '/api': {
        target: 'http://localhost:8093',
        changeOrigin: true,
      },
      '/cesium': {
        target: 'http://localhost:3001',
        changeOrigin: true,
      },
    },
  },
})
