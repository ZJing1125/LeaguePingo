<script setup lang="ts">
import { matchMode } from "../matchMode";
import { computed, getCurrentInstance, nextTick, onBeforeUnmount, ref, watch } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type { Match, MatchDetail, MatchTimeline as Timeline, Participant } from "../../bindings/leaguepingo/internal/core/models";
import { errorMessage, kda, percent } from "../presentation";
import { targetPlayer } from "../analysisData";
import { analysisCache, acquireTimeline } from "../analysisCache";
import { hasFullScoreboard, historyHeroName, historyRoster, matchNumber } from "../historyMatch";
import { isArena } from "../matchStats";
import { uiText } from "../uiText";
import ArenaScoreboard from "./ArenaScoreboard.vue";
import GameIcon from "./GameIcon.vue";
import MatchHistoryRoster from "./MatchHistoryRoster.vue";
import MatchScoreboard from "./MatchScoreboard.vue";
import TeamAnalysis from "./TeamAnalysis.vue";
import MatchTimeline from "./MatchTimeline.vue";
import MatchBuild from "./MatchBuild.vue";

const props = defineProps<{ player: Participant; gameId: number; match?: Match; connected?: boolean; provided?: MatchDetail | null; self?: boolean }>();

const loaded = ref<MatchDetail | null>(null);
const timeline = ref<Timeline | null>(null);
const busy = ref<"detail" | "timeline" | null>(null);
const detailError = ref("");
const timelineError = ref("");
const tab = ref("overview");
const root = ref<HTMLElement | null>(null);
const scopeId = `analysis-${getCurrentInstance()!.uid}`;
const key = computed(() => `${props.player.connectionId}:${props.player.key}:${props.gameId}`);
const detail = computed(() => props.provided ?? loaded.value);
const target = computed(() => targetPlayer(detail.value));
const cachedMatch = computed(() => props.match ?? props.player.history?.find(m => m.gameId === props.gameId));
const match = computed(() => {
  const current = target.value?.summary;
  const cached = cachedMatch.value;
  if (!current || !cached || current.gameId !== cached.gameId || current.championId !== cached.championId) return current ?? cached;
  const shards = current.runeShards?.length === 3 ? [...current.runeShards] : [0, 0, 0];
  if (cached.runeShards?.length === 3) cached.runeShards.forEach((value, i) => { if (!shards[i]) shards[i] = value; });
  return { ...current, queueId: current.queueId || cached.queueId, mapId: current.mapId || cached.mapId, gameVersion: current.gameVersion || cached.gameVersion, runeShards: shards, augmentIds: current.augmentIds ?? cached.augmentIds };
});
const arena = computed(() => !!match.value && isArena(match.value));
const hasScoreboard = computed(() => !!detail.value?.players?.length);
const roster = computed(() => cachedMatch.value ? historyRoster(cachedMatch.value, props.player.connectionId) : []);
const showRoster = computed(() => {
  if (!roster.value.length) return false;
  const current = detail.value;
  if (!current?.players?.length) return true;
  if (hasFullScoreboard(current)) return false;
  const players = current.players;
  return roster.value.some(group => group.peers.some(peer =>
    !players.some(player => player.teamId === peer.teamId && player.participantId === peer.participantId)));
});
const canLoad = computed(() => !!props.connected && !props.player.stale && !props.provided);
const fallbackStatus = computed(() => {
  if (busy.value === "detail") return uiText.matchDetails.loadingScoreboardShort;
  if (detailError.value && canLoad.value) return detailError.value;
  if (!match.value) return uiText.matchDetails.noSavedSummary;
  if (roster.value.length) return uiText.matchDetails.savedRoster;
  return uiText.matchDetails.savedPersonal;
});
const tabs = computed(() => [
  { key: "overview", label: uiText.common.matchOverview },
  { key: "team", label: uiText.common.teamAnalysis },
  { key: "timeline", label: uiText.common.timeline },
  { key: "build", label: matchMode(match.value).augments ? uiText.matchMode.buildAugments : matchMode(match.value).runes ? uiText.common.buildRunes : uiText.matchMode.build },
].filter(item => !arena.value || item.key === "overview"));

let revision = 0;
let autoRequested = "";
let timelineRequested = "";
let releaseTimeline: (() => void) | undefined;
watch(key, () => {
  revision++;
  releaseTimeline?.();
  timelineRequested = "";
  loaded.value = analysisCache.detail(key.value);
  timeline.value = props.provided ? null : analysisCache.timeline(key.value);
  busy.value = null;
  detailError.value = "";
  timelineError.value = "";
  tab.value = "overview";
  autoRequested = "";
}, { immediate: true });
watch(arena, () => { if (arena.value) tab.value = "overview"; });
onBeforeUnmount(() => { revision++; releaseTimeline?.(); });
watch(canLoad, allowed => {
  if (!allowed) {
    revision++;
    releaseTimeline?.();
    busy.value = null;
  }
});

