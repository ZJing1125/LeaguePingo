<script setup lang="ts">
import { matchMode } from "../matchMode";
import { computed } from "vue";
import type { Match, MatchDetail, MatchPlayer } from "../../bindings/leaguepingo/internal/core/models";
import { perMinute, targetPlayer } from "../analysisData";
import { hasFullScoreboard, historyHeroName, historyPosition, matchDamageLeader, matchNumber, teamComparison } from "../historyMatch";
import { isRemake } from "../matchStats";
import { decimal, kda, percent, positionName, sampleStats } from "../presentation";
import { uiText } from "../uiText";
import { historyTarget } from "../playerHistoryNavigation";
import PlayerHistoryAction from "./PlayerHistoryAction.vue";
import GameIcon from "./GameIcon.vue";
import RuneIcon from "./RuneIcon.vue";

const props = defineProps<{ detail: MatchDetail; match?: Match; self?: boolean; connectionId?: number }>();
const players = computed(() => props.detail.players ?? []);
const rules = computed(() => matchMode(props.match ?? targetPlayer(props.detail)?.summary ?? players.value[0]?.summary));
const full = computed(() => hasFullScoreboard(props.detail));
const targetTeam = computed(() => targetPlayer(props.detail)?.teamId);
const teams = computed(() => {
  const ids = [...new Set(players.value.map(player => player.teamId))].sort((a,b) => a-b);
  const own = targetTeam.value;
  return own != null && ids.includes(own) ? [own, ...ids.filter(id => id !== own)] : ids;
});
const groups = computed(() => teams.value.map(id => {
  const members = players.value.filter(player => player.teamId === id);
  const win = members[0]?.summary.win;
  const outcome = members.some(player => isRemake(player.summary))
    ? uiText.common.remake
    : win != null && members.every(player => player.summary.win === win)
      ? win ? uiText.common.win : uiText.common.loss
      : uiText.matchDetails.pendingOutcome;
  const resources = props.detail.teams?.find(team => team.teamId === id);
  return { id, players: members, outcome, objectives: (rules.value.objectives ? [
    rules.value.structures && resources?.towers != null ? uiText.matchDetails.towers(matchNumber(resources.towers)) : "",
    rules.value.riftObjectives && resources?.dragons != null ? uiText.matchDetails.dragons(matchNumber(resources.dragons)) : "",
    rules.value.riftObjectives && resources?.barons != null ? uiText.matchDetails.barons(matchNumber(resources.barons)) : "",
    rules.value.structures && resources?.inhibitors != null ? uiText.matchDetails.inhibitors(matchNumber(resources.inhibitors)) : "",
    rules.value.neutralObjectives && resources?.heralds != null ? uiText.matchDetails.heralds(matchNumber(resources.heralds)) : "",
    rules.value.riftObjectives && resources?.hordes != null ? uiText.matchDetails.grubs(matchNumber(resources.hordes)) : "",
  ] : []).filter(Boolean) };
}));
const totals = computed(() => teamComparison(props.detail, teams.value));
const damageMax = computed(() => Math.max(0, ...players.value.map(p => p.summary.damage ?? 0)));
const damageComplete = computed(() => full.value && players.value.every(p => p.summary.damage != null));
const damageLeader = computed(() => matchDamageLeader(props.detail));
const teamName = (id: number) => id === 100 ? uiText.common.blue : id === 200 ? uiText.common.red : uiText.matchDetails.team(id);
function playerName(player: MatchPlayer) {
  return player.displayName?.trim() && player.displayName !== uiText.matchDetails.maskedPlayer(player.participantId)
    ? player.displayName : uiText.matchDetails.missingPlayerName;
}
function csRate(player: MatchPlayer) {
  const value = perMinute(player.summary.cs, player.summary.duration);
  return value == null ? "—" : uiText.matchDetails.perMinute(value.toFixed(1));
}
function visionDetails(player: MatchPlayer) {
  const { wardsPlaced, wardsKilled, controlWards } = player.summary;
  const wards = uiText.matchDetails.wards(matchNumber(wardsPlaced), matchNumber(wardsKilled));
  return controlWards != null ? `${wards} · ${uiText.matchDetails.controlWards(matchNumber(controlWards))}` : wards;
}
function participation(player: MatchPlayer) {
  return full.value ? (player.participation ?? player.summary.participation) : null;
}
</script>

