<script setup lang="ts">
import { computed } from "vue";
import { gameAsset, iconStyle, type AssetKind } from "../gameAssets";
const props = withDefaults(
  defineProps<{ id?: number | null; kind?: AssetKind; size?: number }>(),
  { kind: "champions", size: 38 },
);
const entry = computed(() => gameAsset(props.kind, props.id));
const label = computed(
  () =>
    entry.value?.name ??
    (props.id === 0 ? "空槽位" : props.id ? `未知资源 #${props.id}` : "未知"),
);
</script>
<template>
  <span
    class="game-icon"
    role="img"
    :aria-label="label"
    :title="label"
    :style="iconStyle(kind, id, size)"
    ><span v-if="!entry">{{ id === 0 ? "·" : "?" }}</span></span
  >
</template>
