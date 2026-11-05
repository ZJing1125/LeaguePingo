<script setup lang="ts">
import { uiText } from "../uiText";
import { computed, getCurrentInstance, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import * as Service from '../../bindings/leaguepingo/appservice';
import type { Match, MatchDetail, MatchTimeline as Timeline, Participant } from '../../bindings/leaguepingo/internal/core/models';
import { errorMessage, kda, number, percent, decimal, sampleStats, positionName } from '../presentation';
import { championName } from '../gameAssets';
import { targetPlayer } from '../analysisData';
import { analysisCache } from '../analysisCache';
import { isRemake } from '../matchStats';
import GameIcon from './GameIcon.vue';
import RuneIcon from './RuneIcon.vue';
import MatchSummary from './MatchSummary.vue';
import TeamAnalysis from './TeamAnalysis.vue';
import MatchTimeline from './MatchTimeline.vue';
import MatchBuild from './MatchBuild.vue';

const props = defineProps<{ player: Participant; gameId: number; match?: Match; connected?: boolean; provided?: MatchDetail | null }>();
const loaded = ref<MatchDetail | null>(null), timeline = ref<Timeline | null>(null), busy = ref(false), error = ref('');
const tab = ref('overview');
const scopeId = `analysis-${getCurrentInstance()!.uid}`;
const root = ref<HTMLElement | null>(null);
const tabs = [{ key: 'overview', label: uiText.common.matchOverview }, { key: 'team', label: uiText.common.teamAnalysis }, { key: 'timeline', label: uiText.common.timeline }, { key: 'build', label: uiText.common.buildRunes }];
const key = computed(() => `${props.player.connectionId}:${props.player.key}:${props.gameId}`);
let revision = 0, autoRequested = '';
const detail = computed(() => props.provided ?? loaded.value);
const match = computed(() => targetPlayer(detail.value)?.summary ?? props.match ?? props.player.history?.find(m => m.gameId === props.gameId));
const target = computed(() => targetPlayer(detail.value));
const canLoad = computed(() => !!props.connected && !props.player.stale && !props.provided);
const teams = computed(() => [...new Set((detail.value?.players ?? []).map(p => p.teamId))].sort());
const teamName = (id: number) => id === 100 ? uiText.common.blue : id === 200 ? uiText.common.red : uiText.matchDetails.team(id);
const teamPlayers = (id: number) => (detail.value?.players ?? []).filter(p => p.teamId === id);
const teamResult = (id: number) => {
  const players = teamPlayers(id);
  if (players.some(p => isRemake(p.summary))) return uiText.common.remake;
  const values = players.map(p => p.summary.win).filter(v => v != null);
  return values.length && values.every(v => v === values[0]) ? (values[0] ? uiText.common.win : uiText.common.loss) : '';
};
type TeamMetric = 'kills' | 'gold' | 'damage' | 'damageTaken' | 'cs' | 'visionScore';
function teamTotal(id: number, field: TeamMetric) {
  const players = teamPlayers(id), values = players.map(p => p.summary[field]);
  return players.length && values.every(v => v != null) ? values.reduce<number>((sum, value) => sum + (value ?? 0), 0) : null;
}
const teamObjective = (id: number) => detail.value?.teams?.find(team => team.teamId === id);
function damageWidth(damage: number | null | undefined) { const max = Math.max(0, ...(detail.value?.players ?? []).map(p => p.summary.damage ?? 0)); return damage != null && max > 0 ? Math.max(0, Math.min(100, damage * 100 / max)) : 0; }
function csRate(value: number | null | undefined, duration: number | null | undefined) { return value != null && duration && duration > 0 ? uiText.matchDetails.perMinute(decimal(value * 60 / duration)) : '—'; }

watch(key, () => {
  revision++;
  loaded.value = analysisCache.detail(key.value);
  timeline.value = props.provided ? null : analysisCache.timeline(key.value);
  busy.value = false;
  error.value = '';
  tab.value = 'overview';
  autoRequested = '';
}, { immediate: true });
onBeforeUnmount(() => { revision++; });
watch(() => props.connected, () => { if (!props.connected) { revision++; busy.value = false; } });
function moveTab(event: KeyboardEvent, index: number) { let next = index; if (event.key === 'ArrowRight') next = (index + 1) % tabs.length; else if (event.key === 'ArrowLeft') next = (index + tabs.length - 1) % tabs.length; else if (event.key === 'Home') next = 0; else if (event.key === 'End') next = tabs.length - 1; else return; event.preventDefault(); tab.value = tabs[next]!.key; (event.currentTarget as HTMLElement).parentElement?.querySelectorAll<HTMLButtonElement>('[role=tab]')[next]?.focus(); }
async function load(kind: 'detail' | 'timeline', scroll = true) {
  if (busy.value || !canLoad.value) return;
  const request = ++revision, requestKey = key.value; busy.value = true; error.value = '';
  try {
    if (kind === 'timeline') {
      const result = await Service.MatchTimeline(props.player.key, props.player.connectionId, props.gameId);
      if (request !== revision || requestKey !== key.value) return;
      timeline.value = result; analysisCache.setTimeline(requestKey, result);
      if (result.detail) { loaded.value = result.detail; analysisCache.setDetail(requestKey, result.detail); }
    } else {
      const result = await Service.MatchDetail(props.player.key, props.player.connectionId, props.gameId);
      if (request !== revision || requestKey !== key.value) return;
      loaded.value = result; analysisCache.setDetail(requestKey, result);
    }
    if (request === revision && scroll) { await nextTick(); root.value?.scrollIntoView({ block: 'start', behavior: 'auto' }); }
  } catch (reason) { if (request === revision) error.value = errorMessage(reason); }
  finally { if (request === revision) busy.value = false; }
}
watch([key, canLoad, detail], ([requestKey, allowed, current]) => {
  if (!allowed || current || autoRequested === requestKey) return;
  autoRequested = requestKey;
  void load('detail', false);
}, { immediate: true });
</script>

<template>
  <section ref="root" class="full-match-detail">
    <div class="analysis-tabs" role="tablist" :aria-label="uiText.matchDetails.title"><button v-for="(item, i) in tabs" :key="item.key" role="tab" :id="`${scopeId}-${gameId}-${item.key}`" :aria-selected="tab === item.key" :aria-controls="`${scopeId}-panel-${gameId}`" :tabindex="tab === item.key ? 0 : -1" @click="tab = item.key" @keydown="moveTab($event, i)">{{ item.label }}</button></div>
    <p v-if="error" class="error-banner" role="alert">{{ error }}</p>
    <div :id="`${scopeId}-panel-${gameId}`" role="tabpanel" :aria-labelledby="`${scopeId}-${gameId}-${tab}`" class="analysis-tab-content">
      <template v-if="tab === 'overview'">
        <div v-if="detail" class="scoreboard-content">
          <header class="scoreboard-heading"><div><h3>{{ uiText.matchDetails.scoreboard }}</h3><p>{{ uiText.matchDetails.comparisonHint }}</p></div><span>{{ uiText.matchDetails.playerCount(detail.players?.length ?? 0) }}<span v-if="detail.partial"> · {{ (detail.players?.length ?? 0) < 10 ? uiText.matchDetails.partialRoster : uiText.matchDetails.incompleteTotals }}</span></span></header>
          <div class="detail-table-scroll" tabindex="0" :aria-label="uiText.matchDetails.scoreboardLabel">
            <div class="scoreboard-board">
              <section v-for="team in teams" :key="team" class="scoreboard-team" :class="{ 'team-blue': team === 100, 'team-red': team === 200 }">
                <header class="team-summary">
                  <div class="team-result"><b :class="{ victory: teamResult(team) === uiText.common.win, defeat: teamResult(team) === uiText.common.loss }">{{ teamResult(team) || uiText.matchDetails.pendingOutcome }}</b><strong>{{ teamName(team) }}</strong><span>{{ detail.partial ? uiText.matchDetails.knownKills : uiText.common.kills }} {{ number(teamTotal(team, 'kills')) }}</span></div>
                  <dl><div><dt>{{ uiText.matchDetails.gold }}</dt><dd>{{ number(teamTotal(team, 'gold')) }}</dd></div><div><dt>{{ uiText.common.damage }}</dt><dd>{{ number(teamTotal(team, 'damage')) }}</dd></div><div><dt>{{ uiText.common.cs }}</dt><dd>{{ number(teamTotal(team, 'cs')) }}</dd></div><div><dt>{{ uiText.common.vision }}</dt><dd>{{ number(teamTotal(team, 'visionScore')) }}</dd></div></dl>
                  <p v-if="teamObjective(team)" class="objective-line"><span>{{ uiText.matchDetails.towers(number(teamObjective(team)?.towers)) }}</span><span>{{ uiText.matchDetails.dragons(number(teamObjective(team)?.dragons)) }}</span><span>{{ uiText.matchDetails.barons(number(teamObjective(team)?.barons)) }}</span><span>{{ uiText.matchDetails.inhibitors(number(teamObjective(team)?.inhibitors)) }}</span><span v-if="teamObjective(team)?.heralds != null">{{ uiText.matchDetails.heralds(number(teamObjective(team)?.heralds)) }}</span><span v-if="teamObjective(team)?.hordes != null">{{ uiText.matchDetails.grubs(number(teamObjective(team)?.hordes)) }}</span></p>
                </header>
                <table>
                  <thead><tr><th scope="col">{{ uiText.matchDetails.playerChampion }}</th><th scope="col">{{ uiText.matchDetails.kdaParticipation }}</th><th scope="col">{{ uiText.matchDetails.damageTaken }}</th><th scope="col" :title="uiText.matchDetails.csHint">{{ uiText.matchDetails.csGold }}</th><th scope="col">{{ uiText.common.vision }}</th><th scope="col">{{ uiText.common.finalItems }}</th></tr></thead>
                  <tbody><tr v-for="p in teamPlayers(team)" :key="p.participantId" :class="{ target: p.target }">
                    <td><div class="scoreboard-player"><span class="scoreboard-hero"><GameIcon :id="p.summary.championId" :size="34"/><small v-if="p.summary.championLevel != null">{{ p.summary.championLevel }}</small></span><span class="scoreboard-systems"><GameIcon kind="spells" :id="p.summary.spell1" :size="15"/><GameIcon kind="spells" :id="p.summary.spell2" :size="15"/><RuneIcon v-if="p.summary.runeIds?.[0]" :id="p.summary.runeIds[0]" :size="15"/><RuneIcon v-if="p.summary.runeSecondary" :id="p.summary.runeSecondary" :size="14"/></span><span class="scoreboard-name"><strong :title="p.displayName">{{ p.displayName }}</strong><small>{{ championName(p.summary.championId).split('·').at(-1) }}<span v-if="p.summary.position"> · {{ positionName(p.summary.position) }}</span><span v-if="p.target" class="target-label">{{ uiText.matchDetails.currentPlayer }}</span></small></span></div></td>
                    <td class="scoreboard-kda"><b>{{ kda(p.summary) }}</b><small>{{ uiText.matchDetails.kdaRate(decimal(sampleStats([p.summary]).kda), percent(p.participation)) }}</small></td>
                    <td><b>{{ number(p.summary.damage) }}</b><span class="damage-track" :title="uiText.matchDetails.damageHint"><i :style="{ width: `${damageWidth(p.summary.damage)}%` }"/></span><small>{{ uiText.matchDetails.taken(number(p.summary.damageTaken)) }}</small></td>
                    <td><b>{{ uiText.matchDetails.csTotal(number(p.summary.cs)) }}</b><small>{{ uiText.matchDetails.economy(csRate(p.summary.cs, p.summary.duration), number(p.summary.gold)) }}</small></td>
                    <td><b>{{ number(p.summary.visionScore) }}</b><small>{{ uiText.matchDetails.wards(number(p.summary.wardsPlaced), number(p.summary.wardsKilled)) }}</small><small>{{ uiText.matchDetails.controlWards(number(p.summary.controlWards)) }}</small></td>
                    <td><span class="detail-loadout"><GameIcon v-for="slot in 7" :key="slot" kind="items" :id="p.summary.items?.[slot - 1]" :size="20"/></span></td>
                  </tr></tbody>
                </table>
              </section>
            </div>
          </div>
          <details class="scoreboard-note"><summary>{{ uiText.common.dataNotes }}</summary><p>{{ uiText.matchDetails.dataHelp }}</p></details>
        </div>
        <div v-else-if="busy" class="scoreboard-loading" role="status"><header><strong>{{ uiText.matchDetails.loadingScoreboard }}</strong><span>{{ uiText.matchDetails.clickedOnly }}</span></header><div v-for="row in 10" :key="row"><i/><span/><span/><span/></div></div>
        <template v-else><div class="personal-content"><MatchSummary v-if="match" :match="match"/></div><div class="analysis-load"><span>{{ canLoad ? uiText.matchDetails.scoreboardFailed : uiText.matchDetails.disconnected }}</span><button v-if="canLoad" class="secondary" :disabled="busy" @click="load('detail')">{{ uiText.matchDetails.retryScoreboard }}</button></div></template>
      </template>
      <template v-else-if="tab === 'team'"><TeamAnalysis v-if="detail" :detail="detail"/><div v-else class="analysis-load"><span>{{ busy ? uiText.matchDetails.loadingScoreboardShort : uiText.matchDetails.teamAnalysisHint }}</span><button v-if="canLoad && !busy" class="secondary" @click="load('detail')">{{ uiText.matchDetails.retryScoreboard }}</button></div></template>
      <template v-else-if="tab === 'timeline'"><MatchTimeline v-if="timeline && detail" :timeline="timeline" :detail="detail"/><div v-else class="analysis-load"><span>{{ provided ? uiText.matchDetails.archivedTimeline : uiText.matchDetails.timelineHint }}</span><button v-if="!provided" class="secondary" :disabled="busy || !canLoad" @click="load('timeline')">{{ busy ? uiText.matchDetails.loadingTimeline : uiText.matchDetails.loadTimeline }}</button></div></template>
      <template v-else-if="tab === 'build'"><MatchBuild v-if="match" :match="match" :timeline="timeline" :participant-id="target?.participantId"/><div v-if="!timeline && !provided" class="analysis-load"><span>{{ uiText.matchDetails.buildHint }}</span><button class="secondary" :disabled="busy || !canLoad" @click="load('timeline')">{{ busy ? uiText.matchDetails.loadingTimeline : uiText.matchDetails.loadTimeline }}</button></div></template>
    </div>
    <details class="detail-note"><summary>{{ uiText.common.statistics }}</summary><p>{{ uiText.matchDetails.matchStatistics(gameId) }}</p></details>
  </section>
</template>

<style scoped>
.full-match-detail{min-width:0;}.scoreboard-content{padding:14px;}.scoreboard-heading{display:flex;justify-content:space-between;align-items:flex-start;gap:18px;margin-bottom:12px;}.scoreboard-heading h3{font-size:15px;margin:0 0 4px;}.scoreboard-heading p{font-size:11px;color:var(--muted);margin:0;}.scoreboard-heading>span{font-size:12px;color:var(--muted);white-space:nowrap;}
.detail-table-scroll{overflow-x:auto;border:1px solid var(--border);border-radius:6px;}.scoreboard-board{min-width:820px;}.scoreboard-team+.scoreboard-team{border-top:8px solid var(--ground);}.team-summary{display:grid;grid-template-columns:180px 1fr;align-items:center;gap:8px 20px;padding:9px 11px;background:color-mix(in srgb,var(--ally) 9%,var(--raised));border-left:3px solid var(--ally);}.team-red .team-summary{background:color-mix(in srgb,var(--enemy) 9%,var(--raised));border-left-color:var(--enemy);}.team-result{display:flex;align-items:baseline;gap:8px;font-size:12px;}.team-result b{font-size:13px;}.team-result strong{font-size:13px;}.team-result span{color:var(--muted);}.victory{color:var(--win);}.defeat{color:var(--loss);}.team-summary dl{display:grid;grid-template-columns:repeat(4,minmax(78px,1fr));gap:14px;margin:0;}.team-summary dl>div{display:flex;align-items:baseline;justify-content:flex-end;gap:6px;}.team-summary dt{font-size:11px;color:var(--muted);}.team-summary dd{font-size:13px;font-weight:600;margin:0;font-variant-numeric:tabular-nums;}.objective-line{grid-column:1/-1;display:flex;gap:15px;flex-wrap:wrap;margin:0;padding-top:7px;border-top:1px solid var(--border);font-size:11px;color:var(--muted);}
table{width:100%;border-collapse:collapse;table-layout:fixed;font-size:12px;font-variant-numeric:tabular-nums;}th,td{padding:7px 8px;border-bottom:1px solid var(--border);vertical-align:middle;text-align:right;}tbody tr:last-child td{border-bottom:0;}th{font-size:11px;font-weight:500;color:var(--muted);background:var(--panel);}th:first-child,td:first-child{text-align:left;width:25%;}th:nth-child(2){width:14%;}th:nth-child(3){width:14%;}th:nth-child(4){width:14%;}th:nth-child(5){width:11%;}th:nth-child(6){width:22%;text-align:left;}td:nth-child(6){text-align:left;}td b{font-weight:600;}td>small,.scoreboard-kda small{display:block;font-size:11px;color:var(--muted);margin-top:4px;line-height:1.25;}
.scoreboard-player{display:flex;align-items:center;gap:6px;min-width:0;}.scoreboard-hero{position:relative;display:flex;flex:none;}.scoreboard-hero>small{position:absolute;right:-2px;bottom:-2px;display:grid;place-items:center;min-width:15px;height:15px;border-radius:50%;background:var(--text);color:var(--surface);font-size:10px;}.scoreboard-systems{display:grid;grid-template-columns:repeat(2,15px);gap:2px;flex:none;}.scoreboard-name{min-width:0;}.scoreboard-name strong{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px;font-weight:600;}.scoreboard-name small{display:block;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;font-size:11px;color:var(--muted);margin-top:3px;}.target{background:var(--selection);}.target-label{color:var(--accent);}.detail-loadout{display:flex;gap:2px;margin:0;}.detail-loadout :deep(.game-icon){flex:none;}.damage-track{display:block;margin:5px 0 0;height:3px;background:var(--border);overflow:hidden;}.damage-track i{display:block;height:100%;background:var(--ally);}.team-red .damage-track i{background:var(--enemy);}
.scoreboard-note{font-size:11px;color:var(--muted);margin-top:10px;line-height:1.6;}.scoreboard-note summary{cursor:pointer;}.scoreboard-note p{margin:7px 0 0;}.detail-note{padding:10px 16px;font-size:11px;color:var(--muted);line-height:1.6;border-top:1px solid var(--border);}.detail-note summary{cursor:pointer;}.detail-note p{margin:8px 0 0;}.analysis-tabs{position:sticky;top:0;z-index:3;display:flex;gap:3px;padding:5px;background:var(--raised);border-block:1px solid var(--border);}.analysis-tabs button{flex:1;min-height:32px;border:0;background:none;border-radius:4px;font-size:12px;color:var(--muted);}.analysis-tabs button[aria-selected=true]{background:var(--panel);color:var(--text);box-shadow:inset 0 -2px var(--accent);}.analysis-tab-content{min-height:80px;}.personal-content{padding:16px;}.analysis-load{display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap;padding:14px 16px;border-top:1px solid var(--border);font-size:12px;color:var(--muted);}.analysis-load button{flex:none;font-size:12px;}.full-match-detail .error-banner{margin:12px 16px;}
.scoreboard-loading{padding:14px 16px;}.scoreboard-loading header{display:flex;align-items:baseline;justify-content:space-between;margin-bottom:10px;}.scoreboard-loading header strong{font-size:14px;}.scoreboard-loading header span{font-size:11px;color:var(--muted);}.scoreboard-loading>div{height:44px;display:grid;grid-template-columns:36px 1.4fr .8fr 1fr;gap:10px;align-items:center;border-top:1px solid var(--border);}.scoreboard-loading i,.scoreboard-loading>div>span{display:block;height:9px;background:var(--raised);border-radius:2px;}.scoreboard-loading i{width:30px;height:30px;}
@container(max-width:620px){.scoreboard-heading p{display:none;}.scoreboard-content{padding:10px;}.analysis-tabs button{font-size:11px;}.scoreboard-board{min-width:760px;}}
</style>
