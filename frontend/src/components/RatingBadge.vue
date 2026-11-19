<script setup lang="ts">
import type { PlayerAnalysis, TeamRating } from '../../bindings/leaguepingo/internal/core/models';
import { analysisTitle } from '../playerAnalysis';
import { uiText } from '../uiText';
defineProps<{ analysis?: PlayerAnalysis | null; rating?: TeamRating | null; compact?: boolean; pending?: boolean }>();
</script>
<template>
  <span class="player-rating" :class="{compact}" :title="analysisTitle(analysis, rating)" :aria-label="analysisTitle(analysis, rating)">
    <template v-if="rating && analysis?.score != null">
      <b>{{ rating.label }}</b><span>{{ uiText.playerAnalysis.teamPlace(rating.place, rating.rated) }}</span><small v-if="analysis.provisional">{{ uiText.playerAnalysis.provisional }}</small>
    </template>
    <template v-else-if="!compact && analysis?.score != null">
      <b>{{ analysis.score.toFixed(1) }} / 10</b><small v-if="analysis.provisional">{{ uiText.playerAnalysis.provisional }}</small>
    </template>
    <small v-else-if="analysis?.score == null">{{ pending ? uiText.playerAnalysis.preparing : uiText.playerAnalysis.insufficient }}</small>
  </span>
</template>
<style scoped>
.player-rating{display:inline-flex;align-items:baseline;gap:5px;white-space:nowrap;flex:none;line-height:14px;font-variant-numeric:tabular-nums;color:var(--text);}
.player-rating b{font-size:12px;font-weight:650;}.player-rating>span{font-size:12px;}.player-rating small{font-size:11px;color:var(--muted);font-weight:400;}
.scout-card .player-rating{color:var(--scout-text);}.scout-card .player-rating small{color:var(--scout-muted);}
.compact{line-height:13px;}.compact b,.compact>span{font-size:11px;}
</style>
