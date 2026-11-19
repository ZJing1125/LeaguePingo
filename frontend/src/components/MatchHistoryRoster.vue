<script setup lang="ts">
import { computed, inject, ref } from "vue";
import { historyRoster, peerShortName } from "../historyMatch";
import { historyTarget, playerHistoryNavigation } from "../playerHistoryNavigation";
import PlayerHistoryAction from "./PlayerHistoryAction.vue";
import { uiText } from "../uiText";
import GameIcon from "./GameIcon.vue";

const props = defineProps<{ groups: ReturnType<typeof historyRoster>; self?: boolean; detail?: boolean; connectionId?: number }>();

const navigation = inject(playerHistoryNavigation, null);
const focusedPeer = ref<number>();
const availablePeers = computed(() => props.groups.flatMap(team => team.peers)
  .filter(peer => navigation?.available(historyTarget(peer.displayName, props.connectionId)))
  .map(peer => peer.participantId));
const tabStop = computed(() => {
  const available = availablePeers.value;
  return focusedPeer.value != null && available.includes(focusedPeer.value) ? focusedPeer.value : available[0];
});

function moveWithinRoster(event: KeyboardEvent) {
  const next = ["ArrowRight", "ArrowDown"].includes(event.key);
  const previous = ["ArrowLeft", "ArrowUp"].includes(event.key);
  if (!next && !previous && event.key !== "Home" && event.key !== "End") return;
  const root = event.currentTarget as HTMLElement;
  const buttons = Array.from(root.querySelectorAll<HTMLButtonElement>("button:not(:disabled)"));
  const current = buttons.indexOf(event.target as HTMLButtonElement);
  if (current === -1) return;
  const index = event.key === "Home" ? 0 : event.key === "End" ? buttons.length - 1
    : (current + (next ? 1 : -1) + buttons.length) % buttons.length;
  event.preventDefault();
  buttons[index]?.focus();
}
</script>

<template>
  <span class="match-mini-roster" :class="{ 'roster-names': detail }" :aria-label="uiText.common.roster" :role="availablePeers.length ? 'toolbar' : 'group'" @keydown="moveWithinRoster">
    <span v-for="team in groups" :key="team.teamId" :aria-label="team.opposing ? uiText.matchDetails.opposingSide : uiText.matchDetails.targetSide" class="roster-team" :class="{ 'roster-red': team.teamId === 200 }">
      <b v-if="detail" class="roster-heading">{{ team.opposing ? uiText.matchDetails.opposingSide : uiText.matchDetails.targetSide }} <small>{{ team.teamId === 100 ? uiText.common.blue : uiText.common.red }}</small></b>
      <span v-if="!team.peers.length" class="roster-empty">{{ uiText.matchHistoryRow.missingRoster }}</span>
      <PlayerHistoryAction v-for="peer in team.peers" :key="peer.participantId" class="roster-peer" :class="{ 'roster-target': peer.target }"
        :target="historyTarget(peer.displayName, connectionId)"
        :tabindex="availablePeers.includes(peer.participantId) ? peer.participantId === tabStop ? 0 : -1 : undefined"
        @focus="focusedPeer = peer.participantId">
        <GameIcon :id="peer.championId" :size="detail ? 22 : 14" />
        <span class="roster-label">{{ peerShortName(peer) }}</span>
        <small v-if="detail && peer.target" class="roster-focus">{{ self ? uiText.matchDetails.self : uiText.matchDetails.lookedUp }}</small>
      </PlayerHistoryAction>
    </span>
  </span>
</template>

<style scoped>
.match-mini-roster { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:8px; text-align:left; min-width:0; }
.roster-team { display:grid; align-content:start; gap:0; min-width:0; border-left:2px solid var(--ally); padding-left:5px; }
.roster-red { border-left-color:var(--enemy); }
.roster-peer { display:flex; align-items:center; gap:4px; min-width:0; color:var(--muted); font-size:11px; line-height:14px; }
.roster-label { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.roster-target { color:var(--text); font-weight:700; }
.roster-target :deep(.game-icon) { outline:1px solid var(--accent); }
.roster-heading { font-size:12px; margin-bottom:5px; }
.roster-heading small, .roster-empty { font-size:11px; color:var(--muted); }
.roster-focus { color:var(--accent); font-size:11px; flex:none; }
.roster-names { max-width:440px; gap:16px; }
.roster-names .roster-peer { font-size:12px; line-height:18px; gap:6px; padding-block:2px; }
@container (max-width:755px) {
  .match-mini-roster:not(.roster-names) { grid-template-columns:1fr; gap:5px; }
  .match-mini-roster:not(.roster-names) .roster-team { display:flex; flex-wrap:wrap; gap:3px; }
  .match-mini-roster:not(.roster-names) .roster-label { position:absolute; width:1px; height:1px; overflow:hidden; clip-path:inset(50%); white-space:nowrap; }
}
</style>
