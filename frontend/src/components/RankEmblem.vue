<script setup lang="ts">
import { uiText } from "../uiText";
import { tierName } from "../presentation";
import { rankEmblem } from "../gameAssets";
defineProps<{ tier?: string }>();
</script>
<template>
  <img v-if="rankEmblem(tier)" class="rank-emblem" :src="rankEmblem(tier)" :alt="tierName(tier!)" decoding="async" />
  <span v-else class="rank-emblem rank-placeholder" role="img" :aria-label="tier === 'NONE' || tier === 'UNRANKED' ? uiText.common.unranked : uiText.rankEmblem.missing">—</span>
</template>
<style scoped>
/* Preserve source cascade and contextual sizing; shared tokens remain global. */
.home-rank>.rank-emblem{width:44px;height:44px;}
.scout-rank .rank-emblem { width: 36px; height: 28px; flex: none; }
.overlay-shell .scout-rank .rank-emblem { height: 22px; width: 30px; }
.rank-emblem { object-fit: contain; }
.rank-placeholder { display: grid; place-items: center; color: var(--muted); font-size: 20px; opacity: .5; }
.rank-card > .rank-emblem { width: 40px; height: 40px; object-fit: contain; flex: none; }
.scout-card .scout-rank .rank-emblem { width: 36px; height: 28px; }
.overlay-shell .scout-card .scout-rank .rank-emblem { width: 30px; height: 22px; }
.sidebar-identity .rank-emblem {object-fit:contain;flex:none;height:30px;width:34px;}
.overlay-shell .scout-rank .rank-emblem {height:22px;}
</style>
