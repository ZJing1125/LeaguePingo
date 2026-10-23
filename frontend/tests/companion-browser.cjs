// Synthetic bridge only: phase-dependent companion UI and isolated postgame review.
const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
const path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const {mockWindowRuntime}=require('./window-runtime.cjs');
const {scoutFixture,stats}=require('./scout-fixtures.cjs');
const {saveUIArtifact}=require('./ui-artifacts.cjs');

async function main(){
 const browser=await chromium.launch({headless:true,executablePath:process.env.BROWSER_EXECUTABLE});
 try{
  const context=await browser.newContext({viewport:{width:320,height:720}});
  const errors=[],external=[],calls=[],pages=[],runtimes=[];
  const base='http://127.0.0.1:43208',time='2026-10-23T12:00:00Z';
  const binding=await fs.readFile(path.join(__dirname,'../bindings/leaguepingo/appservice.ts'),'utf8');
  const methods=Object.fromEntries([...binding.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g)].map(m=>[m[2],m[1]]));
  const avatar='data:image/svg+xml;base64,'+Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64"><rect width="64" height="64" rx="10" fill="#66558f"/><circle cx="32" cy="27" r="16" fill="#efc69a"/></svg>').toString('base64');
  const players=Array.from({length:10},(_,i)=>{
   const history=Array.from({length:20},(_,j)=>({gameId:9000+j,playedAt:time,queueId:j%2?440:420,championId:23,win:j%3!==0,kills:5,deaths:3,assists:10,duration:1800,cs:180+i*5,damage:24000+i*1200,gold:12000+i*250,position:j%2?'UTILITY':'TOP',items:[3006,3078,0,0,0,0,3364],spell1:4,spell2:14}));
   const p={connectionId:1,key:`synthetic-${i}`,puuid:`synthetic-${i}`,displayName:`合成玩家${i+1}#DEMO`,team:i<5?'ally':'enemy',championId:23,championState:'selected',identityState:'known',privacyState:'private',historyState:'ready',history,historyMeta:{fetchedAt:time,parsed:20,returned:20},stats:stats(history),profile:{updatedAt:time,profileIconData:avatar,level:200+i,rankState:'ready',privacyState:'private',fields:{rank:{state:'ready',fetchedAt:time}},ranks:[{queue:'RANKED_SOLO_5x5',tier:'EMERALD',division:'II',lp:86,wins:154,losses:166},{queue:'RANKED_FLEX_SR',tier:'GOLD',division:'III',lp:27,wins:117,losses:135}]}};
   p.scout=scoutFixture(p);p.scout.champions[0].games=1360;p.scout.champions[0].mastery={level:35,points:910000,championId:23};
   const season=(games,rate,kda)=>({games,wins:games*rate/100,losses:games*(100-rate)/100,outcomeGames:games,kdaGames:games,winRate:rate,kda});
   p.scout.champions[0].queues=[{queueId:420,stats:season(800,60,5),average:[8,3,7]},{queueId:440,stats:season(360,40,4.4),average:[4,5,18]}];
   p.scout.queues[0].month=season(100,58,3);p.scout.queues[1].month=season(72,50,4);
   return p;
  });
  let snapshot={connectionId:1,generation:1,connected:true,selfKey:players[0].key,region:'SYNTHETIC',phase:'Lobby',stage:'party',gameId:0,queueId:420,scoutQueue:420,updatedAt:time,participants:[{...players[0],championId:0,championState:'unselected'}]};
  let state={revision:1,visible:true,mode:'sidebar',sidebarSide:'left',sidebarWidth:320,sidebarEnabled:true,previewEnabled:true,holdEnabled:true,holdAvailable:true,holdKey:'Backquote',pinned:true,shortcut:'Ctrl+Shift+P'};
  let info={version:'synthetic',commit:'fixture',buildTime:time,shortcut:'Ctrl+Shift+P',holdKey:'Backquote',holdEnabled:true,sidebarEnabled:true,previewEnabled:true,sidebarWidth:320,goHeapBytes:0,goroutines:0,requests:{}};
  async function emit(name,data){await Promise.all(pages.filter(p=>!p.isClosed()).map(p=>p.evaluate(({name,data})=>window._wails?.dispatchWailsEvent({name,data}),{name,data})));}
  async function publish(){snapshot={...snapshot,generation:snapshot.generation+1,updatedAt:new Date().toISOString()};await emit('leaguepingo:snapshot',snapshot);}
  async function overlay(mode){state={...state,mode,revision:state.revision+1};await emit('leaguepingo:overlay',state);}
  await context.route('**/*',r=>{external.push(r.request().url());return r.abort();});
  await context.route(base+'/**',async route=>{
   const url=new URL(route.request().url());
   if(url.pathname==='/wails/custom.js')return route.fulfill({contentType:'text/javascript',body:''});
   if(url.pathname==='/wails/runtime'){
    const req=route.request().postDataJSON();for(const runtime of runtimes)if(await runtime.handle(route,req))return;
    const method=methods[req.args?.methodID],args=req.args?.args??[];calls.push({method,args});let result=null;
    if(method==='Snapshot')result=snapshot;
    else if(method==='OverlayStatus')result=state;
    else if(method==='CurrentPlayer')result=players[0];
    else if(method==='AuxiliaryReady'){}
    else if(method==='SetSidebarCollapsed'){state={...state,collapsed:args[0],mode:args[0]?'rail':'sidebar',revision:state.revision+1};result=state;}
    else if(method==='SetScoutQueue'){snapshot={...snapshot,scoutQueue:args[0],generation:snapshot.generation+1,updatedAt:new Date().toISOString()};result=snapshot;await emit('leaguepingo:snapshot',snapshot);}
    else if(method==='HideOverlay'){state={...state,visible:false,mode:'hidden',revision:state.revision+1};result=state;}
    else if(method==='Info')result=info;
    else if(method==='ConfigureCompanion'){info={...info,sidebarEnabled:args[0],previewEnabled:args[1],sidebarWidth:args[2]};result=info;}
    else if(method==='RefreshReview'){
     assert.equal(args[0],42);
     const resultPlayers=players.map((p,i)=>({participantId:i+1,displayName:p.displayName,teamId:i<5?100:200,target:i===0,summary:{...p.history[0],gameId:42,kills:17+i,win:i<5}}));
     for(const p of resultPlayers){
      const team=resultPlayers.filter(other=>other.teamId===p.teamId),sum=key=>team.reduce((n,other)=>n+other.summary[key],0);
      p.damageShare=p.summary.damage*100/sum('damage');p.goldShare=p.summary.gold*100/sum('gold');p.participation=(p.summary.kills+p.summary.assists)*100/sum('kills');
     }
     snapshot={...snapshot,review:{...snapshot.review,state:'ready',result:{gameId:42,players:resultPlayers,partial:false,fetchedAt:time}},generation:snapshot.generation+1,updatedAt:new Date().toISOString()};result=snapshot;
    }else throw new Error('Unexpected service call '+method);
    return route.fulfill({json:result});
   }
   const name=url.pathname==='/'?'index.html':url.pathname.slice(1);assert(!name.includes('..'));
   return route.fulfill({body:await fs.readFile(path.join(__dirname,'../dist',name)),contentType:({'.js':'text/javascript','.css':'text/css','.png':'image/png','.jpg':'image/jpeg'})[path.extname(name)]??'text/html'});
  });
  async function pageAt(route,width,height){const p=await context.newPage();p.setDefaultTimeout(12000);p.on('pageerror',e=>errors.push(e.message));pages.push(p);runtimes.push(mockWindowRuntime(p));await p.setViewportSize({width,height});await p.goto(base+route);return p;}
  async function shot(p,name){await saveUIArtifact(p,process.env.UI_SCREENSHOT_PREFIX,name,name==='review-result');}
  const side=await pageAt('/?view=companion',320,720);
  await side.locator('.sidebar-player').waitFor();
  await side.waitForFunction(()=>document.querySelector('.sidebar-identity img')?.complete);
  assert.equal(await side.locator('.sidebar-player').count(),1);
  assert.equal(await side.locator('.desktop-titlebar,.home-workspace,.players-workspace').count(),0,'auxiliary mounted main workspaces');
  assert.equal(await side.locator('.sidebar-hero').count(),0,'lobby should not display missing champion fields');
  assert((await side.locator('.sidebar-rank').innerText()).includes('翡翠 II'));
  assert(!calls.some(c=>/CurrentPlayer|Lookup|History|Profile/.test(c.method)),'sidebar startup performed player queries');
  await shot(side,'sidebar-party');
  const reads=calls.length;
  snapshot={...snapshot,phase:'ChampSelect',stage:'preparing',gameId:42,participants:structuredClone(players)};await publish();
  assert.equal(await side.locator('.sidebar-player').count(),5,'pregame sidebar exposed enemy slots');
  assert((await side.locator('.sidebar-experience').first().innerText()).includes('1360 场'));
  snapshot.participants[0].championState='intent';await publish();
  await side.locator('.sidebar-hero').first().getByText(/预选/).waitFor();
  assert((await side.locator('.sidebar-hero-title').first().innerText()).includes('60%'),'unlocked champion lost ranked statistics');
  assert((await side.locator('.sidebar-experience').first().innerText()).includes('1360 场'),'unlocked champion lost experience');
  snapshot.participants[0].championId=25;snapshot.participants[0].championState='intent';await publish();
  await side.locator('.sidebar-hero').first().getByText(/预选/).waitFor();
  snapshot.participants[0]=structuredClone(players[0]);await publish();
  await shot(side,'sidebar-selection');
  const summaryLayout=await side.locator('.sidebar-player').evaluateAll(els=>({bottom:document.querySelector('.sidebar-players').getBoundingClientRect().bottom,children:[...els[0].querySelector('summary').children].map(el=>({class:el.className,height:el.getBoundingClientRect().height})),rows:els.map(el=>({height:el.getBoundingClientRect().height,bottom:el.getBoundingClientRect().bottom}))}));
  assert(summaryLayout.rows.every(r=>r.bottom<=summaryLayout.bottom+1),'five player summaries should fit a 720px client: '+JSON.stringify(summaryLayout));
  await side.locator('.sidebar-player summary').first().click();
  await side.locator('.sidebar-player[open] .scout-rings').waitFor();
  await side.getByRole('button',{name:'灵活组排',exact:true}).click();
  await side.locator('.sidebar-rank').first().getByText(/黄金 III/).waitFor();
  assert((await side.locator('.sidebar-player[open] .scout-rings').innerText()).includes('辅助'));
  assert((await side.locator('.sidebar-player[open] .scout-rings').innerText()).includes('72 场'));
  assert((await side.locator('.sidebar-player[open] .scout-kda').innerText()).includes('4.0 / 5.0 / 18.0'));
  assert.deepEqual(calls.slice(reads).map(c=>c.method),['SetScoutQueue'],'render/hero changes fetched extra player data');
  for(const width of [280,320,420]){await side.setViewportSize({width,height:720});assert(await side.locator('.client-sidebar').evaluate(el=>el.scrollWidth<=el.clientWidth+1));}
  await side.setViewportSize({width:320,height:720});
  await side.getByRole('button',{name:'收起客户端侧栏'}).click();await side.getByRole('button',{name:'展开客户端侧栏'}).waitFor();
  snapshot={...snapshot,phase:'Matchmaking',stage:'party',participants:[players[0]]};await publish();
  assert.equal(await side.locator('.sidebar-player').count(),0,'stage change reopened manually collapsed rail');
  await side.getByRole('button',{name:'展开客户端侧栏'}).click();
  await side.locator('.sidebar-player').waitFor();assert.equal(await side.locator('.sidebar-hero').count(),0);
  snapshot={...snapshot,phase:'InProgress',stage:'in-game',participants:structuredClone(players),gameId:42};await publish();
  await side.setViewportSize({width:1100,height:640});await overlay('preview');
  await side.locator('.overlay-player').nth(9).waitFor();
  const readOnlyCalls=calls.length;
  snapshot.participants[9]={...snapshot.participants[9],displayName:'玩家名称暂不可用',historyState:'loading',scout:{...snapshot.participants[9].scout,state:'loading',complete:false,complete12:false,complete30:false}};await publish();
  await side.getByText('正在补齐 1 人统计',{exact:true}).waitFor();
  assert((await side.locator('.overlay-player').nth(9).innerText()).includes('正在获取'));
  snapshot.participants[9]=structuredClone(players[9]);await publish();
  assert.equal(calls.length,readOnlyCalls,'initial-match progress/name update triggered a player query');
  assert.equal(await side.locator('.overlay-controls').count(),0);
  assert((await side.locator('.overlay-footer').innerText()).includes('6 秒后收起'));
  assert.equal(await side.locator('.overlay-shell').evaluate(el=>getComputedStyle(el).pointerEvents),'none');
  const fit=await side.locator('.overlay-player').evaluateAll(els=>els.every(el=>{const r=el.getBoundingClientRect();return r.width>=160 && r.bottom<=innerHeight;}));assert(fit,'1100x640 preview cards do not fit');
  await shot(side,'preview');await overlay('hold');
  assert((await side.locator('.overlay-footer').innerText()).includes('松开 ~ 隐藏'));
  await side.setViewportSize({width:400,height:360});
  assert(await side.locator('.overlay-player').evaluateAll(els=>els.every(el=>el.getBoundingClientRect().width>=160)));
  const beforeReview=calls.length;
  const main=await pageAt('/',1280,800);
  await main.getByRole('navigation',{name:'工作区'}).getByRole('button',{name:'对局室',exact:true}).click();
  await main.getByText('尚无赛后记录',{exact:true}).waitFor();
  assert.equal(await main.locator('.player-row').count(),0,'main room duplicated live roster');
  const captured=structuredClone(players);
  captured[9].scout=null;
  snapshot={...snapshot,phase:'Lobby',stage:'party',gameId:0,participants:[players[0]],review:{connectionId:1,gameId:42,region:'SYNTHETIC',selfKey:players[0].key,queueId:420,scoutQueue:420,capturedAt:time,state:'unavailable',participants:captured}};await publish();
  await main.getByRole('button',{name:'同步结算',exact:true}).click();
  await main.locator('.review-result').getByText('本局结果已同步').waitFor();
  assert((await main.locator('.review-result tbody tr').first().innerText()).includes('17 / 3 / 10'),'actual result replaced by pregame average');
  await shot(main,'review-result');
  await main.getByRole('button',{name:'赛前资料',exact:true}).click();
  await main.locator('.player-row').nth(9).waitFor();
  const missingReport=await main.locator('.player-row').nth(9).innerText();
  assert(missingReport.includes('未保留') && !missingReport.includes('准备中') && !missingReport.includes('正在准备'),'completed review retained a running statistics state');
  const fixedCards=await main.locator('.team-columns').innerText();
  snapshot={...snapshot,phase:'ChampSelect',stage:'preparing',gameId:43,participants:[{...players[0],championId:99,displayName:'下一局玩家#DEMO'}]};await publish();
  assert.equal(await main.locator('.team-columns').innerText(),fixedCards,'new selection replaced completed review');
  const beforeMode=calls.length;
  await main.getByRole('button',{name:'灵活组排',exact:true}).click();
  assert.equal(calls.length,beforeMode,'review mode switch changed live mode or fetched data');
  assert.equal(snapshot.scoutQueue,440);
  assert((await main.locator('.player-row').first().innerText()).includes('1360 场'));
  for(const [width,height]of [[1280,800],[1100,760],[880,620]]){await main.setViewportSize({width,height});assert(await main.locator('.review-room').evaluate(el=>el.scrollWidth<=el.clientWidth+1));}
  await main.setViewportSize({width:1280,height:800});await shot(main,'review-pregame');
  const first=main.locator('.player-row').first();await first.click();
  const drawer=main.locator('.player-drawer');await drawer.getByRole('heading',{name:players[0].displayName,exact:true}).waitFor();
  const beforeFilter=calls.length;
  await drawer.getByLabel('分析样本场数',{exact:true}).selectOption('50');
  await drawer.getByLabel('战绩模式筛选',{exact:true}).selectOption('420');
  assert(!(await drawer.innerText()).includes('连接恢复后'),'captured review must not claim the live connection is offline');
  assert((await drawer.locator('.browse-status').innerText()).includes('赛前保留战绩'));
  assert.equal(calls.length,beforeFilter,'captured review started a history scan');
  assert.equal(await drawer.getByRole('button',{name:'刷新数据',exact:true}).isDisabled(),true);
  const firstBounds=await first.boundingBox();
  await main.setViewportSize({width:880,height:620});
  const itemBounds=await drawer.locator('.match-items').first().evaluate(el=>{const r=el.getBoundingClientRect();return {count:el.children.length,fits:[...el.children].every(i=>{const c=i.getBoundingClientRect();return c.left>=r.left-1 && c.right<=r.right+1;})};});
  assert.equal(itemBounds.count,7);assert(itemBounds.fits,'review equipment clipped at 880');
  await shot(main,'review-detail-880');
  await main.getByRole('button',{name:'下一位玩家',exact:true}).click();await drawer.getByRole('heading',{name:players[1].displayName,exact:true}).waitFor();
  await main.keyboard.press('Escape');await drawer.waitFor({state:'detached'});
  await main.setViewportSize({width:1280,height:800});
  assert(Math.abs((await first.boundingBox()).height-firstBounds.height)<2,'modal resized review cards');
  await main.getByRole('button',{name:'设置与诊断',exact:true}).click();await main.getByRole('tab',{name:'游戏辅助',exact:true}).click();
  await main.getByLabel('开局自动展示快览',{exact:true}).uncheck();await main.getByLabel('侧栏宽度',{exact:true}).selectOption('360');
  await main.getByRole('button',{name:'保存游戏辅助设置',exact:true}).click();await main.getByText('游戏辅助设置已保存。').waitFor();
  assert.equal(info.previewEnabled,false);assert.equal(info.sidebarWidth,360);
  await shot(main,'settings-companion');
  assert(calls.slice(beforeReview).some(c=>c.method==='RefreshReview'));
  assert.equal(calls.filter(c=>c.method==='AuxiliaryReady').length,1,'auxiliary frontend reloaded between modes');
  assert(!calls.some(c=>/Lookup|HistoryPage|EnsureProfile|RefreshData/.test(c.method)),'cache-only companion/review paths fetched player data');
  assert.deepEqual(errors,[]);assert.deepEqual(external,[]);
  console.log('PASS: left sidebar party/selection/rail/mode/hero changes, isolated auxiliary startup, 1100/400 preview and hold, review settlement vs captured reports, next-match/filter isolation, cached modal/7 items/focus/880, companion settings; synthetic bridge only.');
 }finally{await browser.close();}
}
main().catch(e=>{console.error(e);process.exitCode=1});
