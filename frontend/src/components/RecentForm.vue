<script setup lang="ts">
import type { Match } from "../../bindings/leaguepingo/internal/core/models";
import { isRemake } from "../matchStats";
defineProps<{ matches?: Match[] | null }>();
const result = (match: Match) => isRemake(match) ? "重开" : match.win == null ? "未知" : match.win ? "胜" : "负";
const resultClass = (match: Match) => isRemake(match) ? "unknown" : match.win == null ? "unknown" : match.win ? "won" : "lost";
</script>
<template>
  <span
    class="recent-form"
    :aria-label="`近五场：${
      (matches ?? [])
        .slice(0, 5)
        .map(result)
        .join('、') || '暂无样本'
    }`"
    ><span
      v-for="(match, i) in (matches ?? []).slice(0, 5)"
      :key="i"
      class="form-result"
      :class="resultClass(match)"
      aria-hidden="true"
      >{{ isRemake(match) ? "重" : match.win == null ? "?" : match.win ? "胜" : "负" }}</span
    ><span v-if="!matches?.length" class="muted">暂无样本</span></span
  >
</template>
