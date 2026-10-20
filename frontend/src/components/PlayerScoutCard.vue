<script setup lang="ts">
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import { playerName } from "../presentation";
import ParticipantAvatar from "./ParticipantAvatar.vue";
import PlayerCardData from "./PlayerCardData.vue";
import AppIcon from "./AppIcon.vue";
defineProps<{ player: Participant; frozen?: boolean; party?: boolean }>();
</script>
<template>
  <span class="scout-identity card-portrait">
    <ParticipantAvatar class="card-avatar" :player="player" :size="34" account />
    <span class="card-identity">
      <strong :title="playerName(player)">{{ playerName(player) }}</strong>
      <span class="scout-level">{{ player.profile?.level != null ? `等级 ${player.profile.level}` : '等级暂缺' }}<small v-if="player.stale"> · 旧资料</small></span>
    </span>
    <AppIcon v-if="player.stale" class="scout-retained" name="history" :size="13" />
  </span>
  <PlayerCardData :player="player" :frozen="frozen" :party="party" />
</template>
