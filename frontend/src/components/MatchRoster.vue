<script setup lang="ts">
import { computed } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import { playerName } from "../presentation";
import { championName } from "../gameAssets";
import PlayerCardData from "./PlayerCardData.vue";
import ParticipantAvatar from "./ParticipantAvatar.vue";
import PlayerCardArt from "./PlayerCardArt.vue";
import GameIcon from "./GameIcon.vue";

const props = defineProps<{
  players: Participant[];
  selectedKey?: string;
  party?: boolean;
  frozen?: boolean;
}>();
defineEmits<{ select: [player: Participant] }>();
const teams = computed(() =>
  (["ally", "enemy"] as const).map((key) => ({
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
          class="player-row hero-card"
          :class="{ selected: selectedKey === player.key }"
          :aria-pressed="selectedKey === player.key"
          :data-player-key="player.key"
          @click="$emit('select', player)"
        >
          <PlayerCardArt :id="player.championId" />
          <span class="card-portrait">
            <ParticipantAvatar
              class="card-avatar"
              :player="player"
              :size="36"
              account
            />
            <span class="card-identity"
              ><strong :title="playerName(player)">{{
                playerName(player)
              }}</strong>
              <span class="card-pick"
                ><GameIcon
                  v-if="player.championId"
                  :id="player.championId"
                  :size="24"
                />{{
                  player.championId
                    ? championName(player.championId).split("·")[1] ||
                      championName(player.championId)
                    : party
                      ? player.profile?.level != null
                        ? `等级 ${player.profile.level}`
                        : "房间成员"
                      : player.championState === "unselected"
                        ? "尚未选择英雄"
                        : "英雄待确认"
                }}</span
              ></span
            >
            <span v-if="player.championState === 'intent'" class="card-intent"
              >预选英雄</span
            >
          </span>
          <PlayerCardData :player="player" :frozen="frozen" />
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
