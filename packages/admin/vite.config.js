import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import { fileURLToPath } from 'url'
import { copyFileSync, mkdirSync, readdirSync, statSync, writeFileSync, readFileSync } from 'fs'
import { bundleBudgetPlugin } from './build/bundle-budget.mjs'

const __dirname = fileURLToPath(new URL('.', import.meta.url))

// Cesium 静态资源目录（Build/Cesium 下的 Assets/Widgets/Workers/ThirdParty）
const CESIUM_DIR = resolve(__dirname, 'node_modules/cesium/Build/Cesium')
const PACKAGE_VERSION = JSON.parse(readFileSync(resolve(__dirname, 'package.json'), 'utf8')).version
const [VERSION_MAJOR, VERSION_MINOR] = PACKAGE_VERSION.split('.')
// CI 构建号保证每次流水线产物可区分；主次版本仍由 package.json 明确维护。
const BUILD_NUMBER = process.env.GITHUB_RUN_NUMBER || process.env.BUILD_NUMBER || ''
const APP_VERSION = BUILD_NUMBER ? `${VERSION_MAJOR}.${VERSION_MINOR}.${BUILD_NUMBER}` : PACKAGE_VERSION
const BUILD_COMMIT = process.env.GITHUB_SHA || 'local'
const BUILD_TIME = new Date().toISOString()

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
    writeBundle(_config) {
      const dest = resolve(__dirname, outDir, 'cesium')
      copyDir(CESIUM_DIR, dest)
      writeFileSync(resolve(__dirname, outDir, 'version.json'), JSON.stringify({
        version: APP_VERSION,
        commit: BUILD_COMMIT,
        build: BUILD_NUMBER || 'local',
        built_at: BUILD_TIME,
      }, null, 2) + '\n')
    },
  }
}

export default defineConfig({
  base: '/tsloms/admin/',
  plugins: [
    vue(),
    copyCesiumAssets(),
    bundleBudgetPlugin(),
  ],
  define: {
    // Cesium 全局：让 Cesium 的 worker 使用我们拷贝的静态资源
    CESIUM_BASE_URL: JSON.stringify('/tsloms/admin/cesium/'),
    'import.meta.env.VITE_APP_VERSION': JSON.stringify(APP_VERSION),
    'import.meta.env.VITE_BUILD_COMMIT': JSON.stringify(BUILD_COMMIT),
    'import.meta.env.VITE_BUILD_NUMBER': JSON.stringify(BUILD_NUMBER || 'local'),
    'import.meta.env.VITE_BUILD_TIME': JSON.stringify(BUILD_TIME),
  },
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  build: {
    // CI-P2-01：体积预算由 bundleBudgetPlugin 显式管控并硬性阻断；
    // 此处提高默认 warning 阈值以避免与自定义预算重复告警（cesium 单码 >2000kB 属预期）。
    chunkSizeWarningLimit: 5000,
    // 分包：把 Cesium / ECharts / Vue 框架独立成 chunk，改善首屏加载与长期缓存命中
    rollupOptions: {
      output: {
        manualChunks: {
          cesium: ['cesium'],
          echarts: ['echarts'],
          vendor: ['vue', 'vue-router', 'pinia', 'axios', 'element-plus'],
        },
      },
    },
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
