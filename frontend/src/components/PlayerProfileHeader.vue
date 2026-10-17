<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import { date, tierName, number } from "../presentation";
import AppIcon from "./AppIcon.vue";
const props = defineProps<{
  player: Participant;
  busy?: boolean;
  connected?: boolean;
  compact?: boolean;
}>();
const emit = defineEmits<{ profile: [player: Participant] }>();
const copyNotice = ref("");
const avatarBroken = ref(false);
watch(
  [() => props.player.key, () => props.player.profile?.profileIconData],
  () => {
    avatarBroken.value = false;
  },
);
const hasRetainedProfile = computed(() =>
  Object.values(props.player.profile?.fields ?? {}).some(
    (field) => field?.state === "retained",
  ),
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
        <h1 :title="player.displayName">{{ player.displayName }}</h1>
        <button
          class="icon-button square"
          aria-label="复制玩家完整 ID"
          title="复制完整 ID"
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
          >生涯不公开（客户端明确返回）<small
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
    <div v-if="!compact" class="profile-ranks">
      <div
        v-for="queue in ['RANKED_SOLO_5x5', 'RANKED_FLEX_SR']"
        :key="queue"
        class="rank-card"
      >
        <AppIcon name="shield" :size="24" />
        <div>
          <small>{{
            queue === "RANKED_SOLO_5x5" ? "单双排" : "灵活排位"
          }}</small
          ><template
            v-for="rank in (player.profile?.ranks ?? []).filter(
              (r) => r.queue === queue,
            )"
            :key="rank.queue"
            ><strong :title="fieldTitle('rank')"
              >{{ tierName(rank.tier) }} {{ rank.division }}
              <span>{{ number(rank.lp) }} LP</span
              ><small
                v-if="player.profile?.fields?.rank?.state === 'retained'"
                class="warning-text"
                >旧段位</small
              ></strong
            ><small
              >{{ number(rank.wins) }} 胜 / {{ number(rank.losses) }} 负</small
            ></template
          ><strong
            v-if="!(player.profile?.ranks ?? []).some((r) => r.queue === queue)"
            class="muted"
            >段位未获取</strong
          >
        </div>
      </div>
    </div>
    <button
      class="icon-button profile-update"
      aria-label="更新玩家资料"
      title="更新等级、段位与隐私"
      :disabled="
        busy || !connected || player.stale || player.identityState !== 'known'
      "
      @click="emit('profile', player)"
    >
      <AppIcon name="refresh" :size="16" /><span>更新资料</span>
    </button>
  </header>
  <p v-if="hasRetainedProfile" class="profile-freshness" role="status">
    部分资料未更新，标“旧”的字段保留了上次结果；悬停查看获取时间。
  </p>
  <p v-if="copyNotice" class="inline-status" role="status">
    {{ copyNotice }}
  </p>
</template>
