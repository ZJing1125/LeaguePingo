<script setup lang="ts">
import { uiText } from "../uiText";
import type { Participant } from '../../bindings/leaguepingo/internal/core/models'
import type { Stats } from '../../bindings/leaguepingo/internal/core/models'
defineProps<{ player: Participant; stats: Stats; filtered: boolean }>()
</script>
<template>
  <details
    class="history-notice"
    v-if="player.historyState === 'ready' || player.historyState === 'empty'"
  >
    <summary>{{ uiText.historyNotice.label }}</summary>
    <p>
      {{ player.cached ? uiText.historyNotice.cached : uiText.historyNotice.fetched }} ·
      {{
        player.historyMeta?.fetchedAt
          ? new Date(player.historyMeta.fetchedAt).toLocaleString("zh-CN")
          : uiText.historyNotice.unknownTime
      }}<strong v-if="player.stale">{{ uiText.historyNotice.stale }}</strong>
    </p>
    <p>{{ player.historyMeta?.scope || uiText.historyNotice.scope }}</p>
    <p v-if="player.historyMeta?.excluded" role="status">{{ uiText.historyNotice.incomplete(player.historyMeta.returned, player.historyMeta.parsed, player.historyMeta.excluded) }}</p>
    <p>
      {{ uiText.historyNotice.denominatorHelp(filtered ? uiText.championAnalysis.currentScope : uiText.historyNotice.allSamples, stats.outcomeGames ?? 0, stats.kdaGames ?? 0) }}</p>
  </details>
</template>
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.history-notice):is(.history-notice) {
  margin: 14px 0;
  color: var(--muted);
  font-size: 12px;
}
:where(.history-notice):is(.history-notice) :deep(summary) {
  cursor: pointer;
  padding: 6px 0;
}
:where(.history-notice):is(.home-sample-note .history-notice) { font-size: 11px; }
:where(.history-notice):is(.history-notice) {
  font-size: 11px;
}
:where(.history-notice):is(.players-workspace .history-notice) {
  margin: 9px 0;
}
</style>
<style scoped>
.history-notice {
  font-size: 11px;
  color: var(--muted);
  line-height: 1.6;
}
.history-notice p {
  margin: 8px 0;
}
.history-notice [role="status"] {
  color: var(--warning-text);
}
</style>
