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
  mapName,
  positionName,
  runeName,
  sampleStats,
} from "../presentation";
import { championName } from "../gameAssets";
import GameIcon from "./GameIcon.vue";
import AppIcon from "./AppIcon.vue";
defineProps<{ match: Match; expanded: boolean; compact?: boolean }>();
const emit = defineEmits<{ toggle: [] }>();
</script>
<template>
  <article
    class="match-card"
    :class="match.win == null ? 'unknown' : match.win ? 'won' : 'lost'"
  >
    <button
      class="match-main"
      :aria-expanded="expanded"
      @click="emit('toggle')"
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
      <span class="expand-indicator"
        ><AppIcon name="chevron" :size="16"
      /></span>
    </button>
    <div v-if="expanded" class="match-expanded">
      <div class="detail-metrics">
        <span>时长 {{ duration(match.duration) }}</span
        ><span>补刀 {{ number(match.cs) }}</span
        ><span>金币 {{ number(match.gold) }}</span
        ><span>英雄伤害 {{ number(match.damage) }}</span>
      </div>
      <div class="match-context-detail" aria-label="对局上下文">
        <span>{{ mapName(match.mapId) }}</span>
        <span>{{ positionName(match.position) }}</span>
        <span v-if="match.lane && !match.position"
          >客户端线路：{{ match.lane }}</span
        >
        <span
          v-if="match.earlyEnd === true"
          class="early-end"
          title="客户端标记提前结束；不等同于确认重开，不自动从统计中排除"
          >提前结束</span
        >
        <span v-else-if="match.earlyEnd == null">提前结束信号未获取</span>
      </div>
      <div class="match-runes" aria-label="本玩家符文">
        <template v-if="match.runeIds?.length">
          <span
            v-for="(rune, index) in match.runeIds"
            :key="`${rune}:${index}`"
            :title="`符文 #${rune}`"
            >{{ runeName(rune) }}</span
          >
          <small v-if="match.runePrimary || match.runeSecondary"
            >{{ match.runePrimary ? runeName(match.runePrimary) : "主系未知" }}
            /
            {{
              match.runeSecondary ? runeName(match.runeSecondary) : "副系未知"
            }}</small
          >
        </template>
        <small v-else>符文未获取</small>
      </div>
      <slot name="details" /><span class="detail-note"
        >对局 #{{ match.gameId }} · 双方详情按需读取 · 缺失字段显示 —</span
      >
    </div>
  </article>
</template>
