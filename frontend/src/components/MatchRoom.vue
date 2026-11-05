<script setup lang="ts">
import { uiText } from "../uiText";
import { computed, reactive, ref, watch } from 'vue';
import * as Service from '../../bindings/leaguepingo/appservice';
import type { Participant, Snapshot } from '../../bindings/leaguepingo/internal/core/models';
import { errorMessage, queueName, type PlayerViewState } from '../presentation';
import { hasCurrentMatch } from '../matchRoom';
import AppIcon from './AppIcon.vue';
import GuidePanel from './GuidePanel.vue';
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
  if (!props.snapshot?.connected) return uiText.matchRoom.disconnected;
  if (party.value) return uiText.matchRoom.party;
  if (props.snapshot?.phase === 'ChampSelect') return uiText.matchRoom.selection;
  if (props.snapshot?.phase === 'GameStart') return uiText.matchRoom.loading;
  if (props.snapshot?.phase === 'Reconnect') return uiText.matchRoom.reconnecting;
  return uiText.matchRoom.inGame;
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
  <section class="match-room" :aria-label="uiText.common.matchRoom">
    <nav class="match-room-nav review-scope-switch" :aria-label="uiText.matchRoom.content">
      <button class="secondary" :aria-pressed="scope === 'current'" @click="scope = 'current'">{{ uiText.matchRoom.current }}</button>
      <button class="secondary" :aria-pressed="scope === 'review'" @click="scope = 'review'">{{ uiText.matchRoom.review }}</button>
    </nav>
    <section v-show="scope === 'current'" class="current-match" :aria-label="uiText.matchRoom.currentData">
      <header class="review-heading"><div><h1>{{ uiText.matchRoom.current }}</h1><p>{{ currentAvailable ? subtitle : uiText.matchRoom.waitingDescription }}<span v-if="currentAvailable && snapshot?.queueId"> · {{ queueName(snapshot.queueId) }}</span></p></div><button v-if="currentAvailable && snapshot?.connected && snapshot.phase !== 'Reconnect'" class="secondary" @click="$emit('companion')">{{ inGame ? uiText.matchRoom.showPreview : uiText.clientSidebar.expand }}</button></header>
      <template v-if="currentAvailable">
        <div class="review-section-title"><h2>{{ party ? uiText.common.party : uiText.common.pregame }}</h2><QueueSwitch :value="queue" :busy="changingQueue || !snapshot?.connected" @change="changeQueue"/></div>
        <p v-if="error || snapshot?.rosterError" class="quality-warning" role="status">{{ error || snapshot?.rosterError }}</p>
        <GuidePanel :snapshot="snapshot"/>
        <p v-if="inGame && snapshot?.groupCoverage" class="review-note">{{ uiText.matchRoom.groupCoverage(snapshot.groupCoverage, players.length) }}</p>
        <RosterProgress v-if="players.length" :players="players" :queue="queue"/>
        <MatchRoster v-if="players.length" :players="players" :selected-key="selectedKey" :party="party" :allies-only="!inGame" :frozen="snapshot?.frozen" :queue="queue" @select="selectedKey = $event.key"/>
        <div v-else class="review-empty"><AppIcon name="match" :size="36"/><h2>{{ uiText.matchRoom.waitingRoster }}</h2><p>{{ inGame ? uiText.matchRoom.identityHint : uiText.matchRoom.partyHint }}</p></div>
      </template>
      <div v-else class="review-empty"><AppIcon name="match" :size="36"/><h2>{{ uiText.matchRoom.noMatch }}</h2><p>{{ uiText.matchRoom.reviewHint }}</p></div>
      <PlayerDrawer v-if="selected" :player-key="selected.key" modal :active="active && scope === 'current'" @close="selectedKey = ''">
        <div class="drawer-toolbar"><button class="text-button" :disabled="!snapshot?.connected || !selected.displayName.includes('#')" @click="$emit('query', selected)">{{ uiText.matchRoom.latestHistory }}</button><span class="drawer-player-switch"><button class="text-button" :aria-label="uiText.matchRoom.previousPlayer" @click="adjacent(-1)">{{ uiText.matchRoom.previous }}</button><button class="text-button" :aria-label="uiText.matchRoom.nextPlayer" @click="adjacent(1)">{{ uiText.matchRoom.next }}</button></span><button class="icon-button" :aria-label="uiText.matchRoom.closePlayer" @click="selectedKey = ''"><AppIcon name="close"/></button></div>
        <p class="review-note">{{ uiText.matchRoom.capturedHint }}</p>
        <PlayerView :player="selected" :state="stateFor(selected.key)" :scout-queue="queue" :connected="false" :active="active && scope === 'current'" compact prepared/>
      </PlayerDrawer>
    </section>
    <ReviewRoom v-show="scope === 'review'" :review="snapshot?.review ?? null" :connected="snapshot?.connected ?? false" :active="active && scope === 'review'" :in-game="inGame" @snapshot="$emit('snapshot', $event)" @query="$emit('query', $event)" @modal="$emit('modal', $event)"/>
  </section>
</template>
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.match-room):is(.match-room) {height:100%;min-height:0;display:flex;flex-direction:column;}
:where(.match-room-nav):is(.match-room-nav) {flex:none;padding:12px 24px 0;}
:where(.current-match):is(.match-room>.current-match) {flex:1;height:auto;min-height:0;}
:where(.current-match):is(.current-match) {overflow:auto;padding:18px 24px 22px;display:flex;flex-direction:column;gap:14px;}
@media(max-width:1000px){:where(.match-room-nav):is(.match-room-nav){padding:12px 16px 0;}}
@media(max-width:1000px){:where(.current-match):is(.current-match){padding:16px;}}
</style>
