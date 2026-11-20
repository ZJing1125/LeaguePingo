<script setup lang="ts">
import { uiText } from "../uiText";
import { computed, reactive, ref, watch } from "vue";
import type { Participant, Snapshot } from "../../bindings/leaguepingo/internal/core/models";
import { date, duration, outcome, playerName, positionName, queueName, sampleStats, type PlayerViewState } from "../presentation";
import { SAMPLE_SIZES, useHistoryBrowser } from "../historyBrowser";
import AppIcon from "./AppIcon.vue";
import ChampionSplash from "./ChampionSplash.vue";
import ParticipantAvatar from "./ParticipantAvatar.vue";
import RankEmblem from "./RankEmblem.vue";
import OverviewPerformance from './OverviewPerformance.vue';
import RankPanel from './RankPanel.vue';
import ChampionAnalysis from './ChampionAnalysis.vue';
import PlaystylePanel from './PlaystylePanel.vue';
import MasteryPanel from './MasteryPanel.vue';
import ConnectionGuide from './ConnectionGuide.vue';

import RecentForm from "./RecentForm.vue";
import HistoryNotice from "./HistoryNotice.vue";
import PlayerDrawer from './PlayerDrawer.vue';
import MatchHistoryRow from './MatchHistoryRow.vue';
import MatchDetails from './MatchDetails.vue';
import { historyHeroName, historyPosition } from '../historyMatch';
import GameIcon from './GameIcon.vue';
import { hasCurrentMatch } from '../matchRoom';
import HistoryServiceStatus from './HistoryServiceStatus.vue';

