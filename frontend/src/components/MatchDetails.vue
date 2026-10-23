<script setup lang="ts">
import {computed,getCurrentInstance,nextTick,onBeforeUnmount,ref,watch} from 'vue';
import * as Service from '../../bindings/leaguepingo/appservice';
import type {Match,MatchDetail,MatchTimeline as Timeline,Participant} from '../../bindings/leaguepingo/internal/core/models';
import {errorMessage,kda,number,percent,decimal,sampleStats,positionName} from '../presentation';
import {championName} from '../gameAssets';
import {targetPlayer} from '../analysisData';
import {analysisCache} from '../analysisCache';
import GameIcon from './GameIcon.vue';
import MatchSummary from './MatchSummary.vue';
import TeamAnalysis from './TeamAnalysis.vue';
import MatchTimeline from './MatchTimeline.vue';
import MatchBuild from './MatchBuild.vue';
const props=defineProps<{player:Participant;gameId:number;match?:Match;connected?:boolean;provided?:MatchDetail|null}>();
const loaded=ref<MatchDetail|null>(null),timeline=ref<Timeline|null>(null),busy=ref(false),error=ref('');
const tab=ref('overview');
const scopeId=`analysis-${getCurrentInstance()!.uid}`;
const root=ref<HTMLElement|null>(null);
const tabs=[{key:'overview',label:'概览'},{key:'team',label:'团队分析'},{key:'timeline',label:'时间线'},{key:'build',label:'出装符文'}];
const key=computed(()=>`${props.player.connectionId}:${props.player.key}:${props.gameId}`);
let revision=0;
const detail=computed(()=>props.provided??loaded.value);
const match=computed(()=>targetPlayer(detail.value)?.summary??props.match??props.player.history?.find(m=>m.gameId===props.gameId));
const target=computed(()=>targetPlayer(detail.value));
const canLoad=computed(()=>!!props.connected&&!props.player.stale&&!props.provided);
const teams=computed(()=>[...new Set((detail.value?.players??[]).map(p=>p.teamId))].sort());
const teamName=(id:number)=>id===100?'蓝方':id===200?'红方':`队伍 ${id}`;
function teamResult(id:number){const values=(detail.value?.players??[]).filter(p=>p.teamId===id).map(p=>p.summary.win).filter(v=>v!=null);return values.length&&values.every(v=>v===values[0])?(values[0]?'胜利':'失败'):'';}
function teamKills(id:number){const values=(detail.value?.players??[]).filter(p=>p.teamId===id).map(p=>p.summary.kills);return values.length&&values.every(v=>v!=null)?values.reduce<number>((a,v)=>a+(v??0),0):null;}
function damageWidth(damage:number|null|undefined){const max=Math.max(0,...(detail.value?.players??[]).map(p=>p.summary.damage??0));return damage!=null&&max>0?Math.max(0,Math.min(100,damage*100/max)):0;}
function csRate(value:number|null|undefined,duration:number|null|undefined){return value!=null&&duration&&duration>0?`${decimal(value*60/duration)} / 分`:'—';}
watch(key,()=>{revision++;loaded.value=analysisCache.detail(key.value);timeline.value=props.provided?null:analysisCache.timeline(key.value);busy.value=false;error.value='';tab.value='overview';},{immediate:true});
onBeforeUnmount(()=>{revision++;});
watch(()=>props.connected,()=>{if(!props.connected){revision++;busy.value=false;}});
function moveTab(event:KeyboardEvent,index:number){let next=index;if(event.key==='ArrowRight')next=(index+1)%tabs.length;else if(event.key==='ArrowLeft')next=(index+tabs.length-1)%tabs.length;else if(event.key==='Home')next=0;else if(event.key==='End')next=tabs.length-1;else return;event.preventDefault();tab.value=tabs[next]!.key;(event.currentTarget as HTMLElement).parentElement?.querySelectorAll<HTMLButtonElement>('[role=tab]')[next]?.focus();}
async function load(kind:'detail'|'timeline'){
 if(busy.value||!canLoad.value)return;
 const request=++revision,requestKey=key.value;busy.value=true;error.value='';
 try{
  if(kind==='timeline'){
   const result=await Service.MatchTimeline(props.player.key,props.player.connectionId,props.gameId);
   if(request!==revision||requestKey!==key.value)return;
   timeline.value=result;analysisCache.setTimeline(requestKey,result);
   if(result.detail){loaded.value=result.detail;analysisCache.setDetail(requestKey,result.detail);}
  }else{
   const result=await Service.MatchDetail(props.player.key,props.player.connectionId,props.gameId);
   if(request!==revision||requestKey!==key.value)return;
   loaded.value=result;analysisCache.setDetail(requestKey,result);
  }
  if(request===revision){await nextTick();root.value?.scrollIntoView({block:'start',behavior:'auto'});}
 }catch(reason){if(request===revision)error.value=errorMessage(reason);}finally{if(request===revision)busy.value=false;}
}
</script>
<template><section ref="root" class="full-match-detail">
  <div class="analysis-tabs" role="tablist" aria-label="对局分析"><button v-for="(item,i) in tabs" :key="item.key" role="tab" :id="`${scopeId}-${gameId}-${item.key}`" :aria-selected="tab===item.key" :aria-controls="`${scopeId}-panel-${gameId}`" :tabindex="tab===item.key?0:-1" @click="tab=item.key" @keydown="moveTab($event,i)">{{item.label}}</button></div>
  <p v-if="error" class="error-banner" role="alert">{{error}}</p>
  <div :id="`${scopeId}-panel-${gameId}`" role="tabpanel" :aria-labelledby="`${scopeId}-${gameId}-${tab}`" class="analysis-tab-content">
   <template v-if="tab==='overview'">
    <template v-if="detail"><div class="scoreboard-content">      <header class="scoreboard-heading"><h3>双方表现</h3><span>{{ detail.players?.length ?? 0 }} 名玩家<span v-if="detail.partial"> · {{ (detail.players?.length ?? 0) < 10 ? '按已返回的玩家展示' : '团队对比暂缺' }}</span></span></header>
      <div class="detail-table-scroll" tabindex="0" aria-label="双方对局记分板">
        <table v-for="team in teams" :key="team" :class="{'team-blue':team===100,'team-red':team===200}">
          <caption><span><b :class="{victory:teamResult(team)==='胜利',defeat:teamResult(team)==='失败'}">{{ teamResult(team) }}</b> {{ teamName(team) }}</span><span>{{ detail.partial?'已读击杀':'击杀' }} {{ number(teamKills(team)) }}</span></caption>
          <thead><tr><th scope="col">玩家 / 英雄</th><th scope="col">K / D / A</th><th scope="col">伤害 / 承伤</th><th scope="col">补刀 / 视野</th><th scope="col">终局装备</th><th scope="col">参团率</th></tr></thead>
          <tbody><tr v-for="p in (detail.players ?? []).filter(p=>p.teamId===team)" :key="p.participantId" :class="{target:p.target}">
            <td><div class="scoreboard-player"><GameIcon :id="p.summary.championId" :size="32"/><span class="scoreboard-spells"><GameIcon kind="spells" :id="p.summary.spell1" :size="15"/><GameIcon kind="spells" :id="p.summary.spell2" :size="15"/></span><span class="scoreboard-name"><strong :title="p.displayName">{{ p.displayName }}</strong><small>{{ championName(p.summary.championId).split('·').at(-1) }}<span v-if="p.summary.position"> · {{ positionName(p.summary.position) }}</span><span v-if="p.target" class="target-label"> · 查看玩家</span></small></span></div></td>
            <td class="scoreboard-kda"><b>{{ kda(p.summary) }}</b><small>{{ decimal(sampleStats([p.summary]).kda) }} KDA</small></td>
            <td><b>{{ number(p.summary.damage) }}</b><span class="damage-track" :title="'英雄伤害；横条以本场最高伤害为基准'"><i :style="{width:`${damageWidth(p.summary.damage)}%`}"/></span><small>承伤 {{number(p.summary.damageTaken)}}</small></td>
            <td><b>{{ number(p.summary.cs) }} CS</b> <span class="cs-rate">{{ csRate(p.summary.cs,p.summary.duration) }}</span><small>{{number(p.summary.gold)}} 金币</small><small :title="`放置 / 排除守卫；控制守卫购买 ${number(p.summary.controlWards)}`">视野 {{number(p.summary.visionScore)}} · {{number(p.summary.wardsPlaced)}} / {{number(p.summary.wardsKilled)}}</small></td>
            <td><span class="detail-loadout"><GameIcon v-for="slot in 7" :key="slot" kind="items" :id="p.summary.items?.[slot-1]" :size="20"/></span></td>
            <td>{{ percent(p.participation) }}</td>
          </tr></tbody>
        </table>
      </div>
      <div v-if="detail.teams?.length" class="team-objectives"><div v-for="team in detail.teams" :key="team.teamId"><b>{{teamName(team.teamId)}}</b><span>防御塔 {{number(team.towers)}}</span><span>巨龙 {{number(team.dragons)}}</span><span>男爵 {{number(team.barons)}}</span><span>水晶 {{number(team.inhibitors)}}</span><span v-if="team.heralds!=null">先锋 {{number(team.heralds)}}</span><span v-if="team.hordes!=null">巢虫 {{number(team.hordes)}}</span></div></div>
      <details class="scoreboard-note"><summary>数据说明</summary><p>伤害条对比本场最高英雄伤害。团队占比和参团率仅在完整 5v5 且数据齐全时计算；未取得的数据保留为 —。</p></details>