<template>
  <div class="scoreboard-content">
    <p v-if="!full" class="scoreboard-status">{{ players.length < 10 ? uiText.matchDetails.partialPlayers(players.length) : uiText.matchDetails.playerCount(players.length) }}</p>
    <div class="detail-table-scroll" tabindex="0" role="region" :aria-label="uiText.matchDetails.scoreboardLabel">
      <div class="scoreboard-board">
        <template v-for="(group, index) in groups" :key="group.id">
          <section class="scoreboard-team" :class="{ 'team-red': group.id === 200 }" :aria-label="teamName(group.id)">
            <header class="team-summary">
              <b :class="{ victory: group.outcome === uiText.common.win, defeat: group.outcome === uiText.common.loss }">{{ group.outcome }}</b>
              <strong v-if="targetTeam != null">{{ group.id === targetTeam ? uiText.matchDetails.targetSide : uiText.matchDetails.opposingSide }}</strong>
              <span>{{ teamName(group.id) }}</span>
              <span v-if="group.objectives.length" class="objective-line"><span v-for="objective in group.objectives" :key="objective">{{ objective }}</span></span>
            </header>
            <table>
              <thead><tr>
                <th scope="col">{{ uiText.matchDetails.playerChampion }}</th>
                <th scope="col">{{ uiText.matchDetails.kdaParticipation }}</th>
                <th scope="col">{{ uiText.matchDetails.damageTaken }}</th>
                <th scope="col" :title="rules.cs ? uiText.matchDetails.csHint : undefined">{{ rules.cs ? uiText.matchDetails.csGold : uiText.matchMode.gold }}</th>
                <th v-if="rules.vision" scope="col">{{ uiText.common.vision }}</th>
                <th scope="col">{{ uiText.common.finalItems }}</th>
              </tr></thead>
              <tbody>
                <tr v-for="player in group.players" :key="player.participantId" :class="{ target: player.target }">
                  <td><PlayerHistoryAction class="scoreboard-player" :target="historyTarget(player.displayName, connectionId)">
                    <span class="scoreboard-hero"><GameIcon :id="player.summary.championId" :size="30"/><small v-if="player.summary.championLevel != null">{{ player.summary.championLevel }}</small></span>
                    <span class="scoreboard-systems">
                      <GameIcon kind="spells" :id="player.summary.spell1" :size="14"/>
                      <RuneIcon v-if="matchMode(player.summary).runes && player.summary.runeIds?.[0]" :id="player.summary.runeIds[0]" :size="14"/><span v-else></span>
                      <GameIcon kind="spells" :id="player.summary.spell2" :size="14"/>
                      <RuneIcon v-if="matchMode(player.summary).runes && player.summary.runeSecondary" :id="player.summary.runeSecondary" :size="14"/>
                    </span>
                    <span class="scoreboard-name">
                      <strong :title="playerName(player)">{{ playerName(player) }}</strong>
                      <small>{{ historyHeroName(player.summary.championId) }}<template v-if="rules.roles && historyPosition(player.summary)"> · {{ positionName(player.summary.position) }}</template></small>
                    </span>
                    <span v-if="player.target" class="target-label">{{ self ? uiText.matchDetails.self : uiText.matchDetails.lookedUp }}</span>
                  </PlayerHistoryAction></td>
                  <td><b>{{ kda(player.summary) }}</b><small>{{ uiText.matchDetails.kdaRate(decimal(sampleStats([player.summary]).kda), percent(participation(player))) }}</small></td>
                  <td>
                    <b>{{ matchNumber(player.summary.damage) }}</b><span v-if="damageLeader === player.participantId" class="damage-leader" :title="uiText.matchDetails.damageLeader" :aria-label="uiText.matchDetails.damageLeader"> ◆</span>
                    <span v-if="player.summary.damage != null && damageMax > 0" class="damage-track" :title="damageComplete ? uiText.matchDetails.damageHint : uiText.matchDetails.visibleDamageHint"><i :style="{width: `${player.summary.damage / damageMax * 100}%`}" /></span>
                    <small>{{ uiText.matchDetails.taken(matchNumber(player.summary.damageTaken)) }}</small>
                  </td>
                  <td><template v-if="rules.cs"><b>{{ matchNumber(player.summary.cs) }}</b><small>{{ uiText.matchDetails.economy(csRate(player), matchNumber(player.summary.gold)) }}</small></template><b v-else>{{matchNumber(player.summary.gold)}}</b></td>
                  <td v-if="rules.vision"><b>{{ matchNumber(player.summary.visionScore) }}</b><small :title="visionDetails(player)">{{ uiText.matchDetails.wardPair(matchNumber(player.summary.wardsPlaced), matchNumber(player.summary.wardsKilled)) }}</small></td>
                  <td><span class="detail-loadout"><GameIcon v-for="slot in 7" :key="slot" kind="items" :id="player.summary.items?.[slot - 1]" :size="19"/></span></td>
                </tr>
              </tbody>
            </table>
          </section>
          <div v-if="index === 0 && totals.length" class="scoreboard-totals" :class="{ 'red-first': group.id === 200 }">
            <div v-for="total in totals" :key="total.key">
              <div><b>{{ matchNumber(total.a) }}</b><span>{{ total.key === 'kills' ? uiText.matchDetails.totalKills : uiText.matchDetails.totalGold }}</span><b>{{ matchNumber(total.b) }}</b></div>
              <span v-if="total.a + total.b > 0" class="total-track" aria-hidden="true"><i :style="{width: `${total.a * 100 / (total.a + total.b)}%`}" /></span>
            </div>
          </div>
        </template>
      </div>
    </div>
    <p v-if="damageLeader != null" class="scoreboard-legend"><b aria-hidden="true">◆</b> {{ uiText.matchDetails.damageLeader }}</p>
  </div>
