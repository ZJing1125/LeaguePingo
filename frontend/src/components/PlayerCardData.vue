<script setup lang="ts">
import { analysisSummary, analysisTitle } from "../playerAnalysis";
import { uiText } from "../uiText";
import { computed } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import { percent, decimal, date, tierName } from "../presentation";
import { rankPresentation } from "../rankPresentation";
import { championName } from "../gameAssets";
import { scoutStats, roleNames } from "../scoutStats";
import GameIcon from "./GameIcon.vue";
import AppIcon from "./AppIcon.vue";
import RankEmblem from "./RankEmblem.vue";
import ScoutRing from "./ScoutRing.vue";
const props = defineProps<{ player: Participant; frozen?: boolean; party?: boolean; queue?: number; captured?: boolean }>();
const data = computed(() => scoutStats(props.player, props.queue ?? 420));
const rank = computed(() => rankPresentation(props.player, props.queue === 440 ? "RANKED_FLEX_SR" : "RANKED_SOLO_5x5"));
const season = computed(() => rank.value.record);
const statisticsPending = computed(() => props.player.identityState === "known" && !props.captured &&
  data.value.report?.state === "loading" && !data.value.report.read);
const hero = computed(() => props.player.championId ? championName(props.player.championId).split("·").at(-1) : props.party || props.player.championState === "unselected" ? uiText.playerCardData.unselected : uiText.playerCardData.unknownChampion);
const heroScope = computed(() => !data.value.ranked ? uiText.modeScouting.hero30 : props.player.championState === "intent" ? uiText.playerCardData.intent : props.player.championId ? data.value.report?.season ? data.value.report.complete ? uiText.playerCardData.seasonRanked : uiText.playerCardData.seasonRead : uiText.playerCardData.championPerformance  : props.party ? uiText.common.party : uiText.playerCardData.waitingSelection);
const heroStatus = computed(() => {
  if (!props.player.championId) return props.party || props.player.championState === "unselected" ? uiText.playerCardData.waitingHeroSelection : uiText.playerCardData.waitingHeroInfo;
  const report = data.value.report;
  if (!report || report.state === "loading") return props.captured ? uiText.playerCardData.incompleteCaptured : uiText.playerCardData.preparingChampion;
  if (!data.value.ranked) return data.value.heroComplete ? uiText.modeScouting.noRecords : uiText.modeScouting.partial;
  if (!report.season) return uiText.playerCardData.pendingSeason;
  if (data.value.champion.games) return uiText.playerCardData.missingKda;
  return report.complete ? uiText.playerCardData.emptySeason : uiText.playerCardData.incompleteSeason;
});
const range = computed(() => {
  const report = data.value.report;
  if (!report) return props.captured ? uiText.playerCardData.incompleteStatistics : uiText.playerCardData.preparing;
  if (report.retained) return uiText.playerCardData.previous;
  if (report.state === "loading") return uiText.playerCardData.preparing;
  if (report.complete30) return uiText.playerCardData.last30Days;
  return report.read ? uiText.playerCardData.read30Days(data.value.overall.games) : uiText.playerCardData.pending30Days;
});
const roleCaption = computed(() => ({ [uiText.playerCardData.roleMissingKey]: uiText.common.noRecords, [uiText.playerCardData.roleSparseKey]: uiText.playerCardData.sparse, [uiText.playerCardData.roleSpreadKey]: uiText.playerCardData.spread }[data.value.role.label] || data.value.role.label));
const shortRank = computed(() => rank.value.rank ? rank.value.label : rank.value.label.includes(uiText.common.loss) ? uiText.common.queryFailed : rank.value.label.includes(uiText.playerCardData.timeoutToken) ? uiText.common.queryTimeout : uiText.common.pendingRank);
const roleTitle = computed(() => uiText.playerCardData.roleHint(data.value.queueLabel, data.value.role.known, data.value.overall.games, data.value.role.positions.map(p => uiText.playerCardData.roleCount(roleNames[p.position], p.games)).join('，')));
function periodCaption(p: ReturnType<typeof scoutStats>["periods"][number]) {
  if (!data.value.report || !p.games && data.value.report.state === "loading") return props.captured ? uiText.common.notRetained : uiText.common.preparing;
  if (data.value.arena) return uiText.arena.samples(p.arena?.placementGames ?? 0);
  return p.outcomeGames ? uiText.playerCardData.validGames(p.outcomeGames) : p.games ? uiText.playerCardData.readGames(p.games) : p.complete ? uiText.playerCardData.zeroGames : uiText.modeScouting.noSample;
}
function periodDetail(p: ReturnType<typeof scoutStats>["periods"][number]) {
  if (!p.games) return p.complete ? uiText.sidebarPlayer.zeroGames : data.value.report?.state === "loading" || !data.value.report ? "—" : uiText.common.incompleteRange;
  if (data.value.arena) return p.arena?.averagePlace != null ? uiText.arena.average(decimal(p.arena.averagePlace)) : uiText.arena.pending;
  return p.outcomeGames ? uiText.playerCardData.outcomes(p.wins, p.losses) : uiText.playerCardData.missingOutcome;
}
const heroArena = computed(() => data.value.champion.arena);
const arenaTitle = (p: ReturnType<typeof scoutStats>["periods"][number]) => `${uiText.arena.hint(p.arena?.placementGames ?? 0, p.games)} · ${uiText.arena.topHalf}: ${percent(p.arena?.topHalfRate)} · ${uiText.arena.topHalfHint(p.arena?.topHalfGames ?? 0)}`;
</script>
<template>
  <span class="card-details scout-data">
    <span class="scout-headline" :style="data.ranked ? undefined : {gridTemplateColumns: 'minmax(0,3fr) minmax(0,2fr)',gridTemplateRows:'minmax(0,1fr)'}">
      <span class="scout-champion">
        <span class="card-pick"><GameIcon v-if="player.championId" :id="player.championId" :size="32" /><AppIcon v-else name="match" :size="28" /><span><strong :title="hero">{{ hero }}</strong><small :title="heroScope">{{ heroScope }}</small></span></span>
        <span v-if="data.champion.average" class="scout-kda" :title="uiText.playerCardData.championAverage(data.queueLabel, data.champion.kdaGames)"><b>{{ data.champion.average[0]!.toFixed(1) }}</b> / <b>{{ data.champion.average[1]!.toFixed(1) }}</b> / <b>{{ data.champion.average[2]!.toFixed(1) }}</b></span>
        <span v-else class="scout-no-kda" :title="heroStatus">{{ heroStatus }}</span>
        <small v-if="data.arena" class="scout-hero-rate" :title="uiText.arena.hint(heroArena?.placementGames ?? 0, data.champion.games)">{{ heroArena?.firstPlaceRate != null ? uiText.arena.first(percent(heroArena.firstPlaceRate)) : uiText.arena.pending }}<span v-if="heroArena?.averagePlace != null"> · {{ uiText.arena.average(decimal(heroArena.averagePlace)) }}</span></small>
        <small v-else class="scout-hero-rate" :class="{ 'no-sample': !data.champion.outcomeGames }">{{ data.champion.outcomeGames ? percent(data.champion.winRate) : data.ranked && data.report?.season ? uiText.playerCardData.seasonQueue(data.queueLabel) : data.queueLabel }}<span v-if="data.champion.outcomeGames">{{ uiText.playerCardData.championOutcomes(data.champion.wins, data.champion.losses) }}</span></small>
      </span>
      <span v-if="data.ranked" class="scout-rank" :title="`${data.queueLabel} · ${rank.title}`">
        <span class="scout-rank-heading"><RankEmblem :tier="player.identityState === 'known' ? rank.rank?.tier : undefined" /><small class="scout-queue-label">{{ data.ranked ? data.queueLabel : uiText.modeScouting.reference }}</small></span>
        <span class="rank-line"><b>{{ shortRank }}</b><small v-if="rank.retained">{{ uiText.common.old }}</small></span>
        <small v-if="data.ranked" class="scout-season" :title="season?.title"><span v-if="season">{{ season.scope }} {{ percent(season.rate) }}</span><span>{{ season ? uiText.playerCardData.outcomes(season.wins, season.losses) : uiText.common.pendingOutcome }}</span></small>
      </span>
    <small v-if="!data.ranked" class="mode-rank-reference" :title="rank.title"><span>{{ uiText.modeScouting.reference }}</span><span>{{ uiText.common.solo }}</span><span>{{ rank.rank ? tierName(rank.rank.tier) + ' ' + (rank.rank.division || '') : rank.label }}</span><span v-if="rank.rank?.lp != null">{{ rank.rank.lp }} LP</span></small>
    </span>
    <template v-if="player.identityState === 'known'">
      <span v-if="statisticsPending" class="scout-stat-skeleton" role="status" :aria-label="uiText.playerCardData.fetchingStatistics">
        <span class="scout-skeleton-rings" aria-hidden="true"><i v-for="slot in 3" :key="slot" /></span>
        <i class="scout-skeleton-line wide" aria-hidden="true" />
        <i class="scout-skeleton-line" aria-hidden="true" />
        <small>{{ uiText.playerCardData.preparing }}</small>
      </span>
      <template v-else>
        <span class="overview-metrics scout-rings" :style="data.roles ? undefined : {gridTemplateColumns: 'repeat(2,minmax(0,1fr))'}" :title="uiText.playerCardData.independentScope(data.queueLabel, date(data.report?.fetchedAt))">
          <ScoutRing :value="data.arena ? data.periods[0]!.arena?.firstPlaceRate ?? null : data.periods[0]!.winRate" :measure="data.arena ? uiText.arena.firstRate : undefined" short="12h" :label="uiText.playerCardData.last12Hours" :caption="periodCaption(data.periods[0]!)" :detail="periodDetail(data.periods[0]!)" :title="data.arena ? arenaTitle(data.periods[0]!) : uiText.playerCardData.last12HoursHint(data.queueLabel, data.periods[0]!.outcomeGames, data.periods[0]!.games, data.periods[0]!.complete ? '' : uiText.playerCardData.readOnly)" />
          <span v-if="data.roles" class="scout-position-cell">
            <ScoutRing :value="data.role.share" :role="data.role.key" short="" :label="uiText.playerCardData.role30Days" :caption="roleCaption" :detail="data.role.known ? uiText.playerCardData.recordedGames(data.role.known) : '—'" :title="roleTitle" />
            <span v-if="data.role.known" class="role-distribution" :title="roleTitle"><i v-for="p in data.role.positions" :key="p.position" :class="p.position.toLowerCase()" :style="{ flexGrow: p.games }" :title="uiText.playerCardData.roleCount(roleNames[p.position], p.games)" /></span>
          </span>
          <ScoutRing :value="data.arena ? data.periods[1]!.arena?.firstPlaceRate ?? null : data.periods[1]!.winRate" :measure="data.arena ? uiText.arena.firstRate : undefined" short="30d" :label="uiText.playerCardData.within30Days" :caption="periodCaption(data.periods[1]!)" :detail="periodDetail(data.periods[1]!)" :title="data.arena ? arenaTitle(data.periods[1]!) : uiText.playerCardData.last30DaysHint(data.queueLabel, data.periods[1]!.outcomeGames, data.periods[1]!.games, data.periods[1]!.complete ? '' : uiText.playerCardData.readOnly)" />
        </span>
        <span class="scout-scope" :title="uiText.playerCardData.cutoffHint(data.queueLabel, range, data.overall.games, data.overall.kdaGames ?? 0, date(data.report?.fetchedAt), data.report?.season?.name || uiText.playerCardData.missingSeason)"><small class="sample-size">{{ range }}</small><span v-if="data.arena" :title="uiText.arena.topHalfHint(data.overall.arena?.topHalfGames ?? 0)">{{ uiText.arena.topHalf }} <b>{{ percent(data.overall.arena?.topHalfRate) }}</b></span><span v-else-if="data.analysis?.score != null" :title="analysisTitle(data.analysis)">{{ analysisSummary(data.analysis) }}</span><span v-else>KDA <b>{{ decimal(data.overall.kda) }}</b></span><span v-if="data.overall.remakes">{{ uiText.modeScouting.remakes(data.overall.remakes) }}</span><span v-if="data.report && !data.report.complete30" class="scout-coverage-mark">{{ uiText.common.incompleteRange }}</span></span>
        <span class="scout-tags recent-block"><span v-for="tag in data.tags" :key="tag.label" class="scout-tag" :class="tag.tone" :title="tag.title">{{ tag.label }}</span><small v-if="!data.tags.length" class="scout-empty-tags">{{ !data.ranked ? uiText.modeScouting.hero30 : data.report?.state === 'loading' || !data.report ? captured ? uiText.playerCardData.capturedRanked : uiText.playerCardData.preparingRanked : uiText.playerCardData.seasonChampion }}</small></span>
      </template>
    </template>
    <span v-else class="rich-state scout-state"><AppIcon name="history" :size="24" /><b>{{ uiText.playerCardData.missingIdentity }}</b><small>{{ uiText.playerCardData.waitingIdentity }}</small></span>
  </span>
