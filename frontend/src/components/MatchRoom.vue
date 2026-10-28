<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import * as Service from '../../bindings/leaguepingo/appservice';
import type { Participant, Snapshot } from '../../bindings/leaguepingo/internal/core/models';
import { errorMessage, queueName, type PlayerViewState } from '../presentation';
import { hasCurrentMatch } from '../matchRoom';
import AppIcon from './AppIcon.vue';
import MatchRoster from './MatchRoster.vue';
import PlayerDrawer from './PlayerDrawer.vue';
import PlayerView from './PlayerView.vue';
import QueueSwitch from './QueueSwitch.vue';
import ReviewRoom from './ReviewRoom.vue';
import RosterProgress from './RosterProgress.vue';

const props = defineProps<{ snapshot: Snapshot | null; active: boolean }>();
const emit = defineEmits<{ query: [player: Participant]; snapshot: [value: Snapshot]; modal: [open: boolean]; companion: [] }>();
const scope = ref<'current' | 'review'>('current');
const selectedKey = ref(''), error = ref(''), changingQueue = ref(false);
const states = reactive<Record<string, PlayerViewState>>({});
const currentAvailable = computed(() => hasCurrentMatch(props.snapshot));
const inGame = computed(() => ['GameStart', 'InProgress', 'Reconnect'].includes(props.snapshot?.phase || ''));
const party = computed(() => ['Lobby', 'Matchmaking', 'ReadyCheck'].includes(props.snapshot?.phase || ''));
const players = computed(() => (props.snapshot?.participants ?? []).filter(p => inGame.value || p.team === 'ally'));
const queue = computed(() => props.snapshot?.scoutQueue || 420);
const currentKey = computed(() => {
  const s = props.snapshot;
  const owner = `${s?.connectionId}:${s?.selfKey || ''}`;
  if (!currentAvailable.value) return `${owner}:idle`;
  if (party.value) return `${owner}:party`;
  return `${owner}:${inGame.value ? 'game' : 'selection'}:${s?.gameId || 0}`;
});
const reviewKey = computed(() => `${props.snapshot?.review?.connectionId}:${props.snapshot?.review?.gameId}`);
const selected = computed(() => players.value.find(p => p.key === selectedKey.value) ?? null);
const subtitle = computed(() => {
  if (!props.snapshot?.connected) return '连接中断 · 保留已准备资料';
  if (party.value) return '房间成员 · 进入选人后展示英雄表现';
  if (props.snapshot?.phase === 'ChampSelect') return '选人阶段 · 我方玩家资料';
  if (props.snapshot?.phase === 'GameStart') return '对局载入 · 已返回玩家资料';
  if (props.snapshot?.phase === 'Reconnect') return '游戏重连中 · 保留本局资料';
  return '对局进行中 · 查看双方赛前表现';
});
watch([currentKey, reviewKey], ([current, review], previous) => {
  if (!previous || current !== previous[0] || !currentAvailable.value && review !== previous[1]) {
    scope.value = currentAvailable.value || !props.snapshot?.review ? 'current' : 'review';
  }
}, { immediate: true });
watch(() => props.active, active => {
  if (active) scope.value = currentAvailable.value || !props.snapshot?.review ? 'current' : 'review';
});
watch(currentKey, () => {
  selectedKey.value = ''; error.value = '';
  for (const key of Object.keys(states)) delete states[key];
});
watch(() => [props.active, scope.value], () => { selectedKey.value = ''; });
watch(() => !!selected.value, open => emit('modal', open));
function stateFor(key: string) { return states[key] ?? (states[key] = { queue: 'all', expanded: [] }); }
function adjacent(direction: number) {
  const index = players.value.findIndex(p => p.key === selectedKey.value);
  if (players.value.length) selectedKey.value = players.value[(index + direction + players.value.length) % players.value.length]!.key;
}
async function changeQueue(value: number) {
  if (changingQueue.value || !props.snapshot?.connected || value === queue.value) return;
  const connection = props.snapshot.connectionId;
  changingQueue.value = true; error.value = '';
  try { emit('snapshot', await Service.SetScoutQueue(value)); }
  catch (reason) { if (connection === props.snapshot?.connectionId) error.value = errorMessage(reason); }
  finally { changingQueue.value = false; }
}
</script>

