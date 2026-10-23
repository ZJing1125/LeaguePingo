<script setup lang="ts">
import type { Match } from "../../bindings/leaguepingo/internal/core/models";
import {
  date,
  decimal,
  duration,
  number,
  kda,
  outcome,
  queueName,
  sampleStats,
} from "../presentation";
import { championName } from "../gameAssets";
import GameIcon from "./GameIcon.vue";
import AppIcon from "./AppIcon.vue";
import MatchSummary from './MatchSummary.vue';
defineProps<{ match: Match; expanded: boolean; compact?: boolean; fixed?: boolean }>();
const emit = defineEmits<{ toggle: [] }>();
</script>
<template>
  <article
    class="match-card"
    :class="match.win == null ? 'unknown' : match.win ? 'won' : 'lost'"
  >
    <component :is="fixed ? 'div' : 'button'"
      class="match-main"
      :class="{'match-static':fixed}"
      :aria-expanded="fixed ? undefined : expanded"
      @click="!fixed && emit('toggle')"
    >
      <span class="match-context"
        ><strong>{{ queueName(match.queueId) }}</strong
        ><small>{{ date(match.playedAt) }}</small
        ><b class="outcome"
          >{{ outcome(match) }} <span>{{ duration(match.duration) }}</span></b
        ></span
      >
      <span class="match-hero"
        ><GameIcon :id="match.championId" :size="compact ? 32 : 40" /><small>{{
          championName(match.championId).split("·")[0]
        }}</small></span
      >
      <span class="match-spells"
        ><GameIcon kind="spells" :id="match.spell1" :size="19" /><GameIcon
          kind="spells"
          :id="match.spell2"
          :size="19"
      /></span>
      <span class="match-performance"
        ><strong>{{ kda(match) }}</strong
        ><small>{{ decimal(sampleStats([match]).kda) }} KDA</small
        ><small
          >补刀 {{ number(match.cs) }} · 伤害 {{ number(match.damage) }}</small
        ></span
      >
      <span class="match-items" aria-label="本玩家终局装备"
        ><GameIcon
          v-for="slot in 7"
          :key="slot"
          kind="items"
          :id="match.items?.[slot - 1]"
          :size="compact ? 17 : 24"
      /></span>
      <span v-if="!fixed" class="expand-indicator"
        ><AppIcon name="chevron" :size="16"
      /></span>
    </component>
    <div v-if="expanded" class="match-expanded">
      <MatchSummary :match="match"/>
      <slot name="details" />
      <details class="detail-note"><summary>统计说明</summary><p>对局 #{{ match.gameId }}。未取得的数值显示 —；提前结束不代表已确认重开，仍计入当前样本。</p></details>
    </div>
  </article>
</template>
<style scoped>
.match-expanded {padding:20px;container-type:inline-size;background:var(--panel);}
.match-main.match-static {cursor:default;}
.match-main.match-static:hover {background:none;}
.detail-note { font-size: 11px; color: var(--muted); line-height: 1.6; }
.detail-note summary { cursor: pointer; }
.detail-note p { margin: 8px 0; }
</style>
