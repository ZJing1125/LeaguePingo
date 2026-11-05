<script setup lang="ts">
import { uiText } from "../uiText";
import { computed } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import PlayerScoutCard from "./PlayerScoutCard.vue";
import ExperimentalBadge from "./ExperimentalBadge.vue";

const props = defineProps<{
  players: Participant[];
  selectedKey?: string;
  party?: boolean;
  frozen?: boolean;
  queue?: number;
  captured?: boolean;
  alliesOnly?: boolean;
}>();
defineEmits<{ select: [player: Participant] }>();
const teams = computed(() =>
  (["ally", "enemy"] as const).filter(key => !props.alliesOnly || key === 'ally').map((key) => ({
    key,
    label: props.party
      ? key === "ally"
        ? uiText.common.party
        : uiText.matchRoster.noOpponent
      : key === "ally"
        ? uiText.common.ally
        : uiText.common.enemy,
    players: props.players.filter((player) => player.team === key),
  })),
);
</script>
<template>
  <div
    class="team-columns"
    :class="{
      'expanded-roster': teams.some((team) => team.players.length > 5),
      'party-roster': party,
    }"
    :aria-label="uiText.matchRoster.title"
  >
    <section
      v-for="team in teams"
      :key="team.key"
      class="team-section"
      :aria-label="team.label"
    >
      <h2>
        <span>{{ team.label }}</span
        ><small><ExperimentalBadge v-if="team.players.some(p => p.premadeGroup && (p.premadeSize || 0) > 1)" :feature="uiText.clientSidebar.groups" :label="uiText.clientSidebar.groupExperimental"/>{{ uiText.matchRoster.members(team.players.length) }}</small>
      </h2>
      <div class="team-player-list">
        <button
          v-for="player in team.players"
          :key="player.key"
          class="player-row hero-card scout-card"
          :class="{ selected: selectedKey === player.key, 'scout-enemy': team.key === 'enemy' }"
          :aria-pressed="selectedKey === player.key"
          :data-player-key="player.key"
          @click="$emit('select', player)"
        >
          <PlayerScoutCard :player="player" :frozen="frozen" :party="party" :queue="queue" :captured="captured" />
        </button>
      </div>
      <p v-if="!team.players.length" class="team-empty">
        {{
          party && team.key === "enemy"
            ? uiText.matchRoster.partyOpponent
            : uiText.matchRoster.waitingRoster
        }}
      </p>
    </section>
  </div>
</template>
