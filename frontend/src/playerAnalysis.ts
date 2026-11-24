import type { PlayerAnalysis, TeamRating } from '../bindings/leaguepingo/internal/core/models';
import { uiText } from './uiText';
import { rankedMode } from './scoutStats';

export function analysisSummary(a?: PlayerAnalysis | null) {
  if (a?.score == null) return a?.state==='empty' ? uiText.playerAnalysis.noHistory : uiText.playerAnalysis.insufficient;
 if (a.version>=2 && a.tags?.length) return a.tags[0];
  if (a.basis === 'basic') return uiText.playerAnalysis.basic;
  const labels = uiText.playerAnalysis.metrics as Record<string,string>;
  const known = (a.metrics ?? []).filter(m => m.games > 0 && m.key !== 'outcome' && labels[m.key]);
  const highest = [...known].sort((x,y) => y.score-x.score)[0];
  const lowest = [...known].sort((x,y) => x.score-y.score)[0];
  if (highest && highest.score >= (a.version>=2?72:6.5) && labels[highest.key]) return uiText.playerAnalysis.strong(labels[highest.key]);
  if (lowest && lowest.score <= (a.version>=2?48:3.5) && labels[lowest.key]) return uiText.playerAnalysis.weak(labels[lowest.key]);
  return uiText.playerAnalysis.balanced;
}

export function analysisTitle(a?: PlayerAnalysis | null, rating?: TeamRating | null) {
  if (a?.score == null) return uiText.playerAnalysis.insufficientHint;
  const text = uiText.playerAnalysis;
  const basis = ({comparative:text.comparative,mixed:text.mixed,basic:text.basic,placement:text.placement} as Record<string,string>)[a.basis] ?? text.basic;
  const scope = rating ? text.modeScope(rankedMode(rating.queueId)) : text.scope;
  const standing = rating && rating.place > 0 ? `${rating.label} · ${text.teamPlaceHint(rating.place, rating.rated)} · ` : rating ? `${rating.label} · ` : '';
  return `${scope} · ${standing}${a.score.toFixed(1)} / ${a.version>=2?100:10} · ${text.samples(a.scoredGames,a.games)} · ${basis} · ${a.provisional ? text.provisional : text.confidence[a.confidence as keyof typeof text.confidence] ?? text.confidence.low} · ${analysisSummary(a)}`;
}
