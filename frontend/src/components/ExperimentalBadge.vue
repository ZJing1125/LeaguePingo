<script setup lang="ts">
import { uiText } from "../uiText";
withDefaults(defineProps<{ feature: string; label?: string }>(), { label: uiText.common.experimental });
</script>

<template>
  <span class="experimental-badge" :title="`${feature}：${label}`" :aria-label="`${feature}：${label}`">{{ label }}</span>
</template>

<style scoped>
.experimental-badge { display: inline-block; flex: none; white-space: nowrap; border: 1px solid var(--border); border-radius: 3px; padding: 0 4px; font-size: 11px; font-weight: 400; line-height: 16px; color: var(--muted); vertical-align: middle; }
</style>
