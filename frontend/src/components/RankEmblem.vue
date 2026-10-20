<script setup lang="ts">
import { computed } from "vue";
import { tierName } from "../presentation";
const props = defineProps<{ tier?: string }>();
const colors: Record<string, string> = {
  IRON: "#a3a8b1", BRONZE: "#bf9068", SILVER: "#c1d3e6", GOLD: "#edc269",
  PLATINUM: "#64cbd0", EMERALD: "#55d5ab", DIAMOND: "#99b8ff",
  MASTER: "#cf92ef", GRANDMASTER: "#f28480", CHALLENGER: "#f4ce76",
};
const color = computed(() => colors[props.tier ?? ""] ?? "var(--scout-muted)");
</script>
<template>
  <svg class="rank-emblem" viewBox="0 0 64 50" :style="{ color }" role="img" :aria-label="tier ? tierName(tier) : '段位暂缺'">
    <path d="M27 34 13 27 4 9 17 16 10 5 26 17M37 34 51 27 60 9 47 16 54 5 38 17" fill="currentColor" opacity=".3" />
    <path d="m27 34-14-7L4 9l15 17 9 3m9 5 14-7L60 9 45 26l-9 3" fill="none" stroke="currentColor" stroke-width="1.7" />
    <path d="m32 5 13 12-4 19-9 9-9-9-4-19z" fill="currentColor" opacity=".16" />
    <path d="m32 8 10 11-4 16-6 7-6-7-4-16zM23 20h18M32 8v34" fill="none" stroke="currentColor" stroke-width="1.8" />
    <path d="m32 15 5 8-5 9-5-9z" fill="currentColor" />
  </svg>
</template>
