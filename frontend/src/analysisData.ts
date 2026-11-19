import type {Match, MatchDetail, MatchPlayer, MatchTimeline, Participant, TimelineFrame} from '../bindings/leaguepingo/internal/core/models';
import {isRemake,isArena,sampleStats} from './matchStats';

export const roleOrder=['TOP','JUNGLE','MIDDLE','BOTTOM','UTILITY'];
export const roleLabels:Record<string,string>={TOP:'上路',JUNGLE:'打野',MIDDLE:'中路',BOTTOM:'下路',UTILITY:'辅助'};
export function average(values:(number|null|undefined)[]){const valid=values.filter((v):v is number=>v!=null && Number.isFinite(v));return valid.length?valid.reduce((s,n)=>s+n,0)/valid.length:null;}
export function perMinute(value:number|null|undefined,seconds:number|null|undefined){return value!=null && seconds!=null && seconds>0?value*60/seconds:null;}
export function overviewData(matches:Match[]){
 const valid=matches.filter(m=>!isRemake(m));
 const kda=valid.filter(m=>m.kills!=null && m.deaths!=null && m.assists!=null);
 const positions=roleOrder.map(role=>({role,count:valid.filter(m=>m.position===role && (m.mapId===11 || [400,420,430,440,490].includes(m.queueId))).length}));
 const knownPositions=positions.reduce((s,p)=>s+p.count,0);
 const hours=Array.from({length:24},(_,hour)=>({hour,count:matches.filter(m=>m.playedAt && !Number.isNaN(Date.parse(m.playedAt)) && new Date(m.playedAt).getHours()===hour).length}));
 return {...sampleStats(matches),arenaOnly:matches.length>0 && matches.every(isArena),average:[average(kda.map(m=>m.kills)),average(kda.map(m=>m.deaths)),average(kda.map(m=>m.assists))],participation:average(valid.map(m=>m.participation)),participationGames:valid.filter(m=>m.participation!=null).length,positions,knownPositions,hours,timedGames:hours.reduce((s,p)=>s+p.count,0)};
}
export function heroRows(matches:Match[]){return [...new Set(matches.map(m=>m.championId).filter(id=>id>0))].map(id=>{const games=matches.filter(m=>m.championId===id);return {id,...overviewData(games),csRate:average(games.map(m=>perMinute(m.cs,m.duration)))};}).sort((a,b)=>b.games-a.games || a.id-b.id);}
export function recentSeven(matches:Match[],fetchedAt:string|null|undefined){const cutoff=fetchedAt?Date.parse(fetchedAt):NaN;return Number.isFinite(cutoff)?matches.filter(m=>m.playedAt && Date.parse(m.playedAt)<=cutoff && Date.parse(m.playedAt)>=cutoff-7*86400000):[];}
export function masteryData(player:Participant){const values=player.profile?.masteries ?? (player.scout?.champions??[]).flatMap(h=>h.mastery?[h.mastery]:[]);return [...values].sort((a,b)=>b.points-a.points || b.level-a.level || a.championId-b.championId);}
export function matchHighlight(match: Match) {
  if (isRemake(match)) return null;
  if ((match.pentaKills ?? 0) > 0) return "penta";
  if ((match.quadraKills ?? 0) > 0) return "quadra";
  if ((match.tripleKills ?? 0) > 0) return "triple";
  return match.deaths === 0 && match.kills != null && match.assists != null && match.kills + match.assists > 0
    ? "deathless" : null;
}
export const teamMetrics=[{key:'kills',label:'击杀'},{key:'gold',label:'金币'},{key:'damage',label:'英雄伤害'},{key:'visionScore',label:'视野得分'},{key:'damageTaken',label:'承受伤害'},{key:'cs',label:'补刀'}] as const;
export type TeamMetricKey=typeof teamMetrics[number]['key'];
export function teamMetric(detail:MatchDetail,key:TeamMetricKey){
 const teams=[100,200].map(teamId=>{const players=(detail.players??[]).filter(p=>p.teamId===teamId);const total=!detail.partial && players.length===5 && players.every(p=>p.summary[key]!=null)?players.reduce((s,p)=>s+(p.summary[key]??0),0):null;return {teamId,players,total};});
 const max=Math.max(0,...(detail.players??[]).map(p=>p.summary[key]??0));return {teams,max};
}
export function teamGold(frame:TimelineFrame,detail:MatchDetail,teamId:number){const ids=(detail.players??[]).filter(p=>p.teamId===teamId).map(p=>p.participantId);if(detail.partial || ids.length!==5)return null;const values=ids.map(id=>(frame.players??[]).find(p=>p.participantId===id)?.gold);return values.every(v=>v!=null)?values.reduce<number>((s,n)=>s+(n??0),0):null;}
export function targetPlayer(detail:MatchDetail|null|undefined):MatchPlayer|undefined{return detail?.players?.find(p=>p.target);}
export function clock(ms:number){const seconds=Math.floor(Math.max(0,ms)/1000);return `${Math.floor(seconds/60)}:${String(seconds%60).padStart(2,'0')}`;}
