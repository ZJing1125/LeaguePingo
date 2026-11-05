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
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.profile-avatar):is(.profile-avatar) :deep(img) {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: inherit;
}
:where(.player-profile):is(.player-profile) {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 22px;
  min-height: 80px;
  flex: none;
  border-bottom: 1px solid var(--border);
}
:where(.profile-avatar):is(.profile-avatar) {
  width: 52px;
  height: 52px;
  display: grid;
  place-items: center;
  background: #252f3a;
  border: 1px solid #384859;
  border-radius: 12px;
  color: #97b9ba;
  flex: none;
}
:where(.profile-identity):is(.profile-identity) {
  min-width: 120px;
  flex: 1;
}
:where(.identity-title):is(.identity-title) {
  display: flex;
  gap: 6px;
  align-items: center;
  min-width: 0;
}
:where(.identity-title):is(.identity-title) :deep(h1) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 19px;
  user-select: text;
}
:where(.identity-title):is(.identity-title) :deep(.square) {
  border-color: transparent;
  flex: none;
}
:where(.profile-meta):is(.profile-meta) {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 5px;
  color: var(--muted);
  font-size: 12px;
}
:where(.privacy-unknown):is(.privacy-unknown) {
  font-size: 11px;
}
:where(.profile-ranks):is(.profile-ranks) {
  display: flex;
  gap: 22px;
  margin-left: auto;
  flex: none;
}
:where(.rank-card):is(.rank-card) {
  display: flex;
  gap: 9px;
  align-items: center;
  min-width: 135px;
}
:where(.rank-card):is(.rank-card) :deep(small) {
  display: block;
  color: var(--muted);
  font-size: 11px;
}
:where(.rank-card):is(.rank-card) :deep(strong) {
  display: block;
  font-size: 13px;
  margin: 4px 0;
  white-space: nowrap;
}
:where(.rank-card):is(.rank-card) :deep(strong span) {
  font-size: 11px;
  font-weight: 400;
  color: var(--muted);
}
:where(.player-profile):is(.compact .player-profile) {
  padding: 12px;
  min-height: 74px;
  gap: 9px;
}
:where(.profile-avatar):is(.compact .profile-avatar) {
  width: 36px;
  height: 36px;
  border-radius: 8px;
}
:where(.identity-title):is(.compact .identity-title) :deep(h1) {
  font-size: 15px;
}
:where(.identity-title):is(.compact .identity-title) :deep(.square) {
  display: none;
}
:where(.profile-meta):is(.compact .profile-meta) {
  font-size: 11px;
}
:where(.privacy-unknown):is(.compact .privacy-unknown) {
  display: none;
}
@media (min-width: 1300px) {
  :where(.profile-ranks):is(.profile-ranks) {
    gap: 34px;
  }
}
@media (max-width: 1180px) {
  :where(.profile-ranks):is(.profile-ranks) {
    gap: 14px;
  }
}
@media (max-width: 1180px) {
  :where(.rank-card):is(.rank-card) {
    min-width: 110px;
  }
}
@media (max-width: 1000px) {
  :where(.profile-ranks):is(.profile-ranks) {
    gap: 12px;
  }
}
@media (max-width: 1000px) {
  :where(.rank-card):is(.rank-card) {
    min-width: 100px;
  }
}
@media (max-width: 1000px) {
  :where(.rank-card):is(.rank-card) :deep(strong) {
    font-size: 12px;
  }
}
@media (max-width: 1000px) {
  :where(.profile-identity):is(.profile-identity) {
    min-width: 100px;
  }
}
@media (max-width: 1000px) {
  :where(.identity-title):is(.identity-title) :deep(h1) {
    font-size: 17px;
  }
}
@media (max-width: 760px) {
  :where(.profile-ranks):is(.profile-ranks) {
    display: none;
  }
}
@media (max-width: 760px) {
  :where(.player-profile):is(.player-profile) {
    padding: 14px;
    min-height: 84px;
  }
}
:where(.player-profile):is(.player-drawer .compact>.player-profile) {
  grid-column: 1;
  grid-row: 1;
  display: flex;
  flex-wrap: wrap;
  align-content: center;
  align-items: center;
  padding: 10px 12px;
  gap: 8px;
  border: 0;
  border-right: 1px solid var(--border);
  min-height: 0;
}
:where(.profile-identity):is(.player-drawer .compact>.player-profile .profile-identity) {
  min-width: 0;
  flex-basis: 130px;
}
:where(.profile-avatar):is(.player-drawer .compact>.player-profile .profile-avatar) {
  width: 30px;
  height: 30px;
  border-radius: 3px;
}
:where(.identity-title):is(.player-drawer .identity-title) :deep(h1) {
  font-size: 12px;
}
:where(.profile-meta):is(.player-drawer .profile-meta) {
  font-size: 9px;
  gap: 3px;
}
:where(.player-profile):is(.drawer-floating .compact>.player-profile) {
  padding: 10px 14px;
  min-height: 60px;
  flex-wrap: nowrap;
}
:where(.profile-identity):is(.drawer-floating .compact>.player-profile .profile-identity) {
  flex: 1;
}
:where(.identity-title):is(.drawer-floating .identity-title) :deep(h1) {
  font-size: 16px;
}
:where(.profile-ranks):is(.profile-ranks) :deep(.rank-query) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  padding: 5px 8px;
  font-size: 10px;
  white-space: nowrap;
  align-self: center;
}
:where(.compact.player-profile):is(.compact.player-profile) {
  flex-wrap: wrap;
}
:where(.player-profile):is(.compact .player-profile) {
  flex-wrap: wrap;
}
:where(.profile-ranks):is(.compact .profile-ranks) {
  flex-basis: 100%;
  flex-wrap: wrap;
  gap: 8px;
  margin: 0;
}
:where(.profile-ranks):is(.player-drawer .compact .profile-ranks) {
  display: flex;
  flex-basis: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 5px;
  margin: 0;
  min-width: 0;
}
:where(.rank-card):is(.player-drawer .compact .rank-card) {
  min-width: 0;
}
:where(.rank-card):is(.player-drawer .compact .rank-card) :deep(small) {
  font-size: 8px;
}
:where(.rank-card):is(.player-drawer .compact .rank-card) :deep(strong) {
  font-size: 10px;
  margin: 2px 0;
}
:where(.player-profile):is(.drawer-floating .compact>.player-profile) {
  flex-wrap: wrap;
}
:where(.profile-ranks):is(.drawer-floating .compact .profile-ranks) {
  flex-basis: auto;
}
:where(.player-profile):is(.drawer-floating .compact>.player-profile) {
  grid-column: 1;
  grid-row: 1;
  display: flex;
  flex-wrap: wrap;
  align-content: flex-start;
  align-items: flex-start;
  padding: 20px 16px;
  gap: 10px;
  border-right: 1px solid var(--border);
}
:where(.profile-identity):is(.drawer-floating .compact>.player-profile .profile-identity) {
  flex-basis: 100%;
  min-width: 0;
}
:where(.identity-title):is(.drawer-floating .identity-title) :deep(h1) {
  font-size: 16px;
  white-space: normal;
  overflow-wrap: anywhere;
}
:where(.profile-meta):is(.drawer-floating .profile-meta) {
  font-size: 12px;
  line-height: 1.6;
  gap: 6px;
}
:where(.profile-ranks):is(.drawer-floating .compact .profile-ranks) {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  flex-basis: 100%;
  gap: 18px;
  margin-top: 10px;
}
:where(.rank-card):is(.drawer-floating .compact .rank-card) {
  min-width: 0;
}
:where(.rank-card):is(.drawer-floating .compact .rank-card) :deep(small) {
  font-size: 12px;
}
:where(.rank-card):is(.drawer-floating .compact .rank-card) :deep(strong) {
  font-size: 14px;
  white-space: normal;
}
:where(.player-profile):is(.players-workspace .analysis-layout:not(.compact)>.player-profile){display:flex;align-items:center;gap:12px;flex:none;padding:12px 16px;background:var(--panel);border:1px solid var(--border);border-radius:8px;min-width:0;color:var(--text);}
:where(.profile-avatar):is(.players-workspace .analysis-layout:not(.compact)>.player-profile .profile-avatar){width:48px;height:48px;flex:none;background:var(--raised);border:1px solid var(--border);}
:where(.profile-identity):is(.players-workspace .analysis-layout:not(.compact)>.player-profile .profile-identity){flex:1;min-width:0;order:0;}
:where(.identity-title):is(.players-workspace .analysis-layout:not(.compact) .identity-title) :deep(h1){font-size:21px;line-height:1.3;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;}
:where(.profile-meta):is(.players-workspace .analysis-layout:not(.compact) .profile-meta){font-size:12px;color:var(--muted);margin-top:5px;}
@media(max-width:940px){:where(.identity-title):is(.players-workspace .analysis-layout:not(.compact) .identity-title) :deep(h1){font-size:18px;}}
:where(.player-profile):is(.players-workspace .analysis-layout:not(.compact)>.player-profile){padding:8px 14px;min-height:66px;}
:where(.identity-title):is(.players-workspace .analysis-layout:not(.compact) .identity-title) :deep(.square){width:28px;height:28px;min-height:28px;padding:0;}
</style>
