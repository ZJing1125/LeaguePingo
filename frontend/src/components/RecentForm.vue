<script setup lang="ts">
import type { Match } from "../../bindings/leaguepingo/internal/core/models";
defineProps<{ matches?: Match[] | null }>();
</script>
<template>
  <span
    class="recent-form"
    :aria-label="`近五场：${
      (matches ?? [])
        .slice(0, 5)
        .map((m) => (m.win == null ? '未知' : m.win ? '胜' : '负'))
        .join('、') || '暂无样本'
    }`"
    ><span
      v-for="(match, i) in (matches ?? []).slice(0, 5)"
      :key="i"
      class="form-result"
      :class="match.win == null ? 'unknown' : match.win ? 'won' : 'lost'"
      aria-hidden="true"
      >{{ match.win == null ? "?" : match.win ? "胜" : "负" }}</span
    ><span v-if="!matches?.length" class="muted">暂无样本</span></span
  >
</template>
