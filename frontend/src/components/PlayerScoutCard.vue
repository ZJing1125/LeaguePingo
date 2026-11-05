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
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.scout-identity.card-portrait):is(.scout-card .scout-identity.card-portrait) {
  position: relative;
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 48px;
  min-height: 48px;
  max-height: 48px;
  padding: 6px 9px;
  background: var(--scout-head);
  border-bottom: 2px solid var(--scout-team);
  flex: none;
}
:where(.card-avatar):is(.scout-card .scout-identity .card-avatar) { position: static; flex: none; overflow: hidden; border-radius: 4px; opacity: 1; }
:where(.card-identity):is(.scout-card .scout-identity .card-identity) { display: flex; flex-direction: column; gap: 3px; width: auto; flex: 1; min-width: 0; padding: 0; text-shadow: none; }
:where(.card-identity):is(.scout-card .scout-identity .card-identity) :deep(>strong) { font-size: 13px; line-height: 15px; color: var(--scout-text); font-weight: 650; text-overflow: ellipsis; overflow: hidden; white-space: nowrap; }
:where(.scout-level):is(.scout-card .scout-identity .card-identity>.scout-level) { font-size: 11px; line-height: 13px; color: var(--scout-muted); }
:where(.scout-level):is(.scout-level) :deep(small) { color: var(--scout-warning); font-size: 11px; }
:where(.scout-retained):is(.scout-retained) { flex: none; color: var(--scout-warning); }
:where(.scout-identity.card-portrait):is(.overlay-shell .scout-card .scout-identity.card-portrait) { height: 40px; min-height: 40px; max-height: 40px; padding: 4px 6px; gap: 6px; }
:where(.card-identity):is(.overlay-shell .scout-card .scout-identity .card-identity) :deep(>strong) { font-size: 11px; }
:where(.scout-level):is(.overlay-shell .scout-card .scout-identity .card-identity>.scout-level) { font-size: 11px; }
:where(.scout-level):is(.scout-card .scout-level) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
:where(.history-preparation):is(.scout-card .history-preparation) { font-size: 11px; color: var(--scout-muted); }
:where(.card-portrait):is(.card-portrait) {
  position: relative;
  width: 100%;
  height: 32%;
  min-height: 62px;
  max-height: 100px;
  flex: none;
  display: flex;
  align-items: flex-end;
  z-index: 2;
  background: transparent;
}
:where(.card-identity):is(.card-identity) {
  width: 100%;
  min-width: 0;
  padding: 15px 12px 7px;
  display: flex;
  flex-direction: column;
  gap: 3px;
  text-shadow: 0 1px 4px #0008;
}
:where(.card-identity):is(.card-identity) :deep(>strong) {
  font-size: 17px;
  font-weight: 600;
  line-height: 1.2;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
:where(.card-identity):is(.card-identity) :deep(>span) {
  font-size: 10px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #ccd9d3;
}
:where(.card-portrait):is(.overlay-player .card-portrait) {
  height: 55px;
  min-height: 55px;
}
:where(.card-identity):is(.overlay-player .card-identity) {
  padding: 12px 9px 5px;
}
:where(.card-identity):is(.overlay-player .card-identity) :deep(>strong) {
  font-size: 15px;
}
:where(.card-identity):is(.overlay-player .card-identity) :deep(>span) {
  font-size: 9px;
}
@media (max-width: 1150px) {
  :where(.card-identity):is(.card-identity) {
    padding-inline: 9px;
  }

}
@media (max-width: 999px) {
  :where(.card-identity):is(.card-identity) :deep(>strong) {
    font-size: 16px;
  }
}
@media (max-width: 999px) {
  :where(.card-identity):is(.overlay-player .card-identity) :deep(>strong) {
    font-size: 13px;
  }
}
@media (max-width: 680px) {
  :where(.card-portrait):is(.overlay-player .card-portrait) {
    height: 31px;
    min-height: 31px;
  }
}
@media (max-width: 680px) {
  :where(.card-identity):is(.overlay-player .card-identity) {
    padding: 3px 6px;
  }
}
@media (max-width: 680px) {
  :where(.card-identity):is(.overlay-player .card-identity) :deep(>strong) {
    font-size: 11px;
  }
}
@media (max-width: 680px) {
  :where(.card-identity):is(.overlay-player .card-identity) :deep(>span) {
    font-size: 9px;
  }
}
:where(.card-avatar):is(.card-portrait .card-avatar) {
  position: absolute;
  right: 8px;
  top: 7px;
  opacity: 0.85;
  z-index: 2;
}
:where(.card-identity):is(.card-portrait .card-identity) {
  padding-right: 34px;
}
:where(.card-identity):is(.overlay-player .card-identity) {
  padding-right: 9px;
}
:where(.card-portrait):is(.match-workspace .card-portrait) {
  height: 68px;
  min-height: 68px;
  max-height: none;
  align-items: center;
  gap: 8px;
  padding: 10px 11px 5px;
}
:where(.card-avatar):is(.match-workspace .card-portrait .card-avatar) {
  position: static;
  opacity: 1;
  border-radius: 7px;
  overflow: hidden;
}
:where(.card-identity):is(.match-workspace .card-identity) {
  padding: 0;
  gap: 5px;
}
:where(.card-identity):is(.match-workspace .card-identity) :deep(>strong) {
  font-size: 14px;
  font-weight: 600;
  line-height: 20px;
}
</style>
