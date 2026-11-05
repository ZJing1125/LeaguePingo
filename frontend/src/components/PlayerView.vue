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
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.player-view):is(.player-view) {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
:where(.player-error):is(.player-error) {
  margin: 8px 18px;
}
:where(.player-columns):is(.player-columns) {
  display: flex;
  flex: 1;
  min-height: 0;
}
:where(.compact-stats):is(.compact-stats) :deep(>select) {
  padding: 4px 6px;
  font-size: 11px;
  max-width: 100px;
}
:where(.history-pagination):is(.history-pagination) {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex-wrap: wrap;
  min-height: 54px;
  padding: 8px 10px;
  border-top: 1px solid var(--border);
  background: #151a22;
  font-size: 11px;
  color: var(--muted);
}
:where(.page-controls):is(.history-pagination .page-controls) {
  display: flex;
  align-items: center;
  gap: 8px;
}
:where(.history-pagination):is(.history-pagination) :deep(small) {
  display: block;
  font-size: 10px;
  margin-top: 3px;
}
:where(.history-pagination):is(.history-pagination) :deep(button) {
  padding: 5px 8px;
  font-size: 11px;
}
:where(.page-jump):is(.page-jump) {
  display: flex;
  align-items: center;
  gap: 6px;
}
:where(.page-jump):is(.page-jump) :deep(label) {
  display: flex;
  align-items: center;
  gap: 6px;
}
:where(.page-jump):is(.page-jump) {
  margin: 0;
}
:where(.page-jump):is(.page-jump) :deep(input) {
  width: 42px;
  padding: 5px 4px;
  font-size: 11px;
  text-align: center;
}
:where(.page-jump):is(.page-jump) :deep(input[aria-invalid="true"]) {
  border-color: var(--loss);
}
:where(.page-indicator):is(.page-indicator) {
  text-align: center;
  min-width: 60px;
  color: var(--text);
}
:where(.browse-status):is(.browse-status) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex: none;
  padding: 8px 0;
  font-size: 11px;
  line-height: 1.6;
  color: var(--muted);
}
:where(.browse-status):is(.browse-status) :deep(button) {
  flex: none;
  color: var(--accent);
}
:where(.history-column):is(.history-column) {
  display: flex;
  flex: 1;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  padding: 0 18px 14px;
}
:where(.history-panel):is(.history-panel) {
  display: flex;
  flex: 1;
  min-height: 0;
  min-width: 0;
  flex-direction: column;
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
}
:where(.history-scroll):is(.history-panel>.history-scroll) {
  padding: 8px 8px 4px;
}
:where(.history-toolbar):is(.history-toolbar) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex: none;
  min-height: 46px;
}
:where(.history-filter):is(.history-filter) {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
:where(.history-actions):is(.history-actions) {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
:where(.history-count):is(.history-count) {
  font-size: 13px;
  white-space: nowrap;
}
:where(.history-count):is(.history-count) :deep(>span) {
  font-weight: 400;
  color: var(--muted);
  margin-left: 4px;
}
:where(.history-toolbar):is(.history-toolbar) :deep(select) {
  max-width: 170px;
  font-size: 12px;
  padding: 5px 8px;
}
:where(.toolbar-time):is(.toolbar-time) {
  font-size: 11px;
  color: var(--muted);
  white-space: nowrap;
}
:where(.history-actions):is(.history-actions) :deep(button) {
  padding: 5px 9px;
}
:where(.history-column):is(.history-column) :deep(>.quality-warning) {
  margin-bottom: 9px;
}
:where(.history-scroll):is(.history-scroll) {
  min-height: 0;
  flex: 1;
  overflow-y: auto;
  padding-right: 3px;
  scrollbar-gutter: stable;
}
:where(.history-column):is(.compact .history-column) {
  padding: 0 10px 10px;
}
:where(.history-pagination):is(.compact .history-pagination) {
  gap: 6px;
  padding: 7px 8px;
}
:where(.page-jump):is(.compact .history-pagination .page-jump) {
  width: 100%;
  justify-content: flex-end;
}
:where(.history-toolbar):is(.compact .history-toolbar) {
  min-height: 48px;
}
:where(.history-count):is(.compact .history-count) {
  display: none;
}
:where(.toolbar-time):is(.compact .toolbar-time) {
  display: none;
}
:where(.history-filter):is(.compact .history-filter) {
  gap: 7px;
}
:where(.history-actions):is(.compact .history-actions) {
  gap: 7px;
}
:where(.history-toolbar):is(.compact .history-toolbar) :deep(select) {
  max-width: 118px;
  font-size: 11px;
}
:where(.history-actions):is(.compact .history-actions) :deep(button) {
  font-size: 11px;
  padding: 6px 8px;
}
:where(.history-filter):is(.compact .history-filter) {
  min-width: 0;
}
:where(.history-filter):is(.history-filter) :deep(>select) {
  max-width: 108px;
}
:where(.compact-stats):is(.compact-stats) {
  display: flex;
  gap: 15px;
  color: var(--muted);
  font-size: 12px;
  padding: 4px 0 12px;
}
:where(.compact-stats):is(.compact-stats) :deep(b) {
  color: var(--text);
  font-weight: 500;
}
@media (max-width: 760px) {
  :where(.history-column):is(.history-column) {
    padding: 0 10px;
  }
}
@media (max-width: 760px) {
  :where(.toolbar-time):is(.toolbar-time) {
    display: none;
  }
}
@media (max-width: 620px) {
  :where(.history-toolbar):is(.history-toolbar) {
    flex-wrap: wrap;
    padding: 7px 0;
  }
}
@media (max-width: 620px) {
  :where(.history-filter):is(.history-filter) {
    gap: 7px;
  }
}
@media (max-width: 620px) {
  :where(.history-actions):is(.history-actions) {
    margin-left: auto;
  }
}
:where(select):is(:is(.desktop-app, .drawer-floating) select) { appearance: none; -webkit-appearance: none; border-radius: var(--control-radius); padding: 6px 28px 6px 10px; background-color: var(--raised); color: var(--text); border: 1px solid var(--border); background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath d='m3 4.5 3 3 3-3' fill='none' stroke='%238d929e' stroke-width='1.5' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E"); background-repeat: no-repeat; background-position: right 9px center; }
:where(.player-view.compact):is(.player-drawer .player-view.compact) {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  min-height: 0;
}
:where(.player-columns):is(.player-drawer .compact>.player-columns) {
  grid-column: 2;
  grid-row: 1;
  display: flex;
  min-height: 0;
}
:where(.history-column):is(.player-drawer .history-column) {
  padding: 0 10px 6px;
}
:where(.history-toolbar):is(.player-drawer .history-toolbar) {
  min-height: 33px;
  gap: 6px;
}
:where(.history-toolbar):is(.player-drawer .history-toolbar) :deep(select) {
  font-size: 10px;
  padding: 3px 4px;
  max-width: 110px;
}
:where(.history-filter):is(.player-drawer .history-filter) {
  gap: 7px;
}
:where(.history-count):is(.player-drawer .history-count) {
  font-size: 10px;
}
:where(.history-actions):is(.player-drawer .history-actions) :deep(button) {
  font-size: 10px;
}
:where(.compact-stats):is(.player-drawer .compact-stats) {
  min-height: 20px;
  margin: 0;
  padding: 0;
  font-size: 10px;
}
:where(.history-panel):is(.player-drawer .history-panel) {
  background: var(--panel);
}
:where(.history-pagination):is(.player-drawer .history-pagination) {
  padding: 3px 7px;
  min-height: 34px;
  gap: 8px;
  flex-wrap: nowrap;
}
:where(.page-controls):is(.player-drawer .page-controls) {
  gap: 6px;
}
:where(.history-pagination):is(.player-drawer .history-pagination) :deep(button) {
  font-size: 9px;
}
:where(.page-jump):is(.player-drawer .page-jump) {
  font-size: 9px;
}
:where(.page-indicator):is(.player-drawer .page-indicator) {
  font-size: 9px;
}
:where(.history-pagination):is(.player-drawer .history-pagination) :deep(>span) {
  font-size: 9px;
}
:where(.history-pagination):is(.player-drawer .history-pagination) :deep(small) {
  font-size: 8px;
}
:where(.history-pagination):is(.player-drawer .history-pagination) :deep(button) {
  padding: 3px 5px;
}
:where(.page-jump):is(.player-drawer .page-jump) :deep(input) {
  width: 30px;
  padding: 2px;
}
:where(.browse-status):is(.player-drawer .browse-status) {
  padding: 2px 0;
  font-size: 10px;
  min-height: 22px;
}
:where(.player-error):is(.player-drawer .player-error) {
  grid-column: 1/-1;
}
:where(.player-view.compact):is(.drawer-floating .player-view.compact) {
  display: flex;
}
:where(.player-columns):is(.drawer-floating .compact>.player-columns) {
  flex: 1;
}
:where(.history-column):is(.players-workspace .history-column) {
  padding: 0;
  flex: 1;
}
:where(.history-toolbar):is(.players-workspace .history-toolbar) {
  min-height: 48px;
  border-top: 1px solid var(--border);
}
:where(.history-panel):is(.history-panel) {
  border-radius: 3px;
  background: var(--panel);
}
:where(.history-scroll):is(.history-panel>.history-scroll) {
  padding: 0 9px;
}
:where(.history-pagination):is(.history-pagination) {
  background: var(--raised);
  border-color: var(--border);
}
:where(.player-view):is(.player-view) :deep(>.inline-status) {
  grid-column: 2;
  position: absolute;
  top: 0;
  right: 28px;
  max-width: 65%;
  z-index: 2;
  font-size: 10px;
  padding: 3px 8px;
}
:where(.player-error):is(.players-workspace .player-error) {
  grid-column: 2;
  position: absolute;
  top: 0;
  right: 20px;
  z-index: 3;
}
@media (max-width: 999px) {
  :where(.history-toolbar):is(.history-toolbar) :deep(select) {
    font-size: 11px;
    max-width: 140px;
  }
}
@media (max-width: 999px) {
  :where(.history-filter):is(.history-filter) {
    gap: 6px;
  }
}
@media (max-width: 999px) {
  :where(.toolbar-time):is(.toolbar-time) {
    display: none;
  }
}
:where(.history-count):is(.player-drawer .compact .history-count) {
  display: block;
}
:where(.history-filter):is(.player-drawer .compact .history-filter) {
  flex-wrap: nowrap;
}
:where(.history-pagination):is(.player-drawer .compact .history-pagination) :deep(>span) {
  white-space: nowrap;
}
:where(.page-jump):is(.player-drawer .compact .history-pagination .page-jump) {
  margin-left: auto;
}
:where(.page-controls):is(.player-drawer .compact .history-pagination .page-controls) {
  margin-left: 5px;
}
:where(.history-panel):is(.players-workspace .history-panel) {
  border-inline: 0;
  border-top: 0;
}
:where(.page-controls):is(.player-drawer .history-pagination .page-controls) {
  flex: none;
}
:where(.history-pagination):is(.player-drawer .history-pagination) {
  justify-content: space-between;
}
:where(.player-view.compact):is(.drawer-floating .player-view.compact) {
  display: grid;
  grid-template-columns: 230px minmax(0, 1fr);
}
:where(.player-columns):is(.drawer-floating .compact>.player-columns) {
  grid-column: 2;
  grid-row: 1;
  min-width: 0;
  min-height: 0;
}
:where(.history-toolbar):is(.drawer-floating .history-toolbar) {
  min-height: 51px;
  padding-block: 8px;
}
:where(.compact-stats):is(.drawer-floating .compact-stats) {
  font-size: 12px;
  padding: 4px 0 9px;
}
:where(.history-pagination):is(.drawer-floating .history-pagination) {
  padding-block: 10px;
  min-height: 48px;
}
:where(.history-actions):is(.drawer-floating .history-actions) :deep(.secondary) {
  background: var(--button-bg);
  color: var(--button-text);
  border-color: var(--border);
}
@media (max-width: 1000px) {
  :where(.player-view.compact):is(.drawer-floating .player-view.compact) {
    grid-template-columns: 210px minmax(0, 1fr);
  }
}
:where(.player-view.analysis-layout:not(.compact)):is(.players-workspace>.player-view.analysis-layout:not(.compact)){display:flex;flex-direction:column;padding:14px 20px 16px;gap:8px;min-height:0;overflow:hidden;}
:where(.player-columns):is(.players-workspace .analysis-layout:not(.compact)>.player-columns){display:grid;grid-template-columns:280px minmax(0,1fr);gap:16px;min-height:0;flex:1;}
:where(.analysis-sidebar):is(.analysis-sidebar){display:flex;flex-direction:column;gap:12px;min-width:0;min-height:0;overflow:auto;overscroll-behavior:contain;padding-right:2px;}
:where(.history-column):is(.players-workspace .analysis-layout:not(.compact) .history-column){display:flex;flex-direction:column;min-width:0;min-height:0;container-type:inline-size;padding:0;}
:where(.history-toolbar):is(.players-workspace .analysis-layout:not(.compact) .history-toolbar){border-top:0;min-height:44px;gap:8px;}
:where(.history-panel):is(.players-workspace .analysis-layout:not(.compact) .history-panel){flex:1;min-height:0;border:1px solid var(--border);border-radius:6px;overflow:hidden;}
:where(.history-scroll):is(.players-workspace .analysis-layout:not(.compact) .history-panel>.history-scroll){padding:0;}
:where(.player-columns):is(.players-workspace .analysis-layout.profile-collapsed:not(.compact)>.player-columns){grid-template-columns:minmax(0,1fr);}
:where(.analysis-layout):is(.players-workspace .analysis-layout) :deep(>.inline-status){position:static;grid-column:auto;max-width:none;}
@media(max-width:1100px){:where(.player-view.analysis-layout:not(.compact)):is(.players-workspace>.player-view.analysis-layout:not(.compact)){padding:12px 16px;gap:10px;}}
@media(max-width:1100px){:where(.player-columns):is(.players-workspace .analysis-layout:not(.compact)>.player-columns){grid-template-columns:250px minmax(0,1fr);gap:12px;}}
@media(max-width:1100px){:where(.toolbar-time):is(.players-workspace .analysis-layout .toolbar-time){display:none;}}
@media(max-width:940px){:where(.player-columns):is(.players-workspace .analysis-layout:not(.compact)>.player-columns){grid-template-columns:230px minmax(0,1fr);gap:10px;}}
@media(max-width:940px){:where(.history-actions):is(.players-workspace .analysis-layout .history-actions){gap:6px;}}
@media(max-width:940px){:where(.history-filter):is(.players-workspace .analysis-layout .history-filter){gap:6px;}}
:where(.analysis-sidebar):is(.analysis-sidebar) :deep(>*){flex:none;}
:where(.analysis-overview):is(.analysis-overview){flex:none;border:1px solid var(--border);border-radius:6px;background:var(--panel);margin-bottom:8px;min-width:0;}
:where(.analysis-overview):is(.analysis-overview) :deep(>summary){display:flex;align-items:center;gap:16px;padding:8px 12px;font-size:12px;cursor:pointer;list-style:none;}
:where(.analysis-overview):is(.analysis-overview) :deep(>summary::-webkit-details-marker){display:none;}
:where(.analysis-overview):is(.analysis-overview) :deep(>summary>strong){font-size:13px;}
:where(.analysis-overview):is(.analysis-overview) :deep(>summary>span){color:var(--muted);white-space:nowrap;}
:where(.analysis-overview):is(.analysis-overview) :deep(>summary b){color:var(--text);font-weight:600;margin-left:5px;}
:where(.analysis-overview):is(.analysis-overview) :deep(>summary small){font-size:11px;color:var(--muted);margin-left:auto;white-space:nowrap;}
:where(.overview-expand):is(.analysis-overview .overview-expand){font-size:11px;color:var(--accent);}
:where(.overview-expand):is(.analysis-overview[open] .overview-expand){visibility:hidden;}
@media(max-width:1100px){:where(.analysis-overview):is(.analysis-overview) :deep(>summary){gap:10px;}}
@media(max-width:1100px){:where(.analysis-overview):is(.analysis-overview) :deep(>summary small){display:none;}}
@media(max-width:1100px){:where(.overview-expand):is(.analysis-overview .overview-expand){margin-left:auto;}}
:where(.history-column):is(.players-workspace .analysis-layout .history-column) :deep(>.quality-warning){padding:4px 10px;margin-bottom:5px;}
:where(.player-view.analysis-layout:not(.compact)):is(.players-workspace>.player-view.analysis-layout:not(.compact)){padding-top:10px;padding-bottom:12px;}
</style>
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
