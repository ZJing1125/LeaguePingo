import type { Snapshot } from "../bindings/leaguepingo/internal/core/models";

export function hasCurrentMatch(snapshot: Snapshot | null | undefined) {
  return !!snapshot &&
    ["Lobby", "Matchmaking", "ReadyCheck", "ChampSelect", "GameStart", "InProgress", "Reconnect"].includes(snapshot.phase) &&
    (snapshot.connected || !!snapshot.participants?.length);
}
