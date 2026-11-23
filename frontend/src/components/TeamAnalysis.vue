<script setup lang="ts">
import { matchMode } from "../matchMode";
import { computed, ref } from "vue";
import type { Match, MatchDetail } from "../../bindings/leaguepingo/internal/core/models";
import { teamMetrics, teamMetric, type TeamMetricKey } from "../analysisData";
import { historyTarget } from "../playerHistoryNavigation";
import { uiText } from "../uiText";
import PlayerHistoryAction from "./PlayerHistoryAction.vue";
import GameIcon from "./GameIcon.vue";

const props = defineProps<{ detail: MatchDetail; match?: Match; connectionId?: number }>();
const selected = ref<TeamMetricKey>("damage");
const metrics = computed(() => teamMetrics.filter(metric => matchMode(props.match ?? props.detail.players?.[0]?.summary).vision || !["visionScore", "cs"].includes(metric.key)).map(metric => ({ ...metric, ...teamMetric(props.detail, metric.key) })));
const metric = computed(() => metrics.value.find(item => item.key === selected.value) ?? metrics.value.find(item => item.key === "damage")!);
const teams = computed(() => metric.value.teams.map(team => ({
  ...team,
  players: [...team.players].sort((a, b) =>
    (b.summary[metric.value.key] ?? -1) - (a.summary[metric.value.key] ?? -1)
    || a.participantId - b.participantId),
})));
const number = (value: number | null | undefined) => value == null ? "—" : value.toLocaleString();
const teamName = (id: number) => id === 100 ? uiText.teamAnalysis.blue : uiText.common.red;
</script>

<template>
  <section class="team-analysis" :aria-label="uiText.teamAnalysis.label">
    <div class="team-totals">
      <div class="team-total-head"><span>{{ uiText.teamAnalysis.title }}</span><span>{{ uiText.teamAnalysis.blue }}</span><span>{{ uiText.common.red }}</span></div>
      <button v-for="item in metrics" :key="item.key" class="team-total-row" :aria-pressed="metric.key === item.key" @click="selected = item.key">
        <span>{{ item.label }}</span><b v-for="team in item.teams" :key="team.teamId">{{ number(team.total) }}</b>
      </button>
    </div>
    <p v-if="detail.partial" class="analysis-note">{{ uiText.teamAnalysis.incompleteTotals }}</p>
    <h3>{{ metric.label }}</h3>
    <div class="metric-comparison">
      <section v-for="team in teams" :key="team.teamId" class="metric-team" :class="{ red: team.teamId === 200 }" :aria-label="teamName(team.teamId)">
        <h4>{{ teamName(team.teamId) }}</h4>
        <div v-for="player in team.players" :key="player.participantId" class="metric-player" :class="{ target: player.target }">
          <PlayerHistoryAction class="metric-identity" :target="historyTarget(player.displayName, connectionId)">
            <GameIcon :id="player.summary.championId" :size="24" />
            <span>{{ player.displayName?.split('#')[0] || uiText.matchDetails.missingPlayerName }}</span>
          </PlayerHistoryAction>
          <span class="metric-track"><i :style="{ width: `${metric.max ? 100 * (player.summary[metric.key] ?? 0) / metric.max : 0}%` }" /></span>
          <b>{{ number(player.summary[metric.key]) }}</b>
        </div>
      </section>
    </div>
  </section>
</template>

<style scoped>
.team-analysis { padding:12px 16px 16px; font-variant-numeric:tabular-nums; }
.team-totals { max-width:620px; margin-inline:auto; }
.team-total-head, .team-total-row { display:grid; grid-template-columns:1fr 1fr 1fr; align-items:center; gap:16px; width:100%; text-align:right; }
.team-total-head { padding:5px 10px; font-size:12px; color:var(--muted); }
.team-total-head>span:first-child, .team-total-row>span { text-align:left; }
.team-total-row { min-height:29px; border:0; border-radius:3px; background:transparent; font-size:12px; padding:5px 10px; }
.team-total-row b { font-weight:500; }
.team-total-row:hover { background:var(--hover); }
.team-total-row[aria-pressed=true] { background:var(--raised); box-shadow:inset 2px 0 var(--accent); }
.team-analysis h3 { font-size:13px; margin:16px 0 8px; }
.metric-comparison { display:grid; grid-template-columns:1fr 1fr; gap:24px; }
.metric-team { min-width:0; }
.metric-team h4 { font-size:12px; font-weight:500; color:var(--ally); margin:0 0 6px; }
.metric-team.red h4 { color:var(--enemy); }
.metric-player { display:grid; grid-template-columns:minmax(80px,1fr) 1fr 52px; align-items:center; gap:8px; padding:4px; min-height:34px; border-radius:3px; }
.metric-player.target { background:var(--raised); box-shadow:inset 2px 0 var(--accent); }
.metric-identity { display:flex; align-items:center; gap:6px; min-width:0; font-size:12px; }
.metric-identity>span { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.metric-track { height:7px; background:var(--raised); }
.metric-track i { display:block; height:100%; background:var(--ally); }
.red .metric-track i { background:var(--enemy); }
.metric-player>b { text-align:right; font-size:12px; font-weight:500; }
.analysis-note { font-size:11px; color:var(--muted); }
@container(max-width:620px) { .metric-comparison { grid-template-columns:1fr; gap:16px; } }
</style>
