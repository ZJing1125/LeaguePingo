<script setup lang="ts">
import { uiText } from "../uiText";
import RatingBadge from "./RatingBadge.vue";
import PremadeBadge from "./PremadeBadge.vue";
import { computed } from 'vue';
import type { Participant } from '../../bindings/leaguepingo/internal/core/models';
import { playerName,percent,decimal } from '../presentation';
import { rankPresentation } from '../rankPresentation';
import { scoutStats } from '../scoutStats';
import { scoutCoverageText } from '../scoutCoverage';
import { championName } from '../gameAssets';
import ParticipantAvatar from './ParticipantAvatar.vue';
import RankEmblem from './RankEmblem.vue';
import GameIcon from './GameIcon.vue';
import PlayerCardData from './PlayerCardData.vue';
const props=defineProps<{player:Participant;queue:number;picking:boolean;showHero?:boolean}>();
const data=computed(()=>scoutStats(props.player,props.queue));
const coverage=computed(()=>scoutCoverageText(data.value.report));
const periodValue=(p:ReturnType<typeof scoutStats>["periods"][number])=>{
  if(!data.value.report) return uiText.common.pending;
  if(p.complete&&!p.games) return uiText.sidebarPlayer.zeroGames;
  if(!p.games) return data.value.report.state === "loading" ? uiText.common.preparing : data.value.report.read ? uiText.modeScouting.noSample : uiText.modeScouting.sampleFailed;
  return percent(data.value.arena ? p.arena?.firstPlaceRate : p.winRate);
};
const rank=computed(()=>rankPresentation(props.player,props.queue===440?'RANKED_FLEX_SR':'RANKED_SOLO_5x5'));
const hero=computed(()=>props.player.championId?championName(props.player.championId).split('·').at(-1):uiText.sidebarPlayer.waitingSelection);
const heroState=computed(()=>!props.player.championId?uiText.playerCardData.waitingSelection:!data.value.report?uiText.sidebarPlayer.pendingStatistics:data.value.report.state==='loading'?uiText.common.preparing:data.value.ranked&&!data.value.report.season?uiText.sidebarPlayer.pendingSeason:data.value.champion.games?uiText.sidebarPlayer.missingOutcome:data.value.heroComplete?uiText.modeScouting.noRecords:uiText.modeScouting.partial);
const heroArena = computed(() => data.value.champion.arena);
const heroRate = computed(() => data.value.arena ? heroArena.value?.firstPlaceRate != null ? uiText.arena.first(percent(heroArena.value.firstPlaceRate)) : uiText.arena.pending : data.value.champion.outcomeGames ? percent(data.value.champion.winRate) : heroState.value);
</script>
<template>
  <details class="sidebar-player" :data-player-key="player.key">
    <summary>
      <span class="sidebar-identity">
        <ParticipantAvatar :player="player" account :size="30"/>
        <span>
          <span class="sidebar-name-row"><strong :title="playerName(player)">{{ playerName(player) }}</strong><RatingBadge :response-id="data.report?.responseId" v-if="queue && (data.rating || data.analysis?.score == null)" :analysis="data.ratingAnalysis ?? data.analysis" :rating="data.rating" :pending="player.identityState === 'known' && (!data.report || data.report.state === 'loading')" compact/></span>
          <span class="sidebar-rank" :title="data.rating?.reason ?? rank.title">
            <template v-if="data.ranked">{{ rank.label }}</template>
            <template v-if="data.rating?.queueId === queue && data.analysis && data.mode">
              <span>{{ uiText.playerAnalysis.metricGames(data.analysis.scoredGames) }}</span>
              <span v-if="data.mode.month.kda != null">KDA {{ decimal(data.mode.month.kda) }}</span>
            </template>
            <PremadeBadge :player="player"/><small v-if="player.stale">{{ uiText.playerProfileHeader.old }}</small>
          </span>
        </span>
        <RankEmblem v-if="data.ranked" :tier="rank.rank?.tier"/>
      </span>
      <span v-if="picking || showHero" class="sidebar-hero"><GameIcon v-if="player.championId" :id="player.championId" :size="26"/><span><span class="sidebar-hero-title"><strong :title="hero">{{ hero }}<small v-if="player.championState==='intent'">{{ uiText.sidebarPlayer.intent }}</small></strong><b>{{ heroRate }}<small v-if="data.arena && heroArena?.averagePlace != null"> · {{ uiText.arena.average(decimal(heroArena.averagePlace)) }}</small><small v-else-if="data.champion.kda!=null"> · {{ decimal(data.champion.kda) }} KDA</small></b></span><span class="sidebar-experience"><small v-for="tag in data.tags" :key="tag.label" :title="tag.title">{{ tag.label }}</small></span></span></span>
      <small v-if="(picking || showHero) && !data.ranked">{{ uiText.modeScouting.hero30 }} · {{ data.queueLabel }}</small>
      <span class="sidebar-metrics" :style="data.roles ? undefined : {gridTemplateColumns: 'repeat(2,minmax(0,1fr))'}"><span><small>{{ uiText.sidebarPlayer.last12Hours }}{{ data.arena ? ' · ' + uiText.arena.firstRate : '' }}</small><b>{{ periodValue(data.periods[0]!) }}</b></span><span v-if="data.roles"><small>{{ uiText.playerCardData.role30Days }}</small><b>{{ data.role.key?data.role.label:data.report?'—':uiText.common.pending }}</b></span><span><small>{{ uiText.sidebarPlayer.last30Days }}{{ data.arena ? ' · ' + uiText.arena.firstRate : '' }}</small><b>{{ periodValue(data.periods[1]!) }}</b></span></span>
      <small v-if="!data.report" class="sidebar-coverage">{{ uiText.sidebarPlayer.pendingStatistics }}</small><small v-else-if="data.report.state==='loading'" class="sidebar-coverage">{{ data.report.read ? uiText.modeScouting.scope(data.queueLabel,data.overall.games) : uiText.sidebarPlayer.preparing }}</small><small v-else-if="!data.report.complete30" class="sidebar-coverage">{{ data.report.read ? uiText.sidebarPlayer.partialRange : coverage }}</small>
    </summary>
    <div class="scout-card sidebar-expanded"><PlayerCardData :player="player" :queue="queue" :party="!picking && !showHero"/></div>
  </details>
