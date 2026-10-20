<script setup lang="ts">
import { computed } from "vue";
import type { Participant, Snapshot } from "../../bindings/leaguepingo/internal/core/models";
import { date, decimal, duration, kda, outcome, percent, playerName, queueName, sampleStats } from "../presentation";
import { championName } from "../gameAssets";
import { rankPresentation } from "../rankPresentation";
import AppIcon from "./AppIcon.vue";
import ChampionPortrait from "./ChampionPortrait.vue";
import ParticipantAvatar from "./ParticipantAvatar.vue";
import GameIcon from "./GameIcon.vue";
import RecentForm from "./RecentForm.vue";

const props = defineProps<{ player: Participant | null; snapshot: Snapshot | null; status: string; loading: boolean; busy: boolean; error: string }>();
defineEmits<{ open: [player: Participant]; match: []; refresh: [] }>();
const matches = computed(() => (props.player?.history ?? []).slice(0, 20));
const stats = computed(() => sampleStats(matches.value));
const mains = computed(() => [...new Set(matches.value.map(m => m.championId).filter(Boolean))]
  .map(id => {
    const sample = matches.value.filter(m => m.championId === id);
    return { id, ...sampleStats(sample) };
  }).sort((a, b) => b.games - a.games).slice(0, 5));
const ranks = computed(() => props.player ? [
  { queue: "RANKED_SOLO_5x5", queueLabel: "单双排", ...rankPresentation(props.player) },
  { queue: "RANKED_FLEX_SR", queueLabel: "灵活排位", ...rankPresentation(props.player, "RANKED_FLEX_SR") },
] : []);
const historyMessage = computed(() => {
  if (!props.snapshot?.connected) return "连接游戏客户端后，查看自己的近期表现。";
  if (props.loading || props.player?.historyState === "loading") return "正在准备你的近期战绩…";
  if (props.player?.historyState === "empty") return "暂时没有可展示的战绩。";
  if (props.player?.historyState === "failed") return "战绩暂时不可用，可刷新重试。";
  if (props.snapshot.stage !== "idle") return "本局资料准备后会同步到这里，也可前往对局室查看。";
  return "当前账号资料尚未就绪，可稍后刷新。";
});
</script>

