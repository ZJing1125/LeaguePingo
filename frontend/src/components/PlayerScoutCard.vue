<script setup lang="ts">
import { uiText } from "../uiText";
import PremadeBadge from "./PremadeBadge.vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import { playerName, preparationLabel } from "../presentation";
import ParticipantAvatar from "./ParticipantAvatar.vue";
import PlayerCardData from "./PlayerCardData.vue";
import AppIcon from "./AppIcon.vue";
defineProps<{ player: Participant; frozen?: boolean; party?: boolean; queue?: number; captured?: boolean }>();
</script>
<template>
  <span class="scout-identity card-portrait">
    <ParticipantAvatar class="card-avatar" :player="player" :size="34" account />
    <span class="card-identity">
      <strong :title="playerName(player)">{{ playerName(player) }}</strong>
      <span class="scout-level">{{ player.profile?.level != null ? uiText.playerProfileHeader.level(player.profile.level) : uiText.playerScoutCard.missingLevel }} <PremadeBadge :player="player"/><small v-if="player.stale">{{ uiText.homeView.stale }}</small><small v-if="player.identityState === 'known' && ['loading', 'pending', 'failed'].includes(player.historyState)" class="history-preparation" :title="uiText.playerScoutCard.recentHistory(preparationLabel(player, frozen))"> · <span>{{ player.historyState === 'loading' ? uiText.playerScoutCard.loading : preparationLabel(player, frozen) }}</span></small></span>
    </span>
    <AppIcon v-if="player.stale" class="scout-retained" name="history" :size="13" />
  </span>
  <PlayerCardData :player="player" :frozen="frozen" :party="party" :queue="queue" :captured="captured" />
</template>
