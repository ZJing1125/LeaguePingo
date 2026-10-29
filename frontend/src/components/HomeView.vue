<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import type { Participant, Snapshot } from "../../bindings/leaguepingo/internal/core/models";
import { date, playerName, queueName, sampleStats, type PlayerViewState } from "../presentation";
import { SAMPLE_SIZES, useHistoryBrowser } from "../historyBrowser";
import AppIcon from "./AppIcon.vue";
import ChampionSplash from "./ChampionSplash.vue";
import ParticipantAvatar from "./ParticipantAvatar.vue";
import RankEmblem from "./RankEmblem.vue";
import OverviewPerformance from './OverviewPerformance.vue';
import RankPanel from './RankPanel.vue';
import ChampionAnalysis from './ChampionAnalysis.vue';
import PlaystylePanel from './PlaystylePanel.vue';
import MasteryPanel from './MasteryPanel.vue';

import RecentForm from "./RecentForm.vue";
import HistoryNotice from "./HistoryNotice.vue";
import PlayerDrawer from './PlayerDrawer.vue';
import MatchHistoryRow from './MatchHistoryRow.vue';
import MatchDetails from './MatchDetails.vue';
import { hasCurrentMatch } from '../matchRoom';
import HistoryServiceStatus from './HistoryServiceStatus.vue';