</div></template>
    <template v-else><div class="personal-content"><MatchSummary v-if="match" :match="match"/></div><div class="analysis-load"><span>查看双方阵容、输出、承伤、视野与装备。</span><button class="secondary" :disabled="busy||!canLoad" @click="load('detail')">{{busy?'正在读取详情…':'读取双方详情'}}</button></div></template>
   </template>
   <template v-else-if="tab==='team'"><TeamAnalysis v-if="detail" :detail="detail"/><div v-else class="analysis-load"><span>读取双方详情后，比较六项个人与团队表现。</span><button class="secondary" :disabled="busy||!canLoad" @click="load('detail')">{{busy?'正在读取详情…':'读取双方详情'}}</button></div></template>
   <template v-else-if="tab==='timeline'"><MatchTimeline v-if="timeline&&detail" :timeline="timeline" :detail="detail"/><div v-else class="analysis-load"><span>{{provided?'这是保留的赛后资料，时间线未保留。':'按时间回看击杀、资源、位置与经济变化。'}}</span><button v-if="!provided" class="secondary" :disabled="busy||!canLoad" @click="load('timeline')">{{busy?'正在读取时间线…':'读取本场时间线'}}</button></div></template>
   <template v-else-if="tab==='build'"><MatchBuild v-if="match" :match="match" :timeline="timeline" :participant-id="target?.participantId"/><div v-if="!timeline&&!provided" class="analysis-load"><span>终局符文已展示，读取时间线后查看出装与加点顺序。</span><button class="secondary" :disabled="busy||!canLoad" @click="load('timeline')">{{busy?'正在读取时间线…':'读取本场时间线'}}</button></div></template>
  </div>
 <details class="detail-note"><summary>统计说明</summary><p>对局 #{{gameId}}。未取得的数值显示 —；提前结束不代表已确认重开，仍计入当前样本。</p></details>
 </section></template>
