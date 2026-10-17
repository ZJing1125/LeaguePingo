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
import GameIcon from "./GameIcon.vue";
import AppIcon from "./AppIcon.vue";
import MatchDetails from "./MatchDetails.vue";
import RecentForm from "./RecentForm.vue";
import TransientNotice from "./TransientNotice.vue";
import { championName } from "../gameAssets";
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
  error?: string;
}>();
const emit = defineEmits<{
  refresh: [player: Participant];
  profile: [player: Participant];
}>();
const historyScroll = ref<HTMLElement | null>(null);
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
const heroes = computed(() =>
  [...new Set(analysisMatches.value.map((m) => m.championId).filter(Boolean))]
    .map((id) => ({
      id,
      stats: sampleStats(
        analysisMatches.value.filter((m) => m.championId === id),
      ),
    }))
    .sort((a, b) => b.stats.games - a.stats.games)
    .slice(0, 5),
);
function toggle(id: number) {
  const i = props.state.expanded.indexOf(id);
  if (i < 0) props.state.expanded.push(id);
  else props.state.expanded.splice(i, 1);
}
</script>
<template>
  <section class="player-view" :class="{ compact }">
    <PlayerProfileHeader
      :player="player"
      :busy="busy"
      :connected="connected"
      :compact="compact"
      @profile="emit('profile', $event)"
    />
    <TransientNotice :message="error" banner-class="player-error" />
    <div class="player-columns">
      <aside v-if="!compact" class="player-summary">
        <div class="section-heading">
          <h2>近期表现</h2>
          <select
            v-model.number="state.sampleSize"
            aria-label="分析样本场数"
            :disabled="busy || state.pageBusy"
            @change="changeCriteria"
          >
            <option v-for="size in SAMPLE_SIZES" :key="size" :value="size">
              {{ size }} 场
            </option>
          </select>
        </div>
        <p class="sample-scope" role="status">{{ sampleLabel }}</p>
        <div class="winrate-display">
          <strong>{{ percent(stats.winRate) }}</strong
          ><span
            >胜率
            <small>{{ stats.wins }} 胜 / {{ stats.losses }} 负</small></span
          >
        </div>
        <dl class="summary-numbers">
          <div>
            <dt>汇总 KDA</dt>
            <dd>{{ decimal(stats.kda) }}</dd>
          </div>
          <div>
            <dt>有效胜负</dt>
            <dd>{{ stats.outcomeGames }} 场</dd>
          </div>
        </dl>
        <RecentForm :matches="analysisMatches" />
        <div class="section-heading heroes-heading">
          <h2>常用英雄</h2>
          <span>分析样本</span>
        </div>
        <p v-if="!heroes.length" class="muted">暂无有效样本</p>
        <div v-for="hero in heroes" :key="hero.id" class="common-hero">
          <GameIcon :id="hero.id" :size="32" />
          <div>
            <strong :title="championName(hero.id)">{{
              championName(hero.id).split("·")[0]
            }}</strong
            ><small
              >{{ hero.stats.games }} 场 ·
              {{ percent(hero.stats.winRate) }} 胜率</small
            >
          </div>
          <b>{{ decimal(hero.stats.kda) }}</b>
        </div>
        <p class="sample-caption">
          胜率、KDA、近况和常用英雄均使用当前模式最近
          {{ stats.games }} 场；翻页不改变分析范围。
        </p>
      </aside>
      <div class="history-column">
        <div class="history-toolbar">
          <div class="history-filter">
            <strong class="history-count"
              >战绩 <span>{{ matches.length }}</span></strong
            ><label
              ><span class="sr-only">模式</span
              ><select
                v-model="state.queue"
                aria-label="战绩模式筛选"
                :disabled="busy || state.pageBusy"
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
          </div>
          <div class="history-actions">
            <span
              class="toolbar-time"
              :title="`战绩获取于 ${date(player.historyMeta?.fetchedAt)}`"
              >{{ date(player.historyMeta?.fetchedAt) }}</span
            ><button
              class="secondary"
              :disabled="busy || !connected || player.identityState !== 'known'"
              @click="emit('refresh', player)"
            >
              <AppIcon name="refresh" :size="14" />{{
                refreshing ? "刷新中…" : "刷新战绩"
              }}
            </button>
          </div>
        </div>
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
          <select
            v-model.number="state.sampleSize"
            aria-label="分析样本场数"
            :disabled="busy || state.pageBusy"
            @change="changeCriteria"
          >
            <option v-for="size in SAMPLE_SIZES" :key="size" :value="size">
              最近 {{ size }} 场
            </option>
          </select>
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
          <span v-else-if="state.pageError" class="warning-text"
            >{{ state.pageError }} · 已有记录保留</span
          >
          <span v-else>{{
            state.browseNotice || "当前样本尚未补齐，可按需继续获取。"
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
            v-else-if="state.hasMore && canFetch"
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
                  ? "资料尚未准备完成"
                  : state.pageError || state.sourceLimit
                    ? "尚未取得该模式的战绩"
                    : "当前没有可展示的战绩"
              }}</strong>
              <p>
                {{
                  player.identityState === "known"
                    ? state.pageError || state.sourceLimit
                      ? "数据获取受限，不能据此判断该玩家没有此模式战绩。"
                      : "可以刷新战绩，或切换模式筛选。"
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
