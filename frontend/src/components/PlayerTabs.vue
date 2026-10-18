<script setup lang="ts">
import { nextTick, ref, watch } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import { playerTabId } from "../playerState";
import { playerName } from "../presentation";

const props = defineProps<{ players: Participant[]; activeKey: string }>();
const emit = defineEmits<{ select: [key: string]; close: [key: string] }>();
const tablist = ref<HTMLElement | null>(null);

function revealActive() {
  tablist.value
    ?.querySelector<HTMLElement>('[role="tab"][aria-selected="true"]')
    ?.scrollIntoView({ block: "nearest", inline: "nearest" });
}
watch(
  () => [props.activeKey, props.players.length],
  async () => {
    await nextTick();
    revealActive();
  },
);

async function focusActive() {
  await nextTick();
  tablist.value
    ?.querySelector<HTMLElement>('[role="tab"][aria-selected="true"]')
    ?.focus({ preventScroll: true });
  revealActive();
}
async function close(key: string) {
  const wasLast = props.players.length === 1;
  emit("close", key);
  await focusActive();
  // Closing the last tab leaves a useful keyboard destination.
  if (wasLast)
    document.querySelector<HTMLInputElement>(".global-search input")?.focus();
}
function navigate(event: KeyboardEvent, index: number) {
  const last = props.players.length - 1;
  let target: number;
  switch (event.key) {
    case "ArrowRight":
      target = (index + 1) % props.players.length;
      break;
    case "ArrowLeft":
      target = (index + last) % props.players.length;
      break;
    case "Home":
      target = 0;
      break;
    case "End":
      target = last;
      break;
    case "Delete":
      event.preventDefault();
      void close(props.players[index]!.key);
      return;
    default:
      return;
  }
  event.preventDefault();
  emit("select", props.players[target]!.key);
  void focusActive();
}
</script>

<template>
  <div
    ref="tablist"
    class="player-tabs"
    role="tablist"
    aria-label="已查看玩家"
    aria-orientation="horizontal"
  >
    <div
      v-for="(player, index) in players"
      :key="player.key"
      class="player-tab"
      :class="{ active: activeKey === player.key }"
    >
      <button
        :id="playerTabId(player.key)"
        role="tab"
        :aria-selected="activeKey === player.key"
        aria-controls="player-panel"
        :tabindex="activeKey === player.key ? 0 : -1"
        :title="playerName(player, index + 1)"
        @click="emit('select', player.key)"
        @keydown="navigate($event, index)"
      >
        {{ playerName(player, index + 1) }}
      </button>
      <button
        class="tab-close"
        :aria-label="`关闭 ${playerName(player, index + 1)}`"
        :tabindex="activeKey === player.key ? 0 : -1"
        @click="close(player.key)"
      >
        ×
      </button>
    </div>
  </div>
</template>
