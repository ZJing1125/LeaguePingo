<script setup lang="ts">
import { uiText } from "../uiText";
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
    (props.id === 0 ? uiText.gameIcon.empty : props.id ? uiText.gameIcon.unknownResource(props.id) : uiText.common.unknown),
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
