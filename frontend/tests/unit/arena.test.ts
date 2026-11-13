import { describe, expect, it } from 'vitest';
import type { Match } from '../../bindings/leaguepingo/internal/core/models';
import { arenaResult, isRemake, matchTone, sampleStats } from '../../src/matchStats';
import { outcome } from '../../src/presentation';

const match = (placement?: number, teamCount?: number, queueId = 1700) => ({ queueId, mapId: 30, win: true, arena: placement == null ? undefined : { placement, teamCount } }) as Match;

describe('Arena result semantics', () => {
  it('keeps first-place, top-half and ordinary win denominators independent', () => {
    const data = sampleStats([match(1, 4), match(3, 4), match(4, 8), match(2), match(), {queueId: 420, win: false} as Match]);
    expect(data.outcomeGames).toBe(1);
    expect(data.wins).toBe(0);
    expect(data.arena?.placementGames).toBe(4);
    expect(data.arena?.averagePlace).toBe(2.5);
    expect(data.arena?.firstPlaceRate).toBe(25);
    expect(data.arena?.topHalfGames).toBe(3);
    expect(data.arena?.topHalfRate).toBeCloseTo(200/3);
  });
  it('leaves missing/invalid results unknown and never uses win for first place', () => {
    for (const value of [match(), match(0), match(-1), match(33), match(5, 4), match(1, 0)]) {
      expect(arenaResult(value)).toBeNull();
      expect(sampleStats([value]).arena?.firstPlaceRate).toBeNull();
      expect(matchTone(value)).toBe('unknown');
      expect(outcome(value)).toBe('名次待确认');
    }
    expect(sampleStats([match(2, 4)]).arena?.firstPlaceRate).toBe(0);
    expect(sampleStats([match(1)]).arena?.topHalfRate).toBeNull();
  });
  it('labels both exact queues by placement and never marks a short Arena as remake', () => {
    expect(outcome(match(2, 4, 1710))).toBe('第 2 名');
    expect(matchTone(match(1))).toBe('won');
    expect(matchTone(match(4))).toBe('unknown');
    expect(isRemake({...match(1),earlyEnd:true,duration:60})).toBe(false);
    expect(arenaResult({...match(1),mapId:11})).toBeNull();
  });
});
