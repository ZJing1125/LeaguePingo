<script setup lang="ts">
import { ref } from "vue";
import PlayerTabs from "../../src/components/PlayerTabs.vue";
import { playerTabId } from "../../src/playerState";
import { makePlayer } from "../fixtures";
const players = ref([makePlayer(0), makePlayer(1), makePlayer(2)]);
const active = ref(players.value[0]!.key);
function close(key: string) {
  players.value = players.value.filter((p) => p.key !== key);
  if (active.value === key) active.value = players.value[0]?.key ?? "";
}
</script>
<template>
  <Story title="工作区 / 玩家标签">
    <Variant title="方向键 Home End Delete">
      <div style="padding: 12px">
        <form class="global-search" @submit.prevent>
          <input aria-label="合成输入焦点回退" placeholder="关闭最后一个标签后焦点回到这里" />
        </form>
        <PlayerTabs
          :players="players"
          :active-key="active"
          @select="active = $event"
          @close="close"
        />
        <section
          v-if="active"
          id="player-panel"
          role="tabpanel"
          :aria-labelledby="playerTabId(active)"
          tabindex="0"
          style="padding: 20px"
        >
          当前合成玩家：{{ players.find((p) => p.key === active)?.displayName }}
        </section>
      </div>
    </Variant>
  </Story>
</template>
