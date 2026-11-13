// Synthetic report for existing layout tests; no production fetcher or user data.
function stats(matches) {
  const outcomes=matches.filter(m=>m.win!=null), complete=matches.filter(m=>m.kills!=null&&m.deaths!=null&&m.assists!=null), wins=outcomes.filter(m=>m.win).length;
  const sums=complete.reduce((a,m)=>a.map((v,i)=>v+[m.kills,m.deaths,m.assists][i]),[0,0,0]);
  return {games:matches.length,wins,losses:outcomes.length-wins,outcomeGames:outcomes.length,kdaGames:complete.length,winRate:outcomes.length?wins*100/outcomes.length:null,kda:complete.length?(sums[0]+sums[2])/Math.max(1,sums[1]):null};
}
function scoutFixture(player) {
 const all=player.history??[], anchor=Date.parse(player.historyMeta?.fetchedAt??'2026-10-21T12:00:00Z');
 const select=(q)=>all.filter(m=>m.queueId===q), ids=[...new Set([...all.map(m=>m.championId),player.championId].filter(Boolean))];
 const complete=Number.isFinite(anchor);
 return {state:'ready',fetchedAt:player.historyMeta?.fetchedAt??'2026-10-21T12:00:00Z',season:{name:'合成赛季',start:'2026-01-01T00:00:00Z',end:'2027-01-01T00:00:00Z'},complete,complete12:complete,complete30:complete,read:all.length,
 champions:ids.map(id=>({championId:id,games:all.filter(m=>m.championId===id).length,mastery:{championId:id,level:20,points:200000},queues:[420,440].map(queueId=>{const selected=select(queueId).filter(m=>m.championId===id);const valid=selected.filter(m=>m.kills!=null&&m.deaths!=null&&m.assists!=null);return{queueId,stats:stats(selected),average:valid.length?['kills','deaths','assists'].map(k=>valid.reduce((n,m)=>n+m[k],0)/valid.length):[]};})})),
 queues:[420,440].map(queueId=>{const selected=select(queueId);const period=h=>selected.filter(m=>{const d=Date.parse(m.playedAt);return d<=anchor&&d>=anchor-h*3600000});const month=period(720);return {queueId,analysis:{version:1,state:"ready",grade:"B",score:6.4,games:month.length,scoredGames:month.length,comparedGames:month.length,basis:"comparative",provisional:false,confidence:"medium",metrics:[{key:"damage",score:7.2,games:month.length},{key:"survival",score:4.8,games:month.length}]},season:stats(selected),twelve:stats(period(12)),month:stats(month),positions:['TOP','JUNGLE','MIDDLE','BOTTOM','UTILITY'].map(position=>({position,games:month.filter(m=>m.position===position).length})).filter(p=>p.games)};})};
}
module.exports={scoutFixture,stats};
