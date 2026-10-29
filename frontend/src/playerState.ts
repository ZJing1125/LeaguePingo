import type { Participant } from "../bindings/leaguepingo/internal/core/models";

export function playerTabId(key: string) {
  return `player-tab-${encodeURIComponent(key)}`;
}

export function operationKey(player: Participant) {
  return `${player.connectionId ?? 0}:${player.key}`;
}

export function hasPlayerName(player: Participant) {
  return (
    !!player.displayName?.trim() &&
    !["玩家资料加载中", "玩家名称暂不可用", "身份暂不可用"].includes(
      player.displayName,
    )
  );
}

export function newestPlayer(
  current: Participant | null,
  retained: Participant | null,
) {
  if (!retained) return current;
  if (!current) return retained;
  if (current.key !== retained.key || current.puuid !== retained.puuid)
    return current;
  if ((current.connectionId ?? 0) !== (retained.connectionId ?? 0))
    return (current.connectionId ?? 0) > (retained.connectionId ?? 0)
      ? current
      : retained;
  const currentTime = Date.parse(current.historyMeta?.fetchedAt ?? "") || 0;
  const retainedTime = Date.parse(retained.historyMeta?.fetchedAt ?? "") || 0;
  const history = currentTime >= retainedTime ? current : retained;
  const currentProfileTime = Date.parse(current.profile?.updatedAt ?? "") || 0;
  const retainedProfileTime =
    Date.parse(retained.profile?.updatedAt ?? "") || 0;
  const profile =
    (currentProfileTime >= retainedProfileTime
      ? current.profile
      : retained.profile) ??
    current.profile ??
    retained.profile;
  const displayName = hasPlayerName(current)
    ? current.displayName
    : hasPlayerName(retained)
      ? retained.displayName
      : history.displayName;
  const currentScoutTime = Date.parse(current.scout?.fetchedAt ?? "") || 0;
  const retainedScoutTime = Date.parse(retained.scout?.fetchedAt ?? "") || 0;
  const scout = (currentScoutTime >= retainedScoutTime ? current.scout : retained.scout) ?? current.scout ?? retained.scout;
  const currentChecked = Date.parse(current.historyMeta?.checkedAt ?? current.historyMeta?.fetchedAt ?? '') || 0;
  const retainedChecked = Date.parse(retained.historyMeta?.checkedAt ?? retained.historyMeta?.fetchedAt ?? '') || 0;
  const service = currentChecked >= retainedChecked ? current.historyMeta : retained.historyMeta;
  const merged = { ...history, historyMeta: { ...history.historyMeta, service: service?.service, checkedAt: service?.checkedAt }, displayName, scout, ...(current.championState !== undefined ? { championId: current.championId, championState: current.championState } : {}) };
  return profile
    ? { ...merged, profile, privacyState: profile.privacyState }
    : merged;
}
