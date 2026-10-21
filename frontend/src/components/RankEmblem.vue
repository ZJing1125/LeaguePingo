<script setup lang="ts">
import { tierName } from "../presentation";
import { rankEmblem } from "../gameAssets";
defineProps<{ tier?: string }>();
</script>
<template>
  <img v-if="rankEmblem(tier)" class="rank-emblem" :src="rankEmblem(tier)" :alt="tierName(tier!)" decoding="async" />
  <span v-else class="rank-emblem rank-placeholder" role="img" :aria-label="tier === 'NONE' || tier === 'UNRANKED' ? '未定级' : '段位暂缺'">—</span>
</template>
