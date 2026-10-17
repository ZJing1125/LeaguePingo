<script setup lang="ts">
import PlayerProfileHeader from "../../src/components/PlayerProfileHeader.vue";
import { exampleTime, makePlayer } from "../fixtures";
const normal = makePlayer();
const missing = makePlayer(1, { privacyState: "unknown", profile: null });
const retained = makePlayer(2, {
  stale: true,
  profile: {
    ...normal.profile!,
    fields: { rank: { state: "retained", fetchedAt: exampleTime } },
  },
});
const longName = makePlayer(3, {
  displayName: "这是一个用于检查中文英文混排与超长名称的合成玩家LongName#DEMO",
});
</script>
<template>
  <Story title="玩家 / 资料头部">
    <Variant title="常规"
      ><div style="width: 1100px; max-width: 100%">
        <PlayerProfileHeader :player="normal" connected /></div
    ></Variant>
    <Variant title="头像等级段位未获取"
      ><PlayerProfileHeader :player="missing" connected
    /></Variant>
    <Variant title="断线保留旧资料"
      ><PlayerProfileHeader :player="retained" :connected="false"
    /></Variant>
    <Variant title="长名称"
      ><PlayerProfileHeader :player="longName" connected
    /></Variant>
    <Variant title="窄抽屉"
      ><div class="compact" style="width: 400px; max-width: 100%">
        <PlayerProfileHeader :player="longName" connected compact /></div
    ></Variant>
  </Story>
</template>