</template>
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.card-details.scout-data):is(.scout-card .card-details.scout-data) { position: relative; z-index: 2; display: flex; flex-direction: column; justify-content: flex-start; flex: 1; min-height: 0; width: 100%; padding: 0; gap: 0; text-shadow: none; color: var(--scout-text); }
:where(.scout-headline):is(.scout-headline) { display: grid; grid-template-columns: 1fr 1fr; height: 90px; min-height: 90px; background: var(--scout-cell); border-bottom: 1px solid var(--scout-border); }
:where(.scout-champion):is(.scout-champion) { display: flex; flex-direction: column; min-width: 0; padding: 7px 7px 5px; gap: 4px; border-right: 1px solid var(--scout-border); }
:where(.card-pick):is(.scout-champion .card-pick) { display: flex; align-items: center; gap: 5px; min-width: 0; }
:where(.card-pick):is(.scout-champion .card-pick) :deep(>span) { min-width: 0; display: flex; flex-direction: column; gap: 3px; }
:where(.card-pick):is(.scout-champion .card-pick) :deep(strong) { display: block; font-size: 13px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
:where(.card-pick):is(.scout-champion .card-pick) :deep(small) { display: block; font-size: 11px; color: var(--scout-muted); white-space: nowrap; }
:where(.scout-kda):is(.scout-kda) { font-size: 12px; white-space: nowrap; line-height: 13px; }
:where(.scout-kda):is(.scout-kda) :deep(b) { font-size: 13px; font-weight: 650; color: var(--scout-good); }
:where(.scout-kda):is(.scout-kda) :deep(b:nth-child(2)) { color: var(--scout-bad); }
:where(.scout-kda):is(.scout-kda) :deep(b:nth-child(3)) { color: var(--scout-warning); }
:where(.scout-no-kda):is(.scout-no-kda) { font-size: 11px; color: var(--scout-muted); white-space: nowrap; }
:where(.scout-hero-rate):is(.scout-hero-rate) { font-size: 12px; color: var(--scout-good); white-space: nowrap; }
:where(.scout-hero-rate.no-sample):is(.scout-hero-rate.no-sample) { color: var(--scout-muted); }
:where(.scout-hero-rate):is(.scout-hero-rate) :deep(>span) { color: var(--scout-muted); font-size: 11px; }
:where(.scout-rank):is(.scout-rank) { display: flex; flex-direction: column; justify-content: center; align-items: center; min-width: 0; gap: 2px; padding: 3px 4px 5px; text-align: center; }
:where(.scout-rank-heading):is(.scout-rank-heading) { display: flex; align-items: center; justify-content: center; gap: 4px; line-height: 12px; }
:where(.scout-queue-label):is(.scout-queue-label) { font-size: 11px; line-height: 12px; }
:where(.rank-line):is(.scout-card .scout-rank .rank-line) { display: flex; align-items: center; justify-content: center; gap: 3px; min-height: 14px; width: 100%; font-size: 11px; white-space: normal; }
:where(.rank-line):is(.scout-card .scout-rank .rank-line) :deep(>b) { flex: initial; color: var(--scout-text); font-weight: 600; font-size: 12px; line-height: 13px; overflow: visible; white-space: normal; }
:where(.rank-line):is(.scout-card .scout-rank .rank-line) :deep(small) { font-size: 11px; color: var(--scout-warning); }
:where(.scout-season):is(.scout-season) { display: flex; flex-direction: column; align-items: center; font-size: 11px; color: var(--scout-muted); line-height: 12px; white-space: nowrap; }
:where(.overview-metrics.scout-rings):is(.scout-card .overview-metrics.scout-rings) { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); align-items: start; height: 98px; min-height: 98px; padding: 7px 3px 3px; gap: 0; }
:where(.scout-scope):is(.scout-scope) { display: flex; align-items: center; justify-content: center; gap: 8px; height: 22px; min-height: 22px; color: var(--scout-muted); font-size: 11px; }
:where(.sample-size):is(.scout-scope .sample-size) { font-size: 11px; white-space: nowrap; }
:where(.scout-scope):is(.scout-scope) :deep(b) { color: var(--scout-text); font-size: 11px; font-weight: 600; }
:where(.scout-tags.recent-block):is(.scout-card .scout-tags.recent-block) { display: flex; flex-direction: row; align-items: flex-start; align-content: center; justify-content: center; flex-wrap: wrap; gap: 3px 4px; height: 40px; min-height: 40px; padding: 4px 6px; border-top: 1px solid var(--scout-border); overflow: hidden; }
:where(.scout-tag):is(.scout-tag) { display: inline-block; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; border: 1px solid currentColor; padding: 1px 4px; border-radius: 3px; color: var(--scout-muted); font-size: 11px; line-height: 13px; }
:where(.scout-tag.positive):is(.scout-tag.positive) { color: var(--scout-good); background: color-mix(in srgb, var(--scout-good) 7%, transparent); }
:where(.scout-tag.accent):is(.scout-tag.accent) { color: var(--scout-accent); background: color-mix(in srgb, var(--scout-accent) 7%, transparent); }
:where(.rich-state.scout-state):is(.scout-card .rich-state.scout-state) { display: flex; flex-direction: column; align-items: center; justify-content: center; flex: 1; min-height: 0; gap: 10px; padding: 12px; color: var(--scout-muted); text-align: center; }
:where(.rich-state.scout-state):is(.scout-card .rich-state.scout-state) :deep(b) { font-size: 15px; color: var(--scout-text); }
:where(.rich-state.scout-state):is(.scout-card .rich-state.scout-state) :deep(small) { font-size: 11px; line-height: 16px; }
:where(.rich-state.scout-state.failed):is(.scout-card .rich-state.scout-state.failed) :deep(>b) { color: var(--scout-warning); }
:where(.scout-headline):is(.overlay-shell .scout-headline) { height: 70px; min-height: 70px; }
:where(.scout-champion):is(.overlay-shell .scout-champion) { padding: 4px; gap: 3px; }
:where(.card-pick):is(.overlay-shell .scout-champion .card-pick) { gap: 4px; }
:where(.card-pick):is(.overlay-shell .scout-champion .card-pick) :deep(strong) { font-size: 12px; }
:where(.scout-rank):is(.overlay-shell .scout-rank) { padding-block: 2px; }
:where(.rank-line):is(.overlay-shell .scout-card .scout-rank .rank-line) :deep(>b) { font-size: 12px; }
:where(.scout-hero-rate):is(.overlay-shell .scout-hero-rate) :deep(>span) { display: none; }
:where(.overview-metrics.scout-rings):is(.overlay-shell .scout-card .overview-metrics.scout-rings) { height: 82px; min-height: 82px; padding-block: 4px 0; }
:where(.scout-scope):is(.overlay-shell .scout-scope) { min-height: 18px; height: 18px; gap: 6px; }
:where(.sample-size):is(.overlay-shell .scout-scope .sample-size) { font-size: 11px; }
:where(.scout-scope):is(.overlay-shell .scout-scope) :deep(b) { font-size: 11px; }
:where(.scout-scope):is(.overlay-shell .scout-scope) :deep(>span) { font-size: 11px; }
:where(.scout-tags.recent-block):is(.overlay-shell .scout-card .scout-tags.recent-block) { flex: 1; min-height: 22px; height: 22px; padding: 2px 4px; flex-wrap: nowrap; align-items: center; }
:where(.scout-tag):is(.overlay-shell .scout-tags .scout-tag) { max-width: calc(50% - 2px); font-size: 11px; }
:where(.scout-tag:nth-child(n+3)):is(.overlay-shell .scout-tags .scout-tag:nth-child(n+3)) { display: none; }
@media (max-width: 680px) {
  :where(.scout-headline):is(.overlay-shell .scout-headline) { height: auto; min-height: 84px; flex-shrink: 0; }
}
:where(.scout-coverage-mark):is(.scout-coverage-mark) { color: var(--scout-warning); font-size: 11px; }
:where(.scout-coverage-mark):is(.overlay-shell .scout-scope .scout-coverage-mark) { display: none; }
:where(.scout-tag:first-child):is(.overlay-shell .scout-tags .scout-tag:first-child) { flex-shrink: 0; }
:where(.scout-queue-label):is(.scout-card .scout-queue-label) { color: var(--scout-muted); font-size: 11px; line-height: 11px; }
:where(.scout-rank):is(.scout-card .scout-rank) { gap: 0; padding: 2px 4px; }
:where(.card-pick):is(.scout-card .scout-champion .card-pick) :deep(small) { overflow: hidden; text-overflow: ellipsis; }
:where(.scout-no-kda):is(.scout-card .scout-no-kda) { overflow: hidden; text-overflow: ellipsis; }
:where(.scout-empty-tags):is(.scout-card .scout-empty-tags) { font-size: 11px; line-height: 18px; color: var(--scout-muted); }
:where(.scout-position-cell):is(.scout-position-cell) { display: flex; flex-direction: column; align-items: center; border-inline: 1px solid var(--scout-border); min-width: 0; position: relative; }
:where(.role-distribution):is(.role-distribution) { position: absolute; display: flex; gap: 1px; height: 3px; width: 80%; bottom: -4px; border-radius: 2px; overflow: hidden; }
:where(.role-distribution):is(.role-distribution) :deep(i) { min-width: 2px; background: #d5b56f; }
:where(.role-distribution):is(.role-distribution) :deep(.jungle) { background: #61ba91; }
:where(.role-distribution):is(.role-distribution) :deep(.middle) { background: #a494e4; }
:where(.role-distribution):is(.role-distribution) :deep(.bottom) { background: #68b6db; }
:where(.role-distribution):is(.role-distribution) :deep(.utility) { background: #d78da9; }
:where(.scout-queue-label):is(.overlay-shell .scout-queue-label) { font-size: 11px; line-height: 9px; }
:where(.scout-empty-tags):is(.overlay-shell .scout-empty-tags) { font-size: 11px; }
:where(.scout-tag):is(.overlay-shell .scout-tags .scout-tag) { flex: 0 1 auto; max-width: 100%; }
:where(.card-details):is(.card-details) {
  position: relative;
  z-index: 2;
  background: transparent;
  text-shadow: 0 1px 3px #071218;
  width: 100%;
  padding: 6px 12px 8px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 4px;
  flex: 1;
  min-height: 0;
  color: #dce8dd;
}
:where(.rank-line):is(.rank-line) {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 14px;
  font-size: 9px;
  white-space: nowrap;
}
:where(.rank-line):is(.rank-line) :deep(>span) {
  font-size: 8px;
  color: #8ca79e;
  flex: none;
}
:where(.rank-line):is(.rank-line) :deep(>b) {
  font-weight: 400;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
  color: #b5c5ac;
}
:where(.rank-line):is(.rank-line) :deep(small) {
  font-size: 8px;
  color: #d8b785;
}
:where(.overview-metrics):is(.overview-metrics) {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto;
  align-items: center;
  gap: 6px;
  padding-bottom: 3px;
}
:where(.overview-metrics):is(.overview-metrics) :deep(>span) {
  min-width: 0;
}
:where(.overview-metrics):is(.overview-metrics) :deep(b) {
  font-size: 23px;
  line-height: 1.1;
  font-weight: 600;
  letter-spacing: -0.5px;
  display: block;
  white-space: nowrap;
  color: #e5ece4;
}
:where(.overview-metrics):is(.overview-metrics) :deep(>span:nth-child(2)>b) {
  font-size: 18px;
  color: #c4d6cf;
}
:where(.overview-metrics):is(.overview-metrics) :deep(small) {
  display: block;
  font-size: 9px;
  color: #9ab4a7;
  margin-top: 4px;
  white-space: nowrap;
}
:where(.sample-size):is(.overview-metrics .sample-size) {
  font-size: 8px;
  white-space: normal;
  max-width: 26px;
  text-align: right;
}
:where(.recent-block):is(.recent-block) {
  display: flex;
  align-items: center;
  gap: 9px;
  min-height: 31px;
}
:where(.rich-state):is(.rich-state) {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 7px;
  font-size: 11px;
  min-height: 80px;
  color: #a6bbb0;
}
:where(.rich-state):is(.rich-state) :deep(small) {
  font-size: 9px;
  line-height: 1.6;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
:where(.rich-state.loading):is(.rich-state.loading) {
  color: #b4d8df;
}
:where(.rich-state.failed):is(.rich-state.failed) {
  color: #e0c59b;
}
:where(.card-details):is(.overlay-player .card-details) {
  padding: 5px 9px 7px;
  gap: 4px;
}
:where(.rich-state):is(.overlay-player .rich-state) {
  min-height: 75px;
}
:where(.overview-metrics):is(.overlay-player .overview-metrics) :deep(b) {
  font-size: 22px;
}
:where(.overview-metrics):is(.overlay-player .overview-metrics) :deep(>span:nth-child(2)>b) {
  font-size: 17px;
}
:where(.rank-line):is(.overlay-player .rank-line) {
  min-height: 12px;
}
@media (max-width: 1150px) {
  :where(.card-details):is(.card-details) {
    padding: 6px 9px;
    gap: 3px;
  }

}
@media (max-width: 1150px) {
  :where(.overview-metrics):is(.overview-metrics) :deep(b) {
    font-size: 21px;
  }

}
@media (max-width: 1150px) {
  :where(.overview-metrics):is(.overview-metrics) :deep(>span:nth-child(2)>b) {
    font-size: 17px;
  }

}
@media (max-width: 1150px) {
  :where(.recent-block):is(.recent-block) {
    gap: 6px;
  }

}
@media (max-width: 1150px) {
  :where(.rank-line):is(.rank-line) {
    gap: 4px;
  }

}
@media (max-width: 1150px) {
  :where(.rank-line):is(.rank-line) :deep(>span) {
    font-size: 7px;
  }

}
@media (max-width: 1150px) {
  :where(.rank-line):is(.rank-line) :deep(b) {
    font-size: 8px;
  }

}
@media (max-width: 999px) {
  :where(.card-details):is(.overlay-player .card-details) {
    padding: 5px 7px;
  }
}
@media (max-width: 999px) {
  :where(.sample-size):is(.overlay-player .sample-size) {
    display: none;
  }
}
@media (max-width: 999px) {
  :where(.overview-metrics):is(.overlay-player .overview-metrics) {
    grid-template-columns: 1fr 1fr;
  }
}
@media (max-width: 680px) {
  :where(.card-details):is(.overlay-player .card-details) {
    padding: 3px 6px;
    gap: 2px;
  }
}
@media (max-width: 680px) {
  :where(.rank-line):is(.overlay-player .rank-line) {
    display: none;
  }
}
@media (max-width: 680px) {
  :where(.recent-block):is(.overlay-player .recent-block) {
    display: none;
  }
}
@media (max-width: 680px) {
  :where(.overview-metrics):is(.overlay-player .overview-metrics) :deep(b) {
    font-size: 18px;
  }
}
@media (max-width: 680px) {
  :where(.overview-metrics):is(.overlay-player .overview-metrics) :deep(small) {
    font-size: 8px;
    margin-top: 2px;
  }
}
@media (max-width: 680px) {
  :where(.rich-state):is(.overlay-player .rich-state) {
    min-height: 38px;
    font-size: 10px;
    gap: 2px;
  }
}
@media (max-width: 680px) {
  :where(.rich-state):is(.overlay-player .rich-state) :deep(small) {
    font-size: 8px;
  }
}
:where(.card-pick):is(.match-workspace .card-identity>.card-pick) {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  line-height: 20px;
  color: #d6e1dc;
}
:where(.card-details):is(.match-workspace .card-details) {
  padding: 6px 12px 12px;
  gap: 8px;
}
:where(.rank-line):is(.match-workspace .rank-line) {
  font-size: 12px;
  min-height: 18px;
  gap: 5px;
}
:where(.rank-line):is(.match-workspace .rank-line) :deep(>span) {
  font-size: 11px;
  color: #afc3bb;
}
:where(.rank-line):is(.match-workspace .rank-line) :deep(>b) {
  color: #e4d2ac;
}
:where(.rank-line):is(.match-workspace .rank-line) :deep(small) {
  font-size: 10px;
}
:where(.overview-metrics):is(.match-workspace .overview-metrics) {
  padding: 2px 0;
  gap: 8px;
}
:where(.overview-metrics):is(.match-workspace .overview-metrics) :deep(b) {
  font-size: 25px;
  line-height: 30px;
}
:where(.overview-metrics):is(.match-workspace .overview-metrics) :deep(>span:nth-child(2)>b) {
  font-size: 23px;
}
:where(.overview-metrics):is(.match-workspace .overview-metrics) :deep(small) {
  font-size: 11px;
  line-height: 16px;
}
:where(.sample-size):is(.match-workspace .overview-metrics .sample-size) {
  max-width: none;
  font-size: 11px;
  white-space: nowrap;
  align-self: end;
}
:where(.recent-block):is(.match-workspace .recent-block) {
  align-items: stretch;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
:where(.rich-state):is(.match-workspace .rich-state) {
  min-height: 130px;
  justify-content: center;
  gap: 8px;
}
:where(.rich-state):is(.match-workspace .rich-state) :deep(b) {
  font-size: 16px;
}
:where(.rich-state):is(.match-workspace .rich-state) :deep(small) {
  font-size: 12px;
  line-height: 1.7;
}
:where(.scout-data):is(.sidebar-expanded .scout-data) {display:flex;flex-direction:column;}
:where(.scout-headline):is(.sidebar-expanded .scout-headline) {height:98px;}
:where(.scout-rings):is(.sidebar-expanded .scout-rings) {height:112px;}
:where(.scout-tags):is(.sidebar-expanded .scout-tags) {min-height:28px;}
:where(.scout-queue-label):is(.overlay-shell .scout-queue-label) {display:block;}
:where(.scout-rank):is(.overlay-shell .scout-rank) {gap:1px;padding:3px;}
:where(.scout-season):is(.overlay-shell .scout-rank .scout-season) {display:flex;flex-direction:column;gap:0;line-height:12px;white-space:normal;}
</style>
<style scoped>
.scout-stat-skeleton { display: flex; flex: 1; min-height: 0; flex-direction: column; align-items: center; justify-content: center; gap: 9px; padding: 8px 12px; color: var(--scout-muted); }
.scout-skeleton-rings { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); width: 100%; place-items: center; }
.scout-skeleton-rings i { display: block; width: 42px; height: 42px; border: 6px solid var(--scout-track); border-radius: 50%; animation: scout-skeleton-pulse 1.4s ease-in-out infinite alternate; }
.scout-skeleton-rings i:nth-child(2) { animation-delay: .15s; }
.scout-skeleton-rings i:nth-child(3) { animation-delay: .3s; }
.scout-skeleton-line { display: block; width: 56%; height: 7px; border-radius: 4px; background: var(--scout-track); animation: scout-skeleton-pulse 1.4s ease-in-out infinite alternate; }
.scout-skeleton-line.wide { width: 82%; }
.scout-stat-skeleton small { font-size: 11px; line-height: 14px; }
@keyframes scout-skeleton-pulse { from { opacity: .45; } to { opacity: 1; } }
@media (prefers-reduced-motion: reduce) { .scout-stat-skeleton i { animation: none; } }
</style>

<style scoped>.mode-rank-reference{display:flex;flex-direction:column;align-items:center;justify-content:center;color:var(--scout-muted);font-size:11px;padding:3px;line-height:14px;min-width:0;text-align:center;} .scout-headline:has(.mode-rank-reference) .scout-hero-rate{line-height:12px;} .scout-headline:has(.mode-rank-reference) .scout-champion{min-height:0;gap:2px;} .scout-headline:has(.mode-rank-reference) .card-pick small{line-height:13px;}</style>
