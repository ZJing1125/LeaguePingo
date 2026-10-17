<script setup lang="ts">
import type { Participant } from '../../bindings/leaguepingo/internal/core/models'
import type { Stats } from '../../bindings/leaguepingo/internal/core/models'
defineProps<{ player: Participant; stats: Stats; filtered: boolean }>()
</script>
<template>
  <details
    class="history-notice"
    v-if="player.historyState === 'ready' || player.historyState === 'empty'"
  >
    <summary>样本范围与统计说明</summary>
    <p>
      {{ player.cached ? "来自缓存" : "本次获取" }} ·
      {{
        player.historyMeta?.fetchedAt
          ? new Date(player.historyMeta.fetchedAt).toLocaleString("zh-CN")
          : "获取时间未知"
      }}<strong v-if="player.stale"> · 旧资料，尚未更新</strong>
    </p>
    <p>{{ player.historyMeta?.scope || "当前已获取样本" }}</p>
    <p v-if="player.historyMeta?.excluded" role="status">
      数据不完整：返回 {{ player.historyMeta.returned }} 条，已核验
      {{ player.historyMeta.parsed }} 场，排除
      {{ player.historyMeta.excluded }} 条。
    </p>
    <p>
      {{ filtered ? "当前筛选样本" : "当前全部样本" }}：胜率按
      {{ stats.outcomeGames ?? 0 }} 场确定胜负计算；汇总 KDA 按
      {{ stats.kdaGames ?? 0 }} 场完整数据计算（总击杀＋总助攻）÷ max(1,
      总死亡)。未知字段不计入相应统计。
    </p>
  </details>
</template>
<style scoped>
.history-notice {
  font-size: 11px;
  color: #a1b5c9;
  line-height: 1.6;
}
.history-notice p {
  margin: 8px 0;
}
.history-notice [role="status"] {
  color: #edc184;
}
</style>
