const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
const path=require('node:path');
const {chromium}=require('playwright');
const {mockWindowRuntime,normalizeServiceRequest}=require('./window-runtime.cjs');
const {saveUIArtifact}=require('./ui-artifacts.cjs');
const {makePlayer,makeDetail,makeTimeline,now}=require('./analysis-fixture.cjs');
const {verifyKeyboardAccess}=require('./keyboard-access.cjs');
async function verifyIncompleteDetails(page, calls, publish, snapshot, releaseDetail, shot) {
 const home = page.locator('.home-workspace');
 const rows = home.locator('.home-match .match-main');
 const dialog = page.getByRole('dialog',{name:'对局详情',exact:true});
 const detailCalls = () => calls.filter(name => name === 'MatchDetail').length;
 const baseline = detailCalls();
 await rows.nth(5).click();
 await dialog.getByText('正在读取双方详情…',{exact:true}).waitFor();
 assert.equal(await dialog.locator('.saved-roster .roster-peer').count(),10);
 await dialog.press('Escape');
 await dialog.waitFor({state:'detached'});
 await rows.nth(5).click();
 await dialog.getByText('正在读取双方详情…',{exact:true}).waitFor();
 releaseDetail();
 await dialog.locator('tr.target').waitFor();
 assert.equal(detailCalls(),baseline+1,'reopening an in-flight match read duplicated it');
 await dialog.press('Escape');
 await rows.nth(2).click();
 await dialog.getByText('已取得 6 / 10 人资料',{exact:true}).waitFor();
 assert.equal(await dialog.locator('tbody tr').count(),6);
 assert.equal(await dialog.locator('.scoreboard-totals, .damage-leader').count(),0,'partial data advertised whole-match comparisons');
 assert.equal(await dialog.locator('.saved-roster .roster-peer').count(),10,'partial detail discarded prior roster');
 assert.match(await dialog.locator('.team-summary').first().innerText(),/本方.*红方/s,'red-side target did not stay first');
 await shot('partial-scoreboard');
 await dialog.press('Escape');

 snapshot.connected = false;
 await publish();
 await home.locator('.connection-guide').waitFor();
 await rows.nth(3).click();
 await dialog.getByText('已保留个人表现和阵容',{exact:true}).waitFor();
 assert.equal(await dialog.locator('.saved-roster .roster-peer').count(),10);
 assert.match(await dialog.locator('.saved-roster').innerText(),/姓名未提供/,'anonymous roster displayed a champion as player name');
 assert.equal(await dialog.locator('.personal-content .summary-items .game-icon').count(),7);
 assert.equal(await dialog.locator('.target-label').count(),0);
 await shot('offline-roster');
 await dialog.press('Escape');

 await rows.nth(4).click();
 await dialog.getByText('仅保留个人表现',{exact:true}).waitFor();
 assert.equal(await dialog.locator('.saved-roster').count(),0);
 await dialog.press('Escape');
 assert.equal(detailCalls(),baseline+2,'offline browsing requested a match or partial detail was fetched twice');
}

