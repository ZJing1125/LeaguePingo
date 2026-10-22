// Synthetic bridge regression for season experience, two ranked modes and home isolation.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const { mockWindowRuntime } = require('./window-runtime.cjs');
const { stats } = require('./scout-fixtures.cjs');
async function main() {
 const browser = await chromium.launch({ headless: true, executablePath: process.env.BROWSER_EXECUTABLE });
 try {
  const context = await browser.newContext({ viewport: { width:1280,height:800 } });
  const page = await context.newPage(); page.setDefaultTimeout(10000);
  const errors=[],external=[],calls=[]; page.on('pageerror',e=>errors.push(e.message));
  const binding=await fs.readFile(path.join(__dirname,'../bindings/leaguepingo/appservice.ts'),'utf8');
  const methods=Object.fromEntries([...binding.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g)].map(m=>[m[2],m[1]]));
  const base='http://127.0.0.1:43204';
  const fixed='2026-10-21T12:00:00Z';
  const summary=(games,rate,kda=3)=>({games,wins:games*rate/100,losses:games*(100-rate)/100,outcomeGames:games,kdaGames:games,winRate:rate,kda});
  const report={state:'ready',fetchedAt:fixed,season:{name:'2026 合成赛季',start:'2026-01-01T00:00:00Z',end:'2027-01-01T00:00:00Z'},complete:true,complete12:true,complete30:true,read:1600,
   champions:[{championId:23,games:1360,mastery:{championId:23,level:35,points:910000},queues:[{queueId:420,stats:summary(800,60),average:[8,3,7]},{queueId:440,stats:summary(360,40),average:[4,5,18]}]}],
   queues:[{queueId:420,season:summary(900,60),twelve:summary(10,60),month:summary(100,58),positions:[{position:'TOP',games:70},{position:'JUNGLE',games:30}]},{queueId:440,season:summary(400,40),twelve:summary(6,50),month:summary(72,50),positions:[{position:'UTILITY',games:60},{position:'MIDDLE',games:12}]}]};
  const avatar='data:image/svg+xml;base64,'+Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64"><rect width="64" height="64" rx="8" fill="#61529c"/><circle cx="32" cy="30" r="16" fill="#edc583"/></svg>').toString('base64');
  function match(i,queueId=450) {return {gameId:900000+i,championId:99,queueId,playedAt:fixed,win:i%3!==0,kills:6,deaths:3,assists:9,duration:1800,items:[],spell1:4,spell2:14};}
  const players=Array.from({length:10},(_,i)=>({connectionId:1,key:'scout-'+i,puuid:'scout-'+i,team:i<5?'ally':'enemy',identityState:'known',championId:23,championState:'selected',displayName:`赛季合成玩家${i+1}#DEMO`,historyState:'ready',history:Array.from({length:20},(_,j)=>match(j)),historyMeta:{fetchedAt:fixed,returned:20,parsed:20},stats:{games:20},scout:structuredClone(report),privacyState:'public',profile:{level:681,profileIconData:avatar,rankState:'ready',updatedAt:fixed,ranks:[{queue:'RANKED_SOLO_5x5',tier:'EMERALD',division:'II',lp:86,wins:154,losses:166},{queue:'RANKED_FLEX_SR',tier:'GOLD',division:'III',lp:27,wins:117,losses:135}],fields:{rank:{state:'ready',fetchedAt:fixed}}}}));
  let snapshot={connectionId:1,selfKey:players[0].key,generation:1,connected:true,phase:'ChampSelect',stage:'preparing',queueId:440,scoutQueue:440,gameId:5151,participants:players,knownCount:10,readyCount:10,updatedAt:new Date(Date.now()-60000).toISOString()};
  const allPages=[];
  await context.route('**/*',route=>{external.push(route.request().url());return route.abort();});
  await context.route(base+'/**',async route=>{
   const url=new URL(route.request().url());
   if(url.pathname==='/wails/custom.js')return route.fulfill({contentType:'text/javascript',body:''});
   if(url.pathname==='/wails/runtime'){
    const req=route.request().postDataJSON();
    for(const runtime of allPages)if(await runtime.handle(route,req))return;
    const method=methods[req.args?.methodID],args=req.args?.args??[];calls.push(method);let result;
    if(method==='Snapshot')result=snapshot;
    else if(method==='OverlayStatus')result={visible:true,pinned:true,mode:'hold',revision:1};
    else if(method==='SetScoutQueue'){assert([420,440].includes(args[0]));snapshot={...snapshot,scoutQueue:args[0],updatedAt:new Date().toISOString()};result=snapshot;}
    else if(method==='HistoryPage'){
     assert.equal(args[0],players[0].key);const offset=args[2];assert([20,40].includes(offset));
     result={matches:Array.from({length:20},(_,j)=>match(offset+j,420)),meta:{fetchedAt:fixed,returned:20,parsed:20,excluded:0},offset,nextOffset:offset+20,hasMore:true};
    }else if(method==='CurrentPlayer')result=players[0];
    else if(method!=='PrioritizePlayer')throw new Error('Unexpected service '+method);
    return route.fulfill({json:result});
   }
   const name=url.pathname==='/'?'index.html':url.pathname.slice(1);assert(!name.includes('..'));
   return route.fulfill({body:await fs.readFile(path.join(__dirname,'../dist',name)),contentType:({'.js':'text/javascript','.css':'text/css','.jpg':'image/jpeg','.png':'image/png'})[path.extname(name)]??'text/html'});
  });
  allPages.push(mockWindowRuntime(page));
  async function screenshot(name){if(process.env.UI_SCREENSHOT_PREFIX)await page.screenshot({path:`${process.env.UI_SCREENSHOT_PREFIX}-${name}.png`});}
  async function emit(target=page){snapshot={...snapshot,updatedAt:new Date().toISOString(),generation:snapshot.generation+1};await target.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:snapshot',data}),snapshot);}
  await page.goto(base+'/');
  const nav=page.getByRole('navigation',{name:'工作区'});
  await nav.getByRole('button',{name:'当前对局',exact:true}).click();
  const first=page.locator('.player-row').first();
  await first.locator('.scout-kda').filter({hasText:'4.0 / 5.0 / 18.0'}).waitFor();
  assert.equal(await page.getByRole('button',{name:'灵活组排',exact:true}).getAttribute('aria-pressed'),'true');
  assert((await first.innerText()).includes('1360 场'));assert((await first.locator('.scout-rings').innerText()).includes('辅助'));
  assert((await first.locator('.rank-line').innerText()).includes('黄金 III'));
  await first.locator('img.rank-emblem').evaluate(img=>img.decode());
  assert.equal(await first.locator('img.rank-emblem').evaluate(img=>img.naturalWidth),1000);
  const before=calls.length;
  await page.getByRole('button',{name:'单双排',exact:true}).click();
  await first.locator('.scout-kda').filter({hasText:'8.0 / 3.0 / 7.0'}).waitFor();
  assert((await first.locator('.rank-line').innerText()).includes('翡翠 II'));
  assert((await first.locator('.scout-rings').innerText()).includes('上路'));
  assert((await first.locator('.scout-rings').innerText()).includes('100 场'));
  assert.deepEqual(calls.slice(before),['SetScoutQueue'],'mode switch must be cache-only');
  assert((await page.getByLabel('本局资料准备进度').innerText()).replace(/\s/g,'').includes('统计完整10/10'));
  await screenshot('season-solo');
  await page.getByRole('button',{name:'灵活组排',exact:true}).click();
  await first.locator('.scout-kda').filter({hasText:'4.0 / 5.0 / 18.0'}).waitFor();
  await screenshot('season-flex');
  const overlay=await context.newPage();allPages.push(mockWindowRuntime(overlay));await overlay.setViewportSize({width:1100,height:640});overlay.on('pageerror',e=>errors.push(e.message));
  await overlay.goto(base+'/?view=overlay');
  await overlay.locator('.overlay-player .rank-line').first().filter({hasText:'黄金 III'}).waitFor();
  assert.equal(await overlay.locator('button').count(),0,'hold overlay is read-only');
  const expectedTime=await overlay.evaluate(value=>new Date(value).toLocaleString('zh-CN',{month:'numeric',day:'numeric',hour:'2-digit',minute:'2-digit'}),fixed);
  assert((await overlay.locator('.overlay-footer').innerText()).includes(expectedTime), 'overlay time must come from the independent report');
  const noCalls=calls.length;await overlay.reload();await overlay.locator('.overlay-player').nth(9).waitFor();assert(calls.slice(noCalls).every(c=>['Snapshot','OverlayStatus'].includes(c)));
  if(process.env.UI_SCREENSHOT_PREFIX)await overlay.screenshot({path:`${process.env.UI_SCREENSHOT_PREFIX}-season-overlay.png`});
  await nav.getByRole('button',{name:'首页',exact:true}).click();
  await page.locator('.home-account h2').filter({hasText:players[0].displayName}).waitFor();
  await page.locator('.hero-splash').evaluate(img=>img.decode());
  assert(await page.locator('.hero-splash').evaluate(img=>img.naturalWidth>=1000));
  assert.equal(await page.locator('.home-rank img.rank-emblem').count(),2);
  assert.equal(await page.locator('.home-workspace').evaluate(el=>getComputedStyle(el).scrollbarWidth),'auto','custom WebView scrollbar must not be replaced by the native thin style');
  const boxes=await page.locator('.home-ranks,.home-recent,.home-mains').evaluateAll(els=>els.map(el=>({top:el.getBoundingClientRect().top,bottom:el.getBoundingClientRect().bottom})));
  const sparseLayout=await page.locator('.home-grid').evaluate(el=>el.classList.contains('sparse-mains'));
  assert(Math.abs(boxes[0].bottom-boxes[1].bottom)<2,'overview main columns must align at the bottom');
  assert(sparseLayout ? boxes[2].top>=boxes[1].bottom : Math.abs(boxes[2].bottom-boxes[1].bottom)<2,'champion summary must use an aligned column or its own compact row');
  await page.getByRole('combobox',{name:'首页战绩模式',exact:true}).selectOption('420');
  await page.waitForFunction(()=>!document.querySelector('.home-browse-status')?.textContent.includes('正在查找'));
  await page.getByRole('combobox',{name:'首页分析场数',exact:true}).selectOption('30');
  await page.locator('.home-history-filters > span').filter({hasText:'30 / 30 场'}).waitFor();
  assert.equal(calls.filter(c=>c==='HistoryPage').length,2);
  await page.getByRole('navigation',{name:'首页战绩分页'}).getByRole('button',{name:'下一页',exact:true}).click();
  assert((await page.locator('.home-pagination').innerText()).includes('第 2 页'));
  assert.equal(calls.filter(c=>c==='HistoryPage').length,2,'home cached pagination fetched twice');
  assert(await page.locator('.home-grid').evaluate(el=>el.classList.contains('sparse-mains')));
  assert(await page.locator('.home-mains').evaluate(el=>el.getBoundingClientRect().height<140), 'small champion sets should use a compact strip, not a stretched panel');
  await screenshot('home-filtered');
  await nav.getByRole('button',{name:'当前对局',exact:true}).click();
  assert((await first.innerText()).includes('1360 场'));assert((await first.locator('.scout-rings').innerText()).includes('72 场'));
  snapshot.participants[0].scout={...structuredClone(report),state:'loading',complete:false,complete12:false,complete30:false};
  await emit();
  await page.getByLabel('本局资料准备进度').getByText('正在补齐 1 人统计').waitFor();
  const progress=await page.getByLabel('本局资料准备进度').innerText();
  assert(progress.replace(/\s/g,'').includes('近期战绩10/10') && progress.replace(/\s/g,'').includes('统计完整9/10'));
  const empty=stats([]);snapshot.participants[0].scout={...structuredClone(report),state:'partial',complete:false,complete12:false,complete30:false,champions:[{...report.champions[0],games:null,queues:[{queueId:420,stats:empty,average:[]},{queueId:440,stats:empty,average:[]}]}]};
  await emit();await first.getByText('本季记录未完整',{exact:true}).waitFor();
  assert(!(await first.innerText()).includes('样本不足'));assert(!(await first.innerText()).includes('累计数据暂不可用'));
  assert((await first.innerText()).includes('熟练度 35 级'));
  await screenshot('partial-experience');
  snapshot.participants[0].scout={...structuredClone(report),state:'partial',retained:true,fetchedAt:'2026-10-21T11:59:00Z'};
  const retainedCalls=calls.length;await emit();
  await first.locator('.scout-scope').filter({hasText:'上次统计'}).waitFor();
  assert((await first.innerText()).includes('1360 场'));
  assert.equal(calls.length,retainedCalls,'retained data display must not request a refresh');
  for(const size of [{width:1100,height:760},{width:880,height:620}]){await page.setViewportSize(size);assert.equal(await page.locator('.match-workspace').evaluate(el=>el.scrollWidth>el.clientWidth+1),false);}
  assert.deepEqual(errors,[]);assert.deepEqual(external,[]);
  console.log('PASS: season hero beyond 20 games, dual ranked mode/rank/window/role switch without requests, hold-overlay cache, HD official assets, aligned home filters/pagination, workspace isolation and retained refresh data.');
 } finally {await browser.close();}
}
main().catch(e=>{console.error(e);process.exitCode=1;});
