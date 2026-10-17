<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { HideOverlay, Lookup, LookupPlayer, OverlayStatus, RefreshRoster, SetOverlayPinned, Snapshot, ToggleOverlay } from '../bindings/leaguepingo/appservice'
import type { OverlayState } from '../bindings/leaguepingo/models'
import type { Match, Participant, Snapshot as SnapshotModel } from '../bindings/leaguepingo/internal/core/models'

const isOverlay = new URLSearchParams(location.search).get('view') === 'overlay'
const current = ref<SnapshotModel | null>(null)
const query = ref('')
const busy = ref(false)
const error = ref('')
const result = ref<Participant | null>(null)
const selectedKey = ref('')
let stopEvents: (() => void) | undefined
let stopOverlayEvents: (() => void) | undefined
const overlayState = ref<OverlayState | null>(null)
const overlayBusy = ref(false)

function acceptOverlay(state: OverlayState) {
  if (!overlayState.value || state.revision >= overlayState.value.revision) overlayState.value = state
}
async function controlOverlay(action: () => Promise<OverlayState>) {
  if (overlayBusy.value) return
  overlayBusy.value = true
  try { acceptOverlay(await action()) }
  catch (reason) { error.value = displayError(reason) }
  finally { overlayBusy.value = false }
}
function hideOverlay() { return controlOverlay(() => HideOverlay()) }
async function refreshRoster() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try { current.value = await RefreshRoster() }
  catch (reason) { error.value = displayError(reason) }
  finally { busy.value = false }
}
async function lookupSelected() {
  if (!selected.value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    await LookupPlayer(selected.value.key)
    current.value = await Snapshot()
  } catch (reason) { error.value = displayError(reason) }
  finally { busy.value = false }
}
function onKeyDown(event: KeyboardEvent) {
  if (isOverlay && event.key === 'Escape') {
    event.preventDefault()
    void hideOverlay()
  }
}

const players = computed(() => current.value?.participants ?? [])
const allies = computed(() => players.value.filter(player => player.team === 'ally'))
const enemies = computed(() => players.value.filter(player => player.team === 'enemy'))
const selected = computed(() => players.value.find(player => player.key === selectedKey.value) ?? null)
const stageLabel = computed(() => {
  if (current.value?.frozen && !current.value.connected) return '连接中断 · 保留缓存'
  if (current.value?.frozen) return '正式对局 · 自动预取暂停'
  if (!current.value?.connected && current.value?.message) return '客户端未连接 · 查看原因'
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
  const message = (reason instanceof Error ? reason.message : String(reason)).replace(/^RuntimeError:\s*/, '')
  if (message.includes('<!DOCTYPE') || message.includes('<html')) return '桌面服务不可用，请从 LeaguePingo 应用打开界面。'
  return message.length > 220 ? `${message.slice(0, 220)}…` : message
}
async function search() {
  if (!query.value.trim() || busy.value) return
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
  window.addEventListener('keydown', onKeyDown)
  stopOverlayEvents = Events.On('leaguepingo:overlay', event => acceptOverlay(event.data as OverlayState))
  stopEvents = Events.On('leaguepingo:snapshot', event => {
    current.value = event.data as SnapshotModel
    if (selectedKey.value && !players.value.some(player => player.key === selectedKey.value)) selectedKey.value = ''
  })
  const results = await Promise.allSettled([Snapshot(), OverlayStatus()])
  if (results[0].status === 'fulfilled') current.value = results[0].value
  else error.value = displayError(results[0].reason)
  if (results[1].status === 'fulfilled') acceptOverlay(results[1].value)
  else error.value = displayError(results[1].reason)
})
onUnmounted(() => {
  stopEvents?.()
  stopOverlayEvents?.()
  window.removeEventListener('keydown', onKeyDown)
})
</script>

