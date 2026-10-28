<script setup lang="ts">
import { computed } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import PlayerScoutCard from "./PlayerScoutCard.vue";

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
        ? "房间成员"
        : "对手未确定"
      : key === "ally"
        ? "我方"
        : "对方",
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
    aria-label="双方名单"
  >
    <section
      v-for="team in teams"
      :key="team.key"
      class="team-section"
      :aria-label="team.label"
    >
      <h2>
        <span>{{ team.label }}</span
        ><small>{{ team.players.length }} 人</small>
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
            ? "组队阶段尚未确定对手"
            : "等待客户端提供名单"
        }}
      </p>
    </section>
  </div>
</template>