const props = defineProps<{ player: Participant | null; snapshot: Snapshot | null; status: string; loading: boolean; busy: boolean; error: string; active?: boolean }>();
const emit=defineEmits<{ open: [player: Participant]; match: []; refresh: []; modal:[open:boolean];recovered:[player:Participant] }>();
const selectedGame=ref<number|null>(null);
watch(()=>`${props.active}:${props.player?.connectionId}:${props.player?.key}`,()=>{selectedGame.value=null;});
const state = ref<PlayerViewState>({ queue: "all", expanded: [], sampleSize: 20 });
const historyScroll = ref<HTMLElement | null>(null);
const historyTop = ref<HTMLElement | null>(null);
const placeholder = { key: "home-empty", puuid: "", connectionId: 0, identityState: "unavailable", historyState: "pending", history: [], historyMeta: {} } as unknown as Participant;
watch(() => `${props.player?.connectionId}:${props.player?.key}`, () => { state.value.stopRequested = true; state.value = { queue: "all", expanded: [], sampleSize: 20 }; });
const browseProps = reactive({ get player() { return props.player ?? placeholder; }, get state() { return state.value; }, get connected() { return props.snapshot?.connected; }, get busy() { return props.busy || props.loading; }, get active() { return props.active; } });
const { allMatches, analysisMatches: matches, pageMatches, knownPages, canFetch, sampleLabel, needsSamples, noticePlayer, changeCriteria, goPage, fill, jumpDraft, jumpError, jumpPage, maxJumpPage } = useHistoryBrowser(browseProps, historyScroll, () => historyTop.value?.scrollIntoView({ block: 'start' }));
const selectedMatch=computed(()=>allMatches.value.find(m=>m.gameId===selectedGame.value));
watch(selectedMatch,match=>{if(!match)selectedGame.value=null;emit('modal',!!match);});
const stats = computed(() => sampleStats(matches.value));
const modes = computed(() => [...new Set([420, 440, 450, 2400, ...allMatches.value.map(m => m.queueId)])]);
const mains = computed(() => [...new Set(matches.value.map(m => m.championId).filter(Boolean))].map(id => ({ id, ...sampleStats(matches.value.filter(m => m.championId === id)) })).sort((a, b) => b.games - a.games).slice(0, 5));
const historyMessage = computed(() => !props.snapshot?.connected ? uiText.homeView.offline : props.loading || props.player?.historyState === "loading" ? uiText.homeView.preparing : props.player?.historyState === "failed" ? uiText.homeView.unavailable : state.value.queue !== "all" ? uiText.homeView.noFilterMatch : uiText.homeView.empty);
</script>
<template>
  <section class="home-workspace" :aria-label="uiText.homeView.title">
    <header class="home-heading"><div><h1>{{ uiText.common.overview }}</h1></div><button class="secondary" :disabled="busy || loading || !snapshot?.connected || !snapshot?.selfKey" @click="$emit('refresh')"><AppIcon name="refresh" :size="15" />{{ loading ? uiText.common.preparingShort : uiText.common.refresh }}</button></header>
    <div class="home-hero">
      <div v-if="mains.length" class="home-hero-art" aria-hidden="true"><ChampionSplash :id="mains[0]?.id" /></div>
      <div class="home-account"><ParticipantAvatar v-if="player" :player="player" account :size="64" /><span v-else class="home-avatar-empty"><AppIcon name="user" :size="30" /></span><div><span class="home-account-caption">{{ snapshot?.connected ? uiText.homeView.account : uiText.homeView.waitingClient }}</span><h2>{{ player ? playerName(player) : snapshot?.connected ? loading ? uiText.homeView.identifying : uiText.homeView.pendingProfile : uiText.homeView.login }}</h2><p><span v-if="player?.profile?.level != null">{{ uiText.homeView.level(player.profile.level) }}</span>{{ status.split(" · ")[0] }}<span v-if="player?.stale || !snapshot?.connected && player">{{ uiText.homeView.stale }}</span></p><span v-if="player?.privacyState === 'private'" class="home-privacy">{{ uiText.common.privateCareer }}</span></div></div>
      <div class="home-form-summary"><div class="home-form-last"><span>{{ uiText.homeView.recent }}</span><RecentForm v-if="matches.length" :matches="matches"/><span v-else>{{ uiText.homeView.waitingHistory }}</span><small>{{sampleLabel}}</small></div></div>
    </div>
    <ConnectionGuide v-if="!snapshot?.connected" />
    <p v-if="error" class="error-banner home-error" role="status">{{ error }}</p>
    <div class="home-grid">
      <aside class="home-ranks">
        <template v-if="player"><RankPanel :player="player"/><RankPanel :player="player" queue="RANKED_FLEX_SR"/></template>
        <section v-if="!player" class="home-panel home-rank"><span>{{ uiText.homeView.rank }}</span><RankEmblem /><strong>{{ uiText.homeView.waitingAccount }}</strong><small>{{ uiText.homeView.connectedHint }}</small></section>
        <section class="home-panel home-session"><span class="home-panel-caption">{{ uiText.homeView.status }}</span><strong><i class="connection-light" :class="{ online: snapshot?.connected }" />{{ status.split(" · ")[0] }}</strong><p>{{ snapshot?.stage === 'in-game' ? uiText.homeView.holdHint : uiText.homeView.sidebarHint }}</p><button v-if="hasCurrentMatch(snapshot) || snapshot?.review" :class="hasCurrentMatch(snapshot) ? 'primary' : 'secondary'" @click="$emit('match')">{{ hasCurrentMatch(snapshot) ? uiText.homeView.currentMatch : uiText.homeView.review }} <AppIcon name="chevron" :size="14" /></button></section>
        <PlaystylePanel :matches="matches" :scope="sampleLabel"/>
        <ChampionAnalysis :matches="matches" :scope="sampleLabel" :fetched-at="player?.historyMeta?.fetchedAt"/>
        <MasteryPanel v-if="player" :player="player"/>
      </aside>
      <div class="home-analysis-main"><OverviewPerformance :matches="matches" :scope="sampleLabel"/>
      <section ref="historyTop" class="home-panel home-recent">
        <header><h2>{{ uiText.common.history }}</h2><HistoryServiceStatus v-if="player" inline readonly :player="player"/><button class="text-button" :disabled="!player || busy || loading" @click="player && $emit('open', player)">{{ uiText.homeView.fullHistory }}<AppIcon name="chevron" :size="13" /></button></header>
        <HistoryServiceStatus v-if="player" :player="player" :connected="snapshot?.connected" :busy="busy || loading || state.pageBusy || state.warmBusy" :automatic="active!==false && selectedGame===null && snapshot?.stage==='idle' && (state.page??1)===1" @recovered="emit('recovered',$event)" @retry="emit('refresh')"/>
        <div class="home-history-filters"><select v-model="state.queue" :aria-label="uiText.homeView.modeFilter" :disabled="!player || busy" @change="changeCriteria()"><option value="all">{{ uiText.common.allModes }}</option><option v-for="mode in modes" :key="mode" :value="String(mode)">{{ queueName(mode) }}</option></select><select v-model.number="state.sampleSize" :aria-label="uiText.homeView.sampleSize" :disabled="!player || busy" @change="changeCriteria('samples')"><option v-for="size in SAMPLE_SIZES" :key="size" :value="size">{{ uiText.common.recentGames(size) }}</option></select><span>{{ uiText.homeView.sampleCount(stats.games, state.sampleSize ?? 20) }}</span></div>
        <div v-if="state.pageBusy || state.warmBusy || state.sourceLimit || state.pageError || state.browseNotice" class="home-browse-status" role="status"><span>{{ state.pageBusy ? uiText.homeView.loadingMore : state.warmBusy ? uiText.homeView.preparingRanked : state.pageError || state.sourceLimit || state.browseNotice }}</span><button v-if="state.pageBusy" class="text-button" @click="state.stopRequested = true">{{ uiText.homeView.stop }}</button><button v-else-if="!state.warmBusy && needsSamples && canFetch" class="text-button" :disabled="busy" @click="fill(state.browseTarget ?? state.page ?? 1, state.pageError || state.browseNotice ? state.browseKind : 'samples')">{{ uiText.homeView.continueSearch }}</button></div>
        <div ref="historyScroll" class="home-match-list">
          <div v-if="!pageMatches.length" class="home-no-history"><AppIcon name="history" :size="34" /><h3>{{ loading || player?.historyState === 'loading' ? uiText.homeView.preparingHistory : uiText.homeView.recentHistory }}</h3><p>{{ historyMessage }}</p></div>
          <MatchHistoryRow v-for="match in pageMatches" :key="match.gameId" class="home-match" :match="match" :connection-id="player?.connectionId" :expanded="false" self @toggle="!busy && !loading && (selectedGame=match.gameId)"/>
        </div>
        <nav class="home-pagination" :aria-label="uiText.homeView.pagination"><button class="secondary" :disabled="busy || state.pageBusy || (state.page ?? 1) <= 1" @click="goPage((state.page ?? 1) - 1)">{{ uiText.common.previous }}</button><span>{{ uiText.common.page(state.page ?? 1) }}<small>{{ uiText.common.pageSize }}</small></span><button class="secondary" :disabled="busy || state.pageBusy || ((state.page ?? 1) >= knownPages && (!state.hasMore || !canFetch))" @click="goPage((state.page ?? 1) + 1)">{{ uiText.common.next }}</button><form class="home-page-jump" @submit.prevent="jumpPage"><input v-model="jumpDraft" :aria-label="uiText.homeView.jumpLabel" :aria-invalid="!!jumpError" :title="uiText.homeView.pageRange(maxJumpPage)" inputmode="numeric" maxlength="3" :disabled="busy || state.pageBusy" /><button class="secondary" :disabled="busy || state.pageBusy">{{ uiText.common.jump }}</button></form></nav>
        <p v-if="jumpError" class="page-jump-error" role="alert">{{ jumpError }}</p>
      </section>
      <div class="home-sample-note"><HistoryNotice v-if="player" :player="noticePlayer" :stats="stats" :filtered="state.queue !== 'all'"/></div>
      </div>
    </div>
    <PlayerDrawer v-if="selectedMatch && player" :player-key="`home-game-${selectedMatch.gameId}`" :label="uiText.common.details" modal fit :active="active ?? true" @close="selectedGame=null">
      <header class="home-game-heading">
        <div class="home-game-identity">
          <GameIcon :id="selectedMatch.championId" :size="32"/>
          <div>
            <h2>{{ historyHeroName(selectedMatch.championId) }} · {{ outcome(selectedMatch) }}</h2>
            <p><span v-if="historyPosition(selectedMatch)">{{ positionName(selectedMatch.position) }}</span><span>{{ queueName(selectedMatch.queueId) }}</span><span>{{ date(selectedMatch.playedAt) }}</span><span>{{ duration(selectedMatch.duration) }}</span></p>
          </div>
        </div>
        <button class="icon-button" :aria-label="uiText.common.closeDetails" @click="selectedGame=null"><AppIcon name="close"/></button>
      </header>
      <div class="home-game-content"><MatchDetails :player="player" :game-id="selectedMatch.gameId" :match="selectedMatch" :connected="snapshot?.connected" self/></div>
    </PlayerDrawer>
  </section>
