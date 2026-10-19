<script setup lang="ts">
import { computed } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import {
  sampleStats,
  percent,
  decimal,
  tierName,
  preparationLabel,
  date,
} from "../presentation";
import { championName } from "../gameAssets";
import GameIcon from "./GameIcon.vue";
const props = defineProps<{ player: Participant; frozen?: boolean }>();
const sample = computed(() => sampleStats(props.player.history ?? []));
const current = computed(() =>
  sampleStats(
    (props.player.history ?? []).filter(
      (m) => m.championId === props.player.championId,
    ),
  ),
);
const mains = computed(() =>
  [
    ...new Set(
      (props.player.history ?? []).map((m) => m.championId).filter(Boolean),
    ),
  ]
    .map((id) => ({
      id,
      n: (props.player.history ?? []).filter((m) => m.championId === id).length,
    }))
    .sort((a, b) => b.n - a.n)
    .slice(0, 3),
);
const rank = computed(() =>
  props.player.profile?.ranks?.find((r) => r.queue === "RANKED_SOLO_5x5"),
);
</script>
<template>
  <span class="card-details">
    <span
      class="rank-line"
      :title="`单双排段位 · ${date(player.profile?.fields?.rank?.fetchedAt)}`"
      ><span>单双排</span
      ><b>{{
        rank
          ? `${tierName(rank.tier)} ${rank.division} ${rank.lp ?? "—"} LP`
          : "段位未获取"
      }}</b
      ><small v-if="player.profile?.fields?.rank?.state === 'retained'"
        >旧</small
      ></span
    >
    <template v-if="player.historyState === 'ready'">
      <span
        class="overview-metrics"
        :title="`胜负有效 ${sample.outcomeGames}/${sample.games} 场 · KDA 有效 ${sample.kdaGames}/${sample.games} 场 · 获取于 ${date(player.historyMeta?.fetchedAt)}`"
        ><span
          ><b>{{ percent(sample.winRate) }}</b
          ><small>{{ sample.wins }} 胜 / {{ sample.losses }} 负</small></span
        ><span
          ><b>{{ decimal(sample.kda) }}</b
          ><small>汇总 KDA</small></span
        ><small class="sample-size">近 {{ sample.games }} 场</small></span
      >
      <span class="current-hero"
        ><span>本英雄</span
        ><b>{{
          current.games
            ? `${percent(current.winRate)} · ${current.games} 场`
            : "样本不足"
        }}</b
        ><small v-if="current.games"
          >{{ decimal(current.kda) }} KDA</small
        ></span
      >
      <span class="recent-block"
        ><span class="block-caption">近况<small>最近 5 场</small></span
        ><span class="recent-games"
          ><span
            v-for="m in (player.history ?? []).slice(0, 5)"
            :key="m.gameId"
            class="recent-game"
            :class="{ loss: m.win === false }"
            :title="`${championName(m.championId)} · ${m.win == null ? '胜负未知' : m.win ? '胜利' : '失败'}`"
            ><GameIcon :id="m.championId" :size="20" /><small>{{
              m.win == null ? "?" : m.win ? "胜" : "负"
            }}</small></span
          ></span
        ></span
      >
      <span class="main-heroes-row"
        ><span>近期常用</span
        ><span class="main-heroes"
          ><span
            v-for="h in mains"
            :key="h.id"
            class="main-hero"
            :title="`${championName(h.id)} · 当前样本 ${h.n} 场`"
            ><GameIcon :id="h.id" :size="18" /><small>{{ h.n }}</small></span
          ></span
        ></span
      >
    </template>
    <span v-else class="rich-state" :class="player.historyState"
      ><b>{{
        player.identityState === "unavailable"
          ? "身份未知"
          : player.historyState === "loading"
            ? "正在获取"
            : player.historyState === "failed"
              ? "可重试"
              : player.historyState === "empty"
                ? "暂无战绩"
                : preparationLabel(player, frozen)
      }}</b
      ><small>{{
        player.identityState === "unavailable"
          ? "客户端未提供有效身份"
          : player.error || preparationLabel(player, frozen)
      }}</small></span
    >
    <span v-if="player.stale" class="card-freshness">旧资料 · 可主动更新</span>
  </span>
</template>
