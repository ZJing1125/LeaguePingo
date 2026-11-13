<script setup lang="ts">
import { computed } from "vue";
import { percent } from "../presentation";
import RoleIcon from "./RoleIcon.vue";
const props = defineProps<{ value: number | null; short: string; label: string; caption: string; detail: string; title: string; role?: string; measure?: string }>();
const tone = computed(() => props.value == null ? "unknown" : props.measure ? "positive" : props.value >= 50 ? "positive" : props.value >= 40 ? "warning" : "negative");
</script>
<template>
  <span class="scout-ring-metric" :class="tone" :title="title" role="group" :aria-label="`${label} ${measure ?? ''}：${role !== undefined ? caption : percent(value)}，${caption}，${detail}`">
    <span class="scout-ring" :style="{ '--ring-value': `${Math.max(0, Math.min(100, value ?? 0))}%` }" aria-hidden="true">
      <span class="scout-ring-center"><RoleIcon v-if="role !== undefined" :role="role" /><template v-else><b>{{ percent(value) }}</b><small v-if="measure">{{ measure }}</small></template></span>
    </span>
    <strong>{{ label }}</strong><span>{{ caption }}</span><small>{{ detail }}</small>
  </span>
</template>
<style scoped>
/* Preserve source cascade and contextual sizing; shared tokens remain global. */
.scout-ring-metric { --ring-color: var(--scout-good); display: flex; flex-direction: column; align-items: center; gap: 2px; padding-inline: 1px; min-width: 0; color: var(--scout-text); }
.scout-ring-metric + .scout-ring-metric { border-left: 1px solid var(--scout-border); }
.scout-ring-metric.warning { --ring-color: var(--scout-warning); }
.scout-ring-metric.negative { --ring-color: var(--scout-bad); }
.scout-ring-metric.unknown { --ring-color: var(--scout-muted); }
.scout-ring { display: grid; place-items: center; width: 43px; height: 43px; flex: none; border-radius: 50%; padding: 3px; background: conic-gradient(var(--ring-color) var(--ring-value), var(--scout-track) 0); }
.scout-ring-center { display: flex; flex-direction: column; align-items: center; justify-content: center; width: 100%; height: 100%; gap: 1px; border-radius: 50%; background: var(--scout-inset); }
.scout-card .scout-rings .scout-ring b { font-size: 16px; font-weight: 700; letter-spacing: -.3px; line-height: 16px; color: var(--scout-text); }
.scout-card .scout-rings .negative .scout-ring b { color: var(--scout-bad); }
.scout-card .scout-rings .unknown .scout-ring b { color: var(--scout-muted); }
.scout-card .scout-rings .scout-ring small { font-size: 11px; color: var(--scout-muted); line-height: 10px; margin: 0; }
.scout-card .scout-rings .scout-ring-metric > strong { font-size: 11px; line-height: 13px; font-weight: 600; white-space: nowrap; }
.scout-card .scout-rings .scout-ring-metric > span:not(.scout-ring) { font-size: 11px; line-height: 12px; white-space: nowrap; }
.scout-card .scout-rings .scout-ring-metric > small { display: block; color: var(--scout-muted); font-size: 11px; line-height: 11px; white-space: nowrap; margin: 0; }
.overlay-shell .scout-ring { width: 36px; height: 36px; }
.overlay-shell .scout-card .scout-rings .scout-ring b { font-size: 15px; line-height: 14px; }
.overlay-shell .scout-card .scout-rings .scout-ring-metric > small { font-size: 11px; line-height: 12px; }
.overlay-shell .scout-card .scout-rings .scout-ring-metric > strong { font-size: 11px; line-height: 12px; }
.overlay-shell .scout-card .scout-rings .scout-ring-metric > span:not(.scout-ring) { font-size: 11px; }
.scout-position-cell .scout-ring-metric { width: 100%; }
.scout-position-cell + .scout-ring-metric { border-left: 0; }
</style>
