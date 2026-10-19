<script setup lang="ts">
import { ref } from "vue";
import type {
  HistoryState,
  IdentityState,
} from "../../bindings/leaguepingo/internal/core/models";
import OverlayView from "../../src/components/OverlayView.vue";
import { makeOverlayState, makeSnapshot, makePlayer } from "../fixtures";
const state = ref(makeOverlayState());
const ready = makeSnapshot();
const partial = makeSnapshot({
  readyCount: 5,
  knownCount: 9,
  participants: Array.from({ length: 10 }, (_, i) =>
    makePlayer(
      i,
      i < 5
        ? {}
        : {
            historyState: (i === 9 ? "failed" : "pending") as HistoryState,
            identityState: (i === 8 ? "unavailable" : "known") as IdentityState,
            displayName: i === 8 ? "身份不可获取" : `合成玩家${i + 1}#DEMO`,
            history: [],
            error: i === 9 ? "本次准备失败，可在主窗口重试" : "",
          },
    ),
  ),
});
</script>
<template>
  <Story title="对局 / 快览">
    <Variant title="十人就绪"
      ><div style="width: 1100px; height: 640px">
        <OverlayView
          :snapshot="ready"
          :state="state"
          status="对局中"
          :busy="false"
          error=""
          @pin="state.pinned = !state.pinned"
        /></div
    ></Variant>
    <Variant title="部分就绪与失败"
      ><div style="width: 1100px; height: 640px">
        <OverlayView
          :snapshot="partial"
          :state="state"
          status="选人 · 准备资料"
          :busy="false"
          error=""
        /></div
    ></Variant>
    <Variant title="断线与旧资料"
      ><div style="width: 1100px; height: 640px">
        <OverlayView
          :snapshot="
            makeSnapshot({
              connected: false,
              participants: ready.participants!.map((p) => ({
                ...p,
                stale: true,
              })),
            })
          "
          :state="state"
          status="连接中断 · 保留资料"
          :busy="false"
          error=""
        /></div
    ></Variant>
    <Variant title="400 × 360 最小尺寸"
      ><div style="width: 400px; height: 360px">
        <OverlayView
          :snapshot="ready"
          :state="state"
          status="对局中"
          :busy="false"
          error=""
        /></div
    ></Variant>
    <Variant title="按住只读快览"
      ><div style="width: 1100px; height: 640px">
        <OverlayView
          :snapshot="ready"
          :state="{ ...state, mode: 'hold' }"
          status="对局中"
          :busy="false"
          error=""
        /></div
    ></Variant>
  </Story>
</template>