<template>
  <section class="match-room" aria-label="对局室">
    <nav class="match-room-nav review-scope-switch" aria-label="对局室内容">
      <button class="secondary" :aria-pressed="scope === 'current'" @click="scope = 'current'">当前对局</button>
      <button class="secondary" :aria-pressed="scope === 'review'" @click="scope = 'review'">最近复盘</button>
    </nav>
    <section v-show="scope === 'current'" class="current-match" aria-label="当前对局资料">
      <header class="review-heading"><div><h1>当前对局</h1><p>{{ currentAvailable ? subtitle : '进入房间或选人后，在这里查看当前玩家资料。' }}<span v-if="currentAvailable && snapshot?.queueId"> · {{ queueName(snapshot.queueId) }}</span></p></div><button v-if="currentAvailable && snapshot?.connected && snapshot.phase !== 'Reconnect'" class="secondary" @click="$emit('companion')">{{ inGame ? '显示局内快览' : '展开客户端侧栏' }}</button></header>
      <template v-if="currentAvailable">
        <div class="review-section-title"><h2>{{ party ? '房间成员' : '赛前资料' }}</h2><QueueSwitch :value="queue" :busy="changingQueue || !snapshot?.connected" @change="changeQueue"/></div>
        <p v-if="error || snapshot?.rosterError" class="quality-warning" role="status">{{ error || snapshot?.rosterError }}</p>
        <RosterProgress v-if="players.length" :players="players" :queue="queue"/>
        <MatchRoster v-if="players.length" :players="players" :selected-key="selectedKey" :party="party" :allies-only="!inGame" :frozen="snapshot?.frozen" :queue="queue" @select="selectedKey = $event.key"/>
        <div v-else class="review-empty"><AppIcon name="match" :size="36"/><h2>等待当前名单</h2><p>{{ inGame ? '客户端返回玩家身份后会在这里展示。' : '客户端返回我方成员后会在这里展示。' }}</p></div>
      </template>
      <div v-else class="review-empty"><AppIcon name="match" :size="36"/><h2>当前没有进行中的对局</h2><p>已结束的比赛可在“最近复盘”中查看。</p></div>
      <PlayerDrawer v-if="selected" :player-key="selected.key" modal :active="active && scope === 'current'" @close="selectedKey = ''">
        <div class="drawer-toolbar"><button class="text-button" :disabled="!snapshot?.connected || !selected.displayName.includes('#')" @click="$emit('query', selected)">查询最新战绩 ↗</button><span class="drawer-player-switch"><button class="text-button" aria-label="上一位玩家" @click="adjacent(-1)">← 上一位</button><button class="text-button" aria-label="下一位玩家" @click="adjacent(1)">下一位 →</button></span><button class="icon-button" aria-label="关闭玩家详情" @click="selectedKey = ''"><AppIcon name="close"/></button></div>
        <p class="review-note">本局已准备资料 · 查询最新战绩将在玩家战绩页打开</p>
        <PlayerView :player="selected" :state="stateFor(selected.key)" :scout-queue="queue" :connected="false" :active="active && scope === 'current'" compact prepared/>
      </PlayerDrawer>
    </section>
    <ReviewRoom v-show="scope === 'review'" :review="snapshot?.review ?? null" :connected="snapshot?.connected ?? false" :active="active && scope === 'review'" :in-game="inGame" @snapshot="$emit('snapshot', $event)" @query="$emit('query', $event)" @modal="$emit('modal', $event)"/>
  </section>
</template>
