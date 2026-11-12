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
 let context;
 try{
  context=await browser.newContext({viewport:{width:320,height:720}});
  const errors=[],external=[],calls=[],pages=[],runtimes=[];
  const base='http://127.0.0.1:43208',time='2026-10-23T12:00:00Z';
  const binding=await fs.readFile(path.join(__dirname,'../bindings/leaguepingo/appservice.ts'),'utf8');
  const methods=Object.fromEntries([...binding.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g)].map(m=>[m[2],m[1]]));
  const avatar='data:image/svg+xml;base64,'+Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64"><rect width="64" height="64" rx="10" fill="#66558f"/><circle cx="32" cy="27" r="16" fill="#efc69a"/></svg>').toString('base64');
  const players=Array.from({length:10},(_,i)=>{
   const history=Array.from({length:20},(_,j)=>({gameId:9000+j,playedAt:time,queueId:j%2?440:420,championId:23,win:j%3!==0,kills:5,deaths:3,assists:10,duration:1800,cs:180+i*5,damage:24000+i*1200,gold:12000+i*250,position:j%2?'UTILITY':'TOP',items:[3006,3078,0,0,0,0,3364],spell1:4,spell2:14}));
   const p={connectionId:1,key:`synthetic-${i}`,puuid:`synthetic-${i}`,displayName:`合成玩家${i+1}#DEMO`,team:i<5?'ally':'enemy',championId:23,championState:'selected',identityState:'known',privacyState:'private',historyState:'ready',history,historyMeta:{fetchedAt:time,parsed:20,returned:20},stats:stats(history),profile:{updatedAt:time,profileIconData:avatar,level:200+i,rankState:'ready',privacyState:'private',fields:{rank:{state:'ready',fetchedAt:time}},ranks:[{queue:'RANKED_SOLO_5x5',tier:'EMERALD',division:'II',lp:86,wins:154,losses:166,source:'sgp',seasonId:37},{queue:'RANKED_FLEX_SR',tier:'GOLD',division:'III',lp:27,wins:117,losses:135,source:'sgp',seasonId:38}]}};
   p.scout=scoutFixture(p);p.scout.champions[0].games=1360;p.scout.champions[0].mastery={level:35,points:910000,championId:23};
   const season=(games,rate,kda)=>({games,wins:games*rate/100,losses:games*(100-rate)/100,outcomeGames:games,kdaGames:games,winRate:rate,kda});
   p.scout.champions[0].queues=[{queueId:420,stats:season(800,60,5),average:[8,3,7]},{queueId:440,stats:season(360,40,4.4),average:[4,5,18]}];
   p.scout.queues[0].month=season(100,58,3);p.scout.queues[1].month=season(72,50,4);
   return p;
  });
  // A non-self zero-loss endpoint response is normalized to unknown by the
  // source. Independent seasonal records then provide each queue's record.
  players[1].profile.ranks[0].wins=384;players[1].profile.ranks[0].losses=null;
  players[1].profile.ranks[1].wins=26;players[1].profile.ranks[1].losses=null;
  players[1].scout.champions[0].queues[0].stats={games:50,wins:30,losses:20,outcomeGames:50,kdaGames:50,winRate:60,kda:5};
  players[1].scout.champions[0].queues[1].stats={games:20,wins:8,losses:12,outcomeGames:20,kdaGames:20,winRate:40,kda:4.4};
  players[1].scout.queues[0].season={games:100,wins:49,losses:51,outcomeGames:100,kdaGames:100,winRate:49,kda:3};
  players[1].scout.queues[1].season={games:50,wins:20,losses:30,outcomeGames:50,kdaGames:50,winRate:40,kda:4};
  let snapshot={connectionId:1,generation:1,connected:true,selfKey:players[0].key,region:'SYNTHETIC',phase:'Lobby',stage:'party',gameId:0,queueId:420,scoutQueue:420,updatedAt:time,participants:[{...players[0],championId:0,championState:'unselected'}]};
  let state={revision:1,visible:true,mode:'sidebar',sidebarSide:'left',sidebarWidth:320,sidebarEnabled:true,previewEnabled:true,holdEnabled:true,holdAvailable:true,holdKey:'Backquote',pinned:true,shortcut:'Ctrl+Shift+P'};
  let info={version:'synthetic',commit:'fixture',buildTime:time,shortcut:'Ctrl+Shift+P',holdKey:'Backquote',holdEnabled:true,sidebarEnabled:true,previewEnabled:true,sidebarWidth:320,guidesEnabled:true,goHeapBytes:0,goroutines:0,requests:{}};
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
    if (method === 'GuideStatus') result={revision:0,status:'idle',query:{},base:null,matchup:null};
    else if (method === 'PostGameStatus') result={revision:0,returnEnabled:false,queueEnabled:false,delaySeconds:10,status:'off',dueAt:'0001-01-01T00:00:00Z'};
    else if(method==='AutoAcceptStatus')result={revision:0,enabled:false,delaySeconds:1,status:'off',dueAt:'0001-01-01T00:00:00Z'};
    else if(method==='Snapshot')result=snapshot;
    else if(method==='OverlayStatus')result=state;
    else if(method==='CurrentPlayer')result=players[0];
    else if(method==='AuxiliaryReady'){}
    else if(method==='SetSidebarCollapsed'){state={...state,collapsed:args[0],mode:args[0]?'rail':'sidebar',revision:state.revision+1};result=state;}
    else if(method==='SetScoutQueue'){snapshot={...snapshot,scoutQueue:args[0] || snapshot.queueId,generation:snapshot.generation+1,updatedAt:new Date().toISOString()};result=snapshot;await emit('leaguepingo:snapshot',snapshot);}
    else if(method==='HideOverlay'){state={...state,visible:false,mode:'hidden',revision:state.revision+1};result=state;}
    else if(method==='Info')result=info;
    else if(method==='ConfigureCompanion'){info={...info,sidebarEnabled:args[0],previewEnabled:args[1],sidebarWidth:args[2]};result=info;}
    else if(method==='ConfigureGuides'){info={...info,guidesEnabled:args[0]};result=info;}
    else if(method==='RefreshReview'){
     assert.equal(args[0],42);
     const resultPlayers=players.map((p,i)=>({participantId:i+1,displayName:p.displayName,teamId:i<5?100:200,target:i===0,summary:{...p.history[0],gameId:42,kills:17+i,win:i<5}}));
     for(const p of resultPlayers){
      const team=resultPlayers.filter(other=>other.teamId===p.teamId),sum=key=>team.reduce((n,other)=>n+other.summary[key],0);
      p.damageShare=p.summary.damage*100/sum('damage');p.goldShare=p.summary.gold*100/sum('gold');p.participation=(p.summary.kills+p.summary.assists)*100/sum('kills');
     }
     snapshot={...snapshot,review:{...snapshot.review,state:'ready',result:{gameId:42,players:resultPlayers,partial:false,fetchedAt:time}},generation:snapshot.generation+1,updatedAt:new Date().toISOString()};result=snapshot;
    }else throw new Error('Unexpected service call '+method+' phase='+snapshot.phase+' args='+JSON.stringify(args)+' recent='+JSON.stringify(calls.slice(-5)));
    return route.fulfill({json:result});
   }
   const name=url.pathname==='/'?'index.html':url.pathname.slice(1);assert(!name.includes('..'));
   return route.fulfill({body:await fs.readFile(path.join(__dirname,'../dist',name)),contentType:({'.js':'text/javascript','.css':'text/css','.png':'image/png','.jpg':'image/jpeg'})[path.extname(name)]??'text/html'});
  });
  async function pageAt(route,width,height){const p=await context.newPage();p.setDefaultTimeout(12000);p.on('pageerror',e=>errors.push(e.message));pages.push(p);runtimes.push(mockWindowRuntime(p));await p.setViewportSize({width,height});await p.goto(base+route);return p;}
  async function shot(p,name){await saveUIArtifact(p,process.env.UI_SCREENSHOT_PREFIX,name,['review-result','loading-preview','sidebar-ranked'].includes(name));}
  async function rankFits(p){
   const problems=await p.locator('.scout-rank:visible').evaluateAll(els=>els.flatMap(el=>{
    const parent=el.closest('.scout-headline').getBoundingClientRect(),cell=el.getBoundingClientRect();
    return [...el.querySelectorAll('.scout-rank-heading,.rank-line,.scout-season')].filter(child=>{
     const r=child.getBoundingClientRect();return r.left<cell.left-1||r.right>cell.right+1||r.bottom>parent.bottom+1||r.top<parent.top-1;
    }).map(child=>({class:child.className,text:child.textContent,rect:child.getBoundingClientRect().toJSON(),parent:parent.toJSON(),styles:{size:getComputedStyle(child).fontSize,line:getComputedStyle(child).lineHeight,display:getComputedStyle(child).display},siblings:[...el.children].map(c=>({class:c.className,height:c.getBoundingClientRect().height,line:getComputedStyle(c).lineHeight,size:getComputedStyle(c).fontSize}))}));
   }));
   assert.deepEqual(problems,[],'rank scope/results escaped their headline');
  }

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
  await side.locator('.sidebar-player summary').nth(1).click();
  await side.locator('.sidebar-player[open] .scout-rings').waitFor();
  const rankResults=side.locator('.sidebar-player[open] .scout-season');
  assert((await rankResults.innerText()).includes('本季 49%'));
  assert((await rankResults.innerText()).includes('49胜 / 51负'));
  assert(!(await rankResults.innerText()).includes('384胜'),'redacted endpoint losses produced an undefeated record');
  await shot(side,'sidebar-ranked');
  snapshot.participants[1].scout.complete=false;await publish();
  await rankResults.getByText('本季已读 49%',{exact:true}).waitFor();
  snapshot.participants[1].scout.complete=true;await publish();

  await side.getByRole('combobox',{name:'对局统计模式'}).selectOption('440');
  await side.locator('.sidebar-rank').first().getByText(/黄金 III/).waitFor();
  assert((await side.locator('.sidebar-player[open] .scout-rings').innerText()).includes('辅助'));
  assert((await rankResults.innerText()).includes('本季 40%'));assert((await rankResults.innerText()).includes('20胜 / 30负'));
  assert((await side.locator('.sidebar-player[open] .scout-rings').innerText()).includes('72 场'));
  assert((await side.locator('.sidebar-player[open] .scout-kda').innerText()).includes('4.0 / 5.0 / 18.0'));
  assert.deepEqual(calls.slice(reads).map(c=>c.method),['SetScoutQueue'],'render/hero changes fetched extra player data');
  for(const width of [280,320,420]){
   await side.setViewportSize({width,height:720});
   assert(await side.locator('.client-sidebar').evaluate(el=>el.scrollWidth<=el.clientWidth+1));
   assert.equal(await side.locator('.sidebar-heading .queue-switch').evaluate(el=>getComputedStyle(el).gap),'8px','mode switch overrode the sidebar layout spacing');
   await rankFits(side);
  }
  await side.setViewportSize({width:320,height:720});
  await side.getByRole('button',{name:'收起客户端侧栏'}).click();await side.getByRole('button',{name:'展开客户端侧栏'}).waitFor();
  snapshot={...snapshot,phase:'Matchmaking',stage:'party',participants:[players[0]]};await publish();
  assert.equal(await side.locator('.sidebar-player').count(),0,'stage change reopened manually collapsed rail');
  await side.getByRole('button',{name:'展开客户端侧栏'}).click();
  await side.locator('.sidebar-player').waitFor();assert.equal(await side.locator('.sidebar-hero').count(),0);
  snapshot={...snapshot,phase:'GameStart',stage:'in-game',participants:structuredClone(players),gameId:42};await publish();
  await side.setViewportSize({width:1100,height:640});await overlay('preview');
  await side.locator('.overlay-player').nth(9).waitFor();
  await side.getByText(/载入快览 · 6 秒后收起/).waitFor();
  await rankFits(side);
  await shot(side,'loading-preview');
  snapshot={...snapshot,phase:'InProgress'};await publish();
  await side.getByText(/开局快览 · 6 秒后收起/).waitFor();

  const readOnlyCalls=calls.length;
  for(const index of [8,9])snapshot.participants[index]={...snapshot.participants[index],displayName:'玩家名称暂不可用',historyState:'loading',history:[],scout:{...snapshot.participants[index].scout,state:'loading',read:0,complete:false,complete12:false,complete30:false,champions:[],queues:[]}};await publish();
  await side.getByText('正在补齐 2 人统计',{exact:true}).waitFor();
  assert.equal(await side.locator('.overlay-player .scout-stat-skeleton').count(),2,'pending players did not render stable skeletons');
  snapshot.participants[8]=structuredClone(players[8]);await publish();
  await side.getByText('正在补齐 1 人统计',{exact:true}).waitFor();
  assert.equal(await side.locator('.overlay-player').nth(8).locator('.scout-stat-skeleton').count(),0,'completed player remained a skeleton');
  assert.equal(await side.locator('.overlay-player').nth(9).locator('.scout-stat-skeleton').count(),1,'slow player lost its skeleton before data arrived');
  assert((await side.locator('.overlay-player').nth(9).innerText()).includes('正在获取'));
  snapshot.participants[9]=structuredClone(players[9]);await publish();
  await side.locator('.overlay-player .scout-stat-skeleton').waitFor({state:'detached'});
  assert.equal(calls.length,readOnlyCalls,'initial-match progress/name update triggered a player query');
  assert.equal(await side.locator('.overlay-controls').count(),0);
  assert((await side.locator('.overlay-footer').innerText()).includes('6 秒后收起'));
  assert.equal(await side.locator('.overlay-shell').evaluate(el=>getComputedStyle(el).pointerEvents),'none');
  const fit=await side.locator('.overlay-player').evaluateAll(els=>els.every(el=>{const r=el.getBoundingClientRect();return r.width>=160 && r.bottom<=innerHeight;}));assert(fit,'1100x640 preview cards do not fit');
  await shot(side,'preview');await overlay('hold');
  assert((await side.locator('.overlay-footer').innerText()).includes('松开 ~ 隐藏'));
  await side.setViewportSize({width:400,height:360});
  assert(await side.locator('.overlay-player').evaluateAll(els=>els.every(el=>el.getBoundingClientRect().width>=160)));
  await rankFits(side);
  await shot(side,'hold-minimum');
  const beforeReview=calls.length;
  const main=await pageAt('/',1280,800);
  await main.getByRole('navigation',{name:'工作区'}).getByRole('button',{name:'对局室',exact:true}).click();
  await main.getByRole('heading',{name:'当前对局',exact:true}).waitFor();
  assert.equal(await main.locator('.current-match .player-row').count(),10,'live room waited for postgame review before showing prepared players');
  const current=main.locator('.current-match'),review=main.locator('.review-room');
  const roomNav=main.getByRole('navigation',{name:'对局室内容'});
  const beforeCurrent=calls.length;
  await current.getByRole('combobox',{name:'对局统计模式'}).selectOption('420');
  try{await current.locator('.queue-switch select option[value="0"]:checked').waitFor({state:'attached'});}catch(e){throw Error(String(e)+' queue='+snapshot.scoutQueue+' actual='+await current.getByRole('combobox',{name:'对局统计模式'}).inputValue()+' calls='+JSON.stringify(calls.slice(beforeCurrent)));}
  assert.equal(await current.getByRole('combobox',{name:'对局统计模式'}).inputValue(),'0');
  assert.equal(snapshot.scoutQueue,420,'current queue did not synchronize with companion');
  assert.deepEqual(calls.slice(beforeCurrent).map(c=>c.method),['SetScoutQueue']);
  await current.locator('.player-row').first().click();
  const liveDrawer=main.locator('.player-drawer');
  await liveDrawer.getByRole('heading',{name:players[0].displayName,exact:true}).waitFor();
  await liveDrawer.getByLabel('分析样本场数',{exact:true}).selectOption('50');
  assert((await liveDrawer.locator('.browse-status').innerText()).includes('本局已准备战绩'));
  assert(await liveDrawer.getByRole('button',{name:'刷新数据',exact:true}).isDisabled());
  await main.keyboard.press('Escape');await liveDrawer.waitFor({state:'detached'});
  snapshot.participants[9]={...snapshot.participants[9],displayName:'当前更新玩家#DEMO'};await publish();
  await current.locator('.player-row').nth(9).getByText('当前更新玩家#DEMO',{exact:true}).waitFor();
  assert.deepEqual(calls.slice(beforeCurrent).map(c=>c.method),['SetScoutQueue'],'live rendering or cached drawer queried player data');
  for(const [phase,stage,count] of [['Lobby','party',2],['ChampSelect','preparing',5],['GameStart','in-game',10],['InProgress','in-game',10]]){
   snapshot={...snapshot,phase,stage,participants:structuredClone(players)};if(phase==='Lobby')snapshot.participants=players.slice(0,2);await publish();
   assert.equal(await current.locator('.player-row').count(),count,phase+' used finished review or exposed opponents before loading');
   if(count<10)assert.equal(await current.getByRole('region',{name:'对方',exact:true}).count(),0);
  }
  snapshot={...snapshot,participants:[]};await publish();await current.getByText('等待当前名单',{exact:true}).waitFor();
  snapshot={...snapshot,participants:structuredClone(players)};await publish();
  for(const [width,height]of [[1280,800],[1100,760],[880,620]]){await main.setViewportSize({width,height});assert(await current.evaluate(el=>el.scrollWidth<=el.clientWidth+1),'current roster overflowed');}
  await main.setViewportSize({width:1280,height:800});await shot(main,'current-match');
  snapshot={...snapshot,connected:false};await publish();await current.getByText(/连接中断 · 保留已准备资料/).waitFor();
  assert.equal(await current.locator('.player-row').count(),10,'disconnect erased prepared live roster');
  snapshot={...snapshot,connected:true};await publish();
  const captured=structuredClone(players);
  captured[9].scout=null;
  snapshot={...snapshot,phase:'EndOfGame',stage:'review',review:{connectionId:1,gameId:42,region:'SYNTHETIC',selfKey:players[0].key,queueId:420,scoutQueue:420,capturedAt:time,state:'unavailable',participants:captured}};await publish();
  await main.getByRole('heading',{name:'赛后分析',exact:true}).waitFor();
  await main.getByRole('button',{name:'同步结算',exact:true}).click();
  await main.locator('.review-result').getByText('本局结果已同步').waitFor();
  assert((await main.locator('.review-result tbody tr').first().innerText()).includes('17 / 3 / 10'),'actual result replaced by pregame average');
  await shot(main,'review-result');
  await main.getByRole('button',{name:'赛前资料快照',exact:true}).click();
  await review.locator('.player-row').nth(9).waitFor();
  const missingReport=await review.locator('.player-row').nth(9).innerText();
  assert(missingReport.includes('未保留') && !missingReport.includes('准备中') && !missingReport.includes('正在准备'),'completed review retained a running statistics state');
  const fixedCards=await review.locator('.team-columns').innerText();
  snapshot={...snapshot,phase:'Lobby',stage:'party',gameId:0,participants:[players[0]]};await publish();
  await main.getByRole('heading',{name:'当前对局',exact:true}).waitFor();
  assert.equal(await current.locator('.player-row').count(),1,'old review hid new lobby');
  await roomNav.getByRole('button',{name:'最近复盘',exact:true}).click();
  assert.equal(await review.locator('.team-columns').innerText(),fixedCards,'new lobby mutated archived players');
  snapshot={...snapshot,phase:'ChampSelect',stage:'preparing',gameId:43,participants:[{...players[0],championId:99,displayName:'下一局玩家#DEMO'}]};await publish();
  await current.getByText('下一局玩家#DEMO',{exact:true}).waitFor();
  assert.equal(await roomNav.getByRole('button',{name:'当前对局',exact:true}).getAttribute('aria-pressed'),'true','new selection stayed on the old review');
  await roomNav.getByRole('button',{name:'最近复盘',exact:true}).click();
  await main.getByRole('navigation',{name:'工作区'}).getByRole('button',{name:'首页',exact:true}).click();
  await main.getByRole('button',{name:'查看当前对局',exact:false}).click();
  await current.getByText('下一局玩家#DEMO',{exact:true}).waitFor();
  await roomNav.getByRole('button',{name:'最近复盘',exact:true}).click();
  assert.equal(await review.locator('.team-columns').innerText(),fixedCards,'new selection replaced completed review');
  const beforeMode=calls.length;
  await review.getByRole('combobox',{name:'对局统计模式'}).selectOption('440');
  assert.equal(calls.length,beforeMode,'review mode switch changed live mode or fetched data');
  assert.equal(snapshot.scoutQueue,420);
  assert((await review.locator('.player-row').first().innerText()).includes('1360 场'));
  await publish();
  assert.equal(await roomNav.getByRole('button',{name:'最近复盘',exact:true}).getAttribute('aria-pressed'),'true','repeated snapshot overrode explicit review choice');
  for(const [width,height]of [[1280,800],[1100,760],[880,620]]){await main.setViewportSize({width,height});assert(await main.locator('.review-room').evaluate(el=>el.scrollWidth<=el.clientWidth+1));}
  await main.setViewportSize({width:1280,height:800});await shot(main,'review-pregame');
  const first=review.locator('.player-row').first();await first.click();
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
  snapshot={...snapshot,phase:'GameStart',stage:'in-game',participants:structuredClone(players)};await publish();
  await main.getByRole('heading',{name:'当前对局',exact:true}).waitFor();
  assert.equal(await current.locator('.player-row').count(),10,'loading kept old postgame view');
  await roomNav.getByRole('button',{name:'最近复盘',exact:true}).click();
  assert.equal(await review.getByRole('combobox',{name:'对局统计模式'}).inputValue(),'440','switching room contexts erased review filter');
  await review.getByRole('button',{name:'本局结果',exact:true}).click();
  assert(await review.getByRole('button',{name:'同步结算',exact:true}).isDisabled(),'in-game review requested settlement');
  await main.getByRole('button',{name:'设置与诊断',exact:true}).click();await main.getByRole('tab',{name:'游戏辅助',exact:true}).click();
  await main.getByLabel('开局自动展示快览',{exact:true}).uncheck();await main.getByLabel('侧栏宽度',{exact:true}).selectOption('360');await main.getByLabel('启用 OP.GG 英雄攻略',{exact:true}).uncheck();
  await main.getByRole('button',{name:'保存游戏辅助设置',exact:true}).click();await main.getByText('游戏辅助设置已保存。').waitFor();
  assert.equal(info.previewEnabled,false);assert.equal(info.sidebarWidth,360);assert.equal(info.guidesEnabled,false);
  await shot(main,'settings-companion');
  await main.keyboard.press('Escape');
  snapshot={...snapshot,connectionId:2,selfKey:'new-synthetic-account',phase:'Lobby',stage:'party',gameId:0,participants:[],review:null};await publish();
  await main.getByRole('heading',{name:'当前对局',exact:true}).waitFor();
  assert.equal(await current.locator('.player-row').count(),0,'new account retained the previous live roster');
  assert.equal(await review.locator('.player-row').count(),0,'new account retained archived players');
  await roomNav.getByRole('button',{name:'最近复盘',exact:true}).click();
  await review.getByText('尚无赛后记录',{exact:true}).waitFor();
  assert(calls.slice(beforeReview).some(c=>c.method==='RefreshReview'));
  assert.equal(calls.filter(c=>c.method==='AuxiliaryReady').length,1,'auxiliary frontend reloaded between modes');
  assert(!calls.some(c=>/Lookup|HistoryPage|EnsureProfile|RefreshData/.test(c.method)),'cache-only companion/review paths fetched player data');
  assert.deepEqual(errors,[]);assert.deepEqual(external,[]);
  const mayhemPlayers=structuredClone(players.slice(0,5));
  for(const p of mayhemPlayers){
   p.connectionId=2;
   const modeStats={games:3,wins:2,losses:0,outcomeGames:2,kdaGames:2,winRate:100,kda:8,remakes:1};
   p.scout={state:'partial',read:3,fetchedAt:time,complete:false,complete12:false,complete30:false,champions:[{championId:23,queues:[{queueId:2400,stats:modeStats,average:[6,2,10]}]}],queues:[{queueId:2400,twelve:modeStats,month:modeStats,positions:[{position:'TOP',games:3}]}]};
  }
  snapshot={...snapshot,queueId:2400,scoutQueue:2400,phase:'ChampSelect',stage:'preparing',gameId:123,selfKey:mayhemPlayers[0].key,participants:mayhemPlayers};
  const modeCalls=calls.length;
  await overlay('sidebar');await publish();
  await side.locator('.sidebar-player').first().waitFor();
  await side.locator('.sidebar-player summary').first().click();
  await side.locator('.sidebar-player[open] .scout-kda').getByText('6.0',{exact:true}).waitFor();
  assert.equal(await side.locator('.sidebar-player[open] .scout-position-cell').count(),0,'mayhem rendered ranked lane');
  assert((await side.locator('.sidebar-player[open]').innerText()).includes('海克斯大乱斗'));
  assert((await side.locator('.sidebar-player[open]').innerText()).includes('排位参考'));
  for(const width of [280,320]){
   await side.setViewportSize({width,height:720});
   assert(await side.locator('.client-sidebar').evaluate(el=>el.scrollWidth<=el.clientWidth+1),'mayhem sidebar overflow');
  }
  await shot(side,'sidebar-mayhem');
  await side.getByRole('combobox',{name:'对局统计模式'}).selectOption('450');
  await side.locator('.queue-switch select option[value="450"]:checked').waitFor({state:'attached'});
  assert.equal(await side.locator('.sidebar-player[open] .scout-kda').count(),0,'ARAM borrowed mayhem hero stats');
  await side.getByRole('combobox',{name:'对局统计模式'}).selectOption('0');
  await side.locator('.sidebar-player[open] .scout-kda').waitFor();
  assert.equal(calls.slice(modeCalls).filter(c=>/Lookup|HistoryPage|EnsureProfile|RefreshData/.test(c.method)).length,0);
  snapshot={...snapshot,phase:'InProgress',stage:'in-game'};await publish();
  await side.setViewportSize({width:1100,height:640});await overlay('preview');
  await side.locator('.overlay-player').first().waitFor();
  assert(await side.locator('.overlay-player .scout-scope').evaluateAll(els=>els.every(el=>el.scrollWidth<=el.clientWidth+1)),'mayhem scope overflowed preview cards');
  assert.equal(await side.locator('.overlay-player .scout-position-cell').count(),0);
  const referenceRects=await side.locator('.overlay-player .mode-rank-reference').evaluateAll(els=>els.map(el=>({self:el.getBoundingClientRect().toJSON(),parent:el.parentElement.getBoundingClientRect().toJSON()})));
  await shot(side,'mayhem-reference-check');
  assert(referenceRects.every(r=>r.self.top>=r.parent.top-1 && r.self.bottom<=r.parent.bottom+1),'rank reference escaped header: '+JSON.stringify(referenceRects));
  assert(await side.locator('.overlay-player .scout-scope').evaluateAll(els=>els.every(el=>[...el.children].every(child=>child.getBoundingClientRect().bottom<=el.getBoundingClientRect().bottom+1))),'mayhem scope clipped vertically');
  await shot(side,'mayhem-preview');
  console.log('PASS: mayhem without season, no lane, rank reference, 280/320 geometry and cache-only mode switches');
  console.log('PASS: live room without review, party/selection allies, loading/game roster, empty/late updates, disconnected cache, home/current priority, archived review and mode/filter isolation, cache-only drawers, 1280/1100/880; scoped solo/flex sidebar, 1100/400 preview and hold, settlement, seven items/focus, companion settings; synthetic bridge only.');
 }finally{await context?.unrouteAll({behavior:'ignoreErrors'});await browser.close();}
}
main().catch(e=>{console.error(e);process.exitCode=1});
