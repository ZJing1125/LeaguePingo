<script setup lang="ts">
import { ref, watch } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type {
  MatchDetail,
  Participant,
} from "../../bindings/leaguepingo/internal/core/models";
import { errorMessage, kda, number, percent } from "../presentation";
import { championName } from "../gameAssets";
import GameIcon from "./GameIcon.vue";
const props = defineProps<{
  player: Participant;
  gameId: number;
  connected?: boolean;
}>();
const detail = ref<MatchDetail | null>(null),
  busy = ref(false),
  error = ref("");
let revision = 0;
watch(
  () => [props.player.key, props.player.connectionId, props.gameId],
  () => {
    revision++;
    detail.value = null;
    busy.value = false;
    error.value = "";
  },
);
async function load() {
  if (busy.value || !props.connected || props.player.stale) return;
  const request = ++revision;
  busy.value = true;
  error.value = "";
  try {
    const result = await Service.MatchDetail(
      props.player.key,
      props.player.connectionId,
      props.gameId,
    );
    if (request === revision) detail.value = result;
  } catch (reason) {
    if (request === revision) error.value = errorMessage(reason);
  } finally {
    if (request === revision) busy.value = false;
  }
}
</script>
<template>
  <section class="full-match-detail">
    <button
      v-if="!detail"
      class="secondary"
      :disabled="busy || !connected || player.stale"
      @click="load"
    >
      {{ busy ? "正在读取详情…" : "读取双方详情" }}
    </button>
    <p v-if="error" class="error-banner" role="alert">{{ error }}</p>
    <template v-if="detail">
      <p class="muted">
        已读取 {{ detail.players?.length ?? 0 }} 名参与者 · 本次按需获取<span
          v-if="detail.partial"
        >
          · 阵容不完整，不计算团队占比</span
        >
      </p>
      <div class="detail-table-scroll">
        <table
          v-for="team in [
            ...new Set((detail.players ?? []).map((p) => p.teamId)),
          ].sort()"
          :key="team"
        >
          <caption>
            {{
              team === 100 ? "蓝方" : team === 200 ? "红方" : `队伍 ${team}`
            }}
          </caption>
          <thead>
            <tr>
              <th>玩家 / 英雄</th>
              <th>K / D / A</th>
              <th>装备 / 技能</th>
              <th>伤害 / 占比</th>
              <th>金币 / 占比</th>
              <th>参团率</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="p in (detail.players ?? []).filter(
                (p) => p.teamId === team,
              )"
              :key="p.participantId"
              :class="{ target: p.target }"
            >
              <td>
                <GameIcon :id="p.summary.championId" :size="24" /><span
                  >{{ p.displayName
                  }}<small
                    >{{ championName(p.summary.championId)
                    }}{{ p.target ? " · 查询玩家" : "" }}</small
                  ></span
                >
              </td>
              <td>{{ kda(p.summary) }}</td>
              <td>
                <span class="detail-loadout"
                  ><GameIcon
                    v-for="slot in 7"
                    :key="slot"
                    kind="items"
                    :id="p.summary.items?.[slot - 1]"
                    :size="18" /></span
                ><span class="detail-loadout"
                  ><GameIcon
                    kind="spells"
                    :id="p.summary.spell1"
                    :size="16" /><GameIcon
                    kind="spells"
                    :id="p.summary.spell2"
                    :size="16"
                /></span>
              </td>
              <td>
                {{ number(p.summary.damage)
                }}<small>{{ percent(p.damageShare) }}</small>
              </td>
              <td>
                {{ number(p.summary.gold)
                }}<small>{{ percent(p.goldShare) }}</small>
              </td>
              <td>{{ percent(p.participation) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p class="muted">
        伤害与金币占比以本队总量为分母；参团率＝（击杀＋助攻）÷
        本队击杀。仅标准完整 5v5 且相关字段齐全时计算，缺失或无有效分母显示 —。
      </p>
    </template>
  </section>
</template>
<style scoped>
.full-match-detail {
  margin-top: 12px;
}
.full-match-detail p {
  font-size: 11px;
  line-height: 1.6;
}
.detail-table-scroll {
  overflow-x: auto;
}
table {
  width: 100%;
  min-width: 520px;
  border-collapse: collapse;
  font-size: 11px;
}
caption {
  text-align: left;
  font-weight: 700;
  padding: 8px;
  color: var(--text);
}
th,
td {
  text-align: left;
  padding: 7px 6px;
  border-bottom: 1px solid var(--border);
}
td:first-child {
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: 225px;
  overflow-wrap: anywhere;
}
small {
  display: block;
  color: var(--muted);
  margin-top: 3px;
}
.target {
  background: var(--selection);
}
</style>
