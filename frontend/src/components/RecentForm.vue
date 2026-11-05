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
<style scoped>
/* Retain original global import order; shared controls and ancestor contexts stay in effect. */
.recent-form {
  display: inline-flex;
  gap: 4px;
  align-items: center;
}
.form-result {
  display: inline-grid;
  place-items: center;
  width: 21px;
  height: 21px;
  font-size: 11px;
  border-radius: 4px;
  background: #2a303b;
  color: #a9b5c6;
}
.form-result.won {
  background: #203e38;
  color: #98e6d1;
}
.form-result.lost {
  background: #422c37;
  color: #f2a8b8;
}
.overlay-player .form-result {
  width: 18px;
  height: 18px;
  font-size: 10px;
}
.overlay-player .recent-form {
  gap: 2px;
}
:root[data-theme="dark"] :is(.desktop-app, .drawer-floating) .form-result.won { background: #2b4365; color: #b8d5ff; }
:root[data-theme="dark"] :is(.desktop-app, .drawer-floating) .form-result.lost { background: #533341; color: #ffc0cf; }
:root[data-theme="dark"] :is(.desktop-app, .drawer-floating) .form-result.unknown { background: var(--raised); color: var(--muted); }
.home-form-last .recent-form { height: 34px; gap: 4px; }
.form-result.won {
  background: #497d65;
  color: #fbfff3;
}
.form-result.lost {
  background: #a36d72;
  color: #fff5f3;
}
.form-result.unknown {
  background: #858f87;
  color: #fff;
}
@media (max-width: 999px) {

  .players-workspace .recent-form .form-result {
    width: 15px;
    height: 19px;
    font-size: 10px;
  }

}
</style>
