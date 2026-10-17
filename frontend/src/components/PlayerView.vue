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
  sampleStats,
  errorMessage,
  type PlayerViewState,
} from "../presentation";
import HistoryNotice from "./HistoryNotice.vue";
import GameIcon from "./GameIcon.vue";
import MatchDetails from "./MatchDetails.vue";
import { championName } from "../gameAssets";
const props = defineProps<{
  player: Participant;
  state: PlayerViewState;
  busy?: boolean;
  connected?: boolean;
  compact?: boolean;
  error?: string;
}>();
const emit = defineEmits<{ refresh: [player: Participant]; profile:[player:Participant] }>();
watch(() => [props.player.key, props.player.connectionId, props.player.historyMeta?.fetchedAt], () => {
  const stamp = `${props.player.connectionId}:${props.player.historyMeta?.fetchedAt}`;
  if (props.state.sourceStamp === stamp) return;
  Object.assign(props.state, {sourceStamp:stamp, extra:[], pageMeta:[], nextOffset:props.player.historyMeta?.returned ?? props.player.history?.length ?? 0, hasMore:(props.player.historyMeta?.returned ?? 0) >= 20, pageBusy:false, pageError:''});
}, {immediate:true});
const allMatches = computed(() => [...new Map([...(props.player.history ?? []), ...(props.state.extra ?? [])].map(m => [m.gameId, m])).values()]);
const noticePlayer = computed(() => {
  const metas = [props.player.historyMeta, ...(props.state.pageMeta ?? [])].filter(Boolean);
  const duplicates = metas.reduce((n,m)=>n+m.parsed,0) - allMatches.value.length;
  return {...props.player, historyMeta:{...props.player.historyMeta, returned:metas.reduce((n,m)=>n+m.returned,0), parsed:allMatches.value.length, excluded:metas.reduce((n,m)=>n+m.excluded,0)+Math.max(0,duplicates), scope:`已加载 ${allMatches.value.length} 场混合模式记录；筛选只作用于已加载样本`}};
});
async function loadMore() {
  if (props.state.pageBusy || props.busy || !props.connected) return;
  const target = props.player, state = props.state, stamp = state.sourceStamp;
  state.pageBusy = true; state.pageError = '';
  try {
    const page = await Service.HistoryPage(target.key, target.connectionId, state.nextOffset ?? 0);
    if (state.sourceStamp !== stamp) return;
    state.extra = [...(state.extra ?? []), ...(page.matches ?? [])];
    state.pageMeta = [...(state.pageMeta ?? []), page.meta];
    state.nextOffset = page.nextOffset; state.hasMore = page.hasMore;
  } catch (reason) { if (state.sourceStamp === stamp) state.pageError = errorMessage(reason); }
  finally { if (state.sourceStamp === stamp) state.pageBusy = false; }
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
const copyNotice = ref("");
const heroes = computed(() => {
  const ids = [
    ...new Set(matches.value.map((m) => m.championId).filter(Boolean)),
  ];
  return ids
    .map((id) => ({
      id,
      stats: sampleStats(matches.value.filter((m) => m.championId === id)),
    }))
    .sort((a, b) => b.stats.games - a.stats.games)
    .slice(0, 5);
});
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
      <div class="profile-avatar" aria-hidden="true">
        {{ player.displayName.slice(0, 1) }}
      </div>
      <div class="profile-identity">
        <h1 :title="player.displayName">{{ player.displayName }}</h1>
        <div class="profile-meta">
          <span
            >{{
              player.profile?.level != null
                ? `等级 ${player.profile.level}`
                : "等级未获取"
            }}
            · 当前客户端资料</span
          ><span v-if="player.stale" class="warning-text">旧资料</span
          ><span v-else-if="player.cached">缓存</span>
        </div>
      </div>
      <button class="icon-button" aria-label="复制玩家完整 ID" @click="copyID">
        复制 ID
      </button>
      <button
        class="secondary"
        :disabled="
          busy || !connected || player.stale || player.identityState !== 'known'
        "
        @click="emit('profile', player)"
      >
        更新玩家资料
      </button>
      <button
        class="primary subtle"
        :disabled="busy || !connected || player.identityState !== 'known'"
        @click="emit('refresh', player)"
      >
        {{ busy ? "查询中…" : "刷新战绩" }}
      </button>
    </header>
    <p v-if="copyNotice" class="inline-status" role="status">
      {{ copyNotice }}
    </p>
    <p v-if="error" class="error-banner" role="alert">
      {{ error }} · 已有资料保留
    </p>
    <div class="player-columns">
      <aside v-if="!compact" class="player-summary">
        <h2>近期表现</h2>
        <p class="muted">
          {{
            state.queue === "all" ? "当前已获取样本" : "已获取样本中的所选模式"
          }}
        </p>
        <dl class="summary-numbers">
          <div>
            <dt>胜 / 负</dt>
            <dd>{{ stats.wins }} / {{ stats.losses }}</dd>
          </div>
          <div>
            <dt>胜率</dt>
            <dd class="accent">{{ percent(stats.winRate) }}</dd>
          </div>
          <div>
            <dt>汇总 KDA</dt>
            <dd>{{ decimal(stats.kda) }}</dd>
          </div>
          <div>
            <dt>样本场次</dt>
            <dd>{{ stats.games }}</dd>
          </div>
        </dl>
        <h2>排位资料</h2>
        <p v-if="!player.profile?.ranks?.length" class="muted">
          段位未获取，不代表未定级<br />可点击“更新玩家资料”尝试读取
        </p>
        <div
          v-for="rank in player.profile?.ranks ?? []"
          :key="rank.queue"
          class="rank-summary"
        >
          <small>{{
            rank.queue === "RANKED_SOLO_5x5" ? "单人 / 双人" : "灵活排位"
          }}</small
          ><strong
            >{{ tierName(rank.tier) }} {{ rank.division }} ·
            {{ number(rank.lp) }} LP</strong
          ><small
            >{{ number(rank.wins) }} 胜 / {{ number(rank.losses) }} 负</small
          >
        </div>
        <h2 class="summary-heading">近期常用英雄</h2>
        <p v-if="!heroes.length" class="muted">暂无有效英雄样本</p>
        <div v-for="hero in heroes" :key="hero.id" class="common-hero">
          <GameIcon :id="hero.id" /><span
            >{{ championName(hero.id)
            }}<small
              >{{ hero.stats.games }} 场 · 胜率
              {{ percent(hero.stats.winRate) }}</small
            ><small>KDA {{ decimal(hero.stats.kda) }} · 当前样本</small></span
          >
        </div>
        <p class="privacy-line">
          生涯隐私：{{
            player.privacyState === "private"
              ? "生涯不公开（客户端明确返回）"
              : player.privacyState === "public"
                ? "公开"
                : "未核验"
          }}
        </p>
      </aside>
      <div class="history-column">
        <div class="history-toolbar">
          <span class="history-count">{{ matches.length }} 场战绩</span
          ><label
            >模式
            <select v-model="state.queue" aria-label="战绩模式筛选">
              <option value="all">所有已获取模式</option>
              <option
                v-if="
                  state.queue !== 'all' && !modes.includes(Number(state.queue))
                "
                :value="state.queue"
              >
                {{ queueName(Number(state.queue)) }}（样本无记录）
              </option>
              <option v-for="mode in modes" :key="mode" :value="String(mode)">
                {{ queueName(mode) }}
              </option>
            </select></label
          ><span class="toolbar-time">{{
            date(player.historyMeta?.fetchedAt)
          }}</span>
        </div>
        <div
          v-if="noticePlayer.historyMeta?.excluded"
          class="quality-warning"
          role="status"
        >
          数据不完整：{{ noticePlayer.historyMeta.returned }} 条中核验
          {{ noticePlayer.historyMeta.parsed }} 场，排除或去重
          {{ noticePlayer.historyMeta.excluded }} 条。
        </div>
        <div v-if="compact" class="compact-stats">
          <span>{{ stats.games }} 场</span
          ><span>胜率 {{ percent(stats.winRate) }}</span
          ><span>KDA {{ decimal(stats.kda) }}</span>
        </div>
        <div class="history-scroll">
          <div v-if="!matches.length" class="empty-state">
            <strong>{{
              player.historyState === "pending" ||
              player.historyState === "failed"
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
                ><b class="outcome">{{ outcome(match) }}</b></span
              >
              <span class="match-hero"
                ><GameIcon :id="match.championId" /><small>{{
                  championName(match.championId)
                }}</small></span
              >
              <span class="match-performance"
                ><strong>{{ kda(match) }}</strong
                ><small>K / D / A · {{ duration(match.duration) }}</small
                ><small
                  >补刀 {{ number(match.cs) }} · 伤害
                  {{ number(match.damage) }}</small
                ></span
              ><span class="expand-indicator"
                >{{
                  state.expanded.includes(match.gameId) ? "收起" : "详情"
                }}
                ›</span
              >
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
              <MatchDetails
                :player="player"
                :game-id="match.gameId"
                :connected="connected"
              />
              <div class="match-loadout" aria-label="本玩家终局装备与技能">
                <GameIcon
                  v-for="(item, i) in match.items ?? []"
                  :key="i"
                  kind="items"
                  :id="item"
                  :size="30"
                /><span class="loadout-divider"></span
                ><GameIcon
                  kind="spells"
                  :id="match.spell1"
                  :size="26"
                /><GameIcon kind="spells" :id="match.spell2" :size="26" />
              </div>
              <span class="detail-note"
                >对局 #{{ match.gameId }} ·
                上方为本玩家摘要，可按需读取双方详情；未知显示
                —；不推断重开、评分或位置。</span
              >
            </div>
          </article>
          <p v-if="state.pageError" class="error-banner" role="alert">
            {{ state.pageError }} · 已加载记录保留
          </p>
          <button
            v-if="state.hasMore"
            class="secondary"
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
