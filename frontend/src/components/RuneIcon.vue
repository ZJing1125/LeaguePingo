<script setup lang="ts">
import { uiText } from "../uiText";
import {computed} from 'vue';
import {runeName} from '../presentation';
const props=withDefaults(defineProps<{id?:number|null;size?:number;selected?:boolean}>(),{size:24,selected:true});
const images=import.meta.glob('../assets/game/analysis/*.png',{eager:true,query:'?url',import:'default'}) as Record<string,string>;
const src=computed(()=>props.id?images[`../assets/game/analysis/${props.id}.png`]:undefined);
const label=computed(()=>props.id?runeName(props.id):uiText.matchSummary.missingRunes);
</script>
<template><span class="rune-icon" role="img" :aria-label="label" :title="label" :class="{'rune-unselected':!selected}" :style="{width:`${size}px`,height:`${size}px`}"><img v-if="src" :src="src" alt=""/><span v-else>◇</span></span></template>
<style scoped>.rune-icon{display:inline-flex;flex:none;align-items:center;justify-content:center;vertical-align:middle;color:var(--muted);}.rune-icon img{width:100%;height:100%;object-fit:contain;}.rune-unselected{opacity:.24;filter:grayscale(1);}</style>
