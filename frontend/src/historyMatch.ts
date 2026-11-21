import { analysisCache } from "./analysisCache";
import type { Match, MatchDetail, MatchPeer } from "../bindings/leaguepingo/internal/core/models";
import modes from "../../internal/core/scout_modes.json";
import { roleOrder, teamMetric } from "./analysisData";
import { gameAsset } from "./gameAssets";
import { isArena, isRemake } from "./matchStats";
import { uiText } from "./uiText";

export function historyHeroName(id: number) {
  const hero = gameAsset("champions", id);
  return hero?.title || hero?.name || uiText.common.unknown;
}

export function historyPosition(match: Match) {
  const mode = modes.find(mode => mode.queueId === match.queueId);
  return mode?.roles && roleOrder.includes(match.position) ? match.position : undefined;
}

export function historyRoster(match: Match, connectionId?: number) {
  const known = connectionId == null ? [] : analysisCache.roster(connectionId, match.gameId);
  const peers = (match.roster ?? []).map(peer => {
    if (peer.displayName) return peer;
    const named = known?.find(player => player.participantId === peer.participantId && player.teamId === peer.teamId && player.summary.championId === peer.championId);
    return named?.displayName && !named.displayName.includes("身份未提供") ? { ...peer, displayName: named.displayName } : peer;
  });
  const targets = peers.filter(peer => peer.target);
  const team = targets[0]?.teamId;
  if (isArena(match) || targets.length !== 1 || (team !== 100 && team !== 200)) return [];
  return [team, team === 100 ? 200 : 100]
    .map((teamId, index) => ({ teamId, opposing: index === 1, peers: peers.filter(peer => peer.teamId === teamId) }));
}

export function peerShortName(peer: MatchPeer) {
  return peer.displayName?.split("#")[0]?.trim() || uiText.matchDetails.missingPlayerName;
}

export function hasFullScoreboard(detail: MatchDetail) {
  const players = detail.players ?? [];
  return !detail.partial && players.length === 10
    && new Set(players.map(player => player.participantId)).size === 10
    && [100, 200].every(id => players.filter(player => player.teamId === id).length === 5);
}

export function teamComparison(detail: MatchDetail, order: readonly number[]) {
  if (order.length !== 2 || !hasFullScoreboard(detail) || detail.players?.some(p => isRemake(p.summary))) return [];
  return (["kills", "gold"] as const).flatMap(key => {
    const { teams } = teamMetric(detail, key);
    const a = teams.find(team => team.teamId === order[0])?.total;
    const b = teams.find(team => team.teamId === order[1])?.total;
    return a != null && b != null ? [{ key, a, b }] : [];
  });
}

export function matchDamageLeader(detail: MatchDetail) {
  const players = detail.players ?? [];
  if (!hasFullScoreboard(detail) || players.some(p => p.summary.damage == null || isRemake(p.summary))) return null;
  const max = Math.max(...players.map(p => p.summary.damage ?? 0));
  if (max <= 0) return null;
  const leaders = players.filter(p => p.summary.damage === max);
  return leaders.length === 1 ? leaders[0]!.participantId : null;
}

export function matchNumber(value?: number | null) {
  return value == null ? "—" : value.toLocaleString("zh-CN");
}
