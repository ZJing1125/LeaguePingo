<script setup lang="ts">
import { uiText } from "../uiText";
import { computed } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import { percent, decimal, date } from "../presentation";
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
const heroScope = computed(() => props.player.championState === "intent" ? uiText.playerCardData.intent : props.player.championId ? data.value.report?.season ? data.value.report.complete ? uiText.playerCardData.seasonRanked : uiText.playerCardData.seasonRead : uiText.playerCardData.championPerformance  : props.party ? uiText.common.party : uiText.playerCardData.waitingSelection);
const heroStatus = computed(() => {
  if (!props.player.championId) return props.party || props.player.championState === "unselected" ? uiText.playerCardData.waitingHeroSelection : uiText.playerCardData.waitingHeroInfo;
  const report = data.value.report;
  if (!report || report.state === "loading") return props.captured ? uiText.playerCardData.incompleteCaptured : uiText.playerCardData.preparingChampion;
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
  return report.read ? uiText.playerCardData.read30Days(report.read) : uiText.playerCardData.pending30Days;
});
const roleCaption = computed(() => ({ [uiText.playerCardData.roleMissingKey]: uiText.common.noRecords, [uiText.playerCardData.roleSparseKey]: uiText.playerCardData.sparse, [uiText.playerCardData.roleSpreadKey]: uiText.playerCardData.spread }[data.value.role.label] || data.value.role.label));
const shortRank = computed(() => rank.value.rank ? rank.value.label : rank.value.label.includes(uiText.common.loss) ? uiText.common.queryFailed : rank.value.label.includes(uiText.playerCardData.timeoutToken) ? uiText.common.queryTimeout : uiText.common.pendingRank);
const roleTitle = computed(() => uiText.playerCardData.roleHint(data.value.queueLabel, data.value.role.known, data.value.overall.games, data.value.role.positions.map(p => uiText.playerCardData.roleCount(roleNames[p.position], p.games)).join('，')));
function periodCaption(p: ReturnType<typeof scoutStats>["periods"][number]) {
  if (!data.value.report || !p.games && data.value.report.state === "loading") return props.captured ? uiText.common.notRetained : uiText.common.preparing;
  return p.outcomeGames ? uiText.playerCardData.validGames(p.outcomeGames) : p.games ? uiText.playerCardData.readGames(p.games) : p.complete ? uiText.playerCardData.zeroGames : uiText.common.pending;
}
function periodDetail(p: ReturnType<typeof scoutStats>["periods"][number]) {
  if (!p.games) return p.complete ? uiText.playerCardData.noRanked : data.value.report?.state === "loading" || !data.value.report ? "—" : uiText.common.incompleteRange;
  return p.outcomeGames ? uiText.playerCardData.outcomes(p.wins, p.losses) : uiText.playerCardData.missingOutcome;
}
</script>
<template>
  <span class="card-details scout-data">
    <span class="scout-headline">
      <span class="scout-champion">
        <span class="card-pick"><GameIcon v-if="player.championId" :id="player.championId" :size="32" /><AppIcon v-else name="match" :size="28" /><span><strong :title="hero">{{ hero }}</strong><small :title="heroScope">{{ heroScope }}</small></span></span>
        <span v-if="data.champion.average" class="scout-kda" :title="uiText.playerCardData.championAverage(data.queueLabel, data.champion.kdaGames)"><b>{{ data.champion.average[0]!.toFixed(1) }}</b> / <b>{{ data.champion.average[1]!.toFixed(1) }}</b> / <b>{{ data.champion.average[2]!.toFixed(1) }}</b></span>
        <span v-else class="scout-no-kda" :title="heroStatus">{{ heroStatus }}</span>
        <small class="scout-hero-rate" :class="{ 'no-sample': !data.champion.outcomeGames }">{{ data.champion.outcomeGames ? percent(data.champion.winRate) : data.report?.season ? uiText.playerCardData.seasonQueue(data.queueLabel) : data.queueLabel }}<span v-if="data.champion.outcomeGames">{{ uiText.playerCardData.championOutcomes(data.champion.wins, data.champion.losses) }}</span></small>
      </span>
      <span class="scout-rank" :title="`${data.queueLabel} · ${rank.title}`">
        <span class="scout-rank-heading"><RankEmblem :tier="player.identityState === 'known' ? rank.rank?.tier : undefined" /><small class="scout-queue-label">{{ data.queueLabel }}</small></span>
        <span class="rank-line"><b>{{ shortRank }}</b><small v-if="rank.retained">{{ uiText.common.old }}</small></span>
        <small class="scout-season" :title="season?.title"><span v-if="season">{{ season.scope }} {{ percent(season.rate) }}</span><span>{{ season ? uiText.playerCardData.outcomes(season.wins, season.losses) : uiText.common.pendingOutcome }}</span></small>
      </span>
    </span>
    <template v-if="player.identityState === 'known'">
      <span v-if="statisticsPending" class="scout-stat-skeleton" role="status" :aria-label="uiText.playerCardData.fetchingStatistics">
        <span class="scout-skeleton-rings" aria-hidden="true"><i v-for="slot in 3" :key="slot" /></span>
        <i class="scout-skeleton-line wide" aria-hidden="true" />
        <i class="scout-skeleton-line" aria-hidden="true" />
        <small>{{ uiText.playerCardData.preparing }}</small>
      </span>
      <template v-else>
        <span class="overview-metrics scout-rings" :title="uiText.playerCardData.independentScope(data.queueLabel, date(data.report?.fetchedAt))">
          <ScoutRing :value="data.periods[0]!.winRate" short="12h" :label="uiText.playerCardData.last12Hours" :caption="periodCaption(data.periods[0]!)" :detail="periodDetail(data.periods[0]!)" :title="uiText.playerCardData.last12HoursHint(data.queueLabel, data.periods[0]!.outcomeGames, data.periods[0]!.games, data.periods[0]!.complete ? '' : uiText.playerCardData.readOnly)" />
          <span class="scout-position-cell">
            <ScoutRing :value="data.role.share" :role="data.role.key" short="" :label="uiText.playerCardData.role30Days" :caption="roleCaption" :detail="data.role.known ? uiText.playerCardData.recordedGames(data.role.known) : '—'" :title="roleTitle" />
            <span v-if="data.role.known" class="role-distribution" :title="roleTitle"><i v-for="p in data.role.positions" :key="p.position" :class="p.position.toLowerCase()" :style="{ flexGrow: p.games }" :title="uiText.playerCardData.roleCount(roleNames[p.position], p.games)" /></span>
          </span>
          <ScoutRing :value="data.periods[1]!.winRate" short="30d" :label="uiText.playerCardData.within30Days" :caption="periodCaption(data.periods[1]!)" :detail="periodDetail(data.periods[1]!)" :title="uiText.playerCardData.last30DaysHint(data.queueLabel, data.periods[1]!.outcomeGames, data.periods[1]!.games, data.periods[1]!.complete ? '' : uiText.playerCardData.readOnly)" />
        </span>
        <span class="scout-scope" :title="uiText.playerCardData.cutoffHint(data.queueLabel, range, data.overall.games, data.overall.kdaGames ?? 0, date(data.report?.fetchedAt), data.report?.season?.name || uiText.playerCardData.missingSeason)"><small class="sample-size">{{ range }}</small><span>KDA <b>{{ decimal(data.overall.kda) }}</b></span><span v-if="data.report && !data.report.complete30" class="scout-coverage-mark">{{ uiText.common.incompleteRange }}</span></span>
        <span class="scout-tags recent-block"><span v-for="tag in data.tags" :key="tag.label" class="scout-tag" :class="tag.tone" :title="tag.title">{{ tag.label }}</span><small v-if="!data.tags.length" class="scout-empty-tags">{{ data.report?.state === 'loading' || !data.report ? captured ? uiText.playerCardData.capturedRanked : uiText.playerCardData.preparingRanked : uiText.playerCardData.seasonChampion }}</small></span>
      </template>
    </template>
    <span v-else class="rich-state scout-state"><AppIcon name="history" :size="24" /><b>{{ uiText.playerCardData.missingIdentity }}</b><small>{{ uiText.playerCardData.waitingIdentity }}</small></span>
  </span>
</template>
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
