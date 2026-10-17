<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import {
  date,
  decimal,
  duration,
  number,
  tierName,
  kda,
  outcome,
  percent,
  queueName,
  mapName,
  positionName,
  runeName,
  sampleStats,
  type PlayerViewState,
} from "../presentation";
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
const copyNotice = ref("");
const avatarBroken = ref(false);
watch(
  [() => props.player.key, () => props.player.profile?.profileIconData],
  () => {
    avatarBroken.value = false;
  },
);
const hasRetainedProfile = computed(() =>
  Object.values(props.player.profile?.fields ?? {}).some(
    (field) => field?.state === "retained",
  ),
);
function fieldTitle(field: string) {
  const meta = props.player.profile?.fields?.[field];
  return `${meta?.state === "retained" ? "本次未更新，保留旧值" : "资料获取时间"} · ${date(meta?.fetchedAt)}`;
}
async function copyID() {
  try {
    await navigator.clipboard.writeText(props.player.displayName);
    copyNotice.value = "已复制完整 ID";
  } catch {
    copyNotice.value = "复制未成功，可选中名称手动复制";
  }
}
function toggle(id: number) {
  const i = props.state.expanded.indexOf(id);
  if (i < 0) props.state.expanded.push(id);
  else props.state.expanded.splice(i, 1);
}
</script>
<template>
  <section class="player-view" :class="{ compact }">
    <header class="player-profile">
      <div
        class="profile-avatar"
        :title="
          player.profile?.profileIconData
            ? fieldTitle('avatar')
            : '玩家头像尚未获取'
        "
      >
        <img
          v-if="player.profile?.profileIconData && !avatarBroken"
          :src="player.profile.profileIconData"
          alt="玩家头像"
          @error="avatarBroken = true"
        />
        <AppIcon v-else name="user" :size="26" />
      </div>
      <div class="profile-identity">
        <div class="identity-title">
          <h1 :title="player.displayName">{{ player.displayName }}</h1>
          <button
            class="icon-button square"
            aria-label="复制玩家完整 ID"
            title="复制完整 ID"
            @click="copyID"
          >
            <AppIcon name="copy" :size="15" />
          </button>
        </div>
        <div class="profile-meta">
          <span :title="fieldTitle('level')"
            >{{
              player.profile?.level != null
                ? `等级 ${player.profile.level}`
                : "等级 —"
            }}<small
              v-if="player.profile?.fields?.level?.state === 'retained'"
              class="warning-text"
            >
              · 旧</small
            ></span
          ><span v-if="player.stale" class="status-badge warning">旧资料</span
          ><span v-else-if="player.cached">缓存资料</span
          ><span
            v-if="player.privacyState === 'private'"
            class="status-badge"
            :title="fieldTitle('privacy')"
            >生涯不公开（客户端明确返回）<small
              v-if="player.profile?.fields?.privacy?.state === 'retained'"
              class="warning-text"
            >
              · 旧</small
            ></span
          ><span
            v-else-if="player.privacyState === 'unknown'"
            class="privacy-unknown"
            >隐私未核验</span
          >
          <span
            v-if="player.privacyState === 'public'"
            :title="fieldTitle('privacy')"
            >生涯公开<small
              v-if="player.profile?.fields?.privacy?.state === 'retained'"
              class="warning-text"
            >
              · 旧</small
            ></span
          >
        </div>
      </div>
      <div v-if="!compact" class="profile-ranks">
        <div
          v-for="queue in ['RANKED_SOLO_5x5', 'RANKED_FLEX_SR']"
          :key="queue"
          class="rank-card"
        >
          <AppIcon name="shield" :size="24" />
          <div>
            <small>{{
              queue === "RANKED_SOLO_5x5" ? "单双排" : "灵活排位"
            }}</small
            ><template
              v-for="rank in (player.profile?.ranks ?? []).filter(
                (r) => r.queue === queue,
              )"
              :key="rank.queue"
              ><strong :title="fieldTitle('rank')"
                >{{ tierName(rank.tier) }} {{ rank.division }}
                <span>{{ number(rank.lp) }} LP</span
                ><small
                  v-if="player.profile?.fields?.rank?.state === 'retained'"
                  class="warning-text"
                  >旧段位</small
                ></strong
              ><small
                >{{ number(rank.wins) }} 胜 /
                {{ number(rank.losses) }} 负</small
              ></template
            ><strong
              v-if="
                !(player.profile?.ranks ?? []).some((r) => r.queue === queue)
              "
              class="muted"
              >段位未获取</strong
            >
          </div>
        </div>
      </div>
      <button
        class="icon-button profile-update"
        aria-label="更新玩家资料"
        title="更新等级、段位与隐私"
        :disabled="
          busy || !connected || player.stale || player.identityState !== 'known'
        "
        @click="emit('profile', player)"
      >
        <AppIcon name="refresh" :size="16" /><span>更新资料</span>
      </button>
    </header>
    <p v-if="hasRetainedProfile" class="profile-freshness" role="status">
      部分资料未更新，标“旧”的字段保留了上次结果；悬停查看获取时间。
    </p>
    <p v-if="copyNotice" class="inline-status" role="status">
      {{ copyNotice }}
    </p>
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
            <article
              v-for="match in pageMatches"
              :key="match.gameId"
              class="match-card"
              :class="
                match.win == null ? 'unknown' : match.win ? 'won' : 'lost'
              "
            >
              <button
                class="match-main"
                :aria-expanded="state.expanded.includes(match.gameId)"
                @click="toggle(match.gameId)"
              >
                <span class="match-context"
                  ><strong>{{ queueName(match.queueId) }}</strong
                  ><small>{{ date(match.playedAt) }}</small
                  ><b class="outcome"
                    >{{ outcome(match) }}
                    <span>{{ duration(match.duration) }}</span></b
                  ></span
                >
                <span class="match-hero"
                  ><GameIcon
                    :id="match.championId"
                    :size="compact ? 32 : 40"
                  /><small>{{
                    championName(match.championId).split("·")[0]
                  }}</small></span
                >
                <span class="match-spells"
                  ><GameIcon
                    kind="spells"
                    :id="match.spell1"
                    :size="19" /><GameIcon
                    kind="spells"
                    :id="match.spell2"
                    :size="19"
                /></span>
                <span class="match-performance"
                  ><strong>{{ kda(match) }}</strong
                  ><small>{{ decimal(sampleStats([match]).kda) }} KDA</small
                  ><small
                    >补刀 {{ number(match.cs) }} · 伤害
                    {{ number(match.damage) }}</small
                  ></span
                >
                <span class="match-items" aria-label="本玩家终局装备"
                  ><GameIcon
                    v-for="slot in 7"
                    :key="slot"
                    kind="items"
                    :id="match.items?.[slot - 1]"
                    :size="compact ? 17 : 24"
                /></span>
                <span class="expand-indicator"
                  ><AppIcon name="chevron" :size="16"
                /></span>
              </button>
              <div
                v-if="state.expanded.includes(match.gameId)"
                class="match-expanded"
              >
                <div class="detail-metrics">
                  <span>时长 {{ duration(match.duration) }}</span
                  ><span>补刀 {{ number(match.cs) }}</span
                  ><span>金币 {{ number(match.gold) }}</span
                  ><span>英雄伤害 {{ number(match.damage) }}</span>
                </div>
                <div class="match-context-detail" aria-label="对局上下文">
                  <span>{{ mapName(match.mapId) }}</span>
                  <span>{{ positionName(match.position) }}</span>
                  <span v-if="match.lane && !match.position"
                    >客户端线路：{{ match.lane }}</span
                  >
                  <span
                    v-if="match.earlyEnd === true"
                    class="early-end"
                    title="客户端标记提前结束；不等同于确认重开，不自动从统计中排除"
                    >提前结束</span
                  >
                  <span v-else-if="match.earlyEnd == null"
                    >提前结束信号未获取</span
                  >
                </div>
                <div class="match-runes" aria-label="本玩家符文">
                  <template v-if="match.runeIds?.length">
                    <span
                      v-for="(rune, index) in match.runeIds"
                      :key="`${rune}:${index}`"
                      :title="`符文 #${rune}`"
                      >{{ runeName(rune) }}</span
                    >
                    <small v-if="match.runePrimary || match.runeSecondary"
                      >{{
                        match.runePrimary
                          ? runeName(match.runePrimary)
                          : "主系未知"
                      }}
                      /
                      {{
                        match.runeSecondary
                          ? runeName(match.runeSecondary)
                          : "副系未知"
                      }}</small
                    >
                  </template>
                  <small v-else>符文未获取</small>
                </div>
                <MatchDetails
                  :player="player"
                  :game-id="match.gameId"
                  :connected="connected"
                /><span class="detail-note"
                  >对局 #{{ match.gameId }} · 双方详情按需读取 · 缺失字段显示
                  —</span
                >
              </div>
            </article>
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
