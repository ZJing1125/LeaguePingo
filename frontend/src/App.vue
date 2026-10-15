<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { Lookup, Snapshot, ToggleOverlay } from '../bindings/leaguepingo/appservice'
import type { Match, Participant, Snapshot as SnapshotModel } from '../bindings/leaguepingo/internal/core/models'

const isOverlay = new URLSearchParams(location.search).get('view') === 'overlay'
const current = ref<SnapshotModel | null>(null)
const query = ref('')
const busy = ref(false)
const error = ref('')
const result = ref<Participant | null>(null)
const selectedKey = ref('')
let stopEvents: (() => void) | undefined

const players = computed(() => current.value?.participants ?? [])
const allies = computed(() => players.value.filter(player => player.team === 'ally'))
const enemies = computed(() => players.value.filter(player => player.team === 'enemy'))
const selected = computed(() => players.value.find(player => player.key === selectedKey.value) ?? null)
const stageLabel = computed(() => {
  if (current.value?.frozen) return '正式对局 · 已冻结'
  if (!current.value?.connected) return '等待客户端'
  if (current.value.stage === 'preparing') return '选人阶段 · 准备资料'
  return '客户端已连接'
})

function resultLabel(player: Participant): string {
  if (player.identityState === 'unavailable') return '身份不可获取'
  if (player.historyState === 'loading') return '查询中'
  if (player.historyState === 'ready') return `近 ${player.stats.games} 场`
  if (player.historyState === 'empty') return '暂无可核验战绩'
  if (player.historyState === 'failed') return '查询失败'
  return '等待查询'
}
function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '日期未知' : date.toLocaleDateString('zh-CN', { month: 'numeric', day: 'numeric' })
}
function kda(match: Match): string { return `${match.kills} / ${match.deaths} / ${match.assists}` }
function displayError(reason: unknown): string {
  const message = String(reason)
  if (message.includes('<!DOCTYPE') || message.includes('<html')) return '桌面服务不可用，请从 LeaguePingo 应用打开界面。'
  return message.length > 220 ? `${message.slice(0, 220)}…` : message
}
async function search() {
  if (!query.value.trim() || busy.value || current.value?.frozen) return
  busy.value = true
  error.value = ''
  result.value = null
  try { result.value = await Lookup(query.value.trim()) }
  catch (reason) { error.value = displayError(reason) }
  finally { busy.value = false }
}
async function toggleOverlay() {
  try { await ToggleOverlay() } catch (reason) { error.value = displayError(reason) }
}
onMounted(async () => {
  stopEvents = Events.On('leaguepingo:snapshot', event => {
    current.value = event.data as SnapshotModel
    if (selectedKey.value && !players.value.some(player => player.key === selectedKey.value)) selectedKey.value = ''
  })
  try { current.value = await Snapshot() } catch (reason) { error.value = displayError(reason) }
})
onUnmounted(() => stopEvents?.())
</script>

