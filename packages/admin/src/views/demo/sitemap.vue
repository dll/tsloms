<template>
  <div class="sitemap-page">
    <el-alert
      type="info"
      :closable="false"
      title="系统导览 · 网站地图（有向无环图 DAG）"
      description="所有主菜单一览；红色粗线为运维核心流程，点击可跳转对应模块。"
      style="margin-bottom: 14px"
    />
    <el-card shadow="never" body-style="padding:8px">
      <div class="dag" @click="onSvgClick">
        <svg viewBox="0 0 1500 820" xmlns="http://www.w3.org/2000/svg" role="img" aria-label="TSLOMS 系统菜单 DAG">
          <defs>
            <marker id="arr" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
              <path d="M0,0 L10,5 L0,10 z" fill="#cbd5e1"></path>
            </marker>
            <marker id="arr-red" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" orient="auto-start-reverse">
              <path d="M0,0 L10,5 L0,10 z" fill="#dc2626"></path>
            </marker>
            <marker id="arr-dash" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
              <path d="M0,0 L10,5 L0,10 z" fill="#94a3b8"></path>
            </marker>
          </defs>

          <!-- 分区 -->
          <rect x="35" y="268" width="1500" height="96" rx="16" class="zone"></rect>
          <text x="46" y="262" class="zone-label">★ 运维核心流程（红色）— 检测器接入 · 设备管理 · 路口管理 · 地图大屏 · 故障管理 · 预警管理 · 工单管理</text>
          <rect x="35" y="430" width="1500" height="170" rx="16" class="zone"></rect>
          <text x="46" y="424" class="zone-label">核心基础模块（恒启）</text>
          <rect x="35" y="660" width="1500" height="150" rx="16" class="zone"></rect>
          <text x="46" y="654" class="zone-label">可选模块（按购买启用）· 库存与成本分组</text>

          <!-- 边 -->
          <path class="edge red" d="M 590 172 C 325 172, 325 300, 135 300" marker-end="url(#arr-red)"></path>
          <path class="edge red" d="M 210 332 C 240 332, 240 332, 270 332" marker-end="url(#arr-red)"></path>
          <path class="edge red" d="M 420 332 C 450 332, 450 332, 480 332" marker-end="url(#arr-red)"></path>
          <path class="edge red" d="M 630 332 C 660 332, 660 332, 690 332" marker-end="url(#arr-red)"></path>
          <path class="edge red" d="M 840 332 C 870 332, 870 332, 900 332" marker-end="url(#arr-red)"></path>
          <path class="edge red" d="M 1050 332 C 1080 332, 1080 332, 1110 332" marker-end="url(#arr-red)"></path>
          <path class="edge red" d="M 1260 332 C 1290 332, 1290 332, 1320 332" marker-end="url(#arr-red)"></path>

          <path class="edge dash" d="M 590 204 C 120 204, 120 460, 135 460" marker-end="url(#arr-dash)"></path>
          <path class="edge dash" d="M 590 204 C 280 204, 280 460, 285 460" marker-end="url(#arr-dash)"></path>
          <path class="edge dash" d="M 975 364 C 975 400, 975 400, 975 440" marker-end="url(#arr-dash)"></path>
          <path class="edge dash" d="M 1185 364 C 1185 400, 1185 400, 1185 440" marker-end="url(#arr-dash)"></path>
          <path class="edge dash" d="M 1395 364 C 1395 430, 1395 430, 1395 456" marker-end="url(#arr-dash)"></path>
          <path class="edge dash" d="M 1395 376 C 1350 500, 1350 500, 1275 512" marker-end="url(#arr-dash)"></path>
          <path class="edge dash" d="M 975 504 C 1030 460, 1030 440, 1110 440" marker-end="url(#arr-dash)"></path>
          <path class="edge dash" d="M 1395 364 C 1330 760, 1330 760, 1240 680" marker-end="url(#arr-dash)"></path>

          <!-- 节点 -->
          <g class="node gray" transform="translate(510,20)" data-path="/">
            <rect width="160" height="52" rx="10"></rect>
            <text x="80" y="30" text-anchor="middle" font-size="14" font-weight="600">登录 / 注册</text>
          </g>
          <g class="node gray" transform="translate(510,120)" data-path="/dashboard">
            <rect width="160" height="60" rx="10"></rect>
            <text x="80" y="26" text-anchor="middle" font-size="14" font-weight="600">仪表盘</text>
            <text x="80" y="45" text-anchor="middle" class="sub">总览看板</text>
          </g>

          <g class="node core" transform="translate(60,300)" data-path="/access">
            <rect width="150" height="64" rx="10"></rect>
            <text x="75" y="26" text-anchor="middle" class="core-title">检测器接入</text>
            <text x="75" y="45" text-anchor="middle" class="sub">硬件 / CSV / Mock</text>
          </g>
          <g class="node core" transform="translate(270,300)" data-path="/device">
            <rect width="150" height="64" rx="10"></rect>
            <text x="75" y="26" text-anchor="middle" class="core-title">设备管理</text>
            <text x="75" y="45" text-anchor="middle" class="sub">台账 / 资料 / 坐标</text>
          </g>
          <g class="node core" transform="translate(480,300)" data-path="/intersection">
            <rect width="150" height="64" rx="10"></rect>
            <text x="75" y="26" text-anchor="middle" class="core-title">路口管理</text>
            <text x="75" y="45" text-anchor="middle" class="sub">路口 / 行政区划</text>
          </g>
          <g class="node core" transform="translate(690,300)" data-path="/map">
            <rect width="150" height="64" rx="10"></rect>
            <text x="75" y="26" text-anchor="middle" class="core-title">地图大屏</text>
            <text x="75" y="45" text-anchor="middle" class="sub">设备分布 / 信号灯</text>
          </g>
          <g class="node core" transform="translate(900,300)" data-path="/fault">
            <rect width="150" height="64" rx="10"></rect>
            <text x="75" y="26" text-anchor="middle" class="core-title">故障管理</text>
            <text x="75" y="45" text-anchor="middle" class="sub">识别 / 研判 / 复核</text>
          </g>
          <g class="node core" transform="translate(1110,300)" data-path="/warning">
            <rect width="150" height="64" rx="10"></rect>
            <text x="75" y="26" text-anchor="middle" class="core-title">预警管理</text>
            <text x="75" y="45" text-anchor="middle" class="sub">预警 / 规则 / 处理</text>
          </g>
          <g class="node core" transform="translate(1320,300)" data-path="/workorder">
            <rect width="150" height="64" rx="10"></rect>
            <text x="75" y="26" text-anchor="middle" class="core-title">工单管理</text>
            <text x="75" y="45" text-anchor="middle" class="sub">派单 / 处理 / SLA</text>
          </g>

          <g class="node blue" transform="translate(60,460)" data-path="/demo">
            <rect width="150" height="58" rx="10"></rect>
            <text x="75" y="26" text-anchor="middle" font-size="14" font-weight="600">系统演示</text>
            <text x="75" y="45" text-anchor="middle" class="sub">全流程演示</text>
          </g>
          <g class="node blue" transform="translate(270,460)" data-path="/feedback">
            <rect width="150" height="58" rx="10"></rect>
            <text x="75" y="26" text-anchor="middle" font-size="14" font-weight="600">问题反馈</text>
            <text x="75" y="45" text-anchor="middle" class="sub">反馈面板</text>
          </g>
          <g class="node gray" transform="translate(900,520)" data-path="/fault/cases">
            <rect width="150" height="46" rx="10"></rect>
            <text x="75" y="27" text-anchor="middle" font-size="13">识别案例库</text>
          </g>
          <g class="node gray" transform="translate(1110,520)" data-path="/warning/rules">
            <rect width="150" height="46" rx="10"></rect>
            <text x="75" y="27" text-anchor="middle" font-size="13">预警配置</text>
          </g>
          <g class="node blue" transform="translate(1320,460)" data-path="/firmware">
            <rect width="150" height="58" rx="10"></rect>
            <text x="75" y="26" text-anchor="middle" font-size="14" font-weight="600">固件管理</text>
            <text x="75" y="45" text-anchor="middle" class="sub">固件包 / 升级</text>
          </g>
          <g class="node blue" transform="translate(1320,516)" data-path="/log">
            <rect width="150" height="50" rx="10"></rect>
            <text x="75" y="30" text-anchor="middle" font-size="14">系统日志</text>
          </g>
          <g class="node blue" transform="translate(1110,516)" data-path="/settings">
            <rect width="150" height="50" rx="10"></rect>
            <text x="75" y="30" text-anchor="middle" font-size="14">系统设置</text>
          </g>
          <g class="node gray" transform="translate(900,594)" data-path="/settings/license">
            <rect width="150" height="46" rx="10"></rect>
            <text x="75" y="27" text-anchor="middle" font-size="13">授权与试用</text>
          </g>

          <g class="node orange" transform="translate(60,680)" data-path="/video">
            <rect width="150" height="56" rx="10"></rect>
            <text x="75" y="26" text-anchor="middle" font-size="14" font-weight="600">视频监控</text>
            <text x="75" y="44" text-anchor="middle" class="sub">实时视频</text>
          </g>
          <g class="node orange" transform="translate(270,680)" data-path="/monitor">
            <rect width="150" height="56" rx="10"></rect>
            <text x="75" y="26" text-anchor="middle" font-size="14" font-weight="600">监控大屏</text>
            <text x="75" y="44" text-anchor="middle" class="sub">电视墙</text>
          </g>
          <g class="node orange" transform="translate(480,680)" data-path="/patrol">
            <rect width="150" height="56" rx="10"></rect>
            <text x="75" y="26" text-anchor="middle" font-size="14" font-weight="600">自动巡检</text>
            <text x="75" y="44" text-anchor="middle" class="sub">巡检计划</text>
          </g>
          <g class="node orange" transform="translate(690,680)" data-path="/ai">
            <rect width="150" height="56" rx="10"></rect>
            <text x="75" y="26" text-anchor="middle" font-size="14" font-weight="600">AI 分析</text>
            <text x="75" y="44" text-anchor="middle" class="sub">预测 / 诊断 / 生命周期</text>
          </g>
          <g class="node green" transform="translate(960,720)" data-path="/inventory/material">
            <rect width="150" height="50" rx="10"></rect>
            <text x="75" y="29" text-anchor="middle" font-size="13">物料库存</text>
          </g>
          <g class="node green" transform="translate(1170,720)" data-path="/inventory/purchase">
            <rect width="150" height="50" rx="10"></rect>
            <text x="75" y="29" text-anchor="middle" font-size="13">采购管理</text>
          </g>
          <g class="node gray" transform="translate(1240,660)" data-path="/ai/config">
            <rect width="150" height="50" rx="10"></rect>
            <text x="75" y="29" text-anchor="middle" font-size="13">额度设置</text>
          </g>
        </svg>
      </div>
      <div class="legend">
        <span><i class="sw core"></i>核心流程</span>
        <span><i class="sw blue"></i>核心基础模块</span>
        <span><i class="sw orange"></i>可选模块</span>
        <span><i class="sw green"></i>库存与成本分组</span>
        <span><i class="sw gray"></i>系统入口/通用</span>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()

