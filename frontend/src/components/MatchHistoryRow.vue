<script setup lang="ts">
import { uiText } from "../uiText";
import { computed } from 'vue';
import type { Match } from '../../bindings/leaguepingo/internal/core/models';
import { date, decimal, duration, number, kda, outcome, queueName, sampleStats, percent, positionName } from '../presentation';
import { perMinute, matchBadges } from '../analysisData';
import { isRemake } from '../matchStats';
import GameIcon from './GameIcon.vue';
import RuneIcon from './RuneIcon.vue';
import AppIcon from './AppIcon.vue';
import RoleIcon from './RoleIcon.vue';
import MatchSummary from './MatchSummary.vue';

const props = defineProps<{ match: Match; expanded: boolean; compact?: boolean; fixed?: boolean }>();
const emit = defineEmits<{ toggle: [] }>();
const teams = computed(() => [100, 200].map(id => (props.match.roster ?? []).filter(p => p.teamId === id)));
const fullRoster = computed(() => teams.value.every(team => team.length === 5));
const cs = computed(() => perMinute(props.match.cs, props.match.duration));
const role = computed(() => props.match.position || undefined);
</script>

<template>
  <article class="match-card" :class="[isRemake(match) ? 'remake' : match.win == null ? 'unknown' : match.win ? 'won' : 'lost', { 'match-open': expanded }]">
    <component :is="fixed ? 'div' : 'button'" class="match-main" :class="{ 'match-static': fixed }" :aria-expanded="fixed ? undefined : expanded" @click="!fixed && emit('toggle')">
      <span class="match-context">
        <strong>{{ queueName(match.queueId) }}</strong><small>{{ date(match.playedAt) }}</small>
        <b class="outcome">{{ outcome(match) }} <span>{{ duration(match.duration) }}</span></b>
      </span>
      <span class="match-champion">
        <span class="match-hero"><GameIcon :id="match.championId" :size="42"/><small v-if="match.championLevel != null" class="champion-level">{{ match.championLevel }}</small></span>
        <span class="match-spells"><GameIcon kind="spells" :id="match.spell1" :size="19"/><GameIcon kind="spells" :id="match.spell2" :size="19"/></span>
      </span>
      <span class="match-build-summary">
        <span class="match-performance"><strong>{{ kda(match) }}</strong><small>{{ decimal(sampleStats([match]).kda) }} KDA</small><span v-if="match.runeIds?.length || match.runeSecondary" class="match-rune-icons"><RuneIcon v-if="match.runeIds?.[0]" :id="match.runeIds[0]" :size="18"/><RuneIcon v-if="match.runeSecondary" :id="match.runeSecondary" :size="16"/></span></span>
        <span class="match-items" :aria-label="uiText.matchHistoryRow.finalItems"><GameIcon v-for="slot in 7" :key="slot" kind="items" :id="match.items?.[slot - 1]" :size="20"/></span>
      </span>
      <span class="match-efficiency">
        <span v-if="role" class="match-role"><RoleIcon :role="role"/><b>{{ positionName(role) }}</b></span>
        <span>{{ uiText.matchHistoryRow.participation }}<b>{{ percent(match.participation) }}</b></span>
        <span :title="uiText.matchHistoryRow.csHint">{{ uiText.matchHistoryRow.cs }}<b>{{ number(match.cs) }}</b><small>{{ uiText.matchHistoryRow.perMinute(cs == null ? '—' : cs.toFixed(1)) }}</small></span>
        <span>{{ uiText.matchHistoryRow.damage }}<b>{{ number(match.damage) }}</b></span>
        <span v-if="matchBadges(match).length" class="match-facts">{{ matchBadges(match).join(' · ') }}</span>
      </span>
      <span v-if="fullRoster" class="match-mini-roster" :aria-label="uiText.common.roster">
        <span v-for="(team, index) in teams" :key="index" class="roster-team" :class="index ? 'enemy-roster' : 'ally-roster'">
          <span v-for="p in team" :key="p.participantId" :title="p.displayName" :class="{ target: p.target }"><GameIcon :id="p.championId" :size="17"/></span>
        </span>
      </span>
      <span v-else class="match-roster-missing">{{ uiText.matchHistoryRow.missingRoster }}</span>
      <span v-if="!fixed" class="expand-indicator"><AppIcon name="chevron" :size="15"/></span>
    </component>
    <div v-if="expanded" class="match-expanded"><slot name="details"><MatchSummary :match="match"/></slot></div>
  </article>
</template>

