<script setup lang="ts">
import { uiText } from "../uiText";
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
    :aria-label="uiText.playerTabs.title"
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
        :aria-label="uiText.playerTabs.close(playerName(player, index + 1))"
        :tabindex="activeKey === player.key ? 0 : -1"
        @click="close(player.key)"
      >
        ×
      </button>
    </div>
  </div>
</template>

<style scoped>
/* Component-owned rules, retaining foundation/style cascade order. */
.player-tabs {
  height: 38px;
  flex: none;
  display: flex;
  gap: 4px;
  border-bottom: 1px solid var(--border);
  padding: 4px 20px 0;
  overflow-x: auto;
  overflow-y: hidden;
}
.player-tabs::-webkit-scrollbar {
  height: 4px;
}
.player-tabs::-webkit-scrollbar-thumb {
  background: var(--border);
  border-radius: 4px;
}
.player-tab {
  display: flex;
  max-width: 220px;
  flex: none;
  border-bottom: 2px solid transparent;
  color: var(--muted);
}
.player-tab.active {
  border-bottom-color: var(--accent);
  color: var(--text);
  background: #1a2029;
  border-radius: 4px 4px 0 0;
}
.player-tab button {
  border: 0;
  background: none;
  padding: 6px 10px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
}
.player-tab .tab-close {
  color: var(--muted);
  padding: 4px 7px;
}
@media (max-width: 760px) {
  .player-tabs {
    padding-left: 12px;
  }
}
.player-tabs {
  background: var(--raised);
  border-color: var(--border);
}
.player-tab {
  background: transparent;
}
.player-tab.active {
  background: var(--surface);
  border-color: var(--accent-art);
}
.player-tab.active {
  border-bottom: 2px solid var(--accent-art);
}
</style>
