<script setup lang="ts">
import { uiText } from "../uiText";
import { computed, ref } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import {
  date,
  decimal,
  percent,
  queueName,
  sampleStats,
  type PlayerViewState,
} from "../presentation";
import PlayerProfileHeader from "./PlayerProfileHeader.vue";
import MatchHistoryRow from "./MatchHistoryRow.vue";
import HistoryNotice from "./HistoryNotice.vue";
import HistoryServiceStatus from './HistoryServiceStatus.vue';

import AppIcon from "./AppIcon.vue";
import MatchDetails from "./MatchDetails.vue";

import TransientNotice from "./TransientNotice.vue";
import OverviewPerformance from './OverviewPerformance.vue';
import RankPanel from './RankPanel.vue';
import ChampionAnalysis from './ChampionAnalysis.vue';
import PlaystylePanel from './PlaystylePanel.vue';
import MasteryPanel from './MasteryPanel.vue';

import { scoutStats, roleNames } from "../scoutStats";
import {
  HISTORY_PAGE_SIZE,
  SAMPLE_SIZES,
  useHistoryBrowser,
} from "../historyBrowser";
const props = defineProps<{
  player: Participant;
  state: PlayerViewState;
  busy?: boolean;
  refreshing?: boolean;
  connected?: boolean;
  compact?: boolean;
  active?: boolean;
  error?: string;
  scoutQueue?: number;
  captured?: boolean;
  prepared?: boolean;
  automaticHistory?: boolean;
}>();
const emit = defineEmits<{
  refresh: [player: Participant];
  recovered:[player:Participant];
}>();
const historyScroll = ref<HTMLElement | null>(null);
const profileCollapsed = ref(false);
const scouting = computed(() => scoutStats(props.player, props.scoutQueue ?? 420));
const roleDistribution = computed(() => scouting.value.role.positions
  .map(p => uiText.playerCardData.roleCount(roleNames[p.position], p.games)).join('、') || uiText.playerView.noValidRecords);
