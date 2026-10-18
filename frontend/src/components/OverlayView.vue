<script setup lang="ts">
import { computed } from "vue";
import type { Snapshot } from "../../bindings/leaguepingo/internal/core/models";
import type { OverlayState } from "../../bindings/leaguepingo/models";
import {
  percent,
  decimal,
  preparationLabel,
  championSample,
  date,
  playerName,
} from "../presentation";
import AppIcon from "./AppIcon.vue";
import ParticipantAvatar from "./ParticipantAvatar.vue";
import RecentForm from "./RecentForm.vue";
const props = defineProps<{
  snapshot: Snapshot | null;
  state: OverlayState | null;
  status: string;
  busy: boolean;
  error: string;
}>();
const players = computed(() => props.snapshot?.participants ?? []);
const newest = computed(() =>
  players.value
    .map((p) => p.historyMeta?.fetchedAt)
    .filter(Boolean)
    .sort()
    .at(-1),
);
defineEmits<{ hide: []; pin: [] }>();
</script>
<template>
  <main class="overlay-shell">
    <header class="overlay-head">
      <div class="overlay-title">
        <span class="brand-mark">P</span>
        <div><strong>LeaguePingo</strong><small>对局快览</small></div>
      </div>
      <div class="overlay-controls">
        <button
          :disabled="busy || !state"
          :aria-label="state?.pinned ? '取消置顶' : '置顶'"
          :title="state?.pinned ? '取消置顶' : '置顶'"
          :aria-pressed="state?.pinned"
          @click="$emit('pin')"
        >
          <AppIcon name="pin" :size="16" /></button
        ><button
          class="overlay-close"
          aria-label="关闭对局快览"
          :disabled="busy"
          @click="$emit('hide')"
        >
          <AppIcon name="close" :size="18" />
        </button>
      </div>
    </header>
    <div class="overlay-content">
      <div class="overlay-status">
        <span
          class="connection-light"
          :class="{ online: snapshot?.connected }"
        ></span
        >{{ status }}
      </div>
      <p v-if="state?.warning || error" class="error-banner" role="status">
        {{ error || state?.warning }}
      </p>
      <div class="overlay-readiness">
        <span
          >已识别
          <b>{{ snapshot?.knownCount ?? 0 }} / {{ players.length }}</b></span
        ><span
          >战绩就绪
          <b>{{ snapshot?.readyCount ?? 0 }} / {{ players.length }}</b></span
        >
      </div>
      <div v-if="!players.length" class="empty-state">
        <AppIcon name="match" :size="30" />
        <p>等待本局名单</p>
      </div>
      <div v-else class="overlay-groups">
        <section v-for="team in ['ally', 'enemy']" :key="team">
          <h2>
            {{
              snapshot?.stage === "party"
                ? team === "ally"
                  ? "房间成员"
                  : "对手未确定"
                : team === "ally"
                  ? "我方"
                  : "对方"
            }}<span>{{ players.filter((p) => p.team === team).length }}</span>
          </h2>
          <div
            v-for="p in players.filter((p) => p.team === team)"
            :key="p.key"
            class="overlay-player"
            :class="{
              unavailable: p.identityState === 'unavailable',
              outdated: p.stale,
            }"
          >
            <div class="overlay-player-heading">
              <ParticipantAvatar :player="p" :size="28" /><strong
                :title="`${playerName(p)} · ${championSample(p)} · ${date(p.historyMeta?.fetchedAt)}`"
                >{{ playerName(p) }}</strong
              ><small
                v-if="p.historyState === 'ready'"
                class="overlay-count"
                :title="`已加载 ${p.stats.games} 场；胜负有效 ${p.stats.outcomeGames} 场，KDA 有效 ${p.stats.kdaGames} 场`"
                >{{ p.stats.games }}场</small
              ><span v-if="p.stale" class="status-badge warning">旧</span>
            </div>
            <small v-if="p.championState === 'intent'" class="muted"
              >预选英雄</small
            >
            <small v-if="!p.championId" class="muted">{{
              p.championState === "unselected"
                ? "暂未选择英雄"
                : "英雄信息暂不可用"
            }}</small>
            <div v-if="p.historyState === 'ready'" class="overlay-data">
              <div class="overlay-metrics">
                <span :title="`${p.stats.games} 场样本胜率`"
                  ><small>胜率</small
                  ><b>{{ percent(p.stats.winRate) }}</b></span
                ><span :title="'汇总 KDA'"
                  ><small>KDA</small><b>{{ decimal(p.stats.kda) }}</b></span
                >
              </div>
              <RecentForm :matches="p.history" />
            </div>
            <p v-else class="overlay-pending">
              {{ preparationLabel(p, snapshot?.frozen) }}
            </p>
            <small v-if="p.error" class="warning-text" :title="p.error">{{
              p.error
            }}</small>
          </div>
        </section>
      </div>
    </div>
    <footer class="overlay-footer">
      <span
        >{{ state?.shortcut || "Ctrl+Shift+P" }}
        {{ state?.shortcutAvailable === false ? "未注册" : "显隐" }} · Esc
        隐藏</span
      ><span :title="'各玩家获取时间以主窗口为准'"
        >最近资料 {{ date(newest) }}</span
      >
    </footer>
  </main>
</template>
