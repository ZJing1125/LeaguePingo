<script setup lang="ts">
import { computed } from 'vue';
import type { Match } from '../../bindings/leaguepingo/internal/core/models';
import { decimal, number, mapName, positionName, runeName, sampleStats } from '../presentation';
import GameIcon from './GameIcon.vue';
const props=defineProps<{match:Match}>();
const perMinute=(value:number|null|undefined)=>value!=null && props.match.duration && props.match.duration>0?`${decimal(value*60/props.match.duration)} / 分`:'—';
const stats=computed(()=>[
  {label:'KDA',value:decimal(sampleStats([props.match]).kda),note:'（击杀 + 助攻）/ 死亡'},
  {label:'补刀',value:number(props.match.cs),note:perMinute(props.match.cs)},
  {label:'英雄伤害',value:number(props.match.damage),note:perMinute(props.match.damage)},
  {label:'获得金币',value:number(props.match.gold),note:perMinute(props.match.gold)},
]);
</script>
<template>
  <section class="match-personal" aria-label="个人对局表现">
    <header class="personal-heading"><h3>个人表现</h3><div class="match-context-detail" aria-label="对局上下文"><span v-if="match.mapId">{{ mapName(match.mapId) }}</span><span v-if="match.position || match.lane">{{ positionName(match.position || match.lane) }}</span><span v-if="match.earlyEnd===true" class="early-end" title="提前结束，不代表已确认重开">提前结束</span></div></header>
    <dl class="personal-metrics"><div v-for="stat in stats" :key="stat.label"><dt>{{ stat.label }}</dt><dd>{{ stat.value }}</dd><small>{{ stat.note }}</small></div></dl>
    <div class="personal-loadout">
      <section><h4>终局装备</h4><div class="personal-items"><GameIcon v-for="slot in 7" :key="slot" kind="items" :id="match.items?.[slot-1]" :size="30"/></div><div class="personal-spells"><GameIcon kind="spells" :id="match.spell1" :size="22"/><GameIcon kind="spells" :id="match.spell2" :size="22"/><span>召唤师技能</span></div></section>
      <section class="personal-runes" aria-label="本玩家符文"><h4>符文配置</h4><small v-if="match.runePrimary || match.runeSecondary">{{ match.runePrimary ? runeName(match.runePrimary) : '主系未知' }} / {{ match.runeSecondary ? runeName(match.runeSecondary) : '副系未知' }}</small><div v-if="match.runeIds?.length" class="rune-path"><span v-for="(rune,index) in match.runeIds" :key="`${rune}:${index}`" :title="`符文 #${rune}`" :class="{keystone:index===0}">{{ runeName(rune) }}</span></div><p v-else class="muted">符文未记录</p></section>
    </div>
  </section>
</template>
<style scoped>
.match-personal{min-width:0;}
.personal-heading{display:flex;justify-content:space-between;align-items:baseline;gap:12px;flex-wrap:wrap;margin-bottom:16px;}
.personal-heading h3{font-size:14px;margin:0;}
.personal-heading .match-context-detail{margin:0;gap:12px;font-size:12px;}
.personal-metrics{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;margin:0 0 20px;}
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
