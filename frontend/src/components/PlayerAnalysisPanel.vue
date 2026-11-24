<script setup lang="ts">
import type { PlayerAnalysis, TeamRating } from '../../bindings/leaguepingo/internal/core/models';
import { analysisSummary, analysisTitle } from '../playerAnalysis';
import { uiText } from '../uiText';
import RatingBadge from './RatingBadge.vue';
defineProps<{ responseId?: number; analysis?: PlayerAnalysis | null; rating?: TeamRating; mode: string }>();
const text = uiText.playerAnalysis;
</script>
<template>
  <section class="player-analysis-panel" :aria-label="text.title">
    <header><h2>{{ text.title }}</h2><span>{{ mode }}</span></header>
    <div class="analysis-answer"><RatingBadge :response-id="responseId" :analysis="analysis" :rating="rating"/><span>{{ rating?.reason ?? analysisSummary(analysis) }}</span></div>
    <p :title="analysisTitle(analysis, rating)">{{ text.scope }} · {{ text.samples(analysis?.scoredGames ?? 0, analysis?.games ?? 0) }}</p>
    <p v-if="rating?.tags && rating.tags.length>1">{{rating.tags.slice(1).join(' · ')}}</p>
    <p v-if="rating && rating.place > 0">{{text.teamPlaceHint(rating.place,rating.rated)}}</p>
    <dl v-if="analysis?.score != null"><template v-for="metric in analysis.metrics" :key="metric.key"><dt>{{ (text.metrics as Record<string,string>)[metric.key] ?? text.unknownMetric }}</dt><dd>{{ metric.score.toFixed(1) }}<small> / {{ analysis.version >= 2 ? 100 : 10 }} · {{ text.metricGames(metric.games) }}</small></dd></template></dl>
    <details><summary>{{ text.rules }}</summary><p>{{ analysis?.basis === 'placement' ? text.arenaRules : analysis && analysis.version >= 4 ? text.rulesHint : analysis && analysis.version >= 3 ? text.legacyTeamHorseRules : analysis && analysis.version >= 2 ? text.absoluteRulesHint : text.legacyRulesHint }}</p><p>{{ analysis && analysis.version >= 2 ? text.basicHint : text.legacyBasicHint }}</p><p>{{ text.liveScope }}</p></details>
  </section>
</template>
<style scoped>
.player-analysis-panel{border:1px solid var(--border);border-radius:8px;background:var(--panel);min-width:0;font-variant-numeric:tabular-nums;}
header{display:flex;align-items:baseline;justify-content:space-between;gap:8px;padding:10px 12px;}h2{font-size:13px;margin:0;}header>span,p{font-size:11px;color:var(--muted);}
.analysis-answer{display:flex;align-items:baseline;justify-content:space-between;gap:8px;padding:12px 12px 0;}.analysis-answer>span:last-child{font-size:12px;}.player-analysis-panel>p{margin:7px 12px 10px;line-height:1.5;}
dl{display:grid;grid-template-columns:1fr auto;gap:8px;margin:0;padding:0 12px 12px;font-size:12px;}dt{color:var(--muted);}dd{margin:0;text-align:right;}dd small{font-size:11px;color:var(--muted);}
details{padding:9px 12px;font-size:11px;}summary{cursor:pointer;color:var(--muted);}details p{line-height:1.6;margin:8px 0 0;}
</style>
