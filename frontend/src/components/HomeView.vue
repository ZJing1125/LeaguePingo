<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import type { Participant, Snapshot } from "../../bindings/leaguepingo/internal/core/models";
import { date, decimal, duration, kda, outcome, percent, playerName, queueName, sampleStats, type PlayerViewState } from "../presentation";
import { championName } from "../gameAssets";
import { rankPresentation } from "../rankPresentation";
import { SAMPLE_SIZES, useHistoryBrowser } from "../historyBrowser";
import AppIcon from "./AppIcon.vue";
import ChampionSplash from "./ChampionSplash.vue";
import ParticipantAvatar from "./ParticipantAvatar.vue";
import RankEmblem from "./RankEmblem.vue";
import GameIcon from "./GameIcon.vue";
import RecentForm from "./RecentForm.vue";
import HistoryNotice from "./HistoryNotice.vue";
import PlayerDrawer from './PlayerDrawer.vue';
import MatchHistoryRow from './MatchHistoryRow.vue';
import MatchDetails from './MatchDetails.vue';

const props = defineProps<{ player: Participant | null; snapshot: Snapshot | null; status: string; loading: boolean; busy: boolean; error: string; active?: boolean }>();
const emit=defineEmits<{ open: [player: Participant]; match: []; refresh: []; modal:[open:boolean] }>();
const selectedGame=ref<number|null>(null);
const selectedMatch=computed(()=>props.player?.history?.find(m=>m.gameId===selectedGame.value));
watch(()=>`${props.active}:${props.player?.connectionId}:${props.player?.key}`,()=>{selectedGame.value=null;});
watch(selectedMatch,match=>{if(!match)selectedGame.value=null;emit('modal',!!match);});
const state = ref<PlayerViewState>({ queue: "all", expanded: [], sampleSize: 20 });
const historyScroll = ref<HTMLElement | null>(null);
const placeholder = { key: "home-empty", puuid: "", connectionId: 0, identityState: "unavailable", historyState: "pending", history: [], historyMeta: {} } as unknown as Participant;
watch(() => `${props.player?.connectionId}:${props.player?.key}`, () => { state.value.stopRequested = true; state.value = { queue: "all", expanded: [], sampleSize: 20 }; });
const browseProps = reactive({ get player() { return props.player ?? placeholder; }, get state() { return state.value; }, get connected() { return props.snapshot?.connected; }, get busy() { return props.busy || props.loading; }, get active() { return props.active; } });
const { allMatches, matches: filtered, analysisMatches: matches, pageMatches, knownPages, canFetch, sampleLabel, needsSamples, noticePlayer, changeCriteria, goPage, fill, jumpDraft, jumpError, jumpPage, maxJumpPage } = useHistoryBrowser(browseProps, historyScroll);
const stats = computed(() => sampleStats(matches.value));
const modes = computed(() => [...new Set([420, 440, 450, 2400, ...allMatches.value.map(m => m.queueId)])]);
const mains = computed(() => [...new Set(matches.value.map(m => m.championId).filter(Boolean))].map(id => ({ id, ...sampleStats(matches.value.filter(m => m.championId === id)) })).sort((a, b) => b.games - a.games).slice(0, 5));
const ranks = computed(() => props.player ? [
  { queue: "RANKED_SOLO_5x5", queueLabel: "单双排", ...rankPresentation(props.player) },
  { queue: "RANKED_FLEX_SR", queueLabel: "灵活组排", ...rankPresentation(props.player, "RANKED_FLEX_SR") },
] : []);
const historyMessage = computed(() => !props.snapshot?.connected ? "连接游戏客户端后，查看自己的近期表现。" : props.loading || props.player?.historyState === "loading" ? "正在准备你的近期战绩…" : props.player?.historyState === "failed" ? "战绩暂时不可用，可刷新重试。" : state.value.queue !== "all" ? "当前筛选暂无记录，可切换模式或继续查找。" : "暂时没有可展示的战绩。");
</script>
<template>
  <section class="home-workspace" aria-label="个人概览">
    <header class="home-heading"><div><h1>概览</h1></div><button class="secondary" :disabled="busy || loading || !snapshot?.connected || !snapshot?.selfKey" @click="$emit('refresh')"><AppIcon name="refresh" :size="15" />{{ loading ? "准备中…" : "刷新数据" }}</button></header>
    <div class="home-hero">
      <div v-if="mains.length" class="home-hero-art" aria-hidden="true"><ChampionSplash :id="mains[0]?.id" /></div>
      <div class="home-account"><ParticipantAvatar v-if="player" :player="player" account :size="64" /><span v-else class="home-avatar-empty"><AppIcon name="user" :size="30" /></span><div><span class="home-account-caption">{{ snapshot?.connected ? "当前账号" : "等待客户端" }}</span><h2>{{ player ? playerName(player) : snapshot?.connected ? loading ? "正在识别玩家" : "账号资料待获取" : "启动并登录英雄联盟" }}</h2><p><span v-if="player?.profile?.level != null">等级 {{ player.profile.level }} · </span>{{ status.split(" · ")[0] }}<span v-if="player?.stale || !snapshot?.connected && player"> · 旧资料</span></p><span v-if="player?.privacyState === 'private'" class="home-privacy">生涯不公开</span></div></div>
      <div class="home-form-summary"><div><span>近期胜率</span><strong>{{ percent(stats.winRate) }}</strong><small>{{ stats.outcomeGames ? `${stats.wins} 胜 / ${stats.losses} 负` : "暂无胜负样本" }}</small></div><div><span>汇总 KDA</span><strong>{{ decimal(stats.kda) }}</strong><small>{{ stats.kdaGames }} 场有效样本</small></div><div class="home-form-last"><span>最近表现</span><RecentForm v-if="matches.length" :matches="matches" /><strong v-else>—</strong><small>{{ sampleLabel }}</small></div></div>
    </div>
    <p v-if="error" class="error-banner home-error" role="status">{{ error }}</p>
    <div class="home-grid" :class="{ 'sparse-mains': mains.length > 0 && mains.length <= 2 }">
      <aside class="home-ranks">
        <section v-for="rank in ranks" :key="rank.queue" class="home-panel home-rank" :title="rank.title"><span>{{ rank.queueLabel }}</span><RankEmblem :tier="rank.rank?.tier" /><strong>{{ rank.label }}<small v-if="rank.retained"> · 旧</small></strong><small :title="rank.record?.title">{{ rank.record ? `${rank.record.scope} ${rank.record.wins} 胜 / ${rank.record.losses} 负` : "胜负待确认" }}</small></section>
        <section v-if="!player" class="home-panel home-rank"><span>我的段位</span><RankEmblem /><strong>等待账号资料</strong><small>连接客户端后显示</small></section>
        <section class="home-panel home-session"><span class="home-panel-caption">当前状态</span><strong><i class="connection-light" :class="{ online: snapshot?.connected }" />{{ status.split(" · ")[0] }}</strong><p>{{ snapshot?.stage === 'in-game' ? "游戏中按住 ~ 查看本局资料，松开隐藏。" : "进入房间或选人后，客户端左侧展示我方资料。" }}</p><button v-if="snapshot?.review || snapshot?.connected && ['Lobby','Matchmaking','ReadyCheck','ChampSelect','GameStart','InProgress'].includes(snapshot.phase)" :class="snapshot?.connected && ['party', 'preparing', 'in-game'].includes(snapshot.stage) ? 'primary' : 'secondary'" :disabled="!snapshot?.review && (!snapshot?.connected || !['Lobby','Matchmaking','ReadyCheck','ChampSelect','GameStart','InProgress'].includes(snapshot?.phase || ''))" @click="$emit('match')">{{ snapshot?.review ? '查看赛后分析' : snapshot?.stage === 'in-game' ? '显示局内快览' : '展开客户端侧栏' }} <AppIcon name="chevron" :size="14" /></button></section>
      </aside>
      <section class="home-panel home-recent">
        <header><h2>最近战绩</h2><button class="text-button" :disabled="!player || busy || loading" @click="player && $emit('open', player)">完整战绩 <AppIcon name="chevron" :size="13" /></button></header>
        <div class="home-history-filters"><select v-model="state.queue" aria-label="首页战绩模式" :disabled="!player || busy || state.pageBusy" @change="changeCriteria"><option value="all">全部模式</option><option v-for="mode in modes" :key="mode" :value="String(mode)">{{ queueName(mode) }}</option></select><select v-model.number="state.sampleSize" aria-label="首页分析场数" :disabled="!player || busy || state.pageBusy" @change="changeCriteria"><option v-for="size in SAMPLE_SIZES" :key="size" :value="size">最近 {{ size }} 场</option></select><span>{{ stats.games }} / {{ state.sampleSize ?? 20 }} 场</span></div>
        <div v-if="state.pageBusy || state.sourceLimit || state.pageError || state.browseNotice" class="home-browse-status" role="status"><span>{{ state.pageBusy ? '正在查找战绩…' : state.pageError || state.sourceLimit || state.browseNotice }}</span><button v-if="state.pageBusy" class="text-button" @click="state.stopRequested = true">停止</button><button v-else-if="needsSamples && canFetch" class="text-button" :disabled="busy" @click="fill()">继续查找</button></div>
        <div ref="historyScroll" class="home-match-list">
          <div v-if="!pageMatches.length" class="home-no-history"><AppIcon name="history" :size="34" /><h3>{{ loading || player?.historyState === 'loading' ? '战绩准备中' : '近期战绩' }}</h3><p>{{ historyMessage }}</p></div>
          <button v-for="match in pageMatches" :key="match.gameId" class="home-match" :class="{ won: match.win === true, lost: match.win === false }" :disabled="busy || loading" @click="selectedGame=match.gameId"><GameIcon :id="match.championId" :size="38" /><span class="home-match-context"><b>{{ queueName(match.queueId) }}</b><small>{{ date(match.playedAt) }}</small></span><span class="home-match-score"><b>{{ kda(match) }}</b><small>{{ championName(match.championId) }}</small></span><span class="home-match-result"><b>{{ outcome(match) }}</b><small>{{ duration(match.duration) }}</small></span><AppIcon name="chevron" :size="14" /></button>
        </div>
        <nav class="home-pagination" aria-label="首页战绩分页"><button class="secondary" :disabled="busy || state.pageBusy || (state.page ?? 1) <= 1" @click="goPage((state.page ?? 1) - 1)">上一页</button><span>第 {{ state.page ?? 1 }} 页<small>每页20场</small></span><button class="secondary" :disabled="busy || state.pageBusy || ((state.page ?? 1) >= knownPages && (!state.hasMore || !canFetch))" @click="goPage((state.page ?? 1) + 1)">下一页</button><form class="home-page-jump" @submit.prevent="jumpPage"><input v-model="jumpDraft" aria-label="首页跳转页码" :aria-invalid="!!jumpError" :title="`可输入 1–${maxJumpPage} 页`" inputmode="numeric" maxlength="3" :disabled="busy || state.pageBusy" /><button class="secondary" :disabled="busy || state.pageBusy">跳转</button></form></nav>
        <p v-if="jumpError" class="page-jump-error" role="alert">{{ jumpError }}</p>
      </section>
      <section class="home-panel home-mains"><header><h2>近期常用</h2><span>{{ state.sampleSize ?? 20 }} 场分析</span></header><div class="home-mains-list"><div v-for="hero in mains" :key="hero.id" class="home-champion"><GameIcon :id="hero.id" :size="38" /><div><strong :title="championName(hero.id)">{{ championName(hero.id) }}</strong><small>{{ hero.games }} 场 · {{ decimal(hero.kda) }} KDA</small></div><b>{{ percent(hero.winRate) }}</b></div><div v-if="!mains.length" class="home-no-champions"><AppIcon name="match" :size="30" /><p>战绩准备后显示近期常用英雄。</p></div></div><div class="home-sample-note"><span>{{ sampleLabel }}</span><HistoryNotice v-if="player" :player="noticePlayer" :stats="stats" :filtered="state.queue !== 'all'" /></div></section>
    </div>
    <PlayerDrawer v-if="selectedMatch && player" :player-key="`home-game-${selectedMatch.gameId}`" label="对局详情" modal fit :active="active ?? true" @close="selectedGame=null">
      <header class="home-game-heading"><div><h2>对局详情</h2><p>{{ queueName(selectedMatch.queueId) }} · {{ date(selectedMatch.playedAt) }} · {{ playerName(player) }}</p></div><button class="icon-button" aria-label="关闭对局详情" @click="selectedGame=null"><AppIcon name="close"/></button></header>
      <div class="home-game-content"><MatchHistoryRow :match="selectedMatch" expanded fixed><template #details><MatchDetails :player="player" :game-id="selectedMatch.gameId" :connected="snapshot?.connected"/></template></MatchHistoryRow></div>
    </PlayerDrawer>
  </section>
</template>
<style scoped>
.home-game-heading{display:flex;justify-content:space-between;align-items:flex-start;gap:16px;padding:20px;border-bottom:1px solid var(--border);flex:none;}
.home-game-heading h2{font-size:20px;margin:0 0 6px;}.home-game-heading p{font-size:12px;color:var(--muted);margin:0;}
.home-game-content{padding:16px;min-height:0;overflow:auto;}.home-game-content .match-card{margin:0;}
.home-game-content :deep(.match-items){max-width:none;flex-wrap:nowrap;}
.home-game-content :deep(.match-items .game-icon){flex:none;}
</style>
