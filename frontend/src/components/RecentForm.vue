<script setup lang="ts">
import { uiText } from "../uiText";
import type { Match } from "../../bindings/leaguepingo/internal/core/models";
import { isRemake } from "../matchStats";
defineProps<{ matches?: Match[] | null }>();
const result = (match: Match) => isRemake(match) ? uiText.common.remake : match.win == null ? uiText.common.unknown : match.win ? uiText.common.winMark : uiText.common.lossMark;
const resultClass = (match: Match) => isRemake(match) ? "unknown" : match.win == null ? "unknown" : match.win ? "won" : "lost";
</script>
<template>
  <span
    class="recent-form"
    :aria-label="uiText.recentForm.recent((matches ?? [])
        .slice(0, 5)
        .map(result)
        .join('、') || uiText.recentForm.noSample)"
    ><span
      v-for="(match, i) in (matches ?? []).slice(0, 5)"
      :key="i"
      class="form-result"
      :class="resultClass(match)"
      aria-hidden="true"
      >{{ isRemake(match) ? uiText.common.remakeMark : match.win == null ? "?" : match.win ? uiText.common.winMark : uiText.common.lossMark }}</span
    ><span v-if="!matches?.length" class="muted">{{ uiText.recentForm.noSample }}</span></span
  >
</template>
