<script setup lang="ts">
import { matchMode } from "../matchMode";
import { uiText } from "../uiText";
import { computed } from 'vue';
import type { Match } from '../../bindings/leaguepingo/internal/core/models';
import { decimal, number, percent, mapName, positionName, runeName, sampleStats } from '../presentation';
import { isArena } from '../matchStats';
import GameIcon from './GameIcon.vue';
const props=defineProps<{match:Match}>();
const perMinute=(value:number|null|undefined)=>value!=null && props.match.duration && props.match.duration>0?uiText.matchSummary.perMinute(decimal(value*60/props.match.duration)):'—';
const stats=computed(()=>[
  {label:'KDA',value:decimal(sampleStats([props.match]).kda),note:uiText.matchSummary.kdaFormula},
  ...(matchMode(props.match).roles ? [{label:uiText.common.cs,value:number(props.match.cs),note:perMinute(props.match.cs)}] : []),
  {label:uiText.common.damage,value:number(props.match.damage),note:perMinute(props.match.damage)},
  {label:uiText.matchSummary.gold,value:number(props.match.gold),note:perMinute(props.match.gold)},
  isArena(props.match) ? {label:uiText.arena.placement,value:props.match.arena ? uiText.arena.place(props.match.arena.placement) : '—',note:props.match.arena?.teamCount ? uiText.arena.teams(props.match.arena.teamCount) : uiText.arena.unknownTeams} : {label:uiText.common.participation,value:percent(props.match.participation),note:uiText.matchSummary.participationHint},
  ...(matchMode(props.match).vision ? [{label:uiText.common.visionScore,value:number(props.match.visionScore),note:uiText.matchSummary.wards(number(props.match.wardsPlaced), number(props.match.wardsKilled))}] : []),
]);
</script>
<template>
  <section class="match-personal" :aria-label="uiText.matchSummary.label">
    <header class="personal-heading"><h3>{{ uiText.matchSummary.title }}</h3><div class="match-context-detail" :aria-label="uiText.matchSummary.context"><span v-if="match.mapId">{{ mapName(match.mapId) }}</span><span v-if="matchMode(match).roles && (match.position || match.lane)">{{ positionName(match.position || match.lane) }}</span><span v-if="match.earlyEnd===true" class="early-end" :title="uiText.matchSummary.unconfirmedEarlyEnd">{{ uiText.common.earlyEnd }}</span></div></header>
    <dl class="personal-metrics"><div v-for="stat in stats" :key="stat.label"><dt>{{ stat.label }}</dt><dd>{{ stat.value }}</dd><small>{{ stat.note }}</small></div></dl>
    <div class="personal-loadout">
      <section><h4>{{ uiText.common.finalItems }}</h4><div class="personal-items"><GameIcon v-for="slot in 7" :key="slot" kind="items" :id="match.items?.[slot-1]" :size="30"/></div><div class="personal-spells"><GameIcon kind="spells" :id="match.spell1" :size="22"/><GameIcon kind="spells" :id="match.spell2" :size="22"/><span>{{ uiText.common.spells }}</span></div></section>
      <section v-if="matchMode(match).runes" class="personal-runes" :aria-label="uiText.matchSummary.runes"><h4>{{ uiText.common.runes }}</h4><small v-if="match.runePrimary || match.runeSecondary">{{ match.runePrimary ? runeName(match.runePrimary) : uiText.matchSummary.unknownPrimary }} / {{ match.runeSecondary ? runeName(match.runeSecondary) : uiText.matchSummary.unknownSecondary }}</small><div v-if="match.runeIds?.length" class="rune-path"><span v-for="(rune,index) in match.runeIds" :key="`${rune}:${index}`" :title="uiText.matchSummary.rune(rune)" :class="{keystone:index===0}">{{ runeName(rune) }}</span></div><p v-else class="muted">{{ uiText.matchSummary.missingRunes }}</p></section>
    </div>
  </section>
</template>
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.match-context-detail):is(.match-context-detail) {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px 12px;
  font-size: 11px;
  color: var(--muted);
  margin: 8px 0;
}
:where(.early-end):is(.match-context-detail .early-end) {
  color: var(--warning);
}
:where(.match-context-detail):is(.match-context-detail) {
  color: var(--muted);
}
</style>
<style scoped>
.match-personal{min-width:0;}
.personal-heading{display:flex;justify-content:space-between;align-items:baseline;gap:12px;flex-wrap:wrap;margin-bottom:16px;}
.personal-heading h3{font-size:14px;margin:0;}
.personal-heading .match-context-detail{margin:0;gap:12px;font-size:12px;}
.personal-metrics{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;margin:0 0 20px;}
.personal-metrics dt{font-size:12px;color:var(--muted);}
.personal-metrics dd{font-size:22px;font-weight:650;font-variant-numeric:tabular-nums;margin:6px 0;}
.personal-metrics small{display:block;font-size:11px;color:var(--muted);line-height:1.5;}
.personal-loadout{display:grid;grid-template-columns:minmax(235px,.9fr) minmax(0,1.1fr);gap:24px;}
.personal-loadout h4{font-size:12px;margin:0 0 10px;font-weight:600;}
.personal-items,.personal-spells{display:flex;gap:4px;align-items:center;}
.personal-items .game-icon,.personal-spells .game-icon{flex:none;}
.personal-spells{margin-top:8px;}.personal-spells>span:last-child{color:var(--muted);font-size:11px;margin-left:4px;}
.personal-runes>small{font-size:12px;color:var(--muted);}
.rune-path{display:flex;flex-wrap:wrap;gap:6px 14px;margin-top:10px;line-height:1.7;font-size:12px;}
.rune-path .keystone{font-weight:650;color:var(--text);}
@container (max-width:600px){.personal-loadout{grid-template-columns:1fr;gap:18px;}.personal-metrics{gap:12px;grid-template-columns:repeat(2,minmax(0,1fr));}}
</style>
