<script setup lang="ts">
withDefaults(defineProps<{ feature: string; label?: string }>(), { label: '实验性' });
</script>

<template>
  <span class="experimental-badge" :title="`${feature}：国服实机验证未完成`" :aria-label="`${feature}：${label}，国服实机验证未完成`">{{ label }}</span>
</template>

<style scoped>
.experimental-badge { display: inline-block; flex: none; white-space: nowrap; border: 1px solid var(--border); border-radius: 3px; padding: 0 4px; font-size: 11px; font-weight: 400; line-height: 16px; color: var(--muted); vertical-align: middle; }
</style>
