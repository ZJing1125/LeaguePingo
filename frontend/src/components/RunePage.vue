<script setup lang="ts">
import { uiText } from "../uiText";
import {computed} from 'vue';
import type{Match}from '../../bindings/leaguepingo/internal/core/models';
import catalog from '../assets/game/analysis/runes.json';
import {runeName}from '../presentation';
import RuneIcon from './RuneIcon.vue';
const props=defineProps<{match:Match}>();
const styles=computed(()=>[props.match.runePrimary,props.match.runeSecondary].map(id=>catalog.find(s=>s.id===id)));
const selected=computed(()=>new Set(props.match.runeIds??[]));
const known=new Set(catalog.flatMap(s=>s.slots.flatMap(slot=>slot.runes.map(r=>r.id))));
const shards = computed(() => props.match.runeShards?.length === 3 ? props.match.runeShards : []);
const extra=computed(()=>(props.match.runeIds??[]).filter(id=>!known.has(id)));
const modernHealth = computed(() => {
 const [major, minor] = (props.match.gameVersion ?? "").split(".").map(Number);
 return Number.isInteger(major) && (major > 14 || major === 14 && minor >= 2);
});
const shardName=(id:number)=>id === 0 ? uiText.common.missing : ({5001:modernHealth.value ? uiText.runePage.scalingHealth : uiText.runePage.scalingHealthUnknown,5002:uiText.runePage.armor,5003:uiText.runePage.magicResistance,5005:uiText.runePage.attackSpeed,5007:uiText.runePage.haste,5008:uiText.runePage.adaptiveForce,5010:uiText.runePage.movementSpeed,5011:uiText.runePage.health,5013:uiText.runePage.tenacity} as Record<number,string>)[id]??uiText.runePage.stat(id);
const shardImages = import.meta.glob('../assets/game/shards/*.png', {eager:true, query:'?url', import:'default'}) as Record<string,string>;
const shardOptions = computed(() => [[5008,5005,5007], [5008,5010,5001], [5011,5013,5001]].map((options, slot) => {
  const selected = shards.value[slot];
  return selected && !options.includes(selected) ? [...options, selected] : options;
}));
</script>
<template><section class="rune-tree" :class="{ 'shards-missing': !shards.some(Boolean) }" :aria-label="uiText.runePage.label"><div v-for="(style,index) in styles" :key="index" class="rune-branch"><header><RuneIcon :id="style?.id" :size="34"/><span>{{index===0?uiText.runePage.primary:uiText.runePage.secondary}}<b>{{style?.name??uiText.common.missing}}</b></span></header><div v-if="style" class="rune-slots"><div v-for="(slot,row) in style.slots" :key="row" class="rune-options"><span v-for="rune in slot.runes" :key="rune.id" :class="{chosen:selected.has(rune.id)}" :title="rune.name"><RuneIcon :id="rune.id" :size="row===0?34:28" :selected="selected.has(rune.id)"/><small v-if="selected.has(rune.id)">{{rune.name}}</small></span></div></div><p v-else class="muted">{{ uiText.runePage.missingTree }}</p></div><div class="rune-shards"><h4>{{ uiText.runePage.shards }}</h4><div v-for="(options, slot) in shardOptions" :key="slot" class="shard-row"><div class="shard-options"><span v-for="id in options" :key="id" :class="{chosen:shards[slot]===id}" :title="shardName(id)"><img v-if="shardImages[`../assets/game/shards/${id}.png`]" :src="shardImages[`../assets/game/shards/${id}.png`]" :alt="shardName(id)"/><span v-else>◇</span></span></div><small>{{shardName(shards[slot] ?? 0)}}</small></div><p v-if="!shards.some(Boolean)" class="muted">{{ uiText.runePage.missingShards }}</p></div><p v-if="extra.length" class="rune-extra">{{extra.map(runeName).join(' · ')}}</p></section></template>
<style scoped>.rune-tree{display:grid;grid-template-columns:1fr 1fr minmax(110px,.55fr);gap:20px;padding:18px 0;font-size:12px;}.rune-branch header{display:flex;align-items:center;justify-content:center;gap:10px;padding-bottom:18px;}.rune-branch header span{color:var(--muted);font-size:11px;}.rune-branch header b{display:block;color:var(--text);font-size:14px;margin-top:3px;}.rune-slots{display:grid;gap:12px;}.rune-options{display:flex;justify-content:center;gap:16px;min-height:42px;}.rune-options>span{width:40px;text-align:center;}.rune-options small{display:block;font-size:11px;white-space:nowrap;margin-top:4px;transform:translateX(-12px);width:64px;color:var(--muted);}.rune-shards{border-left:1px solid var(--border);padding-left:16px;}.rune-shards h4{margin:8px 0 22px;font-size:12px;}.shard-row{margin:12px 0;text-align:center;}.shard-options{display:flex;justify-content:center;gap:12px;margin-bottom:6px;}.shard-options>span{opacity:.25;filter:grayscale(1);}.shard-options .chosen{opacity:1;filter:none;box-shadow:0 0 0 1px var(--accent);border-radius:50%;}.shard-options img{display:block;width:28px;height:28px;}.shard-row small{font-size:11px;line-height:1.5;}.rune-extra{grid-column:1/-1;color:var(--muted);}.muted{font-size:12px;line-height:1.6;}@container(max-width:550px){.rune-tree{grid-template-columns:1fr 1fr;gap:16px 8px;}.rune-shards{grid-column:1/-1;border:0;border-top:1px solid var(--border);display:flex;gap:16px;flex-wrap:wrap;padding:10px 0;align-items:center;}.rune-shards h4,.rune-shards>span{margin:0;}.rune-options{gap:8px;}}.shards-missing .rune-shards{grid-column:1/-1;border:0;padding:0;display:flex;align-items:center;gap:12px;}.shards-missing .shard-row{display:none;}.shards-missing .rune-shards h4,.shards-missing .rune-shards p{margin:0;} .rune-tree.shards-missing{grid-template-columns:1fr 1fr;}</style>
