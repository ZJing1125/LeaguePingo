<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import * as Service from '../../bindings/leaguepingo/appservice';
import type { MatchReview,Participant,Snapshot } from '../../bindings/leaguepingo/internal/core/models';
import type { PlayerViewState } from '../presentation';
import {date,errorMessage,queueName} from '../presentation';
import MatchRoster from './MatchRoster.vue';
import MatchDetails from './MatchDetails.vue';
import QueueSwitch from './QueueSwitch.vue';
import PlayerDrawer from './PlayerDrawer.vue';
import PlayerView from './PlayerView.vue';
import AppIcon from './AppIcon.vue';
const props=defineProps<{review:MatchReview|null;connected:boolean;active:boolean;inGame:boolean;companionAvailable:boolean}>();
const emit=defineEmits<{query:[player:Participant];snapshot:[value:Snapshot];modal:[open:boolean];companion:[]}>();
const queue=ref(420), selectedKey=ref(''), busy=ref(false), error=ref('');
const scope=ref('result');
const states=reactive<Record<string,PlayerViewState>>({});
const selected=computed(()=>props.review?.participants?.find(p=>p.key===selectedKey.value) ?? null);
const target=computed(()=>props.review?.participants?.find(p=>p.key===props.review?.selfKey || p.puuid===props.review?.selfKey) ?? props.review?.participants?.[0]);
watch(()=>props.review?.gameId,()=>{queue.value=props.review?.scoutQueue || 420;selectedKey.value='';error.value='';for(const k of Object.keys(states))delete states[k];},{immediate:true});
watch(()=>props.active,active=>{if(!active)selectedKey.value='';});
watch(()=>!!selected.value,open=>emit('modal',open));
function stateFor(key:string){return states[key] ?? (states[key]={queue:'all',expanded:[]});}
function adjacent(direction:number){const ps=props.review?.participants ?? [];const i=ps.findIndex(p=>p.key===selectedKey.value);if(ps.length)selectedKey.value=ps[(i+direction+ps.length)%ps.length]!.key;}
async function refresh(){if(busy.value || !props.review)return;busy.value=true;error.value='';try{emit('snapshot',await Service.RefreshReview(props.review.gameId));}catch(reason){error.value=errorMessage(reason);}finally{busy.value=false;}}
</script>
<template>
  <section class="review-room" aria-label="赛后分析">
    <header class="review-heading"><div><h1>赛后分析</h1><p v-if="review">{{ queueName(review.queueId) }} · 对局 #{{ review.gameId }} · {{ date(review.capturedAt) }}</p><p v-else>结束一局后，在这里查看双方表现与赛前资料。</p></div><button class="secondary" :disabled="!companionAvailable" @click="$emit('companion')">{{ inGame?'显示局内快览':'展开客户端侧栏' }}</button></header>
    <div v-if="!review" class="review-empty"><AppIcon name="history" :size="36"/><h2>尚无赛后记录</h2><p>赛前资料会显示在客户端左侧，局内按住 ~ 查看双方。</p><p class="review-note">本次运行保留最近一局，结束后自动整理。</p></div>
    <template v-else>
      <div class="review-section-title"><div class="review-scope-switch" role="group" aria-label="复盘内容"><button class="secondary" :aria-pressed="scope==='result'" @click="scope='result'">本局结果</button><button class="secondary" :aria-pressed="scope==='pregame'" @click="scope='pregame'">赛前资料</button></div><p class="review-note">仅本次运行保留 · 新房间不会覆盖</p></div>
      <section v-show="scope==='result'" class="review-result" aria-label="本局结算">
        <div class="review-status"><span>{{ review.state==='waiting'?'正在同步本局结算…':review.state==='ready'?'本局结果已同步':review.state==='partial'?'结算资料部分返回':'结算尚未同步，可保留赛前资料稍后重试' }}</span><button class="secondary" :disabled="busy || !connected || inGame || review.state==='waiting'" @click="refresh">{{ busy?'同步中…':'同步结算' }}</button></div>
        <p v-if="error" class="quality-warning" role="status">{{ error }}</p>
        <MatchDetails v-if="review.result && target" :player="target" :game-id="review.gameId" :provided="review.result" :connected="false"/>
        <p v-else class="review-note">本局胜负、KDA 和装备以结算结果为准，赛前统计可在“赛前资料”中查看。</p>
      </section>
      <template v-if="scope==='pregame'">
        <div class="review-section-title"><h2>赛前资料快照</h2><QueueSwitch :value="queue" @change="queue=$event"/></div>
        <p class="review-note">各玩家统计时间以卡片记录为准；这里的历史表现不代表本局结果。点击玩家查看已保留战绩。</p>
        <MatchRoster :players="review.participants ?? []" :selected-key="selectedKey" :queue="queue" frozen captured @select="selectedKey=$event.key"/>
      </template>
    </template>
    <PlayerDrawer v-if="selected" :player-key="selected.key" modal :active="active" @close="selectedKey=''">
      <div class="drawer-toolbar"><button class="text-button" :disabled="!connected || !selected.displayName.includes('#')" @click="$emit('query',selected)">查询最新战绩 ↗</button><span class="drawer-player-switch"><button class="text-button" aria-label="上一位玩家" @click="adjacent(-1)">← 上一位</button><button class="text-button" aria-label="下一位玩家" @click="adjacent(1)">下一位 →</button></span><button class="icon-button" aria-label="关闭玩家详情" @click="selectedKey=''"><AppIcon name="close"/></button></div>
      <p class="review-note">赛前已保留战绩 · 查询最新战绩将在玩家战绩页打开</p>
      <PlayerView :player="selected" :state="stateFor(selected.key)" :scout-queue="queue" :connected="false" :active="active" compact captured/>
    </PlayerDrawer>
  </section>
</template>
