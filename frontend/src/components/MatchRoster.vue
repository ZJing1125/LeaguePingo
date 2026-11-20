<script setup lang="ts">
import { uiText } from "../uiText";
import { computed } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import { roomHistoryTarget } from "../playerHistoryNavigation";
import { playerName } from "../presentation";
import PlayerScoutCard from "./PlayerScoutCard.vue";
import ExperimentalBadge from "./ExperimentalBadge.vue";

const props = defineProps<{
  players: Participant[];
  selectedKey?: string;
  party?: boolean;
  frozen?: boolean;
  queue?: number;
  captured?: boolean;
  alliesOnly?: boolean;
}>();
defineEmits<{ select: [player: Participant] }>();
const teams = computed(() =>
  (["ally", "enemy"] as const).filter(key => !props.alliesOnly || key === 'ally').map((key) => ({
    key,
    label: props.party
      ? key === "ally"
        ? uiText.common.party
        : uiText.matchRoster.noOpponent
      : key === "ally"
        ? uiText.common.ally
        : uiText.common.enemy,
    players: props.players.filter((player) => player.team === key),
  })),
);
</script>
<template>
  <div
    class="team-columns"
    :class="{
      'expanded-roster': teams.some((team) => team.players.length > 5),
      'party-roster': party,
    }"
    :aria-label="uiText.matchRoster.title"
  >
    <section
      v-for="team in teams"
      :key="team.key"
      class="team-section"
      :aria-label="team.label"
    >
      <h2>
        <span>{{ team.label }}</span
        ><small><ExperimentalBadge v-if="team.players.some(p => p.premadeGroup && (p.premadeSize || 0) > 1)" :feature="uiText.clientSidebar.groups" :label="uiText.clientSidebar.groupExperimental"/>{{ uiText.matchRoster.members(team.players.length) }}</small>
      </h2>
      <div class="team-player-list">
        <div
          v-for="player in team.players"
          :key="player.key"
          class="player-row hero-card scout-card"
          :class="{ selected: selectedKey === player.key, 'scout-enemy': team.key === 'enemy' }"
          :data-player-key="player.key"
        >
          <button type="button" class="player-row-preview" :aria-pressed="selectedKey === player.key"
            :aria-label="uiText.playerHistoryNavigation.roomPreview(playerName(player))" @click="$emit('select', player)" />
          <PlayerScoutCard :player="player" :frozen="frozen" :party="party" :queue="queue" :captured="captured"
            :history="roomHistoryTarget(player, !!captured)" />
        </div>
      </div>
      <p v-if="!team.players.length" class="team-empty">
        {{
          party && team.key === "enemy"
            ? uiText.matchRoster.partyOpponent
            : uiText.matchRoster.waitingRoster
        }}
      </p>
    </section>
  </div>
