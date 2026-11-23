import { expect, test } from 'vitest';
import { matchMode } from '../../src/matchMode';
import type { Match } from '../../bindings/leaguepingo/internal/core/models';

test('mixed history resolves each match independently and rejects conflicting maps', () => {
 const mode = (queueId: number, mapId?: number) => matchMode({queueId,mapId} as Match);
 expect(mode(2400,12)).toMatchObject({augments:true,runes:false,roles:false,vision:false});
 expect(mode(420,11)).toMatchObject({augments:false,runes:true,roles:true,vision:true});
 expect(mode(450,12)).toMatchObject({augments:false,runes:true,roles:false,vision:false});
 expect(mode(2400,11).known).toBe(false);
 expect(mode(9999).runes).toBe(false);
 expect(mode(2400,12).augments).toBe(true);
});
