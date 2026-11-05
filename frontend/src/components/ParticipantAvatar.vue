<script setup lang="ts">
import { uiText } from "../uiText";
import { computed, ref, watch } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import { gameAsset } from "../gameAssets";
import GameIcon from "./GameIcon.vue";
import AppIcon from "./AppIcon.vue";
const props = withDefaults(
  defineProps<{ player: Participant; size?: number; account?: boolean }>(),
  { size: 38 },
);
const broken = ref(false);
watch(
  [() => props.player.key, () => props.player.profile?.profileIconData],
  () => {
    broken.value = false;
  },
);
const hasChampion = computed(
  () => !props.account && !!gameAsset("champions", props.player.championId),
);
const status = computed(() =>
  props.account
    ? props.player.profile?.profileIconData && !broken.value
      ? uiText.participantAvatar.account
      : uiText.participantAvatar.missingAccount
    : props.player.championId
    ? uiText.participantAvatar.missingChampion(props.player.championId)
    : props.player.championState === "unselected"
      ? uiText.participantAvatar.unselected
      : uiText.participantAvatar.unknownChampion,
);
</script>
<template>
  <span
    class="participant-avatar"
    :style="{ width: `${size}px`, height: `${size}px` }"
  >
    <GameIcon
      v-if="hasChampion"
      :id="player.championId"
      :size="size"
      :title="
        player.championState === 'intent' ? uiText.participantAvatar.intent : undefined
      "
    />
    <span
      v-else
      class="participant-avatar-fallback"
      :title="status"
      role="img"
      :aria-label="status"
    >
      <img
        v-if="player.profile?.profileIconData && !broken"
        :src="player.profile.profileIconData"
        alt=""
        @error="broken = true"
      />
      <AppIcon v-else name="user" :size="Math.round(size * 0.6)" />
    </span>
  </span>
</template>
<style scoped>
.participant-avatar {
  display: inline-flex;
  flex: 0 0 auto;
}
.participant-avatar-fallback {
  width: 100%;
  height: 100%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 5px;
  overflow: hidden;
  background: var(--surface-2, #232b35);
  color: var(--muted, #9ba9ba);
}
img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
</style>