</template>
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.sidebar-player):is(.sidebar-player) {background:var(--panel);border:1px solid var(--border);border-radius:8px;overflow:hidden;margin-bottom:6px;}
:where(.sidebar-player):is(.sidebar-player) :deep(summary) {display:flex;flex-direction:column;cursor:pointer;list-style:none;padding:4px 8px;gap:3px;}
:where(.sidebar-player):is(.sidebar-player) :deep(summary::-webkit-details-marker) {display:none;}
:where(.sidebar-player):is(.sidebar-player) :deep(summary:focus-visible) {outline:2px solid var(--accent);outline-offset:-3px;}
:where(.sidebar-player):is(.sidebar-player) :deep(summary:hover) {background:var(--hover);}
:where(.sidebar-player[open]):is(.sidebar-player[open]) {border-color:var(--accent);}
:where(.sidebar-identity):is(.sidebar-identity) {display:flex;align-items:center;min-width:0;gap:7px;min-height:30px;}
:where(.sidebar-identity):is(.sidebar-identity) :deep(>span:not(.participant-avatar):not(.rank-emblem)) {min-width:0;flex:1;}
:where(.sidebar-identity):is(.sidebar-identity) :deep(strong) {display:block;font-size:13px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;line-height:16px;}
:where(.sidebar-identity):is(.sidebar-identity) :deep(small) {font-size:11px;color:var(--muted);}
:where(.sidebar-rank):is(.sidebar-rank) {font-size:11px;color:var(--muted);display:flex;gap:8px;flex-wrap:wrap;}
:where(.sidebar-rank):is(.sidebar-rank) :deep(b) {font-size:12px;color:var(--text);font-weight:600;}
:where(.sidebar-hero):is(.sidebar-hero) {display:flex;align-items:center;min-width:0;gap:6px;padding-top:0;border-top:0;}
:where(.sidebar-hero):is(.sidebar-hero) :deep(>span) {min-width:0;flex:1;}
:where(.sidebar-hero):is(.sidebar-hero) :deep(strong) {display:block;font-size:12px;overflow:hidden;white-space:nowrap;text-overflow:ellipsis;}
:where(.sidebar-hero):is(.sidebar-hero) :deep(small) {font-size:11px;color:var(--muted);}
:where(.sidebar-experience):is(.sidebar-experience) {font-size:11px;color:var(--muted);}
:where(.sidebar-experience):is(.sidebar-experience) {display:flex;gap:7px;white-space:nowrap;overflow:hidden;flex-wrap:nowrap;line-height:13px;}
:where(.sidebar-metrics):is(.sidebar-metrics) {display:grid;grid-template-columns:repeat(3,minmax(0,1fr));padding-top:0;border-top:0;}
:where(.sidebar-metrics):is(.sidebar-metrics) :deep(>span) {display:flex;flex-direction:column;text-align:center;gap:1px;}
:where(.sidebar-metrics):is(.sidebar-metrics) :deep(small) {font-size:11px;color:var(--muted);line-height:12px;}
:where(.sidebar-metrics):is(.sidebar-metrics) :deep(b) {font-weight:650;font-size:15px;line-height:17px;}
:where(.sidebar-coverage):is(.sidebar-coverage) {font-size:11px;color:var(--muted);text-align:center;}
:where(.sidebar-expanded.scout-card):is(.sidebar-expanded.scout-card) {display:block;height:auto;min-height:0;border:0;border-top:1px solid var(--border);border-radius:0;}
:where(.sidebar-rank):is(.sidebar-identity .sidebar-rank) {display:block;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;font-size:11px;line-height:13px;}
:where(.sidebar-hero-title):is(.sidebar-hero-title) {display:flex;align-items:baseline;justify-content:space-between;gap:5px;line-height:14px;}
:where(.sidebar-hero-title):is(.sidebar-hero-title) :deep(strong) {font-size:12px;}
:where(.sidebar-hero-title):is(.sidebar-hero-title) :deep(b) {font-size:11px;font-weight:600;white-space:nowrap;}
:where(.sidebar-hero-title):is(.sidebar-hero-title) :deep(b small) {font-weight:400;}
:where(.sidebar-experience):is(.sidebar-experience) :deep(small) {font-size:11px;}
:where(.sidebar-hero):is(.sidebar-hero) :deep(>span:not(.game-icon)) {flex:1;min-width:0;}
</style>


<style scoped>
.sidebar-name-row { display: flex; align-items: baseline; justify-content: space-between; gap: 6px; min-width: 0; }
.sidebar-rank { display: flex; align-items: baseline; gap: 6px; }
</style>
