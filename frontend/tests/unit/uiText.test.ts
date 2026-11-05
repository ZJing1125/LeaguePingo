import { describe, expect, it } from 'vitest';
import { toDisplayString } from 'vue';
import { uiText } from '../../src/uiText';

describe('central copy interpolation preserves its original rendering semantics', () => {
  it('uses Vue display conversion for merged template text, including absent and structured values', () => {
    for (const value of [undefined, null, 0, false, ['a', 'b'], { count: 2 }, '<b>text</b>']) {
      expect(uiText.common.games(value)).toBe(toDisplayString(value) + ' 场');
    }
  });
  it('retains native template coercion for JavaScript formatters', () => {
    expect(uiText.common.seconds(undefined)).toBe('undefined 秒');
    expect(uiText.common.seconds(null)).toBe('null 秒');
    expect(() => uiText.common.seconds(Symbol('test'))).toThrow(TypeError);
  });
  it('preserves spacing and argument order in the multi-field scout summary', () => {
    expect(uiText.playerCardData.cutoffHint('单双排', '近30天', 12, 10, '10/28', '本赛季'))
      .toBe('单双排 · 近30天，12 场；KDA 有效 10 场；统计截至 10/28；本赛季');
    expect(uiText.matchDetails.wards(0, undefined)).toBe('插眼 0 · 排眼 ');
  });
});
