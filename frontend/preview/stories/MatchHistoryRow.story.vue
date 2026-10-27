<script setup lang="ts">
import { ref } from "vue";
import MatchHistoryRow from "../../src/components/MatchHistoryRow.vue";
import { makeMatch } from "../fixtures";
const expanded = ref(false);
</script>
<template>
  <Story title="战绩 / 战绩行">
    <Variant title="胜利 · 默认装备技能">
      <div style="width: 920px; max-width: 100%; padding: 16px">
        <MatchHistoryRow
          :match="makeMatch(1, { win: true })"
          :expanded="expanded"
          @toggle="expanded = !expanded"
        />
      </div>
    </Variant>
    <Variant title="失败"
      ><div style="padding: 16px">
        <MatchHistoryRow :match="makeMatch(0)" :expanded="false" /></div
    ></Variant>
    <Variant title="未知结果与缺失字段"
      ><div style="padding: 16px">
        <MatchHistoryRow
          :match="
            makeMatch(0, {
              win: null,
              kills: null,
              damage: null,
              playedAt: null,
              items: null,
            })
          "
          :expanded="false"
        /></div
    ></Variant>
    <Variant title="展开 · 自定义详情插槽"
      ><div style="padding: 16px">
        <MatchHistoryRow :match="makeMatch(1)" expanded
          ><template #details
            ><p class="muted">合成详情插槽</p></template
          ></MatchHistoryRow
        >
      </div></Variant
    >
    <Variant title="抽屉 · 紧凑布局"
      ><div
        class="compact"
        style="width: 400px; max-width: 100%; padding: 12px"
      >
        <MatchHistoryRow :match="makeMatch(1)" :expanded="false" compact /></div
    ></Variant>
  </Story>
</template>
