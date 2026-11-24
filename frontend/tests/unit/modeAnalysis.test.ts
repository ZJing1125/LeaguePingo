import { expect, test } from 'vitest';
import { heroRows, overviewData } from '../../src/analysisData';
import { match } from './fixtures';

test('non-lane samples replace CS rates and ignore accidental position fields', () => {
  for (const [queue, map] of [[450,12],[2400,12],[900,11],[1900,11],[1300,21],[9999,11],[2400,11]]) {
    const sample = [match(1, queue, {mapId:map,position:'TOP',duration:600,cs:100,damage:12000,participation:70})];
    expect(overviewData(sample)).toMatchObject({rolesApplicable:false,knownPositions:0});
    expect(heroRows(sample)[0]).toMatchObject({rolesApplicable:false,csRate:null,damageRate:1200});
    if (queue === 9999 || queue === 2400 && map === 11) {
      expect(overviewData(sample)).toMatchObject({participation:null,participationGames:0});
    }
  }
});

test('mixed samples count CS and positions only from eligible matches', () => {
  const sample = [
    match(1,420,{mapId:11,position:'MIDDLE',duration:600,cs:60,damage:6000}),
    match(2,2400,{mapId:12,position:'TOP',duration:600,cs:120,damage:18000}),
  ];
  expect(overviewData(sample)).toMatchObject({rolesApplicable:true,knownPositions:1});
  expect(heroRows(sample)[0]).toMatchObject({csRate:6,damageRate:1200});
});
