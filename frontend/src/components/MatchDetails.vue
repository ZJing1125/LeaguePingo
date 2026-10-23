<script setup lang="ts">
import { computed, ref, watch } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type {
  MatchDetail,
  Participant,
} from "../../bindings/leaguepingo/internal/core/models";
import { errorMessage, kda, number, percent, decimal, sampleStats, positionName } from "../presentation";
import { championName } from "../gameAssets";
import GameIcon from "./GameIcon.vue";
const props = defineProps<{
  player: Participant;
  gameId: number;
  connected?: boolean;
  provided?: MatchDetail | null;
}>();
const loaded = ref<MatchDetail | null>(null),
  busy = ref(false),
  error = ref("");
let revision = 0;
const teams = computed(() => [...new Set((detail.value?.players ?? []).map(p=>p.teamId))].sort());
function teamName(id:number){return id===100?'蓝方':id===200?'红方':`队伍 ${id}`;}
function teamResult(id:number){const values=(detail.value?.players ?? []).filter(p=>p.teamId===id).map(p=>p.summary.win).filter(v=>v!=null);return values.length && values.every(v=>v===values[0])?(values[0]?'胜利':'失败'):'';}
function teamKills(id:number){const values=(detail.value?.players ?? []).filter(p=>p.teamId===id).map(p=>p.summary.kills);return values.every(v=>v!=null)?values.reduce<number>((a,v)=>a+(v??0),0):null;}
function damageWidth(damage:number|null|undefined){const max=Math.max(0,...(detail.value?.players ?? []).map(p=>p.summary.damage??0));return damage!=null && max>0?Math.max(0,Math.min(100,damage*100/max)):0;}
function csRate(value:number|null|undefined,duration:number|null|undefined){return value!=null && duration && duration>0?`${decimal(value*60/duration)} / 分`:'—';}
const detail = computed(() => props.provided ?? loaded.value);
watch(
  () => `${props.player.key}:${props.player.connectionId}:${props.gameId}`,
  () => {
    revision++;
    loaded.value = null;
    busy.value = false;
    error.value = "";
  },
);
async function load() {
  if (busy.value || !props.connected || props.player.stale) return;
  const request = ++revision;
  busy.value = true;
  error.value = "";
  try {
    const result = await Service.MatchDetail(
      props.player.key,
      props.player.connectionId,
      props.gameId,
    );
    if (request === revision) loaded.value = result;
  } catch (reason) {
    if (request === revision) error.value = errorMessage(reason);
  } finally {
    if (request === revision) busy.value = false;
  }
}
</script>
<template>
  <section class="full-match-detail">
    <button
      v-if="!detail"
      class="secondary"
      :disabled="busy || !connected || player.stale"
      @click="load"
    >
      {{ busy ? "正在读取详情…" : "读取双方详情" }}
    </button>
    <p v-if="error" class="error-banner" role="alert">{{ error }}</p>
    <template v-if="detail">
      <header class="scoreboard-heading"><h3>双方表现</h3><span>{{ detail.players?.length ?? 0 }} 名玩家<span v-if="detail.partial"> · {{ (detail.players?.length ?? 0) < 10 ? '按已返回的玩家展示' : '团队对比暂缺' }}</span></span></header>
      <div class="detail-table-scroll" tabindex="0" aria-label="双方对局记分板">
        <table v-for="team in teams" :key="team" :class="{'team-blue':team===100,'team-red':team===200}">
          <caption><span><b :class="{victory:teamResult(team)==='胜利',defeat:teamResult(team)==='失败'}">{{ teamResult(team) }}</b> {{ teamName(team) }}</span><span>{{ detail.partial?'已读击杀':'击杀' }} {{ number(teamKills(team)) }}</span></caption>
          <thead><tr><th scope="col">玩家 / 英雄</th><th scope="col">K / D / A</th><th scope="col">英雄伤害</th><th scope="col">补刀 / 金币</th><th scope="col">终局装备</th><th scope="col">参团率</th></tr></thead>
          <tbody><tr v-for="p in (detail.players ?? []).filter(p=>p.teamId===team)" :key="p.participantId" :class="{target:p.target}">
            <td><div class="scoreboard-player"><GameIcon :id="p.summary.championId" :size="32"/><span class="scoreboard-spells"><GameIcon kind="spells" :id="p.summary.spell1" :size="15"/><GameIcon kind="spells" :id="p.summary.spell2" :size="15"/></span><span class="scoreboard-name"><strong :title="p.displayName">{{ p.displayName }}</strong><small>{{ championName(p.summary.championId).split('·').at(-1) }}<span v-if="p.summary.position"> · {{ positionName(p.summary.position) }}</span><span v-if="p.target" class="target-label"> · 查看玩家</span></small></span></div></td>
            <td class="scoreboard-kda"><b>{{ kda(p.summary) }}</b><small>{{ decimal(sampleStats([p.summary]).kda) }} KDA</small></td>
            <td><b>{{ number(p.summary.damage) }}</b><span class="damage-track" :title="'英雄伤害；横条以本场最高伤害为基准'"><i :style="{width:`${damageWidth(p.summary.damage)}%`}"/></span><small>{{ p.damageShare!=null?`团队 ${percent(p.damageShare)}`:'团队占比 —' }}</small></td>
            <td><b>{{ number(p.summary.cs) }} CS</b> <span class="cs-rate">{{ csRate(p.summary.cs,p.summary.duration) }}</span><small>{{ number(p.summary.gold) }} 金币<span v-if="p.goldShare!=null"> · {{ percent(p.goldShare) }}</span></small></td>
            <td><span class="detail-loadout"><GameIcon v-for="slot in 7" :key="slot" kind="items" :id="p.summary.items?.[slot-1]" :size="20"/></span></td>
            <td>{{ percent(p.participation) }}</td>
          </tr></tbody>
        </table>
      </div>
      <details class="scoreboard-note"><summary>数据说明</summary><p>伤害条对比本场最高英雄伤害。团队占比和参团率仅在完整 5v5 且数据齐全时计算；未取得的数据保留为 —。</p></details>
    </template>
  </section>
</template>
<style scoped>
.full-match-detail{margin-top:20px;min-width:0;}
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
</style>
