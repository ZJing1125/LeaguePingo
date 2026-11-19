<script setup lang="ts">
import { historyHeroName, historyRoster, peerShortName } from "../historyMatch";
import { uiText } from "../uiText";
import GameIcon from "./GameIcon.vue";

defineProps<{ groups: ReturnType<typeof historyRoster>; self?: boolean; detail?: boolean }>();
</script>

<template>
  <span class="match-mini-roster" :class="{ 'roster-names': detail }" :aria-label="uiText.common.roster">
    <span v-for="team in groups" :key="team.teamId" :aria-label="team.opposing ? uiText.matchDetails.opposingSide : uiText.matchDetails.targetSide" class="roster-team" :class="{ 'roster-red': team.teamId === 200 }">
      <b v-if="detail" class="roster-heading">{{ team.opposing ? uiText.matchDetails.opposingSide : uiText.matchDetails.targetSide }} <small>{{ team.teamId === 100 ? uiText.common.blue : uiText.common.red }}</small></b>
      <span v-if="!team.peers.length" class="roster-empty">{{ uiText.matchHistoryRow.missingRoster }}</span>
      <span v-for="peer in team.peers" :key="peer.participantId" class="roster-peer" :class="{ 'roster-target': peer.target }"
        :title="[peer.displayName, historyHeroName(peer.championId), peer.target ? self ? uiText.matchDetails.self : uiText.matchDetails.lookedUp : ''].filter(Boolean).join(' · ')">
        <GameIcon :id="peer.championId" :size="detail ? 22 : 14" />
        <span class="roster-label">{{ peerShortName(peer) }}</span>
        <small v-if="detail && peer.target" class="roster-focus">{{ self ? uiText.matchDetails.self : uiText.matchDetails.lookedUp }}</small>
      </span>
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
