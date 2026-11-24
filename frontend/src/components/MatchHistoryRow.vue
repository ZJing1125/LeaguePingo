<script setup lang="ts">
import { matchMode } from "../matchMode";
import { computed } from "vue";
import type { Match } from "../../bindings/leaguepingo/internal/core/models";
import { date, decimal, duration, kda, outcome, queueName, sampleStats, percent, positionName } from "../presentation";
import { matchHighlight, perMinute } from "../analysisData";
import { isRemake, matchTone } from "../matchStats";
import { historyRoster, historyPosition, historyHeroName, matchNumber } from "../historyMatch";
import { uiText } from "../uiText";
import GameIcon from "./GameIcon.vue";
import RuneIcon from "./RuneIcon.vue";
import AppIcon from "./AppIcon.vue";
import RoleIcon from "./RoleIcon.vue";
import MatchSummary from "./MatchSummary.vue";
import MatchHistoryRoster from "./MatchHistoryRoster.vue";

const props = defineProps<{ match: Match; expanded: boolean; compact?: boolean; self?: boolean; connectionId?: number }>();
defineEmits<{ toggle: [] }>();

const teams = computed(() => matchMode(props.match).known ? historyRoster(props.match, props.connectionId) : []);
const csRate = computed(() => perMinute(props.match.cs, props.match.duration));
const role = computed(() => matchMode(props.match).roles && historyPosition(props.match));
const highlight = computed(() => matchHighlight(props.match));
const remake = computed(() => isRemake(props.match));
const arena = computed(() => matchMode(props.match).arena);
</script>

<template>
  <article class="match-card" :class="[matchTone(match), { 'match-open': expanded, 'match-compact': compact }]">
    <div class="match-row-layout">
      <button type="button" class="match-main" :aria-expanded="expanded"
        :aria-label="uiText.playerHistoryNavigation.matchDetails(historyHeroName(match.championId), date(match.playedAt))" @click="$emit('toggle')">
        <span class="match-context">
          <span class="match-result"><b class="outcome">{{ outcome(match) }}</b><small>{{ duration(match.duration) }}</small></span>
          <strong>{{ queueName(match.queueId) }}</strong>
          <small>{{ date(match.playedAt) }}</small>
        </span>
        <span class="match-build-summary">
          <span class="match-performance">
            <span class="match-hero"><GameIcon :id="match.championId" :size="40"/><small v-if="match.championLevel != null" class="champion-level">{{ match.championLevel }}</small></span>
            <span class="match-systems">
              <GameIcon kind="spells" :id="match.spell1" :size="16"/>
              <RuneIcon v-if="matchMode(match).runes && match.runeIds?.[0]" :id="match.runeIds[0]" :size="16"/><span v-else></span>
              <GameIcon kind="spells" :id="match.spell2" :size="16"/>
              <RuneIcon v-if="matchMode(match).runes && match.runeSecondary" :id="match.runeSecondary" :size="16"/>
            </span>
            <span class="match-identity">
              <strong>{{ kda(match) }}</strong>
              <small v-if="!remake" class="match-kda-rate">{{ decimal(sampleStats([match]).kda) }} KDA</small>
              <span class="match-role"><span>{{ historyHeroName(match.championId) }}</span><span v-if="role" class="match-role-name"><RoleIcon :role="role"/>{{ positionName(role) }}</span></span>
            </span>
          </span>
          <span class="match-items" :aria-label="uiText.matchHistoryRow.finalItems">
            <GameIcon v-for="slot in 7" :key="slot" kind="items" :id="match.items?.[slot - 1]" :size="20"/>
          </span>
        </span>
        <span class="match-efficiency">
          <span v-if="remake">{{ uiText.common.earlyEnd }}</span>
          <template v-else>
            <span v-if="arena && match.arena?.teamCount">{{ uiText.arena.teams(match.arena.teamCount) }}</span>
            <span v-if="!arena && matchMode(match).known">{{ uiText.matchHistoryRow.participation }}<b>{{ percent(match.participation) }}</b></span>
            <span>{{ uiText.matchHistoryRow.damage }}<b>{{ matchNumber(match.damage) }}</b></span>
            <span v-if="matchMode(match).cs" class="match-cs" :title="uiText.matchHistoryRow.csHint">
              {{ uiText.matchHistoryRow.cs }}<b>{{ matchNumber(match.cs) }}</b><span v-if="csRate != null">{{ uiText.matchHistoryRow.perMinute(csRate.toFixed(1)) }}</span>
            </span>
            <b v-if="highlight" class="match-facts">{{ uiText.matchHistoryRow[highlight] }}</b>
          </template>
        </span>
        <span class="expand-indicator" aria-hidden="true"><AppIcon name="chevron" :size="16"/></span>
      </button>
      <MatchHistoryRoster v-if="teams.length" class="match-row-roster" :groups="teams" :self="self" :connection-id="connectionId"/>
      <span v-else-if="!arena && matchMode(match).known" class="match-row-roster match-roster-missing">{{ uiText.matchHistoryRow.missingRoster }}</span>
    </div>
    <div v-if="expanded" class="match-expanded"><slot name="details"><MatchSummary :match="match"/></slot></div>
  </article>
