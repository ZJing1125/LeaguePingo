<script setup lang="ts">
import { uiText } from "../uiText";
import{computed}from'vue';
import type{Match}from'../../bindings/leaguepingo/internal/core/models';
import{overviewData,roleLabels}from'../analysisData';
import RoleIcon from'./RoleIcon.vue';
const props=defineProps<{matches:Match[];scope:string}>();
const data=computed(()=>overviewData(props.matches));
const max=computed(()=>Math.max(1,...data.value.hours.map(h=>h.count)));
</script>
<template><section class="playstyle-panel"><header><h2>{{ data.rolesApplicable ? uiText.playstylePanel.title : uiText.playstylePanel.activity }}</h2><span>{{scope}}</span></header><div v-if="data.rolesApplicable" class="style-roles"><span v-for="p in data.positions" :key="p.role" :title="uiText.playstylePanel.roleCount(roleLabels[p.role], p.count)"><RoleIcon :role="p.role"/><b>{{data.knownPositions?`${Math.round(p.count*100/data.knownPositions)}%`:'—'}}</b></span></div><div class="activity-heading"><h3 v-if="data.rolesApplicable">{{ uiText.playstylePanel.activity }}</h3><span>{{ uiText.playstylePanel.range(max) }}</span></div><div class="activity-hours" :aria-label="uiText.playstylePanel.timedGames(data.timedGames)"><span v-for="hour in data.hours" :key="hour.hour" :title="uiText.playstylePanel.hourCount(hour.hour, hour.hour, hour.count)"><i :style="{height:`${hour.count*100/max}%`}"/></span></div><div class="activity-axis"><span>{{ uiText.playstylePanel.midnight }}</span><span>{{ uiText.playstylePanel.morning }}</span><span>{{ uiText.playstylePanel.noon }}</span><span>{{ uiText.playstylePanel.evening }}</span><span>{{ uiText.playstylePanel.night }}</span></div></section></template>
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
@media(max-width:940px){
  :where(.playstyle-panel):is(.home-ranks>.playstyle-panel){grid-column:1;grid-row:4;}
}
</style>
<style scoped>.playstyle-panel{background:var(--panel);border:1px solid var(--border);border-radius:8px;padding:11px 12px;min-width:0;}.playstyle-panel>header{display:flex;align-items:baseline;justify-content:space-between;gap:8px;}.playstyle-panel h2{font-size:13px;margin:0;white-space:nowrap;}.playstyle-panel header>span{font-size:11px;color:var(--muted);}.style-roles{display:flex;justify-content:space-between;gap:10px;margin:14px 0 15px;}.style-roles>span{display:flex;gap:5px;align-items:center;}.style-roles .role-icon{width:18px;height:18px;color:var(--muted);}.style-roles b{font-weight:500;font-size:11px;}.activity-heading{display:flex;justify-content:space-between;gap:6px;align-items:baseline;}.activity-heading h3{font-size:11px;font-weight:400;margin:0;}.activity-heading>span{font-size:11px;color:var(--muted);}.activity-hours{display:flex;gap:3px;height:28px;margin-top:9px;border-bottom:1px solid var(--border);}.activity-hours>span{flex:1;display:flex;flex-direction:column;justify-content:flex-end;}.activity-hours i{display:block;background:var(--win);width:100%;}.activity-axis{display:flex;justify-content:space-between;color:var(--muted);font-size:11px;margin-top:5px;}</style>
