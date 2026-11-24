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

test('mode capabilities cover ordinary ARAM, rotating modes and unknown queues',()=>{
 const mode=(queueId:number,mapId?:number)=>matchMode({queueId,mapId} as Match);
 expect(mode(2400,12)).toMatchObject({cs:false,structures:true,neutralObjectives:false,team:true,timeline:true,map:false});
 expect(mode(450,12)).toMatchObject({runes:true,cs:false,structures:true,neutralObjectives:false});
 for(const queue of [900,1900]) expect(mode(queue,11)).toMatchObject({roles:false,cs:false,neutralObjectives:true,timeline:true,map:true});
 expect(mode(1300,21)).toMatchObject({roles:false,cs:false,neutralObjectives:true,riftObjectives:false,map:false});
 for(const queue of [1700,1710]) expect(mode(queue,30)).toMatchObject({arena:true,team:false,timeline:false,runes:false});
 expect(mode(9999,11)).toMatchObject({known:false,cs:false,vision:false,team:false,timeline:false,map:false});
 expect(mode(2400,11)).toMatchObject({known:false,team:false,timeline:false});
});

 test('team metrics follow the mode rather than a shared Rift template',()=>{
  const mode=(queueId:number,mapId:number)=>matchMode({queueId,mapId} as Match);
  expect(mode(420,11).metrics).toEqual(['kills','gold','damage','visionScore','damageTaken','cs']);
  for(const queue of [450,2400]) expect(mode(queue,12).metrics).toEqual(['damage','damageTaken','kills','gold']);
  expect(mode(900,11).metrics).toEqual(['damage','kills','damageTaken','gold']);
  expect(mode(1300,21).metrics).toEqual(['kills','damage','gold','damageTaken']);
  expect(mode(9999,11).metrics).toEqual([]);
 });
