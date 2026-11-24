import {expect,test} from 'vitest';
import {shardConfiguration,shardName,shardRows} from '../../src/runeShards';
test('flat and scaling health preserve slots and accurate descriptions',()=>{
 expect(shardName(5011,'16.23.1',2)).toBe('固定生命值 +65');
 expect(shardName(5001,'16.23.1',2)).toBe('成长生命值 +10–180（1–18级）');
 expect(shardRows([5007,0,5001],'16.23.1')[2]).toContain(5001);
 expect(shardRows([5007,0,5001],'16.23.1')[1]).toContain(5010);
});
test('patch boundaries select historical values and actual slot choices',()=>{
 expect(shardName(5001,'14.1.1',2)).toContain('15–140');
 expect(shardRows([5007,5002,5003],'14.1.1')[1]).toEqual([5008,5002,5003]);
 expect(shardName(5010,'15.21.1',1)).toBe('移动速度 +2%');
 expect(shardName(5013,'15.21.1',2)).toBe('韧性与减速抗性 +10%');
 expect(shardName(5010,'15.22.1',1)).toBe('移动速度 +2.5%');
 expect(shardName(5013,'26.23',2)).toBe('韧性与减速抗性 +15%');
});
test('unknown, future or conflicting records retain only actual IDs without invented values',()=>{
 for(const version of [undefined,'broken','16.24.1','99.1']){
  expect(shardConfiguration(version)).toBeUndefined();
  expect(shardName(5011,version)).toBe('固定生命值');
  expect(shardRows([5007,0,5001],version)).toEqual([[5007],[],[5001]]);
 }
 expect(shardName(5011,'16.23.1',1)).not.toContain('+');
 expect(shardRows([5007,5011,5001],'16.23.1')[1]).toEqual([5011]);
});
