<script setup lang="ts">
import { uiText } from "../uiText";
import{computed,ref}from'vue';
import type{Participant}from'../../bindings/leaguepingo/internal/core/models';
import{masteryData}from'../analysisData';
import{championName}from'../gameAssets';
import GameIcon from'./GameIcon.vue';
const props=defineProps<{player:Participant}>();const all=ref(false),heroes=computed(()=>masteryData(props.player));
</script>
<template><section class="mastery-panel"><header><h2>{{ uiText.common.mastery }}</h2><span :title="player.profile?.masteryFetchedAt??undefined">{{ uiText.common.allModes }}<span v-if="player.profile?.fields?.mastery?.state==='retained'">{{ uiText.homeView.stale }}</span></span></header><div v-if="heroes.length" class="mastery-heroes"><div v-for="hero in heroes.slice(0,all?heroes.length:4)" :key="hero.championId"><GameIcon :id="hero.championId" :size="42"/><b>{{ uiText.common.level(hero.level) }}</b><strong :title="championName(hero.championId)">{{championName(hero.championId).split('·').at(-1)}}</strong><small>{{hero.points.toLocaleString()}}</small></div></div><p v-else class="mastery-empty">{{player.profile?.masteries?uiText.masteryPanel.empty:uiText.masteryPanel.preparing}}</p><button v-if="heroes.length>4" @click="all=!all">{{all?uiText.common.collapse:uiText.common.expandChampions(heroes.length)}}</button></section></template>
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
@media(max-width:940px){
  :where(.mastery-panel):is(.home-ranks>.mastery-panel){grid-column:1/-1;grid-row:5;}
}
</style>
<style scoped>.mastery-panel{background:var(--panel);border:1px solid var(--border);border-radius:8px;min-width:0;overflow:hidden;}.mastery-panel header{display:flex;justify-content:space-between;gap:8px;align-items:center;padding:10px 12px;border-bottom:1px solid var(--border);}.mastery-panel h2{font-size:13px;margin:0;}.mastery-panel header>span{font-size:11px;color:var(--muted);}.mastery-heroes{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px 6px;padding:12px 10px;text-align:center;}.mastery-heroes>div{min-width:0;}.mastery-heroes b{display:block;font-size:11px;color:var(--accent);margin:5px 0;}.mastery-heroes strong{font-size:11px;display:block;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;font-weight:500;}.mastery-heroes small{display:block;font-size:11px;color:var(--muted);margin-top:5px;}.mastery-panel>button{width:100%;border:0;border-top:1px solid var(--border);border-radius:0;background:none;color:var(--accent);font-size:11px;padding:8px;}.mastery-empty{font-size:12px;color:var(--muted);padding:14px 12px;margin:0;line-height:1.6;}</style>