<template>
  <main v-if="isOverlay" class="overlay-shell">
    <header class="overlay-head"><div><span class="brand-dot"></span><strong>LeaguePingo</strong><small>对局快览</small></div><span class="status-dot" :class="{ active: current?.connected }"></span></header>
    <p class="overlay-status">{{ stageLabel }} <span v-if="current?.gameId">· #{{ current.gameId }}</span></p>
    <p v-if="!players.length" class="overlay-empty">暂无可展示的对局资料。选人阶段会自动准备已知身份玩家的战绩。</p>
    <section v-else class="overlay-groups">
      <div v-for="group in [{ title: '我方', list: allies }, { title: '对方', list: enemies }]" :key="group.title"><h2>{{ group.title }}</h2>
        <div v-for="player in group.list" :key="player.key" class="overlay-player"><span class="player-name">{{ player.displayName }}</span><span class="player-summary">{{ resultLabel(player) }}<template v-if="player.historyState === 'ready'"> · 胜率 {{ Math.round(player.stats.winRate) }}%</template></span></div>
      </div>
    </section>
    <footer>仅读取已缓存数据 · Ctrl + Shift + P 隐藏</footer>
  </main>

  <main v-else class="app-shell">
    <aside class="sidebar">
      <div class="brand"><span class="brand-dot"></span><div><strong>LeaguePingo</strong><small>轻量战绩助手</small></div></div>
      <div class="side-label">工作区</div><a class="side-link selected" href="#top"><span>◈</span> 概览</a><a class="side-link" href="#lookup"><span>◎</span> 对局外查询</a><a class="side-link" href="#match"><span>▤</span> 当前对局</a>
      <div class="sidebar-spacer"></div><div class="sidebar-note"><strong>低干扰模式</strong><p>正式对局开始后停止自动战绩查询，只保留已准备的数据。</p></div>
      <div class="sidebar-bottom"><span class="status-dot" :class="{ active: current?.connected }"></span>{{ stageLabel }}</div>
    </aside>
    <div id="top" class="workspace">
      <header class="topbar"><div><span class="eyebrow">LEAGUEPINGO / DASHBOARD</span><h1>战绩概览</h1></div><span class="top-stage">{{ stageLabel }}</span></header>
      <section class="intro-card"><div><span class="eyebrow">MATCH INTELLIGENCE</span><h2>需要的信息，<br><em>在开局前准备好。</em></h2><p>输入完整 Riot ID 查询战绩；选人阶段自动收集可识别的对局玩家。正式游戏中按需查看缓存，不发起新的历史战绩请求。</p></div><div class="intro-actions"><button class="ghost-button" @click="toggleOverlay">显示／隐藏快览 <span>↗</span></button><small>全局快捷键 Ctrl + Shift + P</small></div></section>
      <div class="content-grid">
        <section id="lookup" class="panel search-panel"><div class="panel-heading"><div><span class="eyebrow">01 / MANUAL LOOKUP</span><h3>查询玩家</h3></div><span class="panel-tag">腾讯国服优先</span></div>
          <form class="search-form" @submit.prevent="search"><label for="riot-id">玩家完整 ID</label><div class="search-line"><input id="riot-id" v-model="query" placeholder="玩家名#标签" autocomplete="off" :disabled="current?.frozen" /><button type="submit" :disabled="busy || current?.frozen">{{ busy ? '查询中…' : '查询战绩' }}</button></div></form>
          <p class="hint">使用当前登录的 League 客户端本地接口。正式对局中暂停新查询。</p><p v-if="error" class="notice error">{{ error }}</p>
          <div v-if="result" class="result-card"><div class="result-top"><div><small>查询结果</small><h4>{{ result.displayName }}</h4></div><span class="result-state">{{ resultLabel(result) }}</span></div><div class="stats-row"><div><strong>{{ result.stats.games }}</strong><small>样本场次</small></div><div><strong>{{ Math.round(result.stats.winRate) }}%</strong><small>胜率</small></div><div><strong>{{ result.stats.kda.toFixed(2) }}</strong><small>KDA</small></div></div><div class="match-list"><div v-for="match in result.history ?? []" :key="match.gameId" class="match"><span :class="match.win ? 'win' : 'loss'">{{ match.win ? '胜利' : '失败' }}</span><span>英雄 #{{ match.championId }}</span><strong>{{ kda(match) }}</strong><small>{{ formatDate(match.playedAt) }}</small></div></div></div>
        </section>
        <section id="match" class="panel match-panel"><div class="panel-heading"><div><span class="eyebrow">02 / LIVE ROSTER</span><h3>当前对局</h3></div><span class="panel-tag" :class="{ frozen: current?.frozen }">{{ current?.frozen ? '已冻结' : '自动准备' }}</span></div><p class="panel-copy">{{ current?.message || (players.length ? '点击玩家查看已核验的近期战绩。' : '等待进入选人阶段；可识别的玩家会自动出现在这里。') }}</p>
          <div v-if="players.length" class="roster"><div v-for="group in [{ title: '我方', list: allies }, { title: '对方', list: enemies }]" :key="group.title" class="team"><h4>{{ group.title }}<span>{{ group.list.length }} 位玩家</span></h4><button v-for="player in group.list" :key="player.key" class="player-row" :class="{ selected: selectedKey === player.key }" @click="selectedKey = player.key"><span class="avatar">{{ player.identityState === 'unavailable' ? '?' : '◈' }}</span><span class="player-detail"><strong>{{ player.displayName }}</strong><small>{{ resultLabel(player) }}</small></span><span class="chevron">›</span></button></div></div>
          <div v-else class="empty-roster"><span>◎</span><strong>尚无对局名单</strong><p>组队房间、排队和接受确认阶段通常无法取得双方十人的身份。</p></div>
        </section>
      </div>
      <section v-if="selected" class="panel detail-panel"><div class="panel-heading"><div><span class="eyebrow">PLAYER DETAIL</span><h3>{{ selected.displayName }}</h3></div><span class="panel-tag">{{ resultLabel(selected) }}</span></div><p class="hint">身份：{{ selected.identityState === 'known' ? '已获取' : '不可获取' }} · 生涯隐私：{{ selected.privacyState === 'unknown' ? '未核验' : selected.privacyState }}。查询失败不代表玩家隐藏战绩。</p><div v-if="selected.historyState === 'ready'" class="stats-row"><div><strong>{{ selected.stats.games }}</strong><small>样本场次</small></div><div><strong>{{ Math.round(selected.stats.winRate) }}%</strong><small>胜率</small></div><div><strong>{{ selected.stats.kda.toFixed(2) }}</strong><small>KDA</small></div></div><p v-if="selected.error" class="notice error">{{ selected.error }}</p><div class="match-list"><div v-for="match in selected.history ?? []" :key="match.gameId" class="match"><span :class="match.win ? 'win' : 'loss'">{{ match.win ? '胜利' : '失败' }}</span><span>英雄 #{{ match.championId }}</span><strong>{{ kda(match) }}</strong><small>{{ formatDate(match.playedAt) }}</small></div></div></section>
      <footer class="workspace-footer">LeaguePingo 原型版 · 不注入游戏进程 · 不修改游戏流量 · 数据仅来自本地客户端</footer>
    </div>
  </main>
</template>
