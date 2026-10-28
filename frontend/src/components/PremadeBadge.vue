<script setup lang="ts">
import { computed } from 'vue';
import type { Participant } from '../../bindings/leaguepingo/internal/core/models';
const props=defineProps<{player:Participant}>();
const label=computed(()=>{
 const token=props.player.premadeGroup||'';
 // A stable, anonymous short code for this match. Colour is accompanied by text.
 return props.player.premadeLabel || (token?token.split('-').at(-1)!.slice(0,3).toUpperCase():'');
});
</script>
<template><small v-if="player.premadeGroup && (player.premadeSize||0)>1" class="premade-badge" :data-group="player.premadeGroup" :title="`客户端返回的同组标识 · ${player.premadeSize} 人 · 国服组队含义待实机验证`">同组 {{label}} · {{player.premadeSize}}人</small></template>
<style scoped>.premade-badge{display:inline-flex;align-items:center;white-space:nowrap;max-width:100%;font-size:11px;line-height:18px;padding:0 4px;border:1px solid var(--border);border-radius:3px;color:var(--ally);font-weight:500;font-variant-numeric:tabular-nums;}.scout-enemy .premade-badge{color:var(--enemy);}</style>