function moveTab(event: KeyboardEvent, index: number) {
  let next = index;
  if (event.key === "ArrowRight") next = (index + 1) % tabs.value.length;
  else if (event.key === "ArrowLeft") next = (index + tabs.value.length - 1) % tabs.value.length;
  else if (event.key === "Home") next = 0;
  else if (event.key === "End") next = tabs.value.length - 1;
  else return;
  event.preventDefault();
  tab.value = tabs.value[next]!.key;
  (event.currentTarget as HTMLElement).parentElement?.querySelectorAll<HTMLButtonElement>("[role=tab]")[next]?.focus();
}

async function load(kind: "detail" | "timeline", scroll = true) {
  if (busy.value || !canLoad.value) return;
  const request = ++revision;
  const requestKey = key.value;
  const error = kind === "detail" ? detailError : timelineError;
  const { player, gameId } = props;
  busy.value = kind;
  error.value = "";
  try {
    if (kind === "timeline") {
      const task = acquireTimeline(requestKey, () => Service.MatchTimeline(player.key, player.connectionId, gameId));
      releaseTimeline = task.release;
      const result = await task.promise;
      if (request !== revision || requestKey !== key.value) return;
      timeline.value = result;
      analysisCache.setTimeline(requestKey, result);
      if (result.detail) {
        loaded.value = result.detail;
        analysisCache.setDetail(requestKey, result.detail);
        detailError.value = "";
      }
    } else {
      const result = await analysisCache.fetchDetail(requestKey, () => Service.MatchDetail(player.key, player.connectionId, gameId));
      if (request !== revision || requestKey !== key.value) return;
      loaded.value = result;
    }
    if (scroll) {
      await nextTick();
      if (request === revision) root.value?.scrollIntoView({ block: "start", behavior: "auto" });
    }
  } catch (reason) {
    if (request === revision) error.value = errorMessage(reason);
  } finally {
    if (request === revision) { busy.value = null; releaseTimeline?.(); releaseTimeline = undefined; }
  }
}

watch([key, canLoad, detail], ([requestKey, allowed, current]) => {
  if (!allowed || current || autoRequested === requestKey) return;
  autoRequested = requestKey;
  void load("detail", false);
}, { immediate: true });
watch([key, tab, canLoad, busy, detail], () => {
  if (!["timeline", "build"].includes(tab.value) || !canLoad.value || busy.value || timeline.value || !detail.value || arena.value || timelineRequested === key.value) return;
  timelineRequested = key.value;
  void load("timeline", false);
});
</script>

