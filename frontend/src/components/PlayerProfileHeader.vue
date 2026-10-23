<script setup lang="ts">
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
  return `${meta?.state === "retained" ? "本次未更新，保留旧值" : "资料获取时间"} · ${date(meta?.fetchedAt)}`;
}
async function copyID() {
  try {
    await navigator.clipboard.writeText(props.player.displayName);
    copyNotice.value = "已复制完整 ID";
  } catch {
    copyNotice.value = "复制未成功，可选中名称手动复制";
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
          : '玩家头像尚未获取'
      "
    >
      <img
        v-if="player.profile?.profileIconData && !avatarBroken"
        :src="player.profile.profileIconData"
        alt="玩家头像"
        @error="avatarBroken = true"
      />
      <AppIcon v-else name="user" :size="26" />
    </div>
    <div class="profile-identity">
      <div class="identity-title">
        <h1 :title="playerName(player)">{{ playerName(player) }}</h1>
        <button
          class="icon-button square"
          aria-label="复制玩家完整 ID"
          title="复制完整 ID"
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
              ? `等级 ${player.profile.level}`
              : "等级 —"
          }}<small
            v-if="player.profile?.fields?.level?.state === 'retained'"
            class="warning-text"
          >
            · 旧</small
          ></span
        ><span v-if="player.stale" class="status-badge warning">旧资料</span
        ><span v-else-if="player.cached">缓存资料</span
        ><span
          v-if="player.privacyState === 'private'"
          class="status-badge"
          :title="fieldTitle('privacy')"
          >生涯不公开<small
            v-if="player.profile?.fields?.privacy?.state === 'retained'"
            class="warning-text"
          >
            · 旧</small
          ></span
        ><span
          v-else-if="player.privacyState === 'unknown'"
          class="privacy-unknown"
          >隐私未核验</span
        >
        <span
          v-if="player.privacyState === 'public'"
          :title="fieldTitle('privacy')"
          >生涯公开<small
            v-if="player.profile?.fields?.privacy?.state === 'retained'"
            class="warning-text"
          >
            · 旧</small
          ></span
        >
      </div>
    </div>
    <div class="profile-ranks">
      <div
        v-for="item in rankCards"
        :key="item.queue"
        class="rank-card"
        :title="item.title"
      >
        <RankEmblem :tier="item.rank?.tier" />
        <div>
          <small>{{
            item.queue === "RANKED_SOLO_5x5" ? "单双排" : "灵活排位"
          }}</small>
          <strong :class="{ muted: !item.rank }"
            >{{ item.label
            }}<small v-if="item.retained" class="warning-text"
              >旧段位</small
            ></strong
          >
          <small v-if="item.rank && !compact" :title="item.record?.title"
            >{{ item.record ? `${item.record.scope} ${item.record.wins} 胜 / ${item.record.losses} 负` : '胜负待确认' }}</small
          >
        </div>
      </div>
    </div>
  </header>
  <p v-if="copyNotice" class="inline-status" role="status">
    {{ copyNotice }}
  </p>
</template>