function onSvgClick(e: MouseEvent) {
  const el = (e.target as Element).closest('[data-path]') as HTMLElement | null
  if (el?.dataset.path) {
    router.push(el.dataset.path)
  }
}
</script>

<style scoped>
.dag {
  overflow: auto;
  cursor: pointer;
}
.dag svg {
  width: 100%;
  min-width: 900px;
  display: block;
}
.legend {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
  font-size: 12px;
  color: #6b7280;
  margin-top: 8px;
  padding: 0 8px 8px;
}
.legend span { display: inline-flex; align-items: center; gap: 6px; }
.sw { width: 14px; height: 14px; border-radius: 4px; display: inline-block; border: 1px solid #cbd5e1; }
.sw.core { background: #fef2f2; border-color: #dc2626; }
.sw.blue { background: #eaf2ff; border-color: #3b82f6; }
.sw.orange { background: #fff7ed; border-color: #f59e0b; }
.sw.green { background: #f0fdf4; border-color: #16a34a; }
.sw.gray { background: #eef0f4; border-color: #cbd5e1; }

.zone { fill: none; stroke: #c7d0dd; stroke-width: 1; stroke-dasharray: 7 6; }
.zone-label { fill: #6b7280; font-size: 13px; font-weight: 600; }
.node { stroke-width: 1.4; cursor: pointer; }
.node text { fill: #1f2937; }
.node:hover rect { filter: drop-shadow(0 2px 6px rgba(15, 23, 42, .25)); }
.node.core rect { fill: #fef2f2; stroke: #dc2626; }
.node.core .core-title { fill: #991b1b; font-weight: 700; }
.node.blue rect { fill: #eaf2ff; stroke: #3b82f6; }
.node.orange rect { fill: #fff7ed; stroke: #f59e0b; }
.node.green rect { fill: #f0fdf4; stroke: #16a34a; }
.node.gray rect { fill: #eef0f4; stroke: #cbd5e1; }
.sub { font-size: 11px; fill: #6b7280; }
.edge { stroke: #cbd5e1; stroke-width: 1.6; fill: none; }
.edge.red { stroke: #dc2626; stroke-width: 3.2; }
.edge.dash { stroke-dasharray: 6 6; }
</style>