async function verifyPlayerHistoryJumps(page, lookups, calls) {
 const home = page.locator('.home-workspace');
 const nav = page.getByRole('navigation',{name:'工作区'});
 const cards = home.locator('.home-match');
 const details = () => calls.filter(name => name === 'MatchDetail').length;
 const beforeDetails = details();
 const beforeLookups = lookups.length;
 const teammate = '暮色旅人#演示';
 const opponent = '秋日序曲#演示';

 const firstCard = cards.first();
 assert.equal(await firstCard.locator('.match-mini-roster button[tabindex="0"]').count(),1);
 await firstCard.locator('.match-main').press('Tab');
 await page.keyboard.press('ArrowDown');
 assert.match(await page.evaluate(()=>document.activeElement?.getAttribute('aria-label') || ''),/暮色旅人#演示/);
 await cards.first().getByRole('button',{name:`查看 ${teammate} 的战绩`,exact:true}).locator('.game-icon').click();
 const teammateTab = page.getByRole('tab',{name:teammate,exact:true});
 await teammateTab.waitFor();
 assert.equal(await teammateTab.getAttribute('aria-selected'),'true');
 assert.equal(details(),beforeDetails,'clicking the roster avatar opened match details');
 assert.deepEqual(lookups.slice(beforeLookups),[teammate]);

 await nav.getByRole('button',{name:'首页',exact:true}).click();
 assert.equal(await cards.nth(3).locator('.roster-peer button, button.roster-peer').count(),0,'anonymous records rendered history links');
 await cards.first().locator('.match-main').click();
 const dialog = page.getByRole('dialog',{name:'对局详情',exact:true});
 const opponentAction = dialog.getByRole('button',{name:`查看 ${opponent} 的战绩`,exact:true});
 await opponentAction.press('Enter');
 await dialog.waitFor({state:'detached'});
 const opponentTab = page.getByRole('tab',{name:opponent,exact:true});
 await opponentTab.waitFor();
 assert.equal(await opponentTab.getAttribute('aria-selected'),'true');
 await page.waitForFunction(name=>document.activeElement?.textContent?.includes(name),opponent);
 assert.deepEqual(lookups.slice(beforeLookups),[teammate,opponent]);

 await nav.getByRole('button',{name:'首页',exact:true}).click();
 await cards.first().getByRole('button',{name:`查看 ${teammate} 的战绩`,exact:true}).click();
 await teammateTab.waitFor();
 assert.equal(await teammateTab.getAttribute('aria-selected'),'true');
 assert.equal(lookups.length,beforeLookups+2,'existing player tab was queried again');
 await nav.getByRole('button',{name:'首页',exact:true}).click();
}

async function main(){
 const browser=await chromium.launch({headless:true,executablePath:process.env.BROWSER_EXECUTABLE});
 try{
  const page=await browser.newPage({viewport:{width:1280,height:900}});page.setDefaultTimeout(15000);
  const calls=[],lookups=[],errors=[],external=[];page.on('pageerror',e=>errors.push(e.message));const desktop=mockWindowRuntime(page);
  const binding=await fs.readFile(path.join(__dirname,'../bindings/leaguepingo/appservice.ts'),'utf8');
  const methods=Object.fromEntries([...binding.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g)].map(m=>[m[2],m[1]]));
  const player=makePlayer();
  let testQueue = 0;
  const detailFor = gameId => {
   const detail = makeDetail(gameId);
   if (testQueue) for (const p of detail.players) Object.assign(p.summary, {queueId:testQueue,mapId:testQueue===420?11:12,gameVersion:'16.23.1',augmentIds:[999999],runeShards:[5007,5011,5001]});
   return detail;
  };
  let failTimeline=false;
  let releaseDetail;
  const delayedDetail = new Promise(resolve=>{releaseDetail=resolve;});
  player.history[3].roster.forEach(peer => { peer.displayName = ""; });
  delete player.history[4].roster;
  player.history[2].roster.forEach(peer => { peer.teamId = 300 - peer.teamId; });
  const snapshot={connectionId:1,selfKey:player.key,generation:1,connected:true,phase:'None',stage:'idle',updatedAt:now,participants:[]};
  await page.route('**/*',r=>{external.push(r.request().url());return r.abort()});
  await page.route('http://127.0.0.1:43208/**',async route=>{
   const url=new URL(route.request().url());
   if(url.pathname==='/wails/custom.js')return route.fulfill({contentType:'text/javascript',body:''});
   if(url.pathname==='/wails/runtime'){
    const req=normalizeServiceRequest(route.request().postDataJSON());if(await desktop.handle(route,req))return;
    const name=methods[req.args?.methodID],args=req.args?.args??[];calls.push(name);
    let result=null;
    if (name === 'GuideStatus') result={revision:0,status:'idle',query:{},base:null,matchup:null};
    else if (name === 'PostGameStatus') result={revision:0,returnEnabled:false,queueEnabled:false,delaySeconds:10,status:'off',dueAt:'0001-01-01T00:00:00Z'};
    else if(name==='AutoAcceptStatus')result={revision:0,enabled:false,delaySeconds:1,status:'off',dueAt:'0001-01-01T00:00:00Z'};
    else if(name==='Snapshot')result=snapshot;
    else if(name==='Info')result={version:'synthetic',commit:'analysis-test',requests:{},shortcut:'Ctrl+Shift+P',holdEnabled:true,holdKey:'Backquote'};
    else if(name==='OverlayStatus')result={revision:0,visible:false,sidebarEnabled:true,sidebarWidth:320};
    else if(name==='Lookup'){
     const id=args[0];
     lookups.push(id);
     result=id===player.displayName?player:{...player,key:id,puuid:id,displayName:id};
    }
    else if(['CurrentPlayer','RefreshData','EnsureProfile'].includes(name))result=player;
    else if(name==='HistoryPage'){assert.equal(args[2],20,'analysis fixture must not scan beyond its one bounded warm/page read');result={matches:Array.from({length:20},(_,i)=>({...player.history[i],gameId:70021+i,playedAt:new Date(Date.parse(now)-(20+i)*4*3600000).toISOString()})),meta:{fetchedAt:now,returned:20,parsed:20,excluded:0},offset:args[2],nextOffset:args[2]+20,hasMore:false};}
    else if(name==='MatchDetail'){
     if(args[2]===70006)await delayedDetail;
     result=detailFor(args[2]);
     if(args[2]===70003){
      result.players=result.players.filter(p=>![4,5,9,10].includes(p.participantId));
      result.players.forEach(p=>{p.teamId=300-p.teamId;});
      result.partial=true;
     }
    }
    else if(name==='MatchTimeline'){
     if(failTimeline)return route.fulfill({status:500,json:{message:'本场时间线暂未取得'}});
     result={...makeTimeline(args[2]),detail:detailFor(args[2])};
    }else throw Error(`Unexpected analysis service ${name}`);
    return route.fulfill({json:result});
   }
   const file=url.pathname==='/'?'index.html':url.pathname.slice(1);if(file.includes('..'))return route.abort();
   return route.fulfill({body:await fs.readFile(path.join(__dirname,'../dist',file)),contentType:file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':file.endsWith('.png')?'image/png':file.endsWith('.jpg')?'image/jpeg':'text/html'});
  });
  const shot=async name=>{
   if(!process.env.UI_SCREENSHOT_PREFIX)return;
   await page.waitForFunction(()=>[...document.querySelectorAll('img')].filter(img=>img.getClientRects().length).every(img=>img.complete&&img.naturalWidth>0));
   await page.evaluate(()=>document.fonts.ready);
   await saveUIArtifact(page,process.env.UI_SCREENSHOT_PREFIX,name,true);
  };
  await page.goto('http://127.0.0.1:43208/');
  await page.locator('.home-account h2').filter({hasText:player.displayName}).waitFor();
  assert((await page.locator('.home-workspace .playstyle-panel .style-roles').innerText()).includes('100%'),'home position/activity panel lost normalized match positions');
  assert((await page.locator('.home-workspace .overview-performance .performance-roles h3').innerText()).includes('20 场已记录'),'home recent analysis lost its position sample');
  const rankHistory=page.locator('.home-ranks .rank-history').first();
  await rankHistory.locator('summary').click();
  const rankHistoryText=await rankHistory.innerText();
  assert(rankHistoryText.includes('已记录历史最高')&&rankHistoryText.includes('上赛季最高'),'rank history did not distinguish recorded and previous-season highs');
  assert(rankHistoryText.includes('大师'),'recorded historical high did not use previous-season highest');
  await rankHistory.locator('summary').click();
  await shot('overview-dark');
  await page.getByLabel('玩家完整 ID',{exact:true}).fill(player.displayName);
  await page.getByRole('button',{name:'查询战绩',exact:true}).click();
  await page.locator('.players-workspace .match-card').first().waitFor();
  assert((await page.locator('.players-workspace .overview-performance .performance-roles h3').textContent()).includes('20 场已记录'),'queried-player analysis lost normalized match positions');
  await shot('history-dark');
  assert.equal(calls.filter(n=>n==='MatchDetail').length,0,'list prefetched details');
  assert.equal(await page.locator('.players-workspace .match-mini-roster').first().locator('.roster-peer').count(),10);
  await page.locator('.players-workspace .match-main').first().click();
  await page.locator('.full-match-detail tr.target').waitFor();
  await shot('scoreboard-dark');
  if(process.env.ANALYSIS_BASELINE==='1'){assert.deepEqual(errors,[]);return;}
  assert.equal(calls.filter(n=>n==='MatchDetail').length,1,'opening one game must load exactly that game detail');
  assert.equal(await page.locator('.full-match-detail tbody tr').count(),10);
  const totals = page.locator('.scoreboard-totals');
  assert.match(await totals.innerText(),/31.*总击杀.*21/s);
  assert.equal(await page.locator('.damage-leader').count(),1);
  assert.equal(await page.locator('.detail-loadout').first().locator('.game-icon').count(),7);
  // Existing idle ranked warm-up may finish while switching analysis tabs.
  // Its single bounded HistoryPage is checked above; all other calls stay fixed.
  const analysisCalls=()=>calls.filter(n=>n!=='HistoryPage').length;
  const before=analysisCalls();
  await page.getByRole('tab',{name:'团队分析',exact:true}).click();
  await page.locator('.team-analysis').waitFor();
  assert.equal(await page.locator('.team-total-row').count(),6);
  assert.equal(analysisCalls(),before,'team analysis fetched unrequested data');
  assert.equal(await page.locator('.metric-player').count(),10);
  await page.getByRole('button',{name:/视野得分/}).click();
  assert.equal(analysisCalls(),before,'changing metric fetched data');
  await shot('team-dark');
  await page.getByRole('tab',{name:'时间线',exact:true}).click();

  await page.locator('.match-timeline .timeline-map').waitFor();
  assert.equal(calls.filter(n=>n==='MatchTimeline').length,1);
  await page.getByLabel('对局时间').fill('480000');
  await page.locator('.timeline-event').first().click();
  await shot('timeline-dark');
  await page.locator('.players-workspace .history-scroll').evaluate(el=>el.scrollTop+=340);
  const tabsBox=await page.locator('.analysis-tabs').boundingBox(),scrollBox=await page.locator('.players-workspace .history-scroll').boundingBox();
  assert(Math.abs(tabsBox.y-scrollBox.y)<2,'long-game tabs did not remain reachable');
  await shot('timeline-economy-dark');
  await page.getByRole('tab',{name:'出装符文',exact:true}).click();
  assert.equal(await page.locator('.rune-tree').count(),1);
  assert(await page.locator('.build-order').innerText());
  assert.equal(calls.filter(n=>n==='MatchTimeline').length,1,'build tab did not reuse timeline');
  await shot('build-dark');
  await page.locator('.rune-tree').scrollIntoViewIfNeeded();
  await shot('runes-dark');
  await page.getByRole('tab',{name:'出装符文',exact:true}).press('Home');
  assert.equal(await page.getByRole('tab',{name:'对局概览',exact:true}).getAttribute('aria-selected'),'true','Home key did not select first analysis tab');
  // Closing and reopening the same row retains the completed manual result.
  await page.locator('.players-workspace .match-main').first().click();
  await page.locator('.players-workspace .match-main').first().click();
  await page.getByRole('tab',{name:'时间线',exact:true}).click();
  await page.locator('.match-timeline').waitFor();
  assert.equal(calls.filter(n=>n==='MatchTimeline').length,1,'reopening completed game queried timeline again');
  await page.locator('.players-workspace .match-main').first().click();
  // A different game cannot inherit a cached timeline, and a failed read is retryable.
  await page.locator('.players-workspace .match-main').nth(1).click();
  failTimeline=true;
  await page.getByRole('tab',{name:'出装符文',exact:true}).click();
  await page.locator('.full-match-detail .error-banner').waitFor();
  assert.equal(await page.locator('.match-timeline').count(),0,'failed game inherited another game timeline');
  failTimeline=false;
  await page.getByRole('button',{name:'读取本场时间线',exact:true}).click();
  await page.locator('.skill-order').waitFor();
  await page.getByRole('tab',{name:'时间线',exact:true}).click();
  await page.locator('.match-timeline').waitFor();
  assert.equal(calls.filter(n=>n==='MatchTimeline').length,3,'failed timeline did not retry explicitly');
  await page.locator('.players-workspace .match-main').nth(1).click();
  await page.locator('.players-workspace .match-main').first().click();

  for(const width of [1280,1100,880]){
   await page.setViewportSize({width,height:width===880?620:800});
   await page.getByRole('tab',{name:'对局概览',exact:true}).click();
   assert(await page.locator('.players-workspace').evaluate(el=>el.scrollWidth<=el.clientWidth+1),'analysis workspace has horizontal overflow');
   await shot(`scoreboard-${width}`);
  }
  await page.getByRole('combobox',{name:'界面主题',exact:true}).selectOption('light');
  await shot('scoreboard-light');
  await page.getByRole('tab',{name:'时间线',exact:true}).click();await shot('timeline-light');
  await page.locator('.players-workspace .match-main').first().click();
  await page.getByRole('navigation',{name:'工作区'}).getByRole('button',{name:'首页',exact:true}).click();
  await page.locator('.home-workspace .overview-performance').waitFor();
  for(const width of [1280,1100,880]){await page.setViewportSize({width,height:width===880?620:800});assert(await page.locator('.home-workspace').evaluate(el=>el.scrollWidth<=el.clientWidth+1),'overview overflow');await shot(`overview-light-${width}`);}
  const beforeLocalFilters=calls.length;
  const home=page.locator('.home-workspace');
  await home.getByLabel('常用英雄统计范围').selectOption('week');
  await home.getByLabel('查找常用英雄').fill('蛮族');
  assert.equal(await home.locator('.champion-table tbody tr').count(),1);
  assert.equal(await home.locator('.mastery-heroes>div').count(),4,'hero query changed all-mode mastery');
  assert.equal(calls.length,beforeLocalFilters,'local hero filters queried player APIs');
  await home.getByLabel('查找常用英雄').fill('');
  await home.getByRole('button',{name:'下一页',exact:true}).click();
  await home.locator('.home-pagination').filter({hasText:'第 2 页'}).waitFor();
  assert(await home.locator('.home-recent').evaluate(el=>{const workspace=el.closest('.home-workspace');return !!workspace&&Math.abs(el.getBoundingClientRect().top-workspace.getBoundingClientRect().top)<2;}),'home pagination did not return to the page-flow match section');
  const detailsBeforeHome=calls.filter(n=>n==='MatchDetail').length;
  await home.locator('.home-match .match-main').first().click();
  const homeGame=page.getByRole('dialog',{name:'对局详情',exact:true});
  await homeGame.waitFor();
  await homeGame.locator('tr.target').waitFor();
  assert((await homeGame.locator('.analysis-tabs [role=tab]').first().getAttribute('id')).includes('70021'),'second-page row did not open its loaded game');
  assert.equal(calls.filter(n=>n==='MatchDetail').length,detailsBeforeHome+1,'opening a paged home game did not load exactly one detail');
  await homeGame.press('Escape');
  assert.equal(calls.filter(n=>n==='CurrentPlayer').length,1,'analysis navigation restarted automatic home loading');
  await verifyKeyboardAccess(page, player, calls, shot);
  const publish = async () => {
   snapshot.generation++;
   snapshot.updatedAt = new Date().toISOString();
   await page.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:snapshot',data}),snapshot);
  };
  await page.setViewportSize({width:1280,height:800});
  await page.getByRole('navigation',{name:'工作区'}).getByRole('button',{name:'首页',exact:true}).click();
  if (await home.locator('.home-pagination').textContent().then(text=>text.includes('第 2 页'))) {
   await home.getByRole('button',{name:'上一页',exact:true}).click();
   await home.locator('.home-pagination').filter({hasText:'第 1 页'}).waitFor();
  }
  await verifyPlayerHistoryJumps(page,lookups,calls);
  await verifyIncompleteDetails(page,calls,publish,snapshot,releaseDetail,shot);
  snapshot.connected = true;
  await publish();
  for (const [index, queue] of [2400,420,450,2400].entries()) {
   testQueue = queue;
   Object.assign(player.history[0], {queueId:queue,mapId:queue===420?11:12,gameVersion:'16.23.1',augmentIds:[999999],runeShards:[5007,5011,5001]});
   await page.getByLabel('玩家完整 ID',{exact:true}).fill(`模式样例${index}#DEMO`);
   await page.getByRole('button',{name:'查询战绩',exact:true}).click();
   const workspace=page.locator('.players-workspace');
   await workspace.getByRole('heading',{name:`模式样例${index}#DEMO`,exact:true}).waitFor();
   await workspace.locator('.match-main').first().click();
   await workspace.locator('tr.target').waitFor();
   assert.equal(await workspace.getByRole('columnheader',{name:'视野',exact:true}).count(), queue===420?2:0);
   await workspace.getByRole('tab',{name:queue===2400?'出装与强化':'出装符文',exact:true}).click();
   assert.equal(await workspace.locator('.rune-tree').count(),queue===2400?0:1);
   if(queue===2400) await workspace.getByText('强化 #999999',{exact:true}).waitFor();
   else {
    await workspace.getByText('固定生命值 +65',{exact:true}).waitFor();
    await workspace.getByText('成长生命值 +10–180（1–18级）',{exact:true}).waitFor();
    assert.equal(await workspace.locator('.shard-options .chosen img').count(),3);
   }
  }
  assert.deepEqual(external,[]);assert.deepEqual(errors,[]);
  console.log('PASS: professional overview, rich match rows, click-authorized scoreboard, automatic single-game timeline, six team metrics, build/runes, cached tab changes, 1280/1100/880; keyboard-only home/query/history/settings/navigation, dialog focus, theme text/focus contrast and reduced motion in both themes; synthetic bridge.');
 }finally{await browser.close()}
}
main().catch(e=>{console.error(e);process.exitCode=1});
