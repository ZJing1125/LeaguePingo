<script setup lang="ts">
import { computed, ref, watch } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
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
  errorMessage,
  type PlayerViewState,
} from "../presentation";
import HistoryNotice from "./HistoryNotice.vue";
import GameIcon from "./GameIcon.vue";
import AppIcon from "./AppIcon.vue";
import MatchDetails from "./MatchDetails.vue";
import RecentForm from "./RecentForm.vue";
import { championName } from "../gameAssets";
const props = defineProps<{
  player: Participant;
  state: PlayerViewState;
  busy?: boolean;
  connected?: boolean;
  compact?: boolean;
  error?: string;
}>();
const emit = defineEmits<{
  refresh: [player: Participant];
  profile: [player: Participant];
}>();
watch(
  () => [
    props.player.key,
    props.player.connectionId,
    props.player.historyMeta?.fetchedAt,
  ],
  () => {
    const stamp = `${props.player.connectionId}:${props.player.historyMeta?.fetchedAt}`;
    if (props.state.sourceStamp === stamp) return;
    Object.assign(props.state, {
      sourceStamp: stamp,
      extra: [],
      pageMeta: [],
      nextOffset:
        props.player.historyMeta?.returned ?? props.player.history?.length ?? 0,
      hasMore: (props.player.historyMeta?.returned ?? 0) >= 20,
      pageBusy: false,
      pageError: "",
    });
  },
  { immediate: true },
);
const allMatches = computed(() => {
  const seen = new Set<number>();
  return [...(props.player.history ?? []), ...(props.state.extra ?? [])].filter(
    (m) => {
      if (seen.has(m.gameId)) return false;
      seen.add(m.gameId);
      return true;
    },
  );
});
const noticePlayer = computed(() => {
  const metas = [
    props.player.historyMeta,
    ...(props.state.pageMeta ?? []),
  ].filter(Boolean);
  const duplicates =
    metas.reduce((n, m) => n + m.parsed, 0) - allMatches.value.length;
  return {
    ...props.player,
    historyMeta: {
      ...props.player.historyMeta,
      returned: metas.reduce((n, m) => n + m.returned, 0),
      parsed: allMatches.value.length,
      excluded:
        metas.reduce((n, m) => n + m.excluded, 0) + Math.max(0, duplicates),
      scope: `已加载 ${allMatches.value.length} 场混合模式记录；筛选只作用于已加载样本`,
    },
  };
});
async function loadMore() {
  if (props.state.pageBusy || props.busy || !props.connected) return;
  const target = props.player,
    state = props.state,
    stamp = state.sourceStamp;
  state.pageBusy = true;
  state.pageError = "";
  try {
    const page = await Service.HistoryPage(
      target.key,
      target.connectionId,
      state.nextOffset ?? 0,
    );
    if (state.sourceStamp !== stamp) return;
    state.extra = [...(state.extra ?? []), ...(page.matches ?? [])];
    state.pageMeta = [...(state.pageMeta ?? []), page.meta];
    state.nextOffset = page.nextOffset;
    state.hasMore = page.hasMore;
  } catch (reason) {
    if (state.sourceStamp === stamp) state.pageError = errorMessage(reason);
  } finally {
    if (state.sourceStamp === stamp) state.pageBusy = false;
  }
}
const matches = computed(() =>
  allMatches.value.filter(
    (m) =>
      props.state.queue === "all" || String(m.queueId) === props.state.queue,
  ),
);
const stats = computed(() => sampleStats(matches.value));
const modes = computed(() => [
  ...new Set(allMatches.value.map((m) => m.queueId)),
]);
const heroes = computed(() =>
  [...new Set(matches.value.map((m) => m.championId).filter(Boolean))]
    .map((id) => ({
      id,
      stats: sampleStats(matches.value.filter((m) => m.championId === id)),
    }))
    .sort((a, b) => b.stats.games - a.stats.games)
    .slice(0, 5),
);
const copyNotice = ref("");
const avatarBroken = ref(false);
watch(
  () => props.player.profile?.profileIconData,
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
              queue === "RANKED_SOLO_5x5" ? "单人 / 双人" : "灵活排位"
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
    <p v-if="error" class="error-banner player-error" role="alert">
      {{ error }} · 已有资料保留
    </p>
    <div class="player-columns">
      <aside v-if="!compact" class="player-summary">
        <div class="section-heading">
          <h2>近期表现</h2>
          <span>{{ stats.games }} 场样本</span>
        </div>
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
        <RecentForm :matches="matches" />
        <div class="section-heading heroes-heading">
          <h2>常用英雄</h2>
          <span>当前筛选</span>
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
          {{
            state.queue === "all"
              ? "统计基于已加载战绩"
              : "统计基于已加载战绩中的所选模式"
          }}
        </p>
      </aside>
      <div class="history-column">
        <div class="history-toolbar">
          <div class="history-filter">
            <strong class="history-count"
              >战绩 <span>{{ matches.length }}</span></strong
            ><label
              ><span class="sr-only">模式</span
              ><select v-model="state.queue" aria-label="战绩模式筛选">
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
                busy ? "查询中…" : "刷新战绩"
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
          <span>{{ stats.games }} 场</span
          ><span
            >胜率 <b>{{ percent(stats.winRate) }}</b></span
          ><span
            >KDA <b>{{ decimal(stats.kda) }}</b></span
          >
        </div>
        <div class="history-scroll">
          <div v-if="!matches.length" class="empty-state">
            <AppIcon name="history" :size="32" /><strong>{{
              ["pending", "failed", "loading"].includes(player.historyState)
                ? "资料尚未准备完成"
                : "当前没有可展示的战绩"
            }}</strong>
            <p>
              {{
                player.identityState === "known"
                  ? "可以刷新战绩，或切换模式筛选。"
                  : "客户端尚未提供有效身份。"
              }}
            </p>
          </div>
          <article
            v-for="match in matches"
            :key="match.gameId"
            class="match-card"
            :class="match.win == null ? 'unknown' : match.win ? 'won' : 'lost'"
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
          <p v-if="state.pageError" class="error-banner" role="alert">
            {{ state.pageError }} · 已加载记录保留
          </p>
          <button
            v-if="state.hasMore"
            class="secondary load-more"
            :disabled="state.pageBusy || busy || !connected || player.stale"
            @click="loadMore"
          >
            {{ state.pageBusy ? "正在加载…" : "加载更多战绩" }}
          </button>
          <HistoryNotice
            :player="noticePlayer"
            :stats="stats"
            :filtered="state.queue !== 'all'"
          />
        </div>
      </div>
    </div>
  </section>
</template>