<style scoped>
.match-card{container-type:inline-size;min-width:0;border-left:3px solid var(--muted);border-bottom:1px solid var(--border);background:var(--match-bg);margin:0;}.match-card.won{border-left-color:var(--win);background:var(--win-bg);}.match-card.lost{border-left-color:var(--loss);background:var(--loss-bg);}.match-card.remake{border-left-color:var(--muted);background:var(--panel);}
.match-card>.match-main{width:100%;display:grid;grid-template-columns:82px 67px minmax(160px,1fr) 124px 102px 16px;align-items:center;gap:9px;min-height:80px;padding:7px 10px;border:0;border-radius:0;text-align:left;background:none;color:var(--text);font-variant-numeric:tabular-nums;}.match-card>.match-main:hover:not(.match-static){background:color-mix(in srgb,var(--text) 4%,transparent);}.match-card>.match-main.match-static{cursor:default;grid-template-columns:82px 67px minmax(160px,1fr) 124px 102px;}.match-main>span{min-width:0;}
.match-context{display:flex;flex-direction:column;gap:4px;}.match-context>strong{font-size:12px;color:var(--muted);font-weight:600;white-space:nowrap;}.match-context>small{font-size:11px;color:var(--muted);white-space:nowrap;margin:0;}.match-context>.outcome{font-size:12px;font-weight:600;display:flex;align-items:center;gap:7px;margin:0;}.won .outcome{color:var(--win);}.lost .outcome{color:var(--loss);}.remake .outcome{color:var(--muted);}.outcome>span{color:var(--muted);font-size:11px;font-weight:400;}
.match-champion{display:flex;align-items:center;gap:5px;}.match-champion>.match-hero{position:relative;display:flex;flex:none;}.champion-level{position:absolute;right:-2px;bottom:-3px;display:grid;place-items:center;min-width:17px;height:17px;background:var(--text);color:var(--surface);font-size:11px;border-radius:50%;}.match-champion>.match-spells{display:flex;flex-direction:column;gap:3px;}
.match-build-summary{display:flex;flex-direction:column;gap:7px;}.match-build-summary>.match-performance{display:grid;grid-template-columns:auto 1fr;gap:3px 9px;min-width:0;}.match-performance>strong{font-size:14px;line-height:1.25;white-space:nowrap;}.match-build-summary>.match-performance>small{font-size:11px;color:var(--muted);grid-column:1;display:block;margin:0;line-height:1.2;}.match-rune-icons{grid-column:2;grid-row:1/3;display:flex;align-items:center;justify-content:flex-end;gap:4px;}.match-build-summary>.match-items{display:flex;gap:2px;flex-wrap:nowrap;max-width:none;margin:0;}.match-items :deep(.game-icon){flex:none;}
.match-efficiency{display:flex;flex-direction:column;gap:3px;font-size:11px;color:var(--muted);white-space:nowrap;}.match-efficiency b{font-weight:550;color:var(--text);}.match-efficiency small{font-size:11px;color:var(--muted);}.match-role{display:flex;align-items:center;gap:4px;}.match-role .role-icon{width:14px;height:14px;color:var(--muted);}.match-facts{font-size:11px;color:var(--accent);}
.match-mini-roster{display:flex;flex-direction:column;gap:4px;align-items:flex-start;}.roster-team{display:grid;grid-template-columns:repeat(5,17px);gap:3px;padding-left:4px;border-left:2px solid var(--ally);}.roster-team.enemy-roster{border-left-color:var(--enemy);}.roster-team>span{display:flex;position:relative;border-radius:3px;}.roster-team>span.target{outline:1px solid var(--accent);outline-offset:1px;}.match-roster-missing{font-size:11px;color:var(--muted);text-align:center;}.match-card>.match-expanded{padding:0;container-type:inline-size;background:var(--panel);border-top:1px solid var(--border);}.match-open .expand-indicator{transform:rotate(90deg);}.expand-indicator{display:flex;justify-content:center;transition:transform .15s;}.match-main.match-static:hover{background:none;}
@container(max-width:680px){.match-card>.match-main{grid-template-columns:76px 65px minmax(155px,1fr) 108px 16px;gap:8px;}.match-card>.match-main.match-static{grid-template-columns:76px 65px minmax(155px,1fr) 108px;}.match-mini-roster,.match-roster-missing{display:none;}}
@container(max-width:520px){.match-card>.match-main,.match-card>.match-main.match-static{grid-template-columns:76px 65px minmax(150px,1fr) 14px;}.match-efficiency{display:none;}}
.match-card.match-open{overflow:visible;}
</style>
