<script setup lang="ts">
import { computed } from 'vue';
import type { Participant } from '../../bindings/leaguepingo/internal/core/models';
import { playerName,percent,decimal } from '../presentation';
import { rankPresentation } from '../rankPresentation';
import { scoutStats } from '../scoutStats';
import { championName } from '../gameAssets';
import ParticipantAvatar from './ParticipantAvatar.vue';
import RankEmblem from './RankEmblem.vue';
import GameIcon from './GameIcon.vue';
import PlayerCardData from './PlayerCardData.vue';
const props=defineProps<{player:Participant;queue:number;picking:boolean}>();
const data=computed(()=>scoutStats(props.player,props.queue));
const rank=computed(()=>rankPresentation(props.player,props.queue===440?'RANKED_FLEX_SR':'RANKED_SOLO_5x5'));
const hero=computed(()=>props.player.championId?championName(props.player.championId).split('·').at(-1):'等待选择英雄');
const heroState=computed(()=>!props.player.championId?'等待选择':!data.value.report?'统计待获取':data.value.report.state==='loading'?'准备中':!data.value.report.season?'赛季待确认':data.value.champion.games?'胜负暂缺':data.value.report.complete?'本季暂无记录':'本季已读范围');
</script>
<template>
  <details class="sidebar-player" :data-player-key="player.key">
    <summary>
      <span class="sidebar-identity"><ParticipantAvatar :player="player" account :size="30"/><span><strong :title="playerName(player)">{{ playerName(player) }}</strong><span class="sidebar-rank" :title="`${data.queueLabel} · ${rank.title} · 等级 ${player.profile?.level ?? '—'}`">{{ rank.label }}<small v-if="player.stale"> · 旧</small></span></span><RankEmblem :tier="rank.rank?.tier"/></span>
      <span v-if="picking" class="sidebar-hero"><GameIcon v-if="player.championId" :id="player.championId" :size="26"/><span><span class="sidebar-hero-title"><strong :title="hero">{{ hero }}<small v-if="player.championState==='intent'"> · 预选</small></strong><b>{{ data.champion.outcomeGames?percent(data.champion.winRate):heroState }}<small v-if="data.champion.kda!=null"> · {{ decimal(data.champion.kda) }} KDA</small></b></span><span class="sidebar-experience"><small v-for="tag in data.tags" :key="tag.label" :title="tag.title">{{ tag.label }}</small></span></span></span>
      <span class="sidebar-metrics"><span><small>12小时</small><b>{{ data.periods[0]!.complete && !data.periods[0]!.games ? '0场' : percent(data.periods[0]!.winRate) }}</b></span><span><small>30天位置</small><b>{{ data.role.key?data.role.label:'—' }}</b></span><span><small>30天</small><b>{{ data.periods[1]!.complete && !data.periods[1]!.games ? '0场' : percent(data.periods[1]!.winRate) }}</b></span></span>
      <small v-if="!data.report" class="sidebar-coverage">统计待获取</small><small v-else-if="data.report.state==='loading'" class="sidebar-coverage">正在准备统计</small><small v-else-if="!data.report.complete30" class="sidebar-coverage">时段统计为已读范围</small>
    </summary>
    <div class="scout-card sidebar-expanded"><PlayerCardData :player="player" :queue="queue" :party="!picking"/></div>
  </details>
</template>
