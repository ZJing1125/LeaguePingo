import { describe, expect, it } from 'vitest';
import type { PlayerAnalysis } from '../../bindings/leaguepingo/internal/core/models';
import { analysisSummary, analysisTitle } from '../../src/playerAnalysis';

function analysis(patch: Partial<PlayerAnalysis> = {}): PlayerAnalysis {
  return { version: 1, state: 'ready', grade: 'B', score: 6.4, games: 12, scoredGames: 8,
    comparedGames: 8, basis: 'comparative', provisional: false, confidence: 'medium',
    metrics: [{key: 'damage', score: 7.2, games: 8}], ...patch };
}

describe('recent player analysis presentation', () => {
  it('keeps unknown and a real zero distinct', () => {
    expect(analysisSummary(null)).toBe('资料不足');
    const title = analysisTitle(analysis({score: 0, grade: 'D'}));
    expect(title).toContain('0.0 / 10');
    expect(title).not.toContain('D ·');
  });
  it('shows evidence and its valid sample without inventing a live rating', () => {
    const title = analysisTitle(analysis());
    expect(title).toContain('本模式 · 近30天');
    expect(title).toContain('评分有效 8 / 已读 12 场');
    expect(title).toContain('输出较好');
    expect(title).toContain('中等置信度');
  });
  it('labels basic or small-sample grades explicitly', () => {
    const title = analysisTitle(analysis({basis: 'basic', provisional: true}));
    expect(title).toContain('基础评级');
    expect(title).toContain('暂定');
    expect(analysisSummary(analysis({basis: 'basic'}))).toBe('基础评级');
  });
  it('ignores unobserved and unknown metric keys when explaining a grade', () => {
    const a = analysis({metrics: [{key: 'damage', score: 10, games: 0}, {key: 'unrecognized', score: 10, games: 8}]});
    expect(analysisSummary(a)).toBe('表现较均衡');
  });
});