</template>
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.home-workspace):is(.home-workspace) { flex: 1; min-height: 0; padding: 16px 22px 20px; overflow: auto; scrollbar-gutter: stable; }
:where(.home-heading):is(.home-heading) { display: flex; justify-content: space-between; align-items: center; min-height: 40px; margin-bottom: 10px; }
:where(.home-heading):is(.home-heading) :deep(h1) { font-size: 26px; line-height: 30px; letter-spacing: -.5px; }
:where(.home-hero):is(.home-hero) { position: relative; isolation: isolate; overflow: hidden; display: flex; gap: 22px; align-items: center; justify-content: space-between; padding: 14px 18px; min-height: 100px; border: 1px solid var(--border); border-radius: 10px; background: var(--panel); }
:where(.home-hero-art):is(.home-hero-art) { position: absolute; inset: 0 0 0 14%; z-index: -1; opacity: .32; mask-image: linear-gradient(90deg, transparent, #000 48%, #000 68%, transparent); }
:where(.home-hero):is(.home-hero)::after { position: absolute; inset: 0; z-index: -1; content: ''; background: linear-gradient(90deg, var(--panel) 0%, transparent 56%, color-mix(in srgb, var(--panel) 88%, transparent) 85%); }
:where(.home-account):is(.home-account) { display: flex; align-items: center; gap: 16px; min-width: 0; flex: 1; }
:where(.home-account):is(.home-account) :deep(>div) { min-width: 0; }
:where(.home-account):is(.home-account) :deep(h2) { font-size: 20px; line-height: 1.35; margin: 4px 0 6px; overflow-wrap: anywhere; }
:where(.home-account):is(.home-account) :deep(p) { font-size: 12px; color: var(--muted); }
:where(.home-account-caption):is(.home-account-caption) { font-size: 12px; color: var(--muted); }
:where(.home-avatar-empty):is(.home-avatar-empty) { flex: none; display: grid; place-items: center; width: 64px; height: 64px; border-radius: 10px; background: var(--raised); color: var(--muted); }
:where(.home-privacy):is(.home-privacy) { font-size: 11px; color: var(--muted); display: block; margin-top: 7px; }
:where(.home-form-summary):is(.home-form-summary) { display: flex; align-items: center; gap: 24px; flex: none; }
:where(.home-form-summary):is(.home-form-summary) :deep(>div) { display: flex; flex-direction: column; gap: 8px; min-width: 95px; }
:where(.home-form-summary):is(.home-form-summary) :deep(>div>span:first-child) { color: var(--muted); font-size: 12px; }
:where(.home-form-summary):is(.home-form-summary) :deep(strong) { font-size: 32px; line-height: 34px; font-weight: 650; }
:where(.home-form-summary):is(.home-form-summary) :deep(small) { color: var(--muted); font-size: 11px; }
:where(.home-form-last):is(.home-form-last) { align-self: stretch; justify-content: center; }
:where(.home-grid):is(.home-grid) {display:grid;grid-template-columns:280px minmax(0,1fr);gap:16px;margin-top:16px;align-items:start;}
:where(.home-panel):is(.home-panel) {background:var(--panel);border:1px solid var(--border);border-radius:8px;min-width:0;overflow:hidden;}
:where(.home-panel):is(.home-panel) :deep(>header) {display:flex;align-items:center;justify-content:space-between;gap:10px;padding:10px 14px;min-height:42px;border-bottom:1px solid var(--border);}
:where(.home-panel):is(.home-panel) :deep(>header h2) {font-size:14px;}
:where(.home-panel):is(.home-panel) :deep(header .text-button) {font-size:12px;padding:0;}
:where(.home-ranks):is(.home-ranks) {display:flex;flex-direction:column;gap:12px;min-width:0;}
:where(.home-rank):is(.home-rank) {padding:16px;display:flex;flex-wrap:wrap;gap:8px;color:var(--muted);font-size:12px;}
:where(.home-rank):is(.home-rank) :deep(strong){color:var(--text);}
:where(.home-rank):is(.home-rank) :deep(small){font-size:11px;}
:where(.home-session):is(.home-session) {padding:12px;display:flex;flex-direction:column;gap:9px;}
:where(.home-panel-caption):is(.home-panel-caption){font-size:11px;color:var(--muted);}
:where(.home-session):is(.home-session) :deep(strong){display:flex;align-items:center;gap:6px;font-size:13px;}
:where(.home-session):is(.home-session) :deep(p){font-size:12px;line-height:1.6;color:var(--muted);}
:where(.home-session):is(.home-session) :deep(button){display:flex;justify-content:space-between;align-items:center;font-size:12px;}
:where(.home-analysis-main):is(.home-analysis-main){display:flex;flex-direction:column;gap:12px;min-width:0;container-type:inline-size;}
:where(.home-recent):is(.home-recent){display:flex;flex-direction:column;min-height:0;}
:where(.home-match-list):is(.home-match-list){min-height:0;overflow:visible;}
:where(.home-history-filters):is(.home-history-filters) { padding: 7px 10px; display: flex; gap: 7px; align-items: center; min-height: 42px; border-bottom: 1px solid var(--border); flex: none; }
:where(.home-history-filters):is(.home-history-filters) :deep(select) { min-width: 0; max-width: 142px; font-size: 12px; }
:where(.home-history-filters):is(.home-history-filters) :deep(>span) { margin-left: auto; white-space: nowrap; font-size: 11px; color: var(--muted); }
:where(.home-pagination):is(.home-pagination) { flex: none; display: flex; align-items: center; justify-content: center; gap: 8px; min-height: 46px; border-top: 1px solid var(--border); padding: 6px 8px; }
:where(.home-pagination):is(.home-pagination) :deep(>span) { display: flex; flex-direction: column; align-items: center; font-size: 12px; white-space: nowrap; }
:where(.home-pagination):is(.home-pagination) :deep(small) { font-size: 11px; color: var(--muted); margin-top: 2px; }
:where(.home-pagination):is(.home-pagination) :deep(button) { font-size: 11px; padding: 5px 9px; }
:where(.home-page-jump):is(.home-page-jump) { display: flex; gap: 5px; margin-left: auto; }
:where(.home-page-jump):is(.home-page-jump) :deep(input) { width: 38px; min-width: 0; padding: 4px; font-size: 12px; text-align: center; }
:where(.home-browse-status):is(.home-browse-status) { display: flex; align-items: center; justify-content: space-between; font-size: 12px; color: var(--muted); line-height: 1.5; padding: 6px 10px; border-bottom: 1px solid var(--border); flex: none; }
:where(.home-browse-status):is(.home-browse-status) :deep(.text-button) { white-space: nowrap; font-size: 12px; }
:where(.home-sample-note):is(.home-sample-note) { flex: none; border-top: 1px solid var(--border); padding: 10px 12px; font-size: 11px; line-height: 1.6; }
:where(.home-sample-note):is(.home-sample-note) :deep(>span) { display: block; margin-bottom: 4px; }
:where(.home-no-history):is(.home-no-history) { min-height: 180px; height: 100%; display: flex; flex-direction: column; justify-content: center; align-items: center; padding: 24px; gap: 14px; color: var(--muted); text-align: center; }
:where(.home-no-history):is(.home-no-history) :deep(h3) { font-size: 15px; color: var(--text); margin: 0; }
:where(.home-no-history):is(.home-no-history) :deep(p) { font-size: 12px; line-height: 1.8; }
:where(.home-error):is(.home-error) { margin-top: 12px; }
@media(max-width:1100px){:where(.home-grid):is(.home-grid){grid-template-columns:250px minmax(0,1fr);gap:12px;}}
@media(max-width:1100px){:where(.home-workspace):is(.home-workspace){padding-inline:16px;}}
@media(max-width:940px){:where(.home-grid):is(.home-grid){grid-template-columns:minmax(0,1fr);}}
@media(max-width:940px){:where(.home-ranks):is(.home-ranks){display:grid;grid-template-columns:1fr 1fr;gap:10px;}}
@media(max-width:940px){:where(.home-session):is(.home-session){grid-column:1/-1;display:grid;grid-template-columns:1fr auto;align-items:center;gap:6px 12px;}}
@media(max-width:940px){:where(.home-session):is(.home-session) :deep(>span){grid-column:1;}}
@media(max-width:940px){:where(.home-session):is(.home-session) :deep(>strong){grid-column:1;}}
@media(max-width:940px){:where(.home-session):is(.home-session) :deep(>p){grid-column:1;}}
@media(max-width:940px){:where(.home-session):is(.home-session) :deep(>button){grid-column:2;grid-row:1/4;}}
@media(max-width:940px){
  :where(.home-grid):is(.home-grid){grid-template-columns:repeat(2,minmax(0,1fr));gap:12px;}
}
@media(max-width:940px){
  :where(.home-ranks):is(.home-ranks){display:contents;}
}
@media(max-width:940px){
  :where(.home-rank):is(.home-ranks>.home-rank){grid-column:1/-1;grid-row:1;}
}
@media(max-width:940px){
  :where(.home-analysis-main):is(.home-analysis-main){grid-column:1/-1;grid-row:2;}
}
@media(max-width:940px){
  :where(.home-session):is(.home-session){grid-row:3;}
}
</style>
<style scoped>
.home-game-heading { display:flex; justify-content:space-between; align-items:center; gap:12px; padding:10px 14px; flex:none; }
.home-game-identity { display:flex; align-items:center; gap:8px; min-width:0; }
.home-game-heading h2 { margin:0; font-size:15px; }
.home-game-heading p { display:flex; gap:4px 9px; flex-wrap:wrap; margin:2px 0 0; font-size:11px; color:var(--muted); }
.home-game-content { min-height:0; overflow:auto; background:var(--panel); container-type:inline-size; }

</style>