const props = defineProps<{ player: Participant | null; snapshot: Snapshot | null; status: string; loading: boolean; busy: boolean; error: string; active?: boolean }>();
const emit=defineEmits<{ open: [player: Participant]; match: []; refresh: []; modal:[open:boolean];recovered:[player:Participant] }>();
const selectedGame=ref<number|null>(null);
watch(()=>`${props.active}:${props.player?.connectionId}:${props.player?.key}`,()=>{selectedGame.value=null;});
const state = ref<PlayerViewState>({ queue: "all", expanded: [], sampleSize: 20 });
const historyScroll = ref<HTMLElement | null>(null);
const historyTop = ref<HTMLElement | null>(null);
const placeholder = { key: "home-empty", puuid: "", connectionId: 0, identityState: "unavailable", historyState: "pending", history: [], historyMeta: {} } as unknown as Participant;
watch(() => `${props.player?.connectionId}:${props.player?.key}`, () => { state.value.stopRequested = true; state.value = { queue: "all", expanded: [], sampleSize: 20 }; });
const browseProps = reactive({ get player() { return props.player ?? placeholder; }, get state() { return state.value; }, get connected() { return props.snapshot?.connected; }, get busy() { return props.busy || props.loading; }, get active() { return props.active; } });
const { allMatches, analysisMatches: matches, pageMatches, knownPages, canFetch, sampleLabel, needsSamples, noticePlayer, changeCriteria, goPage, fill, jumpDraft, jumpError, jumpPage, maxJumpPage } = useHistoryBrowser(browseProps, historyScroll, () => historyTop.value?.scrollIntoView({ block: 'start' }));
const selectedMatch=computed(()=>allMatches.value.find(m=>m.gameId===selectedGame.value));
watch(selectedMatch,match=>{if(!match)selectedGame.value=null;emit('modal',!!match);});
const stats = computed(() => sampleStats(matches.value));
const modes = computed(() => [...new Set([420, 440, 450, 2400, ...allMatches.value.map(m => m.queueId)])]);
const mains = computed(() => [...new Set(matches.value.map(m => m.championId).filter(Boolean))].map(id => ({ id, ...sampleStats(matches.value.filter(m => m.championId === id)) })).sort((a, b) => b.games - a.games).slice(0, 5));
const historyMessage = computed(() => !props.snapshot?.connected ? "连接游戏客户端后，查看自己的近期表现。" : props.loading || props.player?.historyState === "loading" ? "正在准备你的近期战绩…" : props.player?.historyState === "failed" ? "战绩暂时不可用，可刷新重试。" : state.value.queue !== "all" ? "当前筛选暂无记录，可切换模式或继续查找。" : "暂时没有可展示的战绩。");
</script>
<template>
  <section class="home-workspace" aria-label="个人概览">
    <header class="home-heading"><div><h1>概览</h1></div><button class="secondary" :disabled="busy || loading || !snapshot?.connected || !snapshot?.selfKey" @click="$emit('refresh')"><AppIcon name="refresh" :size="15" />{{ loading ? "准备中…" : "刷新数据" }}</button></header>
    <div class="home-hero">
      <div v-if="mains.length" class="home-hero-art" aria-hidden="true"><ChampionSplash :id="mains[0]?.id" /></div>
      <div class="home-account"><ParticipantAvatar v-if="player" :player="player" account :size="64" /><span v-else class="home-avatar-empty"><AppIcon name="user" :size="30" /></span><div><span class="home-account-caption">{{ snapshot?.connected ? "当前账号" : "等待客户端" }}</span><h2>{{ player ? playerName(player) : snapshot?.connected ? loading ? "正在识别玩家" : "账号资料待获取" : "启动并登录英雄联盟" }}</h2><p><span v-if="player?.profile?.level != null">等级 {{ player.profile.level }} · </span>{{ status.split(" · ")[0] }}<span v-if="player?.stale || !snapshot?.connected && player"> · 旧资料</span></p><span v-if="player?.privacyState === 'private'" class="home-privacy">生涯不公开</span></div></div>
      <div class="home-form-summary"><div class="home-form-last"><span>最近表现</span><RecentForm v-if="matches.length" :matches="matches"/><span v-else>等待战绩</span><small>{{sampleLabel}}</small></div></div>
    </div>
    <p v-if="error" class="error-banner home-error" role="status">{{ error }}</p>
    <div class="home-grid">
      <aside class="home-ranks">
        <template v-if="player"><RankPanel :player="player"/><RankPanel :player="player" queue="RANKED_FLEX_SR"/></template>
        <section v-if="!player" class="home-panel home-rank"><span>我的段位</span><RankEmblem /><strong>等待账号资料</strong><small>连接客户端后显示</small></section>
        <section class="home-panel home-session"><span class="home-panel-caption">当前状态</span><strong><i class="connection-light" :class="{ online: snapshot?.connected }" />{{ status.split(" · ")[0] }}</strong><p>{{ snapshot?.stage === 'in-game' ? "游戏中按住 ~ 查看本局资料，松开隐藏。" : "进入房间或选人后，客户端左侧展示我方资料。" }}</p><button v-if="hasCurrentMatch(snapshot) || snapshot?.review" :class="hasCurrentMatch(snapshot) ? 'primary' : 'secondary'" @click="$emit('match')">{{ hasCurrentMatch(snapshot) ? '查看当前对局' : '查看赛后分析' }} <AppIcon name="chevron" :size="14" /></button></section>
        <PlaystylePanel :matches="matches" :scope="sampleLabel"/>
        <ChampionAnalysis :matches="matches" :scope="sampleLabel" :fetched-at="player?.historyMeta?.fetchedAt"/>
        <MasteryPanel v-if="player" :player="player"/>
      </aside>
      <div class="home-analysis-main"><OverviewPerformance :matches="matches" :scope="sampleLabel"/>
      <section ref="historyTop" class="home-panel home-recent">
        <header><h2>最近战绩</h2><HistoryServiceStatus v-if="player" inline readonly :player="player"/><button class="text-button" :disabled="!player || busy || loading" @click="player && $emit('open', player)">完整战绩 <AppIcon name="chevron" :size="13" /></button></header>
        <HistoryServiceStatus v-if="player" :player="player" :connected="snapshot?.connected" :busy="busy || loading || state.pageBusy || state.warmBusy" :automatic="active!==false && selectedGame===null && snapshot?.stage==='idle' && (state.page??1)===1" @recovered="emit('recovered',$event)" @retry="emit('refresh')"/>
        <div class="home-history-filters"><select v-model="state.queue" aria-label="首页战绩模式" :disabled="!player || busy" @change="changeCriteria"><option value="all">全部模式</option><option v-for="mode in modes" :key="mode" :value="String(mode)">{{ queueName(mode) }}</option></select><select v-model.number="state.sampleSize" aria-label="首页分析场数" :disabled="!player || busy" @change="changeCriteria"><option v-for="size in SAMPLE_SIZES" :key="size" :value="size">最近 {{ size }} 场</option></select><span>{{ stats.games }} / {{ state.sampleSize ?? 20 }} 场</span></div>
        <div v-if="state.pageBusy || state.warmBusy || state.sourceLimit || state.pageError || state.browseNotice" class="home-browse-status" role="status"><span>{{ state.pageBusy ? '正在读取更多战绩…' : state.warmBusy ? '正在后台准备单双排与灵活数据，当前结果可继续使用。' : state.pageError || state.sourceLimit || state.browseNotice }}</span><button v-if="state.pageBusy" class="text-button" @click="state.stopRequested = true">停止</button><button v-else-if="!state.warmBusy && needsSamples && canFetch" class="text-button" :disabled="busy" @click="fill()">继续查找</button></div>
        <div ref="historyScroll" class="home-match-list">
          <div v-if="!pageMatches.length" class="home-no-history"><AppIcon name="history" :size="34" /><h3>{{ loading || player?.historyState === 'loading' ? '战绩准备中' : '近期战绩' }}</h3><p>{{ historyMessage }}</p></div>
          <MatchHistoryRow v-for="match in pageMatches" :key="match.gameId" class="home-match" :match="match" :expanded="false" @toggle="!busy && !loading && (selectedGame=match.gameId)"/>
        </div>
        <nav class="home-pagination" aria-label="首页战绩分页"><button class="secondary" :disabled="busy || state.pageBusy || (state.page ?? 1) <= 1" @click="goPage((state.page ?? 1) - 1)">上一页</button><span>第 {{ state.page ?? 1 }} 页<small>每页20场</small></span><button class="secondary" :disabled="busy || state.pageBusy || ((state.page ?? 1) >= knownPages && (!state.hasMore || !canFetch))" @click="goPage((state.page ?? 1) + 1)">下一页</button><form class="home-page-jump" @submit.prevent="jumpPage"><input v-model="jumpDraft" aria-label="首页跳转页码" :aria-invalid="!!jumpError" :title="`可输入 1–${maxJumpPage} 页`" inputmode="numeric" maxlength="3" :disabled="busy || state.pageBusy" /><button class="secondary" :disabled="busy || state.pageBusy">跳转</button></form></nav>
        <p v-if="jumpError" class="page-jump-error" role="alert">{{ jumpError }}</p>
      </section>
      <div class="home-sample-note"><HistoryNotice v-if="player" :player="noticePlayer" :stats="stats" :filtered="state.queue !== 'all'"/></div>
      </div>
    </div>
    <PlayerDrawer v-if="selectedMatch && player" :player-key="`home-game-${selectedMatch.gameId}`" label="对局详情" modal fit :active="active ?? true" @close="selectedGame=null">
      <header class="home-game-heading"><div><h2>对局详情</h2><p>{{ queueName(selectedMatch.queueId) }} · {{ date(selectedMatch.playedAt) }} · {{ playerName(player) }}</p></div><button class="icon-button" aria-label="关闭对局详情" @click="selectedGame=null"><AppIcon name="close"/></button></header>
      <div class="home-game-content"><MatchHistoryRow :match="selectedMatch" expanded fixed><template #details><MatchDetails :player="player" :game-id="selectedMatch.gameId" :match="selectedMatch" :connected="snapshot?.connected"/></template></MatchHistoryRow></div>
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
