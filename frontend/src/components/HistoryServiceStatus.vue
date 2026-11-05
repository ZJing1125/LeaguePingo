<script setup lang="ts">
import { uiText } from "../uiText";
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import type { Participant } from '../../bindings/leaguepingo/internal/core/models';
import * as Service from '../../bindings/leaguepingo/appservice';
import { errorMessage } from '../errors';
import ExperimentalBadge from './ExperimentalBadge.vue';

const props=defineProps<{player:Participant;connected?:boolean;busy?:boolean;automatic?:boolean;readonly?:boolean;inline?:boolean}>();
const emit=defineEmits<{recovered:[player:Participant];retry:[]}>();
const now=ref(Date.now()),running=ref(false),notice=ref(''),exhausted=ref(false);
const visible=ref(!document.hidden);
function visibility(){visible.value=!document.hidden;}
document.addEventListener('visibilitychange',visibility);
let timer:ReturnType<typeof setTimeout>|undefined,attempts=0,deadline=0,revision=0,alive=true;
let request:ReturnType<typeof Service.RetryHistory>|undefined;
const clock=setInterval(()=>{now.value=Date.now();},1000);
const scope=computed(()=>`${props.player.connectionId}:${props.player.key}`);
const state=computed(()=>props.player.historyMeta?.service);
const due=computed(()=>Date.parse(state.value?.retryAt ?? '')||0);
const seconds=computed(()=>Math.max(0,Math.ceil((due.value-now.value)/1000)));
const age=computed(()=>{
 const at=Date.parse(props.player.historyMeta?.fetchedAt ?? '');
 if(!at)return uiText.historyServiceStatus.unknownTime;
 const s=Math.max(0,Math.floor((now.value-at)/1000));
 return s<60?uiText.historyServiceStatus.secondsAgo(s):s<3600?uiText.historyServiceStatus.minutesAgo(Math.floor(s/60)):uiText.historyServiceStatus.hoursAgo(Math.floor(s/3600));
});
const canAuto=computed(()=>visible.value && !!state.value && !!due.value && props.automatic && props.connected && !props.busy && !props.readonly && !exhausted.value);
const scheduled=ref(false);
function stop(){revision++;if(timer)clearTimeout(timer);timer=undefined;scheduled.value=false;request?.cancel();request=undefined;}
watch(scope,()=>{stop();attempts=0;deadline=0;exhausted.value=false;notice.value='';});
function arm(){
 stop();
 if(!state.value){attempts=0;deadline=0;exhausted.value=false;notice.value='';return;}
 if(!canAuto.value || running.value)return;
 if(!deadline)deadline=Date.now()+60_000;
 const delay=Math.max(500,due.value-Date.now());
 if(attempts>=2 || Date.now()+delay+6000>deadline){exhausted.value=true;return;}
 const token=revision,target=scope.value;
 scheduled.value=true;
 timer=setTimeout(async()=>{
  scheduled.value=false;
  if(!alive || token!==revision || target!==scope.value || !canAuto.value)return;
  running.value=true;attempts++;
  try{
   request=Service.RetryHistory(props.player.key,props.player.connectionId);
   const next=await request;
   if(alive && token===revision && target===scope.value){notice.value='';emit('recovered',next);}
  }catch(reason){if(alive && token===revision){notice.value=errorMessage(reason);exhausted.value=true;}}
  finally{if(token!==revision && target===scope.value)attempts=Math.max(0,attempts-1);request=undefined;running.value=false;if(alive)arm();}
 },delay);
}
watch(()=>[scope.value,props.player.historyMeta?.checkedAt,state.value?.code,due.value,canAuto.value],arm,{immediate:true,flush:'post'});
onBeforeUnmount(()=>{alive=false;stop();clearInterval(clock);document.removeEventListener('visibilitychange',visibility);});
</script>
<template>
 <div v-if="inline || state" class="history-service-status" :class="{degraded:state&&!inline,inline}" role="status">
  <ExperimentalBadge v-if="(inline && player.cached) || (!inline && state)" :feature="uiText.historyServiceStatus.label"/>
  <span>{{age}}</span>
  <template v-if="state && !inline">
   <span>{{player.history?.length?uiText.historyServiceStatus.retained:uiText.historyServiceStatus.unavailable}} {{running?uiText.historyServiceStatus.retrying:scheduled?uiText.historyServiceStatus.retryAfter(seconds):seconds?uiText.historyServiceStatus.waitBeforeRetry(seconds):exhausted?uiText.historyServiceStatus.paused:uiText.historyServiceStatus.manualRetry}}</span>
   <button v-if="!readonly" class="text-button" :disabled="busy || running || !connected || seconds>0" @click="emit('retry')">{{ uiText.common.retryHistory }}</button>
   <span v-if="notice">{{notice}}</span>
  </template>
 </div>
</template>
<style scoped>
.history-service-status{display:flex;flex:none;align-items:center;flex-wrap:wrap;gap:6px 12px;padding:8px 12px;font-size:11px;line-height:1.6;color:var(--muted);border-bottom:1px solid var(--border);}
.history-service-status.degraded{color:var(--warning-text);}.history-service-status button{margin-left:auto;flex:none;}
.history-service-status.inline{display:inline-flex;padding:0;border:0;}
</style>
