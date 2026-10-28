<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { Events } from '@wailsio/runtime';
import * as Service from '../../bindings/leaguepingo/appservice';
import type { State } from '../../bindings/leaguepingo/internal/readycheck/models';
import { errorMessage } from '../presentation';
defineProps<{ compact?: boolean }>();
const state=ref<State|null>(null), now=ref(Date.now()), error=ref(''),busy=ref(false);
let stop:(()=>void)|undefined, clock:ReturnType<typeof setInterval>|undefined,alive=true;
function accept(next:State|undefined){
 if(!alive || !next || (state.value && next.revision<state.value.revision))return;
 state.value=next;error.value='';
 if(clock)clearInterval(clock);
 if(next.status==='countdown'){now.value=Date.now();clock=setInterval(()=>now.value=Date.now(),100);}
}
const text=computed(()=>{
 const s=state.value;if(!s)return '';
 switch(s.status){
 case 'countdown':return `${Math.max(0,(Date.parse(s.dueAt)-now.value)/1000).toFixed(1)} 秒后自动接受`;
 case 'accepting':return '正在确认接受结果…';
 case 'accepted':return '已接受对局';
 case 'skipped':return '本次已跳过，请手动接受';
 case 'manual':return '本次已手动响应';
 case 'failed':return '未确认接受结果，请在客户端检查';
 case 'waiting':return '自动接受已开启 · 等待客户端';
 case 'unsupported':return '当前客户端暂不支持自动接受';
 default:return '自动接受已开启';
 }
});
async function skip(){busy.value=true;try{accept(await Service.SkipAutoAccept());}catch(reason){error.value=errorMessage(reason);}finally{busy.value=false;}}
onMounted(async()=>{stop=Events.On('leaguepingo:auto-accept',event=>accept(event.data as State));try{accept(await Service.AutoAcceptStatus());}catch{/* Connection status remains with the main client indicator. */}});
onUnmounted(()=>{alive=false;stop?.();if(clock)clearInterval(clock);});
</script>
<template>
 <div v-if="state?.enabled" class="auto-accept-notice" :class="{compact}" :data-status="state.status">
  <span role="status">{{ error || text }}</span>
  <button v-if="state.status==='countdown'" class="secondary" :disabled="busy" @click="skip">跳过本次</button>
 </div>
</template>
<style scoped>
.auto-accept-notice{display:flex;align-items:center;justify-content:space-between;gap:8px;min-height:32px;margin:0 0 12px;padding:6px 10px;border:1px solid var(--border);border-radius:6px;color:var(--muted);font-size:12px;font-variant-numeric:tabular-nums;}
.auto-accept-notice[data-status="countdown"],.auto-accept-notice[data-status="accepting"]{color:var(--text);}
.auto-accept-notice button{flex:none;min-height:28px;padding:3px 8px;font-size:12px;}
.auto-accept-notice.workspace-message{margin:8px 20px 0;flex:none;}
.compact{margin:0 8px 8px;flex-wrap:wrap;}
</style>
