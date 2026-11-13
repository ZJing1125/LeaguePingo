<script setup lang="ts">
import type { PlayerAnalysis } from '../../bindings/leaguepingo/internal/core/models';
import { analysisTitle } from '../playerAnalysis';
import { uiText } from '../uiText';
defineProps<{ analysis?: PlayerAnalysis | null; compact?: boolean; pending?: boolean }>();
</script>
<template>
  <span class="player-rating" :class="{compact}" :title="analysisTitle(analysis)" :aria-label="analysisTitle(analysis)">
    <template v-if="analysis?.score != null"><b>{{ uiText.playerAnalysis.grade(analysis.grade) }}</b><span>{{ analysis.score.toFixed(1) }}</span><small v-if="!compact && analysis.provisional">{{ uiText.playerAnalysis.provisional }}</small></template>
    <small v-else>{{ pending ? uiText.playerAnalysis.preparing : uiText.playerAnalysis.insufficient }}</small>
  </span>
</template>
<style scoped>
.player-rating{display:inline-flex;align-items:baseline;gap:5px;white-space:nowrap;flex:none;line-height:14px;font-variant-numeric:tabular-nums;color:var(--text);}
.player-rating b{font-size:12px;font-weight:650;}.player-rating>span{font-size:12px;}.player-rating small{font-size:11px;color:var(--muted);font-weight:400;}
.scout-card .player-rating{color:var(--scout-text);}.scout-card .player-rating small{color:var(--scout-muted);}
.compact{line-height:13px;}.compact b,.compact>span{font-size:11px;}
</style>