</template>
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.team-columns):is(.team-columns) {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  flex: 1;
  min-width: 0;
  align-content: start;
  overflow-y: auto;
  scrollbar-gutter: stable;
  padding: 0 2px 16px;
}
:where(.team-section):is(.team-section) {
  min-width: 0;
}
:where(.team-section):is(.team-section) :deep(h2) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 36px;
  color: #adcabe;
}
:where(.team-section):is(.team-section) :deep(h2>span) {
  display: inline-flex;
  gap: 7px;
  align-items: center;
}
:where(.team-section):is(.team-section) :deep(h2>span::before) {
  content: "";
  width: 3px;
  height: 12px;
  background: #62d6ba;
  border-radius: 2px;
}
:where(.team-section:nth-child(2)):is(.team-section:nth-child(2)) :deep(h2) {
  color: #d1b1be;
}
:where(.team-section:nth-child(2)):is(.team-section:nth-child(2)) :deep(h2>span::before) {
  background: #c08396;
}
:where(.team-section):is(.team-section) :deep(h2 small) {
  color: var(--muted);
  font-size: 11px;
  font-weight: 400;
}
:where(.team-player-list):is(.team-player-list) {
  display: grid;
  gap: 8px;
}
:where(.team-empty):is(.team-empty) {
  border: 1px dashed var(--border);
  border-radius: var(--radius);
  padding: 24px 12px;
  color: var(--muted);
  text-align: center;
  font-size: 12px;
}
:where(.player-row):is(.player-row) {
  width: 100%;
  display: grid;
  grid-template-columns: 32px minmax(0, 1fr) 52px;
  grid-template-rows: auto auto;
  align-items: center;
  gap: 6px 8px;
  padding: 11px 10px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  text-align: left;
  min-height: 84px;
}
:where(.player-row:hover):is(.player-row:hover) {
  background: #1c2530;
  border-color: #43555f;
}
:where(.player-row.selected):is(.player-row.selected) {
  border-color: #559c89;
  background: #1b2d2b;
  box-shadow: inset 3px 0 0 var(--accent);
}
@media (max-width: 760px) {
  :where(.team-columns):is(.team-columns) {
    gap: 10px;
  }
}
@media (max-width: 620px) {
  :where(.team-columns):is(.team-columns) {
    grid-template-columns: 1fr;
  }
}
:where(.player-row.selected):is(:root[data-theme="dark"] :is(.desktop-app,.drawer-floating) .player-row.selected):after { border-color: var(--accent-art); }
:where(.hero-card:focus-visible):is(:root[data-theme="dark"] :is(.desktop-app,.drawer-floating) .hero-card:focus-visible) { box-shadow: inset 0 0 0 3px var(--accent); }
:where(.hero-card.player-row.scout-card):is(.match-workspace .hero-card.player-row.scout-card) {
  --scout-team: var(--scout-ally);
  position: relative;
  isolation: isolate;
  display: flex;
  flex-direction: column;
  gap: 0;
  padding: 0;
  color: var(--scout-text);
  background: var(--scout-card);
  border: 1px solid var(--scout-border);
  border-radius: 5px;
  clip-path: none;
  overflow: hidden;
  box-shadow: 0 2px 5px #11182725;
  min-width: 0;
  min-height: 0;
  height: 300px;
}
:where(.hero-card.player-row.scout-enemy):is(.match-workspace .hero-card.player-row.scout-enemy) { --scout-team: var(--scout-enemy); }
:where(.hero-card.player-row.scout-card):is(.match-workspace .hero-card.player-row.scout-card)::before { content: none; display: none; }
:where(.hero-card.player-row.scout-card):is(.match-workspace .hero-card.player-row.scout-card)::after { border: 0; }
:where(.hero-card.player-row.scout-card.selected):is(.match-workspace .hero-card.player-row.scout-card.selected)::after { border: 2px solid var(--accent-art); border-radius: inherit; }
:where(.hero-card.player-row.scout-card:hover):is(.match-workspace .hero-card.player-row.scout-card:hover) { border-color: var(--scout-team); background: var(--scout-card); }
:where(.team-player-list):is(.match-workspace .team-columns .team-player-list) { grid-auto-rows: 300px; gap: 10px; }
:where(.team-player-list):is(.match-workspace .expanded-roster .team-player-list) { grid-auto-rows: 300px; gap: 10px; }
:where(.team-section):is(.desktop-app .match-workspace .team-section) :deep(h2) { height: 18px; min-height: 18px; font-size: 12px; }
:where(.team-columns):is(.desktop-app .match-workspace .team-columns) { gap: 10px; }
:where(.team-empty):is(.match-workspace .team-empty) { margin-top: 4px; }
:where(.team-columns):is(.team-columns) {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  grid-template-rows: minmax(246px, 1fr) minmax(246px, 1fr);
  width: 100%;
  gap: 12px;
  overflow: auto;
  flex: 1;
  min-height: 0;
  padding-right: 2px;
  scrollbar-width: thin;
}
:where(.team-section):is(.team-section) {
  display: flex;
  flex-direction: column;
  min-height: 0;
  min-width: 0;
}
:where(.team-section):is(.team-section) :deep(h2) {
  font-size: 11px;
  height: 24px;
  flex: none;
  margin: 0;
  letter-spacing: 1px;
}
:where(.team-section):is(.team-section) :deep(h2>span) {
  color: var(--ally);
}
:where(.team-section:nth-child(2)):is(.team-section:nth-child(2)) :deep(h2>span) {
  color: var(--enemy);
}
:where(.team-section):is(.team-section) :deep(h2 small) {
  font-size: 9px;
  color: var(--muted);
}
:where(.team-player-list):is(.team-player-list) {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  grid-auto-rows: minmax(222px, 1fr);
  gap: 11px;
  flex: 1;
  min-height: 0;
}
:where(.team-empty):is(.team-empty) {
  flex: 1;
  display: grid;
  place-items: center;
  background: var(--raised);
  border: 1px dashed var(--border);
  color: var(--muted);
  font-size: 12px;
  margin: 0;
  min-height: 120px;
}
:where(.hero-card.player-row):is(.hero-card.player-row) {
  position: relative;
  isolation: isolate;
  display: flex;
  flex-direction: column;
  text-align: left;
  align-items: stretch;
  justify-content: flex-start;
  background: #15242b;
  color: #e4ebe3;
  padding: 0;
  gap: 0;
  border: 0;
  border-radius: 0;
  overflow: hidden;
  min-width: 0;
  min-height: 222px;
  clip-path: polygon(
    0 0,
    100% 0,
    100% calc(100% - 10px),
    calc(100% - 10px) 100%,
    0 100%
  );
}
:where(.hero-card.player-row):is(.hero-card.player-row):after {
  content: "";
  position: absolute;
  inset: 0;
  border: 1px solid #ffffff22;
  pointer-events: none;
  z-index: 3;
}
:where(.hero-card.player-row:hover):is(.hero-card.player-row:hover) {
  background: #1b3036;
}
:where(.hero-card.player-row.selected):is(.hero-card.player-row.selected):after {
  border: 2px solid #ed8458;
}
:where(.hero-card.player-row:focus-visible):is(.hero-card.player-row:focus-visible) {
  outline: none;
  box-shadow: inset 0 0 0 3px #ffb188;
}
:where(.hero-card):is(.hero-card):before {
  position: absolute;
  content: "";
  inset: 0;
  z-index: 1;
  pointer-events: none;
  background:
    linear-gradient(90deg, #10212bef 0%, #10212bcc 44%, #10212b20 100%),
    linear-gradient(0deg, #10212bf2 0%, #10212b88 30%, #10212b0a 100%);
}
:where(.hero-card):is(.hero-card) :deep(>.card-illustration) {
  inset: 8px;
  width: calc(100% - 16px);
  height: calc(100% - 16px);
  object-fit: contain;
  object-position: right center;
  z-index: 0;
}
@media (max-width: 1150px) {
  :where(.team-player-list):is(.team-player-list) {
    gap: 8px;
  }

}
@media (max-width: 999px) {
  :where(.team-columns):is(.team-columns) {
    grid-template-rows: minmax(260px, 1fr) minmax(260px, 1fr);
  }
}
@media (max-width: 999px) {
  :where(.team-player-list):is(.team-player-list) {
    grid-auto-rows: minmax(236px, 1fr);
  }
}
@media (max-width: 999px) {
  :where(.hero-card.player-row):is(.hero-card.player-row) {
    min-height: 236px;
  }
}
:where(.team-columns):is(.team-columns) {
  padding: 0 2px;
  scrollbar-gutter: auto;
  align-content: stretch;
}
:where(.team-section):is(.team-section) :deep(h2) {
  min-height: 24px;
}
:where(.team-section):is(.team-section) :deep(h2>span:before) {
  background: #32768c;
  width: 12px;
  height: 3px;
  border-radius: 0;
}
:where(.team-section:nth-child(2)):is(.team-section:nth-child(2)) :deep(h2>span:before) {
  background: #be665e;
}
:where(.team-empty):is(.team-empty) {
  border-radius: 0;
}
:where(.team-columns.expanded-roster):is(.team-columns.expanded-roster) {
  grid-template-rows: max-content max-content;
  align-content: start;
}
:where(.team-player-list):is(.team-columns.expanded-roster .team-player-list) {
  grid-auto-rows: 236px;
  flex: none;
}
:where(.team-columns):is(.match-workspace .team-columns) {
  grid-template-rows: max-content max-content;
  align-content: start;
  gap: 16px;
  padding: 0 2px 8px;
}
:where(.team-columns.expanded-roster):is(.match-workspace .team-columns.expanded-roster) {
  grid-template-rows: max-content max-content;
  align-content: start;
  gap: 16px;
  padding: 0 2px 8px;
}
:where(.team-player-list):is(.match-workspace .team-player-list) {
  grid-template-columns: repeat(5, minmax(0, 1fr));
  grid-auto-rows: 300px;
  flex: none;
  gap: 10px;
}
:where(.team-player-list):is(.match-workspace .expanded-roster .team-player-list) {
  grid-template-columns: repeat(5, minmax(0, 1fr));
  grid-auto-rows: 300px;
  flex: none;
  gap: 10px;
}
:where(.team-section):is(.match-workspace .team-section) :deep(h2) {
  height: 27px;
  min-height: 27px;
  font-size: 13px;
}
:where(.team-section):is(.match-workspace .team-section) :deep(h2 small) {
  font-size: 12px;
}
:where(.hero-card.player-row):is(.match-workspace .hero-card.player-row) {
  height: 300px;
  min-height: 300px;
  border-radius: 8px;
  clip-path: none;
  background: radial-gradient(ellipse at top, #29434b, #15272f 70%);
}
:where(.hero-card):is(.match-workspace .hero-card):after {
  border-radius: inherit;
  border-color: #89a5a833;
}
:where(.hero-card.selected):is(.match-workspace .hero-card.selected):after {
  border: 2px solid var(--accent-art);
}
:where(.hero-card):is(.match-workspace .hero-card):before {
  background: linear-gradient(
    180deg,
    #12252ee0 0%,
    #12252e66 24%,
    #12252ebd 48%,
    #12252ef5 84%
  );
}
:where(.team-player-list):is(.match-workspace .party-roster .team-player-list) {
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
}
:where(.team-empty):is(.match-workspace .team-empty) {
  flex: none;
  min-height: 66px;
  font-size: 13px;
  background: transparent;
  border: 0;
  border-top: 1px dashed var(--border);
}
@media (max-width: 1000px) {
  :where(.team-player-list):is(.match-workspace .team-player-list) {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
@media (max-width: 1000px) {
  :where(.team-player-list):is(.match-workspace .expanded-roster .team-player-list) {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
:where(.team-columns):is(.review-room>.team-columns) {flex:none;min-height:680px;overflow:visible;}
:where(.team-columns):is(.current-match>.team-columns) {flex:none;overflow:visible;}
.player-row-preview {
  position:absolute;
  z-index:3;
  inset:0;
  padding:0;
  background:none;
  border:0;
  border-radius:inherit;
}
.player-row-preview:focus-visible { outline-offset:-3px; }
.player-row :deep(button.scout-identity) { z-index:4; text-align:left; }
</style>
