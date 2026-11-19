<script setup lang="ts">
import { uiText } from "../uiText";
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
import { roomHistoryTarget } from "../playerHistoryNavigation";
import PlayerHistoryAction from "./PlayerHistoryAction.vue";
import AppIcon from './AppIcon.vue';
const props=defineProps<{review:MatchReview|null;connected:boolean;active:boolean;inGame:boolean}>();
const emit=defineEmits<{snapshot:[value:Snapshot];modal:[open:boolean]}>();
const queue=ref(0), selectedKey=ref(''), busy=ref(false), error=ref('');
const scope=ref('result');
const states=reactive<Record<string,PlayerViewState>>({});
const selected=computed(()=>props.review?.participants?.find(p=>p.key===selectedKey.value) ?? null);
const target=computed(()=>props.review?.participants?.find(p=>p.key===props.review?.selfKey || p.puuid===props.review?.selfKey) ?? props.review?.participants?.[0]);
watch(()=>`${props.review?.connectionId}:${props.review?.selfKey}:${props.review?.gameId}`,()=>{queue.value=props.review?.scoutQueue ?? props.review?.queueId ?? 0;scope.value='result';selectedKey.value='';error.value='';for(const k of Object.keys(states))delete states[k];},{immediate:true});
watch(()=>props.active,active=>{if(!active)selectedKey.value='';});
watch(()=>!!selected.value,open=>emit('modal',open));
function stateFor(key:string){return states[key] ?? (states[key]={queue:'all',expanded:[]});}
function adjacent(direction:number){const ps=props.review?.participants ?? [];const i=ps.findIndex(p=>p.key===selectedKey.value);if(ps.length)selectedKey.value=ps[(i+direction+ps.length)%ps.length]!.key;}
async function refresh(){if(busy.value || !props.review)return;busy.value=true;error.value='';try{emit('snapshot',await Service.RefreshReview(props.review.gameId));}catch(reason){error.value=errorMessage(reason);}finally{busy.value=false;}}
</script>
<template>
  <section class="review-room" :aria-label="uiText.common.review">
    <header class="review-heading"><div><h1>{{ uiText.common.review }}</h1><p v-if="review">{{ uiText.reviewRoom.context(queueName(review.queueId), review.gameId, date(review.capturedAt)) }}</p><p v-else>{{ uiText.reviewRoom.emptyHelp }}</p></div></header>
    <div v-if="!review" class="review-empty"><AppIcon name="history" :size="36"/><h2>{{ uiText.reviewRoom.empty }}</h2><p>{{ uiText.reviewRoom.liveHelp }}</p><p class="review-note">{{ uiText.reviewRoom.retention }}</p></div>
    <template v-else>
      <div class="review-section-title"><div class="review-scope-switch" role="group" :aria-label="uiText.reviewRoom.content"><button class="secondary" :aria-pressed="scope==='result'" @click="scope='result'">{{ uiText.reviewRoom.result }}</button><button class="secondary" :aria-pressed="scope==='pregame'" @click="scope='pregame'">{{ uiText.reviewRoom.pregame }}</button></div><p class="review-note">{{ uiText.reviewRoom.retentionHint }}</p></div>
      <section v-show="scope==='result'" class="review-result" :aria-label="uiText.reviewRoom.settlement">
        <div class="review-status"><span>{{ review.state==='waiting'?uiText.reviewRoom.syncing:review.state==='ready'?uiText.reviewRoom.synced:review.state==='partial'?uiText.reviewRoom.partial:uiText.reviewRoom.notSynced }}</span><button class="secondary" :disabled="busy || !connected || inGame || review.state==='waiting'" @click="refresh">{{ busy?uiText.common.syncingShort:uiText.reviewRoom.sync }}</button></div>
        <p v-if="error" class="quality-warning" role="status">{{ error }}</p>
        <MatchDetails v-if="review.result && target" :player="target" :game-id="review.gameId" :provided="review.result" :connected="false"/>
        <p v-else class="review-note">{{ uiText.reviewRoom.settlementHelp }}</p>
      </section>
      <template v-if="scope==='pregame'">
        <div class="review-section-title"><h2>{{ uiText.reviewRoom.pregame }}</h2><QueueSwitch :current="review.queueId" :value="queue" @change="queue=$event || review.queueId"/></div>
        <p class="review-note">{{ uiText.reviewRoom.capturedHelp }}</p>
        <MatchRoster :players="review.participants ?? []" :selected-key="selectedKey" :queue="queue" frozen captured @select="selectedKey=$event.key"/>
      </template>
    </template>
    <PlayerDrawer v-if="selected" :player-key="selected.key" modal :active="active" @close="selectedKey=''">
      <div class="drawer-toolbar"><PlayerHistoryAction class="text-button" :target="roomHistoryTarget(selected, true)" show-disabled>{{ uiText.matchRoom.latestHistory }}</PlayerHistoryAction><span class="drawer-player-switch"><button class="text-button" :aria-label="uiText.matchRoom.previousPlayer" @click="adjacent(-1)">{{ uiText.matchRoom.previous }}</button><button class="text-button" :aria-label="uiText.matchRoom.nextPlayer" @click="adjacent(1)">{{ uiText.matchRoom.next }}</button></span><button class="icon-button" :aria-label="uiText.matchRoom.closePlayer" @click="selectedKey=''"><AppIcon name="close"/></button></div>
      <p class="review-note">{{ uiText.reviewRoom.playerHint }}</p>
      <PlayerView :player="selected" :state="stateFor(selected.key)" :scout-queue="queue" :connected="false" :active="active" compact captured/>
    </PlayerDrawer>
  </section>
</template>
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.review-room):is(.review-room) {height:100%;overflow:auto;padding:22px 24px;display:flex;flex-direction:column;gap:14px;min-height:0;}
:where(.review-result):is(.review-result) {background:var(--panel);border:1px solid var(--border);border-radius:8px;padding:12px 16px;}
:where(.review-status):is(.review-status) {display:flex;align-items:center;justify-content:space-between;gap:12px;font-size:12px;color:var(--muted);}
@media(max-width:1000px){:where(.review-room):is(.review-room){padding:16px;}}
:where(.review-room):is(.match-room>.review-room) {flex:1;height:auto;min-height:0;}
</style>
