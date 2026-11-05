<script setup lang="ts">
import { uiText } from "../uiText";
import { computed } from 'vue';
import type { Participant, Rank } from '../../bindings/leaguepingo/internal/core/models';
import { rankPresentation } from '../rankPresentation';
import { percent, tierName } from '../presentation';
import RankEmblem from './RankEmblem.vue';

const props = withDefaults(defineProps<{ player: Participant; queue?: string }>(), { queue: 'RANKED_SOLO_5x5' });
const data = computed(() => rankPresentation(props.player, props.queue));
const label = computed(() => props.queue === 'RANKED_FLEX_SR' ? uiText.queueSwitch.flex : uiText.common.solo);
const tierOrder: Record<string, number> = { NONE: 0, UNRANKED: 0, IRON: 1, BRONZE: 2, SILVER: 3, GOLD: 4, PLATINUM: 5, EMERALD: 6, DIAMOND: 7, MASTER: 8, GRANDMASTER: 9, CHALLENGER: 10 };
const divisionOrder: Record<string, number> = { IV: 1, III: 2, II: 3, I: 4 };

type RankRecord = { label: string; tier?: string; division?: string; title?: string };
const title = (tier: string, division?: string) => ['NONE', 'UNRANKED'].includes(tier)
  ? uiText.common.unranked
  : `${tierName(tier)} ${['MASTER', 'GRANDMASTER', 'CHALLENGER'].includes(tier) ? '' : division ?? ''}`.trim();
const score = (record: RankRecord) => (tierOrder[record.tier ?? ''] ?? -1) * 10 + (divisionOrder[record.division ?? ''] ?? 0);
const valid = (record: RankRecord) => !!record.tier && !['NONE', 'UNRANKED'].includes(record.tier);

const sourceRecords = computed<RankRecord[]>(() => {
  const rank: Rank | undefined = data.value.rank;
  if (!rank) return [];
  return [
    { label: uiText.rankPanel.seasonPeak, tier: rank.highestTier, division: rank.highestDivision },
    { label: uiText.rankPanel.previousPeak, tier: rank.previousHighestTier, division: rank.previousHighestDivision },
    { label: uiText.rankPanel.previousFinal, tier: rank.previousTier, division: rank.previousDivision },
  ].filter(valid);
});
const knownBest = computed<RankRecord | undefined>(() => {
  const current = data.value.rank;
  const candidates = [
    ...sourceRecords.value,
    { label: uiText.rankPanel.current, tier: current?.tier, division: current?.division },
  ].filter(valid);
  return candidates.sort((a, b) => score(b) - score(a))[0];
});
const history = computed<RankRecord[]>(() => [
  ...(knownBest.value ? [{ ...knownBest.value, label: uiText.rankPanel.historyPeak, title: uiText.rankPanel.historyHint }] : []),
  ...sourceRecords.value,
]);
</script>

<template>
  <section class="rank-panel" :title="data.title">
    <header><h2>{{ label }}</h2><span v-if="data.retained">{{ uiText.playerProfileHeader.stale }}</span></header>
    <div class="rank-current">
      <RankEmblem :tier="data.rank?.tier" />
      <div><strong>{{ data.rank ? title(data.rank.tier, data.rank.division) : data.label }}</strong><span>{{ data.rank?.lp != null ? `${data.rank.lp.toLocaleString()} LP` : '—' }}</span></div>
      <div class="rank-record" :title="data.record?.title"><b>{{ percent(data.record?.rate) }}</b><small>{{ data.record ? uiText.rankPanel.outcomes(data.record.wins.toLocaleString(), data.record.losses.toLocaleString()) : uiText.common.pendingOutcome }}</small></div>
    </div>
    <details v-if="history.length" class="rank-history">
      <summary>{{ uiText.rankPanel.records }}<span>{{ knownBest?.tier ? title(knownBest.tier, knownBest.division) : '' }}</span></summary>
      <div v-for="row in history" :key="row.label" :title="row.title">
        <span>{{ row.label }}</span><RankEmblem :tier="row.tier" /><b>{{ title(row.tier!, row.division) }}</b>
      </div>
      <p>{{ uiText.rankPanel.historyHelp }}</p>
    </details>
  </section>
</template>

<style scoped>
.rank-panel{min-width:0;background:var(--panel);border:1px solid var(--border);border-radius:8px;overflow:hidden;}
.rank-panel>header{display:flex;justify-content:space-between;align-items:center;padding:10px 12px;border-bottom:1px solid var(--border);height:36px;}
.rank-panel h2{font-size:13px;font-weight:600;margin:0;}.rank-panel header>span{font-size:11px;color:var(--warning);}
.rank-current{display:flex;gap:9px;align-items:center;padding:12px 10px;}.rank-current>.rank-emblem{width:48px;height:48px;flex:none;}.rank-current>div:not(.rank-record){min-width:0;flex:1;}
.rank-current strong{font-size:14px;line-height:1.4;display:block;white-space:normal;overflow-wrap:anywhere;}.rank-current>div>span{font-size:12px;color:var(--muted);display:block;margin-top:4px;}
.rank-record{text-align:right;flex:none;font-variant-numeric:tabular-nums;}.rank-record b{font-size:13px;font-weight:500;}.rank-record small{font-size:11px;color:var(--muted);display:block;margin-top:5px;white-space:nowrap;}
.rank-history{border-top:1px solid var(--border);font-size:11px;color:var(--muted);}.rank-history summary{padding:8px 12px;cursor:pointer;}.rank-history summary>span{float:right;color:var(--text);}
.rank-history>div{display:flex;align-items:center;gap:7px;padding:5px 12px;font-variant-numeric:tabular-nums;}.rank-history>div:first-of-type{background:color-mix(in srgb,var(--accent) 7%,transparent);}.rank-history>div>span{flex:1;}.rank-history .rank-emblem{width:26px;height:26px;}.rank-history b{font-size:12px;font-weight:500;color:var(--text);}
.rank-history p{margin:3px 12px 10px;font-size:11px;line-height:1.45;color:var(--muted);}
</style>
