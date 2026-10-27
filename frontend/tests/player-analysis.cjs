const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
const path=require('node:path');
const {chromium}=require('playwright');
const {mockWindowRuntime}=require('./window-runtime.cjs');
const {saveUIArtifact}=require('./ui-artifacts.cjs');
const {makePlayer,makeDetail,makeTimeline,now}=require('./analysis-fixture.cjs');
async function main(){
 const browser=await chromium.launch({headless:true,executablePath:process.env.BROWSER_EXECUTABLE});
 try{
  const page=await browser.newPage({viewport:{width:1280,height:900}});page.setDefaultTimeout(15000);
  const calls=[],errors=[],external=[];page.on('pageerror',e=>errors.push(e.message));const desktop=mockWindowRuntime(page);
  const binding=await fs.readFile(path.join(__dirname,'../bindings/leaguepingo/appservice.ts'),'utf8');
  const methods=Object.fromEntries([...binding.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g)].map(m=>[m[2],m[1]]));
  let player=makePlayer();let failTimeline=false;
  const snapshot={connectionId:1,selfKey:player.key,generation:1,connected:true,phase:'None',stage:'idle',updatedAt:now,participants:[]};
  await page.route('**/*',r=>{external.push(r.request().url());return r.abort()});
  await page.route('http://127.0.0.1:43208/**',async route=>{
   const url=new URL(route.request().url());
   if(url.pathname==='/wails/custom.js')return route.fulfill({contentType:'text/javascript',body:''});
   if(url.pathname==='/wails/runtime'){
    const req=route.request().postDataJSON();if(await desktop.handle(route,req))return;
    const name=methods[req.args?.methodID],args=req.args?.args??[];calls.push(name);
    let result=null;
    if(name==='Snapshot')result=snapshot;
    else if(name==='Info')result={version:'synthetic',commit:'analysis-test',requests:{},shortcut:'Ctrl+Shift+P',holdEnabled:true,holdKey:'Backquote'};
    else if(name==='OverlayStatus')result={revision:0,visible:false,sidebarEnabled:true,sidebarWidth:320};
    else if(['CurrentPlayer','Lookup','RefreshData','EnsureProfile'].includes(name))result=player;
    else if(name==='HistoryPage')result={matches:Array.from({length:20},(_,i)=>({...player.history[i],gameId:70021+i,playedAt:new Date(Date.parse(now)-(20+i)*4*3600000).toISOString()})),meta:{fetchedAt:now,returned:20,parsed:20,excluded:0},offset:args[2],nextOffset:args[2]+20,hasMore:false};
    else if(name==='MatchDetail')result=makeDetail(args[2]);
    else if(name==='MatchTimeline'){
     if(failTimeline)return route.fulfill({status:500,json:{message:'本场时间线暂未取得'}});
     result={...makeTimeline(args[2]),detail:makeDetail(args[2])};
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
  await shot('history-dark');
  await page.locator('.players-workspace .match-main').first().click();
  await page.locator('.full-match-detail tr.target').waitFor();
  await shot('scoreboard-dark');
  if(process.env.ANALYSIS_BASELINE==='1'){assert.deepEqual(errors,[]);return;}
  assert.equal(calls.filter(n=>n==='MatchDetail').length,1,'opening one game must load exactly that game detail');
  assert.equal(await page.locator('.full-match-detail tbody tr').count(),10);
  assert.equal(await page.locator('.detail-loadout').first().locator('.game-icon').count(),7);
  const before=calls.length;
  await page.getByRole('tab',{name:'团队分析',exact:true}).click();
  await page.locator('.team-analysis').waitFor();
  assert.equal(await page.locator('.team-metric').count(),6);
  assert.equal(calls.length,before,'team analysis fetched unrequested data');
  await shot('team-dark');
  await page.getByRole('tab',{name:'时间线',exact:true}).click();
  assert.equal(calls.length,before,'opening timeline tab must not scan games');
  await page.getByRole('button',{name:'读取本场时间线',exact:true}).click();
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
  await page.getByRole('tab',{name:'时间线',exact:true}).click();
  failTimeline=true;
  await page.getByRole('button',{name:'读取本场时间线',exact:true}).click();
  await page.locator('.full-match-detail .error-banner').waitFor();
  assert.equal(await page.locator('.match-timeline').count(),0,'failed game inherited another game timeline');
  failTimeline=false;
  await page.getByRole('button',{name:'读取本场时间线',exact:true}).click();
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
  const detailsBeforeHome=calls.filter(n=>n==='MatchDetail').length;
  await home.locator('.home-match .match-main').first().click();
  const homeGame=page.getByRole('dialog',{name:'对局详情',exact:true});
  await homeGame.waitFor();
  await homeGame.locator('tr.target').waitFor();
  assert((await homeGame.locator('.analysis-tabs [role=tab]').first().getAttribute('id')).includes('70021'),'second-page row did not open its loaded game');
  assert.equal(calls.filter(n=>n==='MatchDetail').length,detailsBeforeHome+1,'opening a paged home game did not load exactly one detail');
  await homeGame.press('Escape');
  assert.equal(calls.filter(n=>n==='CurrentPlayer').length,1,'analysis navigation restarted automatic home loading');
  assert.deepEqual(external,[]);assert.deepEqual(errors,[]);
  console.log('PASS: professional overview, rich match rows, click-authorized scoreboard, opt-in timeline, six team metrics, build/runes, cached tab changes, 1280/1100/880 and both themes; synthetic bridge.');
 }finally{await browser.close()}
}
main().catch(e=>{console.error(e);process.exitCode=1});