</template>

<style scoped>
.match-card {
  container-type:inline-size;
  margin:0 0 5px;
  border-inline-start:3px solid var(--border);
  border-radius:5px;
  background:var(--raised);
}
.match-card.won { background:color-mix(in srgb,var(--win) 11%,var(--panel)); border-color:var(--win); }
.match-card.lost { background:color-mix(in srgb,var(--loss) 11%,var(--panel)); border-color:var(--loss); }
.match-card.remake { border-color:var(--muted); }
.match-row-layout {
  width:100%;
  display:grid;
  grid-template-columns:100px minmax(170px,205px) minmax(150px,1fr) minmax(185px,.9fr) 16px;
  grid-template-areas:"context build metrics roster arrow";
  align-items:center;
  gap:10px;
  min-height:79px;
  padding:5px 10px;
}
.match-main {
  display:grid;
  grid-column:1/-1;
  grid-row:1/-1;
  grid-template-columns:subgrid;
  grid-template-rows:subgrid;
  align-items:center;
  min-width:0;
  padding:0;
  border:0;
  border-radius:inherit;
  text-align:left;
  background:none;
  color:var(--text);
  font-variant-numeric:tabular-nums;
}
.match-compact .match-row-layout { padding-inline:8px; }
.match-row-layout:hover { background:color-mix(in srgb,var(--text) 5%,transparent); }
.match-main:focus-visible { outline-offset:-2px; }
.match-context { grid-area:context; display:grid; gap:3px; min-width:0; font-size:12px; }
.match-context small { font-size:11px; color:var(--muted); }
.match-result { display:flex; align-items:baseline; gap:6px; }
.match-result .outcome { color:var(--text); font-size:14px; }
.won .outcome { color:var(--win); }
.lost .outcome { color:var(--loss); }
.match-build-summary { grid-area:build; display:grid; gap:5px; min-width:0; }
.match-performance { display:flex; align-items:center; gap:6px; min-width:0; }
.match-hero { position:relative; display:flex; flex:none; }
.champion-level { position:absolute; right:-2px; bottom:-2px; border-radius:3px; padding:0 3px; color:var(--surface); background:var(--text); font-size:10px; }
.match-systems { display:grid; grid-template-columns:repeat(2,16px); align-items:center; gap:2px; flex:none; }
.match-identity { display:grid; gap:1px; min-width:0; font-size:15px; line-height:1.15; white-space:nowrap; }
.match-kda-rate { color:var(--muted); font-size:11px; }
.match-role { display:flex; align-items:center; gap:5px; min-width:0; font-size:11px; color:var(--muted); }
.match-role>span:first-child { overflow:hidden; text-overflow:ellipsis; }
.match-role-name { display:inline-flex; align-items:center; gap:2px; flex:none; }
.match-role-name .role-icon { width:12px; height:12px; }
.match-items { display:flex; align-items:center; gap:2px; min-width:0; }
.match-items :deep(.game-icon:last-child) { margin-inline-start:3px; }
.match-efficiency { grid-area:metrics; display:grid; gap:2px; font-size:11px; color:var(--muted); white-space:nowrap; }
.match-efficiency b { color:var(--text); font-weight:650; }
.match-efficiency .match-facts { color:var(--accent); }
.match-row-roster { grid-area:roster; z-index:1; pointer-events:none; }
.match-row-roster :deep(button) { pointer-events:auto; }
.match-roster-missing { color:var(--muted); font-size:11px; text-align:center; }
.expand-indicator { grid-area:arrow; display:grid; place-items:center; color:var(--muted); }
.match-open .expand-indicator { transform:rotate(90deg); }
.match-expanded { container-type:inline-size; background:var(--panel); border-top:1px solid var(--border); }

@container (min-width:850px) {
  .match-efficiency { grid-template-columns:auto auto; justify-content:start; align-items:baseline; gap:4px 14px; }
  .match-efficiency .match-facts { justify-self:start; }
}
@container (max-width:755px) {
  .match-row-layout { grid-template-columns:84px minmax(155px,1fr) 115px 106px 16px; gap:8px; }
}
@container (max-width:545px) {
  .match-row-layout { grid-template-columns:79px minmax(155px,1fr) 16px; grid-template-areas:"context build arrow" "roster metrics arrow"; gap:7px; }
  .match-efficiency { display:flex; flex-wrap:wrap; column-gap:8px; white-space:normal; }
  .match-efficiency>span { white-space:nowrap; }
  .match-row-roster :deep(.roster-team) { gap:1px; padding-inline-start:2px; }
  .match-row-roster :deep(.game-icon) { width:13px!important; height:13px!important; }
}
@container (max-width:285px) {
  .match-row-layout { grid-template-columns:minmax(0,1fr) 16px; grid-template-areas:"context arrow" "build arrow" "metrics arrow" "roster arrow"; }
  .match-context { display:flex; flex-wrap:wrap; align-items:baseline; gap:6px; }
}
</style>
