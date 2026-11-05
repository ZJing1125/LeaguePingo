<script setup lang="ts">
import { uiText } from "../uiText";
import { computed, ref, watch } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import { date, playerName } from "../presentation";
import { rankPresentation } from "../rankPresentation";
import { hasPlayerName } from "../playerState";
import AppIcon from "./AppIcon.vue";
import RankEmblem from "./RankEmblem.vue";
const props = defineProps<{
  player: Participant;
  busy?: boolean;
  connected?: boolean;
  compact?: boolean;
  hideRanks?: boolean;
}>();
const rankCards = computed(() =>
  ["RANKED_SOLO_5x5", "RANKED_FLEX_SR"].map((queue) => ({
    queue,
    ...rankPresentation(props.player, queue),
  })),
);
const copyNotice = ref("");
const avatarBroken = ref(false);
watch(
  [() => props.player.key, () => props.player.profile?.profileIconData],
  () => {
    avatarBroken.value = false;
  },
);
function fieldTitle(field: string) {
  const meta = props.player.profile?.fields?.[field];
  return `${meta?.state === "retained" ? uiText.playerProfileHeader.staleWarning : uiText.playerProfileHeader.fetchedAt} · ${date(meta?.fetchedAt)}`;
}
async function copyID() {
  try {
    await navigator.clipboard.writeText(props.player.displayName);
    copyNotice.value = uiText.playerProfileHeader.copied;
  } catch {
    copyNotice.value = uiText.playerProfileHeader.copyFailed;
  }
}
</script>
<template>
  <header class="player-profile">
    <div
      class="profile-avatar"
      :title="
        player.profile?.profileIconData
          ? fieldTitle('avatar')
          : uiText.playerProfileHeader.missingAvatar
      "
    >
      <img
        v-if="player.profile?.profileIconData && !avatarBroken"
        :src="player.profile.profileIconData"
        :alt="uiText.playerProfileHeader.avatar"
        @error="avatarBroken = true"
      />
      <AppIcon v-else name="user" :size="26" />
    </div>
    <div class="profile-identity">
      <div class="identity-title">
        <h1 :title="playerName(player)">{{ playerName(player) }}</h1>
        <button
          class="icon-button square"
          :aria-label="uiText.playerProfileHeader.copyLabel"
          :title="uiText.playerProfileHeader.copy"
          :disabled="
            !hasPlayerName(player) || !player.displayName.includes('#')
          "
          @click="copyID"
        >
          <AppIcon name="copy" :size="15" />
        </button>
      </div>
      <div class="profile-meta">
        <span :title="fieldTitle('level')"
          >{{
            player.profile?.level != null
              ? uiText.playerProfileHeader.level(player.profile.level)
              : uiText.playerProfileHeader.missingLevel
          }}<small
            v-if="player.profile?.fields?.level?.state === 'retained'"
            class="warning-text"
          >{{ uiText.playerProfileHeader.old }}</small
          ></span
        ><span v-if="player.stale" class="status-badge warning">{{ uiText.playerProfileHeader.stale }}</span
        ><span v-else-if="player.cached">{{ uiText.playerProfileHeader.cached }}</span
        ><span
          v-if="player.privacyState === 'private'"
          class="status-badge"
          :title="fieldTitle('privacy')"
          >{{ uiText.common.privateCareer }}<small
            v-if="player.profile?.fields?.privacy?.state === 'retained'"
            class="warning-text"
          >{{ uiText.playerProfileHeader.old }}</small
          ></span
        ><span
          v-else-if="player.privacyState === 'unknown'"
          class="privacy-unknown"
          >{{ uiText.playerProfileHeader.uncheckedPrivacy }}</span
        >
        <span
          v-if="player.privacyState === 'public'"
          :title="fieldTitle('privacy')"
          >{{ uiText.playerProfileHeader.public }}<small
            v-if="player.profile?.fields?.privacy?.state === 'retained'"
            class="warning-text"
          >{{ uiText.playerProfileHeader.old }}</small
          ></span
        >
      </div>
    </div>
    <div v-if="!hideRanks" class="profile-ranks">
      <div
        v-for="item in rankCards"
        :key="item.queue"
        class="rank-card"
        :title="item.title"
      >
        <RankEmblem :tier="item.rank?.tier" />
        <div>
          <small>{{
            item.queue === "RANKED_SOLO_5x5" ? uiText.common.solo : uiText.common.flex
          }}</small>
          <strong :class="{ muted: !item.rank }"
            >{{ item.label
            }}<small v-if="item.retained" class="warning-text"
              >{{ uiText.playerProfileHeader.staleRank }}</small
            ></strong
          >
          <small v-if="item.rank && !compact" :title="item.record?.title"
            >{{ item.record ? uiText.playerProfileHeader.outcomes(item.record.scope, item.record.wins, item.record.losses) : uiText.common.pendingOutcome }}</small
          >
        </div>
      </div>
    </div>
  </header>
  <p v-if="copyNotice" class="inline-status" role="status">
    {{ copyNotice }}
  </p>
</template>
