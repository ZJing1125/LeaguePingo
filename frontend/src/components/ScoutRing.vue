<script setup lang="ts">
import { computed } from "vue";
import { percent } from "../presentation";
import RoleIcon from "./RoleIcon.vue";
const props = defineProps<{ value: number | null; short: string; label: string; caption: string; detail: string; title: string; role?: string }>();
const tone = computed(() => props.value == null ? "unknown" : props.value >= 50 ? "positive" : props.value >= 40 ? "warning" : "negative");
</script>
<template>
  <span class="scout-ring-metric" :class="tone" :title="title" role="group" :aria-label="`${label}：${role !== undefined ? caption : percent(value)}，${caption}，${detail}`">
    <span class="scout-ring" :style="{ '--ring-value': `${Math.max(0, Math.min(100, value ?? 0))}%` }" aria-hidden="true">
      <span class="scout-ring-center"><RoleIcon v-if="role !== undefined" :role="role" /><template v-else><b>{{ percent(value) }}</b><small>{{ short }}</small></template></span>
    </span>
    <strong>{{ label }}</strong><span>{{ caption }}</span><small>{{ detail }}</small>
  </span>
</template>