</template>

<style scoped>
.scoreboard-content { container-type:inline-size; min-width:0; }
.scoreboard-status, .scoreboard-legend { color:var(--muted); font-size:11px; padding:8px 12px; margin:0; }
.scoreboard-legend b, .damage-leader { color:var(--accent); }
.detail-table-scroll { overflow-x:auto; }
.detail-table-scroll:focus-visible { outline-offset:-3px; }
.scoreboard-board { min-width:710px; }
.scoreboard-team { --side-color:var(--ally); }
.scoreboard-team.team-red { --side-color:var(--enemy); }
.scoreboard-team+.scoreboard-team { border-top:4px solid var(--ground); }
.team-summary { display:flex; align-items:baseline; gap:8px; padding:6px 9px; border-inline-start:3px solid var(--side-color); background:color-mix(in srgb,var(--side-color) 12%,var(--raised)); }
.team-summary strong, .team-summary b { font-size:13px; }
.team-summary>span { color:var(--muted); font-size:11px; }
.victory { color:var(--win); }
.defeat { color:var(--loss); }
.objective-line { display:flex; flex-wrap:wrap; column-gap:10px; margin-inline-start:auto; }
table { width:100%; border-collapse:collapse; table-layout:fixed; font-variant-numeric:tabular-nums; }
th,td { padding:4px 7px; text-align:right; vertical-align:middle; border-bottom:1px solid var(--border); }
th { font-size:11px; font-weight:500; color:var(--muted); background:var(--raised); }
td { font-size:12px; }
td>small { display:block; font-size:11px; color:var(--muted); }
th:first-child, td:first-child { padding-inline-start:10px; text-align:left; width:25%; }
th:nth-child(2) { width:15%; }
th:nth-child(3) { width:15%; }
th:nth-child(4) { width:15%; }
th:nth-child(5) { width:8%; }
th:last-child,td:last-child { width:22%; text-align:left; }
.scoreboard-player { display:flex; max-width:100%; align-items:center; gap:5px; min-width:0; }
.scoreboard-hero { position:relative; display:flex; flex:none; }
.scoreboard-hero small { position:absolute; right:-2px; bottom:-2px; background:var(--text); color:var(--surface); padding:0 2px; border-radius:3px; font-size:10px; }
.scoreboard-systems { display:grid; grid-template-columns:repeat(2,14px); gap:2px; flex:none; }
.scoreboard-name { display:grid; min-width:0; }
.scoreboard-name>strong,.scoreboard-name>small { white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
.scoreboard-name>small { color:var(--muted); font-size:11px; }
.target { background:var(--selection); }
.target td:first-child { box-shadow:inset 3px 0 var(--accent); }
.target-label { color:var(--accent); font-size:10px; flex:none; }
.damage-track { display:block; height:3px; margin:3px 0 3px auto; max-width:96px; background:var(--border); }
.damage-track i { display:block; height:100%; background:var(--side-color); }
.detail-loadout { display:flex; gap:2px; align-items:center; }
.detail-loadout :deep(.game-icon:last-child) { margin-inline-start:3px; }
.scoreboard-totals { --first:var(--ally); --second:var(--enemy); display:grid; grid-template-columns:1fr 1fr; gap:20px; padding:7px 10px; background:var(--raised); }
.scoreboard-totals.red-first { --first:var(--enemy); --second:var(--ally); }
.scoreboard-totals>div>div { display:flex; justify-content:space-between; align-items:baseline; font-size:12px; font-variant-numeric:tabular-nums; }
.scoreboard-totals>div>div>span { color:var(--muted); font-size:11px; }
.total-track { display:block; height:4px; background:var(--second); margin-top:3px; }
.total-track i { display:block; height:100%; background:var(--first); }
@container (max-width:740px) {
  .scoreboard-board { min-width:600px; }
  th,td { padding-inline:5px; }
  th:first-child,td:first-child { padding-inline-start:7px; }
  .detail-loadout :deep(.game-icon) { width:15px!important; height:15px!important; }
  .scoreboard-player { gap:3px; }
  .scoreboard-hero :deep(.game-icon) { width:25px!important; height:25px!important; }
}
</style>