<style scoped>
.full-match-detail{min-width:0;}
.scoreboard-heading{display:flex;justify-content:space-between;align-items:baseline;gap:12px;margin-bottom:12px;}
.scoreboard-heading h3{font-size:14px;margin:0;}.scoreboard-heading>span{font-size:12px;color:var(--muted);}
.detail-table-scroll{overflow-x:auto;border-radius:6px;border:1px solid var(--border);}
table{width:100%;min-width:750px;border-collapse:collapse;table-layout:fixed;font-size:13px;font-variant-numeric:tabular-nums;}
caption{padding:11px 12px;text-align:left;background:var(--raised);color:var(--muted);font-size:12px;}
caption>span+span{float:right;}caption b{margin-right:8px;}.victory{color:var(--win);}.defeat{color:var(--loss);}
th,td{padding:8px 8px;border-bottom:1px solid var(--border);vertical-align:middle;text-align:right;}
th{font-size:12px;font-weight:500;color:var(--muted);padding-top:8px;padding-bottom:8px;background:var(--panel);}
th:first-child,td:first-child{text-align:left;width:28%;}th:nth-child(2){width:14%;}th:nth-child(3){width:14%;}th:nth-child(4){width:14%;}th:nth-child(5){width:22%;text-align:left;}th:last-child{width:8%;}
td:nth-child(5){text-align:left;}td b{font-weight:600;}small{display:block;font-size:11px;color:var(--muted);margin-top:4px;line-height:1.3;}
.cs-rate{color:var(--muted);font-size:11px;white-space:nowrap;}
.scoreboard-player{display:flex;align-items:center;gap:6px;min-width:0;}.scoreboard-player>.game-icon{flex:none;}.scoreboard-spells{display:flex;flex-direction:column;gap:2px;flex:none;}
.scoreboard-name{min-width:0;}.scoreboard-name strong{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:13px;font-weight:600;}.scoreboard-name small{white-space:nowrap;overflow:hidden;text-overflow:ellipsis;}
.target{background:var(--selection);}.target-label{color:var(--accent);}
.detail-loadout{gap:2px;margin:0;}.detail-loadout .game-icon{flex:none;}
.damage-track{display:block;margin:5px 0 0;height:4px;background:var(--border);border-radius:2px;overflow:hidden;}.damage-track i{display:block;height:100%;background:var(--win);}.team-red .damage-track i{background:var(--loss);}
.scoreboard-note{font-size:11px;color:var(--muted);margin-top:12px;line-height:1.6;}.scoreboard-note summary{cursor:pointer;}.scoreboard-note p{margin:8px 0 0;}
.detail-note{padding:10px 16px;font-size:11px;color:var(--muted);line-height:1.6;border-top:1px solid var(--border);}.detail-note summary{cursor:pointer;}.detail-note p{margin:8px 0 0;}.team-objectives{font-size:11px;color:var(--muted);line-height:1.5;border:1px solid var(--border);border-top:0;padding:8px 10px;}.team-objectives>div{display:flex;gap:12px;flex-wrap:wrap;padding:4px 0;}.team-objectives b{font-weight:500;color:var(--text);}.analysis-tabs{position:sticky;top:0;z-index:3;display:flex;gap:3px;padding:5px;background:var(--raised);border-block:1px solid var(--border);}.analysis-tabs button{flex:1;min-height:32px;border:0;background:none;border-radius:4px;font-size:12px;color:var(--muted);}.analysis-tabs button[aria-selected=true]{background:var(--panel);color:var(--text);box-shadow:inset 0 -2px var(--accent);}.analysis-tab-content{min-height:80px;}.personal-content,.scoreboard-content{padding:16px;}.analysis-load{display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap;padding:14px 16px;border-top:1px solid var(--border);font-size:12px;color:var(--muted);}.analysis-load button{flex:none;font-size:12px;}.scoreboard-content table{min-width:740px;}.full-match-detail .scoreboard-content .detail-table-scroll{border-radius:4px;}.full-match-detail .error-banner{margin:12px 16px;}
</style>
