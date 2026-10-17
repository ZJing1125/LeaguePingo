import type { Participant } from "../bindings/leaguepingo/internal/core/models";

export function playerTabId(key: string) {
  return `player-tab-${encodeURIComponent(key)}`;
}

export function operationKey(player: Participant) {
  return `${player.connectionId ?? 0}:${player.key}`;
}

export function newestPlayer(
  current: Participant | null,
  retained: Participant | null,
) {
  if (!retained) return current;
  if (!current) return retained;
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
  return profile
    ? { ...history, profile, privacyState: profile.privacyState }
    : history;
}
