<script setup lang="ts">
import { computed } from "vue";
import type { Snapshot } from "../../bindings/leaguepingo/internal/core/models";
import type { OverlayState } from "../../bindings/leaguepingo/models";
import { percent, decimal, preparationLabel, recentOutcomes, championSample, date } from "../presentation";
import GameIcon from "./GameIcon.vue";
import { championName } from "../gameAssets";
const props = defineProps<{
  snapshot: Snapshot | null;
  state: OverlayState | null;
  status: string;
  busy: boolean;
  error: string;
}>();
const players = computed(() => props.snapshot?.participants ?? []);
defineEmits<{ hide: []; pin: [] }>();
</script>
<template>
  <main class="overlay-shell">
    <header class="overlay-head" title="按住标题栏拖动窗口">
      <div class="overlay-title">
        <span class="brand-mark">P</span><strong>LeaguePingo</strong
        ><small>对局快览</small>
      </div>
      <div class="overlay-controls">
        <button
          :disabled="busy || !state"
          :aria-pressed="state?.pinned"
          @click="$emit('pin')"
        >
          {{ state?.pinned ? "取消置顶" : "置顶" }}</button
        ><button
          class="overlay-close"
          aria-label="关闭对局快览"
          :disabled="busy"
          @click="$emit('hide')"
        >
          ×
        </button>
      </div>
    </header>
    <div class="overlay-content">
      <p class="overlay-status">{{ status }}</p>
      <p v-if="state?.warning" class="error-banner">{{ state.warning }}</p>
      <p v-if="error" class="error-banner">{{ error }}</p>
      <p class="muted">
        身份 {{ snapshot?.knownCount ?? 0 }}/{{ players.length }} · 战绩
        {{ snapshot?.readyCount ?? 0 }}/{{ players.length }}
      </p>
      <div v-if="!players.length" class="empty-state">
        等待客户端提供本局名单
      </div>
      <div v-else class="overlay-groups">
        <section v-for="team in ['ally', 'enemy']" :key="team">
          <h2>
            {{
              team === "ally"
                ? snapshot?.stage === "party"
                  ? "房间成员"
                  : "我方"
                : snapshot?.stage === "party"
                  ? "其他玩家未确定"
                  : "对方"
            }}
          </h2>
          <div
            class="overlay-player"
            v-for="p in players.filter((p) => p.team === team)"
            :key="p.key"
          >
            <div class="overlay-player-heading">
              <GameIcon :id="p.championId" :size="28" /><strong
                :title="p.displayName"
                >{{ p.displayName }}</strong
              >
            </div>
            <small class="overlay-champion">{{
              championName(p.championId ?? 0)
            }}</small
            ><span>{{
              p.identityState === "unavailable"
                ? "身份不可获取"
                : p.historyState === "ready"
                  ? `${p.stats.games} 场 · 胜率 ${percent(p.stats.winRate)}`
                  : p.historyState === "empty"
                    ? "暂无战绩"
                    : preparationLabel(p)
            }}</span>
            <small v-if="p.historyState === 'ready'" class="overlay-kda"
              >KDA {{ decimal(p.stats.kda) }} · {{ preparationLabel(p) }}</small
            >
            <small v-if="p.historyState === 'ready'" class="overlay-recent"
              >{{ recentOutcomes(p) }}<br />{{ championSample(p) }}</small
            >
            <small v-if="p.historyMeta?.fetchedAt" class="overlay-time"
              >资料 {{ date(p.historyMeta.fetchedAt) }}</small
            >
            <small v-if="p.error" class="warning-text">{{ p.error }}</small>
          </div>
        </section>
      </div>
    </div>
    <footer class="overlay-footer">
      拖动标题栏移动 · × 隐藏 · 可取消置顶<br />
      {{ state?.shortcut || "Ctrl+Shift+P" }}
      {{
        state?.shortcutAvailable === false ? "未注册，请用主窗口按钮" : "显隐"
      }}
      · 获焦时 Esc 隐藏
    </footer>
  </main>
</template>
