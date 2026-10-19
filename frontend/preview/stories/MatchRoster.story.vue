<script setup lang="ts">
import { ref } from "vue";
import {
  HistoryState,
  IdentityState,
} from "../../bindings/leaguepingo/internal/core/models";
import MatchRoster from "../../src/components/MatchRoster.vue";
import { makePlayer } from "../fixtures";
const selected = ref("");
const players = Array.from({ length: 10 }, (_, i) => makePlayer(i));
const mixed = players.map((p, i) =>
  i < 5
    ? p
    : makePlayer(i, {
        identityState:
          i === 9 ? IdentityState.IdentityUnavailable : p.identityState,
        historyState:
          i === 9
            ? HistoryState.HistoryPending
            : i === 8
              ? HistoryState.HistoryFailed
              : i === 7
                ? HistoryState.HistoryEmpty
                : HistoryState.HistoryLoading,
        history: [],
        championState: i === 6 ? "intent" : "selected",
      }),
);
const longNames = players.map((p) => ({
  ...p,
  displayName: "这是很长的中英文混排玩家名称LongName#DEMO",
}));
const rengar = makePlayer(0, { championId: 107, championState: "selected" });
</script>
<template>
  <Story title="对局 / 双方名单">
    <Variant title="双方十人"
      ><div
        class="match-workspace"
        style="width: 1180px; max-width: 100%; padding: 12px"
      >
        <MatchRoster
          :players="players"
          :selected-key="selected"
          frozen
          @select="selected = $event.key"
        /></div
    ></Variant>
    <Variant title="准备中与身份缺失"
      ><div
        class="match-workspace"
        style="width: 1180px; max-width: 100%; padding: 12px"
      >
        <MatchRoster :players="mixed" frozen /></div
    ></Variant>
    <Variant title="组队阶段"
      ><div
        class="match-workspace"
        style="width: 1180px; max-width: 100%; padding: 12px"
      >
        <MatchRoster :players="players.slice(0, 3)" party /></div
    ></Variant>
    <Variant title="长名称与窄窗阵容"
      ><div
        class="match-workspace"
        style="width: 820px; max-width: 100%; padding: 12px"
      >
        <MatchRoster
          :players="longNames"
          :selected-key="players[1]!.key"
        /></div
    ></Variant>
    <Variant title="完整原画与清晰数据"
      ><div
        class="match-workspace"
        style="width: 1100px; max-width: 100%; height: 680px; padding: 12px"
      >
        <MatchRoster
          :players="[rengar, ...players.slice(1)]"
          :selected-key="rengar.key"
        /></div
    ></Variant>
  </Story>
</template>