const {
  allMatches,
  matches,
  analysisMatches,
  pageMatches,
  knownPages,
  canFetch,
  maxJumpPage,
  jumpDraft,
  jumpError,
  jumpPage,
  sampleLabel,
  needsSamples,
  noticePlayer,
  changeCriteria,
  goPage,
  fill,
} = useHistoryBrowser(props, historyScroll);
const stats = computed(() => sampleStats(analysisMatches.value));
const modes = computed(() => [
  ...new Set([420, 440, 450, 2400, ...allMatches.value.map((m) => m.queueId)]),
]);
function toggle(id: number) {
  const i = props.state.expanded.indexOf(id);
  if (i < 0) props.state.expanded.push(id);
  else props.state.expanded.splice(i, 1);
}
</script>
<template>
  <section class="player-view analysis-layout" :class="{ compact, 'profile-collapsed': !compact && profileCollapsed }">
    <PlayerProfileHeader
      :player="player"
      :busy="busy"
      :connected="connected"
      :compact="compact"
      :hide-ranks="!compact"
    />
    <TransientNotice :message="error" banner-class="player-error" />
    <div class="player-columns">
      <aside v-if="!compact && !profileCollapsed" class="analysis-sidebar">
        <RankPanel :player="player"/>
        <RankPanel :player="player" queue="RANKED_FLEX_SR"/>
        <ChampionAnalysis :matches="analysisMatches" :scope="sampleLabel" :fetched-at="player.historyMeta?.fetchedAt"/>
        <PlaystylePanel :matches="analysisMatches" :scope="sampleLabel"/>
        <MasteryPanel :player="player"/>
      </aside>
      <div class="history-column">
        <details v-if="!compact" class="analysis-overview"><summary><strong>{{ uiText.playerView.recentAnalysis }}</strong><span>{{stats.outcomeGames?uiText.playerView.outcomes(stats.wins, stats.losses):uiText.playerView.pendingOutcome}} <b class="analysis-sample-win">{{percent(stats.winRate)}}</b></span><span>KDA <b class="analysis-sample-kda">{{decimal(stats.kda)}}</b></span><small class="analysis-scope">{{sampleLabel}}</small><span class="overview-expand">{{ uiText.playerView.expandAnalysis }}</span></summary><OverviewPerformance :matches="analysisMatches" :scope="sampleLabel"/></details>
        <div class="history-toolbar">
          <div class="history-filter">
            <strong class="history-count"
              >{{ uiText.playerView.historyHeading }}<span>{{ matches.length }}</span></strong
            ><label
              ><span class="sr-only">{{ uiText.playerView.mode }}</span
              ><select
                v-model="state.queue"
                :aria-label="uiText.playerView.modeFilter"
                :disabled="busy"
                @change="changeCriteria"
              >
                <option value="all">{{ uiText.common.allModes }}</option>
                <option
                  v-if="
                    state.queue !== 'all' &&
                    !modes.includes(Number(state.queue))
                  "
                  :value="state.queue"
                >
                  {{ uiText.playerView.emptyMode(queueName(Number(state.queue))) }}</option>
                <option v-for="mode in modes" :key="mode" :value="String(mode)">
                  {{ queueName(mode) }}
                </option>
              </select></label
            >
            <select
              v-model.number="state.sampleSize"
              :aria-label="uiText.playerView.sampleSize"
              :disabled="busy"
              @change="changeCriteria"
            >
              <option v-for="size in SAMPLE_SIZES" :key="size" :value="size">{{ uiText.playerView.recentGames(size) }}</option>
            </select>
          </div>
          <div class="history-actions">
            <button v-if="!compact" class="text-button profile-toggle" :aria-expanded="!profileCollapsed" @click="profileCollapsed = !profileCollapsed">{{ profileCollapsed ? uiText.playerView.expandProfile : uiText.playerView.collapseProfile }}</button>
            <span
              class="toolbar-time"
              :title="uiText.playerView.fetchedAt(date(player.historyMeta?.fetchedAt))"
              ><HistoryServiceStatus inline readonly :player="player"/></span
            ><button
              class="secondary"
              :disabled="busy || !connected || player.identityState !== 'known'"
              @click="emit('refresh', player)"
            >
              <AppIcon name="refresh" :size="14" />{{
                refreshing ? uiText.common.refreshing : uiText.common.refresh
              }}
            </button>
          </div>
        </div>
        <HistoryServiceStatus :player="player" :connected="connected" :busy="busy || state.pageBusy || state.warmBusy" :readonly="compact || captured || prepared" :automatic="automaticHistory && active!==false && state.expanded.length===0 && (state.page??1)===1" @recovered="emit('recovered',$event)" @retry="emit('refresh',player)"/>
        <details v-if="compact && player.scout" class="scout-explanation">
          <summary>{{ uiText.playerView.statistics(scouting.queueLabel) }}</summary>
          <p v-if="scouting.hero?.games != null && scouting.report?.season">{{ uiText.playerView.championRecords(scouting.report.season.name, scouting.report.complete ? uiText.common.allModes : uiText.playerView.allModes, scouting.hero.games) }}</p>
          <p v-if="scouting.hero?.mastery">{{ uiText.playerView.mastery(scouting.hero.mastery.level, scouting.hero.mastery.points.toLocaleString()) }}</p>
          <p>{{ uiText.playerView.rankedCutoff(date(scouting.report?.fetchedAt), scouting.report?.retained ? uiText.playerView.retained : '', scouting.overall.outcomeGames ?? 0, scouting.overall.kdaGames ?? 0, scouting.report?.complete30 ? '' : uiText.playerView.readRange) }}</p>
          <p>{{ uiText.playerView.role(roleDistribution) }}</p>
        </details>
        <details
          v-if="noticePlayer.historyMeta?.excluded"
          class="quality-warning"
        >
          <summary>
            {{ uiText.playerView.excludedRecords(noticePlayer.historyMeta.excluded) }}<span>{{ uiText.playerView.reason }}</span>
          </summary>
          <p>{{ uiText.playerView.incomplete(noticePlayer.historyMeta.returned, noticePlayer.historyMeta.parsed, noticePlayer.historyMeta.excluded) }}</p>
        </details>
        <div v-if="compact" class="compact-stats">
          <span>{{ uiText.homeView.sampleCount(stats.games, state.sampleSize) }}</span
          ><span
            >{{ uiText.playerView.winRate }}<b>{{ percent(stats.winRate) }}</b></span
          ><span
            >KDA <b>{{ decimal(stats.kda) }}</b></span
          >
        </div>
        <div
          v-if="
            state.pageBusy ||
            state.warmBusy ||
            state.pageError ||
            state.browseNotice ||
            needsSamples
          "
          class="browse-status"
          role="status"
        >
          <span v-if="state.pageBusy"
            >{{ uiText.playerView.searching(state.queue === "all" ? uiText.playerView.history : queueName(Number(state.queue)), (state.browseTarget ?? 1) > 1
                ? uiText.playerView.page(state.browseTarget)
                : "", matches.length) }}</span
          >
          <span v-else-if="state.warmBusy">{{ uiText.playerView.backgroundRanked }}</span>
          <span v-else-if="state.pageError" class="warning-text"
            >{{ uiText.playerView.retainedRecords(state.pageError) }}</span
          >
          <span v-else>{{
            prepared ? uiText.playerView.capturedHistory : captured ? uiText.playerView.archivedHistory : state.browseNotice || uiText.playerView.incompleteSample
          }}</span>
          <button
            v-if="state.pageBusy"
            class="text-button"
            :disabled="state.stopRequested"
            @click="state.stopRequested = true"
          >
            {{ state.stopRequested ? uiText.playerView.stopping : uiText.playerView.stop }}
          </button>
          <button
            v-else-if="!state.warmBusy && state.hasMore && canFetch"
            class="text-button"
            :disabled="busy"
            @click="fill(state.browseTarget ?? state.page ?? 1)"
          >
            {{
              state.pageError
                ? uiText.common.retryAction
                : state.browseNotice
                  ? uiText.homeView.continueSearch
                  : uiText.playerView.completeSample
            }}
          </button>
        </div>
        <div class="history-panel">
          <div ref="historyScroll" class="history-scroll">
            <div v-if="!matches.length" class="empty-state">
              <AppIcon name="history" :size="32" /><strong>{{
                ["pending", "failed", "loading"].includes(player.historyState)
                  ? captured ? uiText.playerView.incompleteArchive : uiText.playerView.pendingProfile
                  : state.pageError || state.sourceLimit
                    ? uiText.playerView.missingMode
                    : uiText.playerView.empty
              }}</strong>
              <p>
                {{
                  player.identityState === "known"
                    ? state.pageError || state.sourceLimit
                      ? uiText.playerView.restricted
                      : prepared ? uiText.playerView.capturedModeEmpty : captured ? uiText.playerView.archivedModeEmpty : uiText.playerView.emptyHelp
                    : uiText.playerView.missingIdentity
                }}
              </p>
            </div>
            <MatchHistoryRow
              v-for="match in pageMatches"
              :key="match.gameId"
              :match="match"
              :expanded="state.expanded.includes(match.gameId)"
              :compact="compact"
              @toggle="toggle(match.gameId)"
            >
              <template #details>
                <MatchDetails
                  :player="player"
                  :game-id="match.gameId"
                  :match="match"
                  :connected="connected"
                />
              </template>
            </MatchHistoryRow>
            <HistoryNotice
              :player="noticePlayer"
              :stats="stats"
              :filtered="state.queue !== 'all'"
            />
          </div>
          <nav class="history-pagination" :aria-label="uiText.playerView.pagination">
            <span
              >{{
                pageMatches.length
                  ? uiText.playerView.gameRange(((state.page ?? 1) - 1) * HISTORY_PAGE_SIZE + 1, ((state.page ?? 1) - 1) * HISTORY_PAGE_SIZE + pageMatches.length)
                  : uiText.common.noRecords
              }}<small>{{ uiText.playerView.pageSize(HISTORY_PAGE_SIZE) }}</small></span
            >
            <div class="page-controls">
              <button
                class="secondary"
                :disabled="busy || state.pageBusy || (state.page ?? 1) <= 1"
                @click="goPage((state.page ?? 1) - 1)"
              >{{ uiText.playerView.previous }}</button>
              <span class="page-indicator"
                >{{ uiText.common.page(state.page ?? 1) }}<small>{{
                  state.hasMore || state.sourceLimit
                    ? uiText.playerView.foundPages(knownPages)
                    : uiText.playerView.totalPages(knownPages)
                }}</small></span
              >
              <button
                class="secondary"
                :disabled="
                  busy ||
                  state.pageBusy ||
                  ((state.page ?? 1) >= knownPages &&
                    (!state.hasMore || !canFetch))
                "
                @click="goPage((state.page ?? 1) + 1)"
              >{{ uiText.playerView.next }}</button>
            </div>
            <form class="page-jump" @submit.prevent="jumpPage" novalidate>
              <label
                >{{ uiText.playerView.goTo }}<input
                  v-model="jumpDraft"
                  :aria-label="uiText.playerView.jumpLabel"
                  :aria-invalid="!!jumpError"
                  :title="uiText.playerView.pageRange(maxJumpPage)"
                  inputmode="numeric"
                  autocomplete="off"
                  maxlength="3"
                  :disabled="busy || state.pageBusy"
                />{{ uiText.playerView.pageUnit }}</label>
              <button class="secondary" :disabled="busy || state.pageBusy">{{ uiText.playerView.jump }}</button>
            </form>
            <p v-if="jumpError" class="page-jump-error" role="alert">
              {{ jumpError }}
            </p>
          </nav>
        </div>
      </div>
    </div>
  </section>
