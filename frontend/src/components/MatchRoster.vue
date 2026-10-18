<script setup lang="ts">
import { computed } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import {
  decimal,
  percent,
  playerName,
  preparationLabel,
} from "../presentation";
import ParticipantAvatar from "./ParticipantAvatar.vue";
import RecentForm from "./RecentForm.vue";

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
function badge(player: Participant) {
  if (player.identityState === "unavailable") return "身份未知";
  if (player.historyState === "loading") return "正在获取";
  if (player.historyState === "empty") return "暂无战绩";
  if (player.historyState === "failed") return "可重试";
  return props.frozen ? "可查询" : "待准备";
}
</script>
<template>
  <div class="team-columns" aria-label="双方名单">
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
          class="player-row"
          :class="{ selected: selectedKey === player.key }"
          :aria-pressed="selectedKey === player.key"
          :data-player-key="player.key"
          @click="$emit('select', player)"
        >
          <ParticipantAvatar :player="player" :size="32" />
          <span class="roster-identity">
            <strong :title="playerName(player)">{{
              playerName(player)
            }}</strong>
            <small
              class="roster-preparation"
              :title="preparationLabel(player, frozen)"
            >
              {{ preparationLabel(player, frozen)
              }}<template v-if="player.historyState === 'ready'">
                · {{ player.stats.games }} 场</template
              >
            </small>
          </span>
          <span
            class="roster-metrics"
            :title="preparationLabel(player, frozen)"
          >
            <template v-if="player.historyState === 'ready'">
              <b
                ><span class="sr-only">胜率 </span
                >{{ percent(player.stats.winRate) }}</b
              >
              <small>{{ decimal(player.stats.kda) }} KDA</small>
            </template>
            <span
              v-else
              class="status-badge"
              :class="{ 'is-loading': player.historyState === 'loading' }"
              >{{ badge(player) }}</span
            >
          </span>
          <span class="roster-form">
            <RecentForm
              v-if="player.historyState === 'ready'"
              :matches="player.history"
            />
            <small
              v-else-if="player.championState !== 'intent' && player.championId"
              >战绩未就绪</small
            >
            <small
              class="roster-champion-note"
              v-if="player.championState === 'intent'"
              >预选英雄</small
            >
            <small
              class="roster-champion-note"
              v-else-if="!player.championId"
              >{{
                player.championState === "unselected"
                  ? "暂未选择英雄"
                  : "英雄信息暂不可用"
              }}</small
            >
          </span>
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
