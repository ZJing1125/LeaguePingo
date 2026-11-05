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
<style scoped>
/* Preserve source cascade and contextual sizing; shared tokens remain global. */
.game-icon {
  display: inline-grid;
  place-items: center;
  flex: none;
  border-radius: 5px;
  background-color: #252c37;
  vertical-align: middle;
  overflow: hidden;
  color: #a7b3c5;
  font-size: 12px;
}
.compact .match-hero > .game-icon {
  width: 32px !important;
  height: 32px !important;
}
.compact .match-items > .game-icon {
  width: 17px !important;
  height: 17px !important;
}
:root[data-theme="dark"] :is(.desktop-app, .drawer-floating) .game-icon { background-color: var(--raised); color: var(--muted); }
.scout-tag .game-icon { margin-right: 3px; border-radius: 1px; vertical-align: -2px; }
.overlay-shell .scout-champion .card-pick .game-icon { width: 25px !important; height: 25px !important; }
.recent-game .game-icon {
  border: 1px solid #9ac5ad;
  border-radius: 0;
}
.recent-game.loss .game-icon {
  border-color: #d49ba0;
}
@media (max-width: 1150px) {
  .recent-game .game-icon {
    width: 18px !important;
    height: 18px !important;
  }

}
.card-portrait .card-avatar .game-icon {
  border-radius: 0;
}
.game-icon {
  background-color: #d3d9c8;
  border-radius: 2px;
}
.match-workspace .recent-game .game-icon {
  width: 22px !important;
  height: 22px !important;
}
.sidebar-hero>.game-icon {flex:0 0 26px;}
</style>