<template>
  <main v-if="isOverlay" class="overlay-shell">
    <header class="overlay-head" title="按住标题栏拖动窗口">
      <div class="overlay-title"><span class="brand-dot"></span><strong>LeaguePingo</strong><small>对局快览 · 可拖动</small></div>
      <div class="overlay-controls">
        <button type="button" :disabled="overlayBusy || !overlayState" :aria-pressed="overlayState?.pinned" @click="controlOverlay(() => SetOverlayPinned(!overlayState?.pinned))">{{ overlayState?.pinned ? '取消置顶' : '置顶' }}</button>
        <button type="button" class="overlay-close" aria-label="关闭对局快览" title="隐藏快览（不退出助手）" :disabled="overlayBusy" @click="hideOverlay">×</button>
      </div>
    </header>
    <div class="overlay-content">
    <p class="overlay-status">{{ stageLabel }} <span v-if="current?.gameId">· #{{ current.gameId }}</span></p>
    <p v-if="overlayState?.warning" class="notice error">{{ overlayState.warning }}</p>
    <p v-if="error" class="notice error">{{ error }}</p>
    <p v-if="current?.message" class="overlay-status">{{ current.message }}</p>
    <p v-if="!players.length" class="overlay-empty">暂无可展示的对局资料。选人阶段会自动准备已知身份玩家的战绩。</p>
    <section v-else class="overlay-groups">
      <div v-for="group in [{ title: '我方', list: allies }, { title: '对方', list: enemies }]" :key="group.title"><h2>{{ group.title }}</h2>
        <div v-for="player in group.list" :key="player.key" class="overlay-player"><span class="player-name">{{ player.displayName }}</span><span class="player-summary">{{ resultLabel(player) }}<template v-if="player.historyState === 'ready'"> · 胜率 {{ Math.round(player.stats.winRate) }}%</template></span></div>
      </div>
    </section>
    </div>
    <footer class="overlay-footer">
      <span>拖动标题栏移动 · × 关闭 · 可切换置顶<br>Ctrl+Shift+P 显隐 · Esc（窗口获焦时）隐藏<br>显示已缓存资料；查询与刷新请到主窗口</span>
    </footer>
  </main>

  <main v-else class="app-shell">
    <aside class="sidebar">
      <div class="brand"><span class="brand-dot"></span><div><strong>LeaguePingo</strong><small>轻量战绩助手</small></div></div>
      <div class="side-label">工作区</div><a class="side-link selected" href="#top"><span>◈</span> 概览</a><a class="side-link" href="#lookup"><span>◎</span> 玩家查询</a><a class="side-link" href="#match"><span>▤</span> 当前对局</a>
      <div class="sidebar-spacer"></div><div class="sidebar-note"><strong>低干扰模式</strong><p>局内暂停自动战绩预取。需要新资料时可主动查询，一次处理一位玩家。</p></div>
      <div class="sidebar-bottom"><span class="status-dot" :class="{ active: current?.connected }"></span>{{ stageLabel }}</div>
    </aside>
    <div id="top" class="workspace">
      <header class="topbar"><div><span class="eyebrow">LEAGUEPINGO / DASHBOARD</span><h1>战绩概览</h1></div><span class="top-stage">{{ stageLabel }}</span></header>
      <section class="intro-card"><div><span class="eyebrow">MATCH INTELLIGENCE</span><h2>需要的信息，<br><em>在开局前准备好。</em></h2><p>选人阶段准备可识别玩家的战绩，开局后更新本地名单。局内暂停自动战绩预取，仍可输入完整 ID 或选择玩家主动查询。</p></div><div class="intro-actions"><button class="ghost-button" @click="toggleOverlay">显示／隐藏快览 <span>↗</span></button><small>全局快捷键 Ctrl + Shift + P</small><div class="quick-controls"><button type="button" :disabled="overlayBusy" @click="hideOverlay">隐藏快览</button></div></div></section>
      <p v-if="overlayState?.warning" class="notice error" role="status">{{ overlayState.warning }}</p>
      <div class="content-grid">
        <section id="lookup" class="panel search-panel"><div class="panel-heading"><div><span class="eyebrow">01 / MANUAL LOOKUP</span><h3>查询玩家</h3></div><span class="panel-tag">腾讯国服优先</span></div>
          <form class="search-form" @submit.prevent="search"><label for="riot-id">玩家完整 ID</label><div class="search-line"><input id="riot-id" v-model="query" placeholder="玩家名#标签" autocomplete="off" /><button type="submit" :disabled="busy">{{ busy ? '处理中…' : '查询战绩' }}</button></div></form>
          <p class="hint">局内外均可手动查询；优先使用本局缓存，缺少资料时只查询所选玩家。</p><p v-if="error" class="notice error">{{ error }}</p>
          <div v-if="result" class="result-card"><div class="result-top"><div><small>查询结果</small><h4>{{ result.displayName }}</h4></div><span class="result-state">{{ resultLabel(result) }}</span></div><div class="stats-row"><div><strong>{{ result.stats.games }}</strong><small>样本场次</small></div><div><strong>{{ Math.round(result.stats.winRate) }}%</strong><small>胜率</small></div><div><strong>{{ result.stats.kda.toFixed(2) }}</strong><small>KDA</small></div></div><div class="match-list"><div v-for="match in result.history ?? []" :key="match.gameId" class="match"><span :class="match.win ? 'win' : 'loss'">{{ match.win ? '胜利' : '失败' }}</span><span>英雄 #{{ match.championId }}</span><strong>{{ kda(match) }}</strong><small>{{ formatDate(match.playedAt) }}</small></div></div></div>
        </section>
        <section id="match" class="panel match-panel"><div class="panel-heading"><div><span class="eyebrow">02 / LIVE ROSTER</span><h3>当前对局</h3></div><span class="panel-tag" :class="{ frozen: current?.frozen }">{{ current?.frozen ? '手动查询可用' : '自动准备' }}</span></div><p class="panel-copy">{{ current?.message || (players.length ? '点击玩家查看近期战绩，缺少资料时可手动查询。' : '等待选人或进入对局，客户端可提供的名单会显示在这里。') }}</p>
          <button type="button" class="ghost-button" :disabled="busy || !current?.connected" @click="refreshRoster">刷新对局名单</button>
          <div v-if="players.length" class="roster"><div v-for="group in [{ title: '我方', list: allies }, { title: '对方', list: enemies }]" :key="group.title" class="team"><h4>{{ group.title }}<span>{{ group.list.length }} 位玩家</span></h4><button v-for="player in group.list" :key="player.key" class="player-row" :class="{ selected: selectedKey === player.key }" @click="selectedKey = player.key"><span class="avatar">{{ player.identityState === 'unavailable' ? '?' : '◈' }}</span><span class="player-detail"><strong>{{ player.displayName }}</strong><small>{{ resultLabel(player) }}</small></span><span class="chevron">›</span></button></div></div>
          <div v-else class="empty-roster"><span>◎</span><strong>尚无对局名单</strong><p>组队房间、排队和接受确认阶段通常无法取得双方十人的身份。</p></div>
        </section>
      </div>
      <section v-if="selected" class="panel detail-panel"><div class="panel-heading"><div><span class="eyebrow">PLAYER DETAIL</span><h3>{{ selected.displayName }}</h3></div><span class="panel-tag">{{ resultLabel(selected) }}</span></div><p class="hint">身份：{{ selected.identityState === 'known' ? '已获取' : '不可获取' }} · 生涯隐私：{{ selected.privacyState === 'unknown' ? '未核验' : selected.privacyState }}。查询失败不代表玩家隐藏战绩。</p><button v-if="selected.identityState === 'known' && selected.historyState !== 'ready' && selected.historyState !== 'empty'" type="button" class="ghost-button" :disabled="busy" @click="lookupSelected">手动查询此玩家</button><p v-if="selected.identityState === 'unavailable'" class="hint">客户端尚未提供有效身份，请刷新名单或输入完整 ID 查询。</p><div v-if="selected.historyState === 'ready'" class="stats-row"><div><strong>{{ selected.stats.games }}</strong><small>样本场次</small></div><div><strong>{{ Math.round(selected.stats.winRate) }}%</strong><small>胜率</small></div><div><strong>{{ selected.stats.kda.toFixed(2) }}</strong><small>KDA</small></div></div><p v-if="selected.error" class="notice error">{{ selected.error }}</p><div class="match-list"><div v-for="match in selected.history ?? []" :key="match.gameId" class="match"><span :class="match.win ? 'win' : 'loss'">{{ match.win ? '胜利' : '失败' }}</span><span>英雄 #{{ match.championId }}</span><strong>{{ kda(match) }}</strong><small>{{ formatDate(match.playedAt) }}</small></div></div></section>
      <footer class="workspace-footer">LeaguePingo 原型版 · 不注入游戏进程 · 不修改游戏流量 · 数据仅来自本地客户端</footer>
    </div>
  </main>
</template>
