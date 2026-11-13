<script setup lang="ts">
import { computed } from 'vue';
import type { MatchDetail } from '../../bindings/leaguepingo/internal/core/models';
import { arenaResult } from '../matchStats';
import { decimal, kda, number, sampleStats } from '../presentation';
import { uiText } from '../uiText';
import GameIcon from './GameIcon.vue';
const props = defineProps<{ detail: MatchDetail }>();
const players = computed(() => [...(props.detail.players ?? [])].sort((a, b) => (arenaResult(a.summary)?.placement ?? 99) - (arenaResult(b.summary)?.placement ?? 99)));
</script>
<template>
  <section class="arena-scoreboard">
    <header><h3>{{ uiText.arena.scoreboard }}</h3><span>{{ uiText.matchDetails.playerCount(detail.players?.length ?? 0) }}</span></header>
    <p>{{ uiText.arena.scoreboardHint }}</p>
    <div class="arena-table-scroll" tabindex="0" :aria-label="uiText.arena.scoreboard">
      <table>
        <thead><tr><th scope="col">{{ uiText.arena.placement }}</th><th scope="col">{{ uiText.matchDetails.playerChampion }}</th><th scope="col">KDA</th><th scope="col">{{ uiText.common.damage }}</th><th scope="col">{{ uiText.arena.gold }}</th><th scope="col">{{ uiText.common.finalItems }}</th></tr></thead>
        <tbody><tr v-for="p in players" :key="p.participantId" :class="{target:p.target}">
          <td><b>{{ arenaResult(p.summary) ? uiText.arena.place(arenaResult(p.summary)!.placement) : '—' }}</b></td>
          <td><span class="arena-player"><GameIcon :id="p.summary.championId" :size="32"/><strong :title="p.displayName">{{ p.displayName }}</strong></span></td>
          <td><b>{{ kda(p.summary) }}</b><small>{{ decimal(sampleStats([p.summary]).kda) }} KDA</small></td>
          <td>{{ number(p.summary.damage) }}<small>{{ uiText.matchDetails.taken(number(p.summary.damageTaken)) }}</small></td>
          <td>{{ number(p.summary.gold) }}</td>
          <td><span class="arena-items"><GameIcon v-for="slot in 7" :key="slot" kind="items" :id="p.summary.items?.[slot-1]" :size="20"/></span></td>
        </tr></tbody>
      </table>
    </div>
  </section>
</template>
<style scoped>
.arena-scoreboard{padding:16px;min-width:0;}
header{display:flex;align-items:baseline;justify-content:space-between;gap:12px;}
h3{font-size:14px;margin:0;}header>span,p{font-size:12px;color:var(--muted);}p{line-height:1.6;}
.arena-table-scroll{overflow-x:auto;border:1px solid var(--border);border-radius:6px;}
table{width:100%;min-width:760px;border-collapse:collapse;font-size:12px;font-variant-numeric:tabular-nums;table-layout:fixed;}
th,td{text-align:right;padding:9px 8px;border-bottom:1px solid var(--border);}
th{font-size:11px;color:var(--muted);font-weight:500;background:var(--panel);}
th:first-child,td:first-child{text-align:left;width:9%;}th:nth-child(2),td:nth-child(2){text-align:left;width:25%;}th:last-child,td:last-child{width:22%;text-align:left;}
tbody tr:last-child td{border-bottom:0;}.target{background:var(--selection);}
.arena-player{display:flex;align-items:center;gap:7px;min-width:0;}.arena-player strong{white-space:nowrap;overflow:hidden;text-overflow:ellipsis;font-weight:600;}
td small{display:block;margin-top:4px;font-size:11px;color:var(--muted);}.arena-items{display:flex;gap:2px;}.arena-items :deep(.game-icon){flex:none;}
</style>