<template>
  <section class="home-workspace" aria-label="个人概览">
    <header class="home-heading">
      <div><span class="section-eyebrow">YOUR GAME / 我的游戏</span><h1>概览</h1></div>
      <button class="secondary" :disabled="busy || loading || !snapshot?.connected || !snapshot?.selfKey" @click="$emit('refresh')">
        <AppIcon name="refresh" :size="15" />{{ loading ? "准备中…" : "刷新数据" }}
      </button>
    </header>
    <div class="home-hero">
      <div v-if="mains.length" class="home-hero-art" aria-hidden="true"><ChampionPortrait :id="mains[0]?.id" /></div>
      <div class="home-account">
        <ParticipantAvatar v-if="player" :player="player" account :size="64" />
        <span v-else class="home-avatar-empty"><AppIcon name="user" :size="30" /></span>
        <div>
          <span class="home-account-caption">{{ snapshot?.connected ? "当前账号" : "欢迎来到 LeaguePingo" }}</span>
          <h2>{{ player ? playerName(player) : snapshot?.connected ? "正在识别玩家" : "准备好下一场精彩" }}</h2>
          <p><span v-if="player?.profile?.level != null">等级 {{ player.profile.level }} · </span>{{ status.split(" · ")[0] }}<span v-if="player?.stale || !snapshot?.connected && player"> · 旧资料</span></p>
          <span v-if="player?.privacyState === 'private'" class="home-privacy">生涯不公开</span>
        </div>
      </div>
      <div class="home-form-summary">
        <div><span>近期胜率</span><strong>{{ percent(stats.winRate) }}</strong><small>{{ stats.outcomeGames ? `${stats.wins} 胜 / ${stats.losses} 负` : "暂无胜负样本" }}</small></div>
        <div><span>汇总 KDA</span><strong>{{ decimal(stats.kda) }}</strong><small>{{ stats.kdaGames }} 场有效样本</small></div>
        <div class="home-form-last"><span>最近表现</span><RecentForm v-if="matches.length" :matches="matches" /><strong v-else>—</strong><small>最近 {{ matches.length }} / 20 场 · 全部模式</small></div>
      </div>
    </div>
    <p v-if="error" class="error-banner home-error" role="status">{{ error }}</p>
    <div class="home-grid">
      <aside class="home-ranks">
        <section v-for="rank in ranks" :key="rank.queue" class="home-panel home-rank" :title="rank.title">
          <span>{{ rank.queueLabel }}</span><AppIcon name="shield" :size="32" />
          <strong>{{ rank.label }}<small v-if="rank.retained"> · 旧</small></strong>
          <small>{{ rank.rank?.wins != null && rank.rank?.losses != null ? `${rank.rank.wins} 胜 / ${rank.rank.losses} 负` : "赛季胜负暂缺" }}</small>
        </section>
        <section v-if="!player" class="home-panel home-rank"><span>我的段位</span><AppIcon name="shield" :size="32" /><strong>等待账号资料</strong><small>连接客户端后显示</small></section>
        <section class="home-panel home-session">
          <span class="home-panel-caption">当前状态</span><strong><i class="connection-light" :class="{ online: snapshot?.connected }" />{{ status.split(" · ")[0] }}</strong>
          <p>{{ snapshot?.stage === 'in-game' ? "游戏中按住 ~ 查看本局资料，松开隐藏。" : "进入房间或选人后，在对局室查看本局玩家。" }}</p>
          <button class="secondary" @click="$emit('match')">查看当前对局 <AppIcon name="chevron" :size="14" /></button>
        </section>
      </aside>
      <section class="home-panel home-recent">
        <header><h2>最近战绩</h2><button class="text-button" :disabled="!player || busy || loading" @click="player && $emit('open', player)">完整战绩 <AppIcon name="chevron" :size="14" /></button></header>
        <div v-if="!matches.length" class="home-no-history"><AppIcon name="history" :size="34" /><h3>{{ loading || player?.historyState === 'loading' ? "战绩准备中" : "开启你的游戏概览" }}</h3><p>{{ historyMessage }}</p></div>
        <button v-for="match in matches.slice(0, 5)" :key="match.gameId" class="home-match" :class="{ won: match.win === true, lost: match.win === false }" :disabled="busy || loading" @click="player && $emit('open', player)">
          <GameIcon :id="match.championId" :size="40" />
          <span class="home-match-context"><b>{{ queueName(match.queueId) }}</b><small>{{ date(match.playedAt) }}</small></span>
          <span class="home-match-score"><b>{{ kda(match) }}</b><small>{{ championName(match.championId) }}</small></span>
          <span class="home-match-result"><b>{{ outcome(match) }}</b><small>{{ duration(match.duration) }}</small></span>
          <AppIcon name="chevron" :size="14" />
        </button>
        <footer v-if="matches.length">最近 {{ matches.length }} 场已获取记录 · {{ stats.outcomeGames }} 场胜负有效</footer>
      </section>
      <section class="home-panel home-champions">
        <header><h2>近期常用</h2><span>近 {{ matches.length }} 场</span></header>
        <div v-for="hero in mains" :key="hero.id" class="home-champion">
          <GameIcon :id="hero.id" :size="40" /><div><strong>{{ championName(hero.id) }}</strong><small>{{ hero.games }} 场 · {{ decimal(hero.kda) }} KDA</small></div><b>{{ percent(hero.winRate) }}</b>
        </div>
        <div v-if="!mains.length" class="home-no-champions"><AppIcon name="match" :size="28" /><p>有近期战绩后，展示你常玩的英雄。</p></div>
        <p class="home-sample-note">按已获取的近期战绩统计，不代表赛季数据。</p>
      </section>
    </div>
  </section>
</template>
