<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { Events } from '@wailsio/runtime';
import * as Service from '../../bindings/leaguepingo/appservice';
import type { State } from '../../bindings/leaguepingo/internal/postgame/models';
import { errorMessage } from '../presentation';
defineProps<{ compact?: boolean }>();
const state=ref<State|null>(null), now=ref(Date.now()), error=ref(''),busy=ref(false);
let stop:(()=>void)|undefined, clock:ReturnType<typeof setInterval>|undefined,alive=true;
function accept(next:State|undefined){
 if(!alive || !next || (state.value && next.revision<state.value.revision))return;
 state.value=next;error.value='';
 if(clock)clearInterval(clock);
 if(['honor-countdown','return-countdown','queue-countdown'].includes(next.status)){now.value=Date.now();clock=setInterval(()=>now.value=Date.now(),100);}
}
const text=computed(()=>{
 const s=state.value;if(!s)return '';
 switch(s.status){
 case 'honor-countdown':return `${Math.max(0,(Date.parse(s.dueAt)-now.value)/1000).toFixed(0)} 秒后跳过点赞`;
 case 'waiting-honor':return '等待点赞界面就绪或手动点赞完成';
 case 'skipping-honor':return '正在跳过点赞并确认结果…';
 case 'honor-skipped':return '已跳过点赞';
 case 'honor-not-needed':return '本局无需跳过点赞或已手动完成';
 case 'honor-unavailable':return '无法自动跳过点赞，请在客户端手动处理';
 case 'honor-mismatch':return '点赞页与本局不匹配，本次自动流程已停止';
 case 'honor-unconfirmed':return '未确认点赞页已结束，请在客户端检查';
 case 'return-countdown':return `${Math.max(0,(Date.parse(s.dueAt)-now.value)/1000).toFixed(0)} 秒后返回房间`;
 case 'queue-countdown':return `${Math.max(0,(Date.parse(s.dueAt)-now.value)/1000).toFixed(0)} 秒后开始排队`;
 case 'waiting-result':return '等待赛后结算与复盘保留';
 case 'waiting-return':return '等待返回房间后自动排队';
 case 'waiting-ready':return '等待队友返回或房间就绪';
 case 'waiting-postgame':return '等待赛后页面完成';
 case 'continuing-postgame':return '正在完成赛后继续步骤…';
 case 'continue-unavailable':return '暂时无法继续赛后页面，请在客户端完成本次流程';
 case 'continue-unconfirmed':return '未确认赛后继续步骤已完成，请在客户端检查';
 case 'returning':return '正在返回房间…';
 case 'queueing':return '正在确认排队状态…';
 case 'returned':return '已返回房间';
 case 'queued':return '已开始排队';
 case 'not-leader':return '已返回房间，等待队长开始';
 case 'changed':return '房间或模式已变化，本次自动排队已停止';
 case 'cancelled':return '本次赛后自动操作已取消';
 case 'timeout':return '等待房间超时，请手动操作';
 case 'failed':return '未确认赛后操作结果，请在客户端检查';
 default:return !s.returnEnabled && !s.queueEnabled && s.skipHonor ? '自动跳过点赞已开启' : s.returnEnabled && s.queueEnabled ? '赛后自动返回与排队已开启' : s.returnEnabled ? '赛后自动返回房间已开启' : '赛后自动排队已开启';
 }

});
async function cancel(){busy.value=true;try{accept(await Service.CancelPostGame());}catch(reason){error.value=errorMessage(reason);}finally{busy.value=false;}}
onMounted(async()=>{stop=Events.On('leaguepingo:post-game',event=>accept(event.data as State));try{accept(await Service.PostGameStatus());}catch{/* Connection status remains with the main client indicator. */}});
onUnmounted(()=>{alive=false;stop?.();if(clock)clearInterval(clock);});
</script>
<template>
 <div v-if="state && (state.returnEnabled || state.queueEnabled || state.skipHonor)" class="post-game-notice" :class="{compact}" :data-status="state.status">
  <span role="status">{{ error || text }}</span>
  <button v-if="['waiting-postgame','continuing-postgame','waiting-honor','honor-countdown','waiting-result','waiting-return','waiting-ready','return-countdown','queue-countdown'].includes(state.status)" class="secondary" :disabled="busy" @click="cancel">取消本次</button>
 </div>
</template>
<style scoped>
.post-game-notice{display:flex;align-items:center;justify-content:space-between;gap:8px;min-height:32px;margin:0 0 12px;padding:6px 10px;border:1px solid var(--border);border-radius:6px;color:var(--muted);font-size:12px;font-variant-numeric:tabular-nums;}
.post-game-notice[data-status="honor-countdown"],.post-game-notice[data-status="queue-countdown"],.post-game-notice[data-status="return-countdown"]{color:var(--text);}
.post-game-notice button{flex:none;min-height:28px;padding:3px 8px;font-size:12px;}
.post-game-notice.workspace-message{margin:8px 20px 0;flex:none;}
.compact{margin:0 8px 8px;flex-wrap:wrap;}
</style>