</template>
<style scoped>
.scout-explanation { padding: 7px 0; font-size: 12px; color: var(--muted); border-top: 1px solid var(--border); flex: none; }
.scout-explanation summary { cursor: pointer; color: var(--text); }
.scout-explanation p { margin: 8px 0; line-height: 1.6; }
.profile-toggle { white-space: nowrap; font-size: 12px; }
@media (max-width: 1000px) {
  .drawer-floating .player-view.compact { grid-template-columns: minmax(0, 1fr); grid-template-rows: auto minmax(0, 1fr); }
  .drawer-floating .player-view.compact :deep(.player-profile) { grid-column: 1; grid-row: 1; padding: 10px 14px; flex-wrap: nowrap; align-items: center; gap: 10px; border-right: 0; border-bottom: 1px solid var(--border); }
  .drawer-floating .player-view.compact :deep(.profile-identity) { flex: 1; flex-basis: auto; }
  .drawer-floating .player-view.compact :deep(.identity-title h1) { font-size: 15px; }
  .drawer-floating .player-view.compact :deep(.profile-ranks) { flex: none; flex-basis: auto; flex-direction: row; align-items: center; gap: 12px; margin: 0; }
  .drawer-floating .player-view.compact :deep(.rank-card) { display: flex; gap: 5px; max-width: 150px; }
  .drawer-floating .player-view.compact :deep(.rank-card > .rank-emblem) { width: 32px; height: 32px; }
  .drawer-floating .player-view.compact :deep(.rank-card strong) { font-size: 12px; }
  .drawer-floating .compact > .player-columns { grid-column: 1; grid-row: 2; }
  .drawer-floating .compact .history-toolbar { min-height: 44px; }
  .drawer-floating .player-view.compact :deep(.match-main) { min-height: 60px; gap: 8px; }
}
</style>
