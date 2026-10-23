<script setup lang="ts">
import { computed } from "vue";
import type { Snapshot } from "../../bindings/leaguepingo/internal/core/models";
import type { OverlayState } from "../../bindings/leaguepingo/models";
import { date } from "../presentation";
import AppIcon from "./AppIcon.vue";
import BrandIcon from "./BrandIcon.vue";
import PlayerScoutCard from "./PlayerScoutCard.vue";
import { rankedMode } from "../scoutStats";
import { scoutStatus } from "../scoutStatus";
import RosterProgress from "./RosterProgress.vue";
const props = defineProps<{
  snapshot: Snapshot | null;
  state: OverlayState | null;
  status: string;
  busy: boolean;
  error: string;
}>();
const players = computed(() => props.snapshot?.participants ?? []);
const reports = computed(() => scoutStatus(players.value, props.snapshot?.scoutQueue ?? 420));
const reportTime = computed(() => {
  const p = reports.value;
  if (!p.earliest) return "统计时间待确认";
  const first = date(p.earliest), last = date(p.latest);
  return `统计截至 ${first}${last !== first ? `–${last}` : ''}${p.untimed ? ' · 部分待确认' : ''}`;
});
defineEmits<{ hide: []; pin: [] }>();
</script>
<template>
  <main class="overlay-shell" :class="{ 'hold-mode': ['hold','preview'].includes(state?.mode || ''), 'preview-mode':state?.mode==='preview' }">
    <header class="overlay-head">
      <div class="overlay-title">
        <BrandIcon />
        <div><strong>LeaguePingo</strong><small>对局快览</small></div>
      </div>
      <div v-if="!['hold','preview'].includes(state?.mode || '')" class="overlay-controls">
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
        >{{ status }} · {{ rankedMode(snapshot?.scoutQueue ?? 420) }}
      </div>
      <p v-if="state?.warning || error" class="error-banner" role="status">
        {{ error || state?.warning }}
      </p>
      <RosterProgress v-if="players.length" :players="players" :queue="snapshot?.scoutQueue ?? 420" compact />
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
            class="overlay-player scout-card"
            :class="{
              'scout-enemy': team === 'enemy',
              unavailable: p.identityState === 'unavailable',
              outdated: p.stale,
            }"
          >
            <PlayerScoutCard :player="p" :frozen="snapshot?.frozen" :party="snapshot?.stage === 'party'" :queue="snapshot?.scoutQueue ?? 420" />
          </div>
        </section>
      </div>
    </div>
    <footer class="overlay-footer">
      <span v-if="state?.mode==='preview'">{{ snapshot?.phase==='GameStart' ? '载入快览' : '开局快览' }} · 6 秒后收起 · {{ state?.holdEnabled && state?.holdAvailable ? `按住 ${state?.holdKey==='Backquote'?'~':state?.holdKey || '~'} 随时查看` : state?.shortcutAvailable ? `${state.shortcut} 调出` : '可在主程序调出' }}</span>
      <span v-else-if="state?.mode === 'hold'"
        >松开 {{ state?.holdKey === "Backquote" ? "~" : state?.holdKey }} 隐藏 ·
        鼠标操作传给游戏</span
      ><span v-else
        >{{ state?.shortcut || "Ctrl+Shift+P" }}
        {{ state?.shortcutAvailable === false ? "未注册" : "显隐" }} · Esc
        隐藏</span
      ><span :title="'独立排位统计的截止时间；各玩家段位以其资料时间为准'"
        >{{ reportTime }}</span
      >
    </footer>
    <div v-if="state?.mode==='preview'" class="preview-timer" aria-hidden="true"><i/></div>
  </main>
</template>
