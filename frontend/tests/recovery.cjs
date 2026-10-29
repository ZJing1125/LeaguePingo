// Synthetic Wails bridge; no real account, service, process or game interaction.
const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
const path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {mockWindowRuntime}=require('./window-runtime.cjs');
const {saveUIArtifact}=require('./ui-artifacts.cjs');
async function main(){
 const browser=await chromium.launch({headless:true,executablePath:process.env.BROWSER_EXECUTABLE});
 try{
  const page=await browser.newPage({viewport:{width:1280,height:800}}),desktop=mockWindowRuntime(page),errors=[],external=[],calls=[];
  page.setDefaultTimeout(10000);page.on('pageerror',e=>errors.push(e.message));
  const binding=await fs.readFile(path.join(__dirname,'../bindings/leaguepingo/appservice.ts'),'utf8');
  const methods=Object.fromEntries([...binding.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g)].map(m=>[m[2],m[1]]));
  const old=new Date(Date.now()-60_000).toISOString();
  function player(key='self',degraded=true){return {key,puuid:key,connectionId:1,displayName:`${key}#DEMO`,identityState:'known',historyState:'ready',privacyState:'public',history:[{gameId:10,championId:23,queueId:420,win:true,kills:6,deaths:2,assists:8,duration:1800}],historyMeta:{returned:1,parsed:1,fetchedAt:old,checkedAt:old,...(degraded?{service:{code:'ServiceBusy',retryAt:new Date(Date.now()+700).toISOString()}}:{})},stats:{games:1},profile:{level:100,rankState:'ready',ranks:[{queue:'RANKED_SOLO_5x5',tier:'GOLD',division:'II',lp:40,wins:8,losses:9}],profileIconData:'data:image/svg+xml;base64,'+Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" fill="#7862b8"/></svg>').toString('base64')}};}
  let self=player(),scenario='restore',retryCount=0;
  let snapshot={connected:true,connectionId:1,selfKey:'self',phase:'None',stage:'idle',generation:1,participants:[],updatedAt:new Date().toISOString()};
  await page.route('**/*',r=>{external.push(r.request().url());return r.abort();});
  await page.route('http://127.0.0.1:43208/**',async route=>{
   const url=new URL(route.request().url());
   if(url.pathname==='/wails/custom.js')return route.fulfill({contentType:'text/javascript',body:''});
   if(url.pathname==='/wails/runtime'){
    const request=route.request().postDataJSON();if(await desktop.handle(route,request))return;
    const method=methods[request.args?.methodID],args=request.args?.args??[];calls.push(method);let result;
    if(method==='Snapshot')result=snapshot;
    else if(method==='CurrentPlayer')result=self;
    else if(method==='OverlayStatus')result={visible:false,revision:0,pinned:true};
    else if(method==='GuideStatus')result={revision:0,status:'idle',query:{}};
    else if(method==='AutoAcceptStatus')result={revision:0,enabled:false,status:'off',delaySeconds:1};
    else if(method==='PostGameStatus')result={revision:0,status:'off',returnEnabled:false,queueEnabled:false,delaySeconds:10};
    else if(method==='Lookup')result=player('other');
    else if(method==='RetryHistory'){
     retryCount++;
     assert.equal(args[1],1);
     if(scenario==='restore' && retryCount===2){self=player('self',false);self.historyMeta.fetchedAt=new Date().toISOString();self.historyMeta.checkedAt=self.historyMeta.fetchedAt;self.history[0].gameId=11;result=self;}
     else{result=player(args[0]);result.historyMeta.checkedAt=new Date().toISOString();}
    }
    else if(method==='RefreshData'){result=player(args[0],false);result.historyMeta.fetchedAt=new Date().toISOString();result.historyMeta.checkedAt=result.historyMeta.fetchedAt;}
    else if(method==='MatchDetail')result={gameId:args[2],connectionId:1,partial:true,players:[],teams:[],fetchedAt:new Date().toISOString()};
    else if(method==='HistoryPage')result={matches:[],offset:args[2],nextOffset:args[2],hasMore:false,meta:{returned:0,parsed:0}};
    else throw new Error(`unexpected service ${method}`);
    return route.fulfill({json:result});
   }
   const relative=url.pathname==='/'?'index.html':url.pathname.slice(1);if(relative.includes('..'))return route.abort();
   return route.fulfill({body:await fs.readFile(path.join(__dirname,'../dist',relative)),contentType:relative.endsWith('.js')?'text/javascript':relative.endsWith('.css')?'text/css':relative.endsWith('.png')?'image/png':relative.endsWith('.jpg')?'image/jpeg':'text/html'});
  });
  const home=page.locator('.home-workspace'),query=page.getByRole('textbox',{name:'玩家完整 ID'});
  await page.goto('http://127.0.0.1:43208/');
  await home.getByText('远端战绩暂不可用，已保留当前记录。',{exact:false}).waitFor();
  assert.equal(await home.locator('.home-match').count(),1,'outage erased local records');
  await home.getByLabel('本地战绩缓存与恢复：实验性，国服实机验证未完成',{exact:true}).waitFor();
  assert((await home.innerText()).includes('黄金 II'),'outage erased rank');
  await home.getByText('更新于 1 分钟前',{exact:true}).first().waitFor();
  await page.waitForFunction(()=>document.querySelector('.home-workspace .history-service-status.degraded')===null);
  assert.equal(retryCount,2,'recovery failed to stop at two reads');
  assert.equal(await home.locator('.home-match').count(),1);
  assert.equal(calls.filter(x=>x==='Lookup').length,0,'background retried identity lookup');
  // A second, queried player's continuing outage cannot retry forever.
  scenario='exhaust';retryCount=0;
  await query.fill('other#DEMO');await query.press('Enter');
  const view=page.locator('.players-workspace .player-view');
  await view.getByText('自动重试已暂停，可手动重试。',{exact:false}).waitFor();
  assert.equal(retryCount,2);assert.equal(await view.locator('.match-card').count(),1);
  await page.waitForTimeout(1000);assert.equal(retryCount,2,'rendering/age ticks restarted recovery');
  await view.getByRole('button',{name:'重试战绩',exact:true}).click();
  await view.locator('.history-service-status.degraded').waitFor({state:'detached'});
  assert(calls.includes('RefreshData'),'manual retry unavailable after exhaustion');
  // A local background update must reach the home even with an empty roster.
  const updated=player('self');updated.historyMeta.checkedAt=new Date().toISOString();updated.historyMeta.service.retryAt=new Date(Date.now()+1200).toISOString();
  await page.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:player-updated',data}),updated);
  await page.getByRole('navigation',{name:'工作区'}).getByRole('button',{name:'首页',exact:true}).click();
  await home.locator('.history-service-status.degraded').waitFor();
  const before=retryCount;
  await home.locator('.home-match .match-main').first().click();
  const detail=page.getByRole('dialog',{name:'对局详情',exact:true});await detail.waitFor();
  const late=player('self',false);late.history[0].gameId=12;late.historyMeta.fetchedAt=new Date().toISOString();late.historyMeta.checkedAt=late.historyMeta.fetchedAt;
  await page.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:player-updated',data}),late);
  await page.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:player-updated',data}),updated);
  await page.waitForTimeout(1400);assert.equal(retryCount,before,'automatic recovery interrupted an open match detail');
  assert(await detail.isVisible(),'arriving cache update replaced the match being read');
  snapshot={...snapshot,phase:'InProgress',stage:'in-game',generation:2};
  await page.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:snapshot',data}),snapshot);
  await detail.press('Escape');await detail.waitFor({state:'detached'});
  await home.locator('.history-service-status.degraded').waitFor({state:'detached'});
  await page.waitForTimeout(1200);assert.equal(retryCount,before,'entering game left a scheduled recovery active');
  assert.equal(await home.locator('.home-match').count(),1);
  for(const width of [1280,880]){await page.setViewportSize({width,height:800});assert(await home.evaluate(el=>el.scrollWidth<=el.clientWidth+1),'recovery status overflow');await saveUIArtifact(page,process.env.UI_SCREENSHOT_PREFIX,`recovery-${width}`);}
  assert.deepEqual(errors,[]);assert.deepEqual(external,[]);
  console.log('PASS: service degradation preserves rank/history/freshness, finite automatic recovery, manual retry, idle background updates, phase cancellation and 880/1280 layouts; synthetic bridge only.');
 }finally{await browser.close();}
}
main().catch(e=>{console.error(e);process.exitCode=1;});