<template>
  <section ref="root" class="full-match-detail">
    <div class="analysis-tabs" role="tablist" :aria-label="uiText.matchDetails.title">
      <button v-for="(item, i) in tabs" :key="item.key" role="tab" :id="`${scopeId}-${gameId}-${item.key}`"
        :aria-selected="tab === item.key" :aria-controls="`${scopeId}-panel-${gameId}`" :tabindex="tab === item.key ? 0 : -1"
        @click="tab = item.key" @keydown="moveTab($event, i)">{{ item.label }}</button>
    </div>
    <div :id="`${scopeId}-panel-${gameId}`" role="tabpanel" :aria-labelledby="`${scopeId}-${gameId}-${tab}`" class="analysis-tab-content">
      <template v-if="tab === 'overview'">
        <ArenaScoreboard v-if="detail && arena && hasScoreboard" :detail="detail" :match="match" :connection-id="player.connectionId"/>
        <MatchScoreboard v-else-if="detail && hasScoreboard" :detail="detail" :match="match" :self="self" :connection-id="player.connectionId"/>
        <template v-else>
          <div class="analysis-load summary-status">
            <span role="status">{{ fallbackStatus }}</span>
            <button v-if="canLoad && busy !== 'detail'" class="secondary" :disabled="!!busy" @click="load('detail')">
              {{ detailError ? uiText.matchDetails.retryScoreboard : uiText.matchDetails.readScoreboard }}
            </button>
          </div>
          <div v-if="match" class="personal-content">
            <GameIcon :id="match.championId" :size="32"/>
            <div><strong>{{ historyHeroName(match.championId) }}</strong><small>{{ kda(match) }}</small></div>
            <div v-if="match.participation != null"><small>{{ uiText.matchHistoryRow.participation }}</small><strong>{{ percent(match.participation) }}</strong></div>
            <div v-if="match.damage != null"><small>{{ uiText.matchHistoryRow.damage }}</small><strong>{{ matchNumber(match.damage) }}</strong></div>
            <div v-if="match.cs != null"><small>{{ uiText.matchHistoryRow.cs }}</small><strong>{{ matchNumber(match.cs) }}</strong></div>
            <div class="summary-items" :aria-label="uiText.matchHistoryRow.finalItems">
              <GameIcon v-for="slot in 7" :key="slot" kind="items" :id="match.items?.[slot - 1]" :size="20"/>
            </div>
          </div>
        </template>
        <div v-if="showRoster" class="saved-roster"><MatchHistoryRoster :groups="roster" :self="self" :connection-id="player.connectionId" detail/></div>
      </template>
      <template v-else-if="tab === 'team'">
        <TeamAnalysis v-if="detail && hasScoreboard" :detail="detail" :match="match" :connection-id="player.connectionId"/>
        <div v-else class="analysis-load">
          <span role="status">{{ busy === 'detail' ? uiText.matchDetails.loadingScoreboardShort : detailError && canLoad ? detailError : uiText.matchDetails.teamAnalysisHint }}</span>
          <button v-if="canLoad && !busy" class="secondary" @click="load('detail')">{{ detailError ? uiText.matchDetails.retryScoreboard : uiText.matchDetails.readScoreboard }}</button>
        </div>
      </template>
      <template v-else>
        <p v-if="timelineError || detailError" class="error-banner" role="alert">{{ timelineError || detailError }}</p>
        <div v-if="!detail && detailError && canLoad" class="analysis-load"><button class="secondary" :disabled="!!busy" @click="load('detail', false)">{{ uiText.matchDetails.retryScoreboard }}</button></div>
        <template v-if="tab === 'timeline'">
          <MatchTimeline v-if="timeline && detail" :timeline="timeline" :detail="detail"/>
          <div v-else class="analysis-load"><span>{{ provided ? uiText.matchDetails.archivedTimeline : busy ? uiText.matchDetails.loadingTimeline : timelineError ? '' : uiText.matchDetails.timelineHint }}</span>
            <button v-if="!provided && timelineError" class="secondary" :disabled="!!busy || !canLoad" @click="load('timeline')">{{ busy === 'timeline' ? uiText.matchDetails.loadingTimeline : uiText.matchDetails.loadTimeline }}</button>
          </div>
        </template>
        <template v-else>
          <MatchBuild v-if="match" :match="match" :timeline="timeline" :participant-id="target?.participantId" :loading="!!busy"/>
          <div v-if="(!timeline || timeline.partial) && !provided" class="analysis-load"><span>{{ busy ? uiText.matchDetails.loadingTimeline : '' }}</span>
            <button v-if="timelineError || timeline?.partial" class="secondary" :disabled="!!busy || !canLoad" @click="load('timeline')">{{ busy === 'timeline' ? uiText.matchDetails.loadingTimeline : uiText.matchDetails.loadTimeline }}</button>
          </div>
        </template>
      </template>
    </div>
    <details class="detail-note"><summary>{{ uiText.common.statistics }}</summary><p>{{ uiText.matchDetails.matchStatistics(gameId) }}</p></details>
  </section>
</template>

<style scoped>
.full-match-detail { min-width:0; }
.analysis-tabs { position:sticky; top:0; z-index:3; display:flex; gap:4px; padding:4px; background:var(--raised); border-block:1px solid var(--border); }
.analysis-tabs button { flex:1; min-height:32px; border:0; background:none; border-radius:4px; font-size:12px; color:var(--muted); }
.analysis-tabs button[aria-selected=true] { background:var(--panel); color:var(--text); box-shadow:inset 0 -2px var(--accent); }
.analysis-tab-content { min-height:72px; }
.analysis-load { display:flex; align-items:center; justify-content:space-between; gap:8px; flex-wrap:wrap; padding:9px 12px; font-size:12px; color:var(--muted); }
.analysis-load button { flex:none; font-size:12px; }
.summary-status { border-bottom:1px solid var(--border); background:var(--raised); }
.personal-content { display:flex; align-items:center; flex-wrap:wrap; gap:8px 14px; padding:10px 12px; font-variant-numeric:tabular-nums; }
.personal-content>div:not(.summary-items) { display:grid; gap:2px; }
.personal-content strong { font-size:13px; }
.personal-content small { color:var(--muted); font-size:11px; }
.summary-items { display:flex; align-items:center; gap:2px; }
.saved-roster { padding:10px 12px; border-top:1px solid var(--border); }
.detail-note { padding:7px 12px; color:var(--muted); font-size:11px; border-top:1px solid var(--border); }
.detail-note summary { cursor:pointer; }
.detail-note p { margin:6px 0 0; }
.full-match-detail .error-banner { margin:8px 12px; }
@container (max-width:620px) {
  .analysis-tabs button { font-size:11px; }
}
</style>
