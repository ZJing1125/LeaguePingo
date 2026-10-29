<script setup lang="ts">
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
        <details v-if="!compact" class="analysis-overview"><summary><strong>近期分析</strong><span>{{stats.outcomeGames?`${stats.wins} 胜 ${stats.losses} 负`:'胜负待获取'}} <b class="analysis-sample-win">{{percent(stats.winRate)}}</b></span><span>KDA <b class="analysis-sample-kda">{{decimal(stats.kda)}}</b></span><small class="analysis-scope">{{sampleLabel}}</small><span class="overview-expand">展开分析</span></summary><OverviewPerformance :matches="analysisMatches" :scope="sampleLabel"/></details>
        <div class="history-toolbar">
          <div class="history-filter">
            <strong class="history-count"
              >战绩 <span>{{ matches.length }}</span></strong
            ><label
              ><span class="sr-only">模式</span
              ><select
                v-model="state.queue"
                aria-label="战绩模式筛选"
                :disabled="busy"
                @change="changeCriteria"
              >
                <option value="all">全部模式</option>
                <option
                  v-if="
                    state.queue !== 'all' &&
                    !modes.includes(Number(state.queue))
                  "
                  :value="state.queue"
                >
                  {{ queueName(Number(state.queue)) }}（无记录）
                </option>
                <option v-for="mode in modes" :key="mode" :value="String(mode)">
                  {{ queueName(mode) }}
                </option>
              </select></label
            >
            <select
              v-model.number="state.sampleSize"
              aria-label="分析样本场数"
              :disabled="busy"
              @change="changeCriteria"
            >
              <option v-for="size in SAMPLE_SIZES" :key="size" :value="size">
                最近 {{ size }} 场
              </option>
            </select>
          </div>
          <div class="history-actions">
            <button v-if="!compact" class="text-button profile-toggle" :aria-expanded="!profileCollapsed" @click="profileCollapsed = !profileCollapsed">{{ profileCollapsed ? '展开资料' : '收起资料' }}</button>
            <span
              class="toolbar-time"
              :title="`战绩获取于 ${date(player.historyMeta?.fetchedAt)}`"
              ><HistoryServiceStatus inline readonly :player="player"/></span
            ><button
              class="secondary"
              :disabled="busy || !connected || player.identityState !== 'known'"
              @click="emit('refresh', player)"
            >
              <AppIcon name="refresh" :size="14" />{{
                refreshing ? "刷新中…" : "刷新数据"
              }}
            </button>
          </div>
        </div>
        <HistoryServiceStatus :player="player" :connected="connected" :busy="busy || state.pageBusy || state.warmBusy" :readonly="compact || captured || prepared" :automatic="automaticHistory && active!==false && state.expanded.length===0 && (state.page??1)===1" @recovered="emit('recovered',$event)" @retry="emit('refresh',player)"/>
        <details v-if="compact && player.scout" class="scout-explanation">
          <summary>本局统计说明 · {{ scouting.queueLabel }}</summary>
          <p v-if="scouting.hero?.games != null && scouting.report?.season">{{ scouting.report.season.name }} · 本英雄{{ scouting.report.complete ? '全部模式' : '已读全模式' }} {{ scouting.hero.games }} 场。</p>
          <p v-if="scouting.hero?.mastery">英雄熟练度 {{ scouting.hero.mastery.level }} 级 · {{ scouting.hero.mastery.points.toLocaleString() }} 分，独立于赛季场次。</p>
          <p>排位统计截至 {{ date(scouting.report?.fetchedAt) }}{{ scouting.report?.retained ? '，本次保留上次结果' : '' }}。近30天胜负有效 {{ scouting.overall.outcomeGames ?? 0 }} 场，KDA 有效 {{ scouting.overall.kdaGames ?? 0 }} 场。{{ scouting.report?.complete30 ? '' : '当前为已读范围。' }}</p>
          <p>近30天位置：{{ scouting.role.positions.map(p => `${roleNames[p.position]} ${p.games} 场`).join('、') || '暂无有效记录' }}；不代表本局分路。</p>
        </details>
        <details
          v-if="noticePlayer.historyMeta?.excluded"
          class="quality-warning"
        >
          <summary>
            {{ noticePlayer.historyMeta.excluded }} 条记录未纳入统计
            <span>查看原因</span>
          </summary>
          <p>
            数据不完整：{{ noticePlayer.historyMeta.returned }} 条中核验
            {{ noticePlayer.historyMeta.parsed }} 场，排除或去重
            {{ noticePlayer.historyMeta.excluded }} 条。统计只使用已核验记录。
          </p>
        </details>
        <div v-if="compact" class="compact-stats">
          <span>{{ stats.games }} / {{ state.sampleSize }} 场</span
          ><span
            >胜率 <b>{{ percent(stats.winRate) }}</b></span
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
            >正在查找{{
              state.queue === "all" ? "战绩" : queueName(Number(state.queue))
            }}{{
              (state.browseTarget ?? 1) > 1
                ? `第 ${state.browseTarget} 页`
                : ""
            }}，已找到 {{ matches.length }} 场…</span
          >
          <span v-else-if="state.warmBusy">正在后台准备单双排与灵活数据；当前资料可正常浏览和操作。</span>
          <span v-else-if="state.pageError" class="warning-text"
            >{{ state.pageError }} · 已有记录保留</span
          >
          <span v-else>{{
            prepared ? "当前为本局已准备战绩，查询最新战绩可获取更多记录。" : captured ? "当前为赛前保留战绩，查询最新战绩可获取更多记录。" : state.browseNotice || "当前样本尚未补齐，可按需继续获取。"
          }}</span>
          <button
            v-if="state.pageBusy"
            class="text-button"
            :disabled="state.stopRequested"
            @click="state.stopRequested = true"
          >
            {{ state.stopRequested ? "正在停止…" : "停止查找" }}
          </button>
          <button
            v-else-if="!state.warmBusy && state.hasMore && canFetch"
            class="text-button"
            :disabled="busy"
            @click="fill(state.browseTarget ?? state.page ?? 1)"
          >
            {{
              state.pageError
                ? "重试"
                : state.browseNotice
                  ? "继续查找"
                  : "补齐样本"
            }}
          </button>
        </div>
        <div class="history-panel">
          <div ref="historyScroll" class="history-scroll">
            <div v-if="!matches.length" class="empty-state">
              <AppIcon name="history" :size="32" /><strong>{{
                ["pending", "failed", "loading"].includes(player.historyState)
                  ? captured ? "赛前战绩未完整保留" : "资料尚未准备完成"
                  : state.pageError || state.sourceLimit
                    ? "尚未取得该模式的战绩"
                    : "当前没有可展示的战绩"
              }}</strong>
              <p>
                {{
                  player.identityState === "known"
                    ? state.pageError || state.sourceLimit
                      ? "数据获取受限，不能据此判断该玩家没有此模式战绩。"
                      : prepared ? "本局已准备记录中暂无对应模式战绩，可查询最新战绩。" : captured ? "保留记录中暂无对应模式战绩，可查询最新战绩。" : "可以刷新战绩，或切换模式筛选。"
                    : "客户端尚未提供有效身份。"
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
          <nav class="history-pagination" aria-label="战绩分页">
            <span
              >{{
                pageMatches.length
                  ? `${((state.page ?? 1) - 1) * HISTORY_PAGE_SIZE + 1}–${((state.page ?? 1) - 1) * HISTORY_PAGE_SIZE + pageMatches.length} 场`
                  : "暂无记录"
              }}<small>每页 {{ HISTORY_PAGE_SIZE }} 场</small></span
            >
            <div class="page-controls">
              <button
                class="secondary"
                :disabled="busy || state.pageBusy || (state.page ?? 1) <= 1"
                @click="goPage((state.page ?? 1) - 1)"
              >
                上一页
              </button>
              <span class="page-indicator"
                >第 {{ state.page ?? 1 }} 页<small>{{
                  state.hasMore || state.sourceLimit
                    ? `已找到 ${knownPages} 页`
                    : `共 ${knownPages} 页`
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
              >
                下一页
              </button>
            </div>
            <form class="page-jump" @submit.prevent="jumpPage" novalidate>
              <label
                >到第
                <input
                  v-model="jumpDraft"
                  aria-label="跳转页码"
                  :aria-invalid="!!jumpError"
                  :title="`可输入 1–${maxJumpPage} 页；未获取的页按需查找`"
                  inputmode="numeric"
                  autocomplete="off"
                  maxlength="3"
                  :disabled="busy || state.pageBusy"
                />页
              </label>
              <button class="secondary" :disabled="busy || state.pageBusy">
                跳转
              </button>
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
