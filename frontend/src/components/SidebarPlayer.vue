<script setup lang="ts">
import { uiText } from "../uiText";
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
const props=defineProps<{player:Participant;queue:number;picking:boolean}>();
const data=computed(()=>scoutStats(props.player,props.queue));
const coverage=computed(()=>scoutCoverageText(data.value.report));
const periodValue=(p:{games:number;winRate:number|null;complete:boolean})=>{
  if(!data.value.report) return uiText.common.pending;
  if(p.complete&&!p.games) return uiText.sidebarPlayer.zeroGames;
  if(!p.games) return uiText.common.pending;
  return percent(p.winRate);
};
const rank=computed(()=>rankPresentation(props.player,props.queue===440?'RANKED_FLEX_SR':'RANKED_SOLO_5x5'));
const hero=computed(()=>props.player.championId?championName(props.player.championId).split('·').at(-1):uiText.sidebarPlayer.waitingSelection);
const heroState=computed(()=>!props.player.championId?uiText.playerCardData.waitingSelection:!data.value.report?uiText.sidebarPlayer.pendingStatistics:data.value.report.state==='loading'?uiText.common.preparing:!data.value.report.season?uiText.sidebarPlayer.pendingSeason:data.value.champion.games?uiText.sidebarPlayer.missingOutcome:data.value.report.complete?uiText.playerCardData.emptySeason:uiText.sidebarPlayer.seasonReadScope);
</script>
<template>
  <details class="sidebar-player" :data-player-key="player.key">
    <summary>
      <span class="sidebar-identity"><ParticipantAvatar :player="player" account :size="30"/><span><strong :title="playerName(player)">{{ playerName(player) }}</strong><span class="sidebar-rank" :title="uiText.sidebarPlayer.summary(data.queueLabel, rank.title, player.profile?.level ?? '—')">{{ rank.label }} <PremadeBadge :player="player"/><small v-if="player.stale">{{ uiText.playerProfileHeader.old }}</small></span></span><RankEmblem :tier="rank.rank?.tier"/></span>
      <span v-if="picking" class="sidebar-hero"><GameIcon v-if="player.championId" :id="player.championId" :size="26"/><span><span class="sidebar-hero-title"><strong :title="hero">{{ hero }}<small v-if="player.championState==='intent'">{{ uiText.sidebarPlayer.intent }}</small></strong><b>{{ data.champion.outcomeGames?percent(data.champion.winRate):heroState }}<small v-if="data.champion.kda!=null"> · {{ decimal(data.champion.kda) }} KDA</small></b></span><span class="sidebar-experience"><small v-for="tag in data.tags" :key="tag.label" :title="tag.title">{{ tag.label }}</small></span></span></span>
      <span class="sidebar-metrics"><span><small>{{ uiText.sidebarPlayer.last12Hours }}</small><b>{{ periodValue(data.periods[0]!) }}</b></span><span><small>{{ uiText.playerCardData.role30Days }}</small><b>{{ data.role.key?data.role.label:data.report?'—':uiText.common.pending }}</b></span><span><small>{{ uiText.sidebarPlayer.last30Days }}</small><b>{{ periodValue(data.periods[1]!) }}</b></span></span>
      <small v-if="!data.report" class="sidebar-coverage">{{ uiText.sidebarPlayer.pendingStatistics }}</small><small v-else-if="data.report.state==='loading'" class="sidebar-coverage">{{ data.report.read ? uiText.sidebarPlayer.readGames(data.report.read) : uiText.sidebarPlayer.preparing }}</small><small v-else-if="!data.report.complete30" class="sidebar-coverage">{{ data.report.read ? uiText.sidebarPlayer.partialRange : coverage }}</small>
    </summary>
    <div class="scout-card sidebar-expanded"><PlayerCardData :player="player" :queue="queue" :party="!picking"/></div>
  </details>
</template>
